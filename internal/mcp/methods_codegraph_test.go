package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeFile es un helper local para armar el proyecto de prueba.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshCodeGraphForPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"),
		"package pkg\n\nimport \"fmt\"\n\nfunc Alpha() {\n\tbeta()\n\tfmt.Println()\n}\n\nfunc beta() {}\n")
	writeFile(t, filepath.Join(dir, "pkg", "b.go"), "package pkg\n\ntype Server struct{}\n")

	s := newTestServerWithPath(t, dir)
	if err := s.refreshCodeGraphForPackage(context.Background(), "pkg"); err != nil {
		t.Fatalf("refresh error: %v", err)
	}

	ctx := context.Background()
	if _, ok, _ := s.engine.GetGraphNodeCtx(ctx, "pkg/a.go#func:Alpha"); !ok {
		t.Error("el grafo debería tener el nodo func:Alpha")
	}
	if n, ok, _ := s.engine.GetGraphNodeCtx(ctx, "pkg/b.go#type:Server"); !ok || n.Kind != "type" {
		t.Errorf("el grafo debería tener el nodo type:Server, got ok=%v kind=%q", ok, n.Kind)
	}

	edges, err := s.engine.GraphOutEdgesCtx(ctx, "pkg/a.go#func:Alpha")
	if err != nil {
		t.Fatal(err)
	}
	var hasCall bool
	for _, e := range edges {
		if e.Kind == "CALLS" && e.ToKey == "pkg/a.go#func:beta" {
			hasCall = true
			if e.SrcFingerprint == "" {
				t.Error("la arista CALLS debería llevar su src_fingerprint estampado")
			}
		}
	}
	if !hasCall {
		t.Errorf("falta CALLS Alpha→beta, edges=%+v", edges)
	}
}

func TestSaveCodeTriggersGraph(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "m.go"), "package main\n\nfunc Helper() {}\n\nfunc main() { Helper() }\n")

	s := newTestServerWithPath(t, dir)
	if _, rpcErr := call(t, s, "musubi_save_code", map[string]interface{}{"path": "m.go", "gist": "main"}); rpcErr != nil {
		t.Fatalf("save_code error: %+v", rpcErr)
	}

	// El guardado del gist de un .go debe haber poblado el grafo del paquete (best-effort).
	ctx := context.Background()
	if _, ok, _ := s.engine.GetGraphNodeCtx(ctx, "m.go#func:main"); !ok {
		t.Error("save_code de un .go debería poblar el grafo (nodo func:main ausente)")
	}
	edges, _ := s.engine.GraphOutEdgesCtx(ctx, "m.go#func:main")
	var hasCall bool
	for _, e := range edges {
		if e.Kind == "CALLS" && e.ToKey == "m.go#func:Helper" {
			hasCall = true
		}
	}
	if !hasCall {
		t.Errorf("esperaba CALLS main→Helper por el trigger de save_code, edges=%+v", edges)
	}
}
