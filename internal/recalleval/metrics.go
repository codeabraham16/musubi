// Package recalleval es el HARNESS DE CALIDAD DE RECALL de Musubi (Track 16 F2.1):
// una forma REPETIBLE y determinista de medir qué tan bueno es el recall, para poder
// probar con números —no con fe— que encender la señal semántica MEJORA sobre el
// baseline léxico antes de cambiar el default (F2.5). Es 100% model-free y sin red: las
// métricas son aritmética pura y el corpus/queries viven en un fixture versionado.
//
// El harness siembra un motor de memoria temporal con un corpus etiquetado (docs), corre
// cada query bajo una o más configuraciones de recall, y compara el ranking devuelto
// contra el conjunto de docs RELEVANTES conocido, agregando métricas estándar de IR:
// recall@k, MRR y nDCG@k.
package recalleval

import "math"

// RecallAtK es la fracción de docs relevantes que aparecen en el top-k del ranking.
// Sin relevantes ⇒ 0 (el caller decide si omitir esas queries del promedio).
func RecallAtK(ranked []string, relevant map[string]bool, k int) float64 {
	total := len(relevant)
	if total == 0 {
		return 0
	}
	if k > len(ranked) {
		k = len(ranked)
	}
	hits := 0
	for i := 0; i < k; i++ {
		if relevant[ranked[i]] {
			hits++
		}
	}
	return float64(hits) / float64(total)
}

// ReciprocalRank es 1/(posición 1-indexada del primer relevante), o 0 si ninguno aparece.
// El promedio sobre queries es el MRR.
func ReciprocalRank(ranked []string, relevant map[string]bool) float64 {
	for i, id := range ranked {
		if relevant[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// NDCGAtK es el Discounted Cumulative Gain normalizado en el top-k con relevancia BINARIA
// (relevante=1, no=0): DCG@k / IDCG@k. El descuento es 1/log2(pos+1) con pos 1-indexada.
// IDCG@k es el DCG del ranking ideal (todos los relevantes arriba). Sin relevantes ⇒ 0.
func NDCGAtK(ranked []string, relevant map[string]bool, k int) float64 {
	idcg := idealDCG(len(relevant), k)
	if idcg == 0 {
		return 0
	}
	limit := k
	if limit > len(ranked) {
		limit = len(ranked)
	}
	var dcg float64
	for i := 0; i < limit; i++ {
		if relevant[ranked[i]] {
			dcg += 1.0 / math.Log2(float64(i)+2.0) // pos 1-indexada ⇒ log2(i+2)
		}
	}
	return dcg / idcg
}

// idealDCG es el DCG del ranking perfecto para r relevantes recortado a k posiciones.
func idealDCG(r, k int) float64 {
	if r > k {
		r = k
	}
	var idcg float64
	for i := 0; i < r; i++ {
		idcg += 1.0 / math.Log2(float64(i)+2.0)
	}
	return idcg
}

// mean promedia un slice (0 si está vacío). Helper de agregación del harness.
func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
