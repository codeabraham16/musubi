package memory

import (
	"testing"
)

// --- Helpers ---

// countOutbox cuenta las filas de outbox para un obs_id dado.
func countOutbox(t *testing.T, e *DbEngine, obsID string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id = ?`, obsID).Scan(&n); err != nil {
		t.Fatalf("countOutbox: %v", err)
	}
	return n
}

// outboxRow lee status/attempts/enqueued_hash de un obs_id.
func outboxRow(t *testing.T, e *DbEngine, obsID string) (status string, attempts int, hash string) {
	t.Helper()
	var h *string
	if err := e.db.QueryRow(`SELECT status, attempts, enqueued_hash FROM outbox WHERE obs_id = ?`, obsID).
		Scan(&status, &attempts, &h); err != nil {
		t.Fatalf("outboxRow(%s): %v", obsID, err)
	}
	if h != nil {
		hash = *h
	}
	return status, attempts, hash
}

// --- T1.1: migración v11 ---

func TestMigrationV11OutboxSchema(t *testing.T) {
	e := newTestEngine(t)

	// user_version quedó en la última migración (12 en este binario: F2.2 añadió
	// embeddings.model_id sobre la v11 del outbox).
	v, err := e.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	if v != latestSchemaVersion() {
		t.Errorf("user_version = %d, esperaba %d", v, latestSchemaVersion())
	}
	if latestSchemaVersion() != 12 {
		t.Errorf("latestSchemaVersion() = %d, esperaba 12", latestSchemaVersion())
	}

	// La tabla outbox existe con las columnas esperadas.
	rows, err := e.db.Query(`PRAGMA table_info(outbox)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(outbox): %v", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        interface{}
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		cols[name] = true
	}
	for _, want := range []string{"id", "obs_id", "status", "enqueued_hash", "attempts", "next_attempt_at", "last_error", "created_at", "updated_at"} {
		if !cols[want] {
			t.Errorf("falta la columna outbox.%s", want)
		}
	}

	// El índice idx_outbox_claim existe.
	var idxName string
	if err := e.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_outbox_claim'`).Scan(&idxName); err != nil {
		t.Fatalf("no se encontró idx_outbox_claim: %v", err)
	}
}

// --- T9: enqueue transaccional al promover / guardar ---

func TestPromoteEnqueuesOutboxOnce(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "t", "contenido para promover", nil); err != nil {
		t.Fatal(err)
	}
	// Un save local NO encola.
	if n := countOutbox(t, e, "o1"); n != 0 {
		t.Fatalf("save local no debía encolar, hay %d filas", n)
	}
	// Promover encola exactamente 1 fila pending.
	if err := e.PromoteObservation("o1"); err != nil {
		t.Fatalf("PromoteObservation: %v", err)
	}
	if n := countOutbox(t, e, "o1"); n != 1 {
		t.Fatalf("tras promover esperaba 1 fila de outbox, hay %d", n)
	}
	if status, attempts, _ := outboxRow(t, e, "o1"); status != outboxPending || attempts != 0 {
		t.Errorf("fila esperada pending/0, obtuve %s/%d", status, attempts)
	}
	// Re-promover (idempotente) no duplica.
	if err := e.PromoteObservation("o1"); err != nil {
		t.Fatalf("re-PromoteObservation: %v", err)
	}
	if n := countOutbox(t, e, "o1"); n != 1 {
		t.Fatalf("re-promover no debía duplicar, hay %d filas", n)
	}
}

func TestPromoteMissingObservation(t *testing.T) {
	e := newTestEngine(t)
	if err := e.PromoteObservation("nope"); err == nil {
		t.Fatal("promover un id inexistente debía fallar")
	}
}

