package memory

import (
	"context"
	"testing"
)

// TestReadIsolationByProjectScope valida el AISLAMIENTO multi-tenant (Track 17) en las 3
// superficies de lectura respaldadas por `observations`: una lectura con ProjectScope acotado
// solo devuelve las filas de ESE proyecto + las sin atribuir; federada (sin scope) ve todo.
// Es el guard de regresión del cross-project bleed a nivel del motor.
func TestReadIsolationByProjectScope(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	vec := []float32{1, 0, 0}
	save := func(origin, id string) {
		if err := e.SaveObservationTypedFrom(origin, "", id, "t/x", "shared qtoken content", 1, "", ScopeLocal, vec); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	save("crm", "crm-1")
	save("web", "web-1")
	save("", "free-1") // sin atribuir: visible para todos

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	fed := context.Background() // sin scope = federado (histórico)

	obsIDs := func(os []Observation) map[string]bool {
		m := map[string]bool{}
		for _, o := range os {
			m[o.ID] = true
		}
		return m
	}
	resIDs := func(rs []SearchResult) map[string]bool {
		m := map[string]bool{}
		for _, r := range rs {
			m[r.ID] = true
		}
		return m
	}
	wantScoped := func(t *testing.T, name string, g map[string]bool) {
		t.Helper()
		if !g["crm-1"] || !g["free-1"] || g["web-1"] {
			t.Errorf("%s acotado a crm: esperaba {crm-1,free-1} SIN web-1, obtuve %v", name, g)
		}
	}
	wantAll := func(t *testing.T, name string, g map[string]bool) {
		t.Helper()
		if !g["crm-1"] || !g["web-1"] || !g["free-1"] {
			t.Errorf("%s federado: esperaba los 3, obtuve %v", name, g)
		}
	}

	// FTS (search_keyword)
	fts, err := e.SearchObservationsFTS(crm, "shared", 10)
	if err != nil {
		t.Fatal(err)
	}
	wantScoped(t, "FTS", obsIDs(fts))
	ftsF, _ := e.SearchObservationsFTS(fed, "shared", 10)
	wantAll(t, "FTS", obsIDs(ftsF))

	// Semántica (search_semantic; full-scan porque el IVF no se entrena con 3 filas)
	sem, err := e.SearchObservations(crm, vec, 10)
	if err != nil {
		t.Fatal(err)
	}
	wantScoped(t, "semantic", resIDs(sem))
	semF, _ := e.SearchObservations(fed, vec, 10)
	wantAll(t, "semantic", resIDs(semF))

	// Hidratación por id (memory_expand): la fuga más grave — traía contenido por id arbitrario.
	exp, _, err := e.GetObservationsBudgetCtx(crm, []string{"crm-1", "web-1", "free-1"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantScoped(t, "expand", obsIDs(exp))
	expF, _, _ := e.GetObservationsBudgetCtx(fed, []string{"crm-1", "web-1", "free-1"}, 0)
	wantAll(t, "expand", obsIDs(expF))
}
