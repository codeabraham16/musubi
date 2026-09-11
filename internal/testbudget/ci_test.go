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
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:
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

		// -----------------------------------------------------------------------------------
		// RONDA 3. Los de arriba los cerró un parser que entendía UNA forma de publicar el
		// techo y UNA forma de apagar un paso. Estos cinco son el MISMO defecto escrito de otra
		// manera, y cada uno entra por una puerta que el parser no tenía.

		{
			// (A) `env:` de YAML a nivel WORKFLOW. `pasoYAML` no tenía el campo, así que el
			// bloque era invisible: pisa RACE_TIMEOUT sin tocar el paso que hace `source`.
			"pisar el techo con un env: a nivel workflow",
			"jobs:\n  test:",
			"env:\n  RACE_TIMEOUT: 20m\njobs:\n  test:",
		},
		{
			"pisar el techo con un env: a nivel job",
			"  test:\n    runs-on: ubuntu-latest\n",
			"  test:\n    runs-on: ubuntu-latest\n    env:\n      RACE_TIMEOUT: 20m\n",
		},
		{
			"pisar el techo con un env: a nivel paso",
			"      - name: Test (race + coverage)\n        shell: bash\n",
			"      - name: Test (race + coverage)\n        shell: bash\n        env:\n          RACE_TIMEOUT: 20m\n",
		},
		{
			// (B) El `source` sigue ahí, el `echo` sigue ahí, y en el medio hay UNA línea. Todo
			// lo que un chequeo de formas mira queda exactamente igual.
			"reasignar la variable entre el source y el echo que la publica",
			`          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			"          RACE_TIMEOUT=40m\n" + `          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
		},
		{
			"reasignar con export antes de publicar",
			`          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			"          export RACE_TIMEOUT=40m\n" + `          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
		},
		{
			// (C) El paso bueno queda intacto y el techo se pisa DESPUÉS, por una boca que no
			// pasa por ningún argv: el cuerpo de un heredoc.
			"publicar el techo por heredoc en un paso posterior",
			"      - name: Test (race + coverage)",
			"      - name: Ajustes\n        shell: bash\n        run: |\n          cat >> \"$GITHUB_ENV\" <<EOF\n          RACE_TIMEOUT=40m\n          EOF\n      - name: Test (race + coverage)",
		},
		{
			"publicar el techo con tee -a en vez de >>",
			"      - name: Test (race + coverage)",
			"      - name: Ajustes\n        shell: bash\n        run: echo \"RACE_TIMEOUT=40m\" | tee -a \"$GITHUB_ENV\"\n      - name: Test (race + coverage)",
		},
		{
			// (D) El comando del guard sigue escrito, con su ruta y su `-salida` correctos. Lo
			// único que cambia es que su código de salida ya no llega a ningún lado.
			"neutralizar el guard con || true",
			"-salida race.txt\n",
			"-salida race.txt || true\n",
		},
		{
			"mandar el guard al fondo con &",
			"-salida race.txt\n",
			"-salida race.txt &\n",
		},
		{
			"perder el código del guard en una tubería sin pipefail",
			"        if: success()\n        shell: bash\n        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n",
			"        if: success()\n        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt | tee p.txt\n",
		},
		{
			"perder el código del guard con set +e y comandos después",
			"        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n",
			"        run: |\n          set +e\n          go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n          echo listo\n",
		},
		{
			// Un shell que este parser no sabe cómo trata un código de salida es «no pude
			// medir», y eso es rojo.
			"correr el guard con un shell que esta guarda no sabe leer",
			"        shell: bash\n        run: go run",
			"        shell: pwsh\n        run: go run",
		},
		{
			// (E) Apagar el paso sin escribir `false`: una condición que no se cumple en NINGÚN
			// evento de los que dispara este workflow.
			"apagar el guard con un if: que nunca es cierto",
			"        if: success()\n",
			"        if: github.event_name == 'schedule'\n",
		},
		{
			"apagar el guard sólo en los PR (no corre en todas las corridas que cubre)",
			"        if: success()\n",
			"        if: github.event_name == 'push'\n",
		},
		{
			"apagar el guard con un if: que esta guarda no puede evaluar",
			"        if: success()\n",
			"        if: contains(github.ref, 'no-existe')\n",
		},
		{
			// (5-residual) El paso que PRODUCE el dato no lo miraba nadie: un guard que corre
			// sobre un log que nadie escribió no mide nada.
			"apagar el paso que produce el log con continue-on-error",
			"      - name: Test (race + coverage)\n        shell: bash\n",
			"      - name: Test (race + coverage)\n        continue-on-error: true\n        shell: bash\n",
		},
		{
			"apagar el paso que produce el log con un if: que nunca es cierto",
			"      - name: Test (race + coverage)\n        shell: bash\n",
			"      - name: Test (race + coverage)\n        if: github.event_name == 'schedule'\n        shell: bash\n",
		},
		{
			// Y un nivel más arriba: el JOB entero.
			"apagar el job entero con continue-on-error",
			"  test:\n    runs-on: ubuntu-latest\n",
			"  test:\n    continue-on-error: true\n    runs-on: ubuntu-latest\n",
		},
		{
			"apagar el job entero con un if: que nunca es cierto",
			"  test:\n    runs-on: ubuntu-latest\n",
			"  test:\n    if: github.event_name == 'schedule'\n    runs-on: ubuntu-latest\n",
		},
		{
			// El reverso del printf legítimo: el formato es el mismo, el número está en el
			// argumento. Se resuelve el verbo, así que el número tipeado sale igual a la luz.
			"publicar el techo con printf y el número en el argumento",
			`echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			`printf 'RACE_TIMEOUT=%s\n' 40m >> "$GITHUB_ENV"`,
		},
		{
			// Comillas simples: el shell NO expande, así que lo que viaja al entorno del job es
			// el texto `$RACE_TIMEOUT` y `go test` cae en su default.
			"publicar el techo entre comillas simples (no expande)",
			`echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			`echo 'RACE_TIMEOUT=$RACE_TIMEOUT' >> "$GITHUB_ENV"`,
		},
		{
			// El `source` sigue escrito, pero antes de publicar se lee OTRO archivo que puede
			// dejar cualquier cosa en la variable: eso es «no pude derivar», que es rojo.
			"colar un source de otro archivo entre la política y el echo",
			`          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			"          source ./otro.env\n" + `          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
		},
		{
			// El contraste genérico: el techo nombrado en una construcción que este parser no
			// lee (el `with:` de una action). No hace falta saber qué hace para saber que no se
			// puede afirmar nada sobre el techo.
			"pasar el techo por el with: de una action que este parser no entiende",
			"      - name: Test (race + coverage)",
			"      - name: Ajustes\n        uses: alguien/pisar-env@v1\n        with:\n          RACE_TIMEOUT: 40m\n      - name: Test (race + coverage)",
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

// UN ROJO FALSO ES PEOR QUE UNA FUGA.
//
// Todo lo de arriba se cierra haciendo que las construcciones que este parser NO entiende sean
// rojas. Esa regla tiene un costo obvio y es el que hay que acotar: una guarda que se pone roja
// sobre trabajo sano la apaga el próximo que la cruce, y entonces no protege de nada.
//
// Así que acá está el reverso de la tabla de sabotajes: cambios LEGÍTIMOS en la misma zona, que
// tienen que seguir VERDES. Si mañana alguien endurece un juez de más, esta tabla se pone roja
// nombrando qué trabajo honesto acaba de romper.
func TestLosCambiosLegitimosSiguenVerdes(t *testing.T) {
	casos := []struct {
		nombre string
		viejo  string
		nuevo  string
	}{
		{
			// `shell: bash` en GitHub es `bash --noprofile --norc -eo pipefail {0}`: con
			// pipefail el código del guard SÍ sobrevive a la tubería. Que esto siga verde es lo
			// que separa «deriva el estado del shell» de «prohíbe la tubería».
			"guardar la salida del guard con tee, bajo shell: bash (hay pipefail)",
			"run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n",
			"run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt | tee presupuesto.txt\n",
		},
		{
			// El patrón que este repo ya usa en el paso de la suite, aplicado al guard.
			"capturar el código del guard con rc=$? y propagarlo con exit",
			"        run: go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n",
			"        run: |\n          set +e\n          go run ./internal/testbudget/cmd/presupuesto -salida race.txt\n          rc=$?\n          set -e\n          exit \"$rc\"\n",
		},
		{
			"propagar el rojo del guard con || exit 1",
			"-salida race.txt\n",
			"-salida race.txt || exit 1\n",
		},
		{
			// Un `env:` que NO toca el techo (el ci.yml real ya tiene uno así en recall-gate).
			"un env: de YAML que declara otra variable",
			"      - name: Test (race + coverage)\n        shell: bash\n",
			"      - name: Test (race + coverage)\n        shell: bash\n        env:\n          MUSUBI_POTION_DIR: /tmp/potion\n",
		},
		{
			// Un heredoc que no publica el techo: el parser tiene que sacarlo del camino sin
			// inventar nada.
			"un heredoc que escribe cualquier otra cosa",
			"      - name: Test (race + coverage)",
			"      - name: Nota\n        shell: bash\n        run: |\n          cat >> \"$GITHUB_STEP_SUMMARY\" <<EOF\n          corrida con techo $RACE_TIMEOUT\n          EOF\n      - name: Test (race + coverage)",
		},
		{
			// Un `if:` compuesto que es cierto en TODOS los eventos que dispara el workflow.
			"un if: compuesto que sí se puede demostrar cierto",
			"        if: success()\n",
			"        if: success() && github.event_name != 'schedule'\n",
		},
		{
			"un if: con ${{ }} alrededor",
			"        if: success()\n",
			"        if: ${{ success() }}\n",
		},
		{
			"un if: que enumera los eventos del workflow con ||",
			"        if: success()\n",
			"        if: github.event_name == 'push' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch'\n",
		},
		{
			"un if: always() en el guard",
			"        if: success()\n",
			"        if: always()\n",
		},
		{
			"renombrar el archivo donde se guarda la salida de la corrida",
			"race.txt",
			"salida-de-la-corrida.txt",
		},
		{
			"renombrar el paso del guard (el juicio no se ancla al nombre)",
			"- name: Presupuesto de pruebas (margen medido)",
			"- name: Verificar el margen del presupuesto",
		},
		{
			"agregar flags a la corrida bajo -race",
			"go test -race -count=1 -timeout",
			"go test -race -count=1 -shuffle=on -timeout",
		},
		{
			"tomar el techo con ${{ env.RACE_TIMEOUT }} en vez de \"$RACE_TIMEOUT\"",
			`-timeout "$RACE_TIMEOUT"`,
			"-timeout ${{ env.RACE_TIMEOUT }}",
		},
		{
			"meter un paso nuevo entre la corrida y el guard",
			"      - name: Presupuesto de pruebas (margen medido)",
			"      - name: Coverage summary\n        run: go tool cover -func=coverage.out\n      - name: Presupuesto de pruebas (margen medido)",
		},
		{
			"un paso nuevo con continue-on-error que no es ni el guard ni la corrida",
			"      - name: Presupuesto de pruebas (margen medido)",
			"      - name: Aviso de vulnerabilidades\n        continue-on-error: true\n        run: govulncheck ./...\n      - name: Presupuesto de pruebas (margen medido)",
		},
		{
			"un job nuevo entero que no toca los paquetes caros",
			"  test-cross:",
			"  lint:\n    runs-on: ubuntu-latest\n    if: github.event_name == 'pull_request'\n    continue-on-error: true\n    env:\n      GOFLAGS: -mod=readonly\n    steps:\n      - name: golangci-lint\n        run: golangci-lint run ./...\n  test-cross:",
		},
		{
			// El caso más exigente: un job NUEVO que corre los paquetes caros con el aparato
			// entero. Si un juez se pasa de duro, se rompe acá y no en producción.
			"un job nuevo que corre los caros con el aparato entero",
			"  test-cross:",
			`  test-nocturno:
    runs-on: ubuntu-latest
    steps:
      - name: Presupuesto de pruebas (política)
        shell: bash
        run: |
          set -euo pipefail
          source ./presupuesto-de-pruebas.env
          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"
      - name: Suite bajo -race
        shell: bash
        run: |
          set +e
          go test -race -count=1 -timeout "$RACE_TIMEOUT" ./... 2>&1 | tee nocturno.txt
          rc=${PIPESTATUS[0]}
          set -e
          exit "$rc"
      - name: Margen del presupuesto
        if: success()
        shell: bash
        run: go run ./internal/testbudget/cmd/presupuesto -salida nocturno.txt
  test-cross:`,
		},
		{
			"publicar el techo con >> $GITHUB_ENV sin comillas",
			`echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			"echo RACE_TIMEOUT=$RACE_TIMEOUT >> $GITHUB_ENV",
		},
		{
			"publicar el techo con printf",
			`echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
			`printf 'RACE_TIMEOUT=%s\n' "$RACE_TIMEOUT" >> "$GITHUB_ENV"`,
		},
		{
			"leer la política con `.` en vez de `source`",
			"source ./presupuesto-de-pruebas.env",
			". ./presupuesto-de-pruebas.env",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if !strings.Contains(ciBase, c.viejo) {
				t.Fatalf("el cambio no se pudo aplicar: el workflow base no contiene %q", c.viejo)
			}
			// ReplaceAll y no Replace: un cambio legítimo se aplica en TODOS los lugares donde
			// corresponde (renombrar el log lo toca en el `tee` y en el `-salida`), y aplicarlo a
			// medias inventaría un workflow incoherente que ninguna guarda tiene por qué aceptar.
			sano := strings.ReplaceAll(ciBase, c.viejo, c.nuevo)
			if errs := juzgarTodo(t, sano); len(errs) != 0 {
				t.Errorf("ROJO FALSO sobre un cambio legítimo. Una guarda que se pone roja sobre "+
					"trabajo sano la apaga el próximo que la cruce.\n--- workflow ---\n%s", sano)
				for _, e := range errs {
					t.Errorf("  %v", e)
				}
			}
		})
	}
}

