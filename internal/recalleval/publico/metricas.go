package publico

import (
	"math"
	"sort"
)

// LAS MÉTRICAS DE LONGMEMEVAL, COMO LAS CALCULA SU CÓDIGO (eval_utils.py), no como las calcula
// recalleval. Las dos familias llevan el mismo nombre y NO son la misma cuenta:
//
//   - recall: el paper reporta recall_all@k, que vale 1 sólo si TODAS las sesiones de oro están en
//     el top-k (y recall_any@k, 1 si está alguna). recalleval.RecallAtK es la FRACCIÓN del oro que
//     entró. Con dos sesiones de oro y una adentro: 0 para el paper, 0,5 para recalleval.
//   - nDCG: el dcg del paper es `rel[0] + Σ rel[i]/log2(i+1)` para i≥1 con índice 0, así que las
//     posiciones 1 y 2 pesan lo mismo (1/log2(2) = 1). recalleval descuenta 1/log2(i+2) desde la
//     primera. Un oro en la posición 2 vale 1 para el paper y 0,63 para recalleval.
//
// Comparar un número de recalleval contra la tabla del paper sería comparar dos cuentas distintas
// con el mismo nombre. Por eso la tabla se arma con éstas, y las de recalleval van aparte.
//
// Todas reciben el ranking en POSICIONES del corpus (mejor primero) y las etiquetas del paper. El
// ranking puede ser más corto que el corpus —el léxico de Musubi no devuelve lo que no coincide— y
// lo que falta cuenta como no recuperado.

// oro es el conjunto de etiquetas de oro (un set, como `list(set(...))` en el paper: una sesión
// repetida en el pajar es UN oro para el recall).
func oro(etiquetas []string) map[string]bool {
	o := map[string]bool{}
	for _, e := range etiquetas {
		if EsOro(e) {
			o[e] = true
		}
	}
	return o
}

// RecallAnyAll replica evaluate_retrieval: el conjunto de etiquetas del top-k contra el conjunto de
// oro. Sin oro, recall_all vale 1 y recall_any 0 (`all([])` y `any([])` de Python); el paper saca
// esas preguntas del promedio antes (ver Exclusion), y el medidor cuenta las que se le escapen.
func RecallAnyAll(ranking []int, etiquetas []string, k int) (anyK, allK float64) {
	correctas := oro(etiquetas)
	if k > len(ranking) {
		k = len(ranking)
	}
	recuperadas := make(map[string]bool, k)
	for _, i := range ranking[:k] {
		recuperadas[etiquetas[i]] = true
	}
	algun, todos := false, true
	for e := range correctas {
		if recuperadas[e] {
			algun = true
		} else {
			todos = false
		}
	}
	if algun {
		anyK = 1
	}
	if todos {
		allK = 1
	}
	return anyK, allK
}

// dcgPaper replica dcg() de eval_utils.py: rel[0] + Σ_{i≥1} rel[i]/log2(i+1), sobre los primeros k.
func dcgPaper(rel []float64, k int) float64 {
	if k < len(rel) {
		rel = rel[:k]
	}
	if len(rel) == 0 {
		return 0
	}
	s := rel[0]
	for i := 1; i < len(rel); i++ {
		s += rel[i] / math.Log2(float64(i)+1)
	}
	return s
}

// NDCGPaper replica ndcg() de eval_utils.py. La relevancia es POR POSICIÓN del corpus: una sesión
// de oro repetida en el pajar suma dos veces al ideal, como en el paper.
func NDCGPaper(ranking []int, etiquetas []string, k int) float64 {
	correctas := oro(etiquetas)
	rel := make([]float64, len(etiquetas))
	for i, e := range etiquetas {
		if correctas[e] {
			rel[i] = 1
		}
	}
	lim := k
	if lim > len(ranking) {
		lim = len(ranking)
	}
	actual := make([]float64, lim)
	for j, i := range ranking[:lim] {
		actual[j] = rel[i]
	}
	ideal := append([]float64(nil), rel...)
	sort.Sort(sort.Reverse(sort.Float64Slice(ideal)))
	id := dcgPaper(ideal, k)
	if id == 0 {
		return 0
	}
	return dcgPaper(actual, k) / id
}
