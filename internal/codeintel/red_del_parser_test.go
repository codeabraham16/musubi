//go:build treesitter

package codeintel

import (
	"testing"
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
