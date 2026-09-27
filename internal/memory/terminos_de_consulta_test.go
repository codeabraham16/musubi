package memory

import (
	"reflect"
	"testing"
)

// TestTerminosDeConsultaYElFallbackDelRecall: TerminosDeConsulta es la definición de término que
// comparten el recall, la compuerta del hook del turno y el corrector de tipeo, y una consulta de
// puro ruido no tiene ninguno. El fallback a los tramos crudos es una decisión del recall
// (rankedTerms) y no de la definición: si se colara en TerminosDeConsulta, «de la» pasaría la
// compuerta como un pedido con dos términos y el recall buscaría medio corpus.
//
// Sabotaje: el recall pierde el fallback.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\treturn camposDeConsulta(q) // fallback: no perder recall si todo era ruido\n"
// arnes: a="\treturn nil // fallback: no perder recall si todo era ruido\n"
//
// Sabotaje: el fallback se cuela en la definición de término.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tterms = append(terms, f)\n\t}\n\treturn terms\n"
// arnes: a="\t\tterms = append(terms, f)\n\t}\n\tif len(terms) == 0 {\n\t\treturn camposDeConsulta(q)\n\t}\n\treturn terms\n"
func TestTerminosDeConsultaYElFallbackDelRecall(t *testing.T) {
	casos := []struct {
		q                string
		terminos, ranked []string
	}{
		// La 'N' y el '1' de «N+1» son de una runa; «con», «en» y «la» son vacías; «qué» con tilde no
		// es la stopword «que».
		{"¿qué pasó con N+1 en la DB?", []string{"qué", "pasó", "DB"}, []string{"qué", "pasó", "DB"}},
		{"v2 del índice", []string{"v2", "índice"}, []string{"v2", "índice"}},
		{"de la", nil, []string{"de", "la"}},
		{"?", nil, nil},
	}
	for _, c := range casos {
		if got := TerminosDeConsulta(c.q); !mismosTerminos(got, c.terminos) {
			t.Errorf("TerminosDeConsulta(%q) = %q, quería %q", c.q, got, c.terminos)
		}
		if got := rankedTerms(c.q); !mismosTerminos(got, c.ranked) {
			t.Errorf("rankedTerms(%q) = %q, quería %q", c.q, got, c.ranked)
		}
	}
}

// mismosTerminos compara dos listas de términos, con la vacía igual a nil.
func mismosTerminos(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	return reflect.DeepEqual(a, b)
}