// El ci.yml REAL del repo tiene que pasar por los mismos jueces con los que se juzgan los
// sintéticos: es el caso de rojo falso que más caro sale.
func TestElCiRealPasaLosJuecesNuevos(t *testing.T) {
	f, modulo, caros := flujoDelRepo(t)
	for _, err := range ErroresDePublicacion(f) {
		t.Error(err)
	}
	for _, err := range ErroresDelPasoDePresupuesto(f, modulo, caros) {
		t.Error(err)
	}
	if len(f.Eventos) == 0 {
		t.Errorf("%s no declaró ni un evento en `on:`: sin eventos no se puede demostrar que un "+
			"`if:` sea cierto, y todos los pasos condicionales quedarían en «no pude medir»", rutaCI)
	}
	t.Logf("eventos: %v · env: declarados: %d", f.Eventos, len(f.Envs))
}

// Y el ci.yml REAL con un job nuevo legítimo pegado encima. Es el rojo falso que más caro sale
// después del anterior: alguien agrega un job honesto y la guarda lo frena.
func TestElCiRealMasUnJobNuevoLegitimoSigueVerde(t *testing.T) {
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
	b, err := os.ReadFile(filepath.Join(raiz, rutaCI))
	if err != nil {
		t.Fatal(err)
	}
	const jobNuevo = `
  test-nocturno:
    runs-on: ubuntu-latest
    env:
      MUSUBI_MODO: nocturno
    steps:
      - uses: actions/checkout@v5
      - name: Presupuesto de pruebas (política)
        shell: bash
        run: |
          set -euo pipefail
          source ./presupuesto-de-pruebas.env
          : "${RACE_TIMEOUT:?falta}"
          echo "RACE_TIMEOUT=$RACE_TIMEOUT" >> "$GITHUB_ENV"
      - name: Suite bajo -race
        shell: bash
        run: |
          set +e
          go test -race -count=1 -timeout "$RACE_TIMEOUT" ./... 2>&1 | tee nocturno.txt
          rc=${PIPESTATUS[0]}
          set -e
          exit "$rc"
      - name: Margen del presupuesto
        if: success() && github.event_name != 'schedule'
        shell: bash
        run: go run ./internal/testbudget/cmd/presupuesto -salida nocturno.txt
`
	f, err := ParsearCI(rutaCI, string(b)+jobNuevo)
	if err != nil {
		t.Fatal(err)
	}
	var errs []error
	errs = append(errs, ErroresDeTecho(f, modulo, dirs)...)
	errs = append(errs, ErroresDePublicacion(f)...)
	errs = append(errs, ErroresDelPasoDePresupuesto(f, modulo, dirs)...)
	for _, e := range errs {
		t.Errorf("ROJO FALSO sobre un job nuevo legítimo: %v", e)
	}
}

