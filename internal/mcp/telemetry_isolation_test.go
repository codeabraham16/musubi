package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// insightsAs invoca musubi_insights como el principal dado y devuelve el reporte decodeado.
func insightsAs(t *testing.T, s *McpServer, p *Principal) memory.InsightsReport {
	t.Helper()
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_insights", Arguments: json.RawMessage(`{}`)})
	out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
	if rpcErr != nil {
		t.Fatalf("insights: %+v", rpcErr)
	}
	var rep memory.InsightsReport
	if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &rep); err != nil {
		t.Fatalf("decodear insights: %v", err)
	}
	return rep
}

// TestTelemetryAndDecisionsEnforceProjectScope valida el aislamiento de Track 18 en el subsistema
// de telemetría/decisiones: resolve_telemetry no lee ni resuelve el log de otro proyecto, e
// insights acota hotspots, conteo de errores y decisiones a la credencial.
func TestTelemetryAndDecisionsEnforceProjectScope(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Telemetría + decisiones atribuidas a "web" y "crm".
	if err := engine.SaveTelemetryLogFrom("web", "web/secret.go", "web boom", "web fix"); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveTelemetryLogFrom("crm", "crm/app.go", "crm boom", "crm fix"); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveSkillDecisionFrom("web", "web-skill", "Web", "accepted", "r"); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveSkillDecisionFrom("crm", "crm-skill", "Crm", "accepted", "r"); err != nil {
		t.Fatal(err)
	}

	// Id del log de web (para intentar resolverlo como crm).
	webLogs, _ := engine.GetUnresolvedTelemetryLogs()
	webID := 0
	for _, l := range webLogs {
		if l.FilePath == "web/secret.go" {
			webID = l.ID
		}
	}
	if webID == 0 {
		t.Fatal("no se sembró el log de web")
	}

	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}

	// resolve_telemetry: un writer/crm NO puede resolver ni leer el log de web (found=false ⇒ error).
	raw, _ := json.Marshal(map[string]int{"id": webID})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_resolve_telemetry", Arguments: raw})
	if _, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), crm), params); rpcErr == nil {
		t.Errorf("crm no debería poder resolver el log de web (cross-tenant)")
	}
	// El log de web sigue SIN resolver (el intento cross-tenant no lo tocó).
	stillUnresolved := false
	after, _ := engine.GetUnresolvedTelemetryLogs()
	for _, l := range after {
		if l.ID == webID {
			stillUnresolved = true
		}
	}
	if !stillUnresolved {
		t.Errorf("el log de web no debería haber sido resuelto por crm")
	}

	// insights como crm: ve su hotspot/decisión/error, NUNCA los de web.
	rep := insightsAs(t, s, crm)
	if rep.UnresolvedErrors != 1 {
		t.Errorf("insights crm: esperaba 1 error no resuelto (el de crm), obtuve %d", rep.UnresolvedErrors)
	}
	if rep.SkillDecisions.Accepted != 1 {
		t.Errorf("insights crm: esperaba 1 decisión accepted (la de crm), obtuve %d", rep.SkillDecisions.Accepted)
	}
	sawCrm, sawWeb := false, false
	for _, h := range rep.ErrorHotspots {
		if h.FilePath == "crm/app.go" {
			sawCrm = true
		}
		if h.FilePath == "web/secret.go" {
			sawWeb = true
		}
	}
	if !sawCrm || sawWeb {
		t.Errorf("insights crm: esperaba hotspot crm/app.go SIN web/secret.go, obtuve %+v", rep.ErrorHotspots)
	}

	// Admin federado: ve los 2 errores y las 2 decisiones (el filtro no rompe el modo legacy).
	adm := insightsAs(t, s, &Principal{Name: "root", Role: RoleAdmin})
	if adm.UnresolvedErrors != 2 || adm.SkillDecisions.Accepted != 2 {
		t.Errorf("insights admin: esperaba 2 errores y 2 decisiones, obtuve %d y %d", adm.UnresolvedErrors, adm.SkillDecisions.Accepted)
	}
}

// TestLogErrorRedactsAndAttributes valida que log_error (Track 18) redacta el ingest en infra
// compartida (forceRedact) y lo atribuye al proyecto de la credencial (no al espacio federado).
func TestLogErrorRedactsAndAttributes(t *testing.T) {
	const secret = "AKIA1234567890ABCDEF" // matchea aws-access-key, no está en la allowlist
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	s.forceRedact = true

	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	raw, _ := json.Marshal(map[string]any{
		"file_path":       "a.go",
		"error_message":   "leaked key " + secret,
		"suggested_patch": "rotate " + secret,
	})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_log_error", Arguments: raw})
	if _, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), crm), params); rpcErr != nil {
		t.Fatalf("log_error: %+v", rpcErr)
	}

	logs, _ := engine.GetUnresolvedTelemetryLogs()
	if len(logs) != 1 {
		t.Fatalf("esperaba 1 log, obtuve %d", len(logs))
	}
	if strings.Contains(logs[0].ErrorMessage, secret) || strings.Contains(logs[0].SuggestedPatch, secret) {
		t.Errorf("con forceRedact el secreto debía redactarse; msg=%q patch=%q", logs[0].ErrorMessage, logs[0].SuggestedPatch)
	}

	// Atribuido a crm: web no lo ve en insights, crm sí (verifica la atribución por credencial).
	if n := insightsAs(t, s, &Principal{Name: "bob", Role: RoleWriter, ProjectID: "web"}).UnresolvedErrors; n != 0 {
		t.Errorf("web no debería ver el error de crm, contó %d", n)
	}
	if n := insightsAs(t, s, crm).UnresolvedErrors; n != 1 {
		t.Errorf("crm debería ver su propio error, contó %d", n)
	}
}
