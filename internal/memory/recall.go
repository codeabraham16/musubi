package memory

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// recall.go implementa el recall por PRESUPUESTO de tokens, 100% model-free.
// El agente pide "lo más útil que entre en N tokens"; el server rankea por
// fusión RRF (relevancia keyword + recencia + frecuencia, ponderada por
// importancia) y devuelve GISTS hasta llenar el presupuesto. El contenido
// completo se trae aparte con GetObservations (hidratación perezosa).

const (
	defaultRecallBudget  = 400
	defaultCandidatePool = 50
	// rrfK es la constante de Reciprocal Rank Fusion (estándar ~60).
	rrfK = 60
)

// RecallOptions configura un recall. Los ceros usan los defaults.
type RecallOptions struct {
	TokenBudget   int // techo de tokens del payload devuelto
	CandidatePool int // candidatos a rankear antes de empaquetar
	GistMaxTokens int // tope de un gist generado al vuelo
}

// RecallItem es un resultado compacto: gist + metadatos para decidir si hidratar.
type RecallItem struct {
	ID         string  `json:"id"`
	TopicKey   string  `json:"topic_key"`
	Gist       string  `json:"gist"`
	Score      float64 `json:"score"`
	FullTokens int     `json:"full_tokens"` // costo de hidratar el contenido completo
}

// RecallResult es la respuesta del recall, con presupuesto y consumo reales.
type RecallResult struct {
	Budget     int          `json:"budget"`
	UsedTokens int          `json:"used_tokens"`
	Count      int          `json:"count"`
	Items      []RecallItem `json:"items"`
}

type candidate struct {
	id           string
	topicKey     string
	gist         string
	content      string
	fullTokens   int
	createdAt    string
	lastAccessed string
	accessCount  int
	importance   float64
}

type scoredCandidate struct {
	candidate
	score float64
}

// Recall devuelve los gists más útiles para query que entren en TokenBudget.
func (e *DbEngine) Recall(query string, opts RecallOptions) (RecallResult, error) {
	budget := opts.TokenBudget
	if budget <= 0 {
		budget = defaultRecallBudget
	}
	pool := opts.CandidatePool
	if pool <= 0 {
		pool = defaultCandidatePool
	}
	gistMax := opts.GistMaxTokens
	if gistMax <= 0 {
		gistMax = defaultGistMaxTokens
	}

	cands, err := e.recallCandidates(query, pool)
	if err != nil {
		return RecallResult{}, err
	}

	result := RecallResult{Budget: budget, Items: []RecallItem{}}
	if len(cands) == 0 {
		return result, nil
	}

	scored := scoreCandidates(cands)

	var chosen []string
	for _, c := range scored {
		gist := c.gist
		if strings.TrimSpace(gist) == "" {
			gist = Gist(c.content, gistMax)
		}
		cost := EstimateTokens(gist)

		// Garantizar al menos el top-1, truncando su gist si excede el presupuesto.
		if len(result.Items) == 0 && cost > budget {
			gist = truncateToTokens(gist, budget)
			cost = EstimateTokens(gist)
		} else if result.UsedTokens+cost > budget {
			continue // no entra; probamos con el siguiente (puede ser más chico)
		}

		result.Items = append(result.Items, RecallItem{
			ID:         c.id,
			TopicKey:   c.topicKey,
			Gist:       gist,
			Score:      c.score,
			FullTokens: c.fullTokens,
		})
		result.UsedTokens += cost
		chosen = append(chosen, c.id)

		if result.UsedTokens >= budget {
			break
		}
	}
	result.Count = len(result.Items)

	if err := e.bumpAccess(chosen); err != nil {
		return result, err
	}
	return result, nil
}

// scoreCandidates fusiona tres rankings (relevancia keyword, recencia, frecuencia)
// vía RRF y pondera por importancia. Determinista, sin LLM.
func scoreCandidates(cands []candidate) []scoredCandidate {
	n := len(cands)

	// Ranking keyword: el orden de entrada ya viene por rank de FTS.
	keywordRank := make(map[string]int, n)
	for i, c := range cands {
		keywordRank[c.id] = i
	}

	recencyRank := rankBy(cands, func(a, b candidate) bool {
		return effectiveRecency(a) > effectiveRecency(b)
	})
	freqRank := rankBy(cands, func(a, b candidate) bool {
		return a.accessCount > b.accessCount
	})

	out := make([]scoredCandidate, n)
	for i, c := range cands {
		rrf := 1.0/float64(rrfK+keywordRank[c.id]) +
			1.0/float64(rrfK+recencyRank[c.id]) +
			1.0/float64(rrfK+freqRank[c.id])
		imp := c.importance
		if imp <= 0 {
			imp = 1.0
		}
		out[i] = scoredCandidate{candidate: c, score: rrf * imp}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// rankBy devuelve, para cada id, su posición (0 = mejor) según el orden less.
func rankBy(cands []candidate, less func(a, b candidate) bool) map[string]int {
	ordered := make([]candidate, len(cands))
	copy(ordered, cands)
	sort.SliceStable(ordered, func(i, j int) bool { return less(ordered[i], ordered[j]) })
	ranks := make(map[string]int, len(ordered))
	for i, c := range ordered {
		ranks[c.id] = i
	}
	return ranks
}

// effectiveRecency usa last_accessed si existe, si no created_at (ISO8601 ordena
// lexicográficamente).
func effectiveRecency(c candidate) string {
	if strings.TrimSpace(c.lastAccessed) != "" {
		return c.lastAccessed
	}
	return c.createdAt
}

// recallCandidates obtiene candidatos por FTS (ordenados por rank). Si la query
// no tiene términos utilizables, cae a las observaciones más recientes.
func (e *DbEngine) recallCandidates(query string, limit int) ([]candidate, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return e.recentCandidates(limit)
	}
	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND o.archived = 0
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("error en recall (FTS): %w", err)
	}
	defer rows.Close()
	return scanCandidates(rows)
}

// recentCandidates devuelve las observaciones más recientes (fallback sin query).
func (e *DbEngine) recentCandidates(limit int) ([]candidate, error) {
	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
		FROM observations o
		WHERE o.archived = 0
		ORDER BY COALESCE(o.last_accessed, o.created_at) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("error en recall (recientes): %w", err)
	}
	defer rows.Close()
	return scanCandidates(rows)
}

func scanCandidates(rows *sql.Rows) ([]candidate, error) {
	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.topicKey, &c.gist, &c.content, &c.fullTokens,
			&c.createdAt, &c.lastAccessed, &c.accessCount, &c.importance); err != nil {
			return nil, fmt.Errorf("error al escanear candidato: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// buildFTSQuery sanea la consulta del usuario para FTS5: extrae términos
// alfanuméricos, los entrecomilla y los une con OR (evita errores de sintaxis y
// maximiza el recall de candidatos).
func buildFTSQuery(q string) string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		terms = append(terms, `"`+f+`"`)
	}
	return strings.Join(terms, " OR ")
}

// bumpAccess actualiza recencia y frecuencia de las observaciones devueltas.
func (e *DbEngine) bumpAccess(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `UPDATE observations
	      SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1
	      WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := e.db.Exec(q, args...); err != nil {
		return fmt.Errorf("error al actualizar stats de acceso: %w", err)
	}
	return nil
}
