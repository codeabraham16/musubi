package memory

import (
	"fmt"
	"sort"
	"time"
)

// prime.go implementa el "memory priming" del arranque: un recall SIN query que
// devuelve los gists de las observaciones de mayor SALIENCIA (importancia ×
// frecuencia × recencia) dentro de un presupuesto de tokens. Es lo que permite
// que cada sesión arranque "acordándose" del proyecto. 100% model-free y de solo
// lectura (no toca stats de acceso: el priming es pasivo, no un recall activo).

const defaultPrimeBudget = 300

// PrimeContext devuelve los gists más salientes que entren en budget tokens,
// rankeados por saliencia. No recibe query: es contexto general del proyecto.
func (e *DbEngine) PrimeContext(budget int) (RecallResult, error) {
	if budget <= 0 {
		budget = defaultPrimeBudget
	}

	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
		FROM observations o
		WHERE o.archived = 0 AND o.superseded_by IS NULL
	`)
	if err != nil {
		return RecallResult{}, fmt.Errorf("error al listar observaciones para priming: %w", err)
	}
	cands, err := scanCandidates(rows)
	rows.Close()
	if err != nil {
		return RecallResult{}, err
	}

	if len(cands) == 0 {
		return RecallResult{Budget: budget, Items: []RecallItem{}}, nil
	}

	// Rankear por saliencia (determinista, sin LLM) y empaquetar con el núcleo
	// compartido packByBudget (mismo estimador y lógica de presupuesto que el recall).
	now := time.Now().UTC()
	ranked := make([]scoredCandidate, len(cands))
	for i, c := range cands {
		ranked[i] = scoredCandidate{candidate: c, score: candidateSalience(c, now)}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	return packByBudget(ranked, budget, defaultGistMaxTokens), nil
}

// candidateSalience calcula la saliencia de un candidato usando la edad derivada
// de last_accessed (o created_at). Reusa la función salience del olvido para que
// priming y decay coincidan en el criterio de "qué memoria importa".
func candidateSalience(c candidate, now time.Time) float64 {
	imp := c.importance
	if imp <= 0 {
		imp = 1.0
	}
	ts := effectiveRecency(c)
	t, err := time.Parse(sqliteTimeLayout, ts)
	if err != nil {
		// Sin timestamp parseable: edad 0 (recencia máxima), no penalizar.
		return salience(imp, c.accessCount, 0, defaultHalfLifeDays)
	}
	ageDays := now.Sub(t).Hours() / 24
	return salience(imp, c.accessCount, ageDays, defaultHalfLifeDays)
}
