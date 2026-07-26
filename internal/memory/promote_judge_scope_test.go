package memory

import (
	"context"
	"errors"
	"testing"
)

// REGRESIÓN (auditoría 2026-07-26, #11): PromoteObservation y ResolveObsRelation mutaban por id sin
// acotar por tenant. Un principal acotado que conociera un id/relation_id ajeno podía (a) forzar la
// obs de otro proyecto a 'shared'+outbox central, o (b) con supersedes, ocultar la obs de otro tenant.
// Las variantes *Ctx lo rechazan.
func TestPromoteAndJudgeScopeByTenant(t *testing.T) {
	e := newTestEngine(t)
	// Obs de proj-B (la víctima).
	if err := e.SaveObservationTypedFrom("proj-B", "", "b1", "arq/db", "elegimos sqlite embebido", 1, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}

	// El caller es de proj-A: no debe poder promover la obs de proj-B.
	ctxA := WithProjectScope(context.Background(), ProjectScope{ProjectID: "proj-A"})
	if err := e.PromoteObservationCtx(ctxA, "b1"); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("promover cross-tenant debe dar ErrCrossTenant, obtuve: %v", err)
	}
	// La víctima sigue local (no promovida).
	var scope string
	if err := e.db.QueryRow(`SELECT COALESCE(scope,'') FROM observations WHERE id='b1'`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if scope == ScopeShared {
		t.Fatal("FUGA: b1 (proj-B) fue promovida a shared por un caller de proj-A")
	}

	// El dueño (proj-B) SÍ puede promover (la guarda no rompe el caso legítimo).
	ctxB := WithProjectScope(context.Background(), ProjectScope{ProjectID: "proj-B"})
	if err := e.PromoteObservationCtx(ctxB, "b1"); err != nil {
		t.Fatalf("el dueño debe poder promover su propia obs: %v", err)
	}

	// Judge cross-tenant: una relación cuya obs de origen es de proj-B no la puede juzgar proj-A.
	if err := e.SaveObservationTypedFrom("proj-B", "", "b2", "arq/db", "sqlite es embebido y model-free", 1, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	relID, err := e.UpsertObsRelation(ObsRelation{SourceID: "b1", TargetID: "b2", Relation: RelPending, Status: RelStatusPending, Confidence: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ResolveObsRelationCtx(ctxA, relID, RelSupersedes, "agent", "intento cross-tenant"); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("juzgar cross-tenant debe dar ErrCrossTenant, obtuve: %v", err)
	}
	// b2 NO debe haber quedado oculta.
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='b2'`).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup != "" {
		t.Fatalf("FUGA: b2 (proj-B) quedó superseded por un veredicto de proj-A")
	}
	// El dueño SÍ puede juzgar.
	if err := e.ResolveObsRelationCtx(ctxB, relID, RelSupersedes, "agent", "ok"); err != nil {
		t.Fatalf("el dueño debe poder juzgar su relación: %v", err)
	}
}
