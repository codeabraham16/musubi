package memory

import (
	"fmt"
	"sort"
)

// obsrank.go implementa la 5ª SEÑAL RRF del recall de observaciones: centralidad de grafo por
// Personalized PageRank sobre observation_relations (estilo HippoRAG, "spreading activation").
// MODEL-FREE y Go-puro: sólo aritmética (reusa el kernel pprPowerIteration de pagerank.go),
// cero LLM, cero cgo. La fusión RRF de recall.go ya combina keyword+recencia+frecuencia+
// semántica vectorial; ésta agrega la centralidad asociativa: una observación que es HUB de un
// cluster relacionado (many related/supersedes/conflicts_with) sube en el ranking aunque el
// FTS/vector no la priorizara.
//
// "DERIVAR, NO GUARDAR-Y-DESFASAR": el ranking se computa al vuelo desde observation_relations;
// no hay tabla de scores que mantener. RERANK-ONLY: NO agrega candidatos nuevos al pool (a
// diferencia del pool vectorial de augmentWithVectorPool), sólo reordena el pool existente.

// obsGraph es el grafo de observaciones en memoria: nodos = observaciones vivas, aristas =
// relaciones semánticas vivas. La adyacencia es NO DIRIGIDA porque la asociación de memoria es
// simétrica: si A supersede/related B, B es relevante partiendo de A y viceversa.
type obsGraph struct {
	ids   []string       // id de observación por posición
	index map[string]int // id de observación → posición
	adj   [][]int        // posición → posiciones vecinas (no dirigido, con multiplicidad)
}

// n es la cantidad de nodos.
func (g *obsGraph) n() int { return len(g.ids) }

// buildObsGraph carga el grafo de relaciones semánticas VIVO en una adyacencia dispersa: sólo
// aristas cuyas dos puntas son observaciones no archivadas ni superseded (una arista a un nodo
// muerto no aporta a la centralidad del recall, que sólo rankea observaciones vivas). Cada
// relación aporta una arista no dirigida; los self-loops (source==target) se omiten (no aportan
// a la difusión y sesgarían el grado).
func (e *DbEngine) buildObsGraph() (*obsGraph, error) {
	g := &obsGraph{index: map[string]int{}}

	// nodeIdx devuelve la posición del id, registrándolo la primera vez.
	nodeIdx := func(id string) int {
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
		SELECT r.source_id, r.target_id
		FROM observation_relations r
		JOIN observations s ON r.source_id = s.id
		JOIN observations t ON r.target_id = t.id
		WHERE s.archived = 0 AND s.superseded_by IS NULL
		  AND t.archived = 0 AND t.superseded_by IS NULL
		ORDER BY r.id
	`)
	if err != nil {
		return nil, fmt.Errorf("error al construir el grafo de observaciones: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sourceID, targetID string
		if err := rows.Scan(&sourceID, &targetID); err != nil {
			return nil, fmt.Errorf("error al escanear arista de observaciones: %w", err)
		}
		if sourceID == targetID {
			continue // self-loop: irrelevante para la difusión
		}
		a := nodeIdx(sourceID)
		b := nodeIdx(targetID)
		g.adj[a] = append(g.adj[a], b)
		g.adj[b] = append(g.adj[b], a) // no dirigido
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar aristas de observaciones: %w", err)
	}
	return g, nil
}

// graphCentralityRank computa el ranking de centralidad de los candidatos por Personalized
// PageRank sobre el grafo de observaciones. Siembra el restart UNIFORME sobre los candidatos que
// son nodos del grafo (mantiene la señal INDEPENDIENTE del ranking léxico/vectorial —no se
// ponderan las semillas por su score de recall— para que RRF fusione señales realmente
// independientes), difunde sobre el grafo COMPLETO (así un candidato conectado a otros vía un
// intermediario no-candidato igual acumula centralidad), y devuelve el rango SÓLO de los
// candidatos (id→posición, 0 = más central), ordenados por score desc con desempate por id asc
// (determinista). RERANK-ONLY: no incorpora nodos no-candidatos al resultado.
//
// No-op seguro (devuelve mapa vacío ⇒ scoreCandidates omite el término ⇒ equivalencia con el
// histórico): menos de 2 candidatos, grafo sin aristas, o menos de 2 candidatos presentes en el
// grafo (la centralidad es un signo COMPARATIVO: entre <2 nodos es degenerada).
func (e *DbEngine) graphCentralityRank(candidateIDs []string) (map[string]int, error) {
	empty := map[string]int{}
	if len(candidateIDs) < 2 {
		return empty, nil
	}

	g, err := e.buildObsGraph()
	if err != nil {
		return nil, err
	}
	if g.n() == 0 {
		return empty, nil // grafo sin aristas: nada que rankear
	}

	// Semillas = candidatos que son nodos del grafo. La masa de restart se reparte uniforme
	// entre ellas (suma 1, conserva la masa del PPR).
	var seedPos []int
	for _, id := range candidateIDs {
		if pos, ok := g.index[id]; ok {
			seedPos = append(seedPos, pos)
		}
	}
	if len(seedPos) < 2 {
		return empty, nil // 0 o 1 candidato en el grafo: sin señal comparativa
	}

	restart := make([]float64, g.n())
	mass := 1.0 / float64(len(seedPos))
	for _, pos := range seedPos {
		restart[pos] = mass
	}
	r := pprPowerIteration(g.adj, restart)

	// Recoger el score PPR de cada candidato presente en el grafo y rankearlos entre sí.
	type candScore struct {
		id    string
		score float64
	}
	scored := make([]candScore, 0, len(seedPos))
	for _, id := range candidateIDs {
		if pos, ok := g.index[id]; ok {
			scored = append(scored, candScore{id: id, score: r[pos]})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].id < scored[j].id // desempate determinista
	})

	ranks := make(map[string]int, len(scored))
	for i, c := range scored {
		ranks[c.id] = i
	}
	return ranks, nil
}
