package memory

import "testing"

func twoUnits() []WorkUnitSpec {
	return []WorkUnitSpec{
		{Title: "unidad A", Spec: "hacer A"},
		{Title: "unidad B", Spec: "hacer B"},
	}
}

func TestCreateWorkBatch(t *testing.T) {
	e := newTestEngine(t)
	b, err := e.CreateWorkBatch("batch-1", twoUnits())
	if err != nil {
		t.Fatalf("CreateWorkBatch error: %v", err)
	}
	if b.BatchID != "batch-1" || b.Total != 2 || b.Open != 2 {
		t.Fatalf("batch inesperado: %+v", b)
	}
	if len(b.Units) != 2 || b.Units[0].Seq != 0 || b.Units[1].Seq != 1 {
		t.Errorf("unidades mal secuenciadas: %+v", b.Units)
	}
	if b.Units[0].ID == "" || b.Units[0].Status != WorkOpen {
		t.Errorf("unidad debe tener id y estar open: %+v", b.Units[0])
	}
}

func TestCreateWorkBatchValidations(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b", nil); err == nil {
		t.Error("crear un batch sin unidades debe fallar")
	}
	// batch_id vacío → se genera uno.
	b, err := e.CreateWorkBatch("", twoUnits())
	if err != nil {
		t.Fatalf("CreateWorkBatch error: %v", err)
	}
	if b.BatchID == "" {
		t.Error("con batch_id vacío se debe generar uno")
	}
}

func TestClaimWorkUnitAtomicNoDoubleClaim(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b", twoUnits()); err != nil {
		t.Fatal(err)
	}

	u1, ok, err := e.ClaimWorkUnit("b", "agente-1")
	if err != nil || !ok {
		t.Fatalf("primer claim debe entregar una unidad, ok=%v err=%v", ok, err)
	}
	if u1.Status != WorkClaimed || u1.ClaimedBy != "agente-1" {
		t.Errorf("la unidad reclamada debe quedar claimed por el agente: %+v", u1)
	}

	u2, ok, err := e.ClaimWorkUnit("b", "agente-2")
	if err != nil || !ok {
		t.Fatalf("segundo claim debe entregar la otra unidad, ok=%v err=%v", ok, err)
	}
	if u2.ID == u1.ID {
		t.Fatalf("dos claims no deben entregar la MISMA unidad (doble-claim): %s", u1.ID)
	}

	// No quedan unidades open.
	if _, ok, err := e.ClaimWorkUnit("b", "agente-3"); err != nil || ok {
		t.Errorf("sin unidades open el claim debe devolver ok=false, ok=%v err=%v", ok, err)
	}
}

func TestCompleteWorkUnit(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b", twoUnits()); err != nil {
		t.Fatal(err)
	}
	u, _, _ := e.ClaimWorkUnit("b", "a")
	if err := e.CompleteWorkUnit(u.ID, "resultado A", WorkDone); err != nil {
		t.Fatalf("CompleteWorkUnit error: %v", err)
	}
	b, _ := e.WorkBatchStatus("b")
	if b.Done != 1 {
		t.Errorf("esperaba 1 done, obtuve %+v", b)
	}
	var found bool
	for _, it := range b.Units {
		if it.ID == u.ID {
			found = true
			if it.Result != "resultado A" || it.Status != WorkDone {
				t.Errorf("la unidad completada debe tener result y status done: %+v", it)
			}
		}
	}
	if !found {
		t.Error("la unidad completada debe seguir en el batch")
	}
}

func TestCompleteWorkUnitRequiresClaimed(t *testing.T) {
	e := newTestEngine(t)
	b, _ := e.CreateWorkBatch("b", twoUnits())
	openID := b.Units[0].ID

	// Completar una unidad OPEN (nunca reclamada) debe fallar.
	if err := e.CompleteWorkUnit(openID, "x", WorkDone); err == nil {
		t.Error("completar una unidad no reclamada debe fallar")
	}

	// Reclamar y completar funciona.
	u, _, _ := e.ClaimWorkUnit("b", "a")
	if err := e.CompleteWorkUnit(u.ID, "ok", WorkDone); err != nil {
		t.Fatalf("completar una unidad reclamada debe funcionar: %v", err)
	}
	// Re-completar una unidad ya done debe fallar (no re-cerrar/sobrescribir).
	if err := e.CompleteWorkUnit(u.ID, "otra vez", WorkDone); err == nil {
		t.Error("re-completar una unidad ya cerrada debe fallar")
	}
}

func TestActiveBatchPrefiereMasReciente(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b1", twoUnits()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CreateWorkBatch("b2", twoUnits()); err != nil {
		t.Fatal(err)
	}
	b, ok, err := e.ActiveBatch()
	if err != nil || !ok {
		t.Fatalf("debe haber un batch activo, ok=%v err=%v", ok, err)
	}
	if b.BatchID != "b2" {
		t.Errorf("ActiveBatch debe preferir el batch más reciente (b2), obtuve %q", b.BatchID)
	}
}

func TestCompleteWorkUnitStatusInvalido(t *testing.T) {
	e := newTestEngine(t)
	b, _ := e.CreateWorkBatch("b", twoUnits())
	if err := e.CompleteWorkUnit(b.Units[0].ID, "x", "raro"); err == nil {
		t.Error("un status de cierre inválido debe fallar")
	}
}

func TestWorkBatchStatusCounts(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", twoUnits())
	u, _, _ := e.ClaimWorkUnit("b", "a")
	e.CompleteWorkUnit(u.ID, "ok", WorkDone)
	b, err := e.WorkBatchStatus("b")
	if err != nil {
		t.Fatalf("WorkBatchStatus error: %v", err)
	}
	if b.Total != 2 || b.Done != 1 || b.Open != 1 {
		t.Errorf("conteos incorrectos: %+v", b)
	}
}

func TestClearWorkBatch(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", twoUnits())
	if err := e.ClearWorkBatch("b"); err != nil {
		t.Fatalf("ClearWorkBatch error: %v", err)
	}
	b, _ := e.WorkBatchStatus("b")
	if b.Total != 0 {
		t.Errorf("tras clear el batch debe quedar vacío: %+v", b)
	}
}

func TestActiveBatch(t *testing.T) {
	e := newTestEngine(t)
	if _, ok, _ := e.ActiveBatch(); ok {
		t.Fatal("sin batches no debe haber batch activo")
	}
	e.CreateWorkBatch("b", twoUnits())
	b, ok, err := e.ActiveBatch()
	if err != nil || !ok || b.BatchID != "b" {
		t.Fatalf("debe reportar el batch con unidades pendientes, ok=%v b=%+v err=%v", ok, b, err)
	}
	// Reclamar y completar todo → ya no hay batch activo.
	for {
		u, claimed, _ := e.ClaimWorkUnit("b", "a")
		if !claimed {
			break
		}
		if err := e.CompleteWorkUnit(u.ID, "ok", WorkDone); err != nil {
			t.Fatalf("CompleteWorkUnit error: %v", err)
		}
	}
	if _, ok, _ := e.ActiveBatch(); ok {
		t.Error("con todas las unidades completas no debe haber batch activo")
	}
}
