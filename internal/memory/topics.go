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

// DomainCount es la cantidad de observaciones activas en un DOMINIO de topic: el
// prefijo del topic_key antes del primer "/" ("roadmap/track-7" -> "roadmap").
type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// TopicDomainCounts agrupa las observaciones NO archivadas por su dominio de topic
// y devuelve los conteos ordenados por cantidad desc (desempate alfabético, salida
// determinista). Alimenta el "mapa de conocimiento" —de qué habla la memoria— sin
// LLM. Un topic_key sin "/" cuenta como su propio dominio.
func (e *DbEngine) TopicDomainCounts() ([]DomainCount, error) {
	rows, err := e.db.Query(`
		SELECT CASE WHEN instr(topic_key, '/') > 0
		            THEN substr(topic_key, 1, instr(topic_key, '/') - 1)
		            ELSE topic_key END AS domain,
		       COUNT(*) AS c
		FROM observations
		WHERE archived = 0
		GROUP BY domain
		ORDER BY c DESC, domain ASC`)
	if err != nil {
		return nil, fmt.Errorf("conteo por dominio de topic: %w", err)
	}
	defer rows.Close()
	var out []DomainCount
	for rows.Next() {
		var d DomainCount
		if err := rows.Scan(&d.Domain, &d.Count); err != nil {
			return nil, fmt.Errorf("conteo por dominio de topic: escanear: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("conteo por dominio de topic: iterar: %w", err)
	}
	return out, nil
}
