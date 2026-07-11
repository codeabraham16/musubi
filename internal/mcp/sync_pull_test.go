package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestToolSyncPullScoped valida el tool musubi_sync_pull (C5.3b): devuelve la memoria 'shared' del
// proyecto de la CREDENCIAL (aislamiento T17-19; nada cross-tenant), en orden de rowid, y respeta el
// cursor after_rowid para paginar. Es el server side del sync entrante.
func TestToolSyncPullScoped(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(engine.SaveObservationTypedFrom("acme", "ana", "a1", "t/a", "alpha del equipo", 1, "semantic", "shared", nil))
	must(engine.SaveObservationTypedFrom("web", "bob", "w1", "t/w", "cosa de web", 1, "semantic", "shared", nil))
	must(engine.SaveObservationTypedFrom("acme", "juan", "a2", "t/a", "beta del equipo", 1, "semantic", "shared", nil))

	pull := func(p *Principal, after int64) (items []memory.SharedObs, next int64) {
		args, _ := json.Marshal(map[string]any{"after_rowid": after, "limit": 100})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_sync_pull", Arguments: args})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("pull: %+v", rpcErr)
		}
		var resp struct {
			Items      []memory.SharedObs `json:"items"`
			NextCursor int64              `json:"next_cursor"`
		}
		if uErr := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &resp); uErr != nil {
			t.Fatalf("unmarshal respuesta: %v", uErr)
		}
		return resp.Items, resp.NextCursor
	}

	acme := &Principal{Name: "ana", Role: RoleWriter, ProjectID: "acme"}

	// Pull de acme: solo a1 y a2 (shared de acme), en orden de rowid. w1 (otro proyecto) NO cruza.
	items, next := pull(acme, 0)
	if len(items) != 2 || items[0].ID != "a1" || items[1].ID != "a2" {
		t.Fatalf("pull de acme esperaba [a1,a2], obtuve %+v", items)
	}
	for _, it := range items {
		if it.ID == "w1" {
			t.Error("fuga cross-tenant: w1 apareció en el pull de acme")
		}
	}
	if items[0].Author != "ana" || items[1].Author != "juan" {
		t.Errorf("author esperado [ana,juan], obtuve [%q,%q]", items[0].Author, items[1].Author)
	}

	// Cursor: la página siguiente (tras el mayor rowid) viene vacía.
	items2, _ := pull(acme, next)
	if len(items2) != 0 {
		t.Errorf("segunda página tras el cursor esperaba vacío, obtuve %+v", items2)
	}
}
