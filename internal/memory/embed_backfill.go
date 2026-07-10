package memory

import "fmt"

// embed_backfill.go implementa el RE-EMBEDDING del histórico (T17.3): cuando se enciende la
// memoria semántica sobre una base con observaciones previas, o cuando se cambia de embedder, esas
// observaciones quedan sin vector de la procedencia actual y son INVISIBLES para el recall
// semántico (la regla de homogeneidad sólo compara vectores del mismo model_id). WarnOnEmbedModelSwitch
// avisaba de ese hueco pero no ofrecía remedio; EmbedBackfill es el remedio. Model-free: el server
// no embebe, recibe el callback de vectorización del caller (el CLI, con el provider resuelto).

// EmbedBackfillResult resume una corrida de re-embedding del histórico.
type EmbedBackfillResult struct {
	ModelID  string `json:"model_id"` // procedencia con la que se re-embebió
	Scanned  int    `json:"scanned"`  // observaciones activas que necesitaban (re)embedding
	Embedded int    `json:"embedded"` // vectores generados y persistidos
	Skipped  int    `json:"skipped"`  // el embedder devolvió vector vacío (no se persiste)
}

// EmbedBackfill (re)genera los embeddings de las observaciones ACTIVAS que no tienen un vector con
// la procedencia (model_id) del embedder ACTUAL — las guardadas antes de encender la semántica, o
// las de otro modelo tras un cambio de embedder. embed es el callback de vectorización (el CLI le
// pasa el provider resuelto). Estampa e.vectorModelID como procedencia (igual que un save normal),
// reconstruye el índice IVF UNA sola vez al final (más barato que Add por vector) y actualiza la
// marca MetaEmbedModel para que el aviso de cambio de modelo no vuelva a dispararse. Es idempotente
// y resumible: una fila ya re-embebida cambia su model_id al actual, así que una corrida posterior
// no la vuelve a listar. Requiere un embedder nombrado (e.vectorModelID != ""): sin él no hay
// semántica que backfillear.
func (e *DbEngine) EmbedBackfill(embed func(string) ([]float32, error)) (EmbedBackfillResult, error) {
	res := EmbedBackfillResult{ModelID: e.vectorModelID}
	if e.vectorModelID == "" {
		return res, fmt.Errorf("no hay un embedder nombrado configurado; encendé la memoria semántica antes de backfillear")
	}
	if embed == nil {
		return res, fmt.Errorf("embed callback nil")
	}

	// Observaciones ACTIVAS sin vector de la procedencia actual: sin fila en embeddings (LEFT JOIN
	// nulo) o con model_id distinto (vector de otro modelo, excluido del recall por homogeneidad).
	rows, err := e.db.Query(`
		SELECT o.id, o.content
		FROM observations o
		LEFT JOIN embeddings em ON o.id = em.observation_id
		WHERE `+visibleObsPredicate+` AND (em.observation_id IS NULL OR em.model_id != ?)
	`, e.vectorModelID)
	if err != nil {
		return res, fmt.Errorf("error al listar observaciones a re-embeber: %w", err)
	}
	type pending struct{ id, content string }
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.content); err != nil {
			rows.Close()
			return res, fmt.Errorf("error al escanear observación pendiente: %w", err)
		}
		todo = append(todo, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, fmt.Errorf("error al iterar observaciones pendientes: %w", err)
	}
	rows.Close()
	res.Scanned = len(todo)

	for _, p := range todo {
		vec, err := embed(p.content)
		if err != nil {
			// Abortar con el progreso ya persistido: la corrida es resumible (lo hecho no se
			// re-lista). Devolver el error para que el operador vea qué falló (p.ej. ollama caído).
			return res, fmt.Errorf("error al embeber la observación %s: %w", p.id, err)
		}
		if len(vec) == 0 {
			res.Skipped++
			continue
		}
		vectorBytes, err := Float32ToBytes(vec)
		if err != nil {
			return res, fmt.Errorf("error al serializar el vector de %s: %w", p.id, err)
		}
		if _, err := e.db.Exec(
			`INSERT OR REPLACE INTO embeddings (observation_id, vector, model_id) VALUES (?, ?, ?)`,
			p.id, vectorBytes, e.vectorModelID,
		); err != nil {
			return res, fmt.Errorf("error al guardar el embedding de %s: %w", p.id, err)
		}
		res.Embedded++
	}

	// Reconstruir el índice IVF una sola vez (si hay índice) para que los vectores nuevos entren al
	// candidateo, y persistir la marca de modelo para que WarnOnEmbedModelSwitch no vuelva a avisar.
	if res.Embedded > 0 {
		if err := e.rebuildVectorIndex(); err != nil {
			return res, fmt.Errorf("re-embedding OK pero falló el rebuild del índice: %w", err)
		}
	}
	if err := e.SetMeta(MetaEmbedModel, e.vectorModelID); err != nil {
		return res, fmt.Errorf("re-embedding OK pero falló al persistir la marca de modelo: %w", err)
	}
	return res, nil
}
