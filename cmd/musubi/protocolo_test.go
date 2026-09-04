package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// camposQueElAgenteIgnoraAProposito son los campos de la respuesta del latido que el agente NO
// consume, cada uno con el motivo. Una lista de excepciones DECLARADAS, no un descuido.
var camposQueElAgenteIgnoraAProposito = map[string]string{
	"ok": "el desenlace lo decide el CÓDIGO HTTP, que es el que el agente ya tiene que interpretar " +
		"igual (401 = baja, 429 = lockout, 5xx = backoff). Un `ok` en el cuerpo sería una segunda " +
		"fuente para lo mismo, y las dos fuentes se contradicen el día que una se olvida.",
	"device": "el agente ya sabe qué máquina es; el cerebro lo devuelve para que se vea en una " +
		"prueba de instalación con curl.",
	"project": "a quién pertenece la máquina lo decide el cerebro por el token, y el agente no " +
		"toma ninguna decisión con eso (invariante B4/D5).",
	"motivo": "viaja SÓLO en el 401 y es el MISMO texto para todos los rechazos, a propósito (B3). " +
		"El agente pone su propio texto en resultadoLatido porque el del cerebro es uniforme por " +
		"diseño y no agrega nada.",
}

// TODO CAMPO DE LA RESPUESTA DEL LATIDO ESTÁ CONSUMIDO O ESTÁ DECLARADO COMO IGNORADO.
//
// Ésta es la guarda de la clase de defecto, no de un defecto. El contrato vivía duplicado —un tipo
// en internal/mcp y un struct anónimo en el agente— y `encoding/json` DESCARTA EN SILENCIO lo que
// el receptor no declara: sin error, sin log, sin nada. Así se perdieron dos campos:
//
//	· `token_nuevo`, y la rotación de token de la Ola 2 no podía completarse nunca;
//	· `servicios`, cuyo único propósito era que un inventario descartado NO desapareciera en
//	  silencio — y desaparecía en silencio.
//
// Unificar el tipo hace que el compilador ate los dos lados, pero no obliga a nadie a DECIDIR qué
// hace el agente con un campo nuevo. Eso lo obliga esta prueba: un campo que aparezca en el
// contrato y no esté ni consumido ni declarado acá la pone roja.
//
// Sabotaje que la hace fallar: agregar un campo a fleet.RespuestaLatido sin tocar nada más.
func TestNingunCampoDeLaRespuestaDelLatidoSePierdeEnSilencio(t *testing.T) {
	// Los que el agente sí consume, cada uno verificado de verdad más abajo.
	consumidos := map[string]bool{
		"muestra": true, "servicios": true, "comandos": true, "token_nuevo": true,
	}

	tipo := reflect.TypeOf(fleet.RespuestaLatido{})
	vistos := 0
	for i := 0; i < tipo.NumField(); i++ {
		nombre := strings.Split(tipo.Field(i).Tag.Get("json"), ",")[0]
		if nombre == "" || nombre == "-" {
			continue
		}
		vistos++
		if consumidos[nombre] {
			continue
		}
		if _, declarado := camposQueElAgenteIgnoraAProposito[nombre]; declarado {
			continue
		}
		t.Errorf("el campo %q del contrato no está consumido por el agente ni declarado como "+
			"ignorado: decidí qué hace el agente con él. Si no hace nada, agregalo a "+
			"camposQueElAgenteIgnoraAProposito CON EL MOTIVO — encoding/json lo va a descartar "+
			"en silencio y nadie se va a enterar", nombre)
	}
	// CONTROL DE QUE LA PRUEBA MIRA ALGO. Sin esto, un cambio que dejara el struct sin tags haría
	// que el bucle no itere y la prueba pasara en verde sin haber verificado un solo campo.
	if vistos < 6 {
		t.Fatalf("se recorrieron %d campos del contrato; son al menos 6 — ¿el struct perdió sus tags?", vistos)
	}
}

