package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

const wfYAML = `
id: demo
schema_version: "1.0"
steps:
  - id: a
  - id: b
    needs: [a]
`

func TestWorkflowToolStartNextComplete(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// start con definición inline
	res, e := call(t, s, "musubi_workflow", map[string]interface{}{
		"action":     "start",
		"run_id":     "r1",
		"definition": wfYAML,
	})
	if e != nil {
		t.Fatalf("start error: %+v", e)
	}
	var startOut struct {
		Ready []string `json:"ready"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &startOut)
	if len(startOut.Ready) != 1 || startOut.Ready[0] != "a" {
		t.Fatalf("ready inicial esperaba [a], obtuve %v", startOut.Ready)
	}

	// completar 'a' → 'b' queda listo
	res, e = call(t, s, "musubi_workflow", map[string]interface{}{
		"action": "complete", "run_id": "r1", "step": "a", "result": "hecho",
	})
	if e != nil {
		t.Fatalf("complete error: %+v", e)
	}
	var compOut struct {
		Ready []string `json:"ready"`
		Run   struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &compOut)
	if len(compOut.Ready) != 1 || compOut.Ready[0] != "b" {
		t.Fatalf("tras completar a esperaba [b], obtuve %v", compOut.Ready)
	}

	// completar 'b' → run done
	res, _ = call(t, s, "musubi_workflow", map[string]interface{}{
		"action": "complete", "run_id": "r1", "step": "b", "result": "ok",
	})
	json.Unmarshal([]byte(textOf(t, res)), &compOut)
	if compOut.Run.Status != "done" {
		t.Fatalf("run completo debería estar done, está %q", compOut.Run.Status)
	}
}

func TestWorkflowToolResume(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_workflow", map[string]interface{}{
		"action": "start", "run_id": "r2", "definition": wfYAML,
	}); e != nil {
		t.Fatalf("start: %+v", e)
	}
	// resume devuelve estado + ready (para retomar en otra sesión)
	res, e := call(t, s, "musubi_workflow", map[string]interface{}{"action": "resume", "run_id": "r2"})
	if e != nil {
		t.Fatalf("resume: %+v", e)
	}
	var out struct {
		Ready []string `json:"ready"`
		Run   struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &out)
	if out.Run.Status != "running" || len(out.Ready) != 1 || out.Ready[0] != "a" {
		t.Fatalf("resume inesperado: status=%q ready=%v", out.Run.Status, out.Ready)
	}
}

func TestWorkflowToolErrorPaths(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	cases := []map[string]interface{}{
		{"action": "boom"},                                       // action inválida
		{"action": "start", "definition": wfYAML},                // falta run_id
		{"action": "start", "run_id": "x"},                       // falta workflow/definition
		{"action": "start", "run_id": "x", "definition": "id: ::bad"}, // YAML/def inválida
		{"action": "next"},                                       // falta run_id
		{"action": "complete", "run_id": "x"},                    // falta step
		{"action": "status", "run_id": "noexiste"},               // run inexistente
	}
	for i, args := range cases {
		if _, e := call(t, s, "musubi_workflow", args); e == nil {
			t.Errorf("caso %d (%v): esperaba error", i, args["action"])
		}
	}
}

func TestWorkflowToolValidateAndList(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// validate de una definición buena
	res, e := call(t, s, "musubi_workflow", map[string]interface{}{"action": "validate", "definition": wfYAML})
	if e != nil {
		t.Fatalf("validate: %+v", e)
	}
	var v struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &v)
	if !v.Valid {
		t.Fatalf("definición válida marcada inválida: %v", v.Errors)
	}

	// validate de una definición con ciclo → inválida
	cyc := "id: c\nsteps:\n  - id: a\n    needs: [b]\n  - id: b\n    needs: [a]\n"
	res, _ = call(t, s, "musubi_workflow", map[string]interface{}{"action": "validate", "definition": cyc})
	json.Unmarshal([]byte(textOf(t, res)), &v)
	if v.Valid || len(v.Errors) == 0 {
		t.Fatalf("ciclo debería ser inválido, obtuve valid=%v errs=%v", v.Valid, v.Errors)
	}

	// list tras arrancar un run
	call(t, s, "musubi_workflow", map[string]interface{}{"action": "start", "run_id": "lr", "definition": wfYAML})
	res, e = call(t, s, "musubi_workflow", map[string]interface{}{"action": "list"})
	if e != nil {
		t.Fatalf("list: %+v", e)
	}
	var l struct {
		Runs []map[string]interface{} `json:"runs"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &l)
	if len(l.Runs) < 1 {
		t.Fatalf("list debería incluir el run arrancado, obtuve %v", l.Runs)
	}
}