// AnalizarSi, sola, contra sus tres respuestas. Es el juez que más fácil se pasa de duro, así
// que sus dos lados están fijados acá y no sólo a través de un workflow.
func TestAnalizarSiDistingueCiertoDeApagadoYDeNoSe(t *testing.T) {
	eventos := []string{"push", "pull_request", "workflow_dispatch"}
	casos := []struct {
		expr   string
		quiero bool
	}{
		{"", true},
		{"success()", true},
		{"${{ success() }}", true},
		{"always()", true},
		{"!cancelled()", true},
		{"success() && github.event_name != 'schedule'", true},
		{"github.event_name == 'push' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch'", true},
		{"(success())", true},
		{"false", false},
		{"${{ false }}", false},
		{"cancelled()", false},
		{"github.event_name == 'schedule'", false},
		{"github.event_name == 'push'", false}, // no corre en los PR
		{"success() && github.event_name == 'schedule'", false},
		{"github.event_name != 'push'", false},
		{"contains(github.ref, 'main')", false}, // no sé evaluarlo: rojo, no verde
		{"github.actor == 'nadie'", false},
		{"inputs.correr", false},
	}
	for _, c := range casos {
		t.Run(c.expr, func(t *testing.T) {
			ok, motivo := AnalizarSi(c.expr, eventos)
			if ok != c.quiero {
				t.Fatalf("AnalizarSi(%q) = %v (%s), quiero %v", c.expr, ok, motivo, c.quiero)
			}
			if !ok {
				t.Logf("rojo (correcto): %s", motivo)
			}
		})
	}
	// Sin eventos no se puede afirmar nada de una condición que depende del evento.
	if ok, _ := AnalizarSi("github.event_name == 'push'", nil); ok {
		t.Error("sin `on:` no se puede demostrar que un `if:` por evento sea cierto, y dio verde")
	}
	if ok, motivo := AnalizarSi("success()", nil); !ok {
		t.Errorf("un `if:` que no depende del evento tiene que poder demostrarse sin `on:`: %s", motivo)
	}
}

