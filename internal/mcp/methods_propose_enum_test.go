package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// newServerWithEnum construye un server con un vocabulario controlado de predicados (Cognición F3).
func newServerWithEnum(t *testing.T, allowed ...string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognitionConfig(config.CognitionConfig{AllowedPredicates: allowed}))
}

// TestProposeFactsPredicateEnum: con enum configurado, un predicado dentro (case-insensitive) se
// acepta y uno fuera rechaza el LOTE entero sin guardar nada.
func TestProposeFactsPredicateEnum(t *testing.T) {
	s := newServerWithEnum(t, "usa", "depende_de")
	ctx := context.Background()

	// Dentro del vocabulario (con otra caja): aceptado.
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"a","predicate":"USA","object":"b"}],"model":"m"}`)); rerr != nil {
		t.Fatalf("predicado dentro del enum debería aceptarse: %v", rerr)
	}

	// Fuera del vocabulario: el lote entero se rechaza.
	_, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"a","predicate":"usa","object":"b"},{"subject":"a","predicate":"utiliza","object":"c"}],"model":"m"}`))
	if rerr == nil {
		t.Fatal("un predicado fuera del enum debería rechazar el lote")
	}
	if rerr.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams, obtuve %d", rerr.Code)
	}

	// Nada del lote rechazado se guardó: sólo la tripleta aceptada previa (a-usa-b) existe como propuesta.
	res, err := s.engine.RecallFactsCtx(memory.WithIncludeProposed(ctx), "a", 2, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Facts {
		if f.Object == "c" {
			t.Error("la tripleta del lote rechazado no debería haberse guardado (validate-all-then-save)")
		}
	}
}

// TestProposeFactsEnumEmptyAllowsAll: sin enum (default), cualquier predicado se acepta (bit-idéntico).
func TestProposeFactsEnumEmptyAllowsAll(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{}) // sin WithCognitionConfig ⇒ enum vacío
	if _, rerr := s.toolProposeFacts(context.Background(), json.RawMessage(`{"facts":[{"subject":"a","predicate":"cualquier_predicado_inventado","object":"b"}]}`)); rerr != nil {
		t.Fatalf("con enum vacío cualquier predicado debería aceptarse: %v", rerr)
	}
}
