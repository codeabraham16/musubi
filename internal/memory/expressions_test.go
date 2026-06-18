package memory

import "testing"

func TestEvalConditionBasics(t *testing.T) {
	ctx := map[string]string{
		"step.build.status": "done",
		"step.build.result": "all green ok",
		"step.test.status":  "failed",
	}
	cases := []struct {
		expr string
		want bool
	}{
		{"", true}, // vacío = true
		{"step.build.status == done", true},
		{"step.build.status == failed", false},
		{"step.build.status != failed", true},
		{`step.build.result contains "green"`, true},
		{`step.build.result contains "rojo"`, false},
		{"step.build.status == done and step.test.status == failed", true},
		{"step.build.status == done and step.test.status == done", false},
		{"step.build.status == done or step.test.status == done", true},
		{"not step.test.status == done", true},
		{"(step.build.status == done or step.test.status == done) and step.build.result contains ok", true},
	}
	for _, c := range cases {
		got, err := EvalCondition(c.expr, ctx)
		if err != nil {
			t.Errorf("EvalCondition(%q) error: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("EvalCondition(%q) = %v, quiero %v", c.expr, got, c.want)
		}
	}
}

func TestEvalConditionErrors(t *testing.T) {
	for _, expr := range []string{`step.x == "abierto`, "a == == b", "( a == b"} {
		if _, err := EvalCondition(expr, map[string]string{}); err == nil {
			t.Errorf("esperaba error para %q", expr)
		}
	}
}

func TestWorkflowGatingSkipsBranch(t *testing.T) {
	engine, _ := NewDbEngine(t.TempDir())
	defer engine.Close()

	// 'build' siempre; 'deploy' solo si build.result contiene "ok"; 'rollback' si NO.
	yaml := `
id: gated
schema_version: "1.0"
steps:
  - id: build
  - id: deploy
    needs: [build]
    when: step.build.result contains "ok"
  - id: rollback
    needs: [build]
    when: not (step.build.result contains "ok")
`
	def, err := ParseWorkflowDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if errs := def.Validate(); len(errs) != 0 {
		t.Fatalf("validate: %v", errs)
	}
	if _, err := engine.StartWorkflowRun("g", def); err != nil {
		t.Fatalf("start: %v", err)
	}

	// build con result "ok" → deploy listo, rollback se salta
	engine.CompleteWorkflowStep("g", "build", "deploy ok", StepDone)
	ready, err := engine.WorkflowReady("g")
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if len(ready) != 1 || ready[0] != "deploy" {
		t.Fatalf("esperaba [deploy], obtuve %v", ready)
	}
	run, _, _ := engine.WorkflowRunStatus("g")
	if run.StepStatus["rollback"] != StepSkipped {
		t.Errorf("rollback debería estar skipped, está %q", run.StepStatus["rollback"])
	}

	// completar deploy → run done (rollback skipped cuenta como terminal)
	run, _ = engine.CompleteWorkflowStep("g", "deploy", "ok", StepDone)
	if run.Status != RunDone {
		t.Errorf("run debería estar done con rollback skipped, está %q", run.Status)
	}
}
