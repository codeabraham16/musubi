package mcp

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// newServerConMemoria construye un McpServer con un DbEngine en directorio temporal.
func newServerConMemoria(t *testing.T) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	return s, engine
}

// TestLogDecisionAceptada verifica que una decisión "accepted" válida se inserta
// y la respuesta es un textResult exitoso.
func TestLogDecisionAceptada(t *testing.T) {
	s, engine := newServerConMemoria(t)

	res, rpcErr := call(t, s, "musubi_log_skill_decision", map[string]interface{}{
		"skill_id": "go-gin",
		"name":     "Go — Gin",
		"decision": "accepted",
		"reason":   "alta utilidad para el proyecto",
	})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	if !strings.Contains(strings.ToLower(resp.Content[0].Text), "accepted") &&
		!strings.Contains(strings.ToLower(resp.Content[0].Text), "aceptada") {
		t.Errorf("el mensaje de éxito debe mencionar la decisión, obtuve: %q", resp.Content[0].Text)
	}

	// Verificar que la fila fue insertada en la base de datos.
	decisiones, err := engine.GetSkillDecisions()
	if err != nil {
		t.Fatalf("GetSkillDecisions error: %v", err)
	}
	if len(decisiones) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(decisiones))
	}
	if decisiones[0].Decision != "accepted" {
		t.Errorf("decision incorrecta: %q", decisiones[0].Decision)
	}
}

// TestLogDecisionRechazada verifica que una decisión "rejected" válida se inserta correctamente.
func TestLogDecisionRechazada(t *testing.T) {
	s, engine := newServerConMemoria(t)

	_, rpcErr := call(t, s, "musubi_log_skill_decision", map[string]interface{}{
		"skill_id": "go-echo",
		"decision": "rejected",
		"reason":   "redundante con gin",
	})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}

	decisiones, err := engine.GetSkillDecisions()
	if err != nil {
		t.Fatalf("GetSkillDecisions error: %v", err)
	}
	if len(decisiones) != 1 || decisiones[0].Decision != "rejected" {
		t.Errorf("decisión incorrecta: %+v", decisiones)
	}
}

// TestLogDecisionValorInvalidoDevuelveInvalidParams verifica que un valor de
// decision fuera del enum retorna codeInvalidParams sin insertar fila.
func TestLogDecisionValorInvalidoDevuelveInvalidParams(t *testing.T) {
	s, engine := newServerConMemoria(t)

	_, rpcErr := call(t, s, "musubi_log_skill_decision", map[string]interface{}{
		"skill_id": "go-gin",
		"decision": "maybe",
	})
	if rpcErr == nil || rpcErr.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams, obtuve: %+v", rpcErr)
	}

	// No debe haberse insertado ninguna fila.
	decisiones, _ := engine.GetSkillDecisions()
	if len(decisiones) != 0 {
		t.Errorf("no debe haber filas con decision inválida, obtuve %d", len(decisiones))
	}
}

// TestLogDecisionSkillIDFaltanteDevuelveInvalidParams verifica que la ausencia de
// skill_id retorna codeInvalidParams.
func TestLogDecisionSkillIDFaltanteDevuelveInvalidParams(t *testing.T) {
	s, _ := newServerConMemoria(t)

	_, rpcErr := call(t, s, "musubi_log_skill_decision", map[string]interface{}{
		"decision": "accepted",
	})
	if rpcErr == nil || rpcErr.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por skill_id faltante, obtuve: %+v", rpcErr)
	}
}

// TestLogDecisionMultiplesLlamadasMismoSkillID verifica que varias llamadas con
// el mismo skill_id crean filas separadas (log append-only).
func TestLogDecisionMultiplesLlamadasMismoSkillID(t *testing.T) {
	s, engine := newServerConMemoria(t)

	for i := 0; i < 3; i++ {
		_, rpcErr := call(t, s, "musubi_log_skill_decision", map[string]interface{}{
			"skill_id": "go-gin",
			"decision": "accepted",
		})
		if rpcErr != nil {
			t.Fatalf("llamada %d error: %+v", i, rpcErr)
		}
	}

	decisiones, err := engine.GetSkillDecisions()
	if err != nil {
		t.Fatalf("GetSkillDecisions error: %v", err)
	}
	if len(decisiones) != 3 {
		t.Errorf("esperaba 3 filas (append-only), obtuve %d", len(decisiones))
	}
}
