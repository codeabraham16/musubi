package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestForceRedactScrubsLocalIngest valida la redacción forzada server-side (Track 16 F1
// 16.1d): con forceRedact activo (el central es infra compartida), un ingest que declara
// scope=local con un secreto se guarda REDACTADO — cerrando el hueco por el que un secreto
// crudo entraba al pozo compartido. Sin forceRedact, un scope=local se guarda crudo (control).
func TestForceRedactScrubsLocalIngest(t *testing.T) {
	const secret = "AKIA1234567890ABCDEF" // matchea la regla aws-access-key, no está en la allowlist
	const marker = "zredactmarker"

	saveLocalAndFetch := func(t *testing.T, forceRedact bool) string {
		t.Helper()
		engine, err := memory.NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer engine.Close()
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
		s.forceRedact = forceRedact

		args := map[string]any{
			"topic_key": "cfg/x",
			"content":   "config " + marker + " key " + secret,
			"scope":     "local", // el cliente declara local a propósito
		}
		raw, _ := json.Marshal(args)
		if _, rpcErr := s.toolSaveObservation(context.Background(), raw); rpcErr != nil {
			t.Fatalf("save: %+v", rpcErr)
		}
		res, err := engine.SearchObservationsFTS(context.Background(), marker, 5)
		if err != nil || len(res) == 0 {
			t.Fatalf("no se encontró la observación guardada: err=%v len=%d", err, len(res))
		}
		return res[0].Content
	}

	// Con forceRedact: el secreto NO debe quedar en el contenido guardado.
	if got := saveLocalAndFetch(t, true); strings.Contains(got, secret) {
		t.Errorf("con forceRedact, el secreto debía redactarse; contenido guardado: %q", got)
	}
	// Control sin forceRedact: scope=local se guarda crudo (comportamiento local histórico).
	if got := saveLocalAndFetch(t, false); !strings.Contains(got, secret) {
		t.Errorf("sin forceRedact, scope=local debía guardarse crudo; contenido: %q", got)
	}
}
