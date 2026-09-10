package testbudget

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rutaCI es el workflow que gobierna el presupuesto de la suite.
//
// Y la excepción, escrita: los benchmarks de escala (bench-scale.yml) tienen su propio techo
// porque miden otra cosa; por eso las guardas se anclan a ci.yml y no a todo .github.
const rutaCI = ".github/workflows/ci.yml"

func flujoDelRepo(t *testing.T) (*FlujoCI, string, []string) {
	t.Helper()
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	modulo, err := ModuloDelRepo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	pol, err := CargarPoliticaDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	caros, err := PaquetesCaros(raiz, pol.UmbralLineasTest)
	if err != nil {
		t.Fatal(err)
	}
	if len(caros) == 0 {
		t.Fatalf("ningún paquete pasa UMBRAL_GUARDA_LINEAS_TEST=%d: la enumeración quedó vacía y "+
			"un conjunto vacío hace pasar TODAS las guardas que dependen de él", pol.UmbralLineasTest)
	}
	var dirs []string
	for _, c := range caros {
		dirs = append(dirs, c.Dir)
	}
	f, err := LeerCI(filepath.Join(raiz, rutaCI))
	if err != nil {
		t.Fatal(err)
	}
	f.Ruta = rutaCI
	return f, modulo, dirs
}

// EL TECHO NO SE PUEDE VOLVER A TIPEAR, NI SACAR, EN EL ci.yml.
//
// El defecto de fondo del cabo no fue el número: fue que el número vivía tipeado en dos comandos
// y justificado en prosa al lado, así que envejeció tres veces sin quien lo contradijera. Esta
// guarda cierra el camino de vuelta por sus DOS puertas —retipearlo y sacarlo— sobre el argv
// parseado, no sobre el texto de la línea.
func TestElCiTomaElTechoDeLaPolitica(t *testing.T) {
	f, modulo, caros := flujoDelRepo(t)
	for _, err := range ErroresDeTecho(f, modulo, caros) {
		t.Error(err)
	}
}

// El nivel de adentro: lo que se publica en $GITHUB_ENV tiene que salir del archivo.
func TestElCiPublicaElTechoQueLee(t *testing.T) {
	f, _, _ := flujoDelRepo(t)
	for _, err := range ErroresDePublicacion(f) {
		t.Error(err)
	}
}

