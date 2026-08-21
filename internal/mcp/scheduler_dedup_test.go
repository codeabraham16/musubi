package mcp

import (
	"context"
	"testing"

	"musubi/internal/embedding"
)

// TestSharpenBatchOnceRespetaGate: el afilado de fondo es OPT-IN (maintenance.AutoSharpenPairs). Con el
// gate en 0 (default) es no-op aunque haya gemelas y el juez diga MERGE; encendido, afila una tanda.
func TestSharpenBatchOnceRespetaGate(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `{"verdict":"MERGE"}`}
	seedCard(t, s, "c1", "contraste-a", "contraste mínimo 4.5:1", []float32{1, 0, 0, 0})
	seedCard(t, s, "c2", "contraste-b", "el texto necesita 4.5:1", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}

	// Gate apagado (default 0): no-op. El par sigue candidato.
	s.maintenance.AutoSharpenPairs = 0
	s.sharpenBatchOnce(context.Background())
	rep, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep.Scanned != 1 {
		t.Fatalf("con el gate apagado no debía afilarse nada; scanned=%d", rep.Scanned)
	}

	// Gate encendido: afila y la gemela queda archivada.
	s.maintenance.AutoSharpenPairs = 5
	s.sharpenBatchOnce(context.Background())
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("con el gate encendido la gemela debía fusionarse; scanned=%d", rep2.Scanned)
	}
}
