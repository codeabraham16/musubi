package memory

import (
	"fmt"
	"sort"

	"musubi/internal/logx"
)

type Observation struct {
	ID        string
	TopicKey  string
	Content   string
	CreatedAt string
}

type SearchResult struct {
	Observation
	Similarity float32
}

// SaveObservation inserta o actualiza una observación y su vector correspondiente.
func (e *DbEngine) SaveObservation(id, topicKey, content string, embedding []float32) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	// 1. Insertar o reemplazar la observación
	queryObs := `INSERT OR REPLACE INTO observations (id, topic_key, content) VALUES (?, ?, ?)`
	_, err = tx.Exec(queryObs, id, topicKey, content)
	if err != nil {
		return fmt.Errorf("error al guardar observación: %w", err)
	}

	// 2. Si se provee embedding, guardarlo
	if len(embedding) > 0 {
		vectorBytes, err := Float32ToBytes(embedding)
		if err != nil {
			return fmt.Errorf("error al serializar vector: %w", err)
		}

		queryEmb := `INSERT OR REPLACE INTO embeddings (observation_id, vector) VALUES (?, ?)`
		_, err = tx.Exec(queryEmb, id, vectorBytes)
		if err != nil {
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
		WHERE observations_fts MATCH ?
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
