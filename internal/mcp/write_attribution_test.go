package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestWriteAttributionFromPrincipal valida que la ATRIBUCIÓN de escritura se derive de la
// CREDENCIAL (Track 17 — cierra el write-poisoning cross-tenant, simétrico al aislamiento de
// lectura de T17.1a): un writer acotado a un proyecto NO puede atribuir una observación a otro
// proyecto declarando project_id; su credencial manda. admin/legacy sí fija el origen explícito
// (ingest del central). Se comprueba vía la visibilidad de lectura ya acotada por proyecto.
func TestWriteAttributionFromPrincipal(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	save := func(p *Principal, id, declaredProject string) {
		args, _ := json.Marshal(map[string]any{
			"id": id, "topic_key": "t/x", "content": "poison qtoken content", "project_id": declaredProject,
		})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_save_observation", Arguments: args})
		if _, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params); rpcErr != nil {
			t.Fatalf("save %s: %+v", id, rpcErr)
		}
	}
	// sees usa search_keyword (acotado por proyecto desde T17.1a): el marker es el id, distintivo.
	sees := func(p *Principal, marker string) bool {
		args, _ := json.Marshal(map[string]any{"query_text": "qtoken"})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_search_keyword", Arguments: args})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("search: %+v", rpcErr)
		}
		return strings.Contains(out.(CallToolResponse).Content[0].Text, marker)
	}

	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	web := &Principal{Name: "bob", Role: RoleWriter, ProjectID: "web"}
	admin := &Principal{Name: "root", Role: RoleAdmin}

	// Un writer/crm intenta atribuir a "web": debe quedar en crm (su credencial), NO en web.
	save(crm, "poison-1", "web")
	if sees(web, "poison-1") {
		t.Error("write-poisoning: writer/crm logró atribuir a 'web' (lo ve el tenant web)")
	}
	if !sees(crm, "poison-1") {
		t.Error("la observación debería quedar atribuida a crm (la ve crm)")
	}

	// admin puede fijar el origen explícito (ingest del central): atribuye a 'web' de verdad.
	save(admin, "central-1", "web")
	if !sees(web, "central-1") {
		t.Error("admin debería poder atribuir a 'web' (ingest del central)")
	}
}
