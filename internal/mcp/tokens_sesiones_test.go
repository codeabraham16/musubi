package mcp

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

type reporteTokensPrueba struct {
	SessionID string `json:"session_id"`
	Total     int    `json:"total"`
	Sesiones  []struct {
		SessionID string `json:"session_id"`
		Total     int    `json:"total"`
	} `json:"sesiones"`
}

func leerReporteTokens(t *testing.T, s *McpServer, args map[string]interface{}) reporteTokensPrueba {
	t.Helper()
	res, e := call(t, s, "musubi_tokens", args)
	if e != nil {
		t.Fatalf("musubi_tokens: %+v", e)
	}
	var r reporteTokensPrueba
	if err := json.Unmarshal([]byte(textOf(t, res)), &r); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	return r
}

// HALLAZGO DE LA REVISIÓN ADVERSARIAL: musubi_tokens corre por MCP y no conoce su propia sesión, así
// que mostraba el número de «la última que escribió» —con varias terminales, a menudo otra— sin decir
// de quién era. Ahora lista todas, y con session_id reporta o resetea la que se le pida.
func TestTokensListaLasSesionesYDejaElegirUna(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, _ = s.engine.LedgerAdd("A", "turn_recall", 100)
	_, _ = s.engine.LedgerAdd("B", "precheck_code", 9000)

	r := leerReporteTokens(t, s, map[string]interface{}{})
	if r.SessionID != "B" || len(r.Sesiones) != 2 {
		t.Fatalf("sin session_id: reporte de la última (B) con las 2 sesiones listadas; obtuve %+v", r)
	}

	r = leerReporteTokens(t, s, map[string]interface{}{"session_id": "A"})
	if r.SessionID != "A" || r.Total != 100 {
		t.Fatalf("con session_id=A tenía que reportar A (100); obtuve %+v", r)
	}

	// reset de una sola: A a cero, B intacta.
	_ = leerReporteTokens(t, s, map[string]interface{}{"action": "reset", "session_id": "A"})
	if b, _ := s.engine.LedgerStatusDe("B"); b.Total != 9000 {
		t.Fatalf("resetear A borró a B: %d", b.Total)
	}
	if a, _ := s.engine.LedgerStatusDe("A"); a.Total != 0 {
		t.Fatalf("A tenía que quedar en cero: %d", a.Total)
	}
}
