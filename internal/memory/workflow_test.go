package memory

import (
	"testing"
)

const sampleWorkflowYAML = `
id: demo
name: Demo
schema_version: "1.0"
steps:
  - id: explore
  - id: build
    needs: [explore]
  - id: docs
    needs: [explore]
  - id: verify
    needs: [build, docs]
`

func TestParseAndValidateWorkflow(t *testing.T) {
	def, err := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if def.ID != "demo" || len(def.Steps) != 4 {
		t.Fatalf("def inesperada: %+v", def)
	}
	if errs := def.Validate(); len(errs) != 0 {
		t.Fatalf("workflow válido reportó errores: %v", errs)
	}
}

func TestValidateDetectsCycleAndDangling(t *testing.T) {
	cycle := WorkflowDef{ID: "c", Steps: []WorkflowStep{
		{ID: "a", Needs: []string{"b"}},
		{ID: "b", Needs: []string{"a"}},
	}}
	if errs := cycle.Validate(); len(errs) == 0 {
		t.Error("esperaba error de ciclo")
	}
	dangling := WorkflowDef{ID: "d", Steps: []WorkflowStep{
		{ID: "a", Needs: []string{"ghost"}},
	}}
	if errs := dangling.Validate(); len(errs) == 0 {
		t.Error("esperaba error de dependencia inexistente")
	}
}

func TestReadyStepsRespetaDependencias(t *testing.T) {
	def, _ := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	// Estado inicial: solo 'explore' está listo (sin needs).
	ready := def.ReadySteps(map[string]string{})
	if len(ready) != 1 || ready[0] != "explore" {
		t.Fatalf("inicial esperaba [explore], obtuve %v", ready)
	}
	// explore done → build y docs listos (ambos dependen solo de explore).
	ready = def.ReadySteps(map[string]string{"explore": StepDone})
	if len(ready) != 2 {
		t.Fatalf("tras explore esperaba 2 (build, docs), obtuve %v", ready)
	}
	// build+docs done → verify listo.
	ready = def.ReadySteps(map[string]string{"explore": StepDone, "build": StepDone, "docs": StepDone})
	if len(ready) != 1 || ready[0] != "verify" {
		t.Fatalf("esperaba [verify], obtuve %v", ready)
	}
}

func TestWorkflowRunLifecycle(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()

	def, _ := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	run, err := engine.StartWorkflowRun("run-1", def)
	if err != nil {
		t.Fatalf("StartWorkflowRun: %v", err)
	}
	if run.Status != RunRunning {
		t.Fatalf("run nuevo debería estar running, está %q", run.Status)
	}

	// next = explore
	ready, _ := engine.WorkflowReady("run-1")
	if len(ready) != 1 || ready[0] != "explore" {
		t.Fatalf("ready inicial: %v", ready)
	}

	// completar explore → build y docs listos
	if _, err := engine.CompleteWorkflowStep("run-1", "explore", "ok", StepDone, ""); err != nil {
		t.Fatalf("complete explore: %v", err)
	}
	ready, _ = engine.WorkflowReady("run-1")
	if len(ready) != 2 {
		t.Fatalf("tras explore: %v", ready)
	}

	// completar todo → run done
	engine.CompleteWorkflowStep("run-1", "build", "ok", StepDone, "")
	engine.CompleteWorkflowStep("run-1", "docs", "ok", StepDone, "")
	run, _ = engine.CompleteWorkflowStep("run-1", "verify", "ok", StepDone, "")
	if run.Status != RunDone {
		t.Fatalf("run completo debería estar done, está %q", run.Status)
	}

	// resumibilidad: una lectura fresca refleja el estado persistido
	got, ok, _ := engine.WorkflowRunStatus("run-1")
	if !ok || got.Status != RunDone || got.StepResults["verify"] != "ok" {
		t.Fatalf("estado persistido inesperado: %+v (ok=%v)", got, ok)
	}
}

// El journal registra cada transición del run en orden de seq.
func TestWorkflowJournalRecordsEvents(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()

	def := WorkflowDef{ID: "j", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "b", Needs: []string{"a"}},
	}}
	if _, err := engine.StartWorkflowRun("J", def); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady("J")
	if _, err := engine.CompleteWorkflowStep("J", "a", "ra", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady("J")
	if _, err := engine.CompleteWorkflowStep("J", "b", "rb", StepDone, ""); err != nil {
		t.Fatal(err)
	}

	ev, err := engine.WorkflowJournal("J")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	var types []string
	for _, e := range ev {
		types = append(types, e.EventType)
	}
	want := []string{EventRunStarted, EventStepCompleted, EventStepCompleted, EventRunDone}
	if len(ev) != len(want) {
		t.Fatalf("journal esperaba %d eventos, obtuve %d: %v", len(want), len(ev), types)
	}
	for i := range want {
		if ev[i].EventType != want[i] {
			t.Errorf("evento %d: esperaba %s, obtuve %s (%v)", i, want[i], ev[i].EventType, types)
		}
		if ev[i].Seq != i+1 {
			t.Errorf("seq del evento %d debe ser %d, es %d", i, i+1, ev[i].Seq)
		}
	}
	if ev[1].StepID != "a" || ev[1].Payload == "" {
		t.Errorf("step_completed(a) mal formado: %+v", ev[1])
	}
}

