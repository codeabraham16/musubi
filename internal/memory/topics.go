package memory

import (
	"database/sql"
	"fmt"
)

// topics.go expone consultas livianas sobre topic_keys, usadas por el arranque
// para decidir si el proyecto ya tiene autoconocimiento (perfil) registrado.

// TopicExists indica si hay al menos una observación NO archivada con ese
// topic_key exacto. Es la señal model-free que usa el hook para saber si el
// proyecto ya está "perfilado" y bajar la inyección de skills cognitivas.
func (e *DbEngine) TopicExists(topicKey string) (bool, error) {
	var x int
	err := e.db.QueryRow(
		`SELECT 1 FROM observations WHERE topic_key = ? AND archived = 0 LIMIT 1`,
		topicKey,
	).Scan(&x)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error al consultar existencia de topic %q: %w", topicKey, err)
	}
	return true, nil
}
