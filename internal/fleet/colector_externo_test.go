package fleet

// colector_externo_test.go custodia el INVENTARIO de campos del colector externo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES UNA PRUEBA Y NO UNA NOTA EN UN README
//
// Antes de que existiera el agente, la telemetría de estas máquinas la juntaba un script de bash
// que corría fuera del repo y emitía 22 campos. Absorberlo adentro de fleet.Muestra es una
// migración, y en toda migración lo caro no es lo que se traduce mal: es lo que se PIERDE en
// silencio. Un campo que no está no rompe nada, no falla ningún build, y se descubre meses
// después, cuando alguien pregunta por un número que antes tenía.
//
// Así que la lista vive en testdata/colector-externo.json —es un DATO, no código— y esta prueba
// exige que cada campo esté resuelto de una de dos formas y de ninguna otra: o tiene un DESTINO
// que existe de verdad en la struct, o tiene un MOTIVO escrito de por qué se dejó afuera.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL ANCLAJE ESTÁ EN GO Y NO EN EL JSON, Y ES LA CORRECCIÓN DE UN AGUJERO REAL
//
// La primera versión custodiaba el conteo comparando `len(inv.Campos)` contra `inv.TotalCampos`:
// DOS NÚMEROS DEL MISMO ARCHIVO que el saboteador está editando. Eso no defiende de un borrado,
// defiende de un descuido — y encima el mensaje de error le decía cuál era el segundo número que
// tenía que bajar. Verificado: con el archivo en `{"total_campos": 0, "campos": []}` la prueba
// pasaba en verde, y dejando sólo dos filas con `total_campos: 2` la suite entera pasaba. Veinte
// filas —incluidas las de `hostname`, `os` y `arch`, cuyos motivos son los que documentan los
// invariantes de identidad— se podían borrar sin que nada se pusiera rojo.
//
// Por eso los NOMBRES de los 22 campos viven acá abajo, en Go. No es duplicación: los nombres son
// un HECHO HISTÓRICO —lo que ese script emitía en agosto de 2026— y no una decisión que vaya a
// cambiar; lo que sí cambia, y por eso sigue en el JSON, es qué se hizo con cada uno. Editar el
// dato ya no alcanza para hacer desaparecer una fila: hay que venir a borrarla acá también, y eso
// no es un descuido.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

type campoExterno struct {
	Campo   string `json:"campo"`
	Destino string `json:"destino"`
	Motivo  string `json:"motivo"`
}

type inventarioExterno struct {
	Version     int            `json:"version"`
	Fuente      string         `json:"fuente"`
	Capturado   string         `json:"capturado"`
	TotalCampos int            `json:"total_campos"`
	Campos      []campoExterno `json:"campos"`
}

// motivoMinimo son las runas que tiene que tener una exclusión para contar como razonada.
// «porque sí» no es una decisión: es la ausencia de una.
const motivoMinimo = 30

// camposDelColectorExterno son los 22 campos que emitía el script, uno por uno. Es el ANCLAJE:
// lo que el inventario tiene que resolver, escrito fuera del archivo que el inventario es.
//
// Si mañana aparece una captura del colector con un campo más, la fila se agrega ACÁ y en el JSON
// —las dos— y recién ahí la prueba vuelve a verde. Ése es el costo, y es el punto.
var camposDelColectorExterno = []string{
	"ts", "cpu_pct", "cpus", "load1", "load5", "load15",
	"mem_total", "mem_used", "mem_free", "swap_total", "swap_used",
	"disk_total", "disk_used", "disk_avail", "uptime", "temp", "procs",
	"hostname", "os", "arch", "ip", "agent_version",
}

// identidadQueNoSeAbsorbe son los campos cuya EXCLUSIÓN es un invariante y no una preferencia.
// El valor es lo que ese motivo tiene que seguir explicando.
var identidadQueNoSeAbsorbe = map[string]string{
	"hostname": "la identidad sale del TOKEN, nunca del cuerpo (B4/D5)",
	"os":       "es un label de Prometheus: dejar que la máquina lo reporte la deja re-etiquetarse sola",
	"arch":     "se declara al enrolar; una máquina que lo reporta puede contradecir su propia fila",
}

