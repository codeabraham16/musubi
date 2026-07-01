package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// claimAndComplete reclama la próxima unidad del batch y la cierra con un resultado.
func claimAndComplete(t *testing.T, s *McpServer, batch, agent, result string) {
	t.Helper()
	res, e := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": batch, "agent": agent})
	if e != nil {
		t.Fatalf("claim: %+v", e)
	}
	var cl struct {
		Claimed bool            `json:"claimed"`
		Unit    memory.WorkUnit `json:"unit"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &cl)
	if !cl.Claimed {
		t.Fatalf("no se pudo reclamar una unidad del batch %q", batch)
	}
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "complete", "id": cl.Unit.ID, "result": result, "agent": agent,
	}); e != nil {
		t.Fatalf("complete: %+v", e)
	}
}

func TestWorkSavingsEstimate(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	planTwoUnits(t, s) // batch "b1", 2 unidades

	// Sin nada completado, el ahorro es cero.
	res, e := call(t, s, "musubi_work", map[string]interface{}{"action": "savings", "batch": "b1"})
	if e != nil {
		t.Fatalf("savings (vacío): %+v", e)
	}
	var ds memory.DelegationSavings
	json.Unmarshal([]byte(textOf(t, res)), &ds)
	if ds.UnitsDone != 0 || ds.EstimatedSavings != 0 {
		t.Fatalf("sin unidades done esperaba cero, obtuve %+v", ds)
	}

	// Completar las 2 unidades → con defaults 4000/2000 el ahorro es 2*(4000-2000)=4000.
	claimAndComplete(t, s, "b1", "sub-1", "resumen de A")
	claimAndComplete(t, s, "b1", "sub-2", "resumen de B")

	res, e = call(t, s, "musubi_work", map[string]interface{}{"action": "savings", "batch": "b1"})
	if e != nil {
		t.Fatalf("savings: %+v", e)
	}
	json.Unmarshal([]byte(textOf(t, res)), &ds)
	if ds.UnitsDone != 2 || ds.EstimatedSavings != 4000 || !ds.PaidOff {
		t.Fatalf("esperaba 2 done / ahorro 4000 / paidOff, obtuve %+v", ds)
	}
	if ds.Note == "" {
		t.Errorf("la estimación debería traer una nota explicativa model-free")
	}
}

func TestWorkSavingsRequiresBatch(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_work", map[string]interface{}{"action": "savings"}); e == nil {
		t.Fatal("savings sin batch debería fallar")
	}
}