// El heredoc se saca del camino ANTES de lexear: su cuerpo son datos, no comandos.
func TestElCuerpoDelHeredocNoSeLexeaComoComandos(t *testing.T) {
	ts := lexearShell("cat >> \"$GITHUB_ENV\" <<EOF\nRACE_TIMEOUT=40m\nls -la\nEOF\necho fin\n")
	if len(ts) != 2 {
		t.Fatalf("se vieron %d tuberías, quiero 2 (el `cat` y el `echo`): el cuerpo del heredoc "+
			"se está lexeando como comandos", len(ts))
	}
	if len(ts[0].Cmds[0].Heredocs) != 1 {
		t.Fatalf("el `cat` no quedó con su heredoc colgado: %#v", ts[0].Cmds[0])
	}
	if !strings.Contains(ts[0].Cmds[0].Heredocs[0], "RACE_TIMEOUT=40m") {
		t.Errorf("el cuerpo no trae lo que se escribió: %q", ts[0].Cmds[0].Heredocs[0])
	}
	if ts[1].Cmds[0].Argv[0] != "echo" {
		t.Errorf("la tubería de después del heredoc es %v, quiero el `echo`", ts[1].Cmds[0].Argv)
	}
}

// El separador con que termina cada tubería es parte del parseo: sin él `cmd || true` y `cmd`
// son indistinguibles, y son lo contrario en lo único que importa.
func TestElSeparadorDeLaTuberiaSeConserva(t *testing.T) {
	ts := lexearShell("a || b && c ; d | e\nf &\ng")
	quiero := []string{"||", "&&", ";", "\n", "&", ""}
	if len(ts) != len(quiero) {
		t.Fatalf("se vieron %d tuberías, quiero %d: %+v", len(ts), len(quiero), ts)
	}
	for i, q := range quiero {
		if ts[i].Separador != q {
			t.Errorf("tubería %d (%s): separador %q, quiero %q", i, ts[i].Texto, ts[i].Separador, q)
		}
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
