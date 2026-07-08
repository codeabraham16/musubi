package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
)

type TelemetryLog struct {
	ID             int
	FilePath       string
	ErrorMessage   string
	SuggestedPatch string
	Resolved       bool
	CreatedAt      string
}

// SaveTelemetryLog registra un error detectado durante la compilación o testing.
func (e *DbEngine) SaveTelemetryLog(filePath, errorMessage, suggestedPatch string) error {
	query := `INSERT INTO telemetry_logs (file_path, error_message, suggested_patch, resolved) VALUES (?, ?, ?, 0)`
	_, err := e.db.Exec(query, filePath, errorMessage, suggestedPatch)
	if err != nil {
		return fmt.Errorf("error al guardar log de telemetría: %w", err)
	}
	return nil
}

// ResolveTelemetryLog marca un log de telemetría como resuelto (resolved = 1).
// Devuelve error si no existe ningún log con el id dado.
func (e *DbEngine) ResolveTelemetryLog(id int) error {
	res, err := e.db.Exec(`UPDATE telemetry_logs SET resolved = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("error al resolver log de telemetría: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al verificar filas afectadas: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("no existe log de telemetría con id %d", id)
	}
	return nil
}

// ResolveTelemetryLogAndGet marca el log id como resuelto Y devuelve su contenido (para que el
// caller pueda capturar el par error→fix como memoria, C4). Atómico (una tx): SELECT la fila y,
// si existe, UPDATE resolved=1. Si el id no existe devuelve found=false (sin error), para que el
// handler traduzca al mismo error que hoy. La variante ResolveTelemetryLog(id) se mantiene.
func (e *DbEngine) ResolveTelemetryLogAndGet(id int) (TelemetryLog, bool, error) {
	tx, err := e.db.Begin()
	if err != nil {
		return TelemetryLog{}, false, fmt.Errorf("error al iniciar tx de telemetría: %w", err)
	}
	defer tx.Rollback()

	var log TelemetryLog
	var resolvedInt int
	err = tx.QueryRow(
		`SELECT id, file_path, error_message, suggested_patch, resolved, created_at FROM telemetry_logs WHERE id = ?`, id,
	).Scan(&log.ID, &log.FilePath, &log.ErrorMessage, &log.SuggestedPatch, &resolvedInt, &log.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TelemetryLog{}, false, nil
	}
	if err != nil {
		return TelemetryLog{}, false, fmt.Errorf("error al leer log de telemetría: %w", err)
	}
	if _, err := tx.Exec(`UPDATE telemetry_logs SET resolved = 1 WHERE id = ?`, id); err != nil {
		return TelemetryLog{}, false, fmt.Errorf("error al resolver log de telemetría: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TelemetryLog{}, false, fmt.Errorf("error al commitear resolución de telemetría: %w", err)
	}
	log.Resolved = true
	return log, true, nil
}

// telemetryPathKey normaliza una ruta para matchear telemetría: minúsculas y separadores
// '/' (los triggers/paths llegan con '\' en Windows y '/' en otros). Determinista, model-free.
func telemetryPathKey(p string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
}

// GetUnresolvedTelemetryLogsForFiles devuelve los logs de telemetría NO resueltos cuyo
// file_path coincide —por ruta completa o por nombre base— con alguno de los archivos dados.
// Es la telemetría RELEVANTE a lo que el agente está tocando (T6.2), en vez de TODA la
// pendiente. La reusan resolve_skills (por turno) y el hook precheck (por archivo, T6.3).
func (e *DbEngine) GetUnresolvedTelemetryLogsForFiles(files []string) ([]TelemetryLog, error) {
	if len(files) == 0 {
		return nil, nil
	}
	all, err := e.GetUnresolvedTelemetryLogs()
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(files)*2)
	for _, f := range files {
		k := telemetryPathKey(f)
		if k == "" {
			continue
		}
		want[k] = true
		want[path.Base(k)] = true
	}
	var out []TelemetryLog
	for _, l := range all {
		lk := telemetryPathKey(l.FilePath)
		if want[lk] || want[path.Base(lk)] {
			out = append(out, l)
		}
	}
	return out, nil
}

// GetUnresolvedTelemetryLogs obtiene logs de telemetría que aún no han sido resueltos.
func (e *DbEngine) GetUnresolvedTelemetryLogs() ([]TelemetryLog, error) {
	rows, err := e.db.Query(`
		SELECT id, file_path, error_message, suggested_patch, resolved, created_at 
		FROM telemetry_logs 
		WHERE resolved = 0
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("error al obtener logs de telemetría: %w", err)
	}
	defer rows.Close()

	var logs []TelemetryLog
	for rows.Next() {
		var log TelemetryLog
		var resolvedInt int
		err := rows.Scan(&log.ID, &log.FilePath, &log.ErrorMessage, &log.SuggestedPatch, &resolvedInt, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("error al escanear log de telemetría: %w", err)
		}
		log.Resolved = resolvedInt == 1
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar logs de telemetría: %w", err)
	}
	return logs, nil
}
