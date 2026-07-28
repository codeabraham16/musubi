package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// newServerWithResolution construye un server con la resolución de entidades F4 encendida.
func newServerWithResolution(t *testing.T, threshold float64) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognitionConfig(config.CognitionConfig{EntityResolutionThreshold: threshold}))
}

// TestProposeFactsEntityResolution: con la resolución encendida, un subject similar a una entidad
// existente se canonicaliza a ella (no fragmenta el grafo); con la resolución apagada, no.
func TestProposeFactsEntityResolution(t *testing.T) {
	s := newServerWithResolution(t, 0.7)
	ctx := context.Background()

	// Primera propuesta crea la entidad "potion".
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"potion","predicate":"usa","object":"mana"}],"model":"m"}`)); rerr != nil {
		t.Fatalf("propose 1: %v", rerr)
	}
	// Segunda propuesta usa la variante "potions": debe canonicalizarse a "potion".
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"potions","predicate":"potencia","object":"hechizo"}],"model":"m"}`)); rerr != nil {
		t.Fatalf("propose 2: %v", rerr)
	}

	// La entidad "potions" no debería existir (canonicalizada) ⇒ recall sobre ella no ve la arista.
	res, err := s.engine.RecallFactsCtx(memory.WithIncludeProposed(ctx), "potions", 2, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 0 {
		t.Errorf("la variante 'potions' debería haberse canonicalizado; recall sobre 'potions' devolvió %d hechos", len(res.Facts))
	}
	// En cambio "potion" debe tener ambas aristas (usa→mana y potencia→hechizo).
	res, err = s.engine.RecallFactsCtx(memory.WithIncludeProposed(ctx), "potion", 2, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 2 {
		t.Errorf("'potion' debería concentrar ambas aristas tras la canonicalización; got %d", len(res.Facts))
	}
}

// TestProposeFactsResolutionOffKeepsVariants: con la resolución apagada (threshold 0), la variante
// se guarda como entidad propia (bit-idéntico al comportamiento sin F4).
func TestProposeFactsResolutionOffKeepsVariants(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{}) // sin WithCognitionConfig ⇒ threshold 0
	ctx := context.Background()

	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"potion","predicate":"usa","object":"mana"}]}`)); rerr != nil {
		t.Fatalf("propose 1: %v", rerr)
	}
	if _, rerr := s.toolProposeFacts(ctx, json.RawMessage(`{"facts":[{"subject":"potions","predicate":"potencia","object":"hechizo"}]}`)); rerr != nil {
		t.Fatalf("propose 2: %v", rerr)
	}
	// Sin resolución, 'potions' es su propia entidad con su arista.
	res, err := s.engine.RecallFactsCtx(memory.WithIncludeProposed(ctx), "potions", 2, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("con resolución apagada 'potions' debería conservar su arista; got %d", len(res.Facts))
	}
}
