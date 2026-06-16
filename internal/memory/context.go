package memory

import "fmt"

// context.go ensambla, de forma MODEL-FREE y on-demand, todo el contexto de una
// entidad: sus HECHOS del grafo + los GISTS de las observaciones (prosa) que la
// mencionan. Es el puente grafo<->prosa, barato en tokens y siempre consistente
// (no precomputa nada; usa el grafo y el índice FTS existentes).

const defaultMaxObservations = 5

// ObsGist es una observación en forma compacta (sin contenido completo).
type ObsGist struct {
	ID       string `json:"id"`
	TopicKey string `json:"topic_key"`
	Gist     string `json:"gist"`
}

// EntityContextResult une los hechos y las observaciones relevantes a una entidad.
type EntityContextResult struct {
	Entity       string    `json:"entity"`
	Facts        []Fact    `json:"facts"`
	Observations []ObsGist `json:"observations"`
}

// EntityContext devuelve los hechos del grafo alrededor de entity (hasta maxHops
// saltos, maxFacts hechos) y los gists de hasta maxObs observaciones que la
// mencionan (vía FTS, excluyendo archivadas).
func (e *DbEngine) EntityContext(entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error) {
	if maxObs <= 0 {
		maxObs = defaultMaxObservations
	}

	graph, err := e.RecallFacts(entity, maxHops, maxFacts)
	if err != nil {
		return EntityContextResult{}, err
	}

	obs, err := e.observationGists(entity, maxObs)
	if err != nil {
		return EntityContextResult{}, err
	}

	return EntityContextResult{
		Entity:       entity,
		Facts:        graph.Facts,
		Observations: obs,
	}, nil
}

// observationGists devuelve los gists de las observaciones (no archivadas) que
// mencionan el texto dado, ordenadas por relevancia FTS.
func (e *DbEngine) observationGists(query string, limit int) ([]ObsGist, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return []ObsGist{}, nil
	}

	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, COALESCE(o.gist, '')
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND o.archived = 0 AND o.superseded_by IS NULL
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("error al buscar observaciones de la entidad: %w", err)
	}
	defer rows.Close()

	out := []ObsGist{}
	for rows.Next() {
		var g ObsGist
		if err := rows.Scan(&g.ID, &g.TopicKey, &g.Gist); err != nil {
			return nil, fmt.Errorf("error al escanear gist: %w", err)
		}
		out = append(out, g)
	}
	return out, nil
}
