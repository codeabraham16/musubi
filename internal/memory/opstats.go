package memory

// opstats.go expone métricas OPERATIVAS puntuales del motor para el endpoint /metrics
// (Track 16 / Producible F3.1: "no podés operar lo que no ves"). Se calculan on-demand en
// cada scrape: unos COUNT baratos sobre SQLite + el estado en RAM del índice vectorial. Es
// best-effort — si una consulta falla, se reporta el error y el caller omite los gauges ese
// scrape (nunca rompe /metrics). Cero dependencias nuevas.

import "fmt"

// OpStats es una foto de las métricas operativas del motor en un instante, pensada para
// exponerse como gauges Prometheus. Todo son magnitudes acotadas (conteos + antigüedad), no
// series por-item, así que la cardinalidad se mantiene baja.
type OpStats struct {
	Observations       int   // observaciones VISIBLES (no archivadas/superseded)
	ActiveEmbeddings   int   // observaciones visibles con embedding (participan del recall vectorial)
	VectorIndexSize    int   // vectores vivos en el índice IVF
	VectorIndexTrained bool  // el IVF tiene centroides utilizables (si no, recall = full-scan exacto)
	VectorIndexDim     int   // dimensión del índice (0 si no entrenado)
	OutboxPending      int   // filas del outbox de sync sin enviar (incluye claimed)
	OutboxSent         int   // filas ya empujadas al central
	OutboxDead         int   // filas que agotaron reintentos (requieren atención)
	OutboxOldestAgeSec int64 // antigüedad de la pendiente más vieja (0 si no hay): mide atraso del sync
}

// OperationalStats reúne las métricas operativas del motor para /metrics. Hace unas pocas
// consultas COUNT + lee el estado en memoria del índice vectorial. Un error en cualquier
// consulta aborta y se reporta (el caller decide: típicamente omite los gauges ese scrape).
func (e *DbEngine) OperationalStats() (OpStats, error) {
	var st OpStats
	if err := e.db.QueryRow(
		`SELECT COUNT(*) FROM observations o WHERE ` + visibleObsPredicate,
	).Scan(&st.Observations); err != nil {
		return st, fmt.Errorf("contar observaciones: %w", err)
	}
	ae, err := e.countActiveEmbeddings()
	if err != nil {
		return st, err
	}
	st.ActiveEmbeddings = ae

	// El índice puede ser nil si la búsqueda vectorial está desactivada por config.
	if e.index != nil {
		st.VectorIndexSize = e.index.Len()
		st.VectorIndexTrained = e.index.Trained()
		st.VectorIndexDim = e.index.Dim()
	}

	h, err := e.OutboxHealth()
	if err != nil {
		return st, err
	}
	st.OutboxPending, st.OutboxSent, st.OutboxDead = h.Pending, h.Sent, h.Dead
	st.OutboxOldestAgeSec = h.OldestPendingAgeSec
	return st, nil
}
