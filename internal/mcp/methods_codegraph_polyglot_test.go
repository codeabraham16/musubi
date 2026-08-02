//go:build treesitter

package mcp

import (
	"context"
	"path/filepath"
	"testing"
)

// Corre sólo con -tags treesitter (+ grammar_subset_*): prueba la CADENA COMPLETA del indexador
// para lenguajes no-Go. Antes, refreshCodeGraphForPackage filtraba a `.go` y el pase polyglot de
// DerivePackage nunca veía los TS/JS/Py; ahora el walker/refresh usan codeintel.IndexableForGraph,
// así que un proyecto TypeScript SÍ puebla el grafo (símbolos, CONTAINS y CALLS intra-archivo).
func TestRefreshCodeGraphPolyglotTS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "app.ts"),
		"import { helper } from './util';\n\nexport function Alpha() {\n  beta();\n  helper();\n}\n\nfunction beta() {}\n")

	s := newTestServerWithPath(t, dir)
	if err := s.refreshCodeGraphForPackage(context.Background(), "src"); err != nil {
		t.Fatalf("refresh error: %v", err)
	}

	ctx := context.Background()
	if _, ok, _ := s.engine.GetGraphNodeCtx(ctx, "src/app.ts#func:Alpha"); !ok {
		t.Error("el grafo debería tener el símbolo TS func:Alpha (indexador cableado a polyglot)")
	}
	edges, err := s.engine.GraphOutEdgesCtx(ctx, "src/app.ts#func:Alpha")
	if err != nil {
		t.Fatal(err)
	}
	var hasCall bool
	for _, e := range edges {
		if e.Kind == "CALLS" && e.ToKey == "src/app.ts#func:beta" {
			hasCall = true
		}
	}
	if !hasCall {
		t.Errorf("falta CALLS Alpha→beta en TS, edges=%+v", edges)
	}
}

// El índice full del repo (musubi_codegraph_index) también debe poblar un repo TypeScript puro,
// que antes daba 0 nodos (el walker sólo juntaba .go).
func TestCodegraphIndexPolyglotRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.js"), "export function main() { boot(); }\nfunction boot() {}\n")
	writeFile(t, filepath.Join(dir, "lib", "svc.py"), "def run():\n    step()\n\ndef step():\n    pass\n")

	s := newTestServerWithPath(t, dir)
	idx := decodeCG(t, mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{}))
	if n, _ := idx["nodes"].(float64); n <= 0 {
		t.Fatalf("un repo JS/Py debería poblar nodos con treesitter, got %v", idx["nodes"])
	}
	if _, ok, _ := s.engine.GetGraphNodeCtx(context.Background(), "index.js#func:main"); !ok {
		t.Error("el índice full debería incluir el símbolo JS func:main")
	}
}
