package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestConflictsEnforcePrincipalScope valida el aislamiento de musubi_conflicts (Track 17
// T17.1b-2): un principal acotado solo ve los conflictos cuya observación de ORIGEN (source) es
// de SU proyecto; un admin ve federado. Guard de no-bleed de la superficie de conflictos.
func TestConflictsEnforcePrincipalScope(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	obs := func(origin, id string) {
		if err := engine.SaveObservationTypedFrom(origin, "", id, "t/x", "contenido "+id, 1, "", memory.ScopeLocal, nil); err != nil {
			t.Fatal(err)
		}
	}
	obs("crm", "crm-src")
	obs("crm", "crm-tgt")
	obs("web", "web-src")
	obs("web", "web-tgt")
	rel := func(src, tgt string) {
		if _, err := engine.UpsertObsRelation(memory.ObsRelation{
			SourceID: src, TargetID: tgt, Relation: memory.RelPending, Status: memory.RelStatusPending,
		}); err != nil {
			t.Fatal(err)
		}
	}
	rel("crm-src", "crm-tgt")
	rel("web-src", "web-tgt")

	sees := func(p *Principal) string {
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_conflicts", Arguments: json.RawMessage(`{}`)})
		ctx := context.Background()
		if p != nil {
			ctx = withPrincipal(ctx, p)
		}
		out, rpcErr := s.handleToolsCall(ctx, params)
		if rpcErr != nil {
			t.Fatalf("conflicts: %+v", rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}

	// writer acotado a crm ⇒ solo el conflicto con source crm-src, NO el de web.
	crm := sees(&Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"})
	if !strings.Contains(crm, "crm-src") || strings.Contains(crm, "web-src") {
		t.Errorf("conflicts writer/crm: esperaba crm-src SIN web-src, obtuve %s", crm)
	}
	// admin ⇒ federado, ve ambos.
	all := sees(&Principal{Name: "root", Role: RoleAdmin})
	if !strings.Contains(all, "crm-src") || !strings.Contains(all, "web-src") {
		t.Errorf("conflicts admin: esperaba ambos, obtuve %s", all)
	}
}
