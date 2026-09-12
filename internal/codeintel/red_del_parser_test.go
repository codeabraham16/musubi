//go:build treesitter

package codeintel

import (
	"testing"

	ts "github.com/odvcencio/gotreesitter"
)

// TestUnPanicoDelParserNoSeLlevaElProceso — la promesa del comentario, ahora medida.
//
// EL DEFECTO QUE CIERRA. `derivePolyglotFile` decía «degrada a vacío sin pánico» y no tenía un solo
// `recover()`. La cadena real es `derivePolyglotFile` → `DerivePackage` → `toolCodegraphIndex` →
// `reindexCodeGraphOnce` → `RunCodeGraphScheduler`, y esa última corre en una goroutine PELADA que
// larga `main` (`go server.RunCodeGraphScheduler(...)`). Un pánico del parser sobre un `.js`
// cualquiera de un repo indexado no degradaba ese archivo: mataba el proceso entero.
//
// NO ES HIPOTÉTICO PARA LA VERSIÓN QUE VIENE: gotreesitter 0.52 mete pánicos incondicionales nuevos
// en el camino del compact parser, que es exactamente por donde pasan nuestros `.ts`, `.tsx` y
// `.py`. Esta red es precondición de ese bump, no una mejora posterior.
//
// POR QUÉ SE INYECTA EL PÁNICO EN VEZ DE PROVOCARLO DE VERDAD: no hay entrada conocida que haga
// entrar en pánico al parser real a pedido. Una guarda que no puede provocar el defecto que dice
// cuidar no distingue «hay red» de «no la probé» — y ese verde vale cero. La indirección
// (`derivarPolyglotSinRed`) existe por esta prueba y su comentario lo dice.
//
// MECANIZADA, Y EL SABOTAJE ES EL HUECO HISTÓRICO. Angostar la red POR TIPO y repanicar lo que
// no sea string: es lo que una auditoría adversaria hizo el 2026-09-12 y dejaba verde a la
// versión vieja de esta prueba, que inyectaba un string. Medido: con el angostamiento esta
// prueba falla («EL PÁNICO SUBIÓ: runtime error: index out of range [3] with length 0») y su
// hermana TestLaRedNoSeTragaLoQuePasaPorAlLado queda VERDE. El reparto es a propósito.
//
// El arreglo es la misma red sin la variable, que no se usa en el cuerpo: equivalente exacto y
// la simplificación que pediría cualquier linter. Tiene que seguir verde, y lo está.
//
// OJO: este archivo es `//go:build treesitter`. Un arnés que corra `go test` sin tags recibe
// `ok ... [no tests to run]` con exit 0 — y eso NO es un verde, es que no se midió nada.
// Sabotaje que la pone roja: angostar la red POR TIPO y repanicar lo que no sea string.
// arnes: archivo="internal/codeintel/treesit_on.go"
//
// POR QUÉ EL LITERAL ES TAN LARGO. Desde #491 este archivo tiene DOS redes: ésta y la de
// `languageFor`, que atrapa el pánico al CARGAR la gramática. Las dos abren con la misma línea
// exacta, así que `"\t\tif r := recover(); r != nil {\n"` ya no dice cuál se toca — y el arnés
// lo rechaza en vez de tocar una al azar. El literal arranca en la FIRMA de la función porque es
// lo primero que las distingue; acortarlo lo vuelve ambiguo de nuevo.
// arnes: de="func derivePolyglotFile(path, content string) (nodes []Node, edges []Edge) {\n\tdefer func() {\n\t\tif r := recover(); r != nil {\n"
// arnes: a="func derivePolyglotFile(path, content string) (nodes []Node, edges []Edge) {\n\tdefer func() {\n\t\tif r := recover(); r != nil {\n\t\t\tif _, ok := r.(string); !ok {\n\t\t\t\tpanic(r)\n\t\t\t}\n"
// arnes: arreglo_de="func derivePolyglotFile(path, content string) (nodes []Node, edges []Edge) {\n\tdefer func() {\n\t\tif r := recover(); r != nil {"
// arnes: arreglo_a="func derivePolyglotFile(path, content string) (nodes []Node, edges []Edge) {\n\tdefer func() {\n\t\tif recover() != nil {"
func TestUnPanicoDelParserNoSeLlevaElProceso(t *testing.T) {
	original := derivarPolyglotSinRed
	t.Cleanup(func() { derivarPolyglotSinRed = original })

	// EL CONTROL, PRIMERO: sin inyectar nada, un archivo real deriva algo. Sin esto, una prueba que
	// sólo mira el caso del pánico pasaría en verde con la función rota del todo.
	nodos, aristas := derivePolyglotFile("ejemplo.py", "def saludar(nombre):\n    return nombre\n\ndef main():\n    saludar('a')\n")
	if len(nodos) == 0 {
		t.Fatal("control: derivePolyglotFile no derivó NI UN nodo de un .py válido; esta prueba " +
			"estaría midiendo el camino del pánico sobre una función que ya no funciona")
	}
	t.Logf("control: %d nodos, %d aristas sobre un .py válido", len(nodos), len(aristas))

	// AHORA SÍ: que el pánico no suba. SE INYECTA UN runtime.Error, NO UN STRING, y eso no es un
	// detalle de estilo: una gramática rota no larga `panic("texto")`, larga un índice fuera de
	// rango o un nil deref — o sea un runtime.Error. La versión original de esta prueba inyectaba
	// un string, y con eso una red angostada al TIPO (`if _, ok := r.(string); !ok { panic(r) }`)
	// la dejaba VERDE mientras el daemon seguía muriendo por el caso real. Medido el 2026-09-12.
	derivarPolyglotSinRed = func(path, content string) ([]Node, []Edge) {
		var vacio []int
		_ = vacio[3] // runtime error: index out of range — lo que de verdad larga un parser roto
		return nil, nil
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EL PÁNICO SUBIÓ: %v\nderivePolyglotFile no tiene red, así que un archivo que "+
				"haga explotar la gramática se lleva el daemon entero por la goroutine pelada del "+
				"scheduler del grafo.", r)
		}
	}()
	n, e := derivePolyglotFile("cualquiera.py", "lo que sea")
	if n != nil || e != nil {
		t.Errorf("después de un pánico devolvió %d nodos y %d aristas, y tiene que devolver VACÍO: "+
			"un grafo a medias REEMPLAZA al anterior y borra símbolos que sí existían, que es peor "+
			"que no tener ninguno", len(n), len(e))
	}
}

