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

// TestCodeGraphVizDeclaraTotalEdges es el simétrico del de nodos, y el que faltaba: el test
// de arriba ya comprobaba que al capar las aristas caen a CERO, pero no exigía que se
// declarara cuántas se habían perdido. Así congelaba el silencio: el fix se podía revertir
// sin que nada se pusiera rojo.
//
// INVARIANTE: TotalEdges y EdgesTruncated describen las aristas igual que TotalNodes y
// Truncated describen los nodos. TotalModules cuenta el grafo completo, no la muestra.
func TestCodeGraphVizDeclaraTotalEdges(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	nodes := []GraphNode{
		vizNode("pkg/a.go#func:Hub", "func", "Hub", "pkg/a.go"),
		vizNode("pkg/a.go#func:Leaf1", "func", "Leaf1", "pkg/a.go"),
		vizNode("otro/b.go#func:Leaf2", "func", "Leaf2", "otro/b.go"),
	}
	edges := []GraphEdge{
		{FromKey: "pkg/a.go#func:Hub", ToKey: "pkg/a.go#func:Leaf1", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
		{FromKey: "pkg/a.go#func:Hub", ToKey: "otro/b.go#func:Leaf2", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
	}
	if err := e.UpsertPackageGraph([]string{"pkg/a.go", "otro/b.go"}, nodes, edges); err != nil {
		t.Fatal(err)
	}

	// Cap a 1: sólo el hub, ninguna arista dibujable. Pero el grafo TIENE 2.
	cap1, err := e.CodeGraphViz(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cap1.Edges) != 0 {
		t.Fatalf("con sólo el hub no hay aristas dibujables, got %d", len(cap1.Edges))
	}
	if cap1.TotalEdges != 2 {
		t.Errorf("TotalEdges debe declarar las 2 que el cap dejó afuera, got %d", cap1.TotalEdges)
	}
	if !cap1.EdgesTruncated {
		t.Error("si se dibujan 0 de 2, EdgesTruncated tiene que ser true")
	}
	// El KPI de módulos habla del código indexado, no de lo que entró en pantalla: con el
	// cap en 1 sólo se dibuja un nodo del módulo 'pkg', pero los módulos siguen siendo 2.
	if cap1.TotalModules != 2 {
		t.Errorf("TotalModules debe contar el grafo completo (pkg y otro), got %d", cap1.TotalModules)
	}

	// Sin cap, el total coincide con lo dibujado: si no, estaría contando otra cosa.
	full, err := e.CodeGraphViz(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if full.TotalEdges != len(full.Edges) {
		t.Errorf("sin truncado el total debe igualar lo dibujado: total=%d dibujadas=%d", full.TotalEdges, len(full.Edges))
	}
	if full.EdgesTruncated {
		t.Error("sin cap no puede marcar EdgesTruncated")
	}
}

// TestCodeGraphVizTotalEdgesDeduplica: en una lectura FEDERADA (cerebro central, scopeClause
// vacío) la misma arista lógica llega una vez por cada project_id que la tenga. El render las
// colapsa por (kind, from, to), así que el TOTAL tiene que colapsarlas con el mismo criterio
// — si no, el denominador crece con la cantidad de tenants y el numerador no lo alcanza nunca.
func TestCodeGraphVizTotalEdgesDeduplica(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	nodes := []GraphNode{
		vizNode("pkg/a.go#func:Hub", "func", "Hub", "pkg/a.go"),
		vizNode("pkg/a.go#func:Leaf1", "func", "Leaf1", "pkg/a.go"),
	}
	edges := []GraphEdge{
		{FromKey: "pkg/a.go#func:Hub", ToKey: "pkg/a.go#func:Leaf1", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
	}
	if err := e.UpsertPackageGraph([]string{"pkg/a.go"}, nodes, edges); err != nil {
		t.Fatal(err)
	}
	// La MISMA arista lógica, bajo otro project_id. El UNIQUE de la tabla es por proyecto,
	// así que esto es una fila legítima y la lectura federada devuelve las dos.
	if _, err := e.db.Exec(`INSERT INTO code_graph_edges
		(project_id, from_key, to_key, kind, confidence, provenance, src_path, src_fingerprint)
		VALUES ('otro-tenant', 'pkg/a.go#func:Hub', 'pkg/a.go#func:Leaf1', 'CALLS', 1, 'EXTRACTED', 'pkg/a.go', '1')`); err != nil {
		t.Fatal(err)
	}

	viz, err := e.CodeGraphViz(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(viz.Edges) != 1 {
		t.Fatalf("el render colapsa la arista repetida, got %d", len(viz.Edges))
	}
	if viz.TotalEdges != 1 {
		t.Errorf("TotalEdges debe deduplicar con el MISMO criterio que el render: esperaba 1, got %d", viz.TotalEdges)
	}
	if viz.EdgesTruncated {
		t.Error("colapsar una repetida no es truncado: no debe marcar EdgesTruncated")
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
