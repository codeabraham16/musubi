//go:build !treesitter

package codeintel

import "testing"

// En el build POR DEFAULT (sin -tags treesitter), los archivos TS/Py quedan solo-símbolos: el
// grafo NO emite nodos/aristas para ellos (comportamiento histórico intacto, binario lean).
func TestDerivePolyglotDisabledByDefault(t *testing.T) {
	files := map[string]string{
		"pkg/a.ts": "export function Alpha() { beta() }\nfunction beta() {}\n",
		"pkg/b.py": "def alpha():\n    beta()\ndef beta():\n    pass\n",
	}
	g := DerivePackage("pkg", files, "example.com/mod")
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Errorf("sin -tags treesitter, TS/Py no deben emitir grafo: %d nodos, %d aristas", len(g.Nodes), len(g.Edges))
	}
}