// TestLaRedNoSeTragaLoQuePasaPorAlLado — que la red sea del ANCHO justo.
//
// LA VERSIÓN ANTERIOR DE ESTA PRUEBA ERA DECORATIVA Y LA ESCRIBÍ YO. Largaba un pánico adentro de
// su propio closure y lo recuperaba con el `defer` de ESE MISMO closure: no nombraba un solo
// símbolo de producción, así que probaba la especificación de Go y no este código. Quedaba en
// VERDE con el `recover()` de `derivePolyglotFile` BORRADO ENTERO — o sea que no distinguía
// «la red es angosta» de «no hay red». La encontró una auditoría adversaria el 2026-09-12, y
// estaba anunciada en el PR como el control sofisticado de las tres.
//
// LO QUE SÍ MIDE EL ANCHO: que `derivePolyglotFile` contenga el pánico de SU archivo, y que un
// pánico largado FUERA de esa llamada —en el mismo paquete y el mismo stack— siga subiendo. Para
// eso hay que atravesar la función de verdad, no un closure de juguete.
//
// Sabotaje: no hay ninguno que un arnés pueda aplicar desde producción. El motivo, abajo.
// arnes: no_mecanizable="el medio que esta prueba AGREGA —que un panico largado AL LADO siga subiendo— sale de `panicoDeAlLado`, que vive en este archivo de prueba a proposito: tiene que atravesar codigo del paquete SIN estar bajo la red. Un arnes que solo muta produccion no puede provocarlo. Su otro medio (que el panico de adentro no suba) ya lo mide TestUnPanicoDelParserNoSeLlevaElProceso con SU sabotaje, y esta MEDIDO que esta prueba queda verde bajo aquel: inyecta un string y la red angostada por tipo se lo traga igual."
func TestLaRedNoSeTragaLoQuePasaPorAlLado(t *testing.T) {
	original := derivarPolyglotSinRed
	t.Cleanup(func() { derivarPolyglotSinRed = original })

	// Adentro: contenido.
	derivarPolyglotSinRed = func(path, content string) ([]Node, []Edge) { panic("adentro") }
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("el pánico de adentro de derivePolyglotFile SUBIÓ (%v): no hay red", r)
			}
		}()
		derivePolyglotFile("x.py", "lo que sea")
	}()

	// Al lado: tiene que subir. Se llama a una función REAL del paquete que no está bajo la red,
	// con una entrada que la hace explotar — no a un closure escrito para la ocasión.
	derivarPolyglotSinRed = original
	subio := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				subio = true
			}
		}()
		// `lineAtByte` indexa el contenido; un offset fuera de rango revienta, y no está protegido.
		_ = panicoDeAlLado()
	}()
	if !subio {
		t.Fatal("un pánico largado FUERA de derivePolyglotFile no subió: la red está más ancha de " +
			"lo que dice y estaría tapando cosas de las que hay que enterarse")
	}
}

