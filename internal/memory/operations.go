package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"musubi/internal/logx"
	"musubi/internal/redact"

	"github.com/google/uuid"
)

type Observation struct {
	ID        string `json:"id"`
	TopicKey  string `json:"topic_key"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type SearchResult struct {
	Observation
	Similarity float32 `json:"similarity"`
}

// CountObservations devuelve el total de observaciones guardadas.
func (e *DbEngine) CountObservations() (int, error) {
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n); err != nil {
		return 0, fmt.Errorf("error al contar observaciones: %w", err)
	}
	return n, nil
}

// CountSavedItems devuelve el total de items persistidos en las TRES superficies de
// memoria: observaciones + hechos (relations) + gists de código (code_memory). Lo usa el
// loop dirigido como señal model-free de "se guardó algo" entre turnos (recordatorio de
// captura): contar solo observaciones daba falsos positivos porque save_fact y save_code
// no las incrementan. Cuenta filas totales (relations incluye invalidadas) para que la
// señal sea MONÓTONA ante cada save nuevo, incluso cuando un hecho supersede a otro.
func (e *DbEngine) CountSavedItems() (int, error) {
	var n int
	if err := e.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM observations) +
		(SELECT COUNT(*) FROM relations) +
		(SELECT COUNT(*) FROM code_memory)`).Scan(&n); err != nil {
		return 0, fmt.Errorf("error al contar items guardados: %w", err)
	}
	return n, nil
}

// ObsCard es una memoria en forma LEGIBLE para humanos (dashboard/observabilidad):
// su tema, el resumen (gist), cuándo se guardó y su importancia. Read-only.
type ObsCard struct {
	TopicKey   string  `json:"topic_key"`
	Gist       string  `json:"gist"`
	CreatedAt  string  `json:"created_at"`
	Importance float64 `json:"importance"`
}

