package memory

import (
	"math"
	"testing"
)

// saveObs crea una observación viva con id explícito (sin embedding).
func saveObs(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	if err := e.SaveObservation(id, "topic", "contenido de "+id, nil); err != nil {
		t.Fatalf("SaveObservation(%s): %v", id, err)
	}
}

// relate crea (o actualiza) una relación semántica entre dos observaciones.
func relate(t *testing.T, e *DbEngine, source, target, relation string) {
	t.Helper()
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: source, TargetID: target, Relation: relation}); err != nil {
		t.Fatalf("UpsertObsRelation(%s->%s): %v", source, target, err)
	}
}

// Escenario (a): el HUB de un cluster relacionado es más central que un nodo periférico.
func TestGraphCentralityHubRanksAbovePeriphery(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"H", "P", "L1", "L2", "L3", "L4"} {
		saveObs(t, e, id)
	}
	// H es hub (conectado a 4 hojas); P es periférico (conectado a 1 hoja).
	relate(t, e, "H", "L1", RelRelated)
	relate(t, e, "H", "L2", RelRelated)
	relate(t, e, "H", "L3", RelRelated)
	relate(t, e, "H", "L4", RelRelated)
	relate(t, e, "P", "L1", RelRelated)

	ranks, err := e.graphCentralityRank([]string{"P", "H"}) // orden de entrada invertido a propósito
	if err != nil {
		t.Fatalf("graphCentralityRank: %v", err)
	}
	rH, okH := ranks["H"]
	rP, okP := ranks["P"]
	if !okH || !okP {
		t.Fatalf("H y P deben estar rankeados, obtuve %v", ranks)
	}
	if rH >= rP {
		t.Errorf("el hub H debe ser más central (rank menor) que P: rank[H]=%d rank[P]=%d", rH, rP)
	}
}

// Escenario (b): rerank-only — un candidato que no es nodo del grafo no aparece en el ranking.
func TestGraphCentralityRerankOnly(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"H", "P", "L1", "outsider"} {
		saveObs(t, e, id)
	}
	relate(t, e, "H", "L1", RelRelated)
	relate(t, e, "P", "L1", RelRelated)
	// 'outsider' existe pero no tiene ninguna relación: no es nodo del grafo.
	ranks, err := e.graphCentralityRank([]string{"H", "P", "outsider"})
	if err != nil {
		t.Fatalf("graphCentralityRank: %v", err)
	}
	if _, ok := ranks["outsider"]; ok {
		t.Error("un candidato sin relaciones no debe aparecer en el ranking (rerank-only)")
	}
	if len(ranks) != 2 {
		t.Errorf("solo H y P deben rankearse, obtuve %v", ranks)
	}
}

// Escenario (c): empate de score → desempate determinista por id ascendente.
func TestGraphCentralityDeterministicTiebreak(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b", "c"} {
		saveObs(t, e, id)
	}
	// a y b son simétricos: ambos conectados solo a c ⇒ mismo score PPR.
	relate(t, e, "a", "c", RelRelated)
	relate(t, e, "b", "c", RelRelated)
	// Correr dos veces con órdenes de entrada distintos: el resultado debe ser idéntico.
	r1, _ := e.graphCentralityRank([]string{"a", "b"})
	r2, _ := e.graphCentralityRank([]string{"b", "a"})
	if r1["a"] != 0 || r1["b"] != 1 {
		t.Errorf("empate debe desempatar por id asc (a=0,b=1), obtuve %v", r1)
	}
	if r2["a"] != r1["a"] || r2["b"] != r1["b"] {
		t.Errorf("el ranking no debe depender del orden de entrada: %v vs %v", r1, r2)
	}
}

// Escenario (d): grafo sin aristas → no-op (mapa vacío ⇒ equivalencia con el histórico).
func TestGraphCentralityNoEdgesNoop(t *testing.T) {
	e := newTestEngine(t)
	saveObs(t, e, "a")
	saveObs(t, e, "b")
	ranks, err := e.graphCentralityRank([]string{"a", "b"})
	if err != nil {
		t.Fatalf("graphCentralityRank: %v", err)
	}
	if len(ranks) != 0 {
		t.Errorf("sin aristas la señal debe ser no-op, obtuve %v", ranks)
	}
}

// Escenario (e): menos de 2 candidatos → no-op.
func TestGraphCentralityFewCandidatesNoop(t *testing.T) {
	e := newTestEngine(t)
	saveObs(t, e, "a")
	ranks, err := e.graphCentralityRank([]string{"a"})
	if err != nil {
		t.Fatalf("graphCentralityRank: %v", err)
	}
	if len(ranks) != 0 {
		t.Errorf("con <2 candidatos la señal debe ser no-op, obtuve %v", ranks)
	}
}

// Escenario (f): menos de 2 candidatos PRESENTES en el grafo → no-op (señal comparativa).
func TestGraphCentralitySingleSeedNoop(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "z", "b"} {
		saveObs(t, e, id)
	}
	relate(t, e, "a", "z", RelRelated) // 'a' es nodo del grafo; 'b' no
	ranks, err := e.graphCentralityRank([]string{"a", "b"})
	if err != nil {
		t.Fatalf("graphCentralityRank: %v", err)
	}
	if len(ranks) != 0 {
		t.Errorf("con <2 candidatos en el grafo la señal debe ser no-op, obtuve %v", ranks)
	}
}

// Escenario (g): las aristas a nodos MUERTOS (archivados/superseded) no cuentan.
func TestGraphCentralityIgnoresDeadNodes(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b", "dead"} {
		saveObs(t, e, id)
	}
	relate(t, e, "a", "b", RelRelated)
	relate(t, e, "a", "dead", RelRelated)
	// Archivar 'dead': su arista debe desaparecer del grafo vivo.
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = ?`, "dead"); err != nil {
		t.Fatal(err)
	}
	g, err := e.buildObsGraph()
	if err != nil {
		t.Fatalf("buildObsGraph: %v", err)
	}
	if _, ok := g.index["dead"]; ok {
		t.Error("un nodo archivado no debe formar parte del grafo vivo")
	}
	if g.n() != 2 {
		t.Errorf("el grafo vivo debe tener solo a y b, obtuve %d nodos", g.n())
	}
}

// TestPPRPowerIterationConservesMass verifica el kernel compartido: con un restart que suma 1,
// el vector de rango también suma ~1 (masa conservada, incluso con nodos colgantes reinyectando
// vía restart). Cubre el camino multi-seed que usa el grafo de observaciones.
func TestPPRPowerIterationConservesMass(t *testing.T) {
	// Línea 0-1-2 con un nodo colgante 3 (sin aristas).
	adj := [][]int{{1}, {0, 2}, {1}, nil}
	n := len(adj)
	restart := make([]float64, n)
	for i := range restart {
		restart[i] = 1.0 / float64(n) // multi-seed uniforme
	}
	r := pprPowerIteration(adj, restart)
	sum := 0.0
	for _, v := range r {
		if v < 0 {
			t.Errorf("los scores PPR no deben ser negativos, obtuve %v", r)
		}
		sum += v
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Errorf("la masa debe conservarse (~1.0), obtuve %v", sum)
	}
}