// Y que el aparato CORRA: el paso que juzga el margen tiene que existir, encadenado al log de
// la corrida y sin estar desactivado.
func TestElCiEjecutaElGuardDelMargen(t *testing.T) {
	f, modulo, caros := flujoDelRepo(t)
	for _, err := range ErroresDelPasoDePresupuesto(f, modulo, caros) {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------------------------
// LOS SABOTAJES, UNO POR UNO.
//
// Doce sabotajes quedaron verdes contra la primera versión de esta guarda. Acá están, aplicados
// a un workflow sintético con la misma forma que el real: si mañana alguien afloja un juez, esta
// tabla se pone roja nombrando cuál.

const ciBase = `
name: CI
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Presupuesto de pruebas (política)
        shell: bash
        run: |
          set -euo pipefail
          source ./presupuesto-de-pruebas.env
          : "${RACE_TIMEOUT:?falta}"
          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"
      - name: Test (race + coverage)
        shell: bash
        run: |
          set +e
          go test -race -count=1 -timeout "$RACE_TIMEOUT" -coverprofile=coverage.out ./... 2>&1 | tee race.txt
          rc=${PIPESTATUS[0]}
          set -e
          exit "$rc"
      - name: Presupuesto de pruebas (margen medido)
        if: success()
        shell: bash
        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt
  test-cross:
    runs-on: windows-latest
    steps:
      - name: Presupuesto de pruebas (política)
        shell: bash
        run: |
          source ./presupuesto-de-pruebas.env
          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"
      - name: Test
        run: go test -timeout ${{ env.RACE_TIMEOUT }} ./...
`

func juzgarTodo(t *testing.T, yaml string) []error {
	t.Helper()
	f, err := ParsearCI("ci-sintetico.yml", yaml)
	if err != nil {
		// Un YAML que no parsea también es rojo: es «no pude mirar».
		return []error{err}
	}
	caros := []string{"internal/mcp", "internal/memory"}
	var errs []error
	errs = append(errs, ErroresDeTecho(f, "musubi", caros)...)
	errs = append(errs, ErroresDePublicacion(f)...)
	errs = append(errs, ErroresDelPasoDePresupuesto(f, "musubi", caros)...)
	return errs
}

// El workflow sano no puede dar errores: si diera, la tabla de abajo estaría midiendo ruido.
func TestElCiSinteticoSanoPasa(t *testing.T) {
	if errs := juzgarTodo(t, ciBase); len(errs) != 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestCadaSabotajeDelCiCae(t *testing.T) {
	casos := []struct {
		nombre string
		viejo  string
		nuevo  string
	}{
		{
			// (1) El hermano declarado cubierto: test-cross, escrito EN LÍNEA. La primera versión
			// exigía que `go test` arrancara renglón o viniera tras un separador, así que la
			// forma inline no era ni candidata.
			"retipear el techo en la forma inline de test-cross",
			"run: go test -timeout ${{ env.RACE_TIMEOUT }} ./...",
			"run: go test -timeout 40m ./...",
		},
		{
			"retipear con dos espacios antes del valor",
			`-timeout "$RACE_TIMEOUT"`,
			"-timeout  40m",
		},
		{
			"retipear entre comillas simples",
			`-timeout "$RACE_TIMEOUT"`,
			"-timeout '40m'",
		},
		{
			// Comillas simples alrededor de la variable: el shell NO la expande, así que el
			// `-timeout` llega literal y `go test` se muere. Verde para cualquier grep.
			"la variable entre comillas simples no expande",
			`-timeout "$RACE_TIMEOUT"`,
			"-timeout '$RACE_TIMEOUT'",
		},
		{
			// (3) El defecto en su forma más pura: sacar el flag entero.
			"sacar el -timeout del job test",
			`-timeout "$RACE_TIMEOUT" `,
			"",
		},
		{
			"sacar el -timeout del job test-cross",
			"-timeout ${{ env.RACE_TIMEOUT }} ",
			"",
		},
		{
			// (4) El número un nivel más adentro: el `source` queda intacto y se publica a mano.
			"publicar el número a mano dejando el source intacto",
			`echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			`echo "RACE_TIMEOUT=40m" >> "$GITHUB_ENV"`,
		},
		{
			"publicar sin leer el archivo de política",
			"source ./presupuesto-de-pruebas.env\n          : \"${RACE_TIMEOUT:?falta}\"\n",
			"",
		},
		{
			// (5) Vaciarlo: el paso entero desaparece y el aparato sigue en el repo sin correr.
			"borrar el paso que juzga el margen",
			"      - name: Presupuesto de pruebas (margen medido)\n        if: success()\n        shell: bash\n        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n",
			"",
		},
		{
			"apagar el paso del margen con continue-on-error",
			"        if: success()\n",
			"        continue-on-error: true\n",
		},
		{
			"apagar el paso del margen con if: false",
			"        if: success()\n",
			"        if: false\n",
		},
		{
			"que el guard juzgue un archivo que no es el de la corrida",
			"-salida race.txt",
			"-salida otro.txt",
		},
		{
			"dejar de guardar la salida de la corrida",
			" 2>&1 | tee race.txt",
			"",
		},
		{
			"que la suite deje de correr bajo -race",
			"go test -race -count=1",
			"go test -count=1",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if !strings.Contains(ciBase, c.viejo) {
				t.Fatalf("el sabotaje no se pudo aplicar: el workflow base no contiene %q "+
					"(esta prueba estaría midiendo nada)", c.viejo)
			}
			roto := strings.Replace(ciBase, c.viejo, c.nuevo, 1)
			errs := juzgarTodo(t, roto)
			if len(errs) == 0 {
				t.Fatalf("el sabotaje quedó VERDE.\n--- workflow saboteado ---\n%s", roto)
			}
			for _, e := range errs {
				t.Logf("rojo (correcto): %v", e)
			}
		})
	}
}

// ---------------------------------------------------------------------------------------------
// EL LÉXICO: la misma orden escrita de siete maneras tiene que dar el mismo argv.
//
// Esto es lo que reemplaza a la lista de formas malas. Mientras estas formas converjan, no hace
// falta enumerar cómo se puede escribir un número.
func TestLasFormasDeEscribirloConvergen(t *testing.T) {
	formas := []struct {
		nombre string
		run    string
	}{
		{"bloque", "run: |\n          go test -timeout 40m ./...\n"},
		{"en línea", "run: go test -timeout 40m ./...\n"},
		{"con =", "run: go test -timeout=40m ./...\n"},
		{"dos espacios", "run: go test -timeout  40m ./...\n"},
		{"comillas simples", "run: go test -timeout '40m' ./...\n"},
		{"comillas dobles", `run: go test -timeout "40m" ./...` + "\n"},
		{"doble guion", "run: go test --timeout 40m ./...\n"},
		{"plegado", "run: >\n          go test -timeout 40m ./...\n"},
		{"después de un &&", "run: cd . && go test -timeout 40m ./...\n"},
		{"al final de una tubería", "run: echo hola | grep -q hola; go test -timeout 40m ./...\n"},
	}
	for _, f := range formas {
		t.Run(f.nombre, func(t *testing.T) {
			yml := "jobs:\n  x:\n    steps:\n      - name: t\n        " + f.run
			flujo, err := ParsearCI("x.yml", yml)
			if err != nil {
				t.Fatal(err)
			}
			gs := ComandosGoTest(flujo)
			if len(gs) != 1 {
				t.Fatalf("se vieron %d comandos `go test`, quiero 1", len(gs))
			}
			clase, valor := gs[0].Techo()
			if clase != TechoTipeado {
				t.Errorf("Techo() = %v (%q), quiero tipeado: esta forma se escapa", clase, valor)
			}
			if !gs[0].Cubre("musubi", "internal/mcp") {
				t.Errorf("Paquetes = %v, `./...` tiene que cubrir internal/mcp", gs[0].Paquetes)
			}
		})
	}
}

// Y el reverso: las formas BUENAS de tomar el techo tienen que reconocerse todas.
func TestLasFormasDeDerivarloSeReconocen(t *testing.T) {
	buenas := []string{
		`-timeout "$RACE_TIMEOUT"`,
		"-timeout $RACE_TIMEOUT",
		"-timeout ${RACE_TIMEOUT}",
		"-timeout ${{ env.RACE_TIMEOUT }}",
		"-timeout ${{env.RACE_TIMEOUT}}",
		`-timeout="$RACE_TIMEOUT"`,
	}
	for _, b := range buenas {
		t.Run(b, func(t *testing.T) {
			yml := "jobs:\n  x:\n    steps:\n      - name: t\n        run: go test " + b + " ./...\n"
			flujo, err := ParsearCI("x.yml", yml)
			if err != nil {
				t.Fatal(err)
			}
			gs := ComandosGoTest(flujo)
			if len(gs) != 1 {
				t.Fatalf("se vieron %d comandos `go test`, quiero 1", len(gs))
			}
			if clase, valor := gs[0].Techo(); clase != TechoDerivado {
				t.Errorf("Techo() = %v (%q), quiero derivado", clase, valor)
			}
		})
	}
}

// UN LÉXICO QUE DEJA DE VER COMANDOS NO PUEDE DAR VERDE. Es el mismo modo de falla que la
// guarda vieja, un nivel más adentro: no encontrar nada que objetar porque no se miró.
func TestUnComandoQueElLexicoNoVeEsError(t *testing.T) {
	// Un `go test` metido en un lugar que el parser de pasos no recorre (una clave que no es
	// `run:`): el contraste contra el texto crudo lo tiene que delatar.
	yml := "jobs:\n  x:\n    steps:\n      - name: t\n        run: go test -timeout $RACE_TIMEOUT ./...\n" +
		"      - name: raro\n        uses: algo@v1\n        with:\n          cmd: go test ./...\n"
	if _, err := ParsearCI("x.yml", yml); err == nil {
		t.Fatal("el parser vio menos `go test` que el texto y NO se quejó")
	}
}

// Un ci.yml vacío, renombrado o reescrito no puede pasar por «no encontré nada malo».
func TestUnCiVacioEsError(t *testing.T) {
	if _, err := ParsearCI("x.yml", "name: CI\non: push\n"); err == nil {
		t.Fatal("un workflow sin jobs pasó como bueno")
	}
	f, err := ParsearCI("x.yml", "jobs:\n  x:\n    steps:\n      - name: t\n        run: echo hola\n")
	if err != nil {
		t.Fatal(err)
	}
	if errs := ErroresDeTecho(f, "musubi", []string{"internal/mcp"}); len(errs) == 0 {
		t.Fatal("un workflow sin ni un `go test` pasó como bueno")
	}
}

// EL HERMANO DEL ARCHIVO: los OTROS workflows.
//
// Las tres guardas de arriba se anclan a ci.yml, que es donde el número estaba tipeado. Pero
// .github/workflows tiene más archivos, y nada impedía que mañana apareciera en el de al lado
// otro `go test ./...` con su propio techo a mano. Esto recorre TODOS y exige lo mínimo que
// vale en cualquiera: el que corre los tests de un paquete caro toma el techo de la política.
//
// bench-scale.yml sigue teniendo su propio techo y está bien que lo tenga: sus comandos son
// `-run=NONE -bench=…`, no corren los tests del paquete y miden otra cosa. La excepción no está
// escrita como una lista de archivos perdonados: sale de la forma del comando.
func TestNingunWorkflowCorreLosPaquetesCarosSinElTecho(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	modulo, err := ModuloDelRepo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	pol, err := CargarPoliticaDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	caros, err := PaquetesCaros(raiz, pol.UmbralLineasTest)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, c := range caros {
		dirs = append(dirs, c.Dir)
	}

	dir := filepath.Join(raiz, ".github", "workflows")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var mirados int
	for _, e := range entradas {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		rel := filepath.Join(".github", "workflows", e.Name())
		f, err := LeerCI(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		f.Ruta = rel
		mirados++
		for _, err := range ErroresDeTechoEnPaquetesCaros(f, modulo, dirs) {
			t.Error(err)
		}
	}
	// CERO WORKFLOWS MIRADOS ES «NO PUDE MIRAR».
	if mirados == 0 {
		t.Fatalf("no se miró ni un workflow en %s", dir)
	}
	t.Logf("workflows mirados: %d · paquetes caros: %v", mirados, dirs)
}