func TestSaveSharedEnqueues(t *testing.T) {
	e := newTestEngine(t)
	// Guardar directamente como shared encola.
	if err := e.SaveObservationTyped("s1", "t", "contenido shared", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if n := countOutbox(t, e, "s1"); n != 1 {
		t.Fatalf("save shared debía encolar 1, hay %d", n)
	}
	// Guardar como local no encola.
	if err := e.SaveObservationTyped("l1", "t", "contenido local", 1.0, "", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if n := countOutbox(t, e, "l1"); n != 0 {
		t.Fatalf("save local no debía encolar, hay %d", n)
	}
}

func TestReSaveSameContentNoDuplicate(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("s1", "t", "contenido original", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	_, _, hash1 := outboxRow(t, e, "s1")
	// Marcar como sent para verificar que un re-save con MISMO contenido no la re-encola.
	if err := e.MarkOutboxSent("s1"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("s1", "t", "contenido original", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := outboxRow(t, e, "s1"); status != outboxSent {
		t.Errorf("re-save con mismo contenido no debía re-encolar; status=%s", status)
	}
	// Re-save con contenido CAMBIADO re-encola (vuelve a pending, hash distinto).
	if err := e.SaveObservationTyped("s1", "t", "contenido MODIFICADO distinto", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	status, attempts, hash2 := outboxRow(t, e, "s1")
	if status != outboxPending {
		t.Errorf("re-save con contenido cambiado debía volver a pending; status=%s", status)
	}
	if attempts != 0 {
		t.Errorf("re-encolado debía resetear attempts a 0, obtuve %d", attempts)
	}
	if hash1 == hash2 {
		t.Errorf("el enqueued_hash debía cambiar al cambiar el contenido (%q == %q)", hash1, hash2)
	}
	if n := countOutbox(t, e, "s1"); n != 1 {
		t.Errorf("re-sync no debía duplicar la fila, hay %d", n)
	}
}

func TestRollbackLeavesNoOutbox(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "t", "para rollback", nil); err != nil {
		t.Fatal(err)
	}
	// Simular una promoción cuya tx hace rollback: UPDATE scope + enqueue, luego Rollback.
	tx, err := e.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE observations SET scope=? WHERE id=?`, ScopeShared, "o1"); err != nil {
		t.Fatal(err)
	}
	if err := enqueueOutboxTx(tx, "o1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Ni el scope cambió ni quedó fila de outbox.
	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id=?`, "o1").Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if scope != ScopeLocal {
		t.Errorf("el rollback no revirtió el scope, quedó %q", scope)
	}
	if n := countOutbox(t, e, "o1"); n != 0 {
		t.Errorf("el rollback dejó %d filas de outbox huérfanas", n)
	}
}

// --- T9: backfill ---

func TestBackfillOutboxIdempotent(t *testing.T) {
	e := newTestEngine(t)
	// Crear una shared SIN pasar por el enqueue: UPDATE directo (simula una shared de F1).
	if err := e.SaveObservation("f1", "t", "shared preexistente", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET scope=? WHERE id=?`, ScopeShared, "f1"); err != nil {
		t.Fatal(err)
	}
	// De entrada no hay fila (el UPDATE directo no encoló).
	if n := countOutbox(t, e, "f1"); n != 0 {
		t.Fatalf("precondición: no debía haber fila, hay %d", n)
	}
	seeded, err := e.BackfillOutbox()
	if err != nil {
		t.Fatalf("BackfillOutbox: %v", err)
	}
	if seeded != 1 {
		t.Errorf("esperaba sembrar 1, sembró %d", seeded)
	}
	if n := countOutbox(t, e, "f1"); n != 1 {
		t.Errorf("tras backfill esperaba 1 fila, hay %d", n)
	}
	// Segundo llamado no siembra nada (idempotente).
	seeded2, err := e.BackfillOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if seeded2 != 0 {
		t.Errorf("segundo backfill no debía sembrar, sembró %d", seeded2)
	}
}

// --- T9: claim atómico + lease + auto-recuperación ---

func TestClaimOutboxBatchLeaseAndRecovery(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("c1", "topic-c1", "contenido c1", 2.5, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}

	// Primer claim: reclama la fila y trae el payload.
	items, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatalf("ClaimOutboxBatch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("esperaba 1 item reclamado, obtuve %d", len(items))
	}
	it := items[0]
	if it.ObsID != "c1" || it.TopicKey != "topic-c1" || it.Content != "contenido c1" || it.MemType != "semantic" {
		t.Errorf("payload mal reconstruido: %+v", it)
	}
	if it.Importance != 2.5 {
		t.Errorf("importance esperada 2.5, obtuve %v", it.Importance)
	}

	// Segundo claim inmediato (dentro del lease de 60s): NO re-reclama.
	items2, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items2) != 0 {
		t.Errorf("una fila leaseada no debía re-reclamarse, obtuve %d", len(items2))
	}

	// Auto-recuperación: si el lease venció (next_attempt_at en el pasado), se re-reclama.
	if _, err := e.db.Exec(`UPDATE outbox SET next_attempt_at = datetime('now','-1 seconds') WHERE obs_id=?`, "c1"); err != nil {
		t.Fatal(err)
	}
	items3, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items3) != 1 {
		t.Errorf("un claim con lease vencido debía re-reclamarse, obtuve %d", len(items3))
	}
}

func TestClaimOnlyDueRows(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("d1", "t", "vencida", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("d2", "t", "futura", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	// d2 con next_attempt_at en el futuro: no debe reclamarse.
	if _, err := e.db.Exec(`UPDATE outbox SET next_attempt_at = datetime('now','+3600 seconds') WHERE obs_id=?`, "d2"); err != nil {
		t.Fatal(err)
	}
	items, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ObsID != "d1" {
		t.Fatalf("sólo d1 (vencida) debía reclamarse, obtuve %+v", items)
	}
}

// --- T9: marks + attempts ---

func TestMarksSentRetryDead(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("m1", "t", "para marks", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}

	// Retry: incrementa attempts, posterga y vuelve a pending.
	if err := e.MarkOutboxRetry("m1", 120, "boom transitorio"); err != nil {
		t.Fatal(err)
	}
	status, attempts, _ := outboxRow(t, e, "m1")
	if status != outboxPending || attempts != 1 {
		t.Errorf("tras retry esperaba pending/1, obtuve %s/%d", status, attempts)
	}
	// next_attempt_at quedó en el futuro (no reclamable ya).
	items, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("una fila con backoff futuro no debía reclamarse, obtuve %d", len(items))
	}

	// Otro retry incrementa de nuevo.
	if err := e.MarkOutboxRetry("m1", 0, "otra vez"); err != nil {
		t.Fatal(err)
	}
	if _, attempts, _ := outboxRow(t, e, "m1"); attempts != 2 {
		t.Errorf("attempts esperado 2, obtuve %d", attempts)
	}

	// Dead: pasa a dead con last_error.
	if err := e.MarkOutboxDead("m1", "fallo permanente"); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := outboxRow(t, e, "m1"); status != outboxDead {
		t.Errorf("tras dead esperaba %s, obtuve %s", outboxDead, status)
	}

	// Sent: deja sent.
	if err := e.SaveObservationTyped("m2", "t", "para sent", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxSent("m2"); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := outboxRow(t, e, "m2"); status != outboxSent {
		t.Errorf("tras sent esperaba %s, obtuve %s", outboxSent, status)
	}
}

func TestOutboxStats(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := e.SaveObservationTyped(id, "t", "contenido "+id, 1.0, "", ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.MarkOutboxSent("a"); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxDead("b", "x"); err != nil {
		t.Fatal(err)
	}
	pending, sent, dead, err := e.OutboxStats()
	if err != nil {
		t.Fatal(err)
	}
	if pending != 1 || sent != 1 || dead != 1 {
		t.Errorf("stats esperadas pending=1 sent=1 dead=1, obtuve pending=%d sent=%d dead=%d", pending, sent, dead)
	}
}

// TestClaimAtomicNoDoubleDelivery valida que dos claims concurrentes no reclaman la misma
// fila (R5): el UPDATE..RETURNING atómico garantiza que cada fila se entregue a lo sumo una
// vez por ventana de lease. Con -race en CI esto ejercita la carrera real.
func TestClaimAtomicNoDoubleDelivery(t *testing.T) {
	e := newTestEngine(t)
	const n = 20
	for i := 0; i < n; i++ {
		id := string(rune('a'+i%26)) + string(rune('0'+i/26))
		if err := e.SaveObservationTyped(id, "t", "contenido "+id, 1.0, "", ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}

	type res struct {
		items []OutboxItem
		err   error
	}
	ch := make(chan res, 2)
	for w := 0; w < 2; w++ {
		go func() {
			items, err := e.ClaimOutboxBatch(n, 60)
			ch <- res{items, err}
		}()
	}
	seen := map[string]bool{}
	total := 0
	for w := 0; w < 2; w++ {
		r := <-ch
		if r.err != nil {
			t.Fatalf("claim concurrente: %v", r.err)
		}
		for _, it := range r.items {
			if seen[it.ObsID] {
				t.Errorf("obs_id %s reclamado por dos claims (doble entrega)", it.ObsID)
			}
			seen[it.ObsID] = true
			total++
		}
	}
	if total != n {
		t.Errorf("entre ambos claims se esperaban %d filas únicas, hubo %d", n, total)
	}
}

// --- sync-hardening: requeue + health ---

func TestRequeueDeadOutbox(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("d1", "t", "para dead", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	// Simular un dead con attempts y last_error.
	if err := e.MarkOutboxRetry("d1", 0, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxDead("d1", "fallo permanente"); err != nil {
		t.Fatal(err)
	}

	n, err := e.RequeueDeadOutbox()
	if err != nil || n != 1 {
		t.Fatalf("RequeueDeadOutbox esperaba (1,nil), obtuve (%d,%v)", n, err)
	}
	status, attempts, _ := outboxRow(t, e, "d1")
	if status != outboxPending || attempts != 0 {
		t.Errorf("tras requeue esperaba pending/0, obtuve %s/%d", status, attempts)
	}
	// Idempotente: sin filas dead, re-encola 0 sin error.
	if n2, err := e.RequeueDeadOutbox(); err != nil || n2 != 0 {
		t.Errorf("requeue sin dead esperaba (0,nil), obtuve (%d,%v)", n2, err)
	}
}

func TestOutboxHealth(t *testing.T) {
	e := newTestEngine(t)

	// Outbox vacío ⇒ ceros sin error.
	h0, err := e.OutboxHealth()
	if err != nil {
		t.Fatal(err)
	}
	if h0.Pending != 0 || h0.Sent != 0 || h0.Dead != 0 || h0.OldestPendingAgeSec != 0 || h0.LastError != "" {
		t.Errorf("outbox vacío esperaba todo en cero, obtuve %+v", h0)
	}

	// Mezcla: 1 pending, 1 sent, 1 dead (con last_error).
	for _, id := range []string{"p", "s", "d"} {
		if err := e.SaveObservationTyped(id, "t", "contenido "+id, 1.0, "", ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.MarkOutboxSent("s"); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxDead("d", "el central rechazó"); err != nil {
		t.Fatal(err)
	}

	h, err := e.OutboxHealth()
	if err != nil {
		t.Fatal(err)
	}
	if h.Pending != 1 || h.Sent != 1 || h.Dead != 1 {
		t.Errorf("counts esperados 1/1/1, obtuve %d/%d/%d", h.Pending, h.Sent, h.Dead)
	}
	if h.OldestPendingAgeSec < 0 {
		t.Errorf("antigüedad de la pendiente no puede ser negativa, obtuve %d", h.OldestPendingAgeSec)
	}
	if h.LastError != "el central rechazó" {
		t.Errorf("last_error esperado 'el central rechazó', obtuve %q", h.LastError)
	}
}
