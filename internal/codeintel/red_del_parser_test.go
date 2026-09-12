//go:build treesitter

package codeintel

import (
	"strings"
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

	// AHORA SÍ: que el pánico no suba.
	derivarPolyglotSinRed = func(path, content string) ([]Node, []Edge) {
		panic("pánico de prueba adentro del parser")
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

// TestLaRedNoSeTragaUnPanicoDeOtroLado — que la red sea del ANCHO justo.
//
// Un `recover()` puesto con la mano floja se come cualquier pánico del proceso que pase por ahí,
// incluido uno que signifique corrupción y del que haya que enterarse. Acá el alcance es el
// `defer` de UNA llamada: se comprueba que el pánico de la función de al lado —el mismo paquete,
// el mismo stack— sigue subiendo.
func TestLaRedNoSeTragaUnPanicoDeOtroLado(t *testing.T) {
	subio := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				subio = true
				if !strings.Contains(r.(string), "de otro lado") {
					t.Errorf("subió un pánico que no es el que largué: %v", r)
				}
			}
		}()
		// Fuera del alcance de derivePolyglotFile: tiene que subir.
		panic("pánico de otro lado")
	}()
	if !subio {
		t.Fatal("un pánico ajeno a derivePolyglotFile NO subió: la red está puesta más ancha de lo " +
			"que dice, y estaría tapando cosas de las que hay que enterarse")
	}
}
