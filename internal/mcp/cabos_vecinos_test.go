package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// LOS CABOS DE LAS CARPETAS VECINAS TIENEN QUE SER VISIBLES, AUNQUE NO SEAN DE ESTE TRACK.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// El barrido de `TestNingunCaboDeFlotaSeQuedaSinRegistro` cubre `specs/flota-*` y la carpeta del
// propio track, y eso está bien: `ABIERTO.md` declara en su regla 4 que cubre el track «Control
// de flota» y NO el repo entero. Leerlo como «todo lo abierto de Musubi» sería el mismo error que
// dio origen a la tabla.
//
// PERO «NO ES DE MI TRACK» NO ES «NO EXISTE». Hay `## Lo que queda fuera` en carpetas vecinas
// —`riel-local` y `profundidad-derivada-del-diff`— cuyos ítems no están en NINGÚN registro: ni en
// éste, que declara no cubrirlos, ni en uno propio, que no existe. O sea que son exactamente lo
// que la primera línea de `ABIERTO.md` prohíbe: algo abierto sin dueño.
//
// ESTA PRUEBA NO LOS REGISTRA NI FALLA POR ELLOS, y la diferencia importa. Dónde vive su registro
// —uno por track, o uno solo para todo— es una decisión de alcance que no se puede inferir del
// código. Lo que sí se puede hacer, y es lo que faltaba, es que dejen de ser invisibles: el
// conteo sale en cada corrida, con nombre y archivo, para que la decisión se tome sobre algo
// contado y no sobre una impresión.
//
// FALLA SI APARECE UNA CARPETA VECINA NUEVA CON CABOS, porque eso sí cambia la pregunta: pasa de
// «dos carpetas pendientes de decisión» a «esto se está acumulando».
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLosCabosDeLasCarpetasVecinasEstanContados(t *testing.T) {
	raiz := filepath.Join("..", "..", "specs")
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		t.Fatalf("no pude listar %s: %v", raiz, err)
	}

	encabezado := regexp.MustCompile(`(?i)^#+\s*(?:\d+\s*[·.)\-]\s*)?lo que queda`)
	vineta := regexp.MustCompile(`^\s*(?:[-*+]|\d{1,9}[.)])\s+\S`)

	type vecina struct {
		carpeta string
		archivo string
		items   int
	}
	var vecinas []vecina
	revisadas := 0
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		// Las del track ya las cubre el barrido de al lado.
		if strings.HasPrefix(e.Name(), "flota-") || e.Name() == "control-de-flota" {
			continue
		}
		revisadas++
		hijos, err := os.ReadDir(filepath.Join(raiz, e.Name()))
		if err != nil {
			continue
		}
		for _, h := range hijos {
			if h.IsDir() || !strings.HasSuffix(strings.ToLower(h.Name()), ".md") {
				continue
			}
			// EL PROPIO REGISTRO NO ES UNA FUENTE DE CABOS SIN REGISTRAR, y esto NO es la
			// excepción cómoda que este repo persigue: estar en `ABIERTO.md` ES estar registrado,
			// por definición. Un cabo no puede esconderse ahí — ahí es donde va.
			if strings.EqualFold(h.Name(), "ABIERTO.md") {
				continue
			}
			crudo, err := os.ReadFile(filepath.Join(raiz, e.Name(), h.Name()))
			if err != nil {
				continue
			}
			dentro, n := false, 0
			for _, linea := range strings.Split(string(crudo), "\n") {
				if strings.HasPrefix(linea, "#") {
					dentro = encabezado.MatchString(linea)
					continue
				}
				if dentro && vineta.MatchString(linea) {
					n++
				}
			}
			if n > 0 {
				vecinas = append(vecinas, vecina{e.Name(), h.Name(), n})
			}
		}
	}
	sort.Slice(vecinas, func(i, j int) bool { return vecinas[i].carpeta < vecinas[j].carpeta })

	if revisadas < 20 {
		t.Fatalf("sólo se revisaron %d carpetas de specs fuera del track y son muchas más: cambió "+
			"dónde viven y esta guarda está en verde sin mirar nada", revisadas)
	}

	total := 0
	for _, v := range vecinas {
		total += v.items
		t.Logf("carpeta vecina con cabos declarados: specs/%s/%s — %d ítem(s)", v.carpeta, v.archivo, v.items)
	}
	t.Logf("%d carpeta(s) vecina(s) con cabos declarados fuera de alcance, %d ítem(s) en total; cada una tiene que tener su ABIERTO.md",
		len(vecinas), total)

	// LA DECISIÓN DE ALCANCE SE TOMÓ EL 2026-09-11: UN REGISTRO POR TRACK.
	//
	// Antes esto tenía un TECHO —«no más de dos carpetas sin registro»— porque la pregunta estaba
	// pendiente y lo único que se podía hacer era avisar si se acumulaban. Con la decisión tomada,
	// el techo sobra y la guarda cambia de forma: en vez de contar cuántas faltan, se le pide a
	// CADA UNA lo mismo. Eso es lo que la hace escalar — una carpeta nueva con cabos entra al
	// barrido sola, sin que nadie suba un número.
	//
	// SE PIDEN DOS COSAS Y NO UNA. Que el archivo exista es barato de satisfacer con un archivo
	// vacío; lo que cierra el hueco es que tenga AL MENOS tantas filas como cabos declara la
	// carpeta. Adoptar tres cabos con un registro de una fila deja dos sin dueño, que es
	// exactamente el estado que esto vino a terminar.
	filasDeRegistro := func(carpeta string) (int, bool) {
		crudo, err := os.ReadFile(filepath.Join(raiz, carpeta, "ABIERTO.md"))
		if err != nil {
			return 0, false
		}
		n := 0
		for _, linea := range strings.Split(string(crudo), "\n") {
			l := strings.TrimSpace(linea)
			// Una fila de datos: empieza con `|`, no es el separador `|---|`, y su primera celda
			// tiene contenido. El encabezado se descarta por el separador que lo sigue.
			if !strings.HasPrefix(l, "|") || strings.Contains(l, "---") {
				continue
			}
			celdas := strings.Split(strings.Trim(l, "|"), "|")
			if len(celdas) >= 3 && strings.TrimSpace(celdas[0]) != "" && !strings.EqualFold(strings.TrimSpace(celdas[0]), "#") {
				n++
			}
		}
		return n, true
	}

	for _, v := range vecinas {
		filas, hay := filasDeRegistro(v.carpeta)
		if !hay {
			t.Errorf("specs/%s declara %d cabo(s) en %s y NO tiene `ABIERTO.md`.\n"+
				"  La decisión de alcance es UN REGISTRO POR TRACK (2026-09-11): el de flota cubre "+
				"`specs/control-de-flota/` y no adopta a nadie más.\n"+
				"  Sin registro propio, esos cabos son lo que la primera línea de todo ABIERTO.md "+
				"prohíbe: algo abierto sin dueño.", v.carpeta, v.items, v.archivo)
			continue
		}
		if filas < v.items {
			t.Errorf("specs/%s/ABIERTO.md tiene %d fila(s) y la carpeta declara %d cabo(s) en %s.\n"+
				"  Un registro con menos filas que cabos deja a los que faltan sin dueño, que es el "+
				"estado que el registro vino a terminar. Un archivo que existe no alcanza: tiene que "+
				"NOMBRARLOS.", v.carpeta, filas, v.items, v.archivo)
		}
	}
}
