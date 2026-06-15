package memory

import "strings"

// similarity.go provee similitud de texto MODEL-FREE (Jaccard sobre trigramas de
// caracteres) para detectar casi-duplicados en la consolidación. Determinista,
// sin LLM ni embeddings.

// Similarity devuelve la similitud de Jaccard (0..1) entre dos textos, comparando
// sus conjuntos de trigramas de caracteres tras normalizar mayúsculas y espacios.
func Similarity(a, b string) float64 {
	na := normalizeForSim(a)
	nb := normalizeForSim(b)
	if na == nb {
		return 1.0
	}

	sa := trigrams(na)
	sb := trigrams(nb)
	if len(sa) == 0 && len(sb) == 0 {
		return 1.0
	}
	if len(sa) == 0 || len(sb) == 0 {
		return 0.0
	}

	inter := 0
	for g := range sa {
		if sb[g] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0.0
	}
	return float64(inter) / float64(union)
}

func normalizeForSim(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// trigrams devuelve el conjunto de trigramas de caracteres de s. Para textos de
// menos de 3 runas usa el texto completo como único gram.
func trigrams(s string) map[string]bool {
	r := []rune(s)
	set := make(map[string]bool)
	if len(r) < 3 {
		if len(r) > 0 {
			set[string(r)] = true
		}
		return set
	}
	for i := 0; i+3 <= len(r); i++ {
		set[string(r[i:i+3])] = true
	}
	return set
}
