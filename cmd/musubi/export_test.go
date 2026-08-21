package main

import (
	"testing"
	"time"

	"musubi/internal/memory"
)

// TestBuildExportSnapshot verifica que el snapshot reúna salud, insights, ledger y el
// mapa de conocimiento por dominio a partir de un motor real (DB temporal).
func TestBuildExportSnapshot(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	for _, s := range []struct{ id, topic string }{
		{"r1", "roadmap/track-7"},
		{"r2", "roadmap/track-8"},
		{"a1", "audit/full"},
	} {
		if err := engine.SaveObservation(s.id, s.topic, "contenido "+s.id, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Algo de gasto en el ledger para que el estado del presupuesto sea verificable.
	if _, err := engine.LedgerAdd("sess-x", "turn_recall", 500); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	snap, err := buildExportSnapshot(engine, "0.51.0", 8000, at)
	if err != nil {
		t.Fatalf("buildExportSnapshot error: %v", err)
	}

	if snap.Version != "0.51.0" {
		t.Errorf("version: esperaba 0.51.0, obtuve %q", snap.Version)
	}
	if snap.GeneratedAt != "2026-06-23T12:00:00Z" {
		t.Errorf("generated_at: esperaba timestamp UTC RFC3339, obtuve %q", snap.GeneratedAt)
	}
	if snap.Health.Status == "" {
		t.Error("health.status no debería estar vacío")
	}
	if snap.Insights.Observations.Active != 3 {
		t.Errorf("insights: esperaba 3 observaciones activas, obtuve %d", snap.Insights.Observations.Active)
	}
	// Ledger: 500 / 8000 = 6%, estado ok.
	if snap.Tokens.Total != 500 || snap.Tokens.Budget != 8000 || snap.Tokens.Status != "ok" {
		t.Errorf("tokens: esperaba 500/8000 ok, obtuve %d/%d %s", snap.Tokens.Total, snap.Tokens.Budget, snap.Tokens.Status)
	}
	// Grafo: total = activas; dominios roadmap(2) y audit(1).
	if snap.Graph.TotalObservations != 3 {
		t.Errorf("graph.total: esperaba 3, obtuve %d", snap.Graph.TotalObservations)
	}
	got := map[string]int{}
	for _, d := range snap.Graph.Domains {
		got[d.Domain] = d.Count
	}
	if got["roadmap"] != 2 || got["audit"] != 1 {
		t.Errorf("graph.domains: esperaba roadmap=2 audit=1, obtuve %+v", snap.Graph.Domains)
	}
	// Recent: las memorias legibles (las 3 guardadas, con tema + gist).
	if len(snap.Recent) != 3 {
		t.Errorf("recent: esperaba 3 memorias, obtuve %d", len(snap.Recent))
	}
	for _, m := range snap.Recent {
		if m.TopicKey == "" || m.Gist == "" {
			t.Errorf("cada memoria reciente debe traer tema y gist, obtuve %+v", m)
		}
	}
}

// TestBuildExportSnapshotOrchestration verifica que el snapshot incluya el pilar de
// orquestación: los runs de workflow (incluidos los flujos SDD) y la pizarra activa.
func TestBuildExportSnapshotOrchestration(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// Un flujo SDD arrancado (run_id sdd-add-auth, 7 fases).
	if _, err := engine.StartWorkflowRun(memory.SDDRunID("Add Auth"), memory.SDDWorkflowDef("Add Auth")); err != nil {
		t.Fatal(err)
	}
	// Una pizarra multi-agente con 2 unidades abiertas.
	if _, err := engine.CreateWorkBatch("b-orq", []memory.WorkUnitSpec{{Title: "A", Spec: "a"}, {Title: "B", Spec: "b"}}); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	snap, err := buildExportSnapshot(engine, "0.58.0", 8000, at)
	if err != nil {
		t.Fatalf("buildExportSnapshot error: %v", err)
	}

	var sdd *memory.WorkflowRunSummary
	for i := range snap.Orchestration.Runs {
		if snap.Orchestration.Runs[i].WorkflowID == "sdd-add-auth" {
			sdd = &snap.Orchestration.Runs[i]
		}
	}
	if sdd == nil {
		t.Fatalf("el snapshot debería incluir el run SDD sdd-add-auth, obtuve %+v", snap.Orchestration.Runs)
	}
	if sdd.Total != len(memory.SDDPhases) || sdd.Status != "running" {
		t.Errorf("run SDD: esperaba %d fases running, obtuve %d %q", len(memory.SDDPhases), sdd.Total, sdd.Status)
	}
	if snap.Orchestration.ActiveBatch == nil || snap.Orchestration.ActiveBatch.Total != 2 {
		t.Errorf("el snapshot debería incluir la pizarra activa con 2 unidades, obtuve %+v", snap.Orchestration.ActiveBatch)
	}
}

// TestSnapshotCoherenteEntreSusCifras es el test de coherencia END-TO-END del snapshot, y
// existe porque el dashboard llegó a afirmar DOS poblaciones distintas para la misma cosa en
// la misma pantalla: el encabezado decía "3724 memorias activas" mientras el grafo dibujaba
// sobre 3660 y el árbol de dominios sumaba una tercera cifra.
//
// INVARIANTE: las cinco formas de contar la memoria viva dan el MISMO número.
//
//	insights.utilization.active == insights.observations.visible
//	                            == brain.total_neurons
//	                            == graph.total_observations
//	                            == suma de graph.domains[].count
//
// La fixture tiene a propósito una observación archivada y una EN CUARENTENA: sin ellas
// Active y Visible coinciden por accidente y el test no prueba nada. La aserción de que
// Active > Visible está justamente para que el test no pueda volverse vacuo en silencio.
func TestSnapshotCoherenteEntreSusCifras(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	for _, s := range []struct{ id, topic string }{
		{"v1", "roadmap/uno"},
		{"v2", "roadmap/dos"},
		{"v3", "audit/tres"},
		{"muerta", "audit/cuatro"},
	} {
		if err := engine.SaveObservation(s.id, s.topic, "contenido "+s.id, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Archivada: sale de los dos universos (Active la descuenta, Visible también).
	if _, err := engine.ArchiveAsDuplicate("", "muerta", "v1"); err != nil {
		t.Fatal(err)
	}
	// En cuarentena: ACÁ está la trampa. archived=0, así que Active la cuenta como viva,
	// pero ningún camino de recall la devuelve y el grafo no la dibuja.
	if _, err := engine.ProposeObservation("", "un-modelo", "audit/propuesta",
		"texto sin corroborar", "modelo-x", 0.5, "semantic", nil); err != nil {
		t.Fatal(err)
	}

	snap, err := buildExportSnapshot(engine, "test", 8000, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	visible := snap.Insights.Observations.Visible
	if visible != 3 {
		t.Fatalf("visibles esperadas 3 (v1,v2,v3), obtuve %d", visible)
	}
	// Si esto no se cumple, la fixture dejó de ejercer la divergencia y el resto del test
	// pasaría por coincidencia.
	if snap.Insights.Observations.Active <= visible {
		t.Fatalf("la fixture debe tener memoria no-visible pero no archivada (cuarentena): active=%d visible=%d",
			snap.Insights.Observations.Active, visible)
	}

	if snap.Insights.Utilization.Active != visible {
		t.Errorf("utilization.active=%d debería igualar observations.visible=%d",
			snap.Insights.Utilization.Active, visible)
	}
	if snap.Brain.TotalNeurons != visible {
		t.Errorf("brain.total_neurons=%d debería igualar visible=%d: el grafo y el encabezado tienen que hablar de la misma memoria",
			snap.Brain.TotalNeurons, visible)
	}
	if snap.Graph.TotalObservations != visible {
		t.Errorf("graph.total_observations=%d debería igualar visible=%d", snap.Graph.TotalObservations, visible)
	}
	suma := 0
	for _, d := range snap.Graph.Domains {
		suma += d.Count
	}
	if suma != visible {
		t.Errorf("las hojas del árbol suman %d y el encabezado dice %d: el mapa de conocimiento se contradice a sí mismo",
			suma, visible)
	}
}
