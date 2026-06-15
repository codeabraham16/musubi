package memory

import (
	"fmt"
	"strings"
)

// GetObservations devuelve el contenido completo de las observaciones indicadas
// (hidratación perezosa tras un Recall). Preserva el orden de ids, omite los que
// no existan y actualiza las estadísticas de acceso de las encontradas.
func (e *DbEngine) GetObservations(ids []string) ([]Observation, error) {
	if len(ids) == 0 {
		return []Observation{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := e.db.Query(
		`SELECT id, topic_key, content, COALESCE(created_at,'')
		 FROM observations WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("error al obtener observaciones: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]Observation, len(ids))
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear observación: %w", err)
		}
		byID[o.ID] = o
	}

	out := make([]Observation, 0, len(ids))
	found := make([]string, 0, len(ids))
	for _, id := range ids {
		if o, ok := byID[id]; ok {
			out = append(out, o)
			found = append(found, id)
		}
	}

	if err := e.bumpAccess(found); err != nil {
		return out, err
	}
	return out, nil
}
