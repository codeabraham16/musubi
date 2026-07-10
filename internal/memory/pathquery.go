package memory

import (
	"context"
	"database/sql"
	"fmt"
)

// pathquery.go responde "¿cómo se conecta X con Y?": el CAMINO más corto (cadena de hechos)
// entre dos entidades en el grafo. Complementa a RecallFacts (vecindad BFS) y al recall
// asociativo por PageRank (pagerank.go). MODEL-FREE y Go-puro: BFS determinista, sin LLM.
// Compone con lo bi-temporal: usa el mismo filtro temporal que RecallFacts, así que el camino
// puede pedirse point-in-time (as_of).

// pathEdge es una arista viva incidente a un nodo: la relación (relID), el hecho original
// (tripleta sin reordenar) y el nodo del otro extremo.
type pathEdge struct {
	relID int64
	fact  Fact
	other int64
}

// pathPred registra, para un nodo visitado, por qué arista se llegó a él (para reconstruir el
// camino hacia atrás).
type pathPred struct {
	from int64 // nodo anterior en el camino
	fact Fact  // hecho que conecta from → este nodo
}

// FactPath devuelve los hechos del camino MÁS CORTO en el espacio FEDERADO (histórico). Fino
// wrapper sobre FactPathCtx con contexto vacío.
func (e *DbEngine) FactPath(from, to string, maxHops int, asOf string) (GraphResult, error) {
	return e.FactPathCtx(context.Background(), from, to, maxHops, asOf)
}

// FactPathCtx devuelve los hechos del camino MÁS CORTO (en número de aristas) entre las entidades
// from y to sobre el grafo VIVO no dirigido VISIBLE AL PROYECTO del contexto (Track 17), en orden
// desde from hacia to. Respeta el mismo filtro combinado (temporal + scope) que RecallFactsCtx,
// así que el camino se puede pedir point-in-time (as_of) y sólo cruza aristas del proyecto (o sin
// atribuir). Sin camino dentro de maxHops, entidades inexistentes o from==to → GraphResult vacío
// (sin error).
func (e *DbEngine) FactPathCtx(ctx context.Context, from, to string, maxHops int, asOf string) (GraphResult, error) {
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}
	result := GraphResult{Entity: from, Hops: maxHops, Facts: []Fact{}}

	fromID, ok, err := e.entityID(from)
	if err != nil {
		return GraphResult{}, err
	}
	if !ok {
		return result, nil
	}
	toID, ok, err := e.entityID(to)
	if err != nil {
		return GraphResult{}, err
	}
	if !ok || fromID == toID {
		// Entidad destino inexistente o camino trivial (misma entidad): sin hechos.
		return result, nil
	}

	// Filtro combinado temporal + scope de proyecto, idéntico a RecallFactsCtx.
	liveFilter, filterArgs := liveFactFilter(ctx, asOf)

	// BFS por niveles con predecesores. maxHops acota la longitud (número de aristas).
	pred := map[int64]pathPred{fromID: {}}
	frontier := []int64{fromID}
	for hop := 0; hop < maxHops && len(frontier) > 0; hop++ {
		var next []int64
		for _, node := range frontier {
			edges, err := e.pathNeighbors(node, liveFilter, filterArgs)
			if err != nil {
				return GraphResult{}, err
			}
			for _, ed := range edges {
				if _, seen := pred[ed.other]; seen {
					continue // ya visitado (BFS garantiza que fue por un camino ≤): no revisitar
				}
				pred[ed.other] = pathPred{from: node, fact: ed.fact}
				if ed.other == toID {
					result.Facts = reconstructPath(pred, fromID, toID)
					result.Count = len(result.Facts)
					return result, nil
				}
				next = append(next, ed.other)
			}
		}
		frontier = next
	}

	// Sin camino dentro de maxHops.
	return result, nil
}

// entityID resuelve el id de una entidad por su nombre normalizado. ok=false si no existe.
func (e *DbEngine) entityID(name string) (int64, bool, error) {
	var id int64
	err := e.db.QueryRow(`SELECT id FROM entities WHERE norm = ?`, normalizeForSim(name)).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("error al resolver entidad %q: %w", name, err)
	}
	return id, true, nil
}

// pathNeighbors devuelve las aristas vivas (respetando el filtro temporal) que tocan nodeID,
// con el hecho original y el nodo del otro extremo. ORDER BY r.id → expansión determinista.
func (e *DbEngine) pathNeighbors(nodeID int64, liveFilter string, filterArgs []interface{}) ([]pathEdge, error) {
	args := make([]interface{}, 0, 2+len(filterArgs))
	args = append(args, nodeID, nodeID)
	args = append(args, filterArgs...)

	rows, err := e.db.Query(`
		SELECT r.id, ef.name, r.predicate, et.name, r.from_id, r.to_id
		FROM relations r
		JOIN entities ef ON r.from_id = ef.id
		JOIN entities et ON r.to_id = et.id
		WHERE (r.from_id = ? OR r.to_id = ?) AND (`+liveFilter+`)
		ORDER BY r.id
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("error al expandir vecinos del camino: %w", err)
	}
	defer rows.Close()

	var edges []pathEdge
	for rows.Next() {
		var (
			relID           int64
			subj, pred, obj string
			fromID, toID    int64
		)
		if err := rows.Scan(&relID, &subj, &pred, &obj, &fromID, &toID); err != nil {
			return nil, fmt.Errorf("error al escanear arista del camino: %w", err)
		}
		other := toID
		if toID == nodeID {
			other = fromID
		}
		if other == nodeID {
			continue // self-loop: no aporta al camino
		}
		edges = append(edges, pathEdge{relID: relID, fact: Fact{Subject: subj, Predicate: pred, Object: obj}, other: other})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar vecinos del camino: %w", err)
	}
	return edges, nil
}

// reconstructPath sigue los predecesores desde toID hacia fromID y devuelve los hechos en
// orden desde from hacia to (invirtiendo la cadena recolectada hacia atrás).
func reconstructPath(pred map[int64]pathPred, fromID, toID int64) []Fact {
	var reversed []Fact
	for cur := toID; cur != fromID; {
		p := pred[cur]
		reversed = append(reversed, p.fact)
		cur = p.from
	}
	// Invertir: recolectamos de to→from, queremos from→to.
	facts := make([]Fact, len(reversed))
	for i, f := range reversed {
		facts[len(reversed)-1-i] = f
	}
	return facts
}
