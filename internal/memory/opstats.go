package memory

// opstats.go expone métricas OPERATIVAS puntuales del motor para el endpoint /metrics
// (Track 16 / Producible F3.1: "no podés operar lo que no ves"). Se calculan on-demand en
// cada scrape: unos COUNT baratos sobre SQLite + el estado en RAM del índice vectorial. Es
// best-effort — si una consulta falla, se reporta el error y el caller omite los gauges ese
// scrape (nunca rompe /metrics). Cero dependencias nuevas.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// opStatsTimeout acota los COUNT O(n) de OperationalStats para que un scrape de /metrics no
// cuelgue si la base está lenta o muy grande (T17.5): pasado el deadline, la consulta se cancela
// y el scrape omite los gauges de dominio ese ciclo (best-effort) en vez de bloquear.
const opStatsTimeout = 5 * time.Second

// OpStats es una foto de las métricas operativas del motor en un instante, pensada para
// exponerse como gauges Prometheus. Todo son magnitudes acotadas (conteos + antigüedad), no
// series por-item, así que la cardinalidad se mantiene baja.
type OpStats struct {
	Observations        int   // observaciones VISIBLES (no archivadas/superseded)
	ActiveEmbeddings    int   // observaciones visibles con embedding (participan del recall vectorial)
	VectorIndexSize     int   // vectores vivos en el índice IVF
	VectorIndexTrained  bool  // el IVF tiene centroides utilizables (si no, recall = full-scan exacto)
	VectorIndexDim      int   // dimensión del índice (0 si no entrenado)
	OutboxPending       int   // filas del outbox de sync sin enviar (incluye claimed)
	OutboxSent          int   // filas ya empujadas al central
	OutboxDead          int   // filas que agotaron reintentos (requieren atención)
	OutboxOldestAgeSec  int64 // antigüedad de la pendiente más vieja (0 si no hay): mide atraso del sync
	BackupOffhostAgeSec int64 // antigüedad del último backup off-host EXITOSO; -1 si no hay marca (T18)
}

// OperationalStats reúne las métricas operativas del motor para /metrics. Hace unas pocas
// consultas COUNT + lee el estado en memoria del índice vectorial. Un error en cualquier
// consulta aborta y se reporta (el caller decide: típicamente omite los gauges ese scrape).
func (e *DbEngine) OperationalStats() (OpStats, error) {
	// Deadline compartido por los COUNT O(n) (observaciones + embeddings activos): el más caro del
	// scrape. Si la base está lenta/bloqueada, la consulta se cancela y el caller omite los gauges.
	ctx, cancel := context.WithTimeout(context.Background(), opStatsTimeout)
	defer cancel()

	var st OpStats
	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM observations o WHERE `+visibleObsPredicate,
	).Scan(&st.Observations); err != nil {
		return st, fmt.Errorf("contar observaciones: %w", err)
	}
	ae, err := e.countActiveEmbeddingsCtx(ctx)
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

	// Staleness del backup off-host como gauge (Track 18): -1 si no hay marca (instancia local o
	// backup que nunca tuvo éxito). Expone el DR a Prometheus para que un backup que dejó de shipear
	// (o que nunca funcionó) sea PAGINABLE, no solo visible en `musubi doctor`.
	st.BackupOffhostAgeSec = -1
	if e.path != "" {
		if fi, statErr := os.Stat(filepath.Join(filepath.Dir(e.path), "backups", offhostMarkerName)); statErr == nil {
			st.BackupOffhostAgeSec = int64(time.Since(fi.ModTime()).Seconds())
		}
	}
	return st, nil
}
