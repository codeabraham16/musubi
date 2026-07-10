package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// read_surface_class_test.go SELLA POR CONTRATO la clase "superficie de lectura aislada por
// proyecto" (Track 19). La leccion de tres auditorias: scopear tool-por-tool siempre deja una
// hermana federada (detect_changes, telemetria, resolve_skills aparecieron una por auditoria). En
// vez de perseguirlas de a una, este archivo:
//   1. BARRE todas las superficies de lectura sobre tablas con project_id con datos cross-tenant
//      sembrados y falla si el marcador del otro tenant aparece (TestReadSurfaceClassIsolation).
//   2. Exige que TODA tool readOnly registrada este CLASIFICADA (cubierta por el barrido, o en la
//      allowlist de "no lee tablas scopeadas") — asi una tool de lectura nueva NO puede colarse sin
//      que un humano decida si necesita scope (TestEveryReadOnlyToolClassified).

// seedVictim siembra datos del proyecto "web" con marcadores distintivos en cada tabla scopeada.
func seedVictim(t *testing.T, e *memory.DbEngine) {
	t.Helper()
	if err := e.SaveObservationTypedFrom("web", "web-obs-1", "web/topic", "VICTIMOBS sobre shared/auth.go", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveTelemetryLogFrom("web", "shared/auth.go", "VICTIMTELEM boom", "fix"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveSkillDecisionFrom("web", "web-skill", "WebSkill", "rejected", "r"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("web", memory.CodeMemory{Path: "shared/auth.go", Gist: "VICTIMGIST", Fingerprint: "h", Tokens: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFrom("web", "SharedEntity", "relates_to", "VICTIMFACT", "", nil); err != nil {
		t.Fatal(err)
	}
}

// readSweepCase es una superficie de lectura y el marcador del tenant "web" que NO debe filtrar.
type readSweepCase struct {
	tool   string
	args   map[string]any
	marker string // string distintivo del tenant web que un atacante NO debe ver
}

// readSweepCases enumera las superficies de lectura marker-in-response cubiertas por el barrido.
// AGREGAR aca toda tool de lectura nueva que consulte una tabla con project_id (ver el guard de
// completitud abajo, que falla si una readOnly nueva no esta clasificada).
func readSweepCases() []readSweepCase {
	return []readSweepCase{
		{"musubi_recall", map[string]any{"query": "shared auth"}, "VICTIMOBS"},
		{"musubi_search_keyword", map[string]any{"query_text": "VICTIMOBS"}, "VICTIMOBS"},
		{"musubi_memory_expand", map[string]any{"ids": []string{"web-obs-1"}}, "VICTIMOBS"},
		{"musubi_recall_facts", map[string]any{"entity": "SharedEntity"}, "VICTIMFACT"},
		{"musubi_entity_context", map[string]any{"entity": "SharedEntity"}, "VICTIMFACT"},
		{"musubi_recall_code", map[string]any{"path": "shared/auth.go"}, "VICTIMGIST"},
		{"musubi_insights", map[string]any{}, "shared/auth.go"}, // hotspot file_path de la telemetria de web
		{"musubi_resolve_skills", map[string]any{"modified_files": []string{"auth.go"}}, "VICTIMTELEM"},
	}
}

func TestReadSurfaceClassIsolation(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	seedVictim(t, engine)

	respText := func(tool string, args map[string]any, p *Principal) string {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("%s: %+v", tool, rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}

	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"} // atacante: otro proyecto
	admin := &Principal{Name: "root", Role: RoleAdmin}                   // federado: control

	for _, tc := range readSweepCases() {
		// El atacante (crm) NUNCA debe ver el marcador de web.
		if got := respText(tc.tool, tc.args, crm); strings.Contains(got, tc.marker) {
			t.Errorf("FUGA cross-tenant en %s: un writer/crm vio el marcador %q de web\nrespuesta: %s", tc.tool, tc.marker, got)
		}
		// El admin federado SÍ debe verlo (prueba que el dato existe y el filtro no rompe legacy).
		if got := respText(tc.tool, tc.args, admin); !strings.Contains(got, tc.marker) {
			t.Errorf("%s: el admin federado deberia ver el marcador %q de web (seed/legacy roto)\nrespuesta: %s", tc.tool, tc.marker, got)
		}
	}
}

// TestEveryReadOnlyToolClassified es el GUARD DE COMPLETITUD: toda tool readOnly registrada debe
// estar cubierta por el barrido de aislamiento, o declarada explicitamente como "no lee tablas
// scopeadas". Asi una tool de lectura nueva NO puede agregarse sin que alguien decida si necesita
// scope de proyecto — cierra el whack-a-mole por contrato, no por vigilancia.
func TestEveryReadOnlyToolClassified(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Cubiertas por el barrido de aislamiento (leen tablas con project_id).
	swept := map[string]bool{}
	for _, tc := range readSweepCases() {
		swept[tc.tool] = true
	}
	// Cubiertas por barrido/aislamiento en OTROS tests dedicados (por args especiales).
	for _, name := range []string{
		"musubi_search_semantic", // scope a nivel motor (scope_isolation_test); necesita embedder
		"musubi_conflicts",       // conflicts_isolation_test (JOIN a observations)
		"musubi_detect_changes",  // methods_detect_test (necesita git runner)
		"musubi_search_skills",   // behavior-bleed via GetSkillDecisionsCtx (Track 19); no marker-in-text
	} {
		swept[name] = true
	}
	// readOnly que NO leen ninguna tabla con project_id (catalogo remoto, estado, salud, etc.):
	// no necesitan scope de proyecto. Si una de estas empieza a leer datos scopeados, MOVERLA arriba.
	noScopedRead := map[string]bool{
		"musubi_discover_skills": true, // catalogo remoto/marketplace
		"musubi_detect_stack":    true, // inspecciona el filesystem del proyecto local
		"musubi_tokens":          true, // ledger de la sesion
		"musubi_sync_status":     true, // estado del outbox (no por-proyecto)
		"musubi_phase":           true, // pipeline de fases de la sesion
	}

	for i := range s.tools {
		tl := &s.tools[i]
		if !tl.readOnly {
			continue
		}
		if swept[tl.Name] || noScopedRead[tl.Name] {
			continue
		}
		t.Errorf("tool readOnly %q SIN clasificar: agregala al barrido de aislamiento (readSweepCases) si lee una tabla con project_id, o a noScopedRead si no", tl.Name)
	}
}