// panicoDeAlLado provoca un pánico REAL en el mismo paquete, fuera del alcance de la red. Existe
// para que la prueba del ancho atraviese código de este paquete y no un closure de juguete: ése
// fue exactamente el defecto de la versión anterior.
func panicoDeAlLado() int {
	var vacio []int
	return vacio[3]
}

// TestUnPanicoAlCARGARLaGramaticaTampocoSeLlevaElProceso — la red empezaba una línea tarde.
//
// LO QUE SE MIDIÓ, Y ES EL MOTIVO DE ESTA PRUEBA. El recover de `derivePolyglotFile` cubre el
// PARSEO. Pero la misma dependencia se toca antes y AFUERA de esa red, al CARGAR la gramática:
//
//	graph.go:116  IndexableForGraph → polyglotSupported → languageFor   ← el indexador MCP
//	graph.go:374  el filtro de DerivePackage, UNA LÍNEA arriba del llamado protegido
//
// El 2026-09-12 se puso `case ".tsx": panic(…)` en `languageFor` —el escenario «gotreesitter mete
// pánicos incondicionales» que la red de al lado cita como SU motivo— y las DOS guardas del pánico
// quedaron VERDES. Caía `TestIndexableForGraphWithTreesitter`, pero por explotar él también, no por
// afirmar contención: CI lo DETECTABA y nadie lo CONTENÍA. En producción eso se lleva el daemon por
// la goroutine pelada del scheduler del grafo, que es exactamente lo que la otra red evita.
//
// Se prueban LOS DOS SITIOS y no uno: son dos llamadores distintos —el indexador y el derivador— y
// cubrir sólo el que uno tiene en la cabeza es la forma dominante de este repo (la guarda que está
// en N-1 de N caminos).
func TestUnPanicoAlCARGARLaGramaticaTampocoSeLlevaElProceso(t *testing.T) {
	original := cargarGramatica
	t.Cleanup(func() { cargarGramatica = original })

	// EL CONTROL, PRIMERO: sin inyectar nada, un .tsx es indexable. Sin esto, la prueba de abajo
	// pasaría en verde con `languageFor` devolviendo nil siempre, o sea midiendo la contención
	// sobre un camino que ya no hace nada.
	if !IndexableForGraph("componente.tsx") {
		t.Fatal("control: un .tsx no es indexable con el binario compilado con tags; esta prueba " +
			"estaría midiendo la red sobre un camino que ya no carga ninguna gramática")
	}

	cargarGramatica = func(path string) *ts.Language {
		var vacio []int
		_ = vacio[3] // runtime error: index out of range — lo que de verdad larga un cargador roto
		return nil
	}

	// SITIO 1: el indexador. `IndexableForGraph` la llama por `polyglotSupported`.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("EL PÁNICO SUBIÓ por IndexableForGraph (%v): el indexador de la capa MCP "+
					"llama a `languageFor` sin red, así que una gramática que explota al cargarse se "+
					"lleva el proceso que esté recolectando archivos", r)
			}
		}()
		if IndexableForGraph("componente.tsx") {
			t.Error("con el cargador roto un .tsx se declaró indexable: la degradación tiene que ser " +
				"a NO derivable, que es lo mismo que contesta un languageFor que no conoce la extensión")
		}
	}()

	// SITIO 2: el derivador. `DerivePackage` filtra con `polyglotSupported` UNA LÍNEA antes de
	// llamar al `derivePolyglotFile` que sí está protegido. Se atraviesa la función real.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("EL PÁNICO SUBIÓ por DerivePackage (%v): el filtro que elige qué archivos "+
					"derivar corre FUERA de la red de derivePolyglotFile, una línea más arriba", r)
			}
		}()
		g := DerivePackage("paquete", map[string]string{
			"componente.tsx": "export const X = 1\n",
			"modulo.go":      "package modulo\n\nfunc F() {}\n",
		}, "ejemplo/modulo")
		// El .go tiene que seguir derivándose: que una gramática polyglot explote no puede apagar
		// el pase Go, que no la toca.
		if len(g.Nodes) == 0 {
			t.Errorf("con el cargador polyglot roto no se derivó NI UN nodo (%d aristas): el pase Go "+
				"no depende de tree-sitter y tiene que seguir andando", len(g.Edges))
		}
	}()
}
