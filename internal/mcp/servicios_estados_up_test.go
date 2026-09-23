package mcp

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// LO QUE CUSTODIA: el mapeo COMPLETO de los cinco estados de servicio a `musubi_fleet_service_up`
// —cuál se exporta, con qué valor, y cuál NO— y, además, que los estados SIGAN SIENDO ESOS CINCO.
//
// SALE DE AUDITAR A131 SOBRE `servicios_test.go`, Y EL HUECO ERA REAL. Las dos guardas hermanas
// —`TestUnServicioOciosoNoEmiteLaSerieDeUp` y `TestUnServicioDesconocidoNoEmiteLaSerieDeUp`— están
// bien escritas y llevan su control positivo adentro, que es más de lo que suele encontrarse. Pero
// las dos clavan el MISMO control: `fallado`. Y el `default` del exportador cubre DOS estados,
// `detenido` y `fallado`. Con eso, esta mutación deja el paquete entero en VERDE:
//
//	case fleet.EstadoDetenido:
//	    return 0, false      // `detenido` deja de emitir; `fallado` sigue igual
//
// Medido el 2026-09-23: mutado y corrido `./internal/mcp` completo, 0 fallas. Una sonda derivada de
// la guarda hermana, cambiando el control a `detenido`, dio PASS en el árbol sano y FAIL en el
// mutado — o sea que la mutación no era vacua y el hueco era de verdad.
//
// LA EXPOSICIÓN NO ERA CERO, Y ESO DECIDIÓ ARREGLARLO EN VEZ DE SÓLO ANOTARLO. En la base del
// cerebro ese mismo día: 103 servicios `corriendo`, 17 `ocioso`, 1 `fallado` y **2 `detenido`** —
// y los dos `detenido` eran EXACTAMENTE los dos que tenían `ServicioCaido` disparada
// (`agora-searx` y `supabase_edge_runtime_altura-erp`). O sea que el estado sin custodiar era el
// único que estaba alertando de verdad: romperlo habría apagado las dos alarmas vivas sin poner
// nada en rojo.
//
// POR QUÉ LA TABLA ENTERA Y NO UN CASO MÁS. Agregar un control `detenido` a las dos hermanas tapa
// este agujero y deja el siguiente: el que aparezca cuando alguien agregue un sexto estado. Acá el
// conjunto se puede CERRAR —es un `const` de cinco miembros en `internal/fleet/servicio.go`— así
// que la guarda enumera los cinco y ADEMÁS comprueba contra la fuente que sigan siendo cinco. Un
// estado nuevo no se le escapa: la pone roja hasta que alguien diga qué exporta.
//
// Sabotaje: hacer que `detenido` deje de emitir la serie, dejando `fallado` intacto — que es la
// mutación que las dos guardas hermanas no ven.
// arnes: archivo="internal/mcp/fleet_prometheus_servicios.go"
// arnes: de="\t\t\t\tdefault:\n\t\t\t\t\treturn 0, true"
// arnes: a="\t\t\t\tcase fleet.EstadoDetenido:\n\t\t\t\t\treturn 0, false\n\t\t\t\tdefault:\n\t\t\t\t\treturn 0, true"
func TestCadaEstadoDeServicioExportaLoQueLeCorresponde(t *testing.T) {
	casos := []struct {
		estado fleet.EstadoServicio
		nombre string
		emite  bool
		valor  string
		porQue string
	}{
		{fleet.EstadoCorriendo, "corriendo.service", true, "1",
			"está corriendo: es el único 1 que emite el exportador"},
		{fleet.EstadoDetenido, "detenido.service", true, "0",
			"está parado de verdad, y `ServicioCaido` matchea `== 0`: sin esta serie la alerta no tiene qué ver"},
		{fleet.EstadoFallado, "fallado.service", true, "0",
			"se murió: el caso que la alerta existe para ver"},
		{fleet.EstadoOcioso, "ocioso.service", false, "",
			"«la pregunta no aplica» (A70): nadie lo pidió, y un 0 acá diría «se cayó»"},
		{fleet.EstadoDesconocido, "desconocido.service", false, "",
			"«no se sabe»: un 0 sería el exportador AFIRMANDO que no corre sobre lo único que el dominio declara no medido"},
	}

	// EL CONJUNTO SE CIERRA CITANDO LA FUENTE, no confiando en que esta tabla esté al día. Si
	// mañana nace un sexto estado, esta comprobación se pone roja ANTES que nada y obliga a
	// decidir qué exporta — que es justo la decisión que se toma mal cuando se toma por omisión.
	rutaEnum := filepath.Join("..", "fleet", "servicio.go")
	crudo, err := os.ReadFile(rutaEnum)
	if err != nil {
		t.Fatalf("leer %s: %v", rutaEnum, err)
	}
	re := regexp.MustCompile(`(?m)^\s*(Estado\w+)\s+EstadoServicio\s*=\s*"([a-z]+)"`)
	declarados := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(crudo), -1) {
		declarados[m[2]] = true
	}
	if len(declarados) == 0 {
		t.Fatalf("no encontré NINGÚN estado declarado en %s: el lector dejó de reconocer la forma "+
			"de la declaración, y con eso esta guarda pasaría en verde sin cerrar nada", rutaEnum)
	}
	enLaTabla := map[string]bool{}
	for _, c := range casos {
		enLaTabla[string(c.estado)] = true
	}
	for e := range declarados {
		if !enLaTabla[e] {
			t.Errorf("`%s` es un EstadoServicio declarado en %s y esta tabla no dice qué exporta.\n"+
				"No es un olvido cosmético: si no se decide, cae en el `default` del exportador y se\n"+
				"emite como 0 —o sea «no está corriendo»—, que es la afirmación más cara que puede\n"+
				"hacer este exportador. Agregalo a `casos` con su valor y su porqué.", e, rutaEnum)
		}
	}
	for e := range enLaTabla {
		if !declarados[e] {
			t.Errorf("esta tabla nombra el estado `%s`, que ya NO está declarado en %s: o se renombró\n"+
				"y la guarda quedó midiendo un mundo que no existe, o se borró y sobra la fila.", e, rutaEnum)
		}
	}

	// UN SOLO LATIDO CON LOS CINCO, a propósito: además de cada caso, prueba que no se pisen entre
	// sí — un exportador que decidiera por el ÚLTIMO servicio visto pasaría cinco pruebas sueltas.
	s, ts, tokenDevice, _ := servidorConFlota(t)
	var reportes []fleet.ReporteServicio
	for _, c := range casos {
		reportes = append(reportes, fleet.ReporteServicio{
			Nombre: c.nombre, Clase: "windows", Salud: saludViva(c.estado)})
	}
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(reportes...)); code != http.StatusOK {
		t.Fatalf("el latido con los cinco servicios devolvió %d: %s", code, body)
	}

	out := exportar(t, s, nil)
	if !strings.Contains(out, `musubi_fleet_service_up{`) {
		t.Fatal("no se exportó NINGUNA serie de service_up: esta guarda no estaría probando nada")
	}

	for _, c := range casos {
		var linea string
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "musubi_fleet_service_up{") && strings.Contains(l, `"`+c.nombre+`"`) {
				linea = l
			}
		}
		switch {
		case c.emite && linea == "":
			t.Errorf("`%s` NO exportó `musubi_fleet_service_up` y tenía que exportar %s.\n  %s\n"+
				"Una serie ausente deja a `ServicioCaido` sin nada que matchear: la alarma no suena\n"+
				"y nada se pone rojo, que es la forma más cara de apagar una alerta.",
				c.estado, c.valor, c.porQue)
		case c.emite && !strings.HasSuffix(linea, " "+c.valor):
			t.Errorf("`%s` exportó un valor que no es %s:\n  %s\n  %s", c.estado, c.valor, linea, c.porQue)
		case !c.emite && linea != "":
			t.Errorf("`%s` exportó `musubi_fleet_service_up` y NO tenía que exportar nada:\n  %s\n  %s",
				c.estado, linea, c.porQue)
		}
	}
	t.Logf("cinco estados verificados contra %s; %d declarados en la fuente", rutaEnum, len(declarados))
}
