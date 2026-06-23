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
