package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func planTwoUnits(t *testing.T, s *McpServer) memory.WorkBatch {
	t.Helper()
	res, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "plan",
		"batch":  "b1",
		"units": []map[string]string{
			{"title": "A", "spec": "hacer A"},
			{"title": "B", "spec": "hacer B"},
		},
	})
	if e != nil {
		t.Fatalf("plan error: %+v", e)
	}
	var b memory.WorkBatch
	if err := json.Unmarshal([]byte(textOf(t, res)), &b); err != nil {
		t.Fatalf("plan no devolvió WorkBatch: %v\n%s", err, textOf(t, res))
	}
	return b
}

func TestWorkToolPlanClaimComplete(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	b := planTwoUnits(t, s)
	if b.Total != 2 || b.Open != 2 {
		t.Fatalf("plan debe crear 2 unidades open: %+v", b)
	}

	// Dos claims → dos unidades distintas (sin doble-claim).
	res1, _ := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": "b1", "agent": "a1"})
	res2, _ := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": "b1", "agent": "a2"})
	var c1, c2 struct {
		Claimed bool            `json:"claimed"`
		Unit    memory.WorkUnit `json:"unit"`
	}
	json.Unmarshal([]byte(textOf(t, res1)), &c1)
	json.Unmarshal([]byte(textOf(t, res2)), &c2)
	if !c1.Claimed || !c2.Claimed || c1.Unit.ID == c2.Unit.ID {
		t.Fatalf("dos claims deben dar dos unidades distintas: %+v / %+v", c1, c2)
	}

	// Tercer claim → no quedan open.
	res3, _ := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": "b1", "agent": "a3"})
	var c3 struct {
		Claimed bool `json:"claimed"`
	}
	json.Unmarshal([]byte(textOf(t, res3)), &c3)
	if c3.Claimed {
		t.Error("sin unidades open el claim no debe entregar nada")
	}

	// Complete una y verificar status.
	if _, e := call(t, s, "musubi_work", map[string]interface{}{"action": "complete", "id": c1.Unit.ID, "result": "listo", "status": "done"}); e != nil {
		t.Fatalf("complete error: %+v", e)
	}
	res, _ := call(t, s, "musubi_work", map[string]interface{}{"action": "status", "batch": "b1"})
	var b2 memory.WorkBatch
	json.Unmarshal([]byte(textOf(t, res)), &b2)
	if b2.Done != 1 {
		t.Errorf("status debe reflejar 1 done: %+v", b2)
	}
}

func TestWorkToolPlanRequiereUnits(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_work", map[string]interface{}{"action": "plan", "batch": "x"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("plan sin units debe dar invalid params, obtuve %+v", e)
	}
}

func TestWorkToolAccionInvalida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_work", map[string]interface{}{"action": "zzz"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("action inválida debe dar invalid params, obtuve %+v", e)
	}
}
