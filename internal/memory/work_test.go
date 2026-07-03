package memory

import "testing"

func twoUnits() []WorkUnitSpec {
	return []WorkUnitSpec{
		{Title: "unidad A", Spec: "hacer A"},
		{Title: "unidad B", Spec: "hacer B"},
	}
}

// ageLease envejece el lease de una unidad para simular a un dueño que crasheó (lease
// vencido). Es white-box: el test vive en el paquete memory y accede a e.db.
func ageLease(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE work_units SET lease_expires_at=datetime('now','-30 seconds') WHERE id=?`, id); err != nil {
		t.Fatalf("no se pudo envejecer el lease: %v", err)
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

	u1, ok, err := e.ClaimWorkUnit("b", "agente-1", 0, 0)
	if err != nil || !ok {
		t.Fatalf("primer claim debe entregar una unidad, ok=%v err=%v", ok, err)
	}
	if u1.Status != WorkClaimed || u1.ClaimedBy != "agente-1" || u1.OwnerID != "agente-1" {
		t.Errorf("la unidad reclamada debe quedar claimed por el agente (owner+claimed_by): %+v", u1)
	}
	if u1.FencingToken != 1 || u1.Attempts != 1 {
		t.Errorf("el primer claim debe dejar fencing_token=1 y attempts=1: %+v", u1)
	}

	u2, ok, err := e.ClaimWorkUnit("b", "agente-2", 0, 0)
	if err != nil || !ok {
		t.Fatalf("segundo claim debe entregar la otra unidad, ok=%v err=%v", ok, err)
	}
	if u2.ID == u1.ID {
		t.Fatalf("dos claims no deben entregar la MISMA unidad (doble-claim): %s", u1.ID)
	}

	// No quedan unidades open ni huérfanas (los leases de u1/u2 están vigentes).
	if _, ok, err := e.ClaimWorkUnit("b", "agente-3", 0, 0); err != nil || ok {
		t.Errorf("sin unidades reclamables el claim debe devolver ok=false, ok=%v err=%v", ok, err)
	}
}

func TestCompleteWorkUnit(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b", twoUnits()); err != nil {
		t.Fatal(err)
	}
	u, _, _ := e.ClaimWorkUnit("b", "a", 0, 0)
	if err := e.CompleteWorkUnit(u.ID, "resultado A", WorkDone, "", 0); err != nil {
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
	if err := e.CompleteWorkUnit(openID, "x", WorkDone, "", 0); err == nil {
		t.Error("completar una unidad no reclamada debe fallar")
	}

	// Reclamar y completar funciona.
	u, _, _ := e.ClaimWorkUnit("b", "a", 0, 0)
	if err := e.CompleteWorkUnit(u.ID, "ok", WorkDone, "", 0); err != nil {
		t.Fatalf("completar una unidad reclamada debe funcionar: %v", err)
	}
	// Re-completar una unidad ya done debe fallar (no re-cerrar/sobrescribir).
	if err := e.CompleteWorkUnit(u.ID, "otra vez", WorkDone, "", 0); err == nil {
		t.Error("re-completar una unidad ya cerrada debe fallar")
	}
}

func TestCompleteWorkUnitOwnership(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.CreateWorkBatch("b", twoUnits()); err != nil {
		t.Fatal(err)
	}
	u, _, _ := e.ClaimWorkUnit("b", "agente-1", 0, 0)
	// Otro agente NO puede cerrar la unidad de 'agente-1'.
	if err := e.CompleteWorkUnit(u.ID, "x", WorkDone, "agente-2", 0); err == nil {
		t.Error("un agente distinto no debe poder cerrar la unidad de otro")
	}
	// El dueño sí.
	if err := e.CompleteWorkUnit(u.ID, "ok", WorkDone, "agente-1", 0); err != nil {
		t.Errorf("el dueño debe poder cerrar su unidad: %v", err)
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
	if err := e.CompleteWorkUnit(b.Units[0].ID, "x", "raro", "", 0); err == nil {
		t.Error("un status de cierre inválido debe fallar")
	}
}

func TestWorkBatchStatusCounts(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", twoUnits())
	u, _, _ := e.ClaimWorkUnit("b", "a", 0, 0)
	e.CompleteWorkUnit(u.ID, "ok", WorkDone, "", 0)
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
		u, claimed, _ := e.ClaimWorkUnit("b", "a", 0, 0)
		if !claimed {
			break
		}
		if err := e.CompleteWorkUnit(u.ID, "ok", WorkDone, "", 0); err != nil {
			t.Fatalf("CompleteWorkUnit error: %v", err)
		}
	}
	if _, ok, _ := e.ActiveBatch(); ok {
		t.Error("con todas las unidades completas no debe haber batch activo")
	}
}

// --- Lease / TTL / fencing (bugfix de claims huérfanos) ---

// Escenario: reclamo lazy de una unidad huérfana + no robar un lease vigente.
func TestClaimReclaimsOrphanedLease(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", []WorkUnitSpec{{Title: "sola", Spec: "x"}})

	u1, ok, err := e.ClaimWorkUnit("b", "agente-1", 300, 5)
	if err != nil || !ok {
		t.Fatalf("primer claim debe entregar la unidad, ok=%v err=%v", ok, err)
	}
	// Con el lease vigente, nadie más la puede reclamar.
	if _, ok2, _ := e.ClaimWorkUnit("b", "agente-2", 300, 5); ok2 {
		t.Fatal("no se debe poder reclamar una unidad con lease vigente")
	}
	// El dueño crashea (lease vence).
	ageLease(t, e, u1.ID)
	u2, ok, err := e.ClaimWorkUnit("b", "agente-2", 300, 5)
	if err != nil || !ok {
		t.Fatalf("una unidad huérfana debe poder reclamarse, ok=%v err=%v", ok, err)
	}
	if u2.ID != u1.ID {
		t.Fatalf("debe reciclar la MISMA unidad, no crear otra: %s vs %s", u1.ID, u2.ID)
	}
	if u2.OwnerID != "agente-2" {
		t.Errorf("el nuevo dueño debe ser agente-2: %+v", u2)
	}
	if u2.FencingToken <= u1.FencingToken {
		t.Errorf("el fencing token debe aumentar al reciclar: %d -> %d", u1.FencingToken, u2.FencingToken)
	}
	if u2.Attempts != 2 {
		t.Errorf("attempts debe ser 2 tras el segundo claim, es %d", u2.Attempts)
	}
}

// Escenario: heartbeat mantiene vivo el lease (y un huérfano revivido no lo renueva).
func TestHeartbeatKeepsLeaseAlive(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", []WorkUnitSpec{{Title: "sola"}})
	u, _, _ := e.ClaimWorkUnit("b", "a", 300, 5)

	// Aunque el lease haya envejecido, el dueño lo renueva a tiempo.
	ageLease(t, e, u.ID)
	ok, err := e.HeartbeatWorkUnit(u.ID, "a", u.FencingToken, 300)
	if err != nil || !ok {
		t.Fatalf("el heartbeat del dueño debe renovar el lease, ok=%v err=%v", ok, err)
	}
	// Tras la renovación, otro agente no puede reclamarla (lease al futuro).
	if _, ok2, _ := e.ClaimWorkUnit("b", "b2", 300, 5); ok2 {
		t.Fatal("tras el heartbeat el lease está vigente; no se debe reclamar")
	}
}

// Escenario: heartbeat de un dueño expropiado → ok=false (debe detenerse).
func TestHeartbeatEvictedOwner(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", []WorkUnitSpec{{Title: "sola"}})
	u1, _, _ := e.ClaimWorkUnit("b", "a", 300, 5)
	ageLease(t, e, u1.ID)
	if _, ok, _ := e.ClaimWorkUnit("b", "b2", 300, 5); !ok {
		t.Fatal("b2 debe poder expropiar la unidad huérfana")
	}
	// 'a' revive e intenta renovar: ya no es el dueño.
	ok, err := e.HeartbeatWorkUnit(u1.ID, "a", u1.FencingToken, 300)
	if err != nil {
		t.Fatalf("HeartbeatWorkUnit error: %v", err)
	}
	if ok {
		t.Error("un dueño expropiado NO debe poder renovar el lease")
	}
}

// Escenario: fencing token bloquea al zombie en complete, incluso con id compartido.
func TestFencingBlocksZombieComplete(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", []WorkUnitSpec{{Title: "sola"}})
	// Ambos agentes comparten el id "worker": owner_id no los distingue, el token sí.
	u1, _, _ := e.ClaimWorkUnit("b", "worker", 300, 5)
	ageLease(t, e, u1.ID)
	u2, ok, _ := e.ClaimWorkUnit("b", "worker", 300, 5)
	if !ok {
		t.Fatal("el segundo worker debe poder expropiar la huérfana")
	}
	// El zombie (u1) intenta cerrar con su token viejo → fencing lo bloquea.
	if err := e.CompleteWorkUnit(u1.ID, "zombie", WorkDone, "worker", u1.FencingToken); err == nil {
		t.Error("el zombie con fencing token viejo NO debe poder cerrar la unidad")
	}
	// El dueño vigente sí puede, con su token.
	if err := e.CompleteWorkUnit(u2.ID, "ok", WorkDone, "worker", u2.FencingToken); err != nil {
		t.Errorf("el dueño vigente debe poder cerrar con su token: %v", err)
	}
}

// Escenario: dead-letter tras agotar los reintentos.
func TestDeadLetterAfterMaxAttempts(t *testing.T) {
	e := newTestEngine(t)
	e.CreateWorkBatch("b", []WorkUnitSpec{{Title: "cae siempre"}})
	const maxA = 3

	// Reclamar y dejar huérfana repetidamente hasta llegar a attempts == maxA.
	for i := 0; i < maxA; i++ {
		u, ok, err := e.ClaimWorkUnit("b", "a", 300, maxA)
		if err != nil {
			t.Fatalf("iter %d: error %v", i, err)
		}
		if !ok {
			t.Fatalf("iter %d: se esperaba poder reclamar (attempts aún < max)", i)
		}
		ageLease(t, e, u.ID)
	}
	// attempts == maxA: el próximo claim debe mandarla a dead-letter, no reciclarla.
	if _, ok, err := e.ClaimWorkUnit("b", "a", 300, maxA); err != nil || ok {
		t.Fatalf("con attempts>=max la unidad debe ir a dead-letter (ok=false), ok=%v err=%v", ok, err)
	}
	b, _ := e.WorkBatchStatus("b")
	if b.Failed != 1 || b.Claimed != 0 {
		t.Errorf("la unidad agotada debe quedar failed (dead-letter): %+v", b)
	}
}
