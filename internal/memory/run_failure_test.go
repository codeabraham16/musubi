package memory

import "testing"

// journalHasEvent indica si el journal del run contiene un evento de ese tipo.
func journalHasEvent(t *testing.T, e *DbEngine, runID, eventType string) bool {
	t.Helper()
	events, err := e.WorkflowJournal(runID)
	if err != nil {
		t.Fatalf("WorkflowJournal: %v", err)
	}
	for _, ev := range events {
		if ev.EventType == eventType {
			return true
		}
	}
	return false
}

// Escenario (a): A→B, A falla ⇒ B nunca ready, no hay progreso ⇒ run failed + journal run_failed.
func TestRunFailsWhenStepFailedWedges(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "f", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "b", Needs: []string{"a"}},
	}}
	if _, err := e.StartWorkflowRun("R", def); err != nil {
		t.Fatal(err)
	}
	run, err := e.CompleteWorkflowStep("R", "a", "boom", StepFailed, "")
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if run.Status != RunFailed {
		t.Errorf("un step failed que wedgea debe dejar el run 'failed', está %q", run.Status)
	}
	if !journalHasEvent(t, e, "R", EventRunFailed) {
		t.Error("el journal debe registrar run_failed")
	}
	// Un run terminal no despacha más steps.
	ready, _ := e.WorkflowReady("R")
	if len(ready) != 0 {
		t.Errorf("un run failed no debe despachar steps, obtuve %v", ready)
	}
}

// Escenario (b): rama independiente en curso ⇒ el run NO se marca failed prematuramente (R3).
func TestRunStaysRunningWhileIndependentBranchProgresses(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "g", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "b", Needs: []string{"a"}},
		{ID: "c"}, // rama independiente
	}}
	e.StartWorkflowRun("R", def)
	run, _ := e.CompleteWorkflowStep("R", "a", "boom", StepFailed, "")
	if run.Status != RunRunning {
		t.Fatalf("con la rama 'c' aún pendiente el run debe seguir running, está %q", run.Status)
	}
	// Al terminar la rama sana ya no hay progreso posible y hay un failed ⇒ failed.
	run, _ = e.CompleteWorkflowStep("R", "c", "ok", StepDone, "")
	if run.Status != RunFailed {
		t.Errorf("tras agotar el progreso el run debe quedar failed, está %q", run.Status)
	}
}

// Escenario happy-path (equivalencia R8): un run sin fallos queda done con run_done (sin run_failed).
func TestRunDoneHappyPathUnchanged(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "h", Steps: []WorkflowStep{{ID: "a"}, {ID: "b", Needs: []string{"a"}}}}
	e.StartWorkflowRun("R", def)
	e.CompleteWorkflowStep("R", "a", "ok", StepDone, "")
	run, _ := e.CompleteWorkflowStep("R", "b", "ok", StepDone, "")
	if run.Status != RunDone {
		t.Errorf("run sin fallos debe quedar done, está %q", run.Status)
	}
	if !journalHasEvent(t, e, "R", EventRunDone) || journalHasEvent(t, e, "R", EventRunFailed) {
		t.Error("el happy-path debe emitir run_done y NO run_failed")
	}
}

// Escenario (c): el verify-gate agota su presupuesto ⇒ step failed ⇒ run failed.
func TestRunFailsWhenVerifyGateExhausts(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "v", Steps: []WorkflowStep{
		{ID: "a", Verify: "chequear algo", MaxIterations: 1},
	}}
	e.StartWorkflowRun("R", def)
	e.WorkflowReady("R")
	// done → pasa a verifying (por el gate).
	if _, err := e.CompleteWorkflowStep("R", "a", "res", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	// fail con presupuesto 1 ⇒ se agota ⇒ step failed ⇒ run failed.
	run, _, err := e.VerifyWorkflowStep("R", "a", false, "no pasa la verificación")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if run.Status != RunFailed {
		t.Errorf("agotar el verify-gate debe dejar el run failed, está %q", run.Status)
	}
}

// Escenario (d): abort de un run running ⇒ aborted + no despacha + journal run_aborted.
func TestAbortRunningRun(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "a", Steps: []WorkflowStep{{ID: "x"}, {ID: "y", Needs: []string{"x"}}}}
	e.StartWorkflowRun("R", def)
	run, err := e.AbortWorkflowRun("R", "ya no hace falta")
	if err != nil {
		t.Fatalf("abort: %v", err)
	}
	if run.Status != RunAborted {
		t.Errorf("debe quedar aborted, está %q", run.Status)
	}
	if ready, _ := e.WorkflowReady("R"); len(ready) != 0 {
		t.Errorf("un run aborted no debe despachar steps, obtuve %v", ready)
	}
	if !journalHasEvent(t, e, "R", EventRunAborted) {
		t.Error("el journal debe registrar run_aborted")
	}
}

// Escenario (e): abort idempotente; abortar un run ya concluido (done) falla.
func TestAbortIdempotentAndGuards(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "a", Steps: []WorkflowStep{{ID: "x"}}}
	e.StartWorkflowRun("R", def)
	e.AbortWorkflowRun("R", "stop")
	run, err := e.AbortWorkflowRun("R", "otra vez")
	if err != nil || run.Status != RunAborted {
		t.Errorf("abort debe ser idempotente, obtuve %q err=%v", run.Status, err)
	}
	// Abortar un run done ⇒ error.
	e.StartWorkflowRun("R2", def)
	e.CompleteWorkflowStep("R2", "x", "ok", StepDone, "")
	if _, err := e.AbortWorkflowRun("R2", "x"); err == nil {
		t.Error("abortar un run ya concluido (done) debe fallar")
	}
}

// Escenario (f): rollback desde un run failed ⇒ plan de compensación de los steps completados.
func TestRollbackFromFailedRun(t *testing.T) {
	e := newTestEngine(t)
	def := WorkflowDef{ID: "s", Steps: []WorkflowStep{
		{ID: "a", Compensate: "deshacer a"},
		{ID: "b", Needs: []string{"a"}},
	}}
	e.StartWorkflowRun("R", def)
	e.CompleteWorkflowStep("R", "a", "ok", StepDone, "")               // a done (con compensación)
	run, _ := e.CompleteWorkflowStep("R", "b", "boom", StepFailed, "") // b falla ⇒ run failed
	if run.Status != RunFailed {
		t.Fatalf("el run debe estar failed, está %q", run.Status)
	}
	plan, _, err := e.WorkflowRollback("R")
	if err != nil {
		t.Fatalf("rollback desde failed: %v", err)
	}
	if len(plan) != 1 || plan[0].StepID != "a" {
		t.Errorf("el plan de compensación debe incluir 'a', obtuve %+v", plan)
	}
}
