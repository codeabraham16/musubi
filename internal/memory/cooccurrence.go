package memory

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// cooccurrence.go implementa el primer slice de SEMÁNTICA MODEL-FREE del recall (Track 14 #2):
// expansión por PSEUDO-RELEVANCE FEEDBACK (PRF). Sin embedder externo el recall sólo tiene señal
// LÉXICA (FTS token-exact) — 'deploy' no encuentra una observación que dice 'despliegue'. PRF
// agrega un PUENTE DE VOCABULARIO derivado del corpus: asume que los top-M resultados de la query
// son relevantes, cosecha los términos que CO-OCURREN con la query en ellos (aparecen en ≥2 de
// esos docs, no son la query ni stopwords) y corre un 2º FTS con esos términos para traer
// observaciones que la query original NO encontró. MODEL-FREE y determinista: sólo tokenización +
// conteo + FTS; la "semántica" se DERIVA del corpus, no se importa de un modelo.

const (
	cooccurrenceTopDocs        = 5 // top-M pseudo-relevantes de los que se cosechan términos
	cooccurrenceExpansionTerms = 4 // tope de términos de expansión
	cooccurrenceMinDocFreq     = 2 // un término debe co-ocurrir en ≥ este nº de top-docs distintos
)

// splitTerms tokeniza un texto igual que buildFTSQuery (runs alfanuméricos), en minúsculas.
func splitTerms(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.ToLower(f))
	}
	return out
}

// queryTermSet devuelve el conjunto de términos (lowercased) de la query, para EXCLUIRLOS de la
// expansión (no se "expande" con las propias palabras buscadas).
func queryTermSet(query string) map[string]bool {
	set := map[string]bool{}
	for _, t := range splitTerms(query) {
		set[t] = true
	}
	return set
}

// extractExpansionTerms cosecha los términos de expansión del contenido de los top-M docs
// pseudo-relevantes: un término califica si aparece en ≥ cooccurrenceMinDocFreq docs DISTINTOS
// (co-ocurrencia real, no de un solo doc), no es stopword ni término de la query y tiene ≥2 runas.
// Orden determinista: docFreq desc, término asc; tope cooccurrenceExpansionTerms. Función PURA.
func extractExpansionTerms(topContents []string, queryTerms map[string]bool) []string {
	docFreq := map[string]int{}
	for _, content := range topContents {
		seen := map[string]bool{} // contar cada término una sola vez por doc
		for _, t := range splitTerms(content) {
			if seen[t] || len([]rune(t)) < 2 || ftsStopwords[t] || queryTerms[t] {
				continue
			}
			seen[t] = true
			docFreq[t]++
		}
	}
	type termFreq struct {
		term string
		df   int
	}
	var qualified []termFreq
	for t, df := range docFreq {
		if df >= cooccurrenceMinDocFreq {
			qualified = append(qualified, termFreq{t, df})
		}
	}
	sort.Slice(qualified, func(i, j int) bool {
		if qualified[i].df != qualified[j].df {
			return qualified[i].df > qualified[j].df
		}
		return qualified[i].term < qualified[j].term // desempate determinista
	})
	out := make([]string, 0, cooccurrenceExpansionTerms)
	for _, q := range qualified {
		if len(out) >= cooccurrenceExpansionTerms {
			break
		}
		out = append(out, q.term)
	}
	return out
}

// augmentWithCooccurrencePool aplica PRF: cosecha términos de expansión de los top-M candidatos
// (por orden FTS, lexRank < cooccurrenceTopDocs), corre un 2º FTS con ellos y UNE por id los
// candidatos NUEVOS al pool (el puente de vocabulario). Devuelve el pool aumentado y coocRank
// (id→rango en los resultados de expansión, 0 = mejor). No-op seguro (cands, nil) cuando no hay
// suficientes docs, no hay términos de expansión, o el 2º FTS no trae nada ⇒ equivalencia.
func (e *DbEngine) augmentWithCooccurrencePool(ctx context.Context, cands []candidate, query string, lexRank map[string]int, limit int) ([]candidate, map[string]int, error) {
	var topContents []string
	for _, c := range cands {
		if r, ok := lexRank[c.id]; ok && r < cooccurrenceTopDocs {
			topContents = append(topContents, c.content)
		}
	}
	if len(topContents) < cooccurrenceMinDocFreq {
		return cands, nil, nil
	}

	terms := extractExpansionTerms(topContents, queryTermSet(query))
	if len(terms) == 0 {
		return cands, nil, nil
	}

	quoted := make([]string, len(terms))
	for i, t := range terms {
		quoted[i] = `"` + t + `"`
	}
	results, scores, err := e.ftsSearch(ctx, strings.Join(quoted, " OR "), limit)
	if err != nil {
		return cands, nil, err
	}
	if len(results) == 0 {
		return cands, nil, nil
	}

	have := make(map[string]bool, len(cands))
	for _, c := range cands {
		have[c.id] = true
	}
	// coocRank DENSO por score bm25 (Q3), igual que lexRank: empates de relevancia comparten rango
	// en vez de recibir posiciones arbitrarias por rowid. results ya viene ordenado por rank.
	coocRank := make(map[string]int, len(results))
	rank := 0
	for i, r := range results {
		if i > 0 && scores[i] != scores[i-1] {
			rank++
		}
		coocRank[r.id] = rank
		if !have[r.id] {
			cands = append(cands, r) // puente de vocabulario: obs que la query original no halló
			have[r.id] = true
		}
	}
	return cands, coocRank, nil
}
