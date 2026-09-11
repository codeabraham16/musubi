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
	var nombres []string
	for _, v := range vecinas {
		total += v.items
		nombres = append(nombres, v.carpeta+"/"+v.archivo)
		t.Logf("cabo vecino SIN REGISTRO: specs/%s/%s — %d ítem(s)", v.carpeta, v.archivo, v.items)
	}
	t.Logf("%d carpeta(s) vecina(s) con cabos declarados fuera de alcance y sin registro propio, %d ítem(s) en total",
		len(vecinas), total)

	// EL TECHO. Hoy son dos carpetas; que aparezca una tercera cambia la pregunta —de «dos
	// pendientes de una decisión» a «esto se acumula»— y eso sí merece parar a alguien.
	if len(vecinas) > 2 {
		t.Errorf("hay %d carpetas vecinas con cabos sin registro (%v), y eran 2.\n"+
			"  Dónde vive su registro es una decisión de alcance —uno por track, o uno solo para "+
			"todo— y mientras no se tome, cada carpeta nueva es otro conjunto de cabos sin dueño.\n"+
			"  La primera línea de ABIERTO.md dice «nada queda abierto sin dueño»; esto lo contradice "+
			"una carpeta a la vez.", len(vecinas), nombres)
	}
}
