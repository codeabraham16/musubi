package memory

import (
	"database/sql"
	"fmt"
	"sort"

	"musubi/internal/logx"

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

// SaveObservation inserta o actualiza una observación y su vector. Computa de
// forma model-free el gist, el content_hash y la estimación de tokens. La
// importancia no se toca en updates (se preserva la existente).
func (e *DbEngine) SaveObservation(id, topicKey, content string, embedding []float32) error {
	return e.saveObservation(id, topicKey, content, 1.0, false, embedding)
}

// SaveObservationWithImportance es como SaveObservation pero fija la importancia
// (también en updates). importance pondera el ranking del recall.
func (e *DbEngine) SaveObservationWithImportance(id, topicKey, content string, importance float64, embedding []float32) error {
	return e.saveObservation(id, topicKey, content, importance, true, embedding)
}

// SaveObservationDeduped guarda content con un id nuevo, salvo que ya exista una
// observación con el mismo content_hash (dedup exacto): en ese caso devuelve ese
// id y deduped=true sin insertar nada.
func (e *DbEngine) SaveObservationDeduped(topicKey, content string, importance float64, embedding []float32) (string, bool, error) {
	existing, found, err := e.FindByContentHash(ContentHash(content))
	if err != nil {
		return "", false, err
	}
	if found {
		return existing, true, nil
	}
	id := uuid.NewString()
	if err := e.SaveObservationWithImportance(id, topicKey, content, importance, embedding); err != nil {
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
func (e *DbEngine) saveObservation(id, topicKey, content string, importance float64, setImportance bool, embedding []float32) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	gist := Gist(content, defaultGistMaxTokens)
	hash := ContentHash(content)
	tokens := EstimateTokens(content)

	setImp := ""
	if setImportance {
		setImp = ",\n\t\t\timportance=excluded.importance"
	}
	queryObs := `INSERT INTO observations (id, topic_key, content, gist, content_hash, tokens, importance)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic_key=excluded.topic_key,
			content=excluded.content,
			gist=excluded.gist,
			content_hash=excluded.content_hash,
			tokens=excluded.tokens` + setImp
	if _, err = tx.Exec(queryObs, id, topicKey, content, gist, hash, tokens, importance); err != nil {
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

	return tx.Commit()
}

// SearchObservations realiza una búsqueda semántica comparando el vector query con la BD.
func (e *DbEngine) SearchObservations(queryEmbedding []float32, limit int) ([]SearchResult, error) {
	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, o.content, o.created_at, e.vector
		FROM observations o
		JOIN embeddings e ON o.id = e.observation_id
		WHERE o.archived = 0
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
func (e *DbEngine) SearchObservationsFTS(queryText string, limit int) ([]Observation, error) {
	rows, err := e.db.Query(`
		SELECT f.id, f.topic_key, f.content, o.created_at
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND o.archived = 0
		ORDER BY rank
		LIMIT ?
	`, queryText, limit)
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

	return results, nil
}