// Un complete repetido con la misma idempotency_key es un no-op seguro.
func TestCompleteWorkflowStepIdempotent(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()
	def := WorkflowDef{ID: "i", Steps: []WorkflowStep{{ID: "a"}, {ID: "b", Needs: []string{"a"}}}}
	engine.StartWorkflowRun("I", def)

	if _, err := engine.CompleteWorkflowStep("I", "a", "primero", StepDone, "k1"); err != nil {
		t.Fatal(err)
	}
	// Reintento con la MISMA clave y otro result → no-op (no sobrescribe).
	run, err := engine.CompleteWorkflowStep("I", "a", "segundo", StepDone, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if run.StepResults["a"] != "primero" {
		t.Errorf("el reintento idempotente no debe sobrescribir el result, quedó %q", run.StepResults["a"])
	}
	ev, _ := engine.WorkflowJournal("I")
	completes := 0
	for _, e := range ev {
		if e.EventType == EventStepCompleted && e.StepID == "a" {
			completes++
		}
	}
	if completes != 1 {
		t.Errorf("un complete idempotente repetido debe dejar 1 solo evento step_completed, hay %d", completes)
	}
}

func TestCompleteStepRechazaStepDesconocido(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()
	def, _ := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	engine.StartWorkflowRun("r", def)
	if _, err := engine.CompleteWorkflowStep("r", "ghost", "x", StepDone, ""); err == nil {
		t.Error("esperaba error por step inexistente")
	}
}

func TestWorkflowRepeatWhileLoop(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()
	yaml := `
id: loopy
schema_version: "1.0"
steps:
  - id: iterate
    repeat_while: step.iterate.result != "stop"
    max_iterations: 10
  - id: after
    needs: [iterate]
`
	def, err := ParseWorkflowDef([]byte(yaml))
	if err != nil || len(def.Validate()) != 0 {
		t.Fatalf("parse/validate: %v / %v", err, def.Validate())
	}
	engine.StartWorkflowRun("L", def)

	// dos iteraciones con result != stop → se re-abre, 'after' NO listo aún
	engine.CompleteWorkflowStep("L", "iterate", "go", StepDone, "")
	run, _, _ := engine.WorkflowRunStatus("L")
	if run.StepStatus["iterate"] != StepPending || run.StepIters["iterate"] != 1 {
		t.Fatalf("tras iter1: status=%q iters=%d", run.StepStatus["iterate"], run.StepIters["iterate"])
	}
	ready, _ := engine.WorkflowReady("L")
	if len(ready) != 1 || ready[0] != "iterate" {
		t.Fatalf("durante el loop solo 'iterate' está listo: %v", ready)
	}

	// result == stop → repeat_while falso → queda done, 'after' se libera
	engine.CompleteWorkflowStep("L", "iterate", "stop", StepDone, "")
	run, _, _ = engine.WorkflowRunStatus("L")
	if run.StepStatus["iterate"] != StepDone {
		t.Fatalf("tras stop, iterate debería estar done, está %q", run.StepStatus["iterate"])
	}
	ready, _ = engine.WorkflowReady("L")
	if len(ready) != 1 || ready[0] != "after" {
		t.Fatalf("tras cerrar el loop esperaba [after], obtuve %v", ready)
	}
}

func TestWorkflowRepeatWhileRespectaCap(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()
	yaml := `
id: capped
schema_version: "1.0"
steps:
  - id: spin
    repeat_while: spin == spin
    max_iterations: 2
`
	def, perr := ParseWorkflowDef([]byte(yaml))
	if perr != nil || len(def.Validate()) != 0 {
		t.Fatalf("parse/validate: %v / %v", perr, def.Validate())
	}
	if got := def.Steps[0].MaxIterations; got != 2 {
		t.Fatalf("max_iterations debía parsear a 2, fue %d", got)
	}
	engine.StartWorkflowRun("C", def)
	// repeat_while siempre true; debería re-abrir hasta el cap (2) y luego quedar done
	for i := 0; i < 5; i++ {
		run, _ := engine.CompleteWorkflowStep("C", "spin", "x", StepDone, "")
		if run.StepStatus["spin"] == StepDone {
			if run.StepIters["spin"] != 2 {
				t.Fatalf("esperaba parar en 2 iteraciones, paró en %d", run.StepIters["spin"])
			}
			return
		}
	}
	t.Fatal("el loop no respetó max_iterations")
}

func TestWorkflowListRuns(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()
	def, _ := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	engine.StartWorkflowRun("a", def)
	engine.StartWorkflowRun("b", def)
	runs, err := engine.WorkflowListRuns()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("esperaba 2 runs, obtuve %d", len(runs))
	}
	if runs[0].Total != 4 {
		t.Errorf("Total esperado 4, obtuve %d", runs[0].Total)
	}
}
