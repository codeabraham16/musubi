package memory

import (
	"testing"
	"time"
)

// insBrainObs inserta una observación con control total de saliencia (importance,
// access_count, created_at) para los tests del grafo neuronal.
func insBrainObs(t *testing.T, e *DbEngine, id, topic string, importance float64, access int, created string) {
	t.Helper()
	_, err := e.db.Exec(`INSERT INTO observations
		(id, topic_key, content, gist, created_at, last_accessed, importance, access_count, archived)
		VALUES (?,?,?,?,?,?,?,?,0)`,
		id, topic, "contenido de "+id, "gist de "+id, created, created, importance, access)
	if err != nil {
		t.Fatalf("insertar obs %s: %v", id, err)
	}
}

func insBrainRel(t *testing.T, e *DbEngine, src, tgt, rel string, conf float64) {
	t.Helper()
	_, err := e.db.Exec(`INSERT INTO observation_relations
		(id, source_id, target_id, relation, confidence, status)
		VALUES (?,?,?,?,?, 'resolved')`,
		src+"->"+tgt, src, tgt, rel, conf)
	if err != nil {
		t.Fatalf("insertar rel %s->%s: %v", src, tgt, err)
	}
}

// TestBrainGraphSalienceAndCap: las neuronas se ordenan por saliencia y se capan a limit,
// exponiendo el total real y truncated (R2).
func TestBrainGraphSalienceAndCap(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	old := now.Add(-400 * 24 * time.Hour).Format(sqliteTimeLayout)

	// alta saliencia: importante y reciente. baja: vieja y sin importancia.
	insBrainObs(t, e, "hi", "a/uno", 3.0, 5, recent)
	insBrainObs(t, e, "mid", "a/dos", 1.0, 0, recent)
	insBrainObs(t, e, "lo", "b/tres", 1.0, 0, old)

	g, err := e.brainGraphAt(now, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.TotalNeurons != 3 {
		t.Fatalf("total esperado 3, obtuve %d", g.TotalNeurons)
	}
	if !g.Truncated {
		t.Error("con 3 neuronas y limit 2 debe marcar truncated")
	}
	if len(g.Neurons) != 2 {
		t.Fatalf("esperaba 2 neuronas tras el cap, obtuve %d", len(g.Neurons))
	}
	if g.Neurons[0].ID != "hi" {
		t.Errorf("la neurona más saliente debe ser 'hi', obtuve %q", g.Neurons[0].ID)
	}
	// 'lo' (vieja, sin importancia) debe quedar fuera del top-2.
	for _, n := range g.Neurons {
		if n.ID == "lo" {
			t.Error("la neurona vieja/sin-peso no debía entrar en el top-2")
		}
	}
	// domain derivado del topic_key.
	if g.Neurons[0].Domain != "a" {
		t.Errorf("domain esperado 'a', obtuve %q", g.Neurons[0].Domain)
	}
}

// TestBrainGraphSynapsesNoDangling: solo se devuelven sinapsis con AMBOS extremos entre
// las neuronas incluidas (R3), incluso si otras relaciones apuntan fuera del set.
func TestBrainGraphSynapsesNoDangling(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	for _, id := range []string{"n1", "n2", "n3"} {
		insBrainObs(t, e, id, "a/"+id, 1.0, 0, ts)
	}
	insBrainRel(t, e, "n1", "n2", "related", 0.9)      // ambos incluidos
	insBrainRel(t, e, "n2", "n3", "conflicts_with", 1) // ambos incluidos
	insBrainRel(t, e, "n1", "ghost", "related", 0.5)   // target inexistente → colgante

	g, err := e.brainGraphAt(now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Synapses) != 2 {
		t.Fatalf("esperaba 2 sinapsis (sin la colgante), obtuve %d: %+v", len(g.Synapses), g.Synapses)
	}
	for _, s := range g.Synapses {
		if s.Target == "ghost" || s.Source == "ghost" {
			t.Error("no debía incluirse la sinapsis colgante hacia 'ghost'")
		}
	}
}

// TestBrainGraphEmpty: una memoria vacía devuelve slices no-nil (JSON [] y no null) y
// no crashea (escenario 'vacío').
func TestBrainGraphEmpty(t *testing.T) {
	e := newTestEngine(t)
	g, err := e.BrainGraph(0)
	if err != nil {
		t.Fatal(err)
	}
	if g.Neurons == nil || g.Synapses == nil {
		t.Error("neurons/synapses deben ser slices no-nil aun con memoria vacía")
	}
	if g.TotalNeurons != 0 || g.Truncated {
		t.Errorf("memoria vacía: total 0 y no truncated, obtuve total=%d truncated=%v", g.TotalNeurons, g.Truncated)
	}
}
