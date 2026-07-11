package memory

import (
	"context"
	"testing"
)

// TestListSharedForPullScopedAndCursor valida el primitivo de LISTADO del sync entrante (C5.3a): el
// central devuelve solo la memoria 'shared' del proyecto de la credencial (aislamiento T17-19), en
// orden de rowid, y respeta el cursor afterRowID para paginar.
func TestListSharedForPullScopedAndCursor(t *testing.T) {
	e := newTestEngine(t)

	// acme: dos shared (a1, a2) + una local (aL). web: una shared (w1) que NO debe cruzar.
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.SaveObservationTypedFrom("acme", "ana", "a1", "t/a", "deploy alpha del equipo", 1, "semantic", "shared", nil))
	must(e.SaveObservationTypedFrom("web", "bob", "w1", "t/w", "cosa de web", 1, "semantic", "shared", nil))
	must(e.SaveObservationTypedFrom("acme", "juan", "a2", "t/a", "fix beta del equipo", 1, "semantic", "shared", nil))
	must(e.SaveObservationTypedFrom("acme", "ana", "aL", "t/a", "tanteo local", 1, "semantic", "local", nil))

	ctx := WithProjectScope(context.Background(), ProjectScope{ProjectID: "acme"})

	got, err := e.ListSharedForPull(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Solo a1 y a2 (shared de acme), en orden de rowid; w1 (otro proyecto) y aL (local) fuera.
	if len(got) != 2 {
		t.Fatalf("esperaba 2 shared de acme, obtuve %d: %+v", len(got), ids(got))
	}
	if got[0].ID != "a1" || got[1].ID != "a2" {
		t.Errorf("orden por rowid esperado [a1,a2], obtuve %v", ids(got))
	}
	if got[0].Author != "ana" || got[1].Author != "juan" {
		t.Errorf("author esperado [ana,juan], obtuve [%q,%q]", got[0].Author, got[1].Author)
	}

	// Cursor: pedir con afterRowID = rowid de a1 ⇒ solo a2.
	after, err := e.ListSharedForPull(ctx, got[0].RowID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].ID != "a2" {
		t.Errorf("con cursor tras a1 esperaba [a2], obtuve %v", ids(after))
	}
}

// TestIngestSharedNoLoop valida el primitivo de INGEST (C5.3a) y su garantía ANTI-LOOP: una obs
// bajada del central se persiste (visible, atribuida) pero NO se encola en el outbox local (si no,
// rebotaría al central). Idempotente por id.
func TestIngestSharedNoLoop(t *testing.T) {
	e := newTestEngine(t)

	o := SharedObs{ID: "central-1", TopicKey: "t/x", Content: "decision del central", Importance: 1, MemType: "semantic", Author: "ana", ProjectID: "acme"}
	inserted, err := e.IngestShared(o)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Error("primer ingest debía insertar (inserted=true)")
	}

	// Persistida como shared, atribuida a ana/acme.
	var scope, author, project string
	if err := e.db.QueryRow(`SELECT scope, COALESCE(author,''), COALESCE(project_id,'') FROM observations WHERE id='central-1'`).
		Scan(&scope, &author, &project); err != nil {
		t.Fatal(err)
	}
	if scope != "shared" || author != "ana" || project != "acme" {
		t.Errorf("obs ingerida: scope=%q author=%q project=%q, esperaba shared/ana/acme", scope, author, project)
	}

	// ANTI-LOOP: NO hay fila de outbox para esta obs (no se re-sube al central).
	var outboxN int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id='central-1'`).Scan(&outboxN); err != nil {
		t.Fatal(err)
	}
	if outboxN != 0 {
		t.Errorf("ANTI-LOOP roto: la obs ingerida tiene %d fila(s) de outbox, esperaba 0", outboxN)
	}

	// Idempotente: re-ingerir la misma no duplica ni inserta.
	inserted2, err := e.IngestShared(o)
	if err != nil {
		t.Fatal(err)
	}
	if inserted2 {
		t.Error("re-ingest de la misma id debía ser update (inserted=false)")
	}
	var count int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id='central-1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("re-ingest duplicó: %d filas para central-1, esperaba 1", count)
	}
}

func ids(o []SharedObs) []string {
	out := make([]string, len(o))
	for i, x := range o {
		out[i] = x.ID
	}
	return out
}
