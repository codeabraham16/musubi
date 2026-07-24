package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// decodeCG decodifica el JSON de una respuesta MCP a un mapa.
func decodeCG(t *testing.T, res interface{}) map[string]interface{} {
	t.Helper()
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %#v", res)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &m); err != nil {
		t.Fatalf("no se pudo decodear: %v\ntext=%s", err, resp.Content[0].Text)
	}
	return m
}

func containsInAny(v interface{}, want string) bool {
	arr, ok := v.([]interface{})
	if !ok {
		return false
	}
	for _, x := range arr {
		if s, ok := x.(string); ok && s == want {
			return true
		}
	}
	return false
}

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

// TestCodegraphQueryToolsE2E ejercita el índice de repo + las 3 tools de consulta + staleness.
func TestCodegraphQueryToolsE2E(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	s := newTestServerWithPath(t, dir)

	// Index de todo el repo.
	idx := decodeCG(t, mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{}))
	if n, _ := idx["nodes"].(float64); n <= 0 {
		t.Fatalf("el índice debería poblar nodos, got %v", idx["nodes"])
	}

	// code_graph sobre Alpha: beta en callees.
	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph", map[string]interface{}{"symbol": "pkg/a.go#func:Alpha"}))
	if cg["found"] != true {
		t.Fatalf("Alpha debería encontrarse: %v", cg)
	}
	if !containsInAny(cg["callees"], "pkg/a.go#func:beta") {
		t.Errorf("callees de Alpha debería incluir beta, got %v", cg["callees"])
	}

	// impact sobre beta: Alpha en callers.
	im := decodeCG(t, mustCall(t, s, "musubi_impact", map[string]interface{}{"symbol": "pkg/a.go#func:beta"}))
	if !containsInAny(im["callers"], "pkg/a.go#func:Alpha") {
		t.Errorf("impact de beta debería incluir Alpha, got %v", im["callers"])
	}

	// map: nodos > 0 y algún entry point.
	mp := decodeCG(t, mustCall(t, s, "musubi_map", map[string]interface{}{}))
	if n, _ := mp["nodes"].(float64); n <= 0 {
		t.Errorf("map debería reportar nodos, got %v", mp["nodes"])
	}

	// staleness: mutar a.go sin re-indexar ⇒ el nodo se reporta stale.
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\n// cambio\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	cg2 := decodeCG(t, mustCall(t, s, "musubi_code_graph", map[string]interface{}{"symbol": "pkg/a.go#func:Alpha"}))
	node, _ := cg2["node"].(map[string]interface{})
	if node == nil || node["stale"] != true {
		t.Errorf("tras mutar el archivo, Alpha debería reportarse stale, got node=%v", node)
	}
}

// mustCall corre una tool y falla si devuelve error JSON-RPC.
func mustCall(t *testing.T, s *McpServer, name string, args map[string]interface{}) interface{} {
	t.Helper()
	res, rpcErr := call(t, s, name, args)
	if rpcErr != nil {
		t.Fatalf("%s: %+v", name, rpcErr)
	}
	return res
}

// TestCodeContextWeldsMemory valida el puente código↔memoria (F3): estructura + explained_by.
func TestCodeContextWeldsMemory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	s := newTestServerWithPath(t, dir)
	mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{})

	// Una decisión guardada que menciona el símbolo Alpha y su archivo.
	if err := s.engine.SaveObservation("obs-alpha", "arq/alpha",
		"Decisión sobre Alpha en pkg/a.go: se implementa así por rendimiento.", nil); err != nil {
		t.Fatal(err)
	}

	// code_context: estructura (callees) + el porqué (explained_by).
	cc := decodeCG(t, mustCall(t, s, "musubi_code_context", map[string]interface{}{"symbol": "pkg/a.go#func:Alpha"}))
	if cc["found"] != true {
		t.Fatalf("Alpha debería encontrarse: %v", cc)
	}
	if !containsInAny(cc["callees"], "pkg/a.go#func:beta") {
		t.Errorf("callees debería incluir beta: %v", cc["callees"])
	}
	if !containsInAny(cc["explained_by"], "arq/alpha") {
		t.Errorf("explained_by debería soldar la decisión arq/alpha: %v", cc["explained_by"])
	}

	// Símbolo sin nodo: found=false pero explained_by se computa igual (por el path del arg).
	cc2 := decodeCG(t, mustCall(t, s, "musubi_code_context", map[string]interface{}{"symbol": "pkg/a.go#func:Nope"}))
	if cc2["found"] != false {
		t.Errorf("un símbolo inexistente debería dar found=false, got %v", cc2["found"])
	}
	if !containsInAny(cc2["explained_by"], "arq/alpha") {
		t.Errorf("aún sin nodo, explained_by por path debería incluir arq/alpha: %v", cc2["explained_by"])
	}
}
