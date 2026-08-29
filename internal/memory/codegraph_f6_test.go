package memory

import (
	"context"
	"testing"
)

// TestReplaceProjectGraphIsolatesAndReplaces valida el receptor de la federación (Track 20 · F6):
// ReplaceProjectGraphFrom (1) reemplaza el grafo COMPLETO de un proyecto, (2) es idempotente y
// (3) NO toca el de otro proyecto — el invariante crítico de aislamiento por tenant (R3/E2) en la
// capa de persistencia. Reusa el helper vizNode de codegraph_viz_test.go.
func TestReplaceProjectGraphIsolatesAndReplaces(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	nodesA := []GraphNode{
		vizNode("a.go#func:A1", "func", "A1", "a.go"),
		vizNode("a.go#func:A2", "func", "A2", "a.go"),
	}
	edgesA := []GraphEdge{{FromKey: "a.go#func:A1", ToKey: "a.go#func:A2", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "1"}}
	nodesB := []GraphNode{vizNode("b.go#func:B1", "func", "B1", "b.go")}

	if err := e.ReplaceProjectGraphFrom("projA", nodesA, edgesA); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectGraphFrom("projB", nodesB, nil); err != nil {
		t.Fatal(err)
	}

	ctxA := WithProjectScope(context.Background(), ProjectScope{ProjectID: "projA"})
	ctxB := WithProjectScope(context.Background(), ProjectScope{ProjectID: "projB"})

	// Cada proyecto ve SOLO lo suyo.
	if nA, _ := e.AllGraphNodesCtx(ctxA); len(nA) != 2 {
		t.Errorf("projA debería ver 2 nodos, ve %d", len(nA))
	}
	if eA, _ := e.AllGraphEdgesCtx(ctxA); len(eA) != 1 {
		t.Errorf("projA debería ver 1 arista, ve %d", len(eA))
	}
	if nB, _ := e.AllGraphNodesCtx(ctxB); len(nB) != 1 || nB[0].Name != "B1" {
		t.Errorf("projB debería ver solo B1, ve %+v", nB)
	}

	// Idempotencia (R4/E5): re-empujar el MISMO grafo deja el estado igual.
	if err := e.ReplaceProjectGraphFrom("projA", nodesA, edgesA); err != nil {
		t.Fatal(err)
	}
	if nA, _ := e.AllGraphNodesCtx(ctxA); len(nA) != 2 {
		t.Errorf("tras re-push idéntico, projA sigue con 2 nodos, ve %d", len(nA))
	}

	// Full-replace real: A pasa a un único nodo distinto ⇒ los viejos DESAPARECEN (sin drift),
	// y B queda INTACTO (aislamiento tras reemplazar A).
	if err := e.ReplaceProjectGraphFrom("projA", []GraphNode{vizNode("a.go#func:A3", "func", "A3", "a.go")}, nil); err != nil {
		t.Fatal(err)
	}
	nA3, _ := e.AllGraphNodesCtx(ctxA)
	if len(nA3) != 1 || nA3[0].Name != "A3" {
		t.Errorf("full-replace: projA debería tener solo A3, tiene %+v", nA3)
	}
	if eA3, _ := e.AllGraphEdgesCtx(ctxA); len(eA3) != 0 {
		t.Errorf("full-replace: la arista vieja de projA debería haberse ido, quedan %d", len(eA3))
	}
	if nB, _ := e.AllGraphNodesCtx(ctxB); len(nB) != 1 || nB[0].Name != "B1" {
		t.Errorf("aislamiento: projB intacto tras reemplazar A, ve %+v", nB)
	}
}