// I1 — NINGÚN CAMPO DEL COLECTOR EXTERNO SE PIERDE SIN MOTIVO DECLARADO.
//
// Sabotaje que la hace fallar (los tres VERIFICADOS): borrar la fila de "procs" del JSON, con o
// sin bajar `total_campos` —el anclaje es la lista de Go, así que bajar el número no salva—;
// vaciar el archivo entero a `{"total_campos": 0, "campos": []}`; o escribir el destino
// "mem_free" en vez de "mem_libre" (no existe ese tag json en fleet.Muestra, y el mensaje nombra
// el campo culpable).
func TestNingunCampoDelColectorExternoSePierdeSinMotivo(t *testing.T) {
	b, err := os.ReadFile("testdata/colector-externo.json")
	if err != nil {
		t.Fatalf("no se pudo leer el inventario del colector externo: %v", err)
	}
	var inv inventarioExterno
	if err := json.Unmarshal(b, &inv); err != nil {
		t.Fatalf("testdata/colector-externo.json ilegible: %v", err)
	}

	// (a) NINGÚN CAMPO REPETIDO: dos filas con el mismo nombre esconden una de las dos.
	vistos := make(map[string]bool, len(inv.Campos))
	for _, c := range inv.Campos {
		if vistos[c.Campo] {
			t.Errorf("el campo %q está dos veces en el inventario: una de las dos filas es invisible", c.Campo)
		}
		vistos[c.Campo] = true
	}

	// (b) ESTÁN LOS 22, UNO POR UNO Y CONTRA LA LISTA DE GO. Es la guarda que reemplazó al conteo
	// autorreferencial: borrar una fila del JSON nombra al campo que falta, y no hay ningún
	// segundo número en el archivo que se pueda bajar para taparlo.
	for _, campo := range camposDelColectorExterno {
		if !vistos[campo] {
			t.Errorf("el campo %q del colector externo no está en el inventario: lo emitía el script y alguien "+
				"borró su fila. Volvé a ponerla, con destino si la Muestra lo absorbe o con motivo si se descarta",
				campo)
		}
	}
	// Y AL REVÉS: una fila que el colector nunca emitió no puede colarse a hacer bulto. Si de
	// verdad apareció una captura con un campo más, se agrega a camposDelColectorExterno.
	esperados := make(map[string]bool, len(camposDelColectorExterno))
	for _, campo := range camposDelColectorExterno {
		esperados[campo] = true
	}
	for _, c := range inv.Campos {
		if !esperados[c.Campo] {
			t.Errorf("el inventario trae un campo %q que el colector externo no emitía: si la captura cambió, "+
				"agregalo también a camposDelColectorExterno en este archivo", c.Campo)
		}
	}

	// (c) `total_campos` sigue existiendo y ahora se contrasta contra la lista de Go, no contra sí
	// mismo. Vale como cartel para quien lee el JSON suelto; ya no es lo que sostiene el conteo.
	if inv.TotalCampos != len(camposDelColectorExterno) {
		t.Errorf("el JSON declara total_campos = %d y el colector emitía %d campos: corregí el número del "+
			"archivo (la lista buena es camposDelColectorExterno, en este archivo)",
			inv.TotalCampos, len(camposDelColectorExterno))
	}

	tags := tagsJSONDeMuestra()

	for _, c := range inv.Campos {
		// (d) DESTINO O MOTIVO, NUNCA LOS DOS NI NINGUNO. Los dos a la vez es una fila que dice
		// «se absorbió» y «se descartó» al mismo tiempo, y no se sabe cuál creerle.
		tieneDestino, tieneMotivo := c.Destino != "", strings.TrimSpace(c.Motivo) != ""
		switch {
		case !tieneDestino && !tieneMotivo:
			t.Errorf("el campo %q del colector externo no tiene destino ni motivo: o lo absorbe la Muestra "+
				"(poné el tag json de destino) o se descarta (escribí por qué)", c.Campo)
			continue
		case tieneDestino && tieneMotivo:
			t.Errorf("el campo %q tiene destino (%q) Y motivo de exclusión: decidí una de las dos", c.Campo, c.Destino)
			continue
		}

		if tieneDestino {
			// (e) EL DESTINO EXISTE DE VERDAD. Es lo que caza el renombre: un JSON que dice
			// "mem_free" mientras la struct dice "mem_libre" pasa cualquier revisión visual.
			if !tags[c.Destino] {
				t.Errorf("el campo %q dice ir a %q, y fleet.Muestra no tiene ningún tag json así "+
					"(¿se renombró el campo, o se escribió el nombre del colector viejo?)", c.Campo, c.Destino)
			}
			continue
		}
		// (f) EL MOTIVO ES UNA RAZÓN, NO UNA MULETILLA.
		if n := utf8.RuneCountInString(strings.TrimSpace(c.Motivo)); n < motivoMinimo {
			t.Errorf("el motivo por el que se descartó %q tiene %d runas (mínimo %d): "+
				"una exclusión de una palabra no es una decisión, es una que nadie tomó", c.Campo, n, motivoMinimo)
		}
	}
}

