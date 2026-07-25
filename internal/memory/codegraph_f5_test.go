package memory

import (
	"context"
	"testing"
)

// oneFileGraph arma un grafo mínimo de un archivo (un file-node + una func) para un fingerprint dado.
func oneFileGraph(path, fp string) ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{
		{Key: path, Kind: "file", Name: path, Path: path, SrcFingerprint: fp},
		{Key: path + "#func:Do", Kind: "func", Name: "Do", Path: path, SrcFingerprint: fp},
	}
	edges := []GraphEdge{
		{FromKey: path, ToKey: path + "#func:Do", Kind: "CONTAINS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: path, SrcFingerprint: fp},
	}
	return nodes, edges
}

// TestPruneGraphFilesProjectIsolation valida que podar un archivo en un proyecto (F5) NO toca las
// filas de otro tenant con el mismo path (R6).
func TestPruneGraphFilesProjectIsolation(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	n, ed := oneFileGraph("svc.go", "fp")
	if err := e.UpsertPackageGraphFrom("crm", []string{"svc.go"}, n, ed); err != nil {
		t.Fatal(err)
	}
	if err := e.UpsertPackageGraphFrom("web", []string{"svc.go"}, n, ed); err != nil {
		t.Fatal(err)
	}

	pruned, err := e.PruneGraphFilesFrom("crm", []string{"svc.go"})
	if err != nil {
		t.Fatal(err)
	}
	if pruned < 1 {
		t.Errorf("esperaba podar >=1 nodo de crm, got %d", pruned)
	}

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})
	if _, ok, _ := e.GetGraphNodeCtx(crm, "svc.go#func:Do"); ok {
		t.Error("crm: el nodo debería haberse podado")
	}
	if _, ok, _ := e.GetGraphNodeCtx(web, "svc.go#func:Do"); !ok {
		t.Error("web: el nodo NO debería tocarse (aislamiento de tenant)")
	}
}

// TestGraphFileFingerprintsCtx valida que se devuelven los path→fingerprint del grafo, scopeados.
func TestGraphFileFingerprintsCtx(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	n, ed := oneFileGraph("svc.go", "fp-123")
	if err := e.UpsertPackageGraphFrom("crm", []string{"svc.go"}, n, ed); err != nil {
		t.Fatal(err)
	}
	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	fps, err := e.GraphFileFingerprintsCtx(crm)
	if err != nil {
		t.Fatal(err)
	}
	if fps["svc.go"] != "fp-123" {
		t.Errorf("esperaba svc.go → fp-123, got %q (mapa %v)", fps["svc.go"], fps)
	}
}
