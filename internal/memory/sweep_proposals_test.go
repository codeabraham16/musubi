package memory

import (
	"fmt"
	"testing"
)

// ageFact envejece el created_at de una arista (por subject/predicate/object) hoursAgo horas al
// pasado, para ejercitar el TTL de barrido sin esperar tiempo real.
func ageFact(t *testing.T, e *DbEngine, subject, predicate, object string, hoursAgo int) {
	t.Helper()
	res, err := e.db.Exec(`
		UPDATE relations
		   SET created_at = datetime('now', ?)
		 WHERE predicate = ?
		   AND from_id = (SELECT id FROM entities WHERE norm = ?)
		   AND to_id   = (SELECT id FROM entities WHERE norm = ?)`,
		fmt.Sprintf("-%d hours", hoursAgo), predicate, normalizeForSim(subject), normalizeForSim(object))
	if err != nil {
		t.Fatalf("ageFact(%s,%s,%s): %v", subject, predicate, object, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("ageFact(%s,%s,%s): esperaba envejecer 1 fila, afectó %d", subject, predicate, object, n)
	}
}

// TestSweepStaleProposals cubre el barrido del pilar Cognición · F3: sólo se invalidan las
// PROPUESTAS LLM vivas más viejas que el TTL; una propuesta reciente, un hecho autoritativo viejo
// y (con ttl<=0) el no-op quedan intactos.
func TestSweepStaleProposals(t *testing.T) {
	e := newTestEngine(t)

	// Propuesta vieja (será barrida), propuesta reciente (respetada), hecho agent viejo (respetado).
	if _, err := e.SaveFactFromSourced("", "vieja", "usa", "x", "", "llm-extract:m", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFromSourced("", "nueva", "usa", "y", "", "llm-extract:m", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFromSourced("", "agente", "usa", "z", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	ageFact(t, e, "vieja", "usa", "x", 100)  // propuesta rancia
	ageFact(t, e, "agente", "usa", "z", 100) // autoritativo viejo: NO debe barrerse

	swept, err := e.SweepStaleProposals(24)
	if err != nil {
		t.Fatal(err)
	}
	if swept != 1 {
		t.Errorf("esperaba barrer 1 propuesta rancia, barrió %d", swept)
	}
	if factLive(t, e, "vieja", "usa", "x") {
		t.Error("la propuesta rancia debería quedar invalidada tras el barrido")
	}
	if !factLive(t, e, "nueva", "usa", "y") {
		t.Error("una propuesta reciente (dentro del TTL) no debería barrerse")
	}
	if !factLive(t, e, "agente", "usa", "z") {
		t.Error("un hecho autoritativo (source=agent) NUNCA debe barrerse, por viejo que sea")
	}
}

// TestSweepStaleProposalsTTLZeroNoOp: ttl <= 0 es no-op (devuelve 0, no invalida nada).
func TestSweepStaleProposalsTTLZeroNoOp(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SaveFactFromSourced("", "p", "usa", "q", "", "llm-extract:m", nil); err != nil {
		t.Fatal(err)
	}
	ageFact(t, e, "p", "usa", "q", 1000)

	for _, ttl := range []float64{0, -5} {
		swept, err := e.SweepStaleProposals(ttl)
		if err != nil {
			t.Fatal(err)
		}
		if swept != 0 {
			t.Errorf("ttl=%v debería ser no-op, barrió %d", ttl, swept)
		}
	}
	if !factLive(t, e, "p", "usa", "q") {
		t.Error("con ttl no-op la propuesta debería seguir viva")
	}
}

// TestMaintainSweepsProposals: el barrido está PLEGADO en Maintain (corre en el mantenimiento
// manual y en el auto-mantenimiento) y reporta el conteo en ProposalsSwept.
func TestMaintainSweepsProposals(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SaveFactFromSourced("", "rancia", "usa", "w", "", "llm-extract:m", nil); err != nil {
		t.Fatal(err)
	}
	ageFact(t, e, "rancia", "usa", "w", 100)

	rep, err := e.Maintain(MaintenanceOptions{ProposalTTLHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ProposalsSwept != 1 {
		t.Errorf("Maintain debería reportar ProposalsSwept=1, obtuvo %d", rep.ProposalsSwept)
	}
	if factLive(t, e, "rancia", "usa", "w") {
		t.Error("Maintain con TTL debería haber barrido la propuesta rancia")
	}
}
