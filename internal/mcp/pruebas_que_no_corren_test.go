package mcp

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"musubi/internal/arbol"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// UNA PRUEBA QUE NO COMPILA NO FALLA: `go test` CONTESTA `ok` Y NADIE SE ENTERA
//
// EL CASO MEDIDO, 2026-09-05: otra sesión escribió cinco pruebas para el actualizador del agente y
// las guardó en `despliegue_agente_windows_test.go`. Un archivo terminado en `_windows_test.go`
// lleva restricción de build IMPLÍCITA por GOOS —sin ninguna línea `//go:build`— así que en Linux
// no compila. `go test` dijo `ok`. `go vet` pasó. Las cinco pruebas no existían, y sólo aparecieron
// preguntando explícitamente `go list -f '{{.IgnoredGoFiles}}'`.
//
// Y ES DOBLEMENTE TRAICIONERO al nombrar una prueba SOBRE algo de Windows, que es justo cuando uno
// quiere esa palabra en el nombre del archivo. Vale igual para `_linux`, `_darwin`, `_amd64`,
// `_arm64` y cualquier otro GOOS/GOARCH pegado antes del `_test.go`.
//
// «¿PASAN LAS PRUEBAS?» Y «¿EXISTEN LAS PRUEBAS?» SON PREGUNTAS DISTINTAS, y `ok` contesta las dos
// igual. Es la misma forma que este repo persigue todo el tiempo: una respuesta plausible a otra
// pregunta. El repo YA conocía la versión de esto para el código —`internal/fleet/tempwindows.go`
// existe con ese nombre y no `colector_windows.go` justamente para no quedar detrás del tag, y su
// comentario dice que era «la tercera vez»— y no tenía la versión para las PRUEBAS, que es peor:
// un símbolo que no compila lo denuncia el compilador en algún lado; una prueba que no compila no
// denuncia nada, porque su ausencia se ve idéntica a su éxito.
//
// LA REGLA ES UNA SOLA Y ES LA MÁS DIRECTA POSIBLE: ninguna prueba puede quedar excluida en
// linux/amd64 sin tags, que es donde corre CI. Cubre de un saque los dos caminos —el sufijo
// implícito del nombre y un `//go:build` explícito— porque los dos terminan en lo mismo: código de
// prueba que en CI no existe.
//
// Sabotajes verificados que la ponen en rojo: crear `algo_windows_test.go`, y agregarle
// `//go:build etiqueta_inventada` a un archivo de prueba cualquiera.
// arnes: prueba="TestNingunaPruebaQuedaFueraDeLaCompilacionSinDecirlo"
// arnes: archivo="internal/config/quota_default_test.go"
// arnes: de="package config\n\nimport \"testing\"\n"
// arnes: a="//go:build etiqueta_inventada\n\npackage config\n\nimport \"testing\"\n"

// pruebasExcluidasAProposito son las que NO corren en CI y está bien, cada una con su motivo.
// Un allowlist sin razones se llena solo; con razones, agregar una entrada obliga a defenderla.
var pruebasExcluidasAProposito = map[string]string{
	// Sale de la red y necesita una credencial del central. Su propia cabecera lo argumenta: es un
	// INSTRUMENTO y no una compuerta, y no falla por umbral a propósito — «una sonda que rompe el
	// build por eso se apaga sola a la semana». CI no puede depender de la red.
	"internal/mcp/sonda_diseno_test.go": "instrumento contra el central real; CI no la CORRE porque no debe depender de la red ni de una credencial, pero SÍ la compila (`go vet -tags sonda`)",

	// Éstas SÍ corren en CI, con su tag encendido, así que no son pruebas muertas: van acá porque
	// esta guarda evalúa sin tags. `-race` define el tag `race` solo; `treesitter` va explícito en
	// el job de codeintel/mcp (`go test -tags "$TAGS" ./internal/codeintel/ ./internal/mcp/`).
	"internal/mcp/detector_carreras_on_test.go":       "tag `race`, que lo define `go test -race` — CI lo corre en su job de carreras",
	"internal/mcp/methods_codegraph_polyglot_test.go": "tag `treesitter`, encendido explícitamente en el job de polyglot del CI",
	"internal/codeintel/treesit_test.go":              "tag `treesitter`, encendido explícitamente en el job de polyglot del CI",
	"internal/codeintel/red_del_parser_test.go":       "tag `treesitter`, encendido explícitamente en el job de polyglot del CI",
}

