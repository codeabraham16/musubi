package memory

import (
	"context"
	"testing"
)

// seedGraph siembra un grafo chico: main→helper→worker (CALLS) con CONTAINS de a.go.
func seedGraph(t *testing.T, e *DbEngine) {
	t.Helper()
	nodes := []GraphNode{
		{Key: "a.go", Kind: "file", Name: "a.go", Path: "a.go", SrcFingerprint: "1"},
		{Key: "a.go#func:main", Kind: "func", Name: "main", Path: "a.go", SrcFingerprint: "1"},
		{Key: "a.go#func:helper", Kind: "func", Name: "helper", Path: "a.go", SrcFingerprint: "1"},
		{Key: "b.go#func:worker", Kind: "func", Name: "worker", Path: "b.go", SrcFingerprint: "1"},
	}
	edges := []GraphEdge{
		{FromKey: "a.go", ToKey: "a.go#func:main", Kind: "CONTAINS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "1"},
		{FromKey: "a.go", ToKey: "a.go#func:helper", Kind: "CONTAINS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "1"},
		{FromKey: "a.go#func:main", ToKey: "a.go#func:helper", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "1"},
		{FromKey: "a.go#func:helper", ToKey: "b.go#func:worker", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "1"},
	}
	if err := e.UpsertPackageGraphFrom("", []string{"a.go", "b.go"}, nodes, edges); err != nil {
		t.Fatal(err)
	}
}

func TestCodeGraphQueries(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	seedGraph(t, e)
	ctx := context.Background()

	// InEdges de worker: lo llama helper.
	in, err := e.GraphInEdgesCtx(ctx, "b.go#func:worker")
	if err != nil || len(in) != 1 || in[0].FromKey != "a.go#func:helper" || in[0].Kind != "CALLS" {
		t.Errorf("in-edges de worker mal: %+v err=%v", in, err)
	}

	// Impacto transitivo de worker: helper (directo) y main (transitivo).
	imp, err := e.GraphImpactCtx(ctx, "b.go#func:worker", 5, 100)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, k := range imp {
		set[k] = true
	}
	if !set["a.go#func:helper"] || !set["a.go#func:main"] {
		t.Errorf("el impacto de worker debería incluir helper y main, got %v", imp)
	}

	// Stats: 4 nodos; 2 CALLS, 2 CONTAINS.
	n, byKind, err := e.GraphStatsCtx(ctx)
	if err != nil || n != 4 || byKind["CALLS"] != 2 || byKind["CONTAINS"] != 2 {
		t.Errorf("stats mal: nodes=%d byKind=%v err=%v", n, byKind, err)
	}

	// God-nodes por grado CALLS: helper tiene grado 2 (1 in + 1 out) → primero.
	top, err := e.GraphTopByDegreeCtx(ctx, 3)
	if err != nil || len(top) == 0 || top[0].Key != "a.go#func:helper" {
		t.Errorf("god-nodes mal: %+v err=%v", top, err)
	}

	// Entry points: main (nadie lo llama). helper y worker SÍ son llamados.
	ep, err := e.GraphEntryPointsCtx(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	epset := map[string]bool{}
	for _, k := range ep {
		epset[k] = true
	}
	if !epset["a.go#func:main"] || epset["a.go#func:helper"] || epset["b.go#func:worker"] {
		t.Errorf("entry points mal (esperaba solo main): %v", ep)
	}

	// Símbolos de a.go: main y helper (NO el nodo file).
	syms, err := e.ListGraphNodesForFileCtx(ctx, "a.go")
	if err != nil || len(syms) != 2 {
		t.Errorf("símbolos de a.go: esperaba 2 (main, helper), got %+v err=%v", syms, err)
	}
	for _, sy := range syms {
		if sy.Kind == "file" {
			t.Errorf("ListGraphNodesForFileCtx no debe incluir el nodo file: %+v", sy)
		}
	}
}

// TestCodeGraphProjectIsolation valida el aislamiento multi-tenant del grafo de código: dos
// proyectos con el MISMO node_key coexisten (UNIQUE por project_id) y cada credencial lee el
// suyo, prefiriendo el propio sobre la fila sin atribuir.
func TestCodeGraphProjectIsolation(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	na := GraphNode{Key: "x.go#func:Foo", Kind: "func", Name: "Foo", Path: "x.go", SrcFingerprint: "fpA"}
	nb := GraphNode{Key: "x.go#func:Foo", Kind: "func", Name: "Foo", Path: "x.go", SrcFingerprint: "fpB"}
	if err := e.UpsertPackageGraphFrom("crm", []string{"x.go"}, []GraphNode{na}, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.UpsertPackageGraphFrom("web", []string{"x.go"}, []GraphNode{nb}, nil); err != nil {
		t.Fatal(err)
	}

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})
	if n, ok, err := e.GetGraphNodeCtx(crm, "x.go#func:Foo"); err != nil || !ok || n.SrcFingerprint != "fpA" {
		t.Errorf("crm debería leer fpA, got ok=%v fp=%q err=%v", ok, n.SrcFingerprint, err)
	}
	if n, ok, err := e.GetGraphNodeCtx(web, "x.go#func:Foo"); err != nil || !ok || n.SrcFingerprint != "fpB" {
		t.Errorf("web debería leer fpB, got ok=%v fp=%q err=%v", ok, n.SrcFingerprint, err)
	}
}

// TestCodeGraphRefreshDeleteBySource valida el refresco incremental: re-derivar UN archivo deja
// stale/actualizadas solo sus filas (por src_fingerprint), sin tocar las de archivos hermanos.
func TestCodeGraphRefreshDeleteBySource(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ctx := context.Background() // federado

	aFoo := GraphNode{Key: "a.go#func:Foo", Kind: "func", Name: "Foo", Path: "a.go", SrcFingerprint: "a1"}
	bBar := GraphNode{Key: "b.go#func:Bar", Kind: "func", Name: "Bar", Path: "b.go", SrcFingerprint: "b1"}
	call := GraphEdge{FromKey: "a.go#func:Foo", ToKey: "b.go#func:Bar", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "a.go", SrcFingerprint: "a1"}
	if err := e.UpsertPackageGraphFrom("", []string{"a.go", "b.go"}, []GraphNode{aFoo, bBar}, []GraphEdge{call}); err != nil {
		t.Fatal(err)
	}

	// Refrescar SOLO a.go con nuevo fingerprint.
	aFoo2 := aFoo
	aFoo2.SrcFingerprint = "a2"
	call2 := call
	call2.SrcFingerprint = "a2"
	if err := e.UpsertPackageGraphFrom("", []string{"a.go"}, []GraphNode{aFoo2}, []GraphEdge{call2}); err != nil {
		t.Fatal(err)
	}

	if n, ok, _ := e.GetGraphNodeCtx(ctx, "a.go#func:Foo"); !ok || n.SrcFingerprint != "a2" {
		t.Errorf("a.go debería re-derivarse a a2, got ok=%v fp=%q", ok, n.SrcFingerprint)
	}
	if n, ok, _ := e.GetGraphNodeCtx(ctx, "b.go#func:Bar"); !ok || n.SrcFingerprint != "b1" {
		t.Errorf("b.go (hermano no tocado) debería quedar intacto en b1, got ok=%v fp=%q", ok, n.SrcFingerprint)
	}
	edges, err := e.GraphOutEdgesCtx(ctx, "a.go#func:Foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].ToKey != "b.go#func:Bar" || edges[0].SrcFingerprint != "a2" {
		t.Errorf("la arista CALLS debería sobrevivir re-insertada con fp a2, got %+v", edges)
	}
}
