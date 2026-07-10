package memory

import (
	"context"
	"fmt"
)

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

// EntityContext ensambla el contexto en el espacio FEDERADO (histórico). Fino wrapper
// sobre EntityContextCtx con contexto vacío.
func (e *DbEngine) EntityContext(entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error) {
	return e.EntityContextCtx(context.Background(), entity, maxHops, maxFacts, maxObs)
}

// EntityContextCtx devuelve los hechos del grafo alrededor de entity (hasta maxHops saltos,
// maxFacts hechos) y los gists de hasta maxObs observaciones que la mencionan (vía FTS,
// excluyendo archivadas), TODO acotado al proyecto del contexto (Track 17): tanto los hechos
// (vía RecallFactsCtx) como las observaciones (vía observationGistsCtx) se scopean, de modo que
// entity_context no filtra ni prosa ni aristas de otros proyectos.
func (e *DbEngine) EntityContextCtx(ctx context.Context, entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error) {
	if maxObs <= 0 {
		maxObs = defaultMaxObservations
	}

	graph, err := e.RecallFactsCtx(ctx, entity, maxHops, maxFacts, "", "")
	if err != nil {
		return EntityContextResult{}, err
	}

	obs, err := e.observationGistsCtx(ctx, entity, maxObs)
	if err != nil {
		return EntityContextResult{}, err
	}

	return EntityContextResult{
		Entity:       entity,
		Facts:        graph.Facts,
		Observations: obs,
	}, nil
}

// observationGistsCtx devuelve los gists de las observaciones (no archivadas) que mencionan el
// texto dado, ordenadas por relevancia FTS, acotadas al proyecto del contexto (Track 17). Ctx
// federado ⇒ sin filtro de proyecto (histórico). El scope se concatena al predicado de
// visibilidad, con sus args entre el término FTS y el LIMIT.
func (e *DbEngine) observationGistsCtx(ctx context.Context, query string, limit int) ([]ObsGist, error) {
	ftsQuery := buildFTSQueryRanked(query)
	if ftsQuery == "" {
		return []ObsGist{}, nil
	}

	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("o")
	args := make([]interface{}, 0, 2+len(scopeArgs))
	args = append(args, ftsQuery)
	args = append(args, scopeArgs...)
	args = append(args, limit)

	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, COALESCE(o.gist, '')
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ? AND `+visibleObsPredicate+scopeSQL+`
		ORDER BY rank
		LIMIT ?
	`, args...)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar gists de entidad: %w", err)
	}
	return out, nil
}
