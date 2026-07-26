package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// REGRESIÓN (auditoría 2026-07-26, #8): canCall sólo distinguía lectura vs escritura, así que
// CUALQUIER writer (write=own/any, un miembro normal o un token filtrado) podía disparar maintain y
// doctor(repair) — operaciones DESTRUCTIVAS y GLOBALES (no acotadas por tenant) sobre toda la base.
// Ahora esas dos exigen un principal admin. El diagnóstico de doctor (sin repair) sigue abierto.
func TestMaintainAndDoctorRepairRequireAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	callAs := func(p *Principal, tool string, args map[string]any) *RpcError {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
		_, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		return rpcErr
	}

	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "musubi"}
	admin := &Principal{Name: "root", Role: RoleAdmin}

	// Un writer normal NO puede maintain ni doctor(repair).
	if err := callAs(writer, "musubi_maintain", map[string]any{"force": true}); err == nil {
		t.Error("un writer no-admin no debe poder correr musubi_maintain")
	} else if err.Code != codeUnauthorized {
		t.Errorf("esperaba unauthorized en maintain, obtuve code %d", err.Code)
	}
	if err := callAs(writer, "musubi_doctor", map[string]any{"repair": true, "check": "fts_consistency", "mode": "apply"}); err == nil {
		t.Error("un writer no-admin no debe poder correr doctor con repair")
	} else if err.Code != codeUnauthorized {
		t.Errorf("esperaba unauthorized en doctor-repair, obtuve code %d", err.Code)
	}

	// El diagnóstico de doctor (SIN repair) SÍ está permitido para un writer.
	if err := callAs(writer, "musubi_doctor", map[string]any{}); err != nil {
		t.Errorf("el diagnóstico de doctor no debe requerir admin: %+v", err)
	}

	// El admin puede ambas.
	if err := callAs(admin, "musubi_maintain", map[string]any{"force": true}); err != nil {
		t.Errorf("el admin debe poder correr maintain: %+v", err)
	}
	if err := callAs(admin, "musubi_doctor", map[string]any{"repair": true, "check": "fts_consistency", "mode": "dry-run"}); err != nil {
		t.Errorf("el admin debe poder correr doctor-repair: %+v", err)
	}
}