// LOS CUATRO CAMPOS QUE EL AGENTE CONSUME LLEGAN DE VERDAD, y no sólo están declarados arriba.
//
// La lista de `consumidos` de la prueba anterior es una afirmación; ésta es la que la sostiene. Sin
// ella, alguien podría agregar un nombre a esa lista para callar el rojo sin cablear nada — que es
// exactamente el atajo que convierte una guarda en decoración.
//
// Sabotaje que la hace fallar: quitar cualquiera de los cuatro del decode de latir().
func TestLosCuatroCamposQueElAgenteConsumeLleganDeVerdad(t *testing.T) {
	cuerpo, err := json.Marshal(fleet.RespuestaLatido{
		OK: true, Device: "pc-gio", Project: "casa",
		Muestra:    "guardada",
		Servicios:  "descartados: falta la capacidad `services`",
		TokenNuevo: "msb_el_rotado",
		Comandos:   []fleet.ComandoParaElAgente{{ID: "cmd-1", Argv: []string{"uptime"}, TimeoutSeg: 30}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cuerpo)
	}))
	defer ts.Close()

	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) { return nil, nil }
	t.Cleanup(func() { enumerarServicios = anterior })

	res := latir(ts.URL+"/fleet/heartbeat", "tok", nil)
	if !res.ok {
		t.Fatalf("el latido falló: %+v", res)
	}
	// muestra y servicios se ven en el texto que el agente IMPRIME, que es donde tienen que verse:
	// quien administra esta máquina no lee los logs del cerebro.
	if !strings.Contains(res.motivo, "guardada") {
		t.Errorf("`muestra` no llegó al texto que ve la máquina: %q", res.motivo)
	}
	if !strings.Contains(res.motivo, "falta la capacidad") {
		t.Errorf("`servicios` no llegó al texto que ve la máquina: %q — es el campo que existe "+
			"para que un inventario descartado no desaparezca en silencio", res.motivo)
	}
	if len(res.comandos) != 1 || res.comandos[0].ID != "cmd-1" {
		t.Errorf("`comandos` no llegó: %+v", res.comandos)
	}
	if res.tokenNuevo != "msb_el_rotado" {
		t.Errorf("`token_nuevo` no llegó: %q", res.tokenNuevo)
	}
}

// NINGÚN VALOR DE CABLE SE VUELVE A ESCRIBIR FUERA DEL DOMINIO.
//
// Los nombres de las operaciones internas (`musubi:avisar`, `musubi:preguntar`, `musubi:pantalla`,
// `musubi:shell`) y el prefijo de la respuesta de permiso viven en internal/fleet, y estaban
// TAMBIÉN escritos a mano de este lado. El cerebro ya derivaba del dominio —methods_shell.go hace
// `const comandoShell = fleet.OpShell`— así que la copia era de un solo lado, que es la peor forma:
// parece que hay una fuente única y no la hay.
//
// LO QUE PASABA SI DIVERGÍAN, que es por qué esto tiene una prueba y no sólo un comentario:
//
//	· un nombre de operación renombrado en el dominio y no acá deja de reconocerse como operación
//	  INTERNA, y el argv cae en el camino del exec — o sea que el agente intentaría lanzar
//	  `musubi:pantalla` como si fuera un binario del host, que es literalmente lo que el
//	  comentario de ejecutor.go prohíbe;
//	· el prefijo del permiso divergido hace que el CutPrefix del cerebro no matchee y TODA la
//	  flota en modo `pide` niegue cada sesión con «el agente no tuvo con qué preguntar», habiendo
//	  preguntado y habiendo recibido un sí. Sin error, sin 4xx, con el agente reportando exit 0.
//
// EL MATCHEO ES DEL LITERAL COMPLETO —comilla, valor, comilla— y eso NO es un detalle: los mensajes
// de error de este paquete nombran las operaciones en prosa (`"musubi:avisar sin texto"`), y ahí el
// valor va seguido de un espacio y no de una comilla. Así la guarda distingue una DECLARACIÓN de
// una mención, que es lo que la hace usable en vez de un falso positivo permanente.
//
// Sabotaje que la hace fallar: volver a poner `const comandoAvisarAgente = "musubi:avisar"`.
func TestNingunValorDeCableSeRedeclaraFueraDelDominio(t *testing.T) {
	valores := map[string]string{
		fleet.OpAvisar:                "fleet.OpAvisar",
		fleet.OpPreguntar:             "fleet.OpPreguntar",
		fleet.OpPantalla:              "fleet.OpPantalla",
		fleet.OpShell:                 "fleet.OpShell",
		fleet.PrefijoRespuestaPermiso: "fleet.PrefijoRespuestaPermiso",
	}
	archivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	mirados := 0
	for _, f := range archivos {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		mirados++
		for _, l := range strings.Split(string(b), "\n") {
			for valor, constante := range valores {
				if strings.Contains(l, `"`+valor+`"`) {
					t.Errorf("%s escribe el literal %q a mano: usá %s. Un valor de cable con dos "+
						"declaraciones no tiene fuente única, y si divergen no falla nada — se "+
						"rompe en silencio.\n    %s", f, valor, constante, strings.TrimSpace(l))
				}
			}
		}
	}
	// CONTROL DE QUE LA PRUEBA MIRA ALGO: sin esto, un glob que no matchea nada la deja en verde
	// sin haber abierto un solo archivo.
	if mirados < 5 {
		t.Fatalf("se miraron %d archivos de este paquete; son bastantes más — ¿el glob dejó de matchear?", mirados)
	}
}
