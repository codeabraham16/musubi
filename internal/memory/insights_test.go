package memory

import "testing"

// TestInsights verifica el agregador de observabilidad activa (T6.4): cuenta observaciones
// (activas/archivadas), errores no resueltos + hotspots, y decisiones por última (last-wins).
func TestInsights(t *testing.T) {
	e := newTestEngine(t)

	// Observaciones: 3, una archivada.
	for _, id := range []string{"o1", "o2", "o3"} {
		if err := e.SaveObservation(id, "t", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1 WHERE id='o3'`); err != nil {
		t.Fatal(err)
	}

	// Telemetría: dos errores en auth.go (hotspot), uno en db.go, uno resuelto.
	for _, m := range []struct{ f, e string }{
		{"auth.go", "err A"}, {"auth.go", "err B"}, {"db.go", "err C"}, {"old.go", "err viejo"},
	} {
		if err := e.SaveTelemetryLog(m.f, m.e, ""); err != nil {
			t.Fatal(err)
		}
	}
	// Resolver el de old.go.
	logs, _ := e.GetUnresolvedTelemetryLogs()
	for _, l := range logs {
		if l.FilePath == "old.go" {
			_ = e.ResolveTelemetryLog(l.ID)
		}
	}

	// Decisiones: go-gin rechazada, go-testing aceptada, rust rechazada→aceptada (last-wins).
	_ = e.SaveSkillDecision("go-gin", "Gin", "rejected", "")
	_ = e.SaveSkillDecision("go-testing", "Testing", "accepted", "")
	_ = e.SaveSkillDecision("rust-axum", "Axum", "rejected", "")
	_ = e.SaveSkillDecision("rust-axum", "Axum", "accepted", "")

	rep, err := e.Insights()
	if err != nil {
		t.Fatalf("Insights error: %v", err)
	}

	if rep.Observations.Total != 3 || rep.Observations.Active != 2 || rep.Observations.Archived != 1 {
		t.Errorf("observaciones: %+v (esperaba total 3, active 2, archived 1)", rep.Observations)
	}
	if rep.UnresolvedErrors != 3 {
		t.Errorf("errores no resueltos: %d (esperaba 3)", rep.UnresolvedErrors)
	}
	if len(rep.ErrorHotspots) == 0 || rep.ErrorHotspots[0].FilePath != "auth.go" || rep.ErrorHotspots[0].Count != 2 {
		t.Errorf("el hotspot top debe ser auth.go con 2, obtuve %+v", rep.ErrorHotspots)
	}
	if rep.SkillDecisions.Accepted != 2 || rep.SkillDecisions.Rejected != 1 {
		t.Errorf("decisiones: %+v (esperaba accepted 2 [go-testing+rust], rejected 1 [go-gin])", rep.SkillDecisions)
	}
}
