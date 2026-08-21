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

// LatestObservationByTopicInProject devuelve el CONTENIDO de la observación visible más reciente con
// ese topic_key ATRIBUIDA EXACTAMENTE a projectID. found=false si no hay ninguna.
//
// El scope es ESTRICTO a propósito (project_id = ?), NO el criterio del recall que conserva las filas
// sin atribuir: la marca de un proyecto se resuelve keyed y aislada, para que una fila 'diseno/marca'
// SIN atribuir no se aplique a TODOS los tenants (la mitigación anti-fuga cross-marca de Musubi
// Renaissance F1). Es la contracara de fetch de la CAPA 3 (marca por proyecto): querés LA marca exacta
// del target, no un match difuso ni una heredada. Model-free (query keyed, sin LLM ni embeddings).
func (e *DbEngine) LatestObservationByTopicInProject(topicKey, projectID string) (content string, found bool, err error) {
	q := `SELECT content FROM observations
	      WHERE topic_key = ? AND project_id = ? AND ` + visibleObsPredicate + `
	      ORDER BY created_at DESC, rowid DESC LIMIT 1`
	err = e.db.QueryRow(q, topicKey, projectID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("marca por topic %q en proyecto %q: %w", topicKey, projectID, err)
	}
	return content, true, nil
}

// ObservationsByTopicPrefixInProject devuelve hasta limit observaciones VISIBLES atribuidas EXACTAMENTE a
// projectID cuyo topic_key empieza con topicPrefix, ordenadas por IMPORTANCIA desc y luego recencia. La usa
// el MÉTODO VIVO del motor de diseño (Musubi Renaissance · CAPA 2): las tarjetas `design-method/*` del
// acervo que reemplazan a los principios hardcodeados, para que el método se pueda ARBITRAR
// (judge/supersede) a medida que el "tell" anti-genérico se mueve. El scope es ESTRICTO (project_id = ?),
// igual que la marca: el método es del acervo COMPARTIDO `musubi-design`, no se hereda difuso desde otro
// tenant. Ordena por importancia para que un método reforzado pese más que uno recién agregado. Model-free
// (query keyed, sin LLM ni embeddings).
func (e *DbEngine) ObservationsByTopicPrefixInProject(projectID, topicPrefix string, limit int) ([]ObsLite, error) {
	if limit <= 0 {
		limit = 20
	}
	q := `SELECT id, topic_key, content FROM observations
	      WHERE project_id = ? AND topic_key LIKE ? AND ` + visibleObsPredicate + `
	      ORDER BY importance DESC, created_at DESC, rowid DESC LIMIT ?`
	rows, err := e.db.Query(q, projectID, topicPrefix+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("observaciones por prefijo %q en proyecto %q: %w", topicPrefix, projectID, err)
	}
	defer rows.Close()
	var out []ObsLite
	for rows.Next() {
		var o ObsLite
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content); err != nil {
			return nil, fmt.Errorf("escanear observación por prefijo %q: %w", topicPrefix, err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar observaciones por prefijo %q: %w", topicPrefix, err)
	}
	return out, nil
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
		WHERE `+visibleObsPredicate+`
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

// TopicTree arma el árbol DOMINIO → temas de las observaciones VISIBLES (recuperables), con
// conteos y última actividad por nodo. Alimenta el grafo de conocimiento interactivo
// (drill-down + brillo por recencia). Agregación SQL determinista, sin LLM. Dominios
// ordenados por cantidad desc (desempate alfabético); temas igual dentro de cada uno.
//
// Usa el predicado CANÓNICO y no un `archived = 0` propio, por el mismo motivo que
// braingraph.go lo documenta para el grafo: con el filtro laxo el árbol contaba las notas
// SUPERADAS y las EN CUARENTENA, así que el encabezado del mapa de conocimiento sumaba una
// población y el grafo dibujaba otra —64 de diferencia en el cerebro central—, y encima
// exponía en la cara visual texto sin corroborar que la cuarentena existe para tapar.
func (e *DbEngine) TopicTree() ([]DomainNode, error) {
	rows, err := e.db.Query(`
		SELECT topic_key, COUNT(*) AS c, COALESCE(MAX(created_at), '') AS last
		FROM observations
		WHERE ` + visibleObsPredicate + `
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
