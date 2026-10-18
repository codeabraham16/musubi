package publico

import (
	"math"
	"testing"

	"musubi/internal/recalleval"
)

func cerca(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// recall_all del paper NO es la fracción de recalleval: con dos sesiones de oro y una sola en el
// top-k, el paper da 0 y recalleval 0,5. La tabla del paper se compara con la primera.
//
// Sabotaje: recall_all se conforma con que entre alguna sesión de oro.
// arnes: archivo="internal/recalleval/publico/metricas.go"
// arnes: de="\t\t} else {\n\t\t\ttodos = false\n\t\t}\n"
// arnes: a="\t\t}\n"
func TestRecallAllExigeTodoElOro(t *testing.T) {
	etiquetas := []string{"answer_a", "x", "y", "answer_b", "z"}
	ranking := []int{0, 1, 2, 3, 4} // answer_a primero, answer_b cuarto
	anyK, allK := RecallAnyAll(ranking, etiquetas, 3)
	if anyK != 1 || allK != 0 {
		t.Fatalf("@3 con un oro de dos: any=%v all=%v, quiero 1 y 0", anyK, allK)
	}
	anyK, allK = RecallAnyAll(ranking, etiquetas, 4)
	if anyK != 1 || allK != 1 {
		t.Fatalf("@4 con los dos oros: any=%v all=%v, quiero 1 y 1", anyK, allK)
	}
	// Ranking más corto que k (el léxico no devuelve lo que no coincide): lo que falta no está.
	anyK, allK = RecallAnyAll([]int{1}, etiquetas, 10)
	if anyK != 0 || allK != 0 {
		t.Fatalf("ranking de un doc sin oro: any=%v all=%v, quiero 0 y 0", anyK, allK)
	}
	// La cuenta de recalleval, para que la diferencia quede escrita y no se «arregle» un día.
	rel := map[string]bool{"answer_a": true, "answer_b": true}
	if got := recalleval.RecallAtK([]string{"answer_a", "x", "y", "answer_b"}, rel, 3); got != 0.5 {
		t.Fatalf("recalleval.RecallAtK = %v; esta prueba documenta que es la fracción (0,5)", got)
	}
}

// Una sesión repetida en el pajar es UN oro para el recall (el paper arma un set de etiquetas) y
// DOS relevantes para el ideal del nDCG (la relevancia es por posición).
func TestOroRepetidoEnElPajar(t *testing.T) {
	etiquetas := []string{"answer_a", "x", "answer_a"}
	if _, allK := RecallAnyAll([]int{0, 1, 2}, etiquetas, 1); allK != 1 {
		t.Fatalf("con la sesión de oro en el top-1 (repetida más abajo) recall_all = %v, quiero 1", allK)
	}
	// dcg real = 1 (pos 0) + 0 + 1/log2(3); ideal = 1 + 1.
	if got, quiero := NDCGPaper([]int{0, 1, 2}, etiquetas, 10), (1+1/math.Log2(3))/2; !cerca(got, quiero) {
		t.Fatalf("NDCGPaper = %v, quiero %v", got, quiero)
	}
}

// EL DESCUENTO DEL PAPER ES OTRO: su dcg es rel[0] + Σ_{i≥1} rel[i]/log2(i+1) con índice 0, así que
// las posiciones 1 y 2 pesan igual. Un único oro en la posición 2 da nDCG 1 en el paper; en
// recalleval, 1/log2(3) ≈ 0,63. Los valores esperados están hechos a mano, no con la función.
//
// Sabotaje: el nDCG del paper usa el descuento de recalleval.
// arnes: archivo="internal/recalleval/publico/metricas.go"
// arnes: de="s += rel[i] / math.Log2(float64(i)+1)"
// arnes: a="s += rel[i] / math.Log2(float64(i)+2)"
func TestNDCGPaperUsaElDescuentoDelPaper(t *testing.T) {
	etiquetas := []string{"x", "answer_a", "y", "z"}
	if got := NDCGPaper([]int{0, 1, 2, 3}, etiquetas, 10); !cerca(got, 1) {
		t.Fatalf("oro único en la posición 2: NDCGPaper = %v, el paper da 1", got)
	}
	if got := NDCGPaper([]int{0, 2, 1, 3}, etiquetas, 10); !cerca(got, 1/math.Log2(3)) {
		t.Fatalf("oro único en la posición 3: NDCGPaper = %v, quiero 1/log2(3)", got)
	}
	// Dos oros, en las posiciones 1 y 3: (1 + 1/log2(3)) / (1 + 1).
	dos := []string{"answer_a", "x", "answer_b", "y"}
	if got, quiero := NDCGPaper([]int{0, 1, 2, 3}, dos, 10), (1+1/math.Log2(3))/2; !cerca(got, quiero) {
		t.Fatalf("NDCGPaper = %v, quiero %v", got, quiero)
	}
	// Fuera del corte no suma: @2 sólo ve la posición 1.
	if got, quiero := NDCGPaper([]int{0, 1, 2, 3}, dos, 2), 1.0/2; !cerca(got, quiero) {
		t.Fatalf("NDCGPaper@2 = %v, quiero %v", got, quiero)
	}
	if got := NDCGPaper([]int{0, 1}, []string{"x", "y"}, 10); got != 0 {
		t.Fatalf("sin oro el ideal es 0 y el paper devuelve 0; dio %v", got)
	}
	// recalleval descuenta desde la primera posición: la misma lista da otro número.
	if got := recalleval.NDCGAtK([]string{"x", "answer_a"}, map[string]bool{"answer_a": true}, 10); !cerca(got, 1/math.Log2(3)) {
		t.Fatalf("recalleval.NDCGAtK = %v; esta prueba documenta que descuenta 1/log2(i+2)", got)
	}
}
