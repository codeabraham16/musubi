package memory

import (
	"database/sql"
	"fmt"
	"sort"
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

// TopicLeaf es un topic_key concreto dentro de un dominio (la hoja del árbol): su
// nombre (el sufijo después del dominio), cuántas memorias tiene y cuándo fue la
// última actividad.
type TopicLeaf struct {
	Topic        string `json:"topic"`
	Count        int    `json:"count"`
	LastActivity string `json:"last_activity"`
}

// DomainNode es un dominio del mapa de conocimiento (rama del árbol): nombre, total de
// memorias activas, última actividad (la más reciente entre sus temas) y sus hojas.
type DomainNode struct {
	Domain       string      `json:"domain"`
	Count        int         `json:"count"`
	LastActivity string      `json:"last_activity"`
	Topics       []TopicLeaf `json:"topics"`
}

// TopicTree arma el árbol DOMINIO → temas de las observaciones NO archivadas, con
// conteos y última actividad por nodo. Alimenta el grafo de conocimiento interactivo
// (drill-down + brillo por recencia). Agregación SQL determinista, sin LLM. Dominios
// ordenados por cantidad desc (desempate alfabético); temas igual dentro de cada uno.
func (e *DbEngine) TopicTree() ([]DomainNode, error) {
	rows, err := e.db.Query(`
		SELECT topic_key, COUNT(*) AS c, COALESCE(MAX(created_at), '') AS last
		FROM observations
		WHERE archived = 0
		GROUP BY topic_key`)
	if err != nil {
		return nil, fmt.Errorf("árbol de topics: %w", err)
	}
	defer rows.Close()

	idx := map[string]*DomainNode{}
	for rows.Next() {
		var tk, last string
		var c int
		if err := rows.Scan(&tk, &c, &last); err != nil {
			return nil, fmt.Errorf("árbol de topics: escanear: %w", err)
		}
		domain, leaf := tk, tk
		if i := indexByte(tk, '/'); i > 0 {
			domain, leaf = tk[:i], tk[i+1:]
		}
		dn := idx[domain]
		if dn == nil {
			dn = &DomainNode{Domain: domain}
			idx[domain] = dn
		}
		dn.Count += c
		if last > dn.LastActivity {
			dn.LastActivity = last
		}
		dn.Topics = append(dn.Topics, TopicLeaf{Topic: leaf, Count: c, LastActivity: last})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("árbol de topics: iterar: %w", err)
	}

	out := make([]DomainNode, 0, len(idx))
	for _, dn := range idx {
		sortTopicsByCount(dn.Topics)
		out = append(out, *dn)
	}
	sortDomainsByCount(out)
	return out, nil
}

// indexByte devuelve el índice del primer b en s, o -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func sortTopicsByCount(ts []TopicLeaf) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Count != ts[j].Count {
			return ts[i].Count > ts[j].Count
		}
		return ts[i].Topic < ts[j].Topic
	})
}

func sortDomainsByCount(ds []DomainNode) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].Count != ds[j].Count {
			return ds[i].Count > ds[j].Count
		}
		return ds[i].Domain < ds[j].Domain
	})
}
