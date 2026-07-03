package memory

import (
	"encoding/json"
	"strings"
	"testing"
)

// otelDemoRun arranca un run a→b, completa a (opcionalmente failed) y devuelve el engine.
func otelDemoRun(t *testing.T, runID string, aStatus string) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	def := WorkflowDef{ID: "otelwf", Steps: []WorkflowStep{{ID: "a"}, {ID: "b", Needs: []string{"a"}}}}
	if _, err := engine.StartWorkflowRun(runID, def); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady(runID)
	if _, err := engine.CompleteWorkflowStep(runID, "a", "resultado-a", aStatus, ""); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady(runID)
	if aStatus == StepDone {
		if _, err := engine.CompleteWorkflowStep(runID, "b", "resultado-b", StepDone, ""); err != nil {
			t.Fatal(err)
		}
	}
	return engine
}

// parseOTLP deserializa el documento a un mapa laxo para inspección.
func parseOTLP(t *testing.T, jsonStr string) map[string]interface{} {
	t.Helper()
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		t.Fatalf("la traza no es JSON válido: %v\n%s", err, jsonStr)
	}
	return doc
}

func TestWorkflowTraceOTLPWellFormed(t *testing.T) {
	engine := otelDemoRun(t, "R", StepDone)
	defer engine.Close()

	out, err := engine.WorkflowTraceOTLP("R")
	if err != nil {
		t.Fatalf("WorkflowTraceOTLP: %v", err)
	}
	doc := parseOTLP(t, out)

	rs, ok := doc["resourceSpans"].([]interface{})
	if !ok || len(rs) != 1 {
		t.Fatalf("esperaba 1 resourceSpans, obtuve %v", doc["resourceSpans"])
	}
	scopeSpans := rs[0].(map[string]interface{})["scopeSpans"].([]interface{})
	spans := scopeSpans[0].(map[string]interface{})["spans"].([]interface{})
	// 1 raíz (run) + 2 steps (a, b).
	if len(spans) != 3 {
		t.Fatalf("esperaba 3 spans (raíz + a + b), obtuve %d", len(spans))
	}

	root := spans[0].(map[string]interface{})
	if id := root["traceId"].(string); len(id) != 32 {
		t.Errorf("traceId debe ser 32 hex, es %d (%q)", len(id), id)
	}
	if _, hasParent := root["parentSpanId"]; hasParent {
		t.Error("el span raíz no debe tener parentSpanId")
	}
	rootSpanID := root["spanId"].(string)
	if len(rootSpanID) != 16 {
		t.Errorf("spanId debe ser 16 hex, es %d", len(rootSpanID))
	}

	// Los steps cuelgan del raíz.
	for _, s := range spans[1:] {
		sp := s.(map[string]interface{})
		if sp["parentSpanId"].(string) != rootSpanID {
			t.Errorf("el span de step debe tener parentSpanId=raíz, tiene %q", sp["parentSpanId"])
		}
		if len(sp["spanId"].(string)) != 16 {
			t.Errorf("spanId de step debe ser 16 hex")
		}
	}
}

func TestWorkflowTraceOTLPDeterministicIDs(t *testing.T) {
	engine := otelDemoRun(t, "R", StepDone)
	defer engine.Close()

	a, err := engine.WorkflowTraceOTLP("R")
	if err != nil {
		t.Fatal(err)
	}
	b, err := engine.WorkflowTraceOTLP("R")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("dos exports del mismo run deben ser idénticos (ids deterministas)")
	}
	// Y el traceId deriva de run_id de forma estable.
	if want := otelTraceID("R"); !strings.Contains(a, want) {
		t.Errorf("la traza debe contener el traceId derivado %q", want)
	}
}

func TestWorkflowTraceOTLPFailedStatus(t *testing.T) {
	engine := otelDemoRun(t, "RF", StepFailed) // a se completa failed
	defer engine.Close()

	out, err := engine.WorkflowTraceOTLP("RF")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseOTLP(t, out)
	spans := doc["resourceSpans"].([]interface{})[0].(map[string]interface{})["scopeSpans"].([]interface{})[0].(map[string]interface{})["spans"].([]interface{})

	var found bool
	for _, s := range spans {
		sp := s.(map[string]interface{})
		if sp["name"] == "a" {
			found = true
			status := sp["status"].(map[string]interface{})
			if int(status["code"].(float64)) != otelStatusError {
				t.Errorf("el step 'a' failed debe mapear a STATUS_CODE_ERROR(2), es %v", status["code"])
			}
		}
	}
	if !found {
		t.Error("no se encontró el span del step 'a'")
	}
}

func TestWorkflowTraceOTLPSkipped(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()
	// gate falso -> el step 'skip' se salta.
	def := WorkflowDef{ID: "gw", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "skip", Needs: []string{"a"}, When: "step.a.result == nunca"},
	}}
	engine.StartWorkflowRun("RS", def)
	engine.WorkflowReady("RS")
	engine.CompleteWorkflowStep("RS", "a", "ok", StepDone, "")
	engine.WorkflowReady("RS") // evalúa el gate -> skip

	out, err := engine.WorkflowTraceOTLP("RS")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"musubi.skipped"`) {
		t.Errorf("un step saltado debe aparecer con el atributo musubi.skipped:\n%s", out)
	}
}

func TestWorkflowTraceOTLPUnknownRun(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()
	if _, err := engine.WorkflowTraceOTLP("no-existe"); err == nil {
		t.Error("un run sin eventos debe devolver error, no una traza vacía")
	}
}

func TestOtelIDsShapeAndCollision(t *testing.T) {
	if len(otelTraceID("x")) != 32 {
		t.Error("traceId debe ser 32 hex")
	}
	if len(otelSpanID("x", "y")) != 16 {
		t.Error("spanId debe ser 16 hex")
	}
	// El separador evita la colisión (run="a",step="bc") vs (run="ab",step="c").
	if otelSpanID("a", "bc") == otelSpanID("ab", "c") {
		t.Error("el separador debe evitar la colisión de concatenación")
	}
}
