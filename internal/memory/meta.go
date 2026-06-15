package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// meta.go guarda metadatos clave/valor y la lógica de throttle del
// auto-mantenimiento (cada cuánto correr consolidación + olvido).

const metaLastMaintenance = "last_maintenance"

// GetMeta devuelve el valor de una clave de metadatos (ok=false si no existe).
func (e *DbEngine) GetMeta(key string) (string, bool, error) {
	var v string
	err := e.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("error al leer meta %q: %w", key, err)
	}
	return v, true, nil
}

// SetMeta inserta o actualiza una clave de metadatos.
func (e *DbEngine) SetMeta(key, value string) error {
	_, err := e.db.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("error al guardar meta %q: %w", key, err)
	}
	return nil
}

// MaintenanceDue indica si corresponde correr el auto-mantenimiento: true si no
// hay marca previa, si no se puede parsear, o si pasaron >= intervalHours.
func (e *DbEngine) MaintenanceDue(intervalHours float64) (bool, error) {
	v, ok, err := e.GetMeta(metaLastMaintenance)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	last, perr := time.Parse(time.RFC3339, v)
	if perr != nil {
		return true, nil // marca corrupta: mejor correr
	}
	return time.Since(last).Hours() >= intervalHours, nil
}

// MarkMaintenanceNow registra que el mantenimiento acaba de correr.
func (e *DbEngine) MarkMaintenanceNow() error {
	return e.SetMeta(metaLastMaintenance, time.Now().UTC().Format(time.RFC3339))
}
