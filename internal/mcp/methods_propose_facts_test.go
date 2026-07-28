package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestProposeFactsQuarantineAndReview cubre el loop completo del pilar Cognición · F2 por la superficie
// MCP: proponer (caller-borrowed) → cuarentena → revisar (include_proposed) → corroborar (save_fact)
// promueve a autoritativo.
func TestProposeFactsQuarantineAndReview(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx := context.Background()

	// Proponer: entra en cuarentena con source 'llm-extract:m'.
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"alpha","predicate":"usa","object":"potion"}],"model":"m"}`)); rerr != nil {
		t.Fatalf("propose_facts: %v", rerr)
	}

	// El read autoritativo NO la ve (cuarentena).
	res, err := s.engine.RecallFactsCtx(ctx, "alpha", 3, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 0 {
		t.Errorf("cuarentena: la propuesta no debería aparecer en el read autoritativo; got %d", len(res.Facts))
	}

	// Con include_proposed (vía WithIncludeProposed) se revisa.
	res, err = s.engine.RecallFactsCtx(memory.WithIncludeProposed(ctx), "alpha", 3, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("revisión: la propuesta debería verse con include_proposed; got %d", len(res.Facts))
	}

	// Corroborar con save_fact la promueve a autoritativa: ya visible por default.
	if _, rerr := s.toolSaveFact(ctx, json.RawMessage(`{"subject":"alpha","predicate":"usa","object":"potion"}`)); rerr != nil {
		t.Fatalf("save_fact: %v", rerr)
	}
	res, err = s.engine.RecallFactsCtx(ctx, "alpha", 3, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("tras corroborar, el hecho debería ser visible por default; got %d", len(res.Facts))
	}
}

// TestProposeFactsValidation: lote vacío o tripleta incompleta ⇒ error (validate-all-then-save).
func TestProposeFactsValidation(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx := context.Background()

	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[]}`)); rerr == nil {
		t.Error("facts vacío debería fallar")
	}
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"a","predicate":"","object":"b"}]}`)); rerr == nil {
		t.Error("una tripleta con predicate vacío debería rechazar el lote")
	}
}
