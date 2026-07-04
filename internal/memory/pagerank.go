package memory

import (
	"fmt"
	"sort"
)

// pagerank.go implementa recall asociativo por Personalized PageRank (PPR) sobre el grafo
// de hechos (estilo HippoRAG). MODEL-FREE y Go-puro: sólo aritmética (power iteration), cero
// LLM, cero cgo. SELF-CONTAINED: no toca el hot path de observaciones (recall.go); se apoya
// en las relaciones ya persistidas.
//
// El BFS de RecallFacts, al toparse con maxFacts, corta en orden de rel.id (arbitrario /
// cronológico), perdiendo los hechos más relevantes que están a 2+ saltos. PPR personalizado
// a la entidad semilla rankea TODAS las entidades por relevancia asociativa (multi-hop vía
// damping) y devuelve primero los hechos más pertinentes.
//
// "DERIVAR, NO GUARDAR-Y-DESFASAR": el ranking se computa al vuelo desde relations; no hay
// tabla de scores que mantener. Compone con lo bi-temporal: el grafo se arma con el mismo
// filtro temporal que RecallFacts (verdad actual o point-in-time), así que PPR es
// automáticamente point-in-time cuando se pide as_of.

const (
	// pprDamping es el factor de amortiguación canónico de PageRank (probabilidad de seguir
	// una arista vs. reiniciar en la semilla).
	pprDamping = 0.85
	// pprMaxIter acota la power iteration: O(iter*edges), predecible y siempre termina. El
	// residuo contrae ~pprDamping por iteración, así que 200 basta para cruzar pprTol
	// (0.85^200 ≪ 1e-8) en cualquier grafo de memoria realista; el corte por tolerancia
	// suele disparar mucho antes.
	pprMaxIter = 200
	// pprTol es el umbral de convergencia en norma L1 entre iteraciones sucesivas.
	pprTol = 1e-8
)

// pprGraph es el grafo de hechos en memoria: nodos = entidades, aristas = relaciones vivas.
// La adyacencia es NO DIRIGIDA porque la asociación de memoria es simétrica: si (A trabaja_en
// B), B es relevante partiendo de A y viceversa.
type pprGraph struct {
	ids   []int64       // id de entidad por posición
	index map[int64]int // id de entidad → posición
	adj   [][]int       // posición → posiciones vecinas (no dirigido, con multiplicidad)
}

// n es la cantidad de nodos.
func (g *pprGraph) n() int { return len(g.ids) }

