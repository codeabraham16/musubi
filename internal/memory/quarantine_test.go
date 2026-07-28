package memory

import (
	"context"
	"database/sql"
	"testing"
)

// TestFactQuarantineAndCorroboration: una propuesta LLM está en CUARENTENA (no aparece en el read
// autoritativo de RecallFacts) hasta que un 'agent' la CORROBORA, promoviéndola y haciéndola visible.
// Pilar Cognición · F1.
func TestFactQuarantineAndCorroboration(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if _, err := e.SaveFactFromSourced("", "alpha", "usa", "potion", "", "llm-extract:x", nil); err != nil {
		t.Fatal(err)
	}
	res, err := e.RecallFactsCtx(ctx, "alpha", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 0 {
		t.Errorf("cuarentena: una propuesta LLM no debería aparecer en RecallFacts; got %d hechos", len(res.Facts))
	}

	if _, err := e.SaveFactFromSourced("", "alpha", "usa", "potion", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	res, err = e.RecallFactsCtx(ctx, "alpha", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("tras corroborar, el hecho debería ser visible; got %d", len(res.Facts))
	}
	if got := factSource(t, e, "alpha", "usa", "potion"); got != "agent" {
		t.Errorf("corroboración: source=%q, esperaba agent", got)
	}
}

// TestProvenancePrecedenceNoDowngrade: una re-propuesta LLM sobre un hecho 'agent' NO lo degrada.
func TestProvenancePrecedenceNoDowngrade(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SaveFactFromSourced("", "x", "p", "y", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFromSourced("", "x", "p", "y", "", "llm-extract:z", nil); err != nil {
		t.Fatal(err)
	}
	if got := factSource(t, e, "x", "p", "y"); got != "agent" {
		t.Errorf("re-propuesta LLM no debe degradar: source=%q, esperaba agent", got)
	}
}

// TestProposeOnlyCardinality: una propuesta LLM contradictoria NO invalida un hecho 'agent'
// (propose-only); un save 'agent' SÍ invalida (cardinalidad single-valued, sin regresión).
func TestProposeOnlyCardinality(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"estado"}

	if _, err := e.SaveFactFromSourced("", "srv", "estado", "activo", "", "agent", sv); err != nil {
		t.Fatal(err)
	}
	// Propuesta LLM contradictoria: no debe tachar el hecho agent.
	if _, err := e.SaveFactFromSourced("", "srv", "estado", "caido", "", "llm-extract:x", sv); err != nil {
		t.Fatal(err)
	}
	if !factLive(t, e, "srv", "estado", "activo") {
		t.Error("propose-only: una propuesta LLM no debería invalidar el hecho agent")
	}
	// Un agente SÍ invalida por cardinalidad (comportamiento histórico intacto).
	if _, err := e.SaveFactFromSourced("", "srv", "estado", "mantenimiento", "", "agent", sv); err != nil {
		t.Fatal(err)
	}
	if factLive(t, e, "srv", "estado", "activo") {
		t.Error("cardinalidad: un save agent debería invalidar el hecho contradictorio previo")
	}
}

// TestWithIncludeProposedRevealsFacts: una propuesta LLM está oculta en RecallFacts por default
// (cuarentena) y se revela cuando el contexto lleva WithIncludeProposed (superficie de revisión · F2).
func TestWithIncludeProposedRevealsFacts(t *testing.T) {
	e := newTestEngine(t)

	if _, err := e.SaveFactFromSourced("", "alpha", "usa", "potion", "", "llm-extract:m", nil); err != nil {
		t.Fatal(err)
	}
	res, err := e.RecallFactsCtx(context.Background(), "alpha", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 0 {
		t.Errorf("por default la propuesta debería estar en cuarentena; got %d", len(res.Facts))
	}

	res, err = e.RecallFactsCtx(WithIncludeProposed(context.Background()), "alpha", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("con WithIncludeProposed la propuesta debería verse; got %d", len(res.Facts))
	}
}

// factLive indica si la arista (subject,predicate,object) está viva (invalidated_at IS NULL).
func factLive(t *testing.T, e *DbEngine, subject, predicate, object string) bool {
	t.Helper()
	var invalidatedAt sql.NullString
	err := e.db.QueryRow(`
		SELECT r.invalidated_at FROM relations r
		JOIN entities fe ON fe.id = r.from_id
		JOIN entities te ON te.id = r.to_id
		WHERE fe.norm=? AND r.predicate=? AND te.norm=?`,
		normalizeForSim(subject), predicate, normalizeForSim(object)).Scan(&invalidatedAt)
	if err != nil {
		t.Fatalf("factLive(%s,%s,%s): %v", subject, predicate, object, err)
	}
	return !invalidatedAt.Valid
}
