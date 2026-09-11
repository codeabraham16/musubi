package testbudget

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// UNA COMPUERTA DE CI QUE NOMBRA UN TEST SE APAGA SOLA CUANDO ALGUIEN LO RENOMBRA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `go test -run NombreQueNoExiste` SALE 0. No es un error, no imprime nada raro: informa que
// corrió cero pruebas y devuelve éxito. Lo mismo `-bench=NombreQueNoExiste`. Así que una compuerta
// escrita como
//
//	go test ./internal/recalleval/ -run TestSemanticVsLexicalReal
//
// deja de medir en el instante en que alguien renombra esa función —un refactor de rutina, sin
// mala intención— y CI sigue en verde. La compuerta no se rompe: DESAPARECE, que es peor, porque
// el verde sigue estando y ya no dice nada.
//
// NO ES UNA COMPUERTA, SON CINCO, y por eso esto es una guarda de la clase y no un arreglo suelto:
// `recall-gate` nombra un test, y los cuatro jobs de benchmark de `ci.yml` y `bench-scale.yml`
// nombran `BenchmarkMaintain` y `BenchmarkSearchVector`. Cualquiera de los cinco nombres puede
// quedar huérfano sin que nada avise.
//
// EL REPO YA PAGÓ ESTA LECCIÓN EN SU OTRA FORMA: cuatro pruebas en un `*_windows_test.go` no
// compilaban fuera de Windows, `go test` contestaba `ok`, y las pruebas simplemente no existían.
// Es el mismo defecto —una ausencia que se lee como un éxito— movido del nombre del archivo al
// nombre de la función.
// ────────────────────────────────────────────────────────────────────────────────────────────

// El nombre puede venir pegado con `=` o separado por espacio, y entre comillas o sin ellas.
var (
	banderaRun   = regexp.MustCompile(`-run[= ]'?"?([A-Za-z_][A-Za-z0-9_]*)'?"?`)
	banderaBench = regexp.MustCompile(`-bench[= ]'?"?([A-Za-z_][A-Za-z0-9_]*)'?"?`)
	paqueteEnCmd = regexp.MustCompile(`\./([A-Za-z0-9_./-]+?)/?\s`)
)

func TestCadaCompuertaDeCINombraUnTestQueExiste(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	dirFlujos := filepath.Join(raiz, ".github", "workflows")
	entradas, err := os.ReadDir(dirFlujos)
	if err != nil {
		t.Fatalf("no pude listar %s: %v", dirFlujos, err)
	}

	compuertas := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		crudo, err := os.ReadFile(filepath.Join(dirFlujos, e.Name()))
		if err != nil {
			t.Fatalf("no pude leer %s: %v", e.Name(), err)
		}
		for n, linea := range strings.Split(string(crudo), "\n") {
			if !strings.Contains(linea, "go test") {
				continue
			}
			var nombres []string
			for _, m := range banderaRun.FindAllStringSubmatch(linea, -1) {
				// `-run=NONE` es deliberado: significa «ninguna prueba, sólo benchmarks».
				if m[1] != "NONE" {
					nombres = append(nombres, m[1])
				}
			}
			for _, m := range banderaBench.FindAllStringSubmatch(linea, -1) {
				nombres = append(nombres, m[1])
			}
			if len(nombres) == 0 {
				continue
			}
			mp := paqueteEnCmd.FindStringSubmatch(linea)
			if mp == nil {
				t.Errorf("%s:%d nombra %v pero no pude ver sobre qué paquete corre; enseñale a esta "+
					"guarda a leer esa forma antes de que el nombre quede huérfano sin que nadie mire.\n  %s",
					e.Name(), n+1, nombres, strings.TrimSpace(linea))
				continue
			}
			dirPaquete := filepath.Join(raiz, filepath.FromSlash(mp[1]))
			for _, nombre := range nombres {
				compuertas++
				if existeFuncionDeTest(t, dirPaquete, nombre) {
					continue
				}
				t.Errorf("%s:%d corre `%s` sobre ./%s y esa función NO EXISTE ahí.\n"+
					"  `go test` con un nombre que no matchea sale 0: esta compuerta está en VERDE\n"+
					"  sin medir nada, y va a seguir así hasta que alguien lea el yaml.\n"+
					"  Si la renombraste, actualizá el workflow; si la borraste, borrá el job.",
					e.Name(), n+1, nombre, mp[1])
			}
		}
	}

	// EL CONTROL. Sin esto, el día que los workflows cambien de forma —otra sintaxis, otro
	// directorio— esta prueba pasaría en verde sin haber encontrado una sola compuerta, y ese
	// verde diría «están todas bien» en vez de «no encontré ninguna». Es exactamente el defecto
	// que persigue, una vuelta más adentro.
	if compuertas < 5 {
		t.Fatalf("sólo encontré %d compuertas que nombran un test o un benchmark, y son al menos 5 "+
			"(recall-gate más los cuatro jobs de benchmark): cambió la forma de los workflows y esta "+
			"guarda está en verde sin mirar nada", compuertas)
	}
}

// existeFuncionDeTest busca `func <nombre>(` en los `_test.go` del paquete.
//
// Se mira el archivo y no se usa `go list`: invocar al toolchain desde una prueba la ata a que
// haya red, módulos y caché disponibles, y esta guarda tiene que poder correr en el runner más
// pelado.
func existeFuncionDeTest(t *testing.T, dir, nombre string) bool {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	aguja := "func " + nombre + "("
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		crudo, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(crudo), aguja) {
			return true
		}
	}
	return false
}
