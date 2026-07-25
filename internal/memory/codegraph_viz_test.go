package memory

import (
	"context"
	"testing"
)

func vizNode(key, kind, name, path string) GraphNode {
	return GraphNode{Key: key, Kind: kind, Name: name, Path: path, SrcFingerprint: "1"}
}

// TestCodeGraphVizRanksAndFilters valida la lente código (F5-bonus): nodos ordenados por
// centralidad (grado), módulo derivado del path, cap por límite (con Truncated + TotalNodes) y
// aristas sin colgantes (ambos extremos incluidos).
func TestCodeGraphVizRanksAndFilters(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	nodes := []GraphNode{
		vizNode("pkg/a.go#func:Hub", "func", "Hub", "pkg/a.go"),
		vizNode("pkg/a.go#func:Leaf1", "func", "Leaf1", "pkg/a.go"),
		vizNode("pkg/b.go#func:Leaf2", "func", "Leaf2", "pkg/b.go"),
	}
	edges := []GraphEdge{
		{FromKey: "pkg/a.go#func:Hub", ToKey: "pkg/a.go#func:Leaf1", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
		{FromKey: "pkg/a.go#func:Hub", ToKey: "pkg/b.go#func:Leaf2", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
	}
	if err := e.UpsertPackageGraph([]string{"pkg/a.go", "pkg/b.go"}, nodes, edges); err != nil {
		t.Fatal(err)
	}

	viz, err := e.CodeGraphViz(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(viz.Nodes) != 3 {
		t.Fatalf("esperaba 3 nodos, got %d", len(viz.Nodes))
	}
	if viz.Nodes[0].Name != "Hub" || viz.Nodes[0].Degree != 2 {
		t.Errorf("el hub (grado 2) debería ir primero, got name=%q deg=%d", viz.Nodes[0].Name, viz.Nodes[0].Degree)
	}
	if viz.Nodes[0].Module != "pkg" {
		t.Errorf("el módulo de Hub debería ser 'pkg', got %q", viz.Nodes[0].Module)
	}
	if len(viz.Edges) != 2 {
		t.Errorf("esperaba 2 aristas, got %d", len(viz.Edges))
	}

	// Cap a 1: solo el hub; sus aristas quedan colgantes (targets excluidos) → 0 aristas.
	capped, err := e.CodeGraphViz(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Nodes) != 1 || !capped.Truncated {
		t.Errorf("cap a 1 debería truncar a 1 nodo, got %d trunc=%v", len(capped.Nodes), capped.Truncated)
	}
	if len(capped.Edges) != 0 {
		t.Errorf("con solo el hub no debería quedar ninguna arista (ambos extremos incluidos), got %d", len(capped.Edges))
	}
	if capped.TotalNodes != 3 {
		t.Errorf("TotalNodes debería ser 3 (pre-cap), got %d", capped.TotalNodes)
	}
}

// TestCodeGraphVizEmptyNonNil: sin grafo, Nodes/Edges son slices no-nil (JSON [] y no null).
func TestCodeGraphVizEmptyNonNil(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	viz, err := e.CodeGraphViz(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if viz.Nodes == nil || viz.Edges == nil {
		t.Errorf("Nodes/Edges deberían ser slices no-nil, got nodes=%v edges=%v", viz.Nodes, viz.Edges)
	}
}

// TestExplainedByWeld: una decisión que menciona el símbolo/archivo se solda por FTS (F3).
func TestExplainedByWeld(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.SaveObservation("obs1", "arq/hub", "Decisión sobre Hub en pkg/a.go: se hace así por rendimiento.", nil); err != nil {
		t.Fatal(err)
	}
	exp, err := e.ExplainedBy(context.Background(), "pkg/a.go", "Hub", 5)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range exp {
		if x.TopicKey == "arq/hub" {
			found = true
		}
	}
	if !found {
		t.Errorf("ExplainedBy debería soldar la decisión arq/hub, got %+v", exp)
	}
}
