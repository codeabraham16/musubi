package mcp

import (
	"context"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestCodegraphPushRedactaConForceRedact cierra la hoja que T17.2 dejó abierta (Tramo 0 · M01):
// el central redactaba todo lo que entraba de a uno (save_fact, save_code, ingest) y NADA de lo
// que entraba a granel por musubi_codegraph_push. Con forceRedact (bind compartido), un gist o
// un nombre de símbolo con una clave adentro tienen que quedar tapados en el central; sin
// forceRedact (loopback local) el texto queda crudo, porque el dev local necesita el texto real.
func TestCodegraphPushRedactaConForceRedact(t *testing.T) {
	const secret = "AKIA1234567890ABCDEF" // regla aws-access-key, sintética, no allowlisted

	nodosDe := func(t *testing.T, s *McpServer, proyecto string) []memory.GraphNode {
		t.Helper()
		ctx := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: proyecto})
		n, err := s.engine.AllGraphNodesCtx(ctx)
		if err != nil {
			t.Fatalf("AllGraphNodesCtx(%s): %v", proyecto, err)
		}
		return n
	}

	for _, tc := range []struct {
		name        string
		forceRedact bool
		wantRaw     bool
	}{
		{"central (forceRedact) tapa el secreto", true, false},
		{"loopback (sin forceRedact) lo deja crudo", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			s.forceRedact = tc.forceRedact
			pushear(t, s, map[string]interface{}{
				"nodes": []memory.GraphNode{
					{Key: "cfg/aws.go#const:KEY", Kind: "const", Name: secret, Path: "cfg/aws.go", StartLine: 3, EndLine: 3},
				},
				"edges": []memory.GraphEdge{},
				"gists": []memory.CodeMemory{
					{Path: "cfg/aws.go", Gist: "carga la credencial " + secret + " hardcodeada", Symbols: "KEY," + secret, Fingerprint: "f1", Tokens: 9},
				},
			})

			gists := gistsDe(t, s, "musubi")
			if len(gists) != 1 {
				t.Fatalf("esperaba 1 gist bajo 'musubi', hay %d", len(gists))
			}
			if got := strings.Contains(gists[0].Gist, secret); got != tc.wantRaw {
				t.Errorf("gist: secreto crudo presente=%v, esperaba %v — %q", got, tc.wantRaw, gists[0].Gist)
			}
			if got := strings.Contains(gists[0].Symbols, secret); got != tc.wantRaw {
				t.Errorf("symbols: secreto crudo presente=%v, esperaba %v — %q", got, tc.wantRaw, gists[0].Symbols)
			}
			// Path es la clave del upsert (path, project_id): NUNCA se redacta, o el re-push no
			// encontraría su propia fila.
			if gists[0].Path != "cfg/aws.go" {
				t.Errorf("el path del gist es estructural y no debía cambiar: %q", gists[0].Path)
			}

			nodos := nodosDe(t, s, "musubi")
			if len(nodos) != 1 {
				t.Fatalf("esperaba 1 nodo bajo 'musubi', hay %d", len(nodos))
			}
			if got := strings.Contains(nodos[0].Name, secret); got != tc.wantRaw {
				t.Errorf("nodo.name: secreto crudo presente=%v, esperaba %v — %q", got, tc.wantRaw, nodos[0].Name)
			}
			// La Key es el identificador que las aristas usan como from/to: se conserva tal cual.
			if nodos[0].Key != "cfg/aws.go#const:KEY" {
				t.Errorf("la key del nodo es estructural y no debía cambiar: %q", nodos[0].Key)
			}
		})
	}
}
