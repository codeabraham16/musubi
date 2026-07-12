package memory

import (
	"fmt"

	"musubi/internal/logx"
)

// embed_backfill.go implementa el RE-EMBEDDING del histórico (T17.3): cuando se enciende la
// memoria semántica sobre una base con observaciones previas, o cuando se cambia de embedder, esas
// observaciones quedan sin vector de la procedencia actual y son INVISIBLES para el recall
// semántico (la regla de homogeneidad sólo compara vectores del mismo model_id). WarnOnEmbedModelSwitch
// avisaba de ese hueco pero no ofrecía remedio; EmbedBackfill es el remedio. Model-free: el server
// no embebe, recibe el callback de vectorización del caller (el CLI, con el provider resuelto).

// stalePredicate es el WHERE que define "observación PENDIENTE de (re)embedding": activa y sin
// vector de la procedencia ACTUAL — sin fila en embeddings (LEFT JOIN nulo) o con model_id distinto
// (vector de otro modelo, ya excluido del recall por la regla de homogeneidad). Una sola fuente de
// verdad, compartida por el COUNT (AutoEmbedBackfill) y el SELECT (EmbedBackfill): si divergieran,
// el auto-backfill podría creer que no hay nada que hacer mientras el backfill sí tiene trabajo.
// Espera un solo parámetro: el model_id actual.
func stalePredicate() string {
	return visibleObsPredicate + ` AND (em.observation_id IS NULL OR em.model_id != ?)`
}

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
		WHERE `+stalePredicate(), e.vectorModelID)
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

// countStaleEmbeddings cuenta las observaciones PENDIENTES de (re)embedding (ver stalePredicate).
func (e *DbEngine) countStaleEmbeddings() (int, error) {
	var n int
	err := e.db.QueryRow(`
		SELECT COUNT(*)
		FROM observations o
		LEFT JOIN embeddings em ON o.id = em.observation_id
		WHERE `+stalePredicate(), e.vectorModelID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error al contar observaciones pendientes de embedding: %w", err)
	}
	return n, nil
}

// AutoEmbedBackfill (M3) cierra SOLO el hueco de procedencia, sin intervención manual: si hay
// observaciones activas sin vector del model_id ACTUAL —memoria previa a encender la semántica, o
// vectores de otra tabla tras un cambio de modelo/checksum (N1)— lanza EmbedBackfill EN BACKGROUND.
//
// Sin esto, cambiar de modelo APAGA el recall semántico (el contrato de procedencia excluye los
// vectores viejos) hasta que alguien corra `musubi embed backfill` a mano: el server avisaba del
// hueco pero no lo remediaba.
//
// Va en background y no síncrono a propósito: un daemon bajo systemd tiene timeout de arranque, y
// re-embeber una base grande tardaría minutos y haría FALLAR el arranque de la unit. spawnBackground
// ya resuelve el cierre limpio (no lanza si el engine está cerrado; Close espera a que termine).
//
// El engine sigue siendo model-free: recibe el callback de vectorización del caller, no embebe.
func (e *DbEngine) AutoEmbedBackfill(embed func(string) ([]float32, error)) {
	if e.vectorModelID == "" || embed == nil {
		return // sin semántica activa no hay nada que backfillear
	}
	n, err := e.countStaleEmbeddings()
	if err != nil {
		logx.Warn("no se pudo verificar si hay memoria sin vector del modelo actual", "error", err)
		return
	}
	if n == 0 {
		return // el caso común en cada arranque: nada pendiente, ni goroutine ni ruido en el log
	}
	// Visible, no silencioso: durante la ventana del backfill esas observaciones siguen excluidas
	// del recall semántico (degradación TEMPORAL, no corrupción).
	logx.Info("re-embebiendo memoria histórica en background",
		"pendientes", n, "modelo", e.vectorModelID,
		"nota", "hasta que termine, esas observaciones no aparecen en la búsqueda semántica")
	e.spawnBackground(func() {
		res, err := e.EmbedBackfill(embed)
		if err != nil {
			logx.Warn("el re-embedding automático falló; corré `musubi embed backfill` a mano",
				"error", err, "embebidas", res.Embedded)
			return
		}
		logx.Info("re-embedding automático completo",
			"embebidas", res.Embedded, "omitidas", res.Skipped, "modelo", res.ModelID)
	})
}
