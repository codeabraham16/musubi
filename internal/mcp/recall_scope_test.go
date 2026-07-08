package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func TestRecallScopeFor(t *testing.T) {
	// admin ⇒ federado (ve todo).
	if scope, fed := recallScopeFor(&Principal{Role: RoleAdmin}); scope != "" || !fed {
		t.Errorf("admin: scope=%q federate=%v, esperaba \"\",true", scope, fed)
	}
	// writer con proyecto ⇒ acotado a su proyecto.
	if scope, fed := recallScopeFor(&Principal{Role: RoleWriter, ProjectID: "crm"}); scope != "crm" || fed {
		t.Errorf("writer/crm: scope=%q federate=%v, esperaba \"crm\",false", scope, fed)
	}
	// reader sin proyecto ⇒ sin scope (federado, histórico).
	if scope, fed := recallScopeFor(&Principal{Role: RoleReader}); scope != "" || fed {
		t.Errorf("reader sin proyecto: scope=%q federate=%v, esperaba \"\",false", scope, fed)
	}
	// nil (stdio local) ⇒ sin scope.
	if scope, fed := recallScopeFor(nil); scope != "" || fed {
		t.Errorf("nil: scope=%q federate=%v, esperaba \"\",false", scope, fed)
	}
}

// TestToolRecallEnforcesPrincipalScope valida el enforcement end-to-end: un writer acotado a
// un proyecto solo recupera memoria de ESE proyecto (más la sin atribuir), mientras un admin
// ve todos los proyectos — todo derivado del principal en el contexto, sin que el cliente lo pida.
func TestToolRecallEnforcesPrincipalScope(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Memoria de dos proyectos + una sin atribuir, con un término común buscable.
	seed := func(origin, id, content string) {
		if err := engine.SaveObservationTypedFrom(origin, id, "t/x", content, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	seed("crm", "c1", "deploy crm qtoken")
	seed("web", "w1", "deploy web qtoken")
	seed("", "u1", "deploy libre qtoken")

	recallIDs := func(p *Principal) map[string]bool {
		raw, _ := json.Marshal(map[string]any{"query": "qtoken"})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_recall", Arguments: raw})
		ctx := context.Background()
		if p != nil {
			ctx = withPrincipal(ctx, p)
		}
		out, rpcErr := s.handleToolsCall(ctx, params)
		if rpcErr != nil {
			t.Fatalf("recall: %+v", rpcErr)
		}
		// jsonResult envuelve el JSON como texto en CallToolResponse.
		resp := out.(CallToolResponse)
		var res memory.RecallResult
		if err := json.Unmarshal([]byte(resp.Content[0].Text), &res); err != nil {
			t.Fatalf("parse recall result: %v", err)
		}
		ids := make(map[string]bool, len(res.Items))
		for _, it := range res.Items {
			ids[it.ID] = true
		}
		return ids
	}

	// writer acotado a crm ⇒ ve c1 + u1 (sin atribuir), NO w1.
	got := recallIDs(&Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"})
	if !got["c1"] || !got["u1"] || got["w1"] {
		t.Errorf("writer/crm: esperaba {c1,u1} sin w1, obtuve %v", got)
	}

	// admin ⇒ federado, ve los tres.
	got = recallIDs(&Principal{Name: "root", Role: RoleAdmin})
	if !got["c1"] || !got["w1"] || !got["u1"] {
		t.Errorf("admin: esperaba {c1,w1,u1}, obtuve %v", got)
	}
}
