package memory

import "fmt"

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
