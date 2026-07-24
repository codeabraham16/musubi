package memory

import (
	"context"
	"testing"
)

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
