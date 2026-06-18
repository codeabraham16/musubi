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
	if _, err := engine.CompleteWorkflowStep("run-1", "explore", "ok", StepDone); err != nil {
		t.Fatalf("complete explore: %v", err)
	}
	ready, _ = engine.WorkflowReady("run-1")
	if len(ready) != 2 {
		t.Fatalf("tras explore: %v", ready)
	}

	// completar todo → run done
	engine.CompleteWorkflowStep("run-1", "build", "ok", StepDone)
	engine.CompleteWorkflowStep("run-1", "docs", "ok", StepDone)
	run, _ = engine.CompleteWorkflowStep("run-1", "verify", "ok", StepDone)
	if run.Status != RunDone {
		t.Fatalf("run completo debería estar done, está %q", run.Status)
	}

	// resumibilidad: una lectura fresca refleja el estado persistido
	got, ok, _ := engine.WorkflowRunStatus("run-1")
	if !ok || got.Status != RunDone || got.StepResults["verify"] != "ok" {
		t.Fatalf("estado persistido inesperado: %+v (ok=%v)", got, ok)
	}
}

func TestCompleteStepRechazaStepDesconocido(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()
	def, _ := ParseWorkflowDef([]byte(sampleWorkflowYAML))
	engine.StartWorkflowRun("r", def)
	if _, err := engine.CompleteWorkflowStep("r", "ghost", "x", StepDone); err == nil {
		t.Error("esperaba error por step inexistente")
	}
}
