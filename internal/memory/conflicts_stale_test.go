package memory

import "testing"

// addPendingRel inserta una relación 'pending' directa (sin pasar por la detección), para armar
// escenarios de cola controlados.
func addPendingRel(t *testing.T, e *DbEngine, src, tgt string) {
	t.Helper()
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: src, TargetID: tgt, Relation: RelPending, Status: RelStatusPending, Confidence: 0.5,
	}); err != nil {
		t.Fatalf("upsert %s→%s: %v", src, tgt, err)
	}
}

// El cleanup 'stale_conflicts' poda el RUIDO ESTRUCTURAL de la cola —target histórico y recíprocos—
// sin tocar las relaciones nota↔nota legítimas ni las resueltas. Es la limpieza del residuo que se
// acumuló antes de que existieran las guardas (complementaryPair).
func TestStaleConflictsPodaRuidoEstructural(t *testing.T) {
	e := newTestEngine(t)

	saveAt(t, e, "nota-a", "arch/db", "Usamos Postgres como base principal.", "2026-01-01 10:00:00")
	saveAt(t, e, "nota-b", "arch/api", "El API valida tokens JWT.", "2026-01-02 10:00:00")
	saveAt(t, e, "nota-c", "arch/cache", "Cacheamos en Redis con TTL corto.", "2026-01-05 10:00:00")
	saveAt(t, e, "sdd-x", "sdd/mi-cambio/spec", "Spec del cambio mi-cambio.", "2026-01-03 10:00:00")
	saveAt(t, e, "commit-y", "git-commit", "feat: agrega algo al sistema.", "2026-01-04 10:00:00")

	// (1) target HISTÓRICO (commit / SDD) ⇒ ruido: complementaryPair ya no lo crea.
	addPendingRel(t, e, "nota-a", "sdd-x")
	addPendingRel(t, e, "nota-a", "commit-y")
	// (2) RECÍPROCO duplicado: nota-a↔nota-b en ambas direcciones.
	addPendingRel(t, e, "nota-a", "nota-b")
	addPendingRel(t, e, "nota-b", "nota-a")
	// (3) LEGÍTIMA nota→nota, sin recíproco y target no histórico ⇒ se conserva.
	addPendingRel(t, e, "nota-c", "nota-b")

	ids, err := staleConflictIDs(e)
	if err != nil {
		t.Fatal(err)
	}
	// Podar: nota-a→sdd-x, nota-a→commit-y, y el lado NO canónico del recíproco (nota-b→nota-a,
	// porque "nota-b" > "nota-a"). La dirección canónica (nota-a→nota-b) se conserva.
	if len(ids) != 3 {
		t.Fatalf("esperaba 3 relaciones a podar, obtuve %d", len(ids))
	}

	before, _ := e.AllObsRelations()
	n, err := applyDeleteStaleConflicts(e)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("borró %d, esperaba 3", n)
	}

	after, err := e.AllObsRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)-3 {
		t.Errorf("quedaron %d relaciones, esperaba %d", len(after), len(before)-3)
	}

	survives := func(src, tgt string) bool {
		for _, r := range after {
			if r.SourceID == src && r.TargetID == tgt {
				return true
			}
		}
		return false
	}
	if !survives("nota-a", "nota-b") {
		t.Error("se podó la dirección canónica del recíproco (nota-a→nota-b)")
	}
	if !survives("nota-c", "nota-b") {
		t.Error("se podó una relación legítima nota→nota (nota-c→nota-b)")
	}

	// Idempotente: una segunda corrida no encuentra nada.
	if again, err := countStaleConflicts(e); err != nil || again != 0 {
		t.Errorf("segunda corrida: esperaba 0 sin error, obtuve %d (%v)", again, err)
	}
}
