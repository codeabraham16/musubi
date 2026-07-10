package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestReadSurfacesEnforcePrincipalScope valida el wiring principal→scope (Track 17) end-to-end
// en las superficies de lectura directas: un writer acotado a un proyecto solo ve su proyecto
// (+ lo sin atribuir); un admin ve todo. Cubre search_keyword y memory_expand (no necesitan
// embedder; la semántica se cubre a nivel motor en internal/memory/scope_isolation_test.go).
func TestReadSurfacesEnforcePrincipalScope(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	seed := func(origin, id string) {
		if err := engine.SaveObservationTypedFrom(origin, id, "t/x", "shared qtoken content", 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	seed("crm", "iso-crm")
	seed("web", "iso-web")
	seed("", "iso-free") // sin atribuir: visible para todos
	all := []string{"iso-crm", "iso-web", "iso-free"}

	// present devuelve qué ids sembrados aparecen en la respuesta de la tool (los ids son
	// distintivos y no viven en el contenido, así que un substring es un chequeo fiable).
	present := func(tool string, args map[string]any, p *Principal) map[string]bool {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
		ctx := context.Background()
		if p != nil {
			ctx = withPrincipal(ctx, p)
		}
		out, rpcErr := s.handleToolsCall(ctx, params)
		if rpcErr != nil {
			t.Fatalf("%s: %+v", tool, rpcErr)
		}
		text := out.(CallToolResponse).Content[0].Text
		got := map[string]bool{}
		for _, id := range all {
			if strings.Contains(text, id) {
				got[id] = true
			}
		}
		return got
	}

	writer := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	admin := &Principal{Name: "root", Role: RoleAdmin}

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"musubi_search_keyword", map[string]any{"query_text": "qtoken"}},
		{"musubi_memory_expand", map[string]any{"ids": all}},
	} {
		// writer acotado a crm ⇒ ve iso-crm + iso-free, NUNCA iso-web (otro proyecto).
		g := present(tc.tool, tc.args, writer)
		if !g["iso-crm"] || !g["iso-free"] || g["iso-web"] {
			t.Errorf("%s writer/crm: esperaba {iso-crm,iso-free} SIN iso-web, obtuve %v", tc.tool, g)
		}
		// admin ⇒ federado, ve los tres (guard de que el filtro no rompe el modo legacy).
		g = present(tc.tool, tc.args, admin)
		if !g["iso-crm"] || !g["iso-web"] || !g["iso-free"] {
			t.Errorf("%s admin: esperaba los 3, obtuve %v", tc.tool, g)
		}
	}
}
