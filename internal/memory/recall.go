package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"musubi/internal/logx"
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
	// GraphCentrality, si es true, agrega la 5ª señal RRF (B4): centralidad de grafo por
	// Personalized PageRank sobre observation_relations (HippoRAG), que favorece las
	// observaciones más CENTRALES en la telaraña semántica de la memoria. Es un RERANK del
	// pool existente (no incorpora candidatos nuevos). El zero-value (false) preserva el
	// comportamiento histórico bit-a-bit; la capa MCP lo enciende según config (default ON).
	GraphCentrality bool
	// Stemming, si es true, activa el match por PREFIJO de raíz en el FTS (2ª ola de #2): la
	// query 'deploy' matchea también 'deploys'/'deployment' (variantes morfológicas de sufijo),
	// sin re-indexar ni dependencia. El zero-value (false) usa el match exacto histórico; la capa
	// MCP lo enciende según config (default ON).
	Stemming bool
	// Cooccurrence, si es true, agrega la 6ª señal RRF (Track 14 #2, semántica model-free):
	// expansión por pseudo-relevance feedback (PRF) — cosecha términos que co-ocurren con la
	// query en los top resultados y corre un 2º FTS para traer observaciones con vocabulario
	// distinto (puente 'deploy'↔'despliegue'), derivado del corpus. AUGMENTA el pool. El
	// zero-value (false) preserva el comportamiento histórico; la capa MCP lo enciende según
	// config (default ON).
	Cooccurrence bool
	// RankedFTS, si es true, filtra el RUIDO del MATCH de FTS antes de armarlo: descarta
	// stopwords (es/en) y tokens de 1 runa, que solo diluyen el OR y dejan que la recencia
	// vuelque el orden. Lo usa el recall POR TURNO (la superficie más caliente), que antes
	// corría FTS crudo. El zero-value (false) preserva el recall histórico del tool bit-a-bit.
	RankedFTS bool
	// ProjectScope activa el AISLAMIENTO por proyecto (Track 16 F1 16.1b): si no es vacío y
	// Federate es false, el recall descarta los candidatos de OTROS proyectos (se conservan
	// los del proyecto pedido y los sin atribuir). El zero-value ("") NO filtra: comportamiento
	// histórico (federado) bit-a-bit. El enforcement por defecto lo cablea la identidad (16.1c).
	ProjectScope string
	// Federate, si es true, IGNORA ProjectScope y devuelve memoria de todos los proyectos
	// (recall federado explícito). Es el opt-in del modelo "aislado + federación opt-in".
	Federate bool
	// VectorFloor es el piso de coseno (0..1) del pool vectorial del recall híbrido (Q1): los
	// candidatos con similitud < VectorFloor se descartan ANTES de entrar al ranking, para no
	// inyectar vecinos de baja señal con peso RRF pleno. <= 0 ⇒ sin piso (histórico bit-a-bit).
	// Lo cablea la capa MCP desde config (default 0.30).
	VectorFloor float64
}

