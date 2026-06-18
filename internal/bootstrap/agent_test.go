package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgent(t *testing.T) {
	// vacío → claude (default histórico)
	if a, ok := ResolveAgent(""); !ok || a.Name != "claude" || !a.SupportsHooks {
		t.Errorf("agente vacío debería ser claude con hooks, fue %+v ok=%v", a, ok)
	}
	// cursor → sin hooks, MCP en .cursor/mcp.json
	a, ok := ResolveAgent("Cursor")
	if !ok || a.Name != "cursor" || a.SupportsHooks {
		t.Errorf("cursor debería existir sin hooks, fue %+v ok=%v", a, ok)
	}
	if a.MCPPath != filepath.Join(".cursor", "mcp.json") {
		t.Errorf("MCPPath de cursor inesperado: %q", a.MCPPath)
	}
	// desconocido → !ok
	if _, ok := ResolveAgent("emacs-overlord"); ok {
		t.Error("agente desconocido debería devolver ok=false")
	}
}

func TestDetectAgents(t *testing.T) {
	root := t.TempDir()
	if got := DetectAgents(root); len(got) != 0 {
		t.Errorf("proyecto vacío no debería detectar agentes, obtuve %v", got)
	}
	if err := os.MkdirAll(filepath.Join(root, ".cursor"), 0755); err != nil {
		t.Fatal(err)
	}
	got := DetectAgents(root)
	if len(got) != 1 || got[0] != "cursor" {
		t.Errorf("esperaba detectar [cursor], obtuve %v", got)
	}
}

func TestKnownAgentNames(t *testing.T) {
	names := KnownAgentNames()
	if len(names) < 2 {
		t.Errorf("esperaba al menos claude y cursor, obtuve %v", names)
	}
}
