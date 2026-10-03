package publico

import (
	"math"
	"sort"
	"strings"
)

// LA RÉPLICA DEL BM25 DEL PAPER. Existe para una sola cosa: decidir si los números de la tabla del
// paper son comparables con lo que mide este adaptador. Si la réplica, sobre este mismo corpus,
// da lo que el paper publica, el corpus, el oro, las exclusiones y las métricas están bien
// replicados, y la distancia entre Musubi y esa fila es de Musubi. Si no da, algo de lo de arriba
// difiere, y comparar a Musubi contra la tabla sería comparar dos experimentos distintos.
//
// Replica rank_bm25.BM25Okapi (v0.2.x), que es lo que llama run_retrieval.py:105-111, con SU
// tokenización: `doc.split(" ")` y `query.split(" ")`. Nada de minúsculas ni de puntuación:
// «What» y «what» son tokens distintos, y «with?» no es «with». Eso no es un descuido de la réplica:
// es el baseline que el paper midió.

// Tokens replica str.split(" ") de Python con separador explícito: corta en CADA espacio y conserva
// las cadenas vacías entre espacios seguidos. strings.Split hace exactamente eso.
func Tokens(s string) []string { return strings.Split(s, " ") }

// BM25Okapi es el índice de UN corpus, con los parámetros por defecto de rank_bm25.
type BM25Okapi struct {
	k1, b, eps float64
	frecs      []map[string]int
	largo      []int
	avgdl      float64
	idf        map[string]float64
}

// NuevoBM25Okapi indexa el corpus tokenizado.
//
// EL ORDEN DE LA SUMA DEL IDF IMPORTA para ser bit-exacto: average_idf se acumula recorriendo el
// dict `nd` de Python, que conserva el orden de inserción —la primera aparición de cada palabra,
// doc por doc—. Acá se recorre el vocabulario en ese mismo orden.
func NuevoBM25Okapi(corpus [][]string) *BM25Okapi {
	m := &BM25Okapi{k1: 1.5, b: 0.75, eps: 0.25, idf: map[string]float64{}}
	nd := map[string]int{}
	var orden []string
	total := 0
	for _, doc := range corpus {
		m.largo = append(m.largo, len(doc))
		total += len(doc)
		f := map[string]int{}
		var ordenDoc []string
		for _, w := range doc {
			if _, ok := f[w]; !ok {
				ordenDoc = append(ordenDoc, w)
			}
			f[w]++
		}
		m.frecs = append(m.frecs, f)
		for _, w := range ordenDoc {
			if _, ok := nd[w]; !ok {
				orden = append(orden, w)
			}
			nd[w]++
		}
	}
	n := len(corpus)
	if n == 0 {
		return m
	}
	m.avgdl = float64(total) / float64(n)

	// _calc_idf: idf = ln(N - df + 0,5) - ln(df + 0,5); los negativos (palabras en más de la mitad
	// de los docs) pasan a eps · average_idf. El cero exacto (df = N/2) NO se toca: la condición del
	// original es `idf < 0`.
	suma := 0.0
	var negativas []string
	for _, w := range orden {
		df := float64(nd[w])
		v := math.Log(float64(n)-df+0.5) - math.Log(df+0.5)
		m.idf[w] = v
		suma += v
		if v < 0 {
			negativas = append(negativas, w)
		}
	}
	piso := m.eps * (suma / float64(len(m.idf)))
	for _, w := range negativas {
		m.idf[w] = piso
	}
	return m
}

// Puntajes replica get_scores: para cada token de la consulta, CON repeticiones y en orden,
// score += idf · tf·(k1+1) / (tf + k1·(1 − b + b·dl/avgdl)). Un token que no está en el corpus
// suma 0 (`self.idf.get(q) or 0`). Las operaciones van en el orden de numpy para que la suma dé el
// mismo float.
func (m *BM25Okapi) Puntajes(consulta []string) []float64 {
	s := make([]float64, len(m.frecs))
	for _, q := range consulta {
		idf := m.idf[q]
		for d, f := range m.frecs {
			tf := float64(f[q])
			num := tf * (m.k1 + 1)
			den := tf + m.k1*(1-m.b+m.b*float64(m.largo[d])/m.avgdl)
			s[d] += idf * (num / den)
		}
	}
	return s
}

// RankingBM25 ordena las posiciones por puntaje, de mayor a menor, como `np.argsort(scores)[::-1]`.
//
// LOS EMPATES NO SE PUEDEN REPLICAR BIT A BIT. argsort usa por defecto un introsort, que no es
// estable: el orden entre puntajes iguales depende de su implementación. Acá se desempata como el
// reverso de un orden ascendente ESTABLE —a igual puntaje, la posición mayor primero—, que es
// determinista, y el medidor cuenta en cuántas preguntas un empate cruza el corte de k, que es
// cuando el desempate puede mover la métrica (ver EmpateEnCorte).
func RankingBM25(puntajes []float64) []int {
	idx := make([]int, len(puntajes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := puntajes[idx[a]], puntajes[idx[b]]
		if pa != pb {
			return pa > pb
		}
		return idx[a] > idx[b]
	})
	return idx
}

// EmpateEnCorte dice si el puntaje del último doc que entra al top-k es igual al del primero que
// queda afuera: ahí el desempate decide qué doc entra, y la métrica @k depende de él.
func EmpateEnCorte(ranking []int, puntajes []float64, k int) bool {
	if k <= 0 || k >= len(ranking) {
		return false
	}
	return puntajes[ranking[k-1]] == puntajes[ranking[k]]
}
