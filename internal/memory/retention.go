package memory

import (
	"fmt"
	"strings"
	"time"
)

// retention.go acota el crecimiento perpetuo de la base: purga DEFINITIVA de las
// observaciones archivadas frías (el olvido/decay solo las marca `archived`, nunca
// las borra) y compactación física (checkpoint del WAL + optimize + VACUUM). Junto
// con la consolidación y el olvido, conforma el ciclo de mantenimiento (Maintain).

// MaintenanceOptions agrupa los parámetros del ciclo de mantenimiento. Se mapea
// desde config.MaintenanceConfig en la capa que invoca (CLI/daemon/MCP), para no
// acoplar el engine a la forma del struct de config.
type MaintenanceOptions struct {
	DedupThreshold         float64
	DecayHalfLifeDays      float64
	DecayMinSalience       float64
	DecayMinAgeDays        float64
	DecayProtectImportance float64
	PurgeArchivedAfterDays float64
	Vacuum                 bool
}

// MaintenanceReport resume una corrida completa de mantenimiento.
type MaintenanceReport struct {
	Consolidate ConsolidateResult `json:"consolidate"`
	Decay       DecayResult       `json:"decay"`
	Purged      int               `json:"purged"`
	Compacted   bool              `json:"compacted"`
}

// Maintain corre el ciclo completo: consolidar casi-duplicados → olvidar (archivar)
// memorias frías → purgar definitivamente las archivadas vencidas → compactar. Es la
// única entrada al mantenimiento; la usan el subcomando `maintain`, el auto-mantenimiento
// del daemon y la tool MCP musubi_maintain (sin duplicar la secuencia).
func (e *DbEngine) Maintain(opts MaintenanceOptions) (MaintenanceReport, error) {
	var rep MaintenanceReport

	cons, err := e.Consolidate(opts.DedupThreshold)
	if err != nil {
		return rep, err
	}
	rep.Consolidate = cons

	dec, err := e.Decay(DecayOptions{
		HalfLifeDays:      opts.DecayHalfLifeDays,
		MinSalience:       opts.DecayMinSalience,
		MinAgeDays:        opts.DecayMinAgeDays,
		ProtectImportance: opts.DecayProtectImportance,
	})
	if err != nil {
		return rep, err
	}
	rep.Decay = dec

	purged, err := e.PurgeArchived(opts.PurgeArchivedAfterDays)
	if err != nil {
		return rep, err
	}
	rep.Purged = purged

	// Checkpoint del WAL + optimize SIEMPRE (baratos). VACUUM solo si la purga borró
	// filas: reclama el espacio liberado, pero reescribe toda la base (caro).
	if err := e.Compact(opts.Vacuum && purged > 0); err != nil {
		return rep, err
	}
	rep.Compacted = true

	return rep, nil
}

// PurgeArchived borra DEFINITIVAMENTE las observaciones archivadas cuyo último uso
// (last_accessed, o created_at si nunca se accedió) es más viejo que olderThanDays.
// Limpia en la misma transacción los embeddings (FK ON DELETE CASCADE), las relaciones
// semánticas (observation_relations, sin FK) y los punteros superseded_by colgantes.
// Devuelve cuántas observaciones se borraron. olderThanDays <= 0 desactiva la purga.
func (e *DbEngine) PurgeArchived(olderThanDays float64) (int, error) {
	if olderThanDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(olderThanDays * 24 * float64(time.Hour))).Format(sqliteTimeLayout)

	// Solo archivadas cuyo archived_at (el momento en que se archivaron) es más viejo
	// que el corte: la ventana cuenta DESDE el archivado, dando un período de gracia
	// antes del borrado irreversible. archived_at NULL (archivadas por una vía que no lo
	// setea) NO se purgan: no borramos lo que no podemos datar como archivado.
	rows, err := e.db.Query(`
		SELECT id FROM observations
		WHERE archived = 1 AND archived_at IS NOT NULL AND archived_at < ?
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("error al listar archivadas a purgar: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("error al escanear id a purgar: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("error al iterar archivadas a purgar: %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := e.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error al iniciar transacción de purga: %w", err)
	}
	defer tx.Rollback()

	for _, chunk := range chunkStrings(ids, maxSQLParams) {
		ph := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			ph[i] = "?"
			args[i] = id
		}
		in := strings.Join(ph, ",")
		// Borrar la observación (embeddings caen por FK ON DELETE CASCADE).
		if _, err := tx.Exec(`DELETE FROM observations WHERE id IN (`+in+`)`, args...); err != nil {
			return 0, fmt.Errorf("error al borrar observaciones purgadas: %w", err)
		}
		// observation_relations no tiene FK: limpiar en dos sentencias (source y target)
		// para no exceder el tope de parámetros enlazados con un OR de dos IN.
		if _, err := tx.Exec(`DELETE FROM observation_relations WHERE source_id IN (`+in+`)`, args...); err != nil {
			return 0, fmt.Errorf("error al limpiar relaciones (source) purgadas: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM observation_relations WHERE target_id IN (`+in+`)`, args...); err != nil {
			return 0, fmt.Errorf("error al limpiar relaciones (target) purgadas: %w", err)
		}
		// Punteros superseded_by colgantes hacia un id ya borrado -> NULL.
		if _, err := tx.Exec(`UPDATE observations SET superseded_by=NULL WHERE superseded_by IN (`+in+`)`, args...); err != nil {
			return 0, fmt.Errorf("error al limpiar punteros superseded_by purgados: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error al commitear purga: %w", err)
	}

	// Sacar del índice vectorial (defensivo: ya estaban archivadas y removidas al
	// archivarse, pero un Remove idempotente cierra cualquier rastro).
	if e.index != nil {
		for _, id := range ids {
			e.index.Remove(id)
		}
	}

	return len(ids), nil
}

// Compact compacta físicamente la base: trunca el WAL (wal_checkpoint TRUNCATE) y corre
// PRAGMA optimize (mantiene las estadísticas de los índices). Si vacuum es true, además
// corre VACUUM para reclamar el espacio de filas borradas (reescribe toda la base).
func (e *DbEngine) Compact(vacuum bool) error {
	if _, err := e.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("error en wal_checkpoint: %w", err)
	}
	if _, err := e.db.Exec(`PRAGMA optimize`); err != nil {
		return fmt.Errorf("error en PRAGMA optimize: %w", err)
	}
	if vacuum {
		if _, err := e.db.Exec(`VACUUM`); err != nil {
			return fmt.Errorf("error en VACUUM: %w", err)
		}
	}
	return nil
}
