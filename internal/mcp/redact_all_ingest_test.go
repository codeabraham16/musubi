package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestForceRedactCoversAllIngest valida que la redacción forzada server-side cubra TODO ingest
// al central (Track 17 T17.2), no solo save_observation: save_fact y save_code también se
// redactan con forceRedact; sin forceRedact (loopback local) el dev conserva el texto crudo.
func TestForceRedactCoversAllIngest(t *testing.T) {
	const secret = "AKIA1234567890ABCDEF" // regla aws-access-key, no allowlisted

	setup := func(forceRedact bool) (*McpServer, *memory.DbEngine) {
		engine, err := memory.NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = engine.Close() })
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
		s.forceRedact = forceRedact
		return s, engine
	}
	call := func(s *McpServer, tool string, args map[string]any) {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
		if _, rpcErr := s.handleToolsCall(context.Background(), params); rpcErr != nil {
			t.Fatalf("%s: %+v", tool, rpcErr)
		}
	}

	// save_fact: el object con secreto se redacta con forceRedact; queda crudo sin él (control).
	for _, tc := range []struct {
		name        string
		forceRedact bool
		wantRaw     bool
	}{
		{"fact redactado (central)", true, false},
		{"fact crudo (local)", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, engine := setup(tc.forceRedact)
			call(s, "musubi_save_fact", map[string]any{"subject": "svc", "predicate": "usa", "object": secret})
			g, err := engine.RecallFacts("svc", 2, 20, "", "")
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(g)
			if got := strings.Contains(string(raw), secret); got != tc.wantRaw {
				t.Errorf("save_fact: secreto crudo presente=%v, esperaba %v — %s", got, tc.wantRaw, raw)
			}
		})
	}

	// save_code: el gist con secreto se redacta con forceRedact.
	t.Run("code redactado (central)", func(t *testing.T) {
		s, engine := setup(true)
		call(s, "musubi_save_code", map[string]any{"path": "svc.go", "gist": "carga la key " + secret, "symbols": "x"})
		key := memory.NormalizeCodePath(s.projectPath, "svc.go")
		cm, ok, err := engine.GetCodeMemory(key)
		if err != nil || !ok {
			t.Fatalf("GetCodeMemory(%q): ok=%v err=%v", key, ok, err)
		}
		if strings.Contains(cm.Gist, secret) {
			t.Errorf("save_code con forceRedact dejó el secreto crudo en el gist: %q", cm.Gist)
		}
	})
}
