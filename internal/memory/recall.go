package memory

import (
	"context"
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
	TokenBudget   int  // techo de tokens del payload devuelto
	CandidatePool int  // candidatos a rankear antes de empaquetar
	GistMaxTokens int  // tope de un gist generado al vuelo
	NoBump        bool // si true, no actualiza stats de acceso (recall read-only)
	// QueryVector, si no es vacío, activa el recall HÍBRIDO (T5.7 R2): suma un pool de
	// candidatos por similitud vectorial (coseno) al pool léxico (FTS), unidos por id, y
	// agrega una 4ta señal RRF por rango vectorial. Lo computa la capa MCP con el embedder.
	// Vacío ⇒ recall 100% léxico (idéntico al histórico).
	QueryVector []float32
}

// RecallItem es un resultado compacto: gist + metadatos para decidir si hidratar.
type RecallItem struct {
	ID          string  `json:"id"`
	TopicKey    string  `json:"topic_key"`
	Gist        string  `json:"gist"`
	Score       float64 `json:"score"`
	FullTokens  int     `json:"full_tokens"`  // costo de hidratar el contenido completo
	ContentHash string  `json:"content_hash"` // huella del contenido (para inyección diferencial)
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
	contentHash  string
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
func (e *DbEngine) Recall(ctx context.Context, query string, opts RecallOptions) (RecallResult, error) {
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

	cands, lexRank, err := e.recallCandidates(ctx, query, pool)
	if err != nil {
		return RecallResult{}, err
	}

	// Recall híbrido (T5.7 R2): si hay vector de query, unir el pool vectorial por id (trae
	// también semánticamente-relacionadas que el léxico no encontró) y rankear por coseno.
	var vecRank map[string]int
	if len(opts.QueryVector) > 0 {
		cands, vecRank, err = e.augmentWithVectorPool(ctx, cands, opts.QueryVector, pool)
		if err != nil {
			return RecallResult{}, err
		}
	}

	result := RecallResult{Budget: budget, Items: []RecallItem{}}
	if len(cands) == 0 {
		return result, nil
	}

	// El ranking keyword (lexRank) solo existe si la query tuvo términos FTS; sin ellos
	// (fallback por recencia) es nil y se omite, para no doble-contar la recencia. vecRank
	// solo existe en recall híbrido.
	scored := scoreCandidates(cands, lexRank, vecRank)

	result = packByBudget(scored, budget, gistMax)

	// Recall read-only (ej. inyección por turno): no contar como acceso para no
	// distorsionar el ranking por frecuencia con accesos que el agente no pidió.
	if opts.NoBump {
		return result, nil
	}
	chosen := make([]string, 0, len(result.Items))
	for _, it := range result.Items {
		chosen = append(chosen, it.ID)
	}
	if err := e.bumpAccess(ctx, chosen); err != nil {
		return result, err
	}
	return result, nil
}

// packByBudget empaqueta gists en orden de score hasta llenar budget tokens,
// garantizando el top-1 (truncado si hace falta). Es el núcleo compartido por el
// recall por query y el priming de arranque: un único lugar donde vive la lógica
// de presupuesto y el estimador de tokens. Determinista, sin LLM.
func packByBudget(ranked []scoredCandidate, budget, gistMax int) RecallResult {
	result := RecallResult{Budget: budget, Items: []RecallItem{}}
	for _, c := range ranked {
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
			ID:          c.id,
			TopicKey:    c.topicKey,
			Gist:        gist,
			Score:       c.score,
			FullTokens:  c.fullTokens,
			ContentHash: c.contentHash,
		})
		result.UsedTokens += cost
		if result.UsedTokens >= budget {
			break
		}
	}
	result.Count = len(result.Items)
	return result
}

