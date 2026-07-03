package memory

import "testing"

// sagaRun arma un run a→b→c con compensate en los steps indicados y lo completa hasta el
// último. compensa es el mapa step→directiva (vacío = sin compensación).
func sagaRun(t *testing.T, runID string, compensa map[string]string) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	def := WorkflowDef{ID: "saga", Steps: []WorkflowStep{
		{ID: "a", Compensate: compensa["a"]},
		{ID: "b", Needs: []string{"a"}, Compensate: compensa["b"]},
		{ID: "c", Needs: []string{"b"}, Compensate: compensa["c"]},
	}}
	if _, err := engine.StartWorkflowRun(runID, def); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"a", "b", "c"} {
		engine.WorkflowReady(runID)
		if _, err := engine.CompleteWorkflowStep(runID, s, "ok", StepDone, ""); err != nil {
			t.Fatal(err)
		}
	}
	return engine
}

func planIDs(plan []CompensationStep) []string {
	ids := make([]string, len(plan))
	for i, c := range plan {
		ids[i] = c.StepID
	}
	return ids
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Plan LIFO: completado a→b→c, el plan de compensación es c,b,a.
func TestSagaRollbackLIFO(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{"a": "undo a", "b": "undo b", "c": "undo c"})
	defer engine.Close()

	plan, run, err := engine.WorkflowRollback("R")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !eq(planIDs(plan), []string{"c", "b", "a"}) {
		t.Errorf("plan LIFO esperado [c b a], obtuve %v", planIDs(plan))
	}
	if run.Status != RunCompensating {
		t.Errorf("el run debe quedar compensating, está %q", run.Status)
	}
}

// Filtro: sólo steps con compensate y en estado done entran al plan.
func TestSagaPlanFiltersNonCompensable(t *testing.T) {
	// Sólo 'a' y 'c' tienen compensación; 'b' no.
	engine := sagaRun(t, "R", map[string]string{"a": "undo a", "c": "undo c"})
	defer engine.Close()

	plan, _, err := engine.WorkflowRollback("R")
	if err != nil {
		t.Fatal(err)
	}
	if !eq(planIDs(plan), []string{"c", "a"}) {
		t.Errorf("plan debe excluir 'b' (sin compensación): esperaba [c a], obtuve %v", planIDs(plan))
	}
}

// Ejecución completa: compensar c, b, a vacía el plan y cierra el run como compensated.
func TestSagaCompensateToCompletion(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{"a": "undo a", "b": "undo b", "c": "undo c"})
	defer engine.Close()
	engine.WorkflowRollback("R")

	for _, step := range []string{"c", "b"} {
		plan, run, err := engine.CompleteCompensation("R", step)
		if err != nil {
			t.Fatalf("compensated(%s): %v", step, err)
		}
		if run.Status == RunCompensated {
			t.Fatalf("el run no debe estar compensated aún tras %s (plan=%v)", step, planIDs(plan))
		}
	}
	plan, run, err := engine.CompleteCompensation("R", "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 0 {
		t.Errorf("tras compensar todo el plan debe estar vacío, es %v", planIDs(plan))
	}
	if run.Status != RunCompensated {
		t.Errorf("el run debe quedar compensated, está %q", run.Status)
	}
	// El journal debe contener run_compensated.
	ev, _ := engine.WorkflowJournal("R")
	if !hasEvent(ev, EventRunCompensated) {
		t.Error("el journal debe tener run_compensated al cerrar la saga")
	}
}

// Doble compensación del mismo step es no-op (no error, no evento duplicado).
func TestSagaDoubleCompensateNoop(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{"c": "undo c"})
	defer engine.Close()
	engine.WorkflowRollback("R")

	if _, _, err := engine.CompleteCompensation("R", "c"); err != nil {
		t.Fatal(err)
	}
	// Segunda vez: no-op.
	if _, _, err := engine.CompleteCompensation("R", "c"); err != nil {
		t.Errorf("compensar dos veces debe ser no-op, no error: %v", err)
	}
	ev, _ := engine.WorkflowJournal("R")
	n := 0
	for _, e := range ev {
		if e.EventType == EventStepCompensated && e.StepID == "c" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("debe haber un solo evento step_compensated para 'c', hay %d", n)
	}
}

// Rollback sin nada que compensar cierra el run directamente como compensated.
func TestSagaRollbackNothingToCompensate(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{}) // ningún step tiene compensate
	defer engine.Close()

	plan, run, err := engine.WorkflowRollback("R")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 0 {
		t.Errorf("plan debe ser vacío, es %v", planIDs(plan))
	}
	if run.Status != RunCompensated {
		t.Errorf("sin nada que compensar el run debe quedar compensated, está %q", run.Status)
	}
}

// Re-entrancia: re-llamar rollback recomputa el plan sin duplicar run_rollback.
func TestSagaRollbackReentrant(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{"a": "undo a", "b": "undo b", "c": "undo c"})
	defer engine.Close()
	engine.WorkflowRollback("R")
	engine.CompleteCompensation("R", "c")

	plan, _, err := engine.WorkflowRollback("R") // re-rollback
	if err != nil {
		t.Fatal(err)
	}
	if !eq(planIDs(plan), []string{"b", "a"}) {
		t.Errorf("re-rollback debe recomputar [b a], obtuve %v", planIDs(plan))
	}
	ev, _ := engine.WorkflowJournal("R")
	n := 0
	for _, e := range ev {
		if e.EventType == EventRunRollback {
			n++
		}
	}
	if n != 1 {
		t.Errorf("run_rollback no debe duplicarse (re-entrante), hay %d", n)
	}
}

// Compensar un step sin compensación declarada es un error.
func TestSagaCompensateNonCompensableErrors(t *testing.T) {
	engine := sagaRun(t, "R", map[string]string{"c": "undo c"}) // 'a' no tiene compensación
	defer engine.Close()
	engine.WorkflowRollback("R")
	if _, _, err := engine.CompleteCompensation("R", "a"); err == nil {
		t.Error("compensar un step sin compensación declarada debe fallar")
	}
}
