package memory

import (
	"fmt"
	"strings"
)

// GetObservations devuelve el contenido completo de las observaciones indicadas
// (hidratación perezosa tras un Recall). Preserva el orden de ids, omite los que
// no existan y actualiza las estadísticas de acceso de las encontradas.
func (e *DbEngine) GetObservations(ids []string) ([]Observation, error) {
	out, _, err := e.GetObservationsBudget(ids, 0)
	return out, err
}

// GetObservationsBudget hidrata observaciones por id respetando un techo de
// tokens (budget). Empaqueta contenidos completos en orden de id hasta que el
// siguiente no entra; garantiza al menos el primero (truncado si excede el
// budget). budget <= 0 significa sin límite. Devuelve también los tokens usados,
// para contabilizarlos en el ledger. Actualiza stats de acceso de lo devuelto.
func (e *DbEngine) GetObservationsBudget(ids []string, budget int) ([]Observation, int, error) {
	if len(ids) == 0 {
		return []Observation{}, 0, nil
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
		return nil, 0, fmt.Errorf("error al obtener observaciones: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]Observation, len(ids))
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content, &o.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("error al escanear observación: %w", err)
		}
		byID[o.ID] = o
	}

	out := make([]Observation, 0, len(ids))
	found := make([]string, 0, len(ids))
	used := 0
	for _, id := range ids {
		o, ok := byID[id]
		if !ok {
			continue
		}
		cost := EstimateTokens(o.Content)
		if budget > 0 {
			if len(out) == 0 && cost > budget {
				// Garantizar el primero, truncado al presupuesto.
				o.Content = truncateToTokens(o.Content, budget)
				cost = EstimateTokens(o.Content)
			} else if used+cost > budget {
				continue // no entra; probamos el siguiente (puede ser más chico)
			}
		}
		out = append(out, o)
		found = append(found, id)
		used += cost
		if budget > 0 && used >= budget {
			break
		}
	}

	if err := e.bumpAccess(found); err != nil {
		return out, used, err
	}
	return out, used, nil
}
