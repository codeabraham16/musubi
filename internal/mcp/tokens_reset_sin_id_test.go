package mcp

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// HALLAZGO (revisión ronda 2, lente producción): el reset de musubi_tokens ya no vaciaba TODAS las
// sesiones, pero sin `session_id` ponía en cero «la última que escribió». En producción el MCP nunca
// tiene ese id (Claude Code no se lo pasa), y con dos terminales activas la última en escribir es a
// menudo LA OTRA —su precheck cobra en cada Edit—, así que el reset pedido desde A borraba la cuenta
// de B. Ahora, sin id y con más de una sesión, se niega y lista las sesiones para elegir.
//
// Sabotaje que la pone roja: que el reset sin id vuelva a imputarse a la última.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\t\t\tif len(sesiones) > 1 {"
// arnes: a="\t\t\tif len(sesiones) > 1 && false {"
func TestTokensResetSinIDNoBorraLaCuentaDeOtraTerminal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, _ = s.engine.LedgerAdd("A", "turn_recall", 5000)   // turno de la terminal A
	_, _ = s.engine.LedgerAdd("B", "precheck_code", 3000) // B sigue editando en su terminal

	// El agente de A pide el reset sin saber su session_id.
	_, rpcErr := call(t, s, "musubi_tokens", map[string]interface{}{"action": "reset"})
	if rpcErr == nil || rpcErr.Code != codeInvalidParams {
		t.Fatalf("sin session_id y con dos sesiones el reset tenía que negarse, dio %+v", rpcErr)
	}
	if !strings.Contains(rpcErr.Message, "A (5000 tokens") || !strings.Contains(rpcErr.Message, "B (3000 tokens") {
		t.Errorf("la negativa tiene que listar las sesiones para que el agente elija la suya: %s", rpcErr.Message)
	}
	if b, _ := s.engine.LedgerStatusDe("B"); b.Total != 3000 {
		t.Errorf("el reset pedido desde A puso en cero la cuenta de B: %d (esperado 3000)", b.Total)
	}

	// Con el id, pone en cero ésa y sólo ésa.
	if _, rpcErr := call(t, s, "musubi_tokens", map[string]interface{}{"action": "reset", "session_id": "A"}); rpcErr != nil {
		t.Fatalf("reset con session_id: %+v", rpcErr)
	}
	if a, _ := s.engine.LedgerStatusDe("A"); a.Total != 0 {
		t.Errorf("el reset de A no la puso en cero: %d", a.Total)
	}
	if b, _ := s.engine.LedgerStatusDe("B"); b.Total != 3000 {
		t.Errorf("el reset de A tocó a B: %d", b.Total)
	}
}
