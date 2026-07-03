package memory

import "testing"

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// hitlRun arma a → gate(await) → b y completa a. gateWhen es el `when` opcional del gate.
func hitlRun(t *testing.T, runID, gateWhen string) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	def := WorkflowDef{ID: "hitl", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "gate", Needs: []string{"a"}, Await: "¿Aprobar?", When: gateWhen},
		{ID: "b", Needs: []string{"gate"}},
	}}
	if _, err := engine.StartWorkflowRun(runID, def); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady(runID)
	if _, err := engine.CompleteWorkflowStep(runID, "a", "ok", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	return engine
}

// Un step con await no se ofrece como ready: el run se pausa y bloquea dependientes.
func TestHITLPausesOnAwait(t *testing.T) {
	engine := hitlRun(t, "R", "")
	defer engine.Close()

	ready, err := engine.WorkflowReady("R")
	if err != nil {
		t.Fatal(err)
	}
	if contains(ready, "gate") {
		t.Error("un gate con await NO debe aparecer en ready")
	}
	if contains(ready, "b") {
		t.Error("'b' está bloqueado por el gate en espera, no debe estar ready")
	}
	awaiting, _ := engine.WorkflowAwaiting("R")
	if len(awaiting) != 1 || awaiting[0].StepID != "gate" || awaiting[0].Prompt != "¿Aprobar?" {
		t.Errorf("debe haber 1 gate en espera con su prompt, obtuve %+v", awaiting)
	}
	run, _, _ := engine.WorkflowRunStatus("R")
	if run.StepStatus["gate"] != StepWaiting {
		t.Errorf("el gate debe estar waiting_input, está %q", run.StepStatus["gate"])
	}
}

// provide=done reanuda: el gate queda done y los dependientes se destraban.
func TestHITLProvideDoneResumes(t *testing.T) {
	engine := hitlRun(t, "R", "")
	defer engine.Close()
	engine.WorkflowReady("R") // pausa el gate

	run, err := engine.ProvideWorkflowInput("R", "gate", "aprobado por Ana", StepDone)
	if err != nil {
		t.Fatalf("provide: %v", err)
	}
	if run.StepStatus["gate"] != StepDone || run.StepResults["gate"] != "aprobado por Ana" {
		t.Errorf("el gate debe quedar done con el input como result: %+v", run.StepStatus)
	}
	ready, _ := engine.WorkflowReady("R")
	if !contains(ready, "b") {
		t.Errorf("tras aprobar el gate, 'b' debe estar ready, obtuve %v", ready)
	}
}

// provide=failed bloquea: los dependientes no se destraban.
func TestHITLProvideFailedBlocks(t *testing.T) {
	engine := hitlRun(t, "R", "")
	defer engine.Close()
	engine.WorkflowReady("R")

	if _, err := engine.ProvideWorkflowInput("R", "gate", "rechazado", StepFailed); err != nil {
		t.Fatal(err)
	}
	ready, _ := engine.WorkflowReady("R")
	if contains(ready, "b") {
		t.Error("con el gate rechazado (failed), 'b' NO debe destrabarse")
	}
}

// Un gate con `when` falso se salta, no pausa.
func TestHITLGateWhenFalseSkips(t *testing.T) {
	engine := hitlRun(t, "R", "step.a.result == nunca")
	defer engine.Close()

	ready, _ := engine.WorkflowReady("R")
	run, _, _ := engine.WorkflowRunStatus("R")
	if run.StepStatus["gate"] != StepSkipped {
		t.Errorf("un gate con when falso debe saltarse, está %q", run.StepStatus["gate"])
	}
	// Al saltarse el gate, 'b' se destraba (skipped satisface la dependencia).
	if !contains(ready, "b") {
		t.Errorf("con el gate skipped, 'b' debe estar ready, obtuve %v", ready)
	}
}

// Durabilidad: una relectura fresca del run puede proveer y continuar.
func TestHITLDurableResume(t *testing.T) {
	dir := t.TempDir()
	engine, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	def := WorkflowDef{ID: "hitl", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "gate", Needs: []string{"a"}, Await: "¿Aprobar?"},
	}}
	engine.StartWorkflowRun("R", def)
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "a", "ok", StepDone, "")
	engine.WorkflowReady("R") // pausa el gate
	engine.Close()

	// Otra "sesión": reabrir la misma base.
	engine2, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer engine2.Close()
	awaiting, _ := engine2.WorkflowAwaiting("R")
	if len(awaiting) != 1 || awaiting[0].StepID != "gate" {
		t.Fatalf("la relectura fresca debe ver el gate en espera, obtuve %+v", awaiting)
	}
	run, err := engine2.ProvideWorkflowInput("R", "gate", "ok tarde", StepDone)
	if err != nil {
		t.Fatalf("provide en la 2a sesión: %v", err)
	}
	if run.Status != RunDone {
		t.Errorf("tras proveer el último gate el run debe quedar done, está %q", run.Status)
	}
}

// provide sobre un step que no está esperando es error.
func TestHITLProvideNonWaitingErrors(t *testing.T) {
	engine := hitlRun(t, "R", "")
	defer engine.Close()
	// 'a' está done, no waiting.
	if _, err := engine.ProvideWorkflowInput("R", "a", "x", StepDone); err == nil {
		t.Error("provide sobre un step que no está en waiting_input debe fallar")
	}
}

// step_waiting se journalea una sola vez pese a múltiples WorkflowReady.
func TestHITLStepWaitingJournaledOnce(t *testing.T) {
	engine := hitlRun(t, "R", "")
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.WorkflowReady("R")
	engine.WorkflowReady("R")
	ev, _ := engine.WorkflowJournal("R")
	n := 0
	for _, e := range ev {
		if e.EventType == EventStepWaiting && e.StepID == "gate" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("step_waiting debe journalearse una sola vez, hay %d", n)
	}
}
