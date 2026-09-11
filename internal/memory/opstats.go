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
	"strings"
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
	// BackupOffhostModo es lo que el guion de backup DECLARÓ sobre el DR: "remoto", "local-only"
	// o "" si todavía no corrió ninguna vez.
	//
	// EXISTE PORQUE EL -1 DE ARRIBA SIGNIFICA DOS COSAS OPUESTAS. Vale -1 tanto en un local-only
	// DECLARADO —una decisión, con su motivo y su costo escrito— como en un off-host que falla
	// todas las noches. Desde Prometheus los dos son el mismo número, así que la única forma de
	// saber cuál era la de ir a preguntarle a `musubi doctor` a mano: justo lo que una alerta
	// existe para no tener que hacer.
	BackupOffhostModo string
	BackupLocalAgeSec int64 // antigüedad del último SNAPSHOT local; -1 si no hay marca. Distinto del de arriba: éste dice si el timer corre, aquél si el backup sale de la máquina
	// MaintenanceAgeSec dice hace cuánto corrió el ciclo de memoria (consolidar/olvidar/purgar);
	// -1 si nunca. Sin esta serie, un cerebro que dejó de mantenerse se ve EXACTAMENTE igual que uno
	// que se mantiene: la memoria sigue respondiendo, sólo deja de envejecer bien.
	MaintenanceAgeSec int64
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
	st.BackupOffhostModo = ""
	// Y la del SNAPSHOT local, que es la que dice si el timer sigue corriendo. En local-only la de
	// arriba vale -1 para siempre, así que sin ésta el único trabajo programado del servidor no
	// tenía ninguna señal: ni al fallar (nadie recoge su exit code) ni al dejar de dispararse.
	st.BackupLocalAgeSec = -1
	st.MaintenanceAgeSec = e.MantenimientoEdadSegundos()
	if e.path != "" {
		dir := filepath.Join(filepath.Dir(e.path), "backups")
		if fi, statErr := os.Stat(filepath.Join(dir, offhostMarkerName)); statErr == nil {
			st.BackupOffhostAgeSec = int64(time.Since(fi.ModTime()).Seconds())
		}
		// El modo lo escribe QUIEN LO SABE: el guion de backup, que es el único que ve
		// `BACKUP_REMOTE`. El cerebro no puede inferirlo, y adivinarlo sería peor que no decirlo.
		if crudo, readErr := os.ReadFile(filepath.Join(dir, offhostModoName)); readErr == nil {
			st.BackupOffhostModo = strings.TrimSpace(string(crudo))
		}
		if fi, statErr := os.Stat(filepath.Join(dir, snapshotMarkerName)); statErr == nil {
			st.BackupLocalAgeSec = int64(time.Since(fi.ModTime()).Seconds())
		}
	}
	return st, nil
}