// buildFactGraph carga el grafo de hechos vivo (respetando el filtro temporal liveFilter +
// filterArgs, idéntico al de RecallFacts) en una estructura de adyacencia dispersa. Cada
// relación aporta una arista no dirigida entre sujeto y objeto. Los self-loops (from==to) se
// omiten: no aportan a la difusión y podrían sesgar el grado.
func (e *DbEngine) buildFactGraph(liveFilter string, filterArgs []interface{}) (*pprGraph, error) {
	g := &pprGraph{index: map[int64]int{}}

	// nodeIdx devuelve la posición del id, registrándolo la primera vez.
	nodeIdx := func(id int64) int {
		if pos, ok := g.index[id]; ok {
			return pos
		}
		pos := len(g.ids)
		g.index[id] = pos
		g.ids = append(g.ids, id)
		g.adj = append(g.adj, nil)
		return pos
	}

	rows, err := e.db.Query(`
		SELECT r.from_id, r.to_id
		FROM relations r
		WHERE `+liveFilter+`
		ORDER BY r.id
	`, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("error al construir el grafo de hechos: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fromID, toID int64
		if err := rows.Scan(&fromID, &toID); err != nil {
			return nil, fmt.Errorf("error al escanear arista del grafo: %w", err)
		}
		if fromID == toID {
			continue // self-loop: irrelevante para la difusión
		}
		a := nodeIdx(fromID)
		b := nodeIdx(toID)
		g.adj[a] = append(g.adj[a], b)
		g.adj[b] = append(g.adj[b], a) // no dirigido
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar aristas del grafo: %w", err)
	}
	return g, nil
}

// personalizedPageRank corre PPR con reinicio en seedIdx (posición de la entidad semilla).
// Devuelve el score por id de entidad. Power iteration:
//
//	r_{t+1} = (1-d)*p + d * (Aᵀ r_t normalizado por grado)
//
// donde p es el vector de restart personalizado (masa 1.0 concentrada en la semilla). Los
// NODOS COLGANTES (grado 0) redistribuyen su masa al restart p (patrón anti-fuga estándar),
// preservando la estocasticidad. Determinista: sin aleatoriedad, corta por tolerancia L1 o
// pprMaxIter. Grafo vacío o semilla fuera de rango → mapa vacío (sin panic).
func personalizedPageRank(g *pprGraph, seedIdx int) map[int64]float64 {
	n := g.n()
	if n == 0 || seedIdx < 0 || seedIdx >= n {
		return map[int64]float64{}
	}

	// Vector de restart personalizado: toda la masa en la semilla.
	p := make([]float64, n)
	p[seedIdx] = 1.0

	r := make([]float64, n)
	r[seedIdx] = 1.0 // arrancar concentrado en la semilla acelera la convergencia
	next := make([]float64, n)

	for iter := 0; iter < pprMaxIter; iter++ {
		// Masa colgante: la de los nodos sin vecinos se reinyecta vía p.
		dangling := 0.0
		for i := 0; i < n; i++ {
			if len(g.adj[i]) == 0 {
				dangling += r[i]
			}
		}

		// Base: teletransporte (restart) + masa colgante redistribuida al restart.
		base := make([]float64, n)
		for i := 0; i < n; i++ {
			base[i] = (1.0-pprDamping)*p[i] + pprDamping*dangling*p[i]
		}
		copy(next, base)

		// Difusión por aristas: cada nodo reparte su rango entre sus vecinos por grado.
		for i := 0; i < n; i++ {
			deg := len(g.adj[i])
			if deg == 0 {
				continue
			}
			share := pprDamping * r[i] / float64(deg)
			for _, j := range g.adj[i] {
				next[j] += share
			}
		}

		// Convergencia por norma L1.
		diff := 0.0
		for i := 0; i < n; i++ {
			d := next[i] - r[i]
			if d < 0 {
				d = -d
			}
			diff += d
		}
		r, next = next, r
		if diff < pprTol {
			break
		}
	}

	scores := make(map[int64]float64, n)
	for i, id := range g.ids {
		scores[id] = r[i]
	}
	return scores
}

// scoredRel es una relación con su score PPR combinado, para el ordenamiento final.
type scoredRel struct {
	relID int64
	fact  Fact
	score float64
}

// recallFactsPageRank implementa el modo rank='pagerank' de RecallFacts: arma el grafo vivo,
// corre PPR desde startID, puntúa cada hecho vivo por la relevancia asociativa de sus
// extremos (score[from]+score[to]) y devuelve los maxFacts de mayor score. Orden determinista:
// score descendente, con desempate por rel.id ascendente. Respeta el mismo filtro temporal
// que el BFS (liveFilter + filterArgs), de modo que as_of da PageRank point-in-time.
func (e *DbEngine) recallFactsPageRank(startID int64, maxFacts int, liveFilter string, filterArgs []interface{}) (GraphResult, error) {
	result := GraphResult{Entity: "", Facts: []Fact{}}

	g, err := e.buildFactGraph(liveFilter, filterArgs)
	if err != nil {
		return GraphResult{}, err
	}

	seedIdx, ok := g.index[startID]
	if !ok {
		// La semilla no tiene ninguna relación viva: nada que rankear.
		return result, nil
	}
	scores := personalizedPageRank(g, seedIdx)

	// Cargar todos los hechos vivos (mismas columnas que expandFrontier, sin frontera).
	rows, err := e.db.Query(`
		SELECT r.id, ef.name, r.predicate, et.name, r.from_id, r.to_id
		FROM relations r
		JOIN entities ef ON r.from_id = ef.id
		JOIN entities et ON r.to_id = et.id
		WHERE `+liveFilter+`
		ORDER BY r.id
	`, filterArgs...)
	if err != nil {
		return GraphResult{}, fmt.Errorf("error al cargar hechos para pagerank: %w", err)
	}
	defer rows.Close()

	var ranked []scoredRel
	for rows.Next() {
		var (
			relID           int64
			subj, pred, obj string
			fromID, toID    int64
		)
		if err := rows.Scan(&relID, &subj, &pred, &obj, &fromID, &toID); err != nil {
			return GraphResult{}, fmt.Errorf("error al escanear hecho para pagerank: %w", err)
		}
		ranked = append(ranked, scoredRel{
			relID: relID,
			fact:  Fact{Subject: subj, Predicate: pred, Object: obj},
			score: scores[fromID] + scores[toID],
		})
	}
	if err := rows.Err(); err != nil {
		return GraphResult{}, fmt.Errorf("error al iterar hechos para pagerank: %w", err)
	}

	// Orden determinista: score desc, desempate por rel.id asc.
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].relID < ranked[j].relID
	})

	for _, sr := range ranked {
		if len(result.Facts) >= maxFacts {
			break
		}
		result.Facts = append(result.Facts, sr.fact)
	}
	result.Count = len(result.Facts)
	return result, nil
}
