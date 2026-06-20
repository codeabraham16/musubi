package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// TestMaintainThrottleAndForce verifica el guard del ciclo on-demand (T5.1):
// una segunda corrida dentro del intervalo se saltea (no-op informativo), y
// force=true ignora el throttle. Evita que un agente dispare consolidación +
// VACUUM en loop.
func TestMaintainThrottleAndForce(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	parse := func(t *testing.T, raw string) (skipped bool, lastSet bool) {
		t.Helper()
		var r struct {
			Skipped         bool   `json:"skipped"`
			LastMaintenance string `json:"last_maintenance"`
		}
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			t.Fatalf("respuesta de maintain no es JSON válido: %v\n%s", err, raw)
		}
		return r.Skipped, r.LastMaintenance != ""
	}

	// 1) DB fresca: no hay marca previa => corre (no throttled).
	res, e := call(t, s, "musubi_maintain", map[string]interface{}{})
	if e != nil {
		t.Fatalf("maintain #1 error: %+v", e)
	}
	if skipped, _ := parse(t, textOf(t, res)); skipped {
		t.Fatalf("la primera corrida no debe estar throttled: %s", textOf(t, res))
	}

	// 2) Corrida inmediata: el intervalo default (24h) no pasó => throttled (skipped).
	res2, e := call(t, s, "musubi_maintain", map[string]interface{}{})
	if e != nil {
		t.Fatalf("maintain #2 error: %+v", e)
	}
	skipped2, lastSet2 := parse(t, textOf(t, res2))
	if !skipped2 {
		t.Errorf("la segunda corrida inmediata debe estar throttled: %s", textOf(t, res2))
	}
	if !lastSet2 {
		t.Errorf("el resultado throttled debe exponer last_maintenance: %s", textOf(t, res2))
	}

	// 3) force=true ignora el throttle y corre igual.
	res3, e := call(t, s, "musubi_maintain", map[string]interface{}{"force": true})
	if e != nil {
		t.Fatalf("maintain #3 (force) error: %+v", e)
	}
	if skipped3, _ := parse(t, textOf(t, res3)); skipped3 {
		t.Errorf("force=true debe ignorar el throttle y correr: %s", textOf(t, res3))
	}
}

// TestDoctorExposesLastMaintenance verifica que musubi_doctor expone last_maintenance
// para visibilidad del estado del ciclo (T5.1), sin romper el contrato DiagnoseReport.
func TestDoctorExposesLastMaintenance(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// Tras correr el mantenimiento queda marca de last_maintenance.
	if _, e := call(t, s, "musubi_maintain", map[string]interface{}{"force": true}); e != nil {
		t.Fatalf("maintain error: %+v", e)
	}
	res, e := call(t, s, "musubi_doctor", map[string]interface{}{})
	if e != nil {
		t.Fatalf("doctor error: %+v", e)
	}
	var view struct {
		Status          string `json:"status"`
		LastMaintenance string `json:"last_maintenance"`
	}
	if err := json.Unmarshal([]byte(textOf(t, res)), &view); err != nil {
		t.Fatalf("doctor no devolvió JSON válido: %v\n%s", err, textOf(t, res))
	}
	if view.Status == "" {
		t.Errorf("doctor debe seguir reportando status (contrato DiagnoseReport): %s", textOf(t, res))
	}
	if view.LastMaintenance == "" {
		t.Errorf("doctor debe exponer last_maintenance tras una corrida: %s", textOf(t, res))
	}
}