// scoreCandidates fusiona rankings (relevancia keyword, recencia, frecuencia) vía RRF y
// pondera por importancia. Determinista, sin LLM. Los rankings por pool se pasan como mapas
// id→posición (0 = mejor): un candidato ausente de un pool simplemente no suma ese término.
// lexRank es el ranking keyword (FTS) y vecRank el ranking vectorial (coseno); cada uno
// nil ⇒ se omite ese término. Con solo lexRank (NoopProvider) el resultado es idéntico al
// histórico; vecRank lo activa el recall híbrido (T5.7 R2).
func scoreCandidates(cands []candidate, lexRank, vecRank map[string]int) []scoredCandidate {
	n := len(cands)

	recencyRank := rankBy(cands, func(a, b candidate) bool {
		return effectiveRecency(a) > effectiveRecency(b)
	})
	freqRank := rankBy(cands, func(a, b candidate) bool {
		return a.accessCount > b.accessCount
	})

	out := make([]scoredCandidate, n)
	for i, c := range cands {
		rrf := 1.0/float64(rrfK+recencyRank[c.id]) +
			1.0/float64(rrfK+freqRank[c.id])
		if r, ok := lexRank[c.id]; ok {
			rrf += 1.0 / float64(rrfK+r)
		}
		if r, ok := vecRank[c.id]; ok {
			rrf += 1.0 / float64(rrfK+r)
		}
		imp := c.importance
		if imp <= 0 {
			imp = 1.0
		}
		out[i] = scoredCandidate{candidate: c, score: rrf * imp}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// augmentWithVectorPool une al pool léxico (cands) el pool por similitud vectorial: rankea
// por coseno (SearchObservations), trae el candidate completo de los ids que el léxico no
// tenía (union, no intersección) y devuelve el ranking vectorial (id→posición). Best-effort
// sobre el universo de candidatos: si no hay resultados vectoriales, deja cands intacto.
func (e *DbEngine) augmentWithVectorPool(ctx context.Context, cands []candidate, queryVec []float32, limit int) ([]candidate, map[string]int, error) {
	results, err := e.SearchObservations(ctx, queryVec, limit)
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
	vecRank := make(map[string]int, len(results))
	var missing []string
	for i, r := range results {
		vecRank[r.ID] = i
		if !have[r.ID] {
			missing = append(missing, r.ID)
		}
	}
	if len(missing) > 0 {
		extra, err := e.candidatesByIDs(ctx, missing)
		if err != nil {
			return cands, nil, err
		}
		cands = append(cands, extra...)
	}
	return cands, vecRank, nil
}

// candidatesByIDs trae los candidatos vivos (no archivados ni superseded) para los ids
// dados, con las mismas columnas que scanCandidates. Trocea el IN(...) por el tope de
// parámetros de SQLite. El orden del slice no importa: el ranking va por mapas.
func (e *DbEngine) candidatesByIDs(ctx context.Context, ids []string) ([]candidate, error) {
	var out []candidate
	for _, chunk := range chunkStrings(ids, maxSQLParams) {
		ph := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			ph[i] = "?"
			args[i] = id
		}
		rows, err := e.db.QueryContext(ctx, `
			SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
			       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
			FROM observations o
			WHERE o.archived = 0 AND o.superseded_by IS NULL AND o.id IN (`+strings.Join(ph, ",")+`)
		`, args...)
		if err != nil {
			return nil, fmt.Errorf("error al traer candidatos del pool vectorial: %w", err)
		}
		part, err := scanCandidates(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
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

// recallCandidates obtiene candidatos por FTS (ordenados por rank) y su ranking keyword
// (lexRank, id→posición). Si la query no tiene términos utilizables, cae a las observaciones
// más recientes y devuelve lexRank=nil (no hay señal keyword). Devolver el ranking acá (en
// vez de derivarlo del orden del slice al scorear) es lo que deja unir varios pools sin
// ambigüedad de rangos (T5.7).
func (e *DbEngine) recallCandidates(ctx context.Context, query string, limit int) ([]candidate, map[string]int, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		cands, err := e.recentCandidates(ctx, limit)
		return cands, nil, err
	}
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND o.archived = 0 AND o.superseded_by IS NULL
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("error en recall (FTS): %w", err)
	}
	defer rows.Close()
	cands, err := scanCandidates(rows)
	if err != nil {
		return nil, nil, err
	}
	lexRank := make(map[string]int, len(cands))
	for i, c := range cands {
		lexRank[c.id] = i
	}
	return cands, lexRank, nil
}

// recentCandidates devuelve las observaciones más recientes (fallback sin query).
func (e *DbEngine) recentCandidates(ctx context.Context, limit int) ([]candidate, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance
		FROM observations o
		WHERE o.archived = 0 AND o.superseded_by IS NULL
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
		if err := rows.Scan(&c.id, &c.topicKey, &c.gist, &c.content, &c.contentHash, &c.fullTokens,
			&c.createdAt, &c.lastAccessed, &c.accessCount, &c.importance); err != nil {
			return nil, fmt.Errorf("error al escanear candidato: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar candidatos: %w", err)
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

// ftsStopwords son términos muy frecuentes (es/en) que no aportan señal de recall y solo
// diluyen el OR del MATCH. Lista corta y determinista (model-free).
var ftsStopwords = map[string]bool{
	// Español
	"el": true, "la": true, "los": true, "las": true, "un": true, "una": true, "unos": true,
	"unas": true, "de": true, "del": true, "al": true, "en": true, "con": true, "por": true,
	"para": true, "que": true, "como": true, "su": true, "sus": true,
	// Inglés
	"the": true, "an": true, "of": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "with": true, "and": true, "or": true, "is": true, "are": true, "be": true,
	"by": true, "as": true, "it": true,
}

// buildFTSQueryRanked es como buildFTSQuery pero descarta el ruido que diluye el OR:
// stopwords (lista determinista) y tokens de una sola runa (p. ej. la 'N' y el '1' de
// 'N+1'). Preserva entidades cortas significativas como 'Go', 'DB', 'API' (>= 2 runas y no
// stopwords). Si tras filtrar no queda nada (consulta toda de ruido), cae a buildFTSQuery
// para no perder recall. Proxy de IDF: lo corto/frecuente pesa menos.
func buildFTSQueryRanked(q string) string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) <= 1 || ftsStopwords[strings.ToLower(f)] {
			continue
		}
		terms = append(terms, `"`+f+`"`)
	}
	if len(terms) == 0 {
		return buildFTSQuery(q) // fallback: no perder recall si todo era ruido
	}
	return strings.Join(terms, " OR ")
}

// bumpAccess actualiza recencia y frecuencia de las observaciones devueltas.
func (e *DbEngine) bumpAccess(ctx context.Context, ids []string) error {
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
	if _, err := e.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("error al actualizar stats de acceso: %w", err)
	}
	return nil
}