// LOS DOS CAMPOS DE ESTE SLICE ESTÁN, Y ESTÁN ABSORBIDOS.
//
// La prueba de arriba pasaría si alguien pusiera `procs` con un motivo de exclusión. Ésta exige
// que sigan siendo campos MEDIDOS, que es lo que U1 se comprometió a hacer.
//
// Sabotaje que la hace fallar: cambiarle el destino a "procs" por un motivo de exclusión.
func TestLosDosCamposAbsorbidosSiguenTeniendoDestino(t *testing.T) {
	b, err := os.ReadFile("testdata/colector-externo.json")
	if err != nil {
		t.Fatal(err)
	}
	var inv inventarioExterno
	if err := json.Unmarshal(b, &inv); err != nil {
		t.Fatal(err)
	}
	quiero := map[string]string{"procs": "num_procesos", "mem_free": "mem_libre"}
	for _, c := range inv.Campos {
		destino, nosImporta := quiero[c.Campo]
		if !nosImporta {
			continue
		}
		if c.Destino != destino {
			t.Errorf("%q debería ir a %q y va a %q: los dos campos de este slice se ABSORBEN, no se descartan",
				c.Campo, destino, c.Destino)
		}
		delete(quiero, c.Campo)
	}
	for campo := range quiero {
		t.Errorf("el campo %q desapareció del inventario", campo)
	}
}

// LA IDENTIDAD NO SE ABSORBE, Y ESTAS TRES EXCLUSIONES SON UN INVARIANTE.
//
// `hostname`, `os` y `arch` no quedaron afuera por gusto: el primero porque la identidad sale del
// TOKEN y de ningún otro lado (B4/D5), los otros dos porque son LABELS de Prometheus y una máquina
// que los reporta se está re-etiquetando sola. La prueba de arriba obliga a que las tres filas
// existan; ésta obliga a que sigan siendo EXCLUSIONES, que es lo que documenta el invariante.
//
// Sabotaje que la hace fallar (VERIFICADO tal cual): ponerle a "hostname" un destino cualquiera
// que exista en la Muestra —se probó con "num_cpu"— y vaciarle el motivo. Ahí la máquina pasaría a
// poder decir algo sobre su identidad, y ninguna otra prueba de este paquete lo notaría.
func TestLaIdentidadNoSeAbsorbeDelColectorExterno(t *testing.T) {
	b, err := os.ReadFile("testdata/colector-externo.json")
	if err != nil {
		t.Fatal(err)
	}
	var inv inventarioExterno
	if err := json.Unmarshal(b, &inv); err != nil {
		t.Fatal(err)
	}
	falta := make(map[string]string, len(identidadQueNoSeAbsorbe))
	for k, v := range identidadQueNoSeAbsorbe {
		falta[k] = v
	}
	for _, c := range inv.Campos {
		porque, esDeIdentidad := identidadQueNoSeAbsorbe[c.Campo]
		if !esDeIdentidad {
			continue
		}
		delete(falta, c.Campo)
		if c.Destino != "" {
			t.Errorf("el campo %q del colector externo pasó a absorberse en %q, y no puede: %s",
				c.Campo, c.Destino, porque)
		}
		if strings.TrimSpace(c.Motivo) == "" {
			t.Errorf("la exclusión de %q se quedó sin motivo escrito: %s", c.Campo, porque)
		}
	}
	for campo, porque := range falta {
		t.Errorf("la fila de %q desapareció del inventario, y su motivo es el que documenta un invariante: %s",
			campo, porque)
	}
}

// tagsJSONDeMuestra junta los nombres de los tags `json` de fleet.Muestra. Por reflect y no a
// mano: una lista escrita a mano se desincroniza de la struct y deja de custodiar nada.
func tagsJSONDeMuestra() map[string]bool {
	tipo := reflect.TypeOf(Muestra{})
	tags := make(map[string]bool, tipo.NumField())
	for i := 0; i < tipo.NumField(); i++ {
		tag := tipo.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		tags[strings.Split(tag, ",")[0]] = true
	}
	return tags
}
