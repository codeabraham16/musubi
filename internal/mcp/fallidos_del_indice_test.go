package mcp

import "testing"

// B1 — fallidosDelIndice LEE LA CLAVE DE CADA CORRIDA.
//
// En el incremental `skipped` son archivos SIN CAMBIO y los fallidos van en `failed`; en el completo
// `skipped` son los directorios que FALLARON. Confundirlas sella el commit del índice con fallidos o
// lo frena con el árbol sano, y ninguna otra prueba lo notaba: con `return 0` la suite de mcp daba
// ok.
//
// Sabotaje que la pone roja: `return 0` al entrar (S3).
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="func fallidosDelIndice(res map[string]interface{}, incremental bool) int {"
// arnes: a="func fallidosDelIndice(res map[string]interface{}, incremental bool) int {\n\treturn 0"
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
