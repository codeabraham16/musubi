package mcp

import "testing"

// B1 — fallidosDelIndice LEE LA CLAVE DE CADA CORRIDA.
//
// En el incremental `skipped` son archivos SIN CAMBIO y los fallidos van en `failed`; en el completo
// `skipped` son los directorios que FALLARON. Confundirlas sella el commit del índice con fallidos o
// lo frena con el árbol sano, y ninguna otra prueba lo notaba: con `return 0` la suite de mcp daba
// ok.
//
// Sabotaje que la pone roja: entrar SIEMPRE por la rama del índice completo, o sea leer `skipped`
// también en el incremental — la confusión exacta que esta guarda existe para impedir. Con
// {failed:2, skipped:40} devuelve 40 donde tiene que devolver 2.
//
// ESTA DIRECTIVA NACIÓ ROTA Y LO DIJO EL BARRIDO, NO UNA REVISIÓN. La versión original inyectaba
// `return 0` justo después de la firma, con lo que las cinco líneas siguientes quedaban INALCANZABLES
// y `go vet` rechazaba el paquete:
//
//	internal/mcp/methods_codegraph.go:601:2: unreachable code
//
// Un sabotaje que no compila no prueba nada —con el paquete roto, `go test` sale distinto de cero
// pase lo que pase— así que el arnés no lo contó como rojo sino como SIN VEREDICTO, que no es un
// verde: era la única de las 61 corridas de `internal/mcp` que no se estaba midiendo. La salida es
// mutar la DECISIÓN en vez de inyectar un retorno temprano; el parámetro sigue usado y no queda
// código muerto.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="func fallidosDelIndice(res map[string]interface{}, incremental bool) int {"
// arnes: a="func fallidosDelIndice(res map[string]interface{}, incremental bool) int {\n\tincremental = false"
func TestFallidosDelIndiceLeeLaClaveDeCadaCorrida(t *testing.T) {
	casos := []struct {
		nombre      string
		res         map[string]interface{}
		incremental bool
		quiere      int
	}{
		{"incremental con fallidos", map[string]interface{}{"failed": 2, "skipped": 40}, true, 2},
		{"incremental sano: skipped son archivos sin cambio", map[string]interface{}{"skipped": 40}, true, 0},
		{"completo con fallidos: van en skipped", map[string]interface{}{"skipped": 3}, false, 3},
		{"completo sano", map[string]interface{}{"skipped": 0, "packages": 9}, false, 0},
	}
	for _, c := range casos {
		if got := fallidosDelIndice(c.res, c.incremental); got != c.quiere {
			t.Errorf("%s: fallidosDelIndice=%d, quiere %d", c.nombre, got, c.quiere)
		}
	}
}