// Los GOOS y GOARCH que Go reconoce como sufijo de archivo. No hace falta que estén todos: alcanza
// con los que alguien podría escribir sin querer, y los cuatro primeros son los que importan acá.
var sufijosDePlataforma = []string{
	"windows", "linux", "darwin", "android", "ios", "js", "wasip1", "plan9", "aix", "dragonfly",
	"freebsd", "hurd", "illumos", "netbsd", "openbsd", "solaris", "zos",
	"amd64", "arm64", "386", "arm", "riscv64", "s390x", "wasm", "loong64",
	"mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le", "sparc64",
}

func TestNingunaPruebaQuedaFueraDeLaCompilacionSinDecirlo(t *testing.T) {
	raiz := filepath.Join("..", "..")
	// linux/amd64 sin tags: exactamente el `go test ./...` de CI.
	activa := func(tag string) bool {
		switch tag {
		case "linux", "amd64", "unix", "cgo", "gc":
			return true
		}
		// Las versiones de Go se cuentan como activas: `go1.21` y anteriores lo están en go1.26.
		return strings.HasPrefix(tag, "go1.")
	}

	revisados := 0
	var excluidos []string
	// EL ÁRBOL LO DICE GIT Y NO EL DIRECTORIO (A128). La lista de carpetas a saltear que había acá
	// no converge, y `.claude/worktrees/` son copias enteras del repo: esta guarda contaba sus
	// pruebas como propias y habría acusado restricciones de build de otras ramas.
	pruebas, err := arbol.ConSufijo(raiz, "_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range pruebas {
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		nombre := filepath.Base(rel)
		revisados++

		motivo := ""
		base := strings.TrimSuffix(nombre, "_test.go")
		for _, s := range sufijosDePlataforma {
			if strings.HasSuffix(base, "_"+s) && !activa(s) {
				motivo = "el nombre termina en `_" + s + "_test.go`, que es una restricción de build IMPLÍCITA por GOOS/GOARCH — sin ninguna línea `//go:build`"
				break
			}
		}
		if motivo == "" {
			b, e := os.ReadFile(ruta)
			if e != nil {
				t.Fatalf("no pude leer %s: %v", rel, e)
			}
			for _, linea := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(strings.TrimSpace(linea), "package ") {
					break // las restricciones van ANTES del package
				}
				if !constraint.IsGoBuild(linea) && !constraint.IsPlusBuild(linea) {
					continue
				}
				expr, e := constraint.Parse(linea)
				if e != nil {
					t.Errorf("%s: la línea de restricción no parsea (%v), así que Go la ignora y el "+
						"archivo compila cuando no debería, o al revés:\n    %s", rel, e, strings.TrimSpace(linea))
					continue
				}
				if !expr.Eval(activa) {
					motivo = "su restricción `" + strings.TrimSpace(linea) + "` no se cumple en linux/amd64 sin tags"
				}
			}
		}
		if motivo != "" {
			if _, ok := pruebasExcluidasAProposito[rel]; !ok {
				excluidos = append(excluidos, rel+" — "+motivo)
			}
		}
	}

	// CONTROL DE QUE MIRÓ ALGO: si el filtro se rompiera, la lista quedaría vacía y esto pasaría en
	// verde sin haber revisado un solo archivo. Es la falla que esta prueba entera persigue, así que
	// no puede tenerla ella.
	if revisados < 100 {
		t.Fatalf("se revisaron %d archivos `_test.go` y el repo tiene cientos: el recorrido dejó de "+
			"mirar y esta guarda está en verde sin haber comprobado nada", revisados)
	}

	sort.Strings(excluidos)
	for _, x := range excluidos {
		t.Errorf("esta prueba NO COMPILA en linux/amd64 sin tags, o sea que en CI no existe:\n"+
			"    %s\n"+
			"  `go test` va a contestar `ok` igual, y la ausencia se ve idéntica al éxito.\n"+
			"  Si el nombre fue un accidente —pasa al nombrar una prueba SOBRE algo de Windows—\n"+
			"  renombrala metiendo la plataforma en el medio (`agente_windows_test.go` →\n"+
			"  `agentewindows_test.go`), que es el mismo truco que usa internal/fleet/tempwindows.go.\n"+
			"  Si de verdad tiene que quedar afuera, sumala a `pruebasExcluidasAProposito` CON SU MOTIVO.", x)
	}

	// Y al revés: una exención sobre un archivo que ya compila —o que ya no existe— queda diciendo
	// que cuida algo, y la próxima persona la va a leer como vigente.
	for rel := range pruebasExcluidasAProposito {
		if _, e := os.Stat(filepath.Join(raiz, rel)); e != nil {
			t.Errorf("`pruebasExcluidasAProposito` exime a %s y ese archivo ya no está: sacá la entrada", rel)
		}
	}
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// UNA EXENCIÓN QUE DICE «LO CUBRE EL CI» ES PROSA HASTA QUE ALGUIEN LA MIDE
//
// El allowlist de arriba justifica tres entradas diciendo «tag `treesitter`, encendido
// explícitamente en el job de polyglot del CI». Eso es una AFIRMACIÓN SOBRE OTRO ARCHIVO escrita
// en un comentario: si ese job cambia sus tags o se va, el allowlist sigue diciendo que están
// cubiertas y las tres pruebas se vuelven invisibles de verdad, en silencio. Es exactamente el
// defecto que la guarda de arriba existe para impedir, alojado en su propia justificación.
//
// La sonda tenía la mitad complementaria: su motivo justifica no EJECUTARLA —sale a la red— y de
// ahí se seguía, sin decirlo, que tampoco se compilaba. Compilar no sale a la red.
//
// LOS TAGS DEL CI SE DERIVAN DEL CI, no se escriben acá: una lista a mano sería una segunda copia
// que se queda vieja, que es el mismo defecto una capa más arriba.
//
// Sabotajes verificados que la ponen en rojo: sacar el paso `go vet -tags sonda` del workflow, y
// sacarle `treesitter` al TAGS del job de polyglot.
// arnes: archivo=".github/workflows/ci.yml"
// arnes: de="        run: go vet -tags sonda ./internal/mcp/\n"
// arnes: a=""
func TestCadaExencionPorTagSeCompilaEnAlgunPasoDelCI(t *testing.T) {
	ci, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("no se pudo leer el workflow: %v", err)
	}
	texto := string(ci)

	// Los tags ENCENDIDOS en el CI, derivados de sus comandos. `-race` define el tag `race` solo.
	encendidos := map[string]bool{}
	// `-tags <literal>` y TAMBIEN `-tags "$VAR"`, siguiendo la asignacion de esa variable en el
	// mismo workflow. El job de polyglot arma su lista en `TAGS='treesitter ...'`, asi que una
	// derivacion que no sigue la variable acusa de descubiertas a tres pruebas que el CI SI
	// compila: medido, la primera version de esta guarda hizo exactamente eso.
	for _, m := range regexp.MustCompile(`-tags[= ]+["']?(\$\{?(\w+)\}?|[a-zA-Z0-9_ ]+)["']?`).FindAllStringSubmatch(texto, -1) {
		valor := m[1]
		if m[2] != "" {
			asig := regexp.MustCompile(m[2] + `=['"]([^'"]*)['"]`).FindStringSubmatch(texto)
			if asig == nil {
				continue // una variable que no se asigna en este archivo: no se puede derivar
			}
			valor = asig[1]
		}
		for _, tag := range strings.Fields(valor) {
			encendidos[tag] = true
		}
	}
	if regexp.MustCompile(`go test[^\n]*-race`).MatchString(texto) {
		encendidos["race"] = true
	}
	// CONTROL DE QUE MIDIÓ ALGO: si el patrón dejara de matchear, el mapa quedaría vacío y todas
	// las exenciones se verían descubiertas —o peor, con otra redacción, todas cubiertas—.
	if len(encendidos) < 2 {
		t.Fatalf("se derivaron %d tags del CI y el workflow usa varios: el patrón dejó de matchear "+
			"y esta guarda está midiendo el vacío", len(encendidos))
	}

	for rel := range pruebasExcluidasAProposito {
		b, err := os.ReadFile(filepath.Join("..", "..", rel))
		if err != nil {
			continue // la guarda de arriba ya denuncia las entradas que no existen
		}
		for _, linea := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(linea), "package ") {
				break
			}
			if !constraint.IsGoBuild(linea) {
				continue
			}
			// Los tags que este archivo necesita para compilar.
			for _, tag := range regexp.MustCompile(`[a-zA-Z0-9_]+`).FindAllString(
				strings.TrimPrefix(strings.TrimSpace(linea), "//go:build"), -1) {
				if encendidos[tag] {
					goto cubierto
				}
			}
			t.Errorf("%s está exento de correr en CI y NINGÚN paso del workflow lo compila con su "+
				"tag (`%s`): puede dejar de compilar y nada se va a poner rojo.\n"+
				"  No correrlo puede estar bien —la sonda sale a la red—, pero COMPILARLO no sale a\n"+
				"  la red: alcanza con un `go vet -tags <tag>` en el CI.",
				rel, strings.TrimSpace(linea))
		cubierto:
		}
	}
}