// RecentObservations devuelve las últimas observaciones NO archivadas (más nuevas
// primero) en forma legible, para los paneles "lo que Musubi recuerda" y "actividad
// reciente". Si una no tiene gist, cae a un recorte del contenido. limit<=0 usa 12.
func (e *DbEngine) RecentObservations(limit int) ([]ObsCard, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := e.db.Query(`
		SELECT topic_key,
		       COALESCE(NULLIF(gist, ''), substr(content, 1, 160)) AS summary,
		       COALESCE(created_at, ''),
		       COALESCE(importance, 1.0)
		FROM observations
		WHERE archived = 0
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("memorias recientes: %w", err)
	}
	defer rows.Close()
	var out []ObsCard
	for rows.Next() {
		var c ObsCard
		if err := rows.Scan(&c.TopicKey, &c.Gist, &c.CreatedAt, &c.Importance); err != nil {
			return nil, fmt.Errorf("memorias recientes: escanear: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveObservation inserta o actualiza una observación y su vector. Computa de
// forma model-free el gist, el content_hash y la estimación de tokens. La
// importancia no se toca en updates (se preserva la existente).
func (e *DbEngine) SaveObservation(id, topicKey, content string, embedding []float32) error {
	return e.saveObservation(id, topicKey, content, 1.0, false, "", "local", "", embedding)
}

// SaveObservationWithImportance es como SaveObservation pero fija la importancia
// (también en updates). importance pondera el ranking del recall.
func (e *DbEngine) SaveObservationWithImportance(id, topicKey, content string, importance float64, embedding []float32) error {
	return e.saveObservation(id, topicKey, content, importance, true, "", "local", "", embedding)
}

// SaveObservationTyped es como SaveObservationWithImportance pero además fija el TIPO de
// memoria (mem_type: semantic/episodic/procedural), que modula el olvido, y el SCOPE
// (local/shared) de la memoria híbrida. memType se normaliza al enum canónico
// (vacío/desconocido → sin tipo); scope vacío se trata como 'local'. Ver memtype.go/scope.go.
func (e *DbEngine) SaveObservationTyped(id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error {
	return e.SaveObservationTypedFrom("", id, topicKey, content, importance, memType, scope, embedding)
}

// SaveObservationTypedFrom es SaveObservationTyped con el project_id de ORIGEN explícito
// (atribución multi-tenant, Track 16 Fase 1). Lo usa el ingest del central para preservar
// el proyecto de la máquina que originó la observación, en vez de estampar el del propio
// central. originProjectID == "" ⇒ se usa el project_id del engine (comportamiento de
// siempre para los guardados locales).
func (e *DbEngine) SaveObservationTypedFrom(originProjectID, id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error {
	return e.saveObservation(id, topicKey, content, importance, true, memType, scope, originProjectID, embedding)
}

// SaveObservationDeduped guarda content con un id nuevo, salvo que ya exista una
// observación con el mismo content_hash (dedup exacto): en ese caso devuelve ese
// id y deduped=true sin insertar nada.
func (e *DbEngine) SaveObservationDeduped(topicKey, content string, importance float64, embedding []float32) (string, bool, error) {
	return e.SaveObservationDedupedTyped(topicKey, content, importance, "", "local", embedding)
}

// SaveObservationDedupedTyped es SaveObservationDeduped con TIPO de memoria (mem_type)
// y SCOPE (local/shared) de la memoria híbrida.
func (e *DbEngine) SaveObservationDedupedTyped(topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error) {
	return e.SaveObservationDedupedTypedFrom("", topicKey, content, importance, memType, scope, embedding)
}

// SaveObservationDedupedTypedFrom es SaveObservationDedupedTyped con el project_id de ORIGEN
// explícito (atribución multi-tenant, Track 16 Fase 1). originProjectID == "" ⇒ project_id
// del engine (comportamiento de siempre).
func (e *DbEngine) SaveObservationDedupedTypedFrom(originProjectID, topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error) {
	existing, found, err := e.FindByContentHash(ContentHash(content))
	if err != nil {
		return "", false, err
	}
	if found {
		return existing, true, nil
	}
	id := uuid.NewString()
	if err := e.SaveObservationTypedFrom(originProjectID, id, topicKey, content, importance, memType, scope, embedding); err != nil {
		return "", false, err
	}
	return id, false, nil
}

// FindByContentHash devuelve el id de la observación con ese content_hash, si existe.
func (e *DbEngine) FindByContentHash(hash string) (string, bool, error) {
	var id string
	err := e.db.QueryRow(`SELECT id FROM observations WHERE content_hash = ? LIMIT 1`, hash).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("error al buscar content_hash: %w", err)
	}
	return id, true, nil
}

// saveObservation es el núcleo del guardado: UPSERT por id que preserva created_at
// y las estadísticas de acceso en updates, y mantiene el FTS sincronizado vía
// triggers (AFTER INSERT/UPDATE). Si setImportance es false, no pisa importance.
func (e *DbEngine) saveObservation(id, topicKey, content string, importance float64, setImportance bool, memType, scope, originProjectID string, embedding []float32) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	// scope de la memoria híbrida: vacío ⇒ 'local' (privada). project_id se estampa desde
	// el engine (lo inyecta el entrypoint). En el UPSERT NO se pisan scope ni project_id:
	// un re-save por id preserva una promoción a 'shared' previa y la atribución original
	// (aditivo/backward-compat; F1 no sincroniza ni filtra por scope todavía).
	scope = normalizeScope(scope)

	// Redacción de secretos en el borde a 'shared' (C2): el scope EFECTIVO es el pasado, o el
	// ya almacenado para esta id — el UPSERT PRESERVA un 'shared' previo, así que un re-save por
	// vía 'local' de una fila ya shared igual queda shared. Se limpia ANTES de derivar
	// gist/hash/tokens para que el outbox —que reconstruye el payload desde esta fila— nunca
	// empuje un secreto al cerebro compartido, por ninguna ruta.
	if scope != ScopeShared {
		var stored string
		_ = tx.QueryRow(`SELECT scope FROM observations WHERE id = ?`, id).Scan(&stored)
		if stored == ScopeShared {
			scope = ScopeShared
		}
	}
	if scope == ScopeShared {
		content, _ = redact.Redact(content)
	}

	gist := Gist(content, defaultGistMaxTokens)
	hash := ContentHash(content)
	tokens := EstimateTokens(content)
	// mem_type se normaliza al enum canónico; "" (sin tipo) se guarda como cadena vacía y
	// pesa neutro en el olvido. El tipo lo aporta el agente (model-free).
	memType = normalizeMemType(memType)

	setImp := ""
	if setImportance {
		setImp = ",\n\t\t\timportance=excluded.importance"
	}
	// En UPSERT, un guardado SIN tipo (mem_type='') PRESERVA la clasificación existente:
	// sólo un tipo no vacío la reemplaza. Así un update por la vía histórica (untyped) no
	// borra el mem_type que otro guardado tipado ya fijó (evita pérdida de clasificación).
	queryObs := `INSERT INTO observations (id, topic_key, content, gist, content_hash, tokens, importance, mem_type, scope, project_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic_key=excluded.topic_key,
			content=excluded.content,
			gist=excluded.gist,
			content_hash=excluded.content_hash,
			tokens=excluded.tokens,
			mem_type=CASE WHEN excluded.mem_type != '' THEN excluded.mem_type ELSE observations.mem_type END` + setImp
	// Atribución (Track 16 F1): estampar el project_id de ORIGEN si el caller lo pasó
	// (ingest del central), o el del engine si no (guardado local). El UPSERT NO pisa
	// project_id en updates, así que un re-save no borra la atribución original.
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	if _, err = tx.Exec(queryObs, id, topicKey, content, gist, hash, tokens, importance, memType, scope, projectID); err != nil {
		return fmt.Errorf("error al guardar observación: %w", err)
	}

	// Si se provee embedding, guardarlo (keyed por observation_id).
	if len(embedding) > 0 {
		vectorBytes, err := Float32ToBytes(embedding)
		if err != nil {
			return fmt.Errorf("error al serializar vector: %w", err)
		}
		queryEmb := `INSERT OR REPLACE INTO embeddings (observation_id, vector) VALUES (?, ?)`
		if _, err = tx.Exec(queryEmb, id, vectorBytes); err != nil {
			return fmt.Errorf("error al guardar embedding: %w", err)
		}
	}

	// Cerebro híbrido F2: encolar en el OUTBOX dentro de la MISMA tx del UPSERT. El statement
	// es un no-op si la fila NO quedó 'shared' (el caso común 'local'): sólo las observaciones
	// compartidas se sincronizan al central. Idempotente por obs_id (re-save no duplica; sólo
	// re-encola si el content_hash cambió). Va antes del Commit para ser atómico con el guardado.
	if err := enqueueOutboxTx(tx, id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear observación: %w", err)
	}

	// Post-commit (SQLite ya es la verdad): mantener el índice IVF al día. Add es
	// O(C) y no toca disco; el rebuild eventual (throttled, en segundo plano) corrige
	// el drift de centroides. Sin embedding no hay nada que indexar.
	if e.index != nil && e.vindexCfg.Enabled && len(embedding) > 0 {
		e.index.Add(id, embedding)
		e.maybeRebuildVectorIndex()
	}

	return nil
}

// SearchObservations realiza una búsqueda semántica. Si el índice IVF está entrenado
// y la dimensión coincide, lo usa para ACOTAR los candidatos (sublineal) y luego
// rankea EXACTO sobre ellos; si no, cae al full-scan exacto de siempre. En ambos
// caminos el ranking final es coseno exacto y se re-filtra archived/superseded contra
// SQLite, así que el índice nunca compromete la correctitud (a lo sumo, el recall).
func (e *DbEngine) SearchObservations(ctx context.Context, queryEmbedding []float32, limit int) ([]SearchResult, error) {
	if e.index != nil && e.vindexCfg.Enabled {
		if ids, ok := e.index.Search(queryEmbedding, e.vindexCfg.NProbe); ok && len(ids) > 0 {
			return e.searchExactByIDs(ctx, queryEmbedding, ids, limit)
		}
	}
	return e.searchExactFullScan(ctx, queryEmbedding, limit)
}

// searchExactByIDs rankea exactamente por coseno SOLO el conjunto de ids candidato
// (las celdas IVF sondeadas), re-filtrando archived/superseded contra SQLite. El
// IN(...) se trocea para respetar el tope de parámetros de SQLite. Misma semántica de
// dim-mismatch (warn+skip) y de limit (<=0 => sin límite) que el full-scan.
func (e *DbEngine) searchExactByIDs(ctx context.Context, queryEmbedding []float32, ids []string, limit int) ([]SearchResult, error) {
	seen := make(map[string]bool, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}

	var results []SearchResult
	for _, chunk := range chunkStrings(uniq, maxSQLParams) {
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		q := `SELECT o.id, o.topic_key, o.content, o.created_at, e.vector
			FROM observations o
			JOIN embeddings e ON o.id = e.observation_id
			WHERE ` + visibleObsPredicate + `
			  AND o.id IN (` + strings.Join(placeholders, ",") + `)`
		rows, err := e.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("error al consultar candidatos: %w", err)
		}
		for rows.Next() {
			var res SearchResult
			var vectorBytes []byte
			if err := rows.Scan(&res.ID, &res.TopicKey, &res.Content, &res.CreatedAt, &vectorBytes); err != nil {
				rows.Close()
				return nil, fmt.Errorf("error al escanear candidato: %w", err)
			}
			stored, err := BytesToFloat32(vectorBytes)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("error al deserializar vector de candidato: %w", err)
			}
			sim, err := CosineSimilarity(queryEmbedding, stored)
			if err != nil {
				// Dim incompatible (drift de modelo): se omite, igual que el full-scan.
				logx.Warn("candidato omitido en búsqueda semántica por dimensión incompatible",
					"id", res.ID, "error", err)
				continue
			}
			res.Similarity = sim
			results = append(results, res)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error al iterar candidatos: %w", err)
		}
		rows.Close()
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// searchExactFullScan es la búsqueda semántica exacta original: escanea TODOS los
// embeddings activos y rankea por coseno. Es el camino por defecto para DBs por
// debajo del umbral (o con el índice sin entrenar) y la red de seguridad de exactitud.
func (e *DbEngine) searchExactFullScan(ctx context.Context, queryEmbedding []float32, limit int) ([]SearchResult, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, o.content, o.created_at, e.vector
		FROM observations o
		JOIN embeddings e ON o.id = e.observation_id
		WHERE `+visibleObsPredicate+`
	`)
	if err != nil {
		return nil, fmt.Errorf("error al consultar observaciones: %w", err)
	}
	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var res SearchResult
		var vectorBytes []byte

		err := rows.Scan(&res.ID, &res.TopicKey, &res.Content, &res.CreatedAt, &vectorBytes)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila: %w", err)
		}

		storedVector, err := BytesToFloat32(vectorBytes)
		if err != nil {
			return nil, fmt.Errorf("error al deserializar vector de base de datos: %w", err)
		}

		similarity, err := CosineSimilarity(queryEmbedding, storedVector)
		if err != nil {
			// Dimensiones distintas (ej. cambio de modelo de embeddings): se ignora
			// esta fila para no romper la búsqueda, pero se deja rastro.
			logx.Warn("observación omitida en búsqueda semántica por dimensión incompatible",
				"id", res.ID, "error", err)
			continue
		}

		res.Similarity = similarity
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar observaciones semánticas: %w", err)
	}

	// Ordenar descendentemente por similitud de coseno
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Aplicar límite (limit <= 0 significa "sin límite", evita panic por slice negativo)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchObservationsFTS realiza una búsqueda por palabras clave utilizando FTS5 de SQLite.
// Sanea la query del usuario (buildFTSQuery) para que caracteres especiales de FTS5
// no produzcan un error de sintaxis; una query sin términos útiles devuelve vacío.
func (e *DbEngine) SearchObservationsFTS(ctx context.Context, queryText string, limit int) ([]Observation, error) {
	ftsQuery := buildFTSQuery(queryText)
	if ftsQuery == "" {
		return []Observation{}, nil
	}
	rows, err := e.db.QueryContext(ctx, `
		SELECT f.id, f.topic_key, f.content, o.created_at
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND `+visibleObsPredicate+`
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("error en búsqueda FTS5: %w", err)
	}
	defer rows.Close()

	var results []Observation
	for rows.Next() {
		var obs Observation
		if err := rows.Scan(&obs.ID, &obs.TopicKey, &obs.Content, &obs.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear fila FTS5: %w", err)
		}
		results = append(results, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar resultados FTS5: %w", err)
	}

	return results, nil
}
