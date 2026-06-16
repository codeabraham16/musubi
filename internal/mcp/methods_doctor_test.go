package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func TestDoctorToolDiagnose(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := saveObs(t, s, "a", "topic/x", "una observación sana"); e != nil {
		t.Fatal(e)
	}
	res, e := call(t, s, "musubi_doctor", map[string]interface{}{})
	if e != nil {
		t.Fatalf("musubi_doctor error: %+v", e)
	}
	var rep memory.DiagnoseReport
	if err := json.Unmarshal([]byte(textOf(t, res)), &rep); err != nil {
		t.Fatalf("respuesta no es DiagnoseReport: %v\n%s", err, textOf(t, res))
	}
	if rep.Status != "ok" {
		t.Errorf("DB sana debe diagnosticar ok, obtuve %q", rep.Status)
	}
}

func TestDoctorToolRepair(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, err := s.engine.UpsertObsRelation(memory.ObsRelation{SourceID: "fantasma", TargetID: "otro", Relation: memory.RelPending, Status: memory.RelStatusPending}); err != nil {
		t.Fatal(err)
	}
	res, e := call(t, s, "musubi_doctor", map[string]interface{}{
		"check": "orphan_relations", "repair": true, "mode": "apply",
	})
	if e != nil {
		t.Fatalf("musubi_doctor repair error: %+v", e)
	}
	var rr memory.RepairResult
	if err := json.Unmarshal([]byte(textOf(t, res)), &rr); err != nil {
		t.Fatalf("respuesta no es RepairResult: %v\n%s", err, textOf(t, res))
	}
	if !rr.Applied {
		t.Errorf("el repair en modo apply debe aplicarse: %+v", rr)
	}
}

func TestDoctorToolRepairRequiereCheck(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_doctor", map[string]interface{}{"repair": true}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("repair sin check debe dar invalid params, obtuve %+v", e)
	}
}