// RecallItem es un resultado compacto: gist + metadatos para decidir si hidratar.
type RecallItem struct {
	ID         string  `json:"id"`
	TopicKey   string  `json:"topic_key"`
	Gist       string  `json:"gist"`
	Score      float64 `json:"score"`
	FullTokens int     `json:"full_tokens"` // costo de hidratar el contenido completo
	// Author es la atribución por PERSONA (C5.1): quién aportó la memoria. omitempty ⇒ no ensucia
	// la respuesta cuando no hay atribución (captura local/legacy/stdio).
	Author string `json:"author,omitempty"`
	// ContentHash es maquinaria server-side (la inyección diferencial la consume in-process
	// en Go): json:"-" para NO enviar 64 hex de ruido al modelo en la respuesta del tool.
	ContentHash string `json:"-"`
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
	projectID    string // atribución (F1): proyecto de origen; "" = sin atribuir
	author       string // atribución por PERSONA (C5.1): quién aportó la memoria; "" = sin atribuir
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

	cands, lexRank, err := e.recallCandidates(ctx, query, pool, opts.Stemming, opts.RankedFTS)
	if err != nil {
		// Degradación elegante (Q2): un FTS corrupto NO debe tumbar TODO el recall si hay un pool
		// vectorial servible. Ante corrupción del índice, logear y seguir con pool léxico vacío
		// (el vectorial y/o el fallback llenan); cualquier otro error se propaga (acota el rescate
		// a la clase corrupción, para no enmascarar fallos reales).
		if !isFTSCorruption(err) {
			return RecallResult{}, err
		}
		logx.Warn("recall: FTS corrupto, degradando a pool no-léxico", "error", err)
		cands, lexRank = nil, nil
	}

	// Recall híbrido (T5.7 R2): si hay vector de query, unir el pool vectorial por id (trae
	// también semánticamente-relacionadas que el léxico no encontró) y rankear por coseno.
	var vecRank map[string]int
	if len(opts.QueryVector) > 0 {
		cands, vecRank, err = e.augmentWithVectorPool(ctx, cands, opts.QueryVector, pool, opts.VectorFloor)
		if err != nil {
			return RecallResult{}, err
		}
	}

	// Co-ocurrencia / PRF (Track 14 #2): 6ª señal RRF opcional, semántica model-free derivada del
	// corpus. Corre TRAS la augmentación vectorial (para expandir sobre el mejor pool léxico) y
	// ANTES de la centralidad de grafo (para que el grafo vea el pool ya expandido). Sólo si hubo
	// query FTS (lexRank != nil) y hay ≥2 candidatos. No-op seguro ⇒ coocRank vacío ⇒ equivalencia.
	var coocRank map[string]int
	if opts.Cooccurrence && lexRank != nil && len(cands) >= 2 {
		cands, coocRank, err = e.augmentWithCooccurrencePool(ctx, cands, query, lexRank, pool)
		if err != nil {
			return RecallResult{}, err
		}
	}

	// Aislamiento por proyecto (Track 16 F1 16.1b): CHOKE POINT único. Todos los pools
	// (léxico, vectorial, co-ocurrencia) confluyen en `cands`; filtrar acá cubre todos de una
	// vez, antes del grafo y el scoring. Scope vacío o Federate ⇒ NO filtra (federado histórico
	// bit-a-bit). Se conservan el proyecto pedido y las filas sin atribuir (project_id vacío).
	if !opts.Federate && opts.ProjectScope != "" {
		cands = filterCandidatesByProject(cands, opts.ProjectScope)
	}

	result := RecallResult{Budget: budget, Items: []RecallItem{}}
	if len(cands) == 0 {
		return result, nil
	}

	// Centralidad de grafo (B4): 5ª señal RRF opcional. Se computa sobre el pool YA armado
	// (léxico + augmentación vectorial) para que la difusión vea todos los candidatos, y es
	// rerank-only (no agrega ids nuevos). No-op seguro cuando no aporta (grafo vacío, <2
	// candidatos en el grafo) ⇒ graphRank vacío ⇒ score idéntico al histórico.
	var graphRank map[string]int
	if opts.GraphCentrality && len(cands) >= 2 {
		ids := make([]string, len(cands))
		for i, c := range cands {
			ids[i] = c.id
		}
		graphRank, err = e.graphCentralityRank(ids)
		if err != nil {
			return RecallResult{}, err
		}
	}

	// El ranking keyword (lexRank) solo existe si la query tuvo términos FTS; sin ellos
	// (fallback por recencia) es nil y se omite, para no doble-contar la recencia. vecRank
	// solo existe en recall híbrido; graphRank solo con GraphCentrality on.
	scored := scoreCandidates(cands, lexRank, vecRank, graphRank, coocRank)

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
			Author:      c.author,
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
// lexRank es el ranking keyword (FTS), vecRank el ranking vectorial (coseno), graphRank el de
// centralidad de grafo (PPR sobre observation_relations) y coocRank el de expansión por
// co-ocurrencia/PRF; cada uno nil ⇒ se omite ese término. Con solo lexRank (NoopProvider) el
// resultado es idéntico al histórico; vecRank lo activa el recall híbrido (T5.7 R2), graphRank la
// centralidad de grafo (B4) y coocRank la semántica model-free por co-ocurrencia (Track 14 #2).
func scoreCandidates(cands []candidate, lexRank, vecRank, graphRank, coocRank map[string]int) []scoredCandidate {
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
		if r, ok := graphRank[c.id]; ok {
			rrf += 1.0 / float64(rrfK+r)
		}
		if r, ok := coocRank[c.id]; ok {
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
func (e *DbEngine) augmentWithVectorPool(ctx context.Context, cands []candidate, queryVec []float32, limit int, floor float64) ([]candidate, map[string]int, error) {
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
	rank := 0
	for _, r := range results {
		// Piso de coseno (Q1): descartar los vecinos de baja señal ANTES de rankearlos, para no
		// inyectarlos al pool con peso RRF pleno (0.42 pesando igual que 0.95). results viene
		// ordenado por Similarity desc (SearchObservations), así que saltear los de baja sim NO
		// altera el rango relativo de los que sobreviven. floor <= 0 ⇒ sin piso (histórico).
		if floor > 0 && float64(r.Similarity) < floor {
			continue
		}
		vecRank[r.ID] = rank
		rank++
		if !have[r.ID] {
			missing = append(missing, r.ID)
		}
	}
	if len(vecRank) == 0 {
		// Todos los vecinos cayeron bajo el piso: sin señal vectorial, equivalente a no tener
		// resultados (no se agregan candidatos ni término RRF).
		return cands, nil, nil
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

// isFTSCorruption indica si err es un error de CORRUPCIÓN del índice/base (SQLITE_CORRUPT o
// FTS5 malformado). El driver (modernc/sqlite) no expone un código tipado estable acá, así que
// se reconoce por el texto del mensaje. Acota la degradación elegante del recall (Q2) a la clase
// corrupción, para no tragar silenciosamente otros errores (contexto cancelado, etc.).
func isFTSCorruption(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "corrupt") ||
		strings.Contains(msg, "malformed") ||
		strings.Contains(msg, "database disk image")
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
			       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance, COALESCE(o.project_id,''), COALESCE(o.author,'')
			FROM observations o
			WHERE `+visibleObsPredicate+` AND o.id IN (`+strings.Join(ph, ",")+`)
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

// filterCandidatesByProject conserva los candidatos del proyecto `scope` y los SIN atribuir
// (project_id vacío: legacy y locales sin estampar, que no son de "otro" proyecto). Es el
// aislamiento por proyecto del recall (16.1b). scope=="" no debería llegar acá (el caller
// ya cortocircuita), pero por robustez devuelve todo sin filtrar.
func filterCandidatesByProject(cands []candidate, scope string) []candidate {
	if scope == "" {
		return cands
	}
	out := cands[:0]
	for _, c := range cands {
		if c.projectID == scope || c.projectID == "" {
			out = append(out, c)
		}
	}
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

// prefixSuffixes es la lista CURADA y corta de sufijos de flexión (ES+EN) que stemForPrefix
// recorta para acercar un término a su raíz. Ordenada por longitud DESC (se recorta el primero que
// matchee). Conservadora a propósito: sólo sufijos seguros, nunca vocales/acentos sueltos.
var prefixSuffixes = []string{
	"aciones", "ciones", "mientos", "iendo", "mente", "ación", "acion",
	"miento", "ments", "ando", "ados", "idos", "ment", "ado", "ido",
	"ing", "ar", "er", "ir", "es", "ed", "s",
}

// stemForPrefix reduce un término a una raíz para el match por prefijo del FTS (2ª ola de #2).
// Determinista y CONSERVADOR: lowercase; términos de <5 runas quedan intactos; si no, recorta el
// primer sufijo de prefixSuffixes que deje una raíz de ≥4 runas; si ninguno aplica, devuelve el
// término. Un solo sufijo por término — acota el over-stemming; el prefijo FTS hace el resto.
func stemForPrefix(term string) string {
	t := strings.ToLower(term)
	r := []rune(t)
	if len(r) < 5 {
		return t
	}
	for _, suf := range prefixSuffixes {
		sr := []rune(suf)
		if len(r)-len(sr) >= 4 && strings.HasSuffix(t, suf) {
			return string(r[:len(r)-len(sr)])
		}
	}
	return t
}

// buildFTSQueryPrefix construye una query FTS5 por PREFIJO: cada término se stemmea (stemForPrefix)
// y se emite como '"stem"*' (prefijo verificado en FTS5/modernc), unidos por OR. Atrapa las
// variantes morfológicas de sufijo (deploy/deploys/deployment) sin re-indexar. Vacío ⇒ "".
func buildFTSQueryPrefix(q string) string {
	terms := splitTerms(q)
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = append(out, `"`+stemForPrefix(t)+`"*`)
	}
	return strings.Join(out, " OR ")
}

// recallCandidates obtiene candidatos por FTS (ordenados por rank) y su ranking keyword
// (lexRank, id→posición). Si la query no tiene términos utilizables, cae a las observaciones
// más recientes y devuelve lexRank=nil (no hay señal keyword). Devolver el ranking acá (en
// vez de derivarlo del orden del slice al scorear) es lo que deja unir varios pools sin
// ambigüedad de rangos (T5.7).
func (e *DbEngine) recallCandidates(ctx context.Context, query string, limit int, stemming, ranked bool) ([]candidate, map[string]int, error) {
	var ftsQuery string
	switch {
	case stemming && ranked:
		ftsQuery = buildFTSQueryRankedPrefix(query) // sin stopwords + prefijo de la raíz
	case stemming:
		ftsQuery = buildFTSQueryPrefix(query) // 2ª ola de #2: match por prefijo de la raíz
	case ranked:
		ftsQuery = buildFTSQueryRanked(query) // sin stopwords ni tokens de 1 runa
	default:
		ftsQuery = buildFTSQuery(query)
	}
	if ftsQuery == "" {
		cands, err := e.recentCandidates(ctx, limit)
		return cands, nil, err
	}
	cands, err := e.ftsSearch(ctx, ftsQuery, limit)
	if err != nil {
		return nil, nil, err
	}
	lexRank := make(map[string]int, len(cands))
	for i, c := range cands {
		lexRank[c.id] = i
	}
	return cands, lexRank, nil
}

// ftsSearch corre una MATCH de FTS5 ya construida (ftsQuery) sobre las observaciones vivas y
// devuelve los candidatos en orden de `rank` (mejor primero). Es el núcleo compartido por el
// recall léxico (recallCandidates) y la expansión por co-ocurrencia (augmentWithCooccurrencePool).
func (e *DbEngine) ftsSearch(ctx context.Context, ftsQuery string, limit int) ([]candidate, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance, COALESCE(o.project_id,''), COALESCE(o.author,'')
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND `+visibleObsPredicate+`
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
func (e *DbEngine) recentCandidates(ctx context.Context, limit int) ([]candidate, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance, COALESCE(o.project_id,''), COALESCE(o.author,'')
		FROM observations o
		WHERE `+visibleObsPredicate+`
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
			&c.createdAt, &c.lastAccessed, &c.accessCount, &c.importance, &c.projectID, &c.author); err != nil {
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

// rankedTerms extrae los términos "con señal" de q: descarta stopwords (es/en) y tokens de
// una sola runa (p. ej. la 'N'/'1' de 'N+1'), preservando entidades cortas como 'Go'/'DB'/
// 'API' (>= 2 runas y no stopwords). Si tras filtrar no queda nada (consulta toda de ruido),
// devuelve los términos crudos para no perder recall. Proxy de IDF, determinista.
func rankedTerms(q string) []string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) <= 1 || ftsStopwords[strings.ToLower(f)] {
			continue
		}
		terms = append(terms, f)
	}
	if len(terms) == 0 {
		return fields // fallback: no perder recall si todo era ruido
	}
	return terms
}

// buildFTSQueryRanked arma un MATCH de FTS5 con los términos con señal (sin stopwords ni
// tokens de 1 runa), entrecomillados y unidos por OR. Vacío ⇒ "".
func buildFTSQueryRanked(q string) string {
	terms := rankedTerms(q)
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = append(out, `"`+t+`"`)
	}
	return strings.Join(out, " OR ")
}

// buildFTSQueryRankedPrefix combina el filtrado de ruido (rankedTerms) con el match por
// PREFIJO de la raíz (stemForPrefix): '"stem"*' OR ... — evita que un stopword, como prefijo,
// matchee medio corpus. Es el builder del recall por turno con stemming. Vacío ⇒ "".
func buildFTSQueryRankedPrefix(q string) string {
	terms := rankedTerms(q)
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = append(out, `"`+stemForPrefix(t)+`"*`)
	}
	return strings.Join(out, " OR ")
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
