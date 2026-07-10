package memory

import (
	"context"
	"testing"
)

// TestInsightsCtxScopesObservationCounts valida el aislamiento PARCIAL de insights (Track 17): los
// counts de observations se acotan al proyecto del contexto (propias + sin atribuir); federado
// cuenta todas. Es el guard de que un principal no ve el VOLUMEN de memoria de otros proyectos.
func TestInsightsCtxScopesObservationCounts(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	save := func(origin, id string) {
		if err := e.SaveObservationTypedFrom(origin, id, "t/x", "contenido de "+id, 1, "", ScopeLocal, nil); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	save("crm", "crm-1")
	save("crm", "crm-2")
	save("web", "web-1")
	save("", "free-1") // sin atribuir: visible para todos

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	crmRep, err := e.InsightsCtx(crm)
	if err != nil {
		t.Fatal(err)
	}
	if crmRep.Observations.Total != 3 { // crm-1, crm-2, free-1
		t.Errorf("crm esperaba Total=3 (2 propias + 1 sin atribuir), obtuvo %d", crmRep.Observations.Total)
	}

	webRep, _ := e.InsightsCtx(web)
	if webRep.Observations.Total != 2 { // web-1, free-1
		t.Errorf("web esperaba Total=2 (1 propia + 1 sin atribuir), obtuvo %d", webRep.Observations.Total)
	}

	// Federado (sin scope): cuenta las 4.
	fedRep, _ := e.Insights()
	if fedRep.Observations.Total != 4 {
		t.Errorf("federado esperaba Total=4, obtuvo %d", fedRep.Observations.Total)
	}
	if fedRep.Observations.Active != 4 {
		t.Errorf("federado esperaba Active=4, obtuvo %d", fedRep.Observations.Active)
	}
}
