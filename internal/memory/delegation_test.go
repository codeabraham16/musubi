package memory

import "testing"

func batchWith(statuses ...string) WorkBatch {
	b := WorkBatch{}
	for i, st := range statuses {
		u := WorkUnit{ID: string(rune('a' + i)), Status: st}
		if st == WorkDone {
			u.Result = "resultado compacto de la unidad"
		}
		b.Units = append(b.Units, u)
		b.Total++
	}
	return b
}

func TestEstimateDelegationSavingsPaysOffWithVolume(t *testing.T) {
	// 3 unidades done, avoided=4000, overhead=2000 → ahorro neto 3*(4000-2000)=6000.
	b := batchWith(WorkDone, WorkDone, WorkDone)
	ds := EstimateDelegationSavings(b, 4000, 2000)
	if ds.UnitsDone != 3 {
		t.Fatalf("UnitsDone = %d, quería 3", ds.UnitsDone)
	}
	if ds.EstimatedSavings != 6000 || !ds.PaidOff {
		t.Fatalf("ahorro = %d paidOff=%v, quería 6000/true", ds.EstimatedSavings, ds.PaidOff)
	}
	if ds.OrchestratorTokens <= 0 {
		t.Errorf("los resultados deberían contar tokens de orquestador, obtuve %d", ds.OrchestratorTokens)
	}
}

func TestEstimateDelegationSavingsSmallBatchDoesNotPayOff(t *testing.T) {
	// 1 unidad done con overhead > contexto evitado → ahorro negativo, no rinde.
	b := batchWith(WorkDone)
	ds := EstimateDelegationSavings(b, 1000, 3000)
	if ds.EstimatedSavings != -2000 || ds.PaidOff {
		t.Fatalf("esperaba ahorro -2000 y paidOff=false, obtuve %d/%v", ds.EstimatedSavings, ds.PaidOff)
	}
}

func TestEstimateDelegationSavingsOnlyCountsDone(t *testing.T) {
	// open/claimed/failed no cuentan; solo la done.
	b := batchWith(WorkOpen, WorkClaimed, WorkFailed, WorkDone)
	ds := EstimateDelegationSavings(b, 4000, 2000)
	if ds.UnitsDone != 1 {
		t.Fatalf("UnitsDone = %d, quería 1 (solo la done)", ds.UnitsDone)
	}
}

func TestEstimateDelegationSavingsEmpty(t *testing.T) {
	ds := EstimateDelegationSavings(WorkBatch{}, 4000, 2000)
	if ds.UnitsDone != 0 || ds.EstimatedSavings != 0 || ds.PaidOff {
		t.Fatalf("batch vacío debería dar cero sin rinde: %+v", ds)
	}
}
