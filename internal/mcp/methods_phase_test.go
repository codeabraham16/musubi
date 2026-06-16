package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

func phaseResult(t *testing.T, res interface{}) phaseView {
	t.Helper()
	var v phaseView
	if err := json.Unmarshal([]byte(textOf(t, res)), &v); err != nil {
		t.Fatalf("respuesta no es phaseView: %v\n%s", err, textOf(t, res))
	}
	return v
}

func TestPhaseToolStatusVacio(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_phase", map[string]interface{}{})
	if e != nil {
		t.Fatalf("musubi_phase status error: %+v", e)
	}
	if v := phaseResult(t, res); v.Active {
		t.Errorf("sin pipeline activo, active debe ser false: %+v", v)
	}
}

func TestPhaseToolStartAdvanceClear(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// start
	res, e := call(t, s, "musubi_phase", map[string]interface{}{"action": "start", "task": "refactor"})
	if e != nil {
		t.Fatalf("start error: %+v", e)
	}
	v := phaseResult(t, res)
	if !v.Active || v.Phase != "explore" || v.Directive == "" {
		t.Fatalf("start debe activar la fase explore con directiva: %+v", v)
	}

	// advance → plan
	res, _ = call(t, s, "musubi_phase", map[string]interface{}{"action": "advance"})
	if v := phaseResult(t, res); v.Phase != "plan" || v.Index != 1 {
		t.Errorf("advance debe ir a plan(1): %+v", v)
	}

	// set → verify
	res, _ = call(t, s, "musubi_phase", map[string]interface{}{"action": "set", "phase": "verify"})
	if v := phaseResult(t, res); v.Phase != "verify" || v.Index != 3 {
		t.Errorf("set verify debe ir a index 3: %+v", v)
	}

	// advance desde la última → done, sin tarea activa
	res, _ = call(t, s, "musubi_phase", map[string]interface{}{"action": "advance"})
	if v := phaseResult(t, res); v.Active {
		t.Errorf("avanzar desde la última fase debe completar el pipeline: %+v", v)
	}
}

func TestPhaseToolStartRequiereTask(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_phase", map[string]interface{}{"action": "start"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("start sin task debe dar invalid params, obtuve %+v", e)
	}
}

func TestPhaseToolAccionInvalida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_phase", map[string]interface{}{"action": "fly"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("action inválida debe dar invalid params, obtuve %+v", e)
	}
}
