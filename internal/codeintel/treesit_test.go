//go:build treesitter

package codeintel

import "testing"

// Corre solo con -tags treesitter (+ grammar_subset_*). Valida las aristas TS/Py vía tree-sitter.
func TestDerivePolyglot(t *testing.T) {
	files := map[string]string{
		"pkg/a.ts": "import { helper } from './util';\n\nexport function Alpha() {\n  beta();\n  helper();\n}\n\nfunction beta() {}\n",
		"pkg/b.py": "import os\nfrom util import helper\n\ndef alpha():\n    beta()\n    helper()\n\ndef beta():\n    pass\n",
	}
	g := DerivePackage("pkg", files, "example.com/mod")

	// TS: símbolos + CONTAINS + CALLS intra-archivo (Alpha→beta); helper es import, no genera CALLS.
	if findNode(g, "pkg/a.ts#func:Alpha") == nil || findNode(g, "pkg/a.ts#func:beta") == nil {
		t.Error("faltan los símbolos TS (Alpha, beta)")
	}
	if !hasEdge(g, "pkg/a.ts", "pkg/a.ts#func:Alpha", EdgeContains) {
		t.Error("falta CONTAINS a.ts → Alpha")
	}
	if !hasEdge(g, "pkg/a.ts#func:Alpha", "pkg/a.ts#func:beta", EdgeCalls) {
		t.Error("falta CALLS Alpha → beta (TS)")
	}

	// Py: símbolos + CALLS intra-archivo (alpha→beta) + IMPORTS.
	if findNode(g, "pkg/b.py#func:alpha") == nil || findNode(g, "pkg/b.py#func:beta") == nil {
		t.Error("faltan los símbolos Py (alpha, beta)")
	}
	if !hasEdge(g, "pkg/b.py#func:alpha", "pkg/b.py#func:beta", EdgeCalls) {
		t.Error("falta CALLS alpha → beta (Py)")
	}
	imports := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeImports {
			imports++
		}
	}
	if imports == 0 {
		t.Error("esperaba al menos una arista IMPORTS de los archivos Py/TS")
	}
}
