package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// runRequests envía cada línea por Serve y devuelve las respuestas decodificadas.
func runRequests(t *testing.T, lines ...string) []JsonRpcResponse {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("engine error: %v", err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	s.Serve(in, &out)

	var responses []JsonRpcResponse
	for _, raw := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if raw == "" {
			continue
		}
		var resp JsonRpcResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			t.Fatalf("respuesta no es JSON válido (%q): %v", raw, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestServeInitialize(t *testing.T) {
	resps := runRequests(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if len(resps) != 1 || resps[0].Error != nil {
		t.Fatalf("respuesta inesperada: %+v", resps)
	}
	result := resps[0].Result.(map[string]interface{})
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion inesperado: %v", result["protocolVersion"])
	}
}

func TestServeToolsListCountsAllTools(t *testing.T) {
	resps := runRequests(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result := resps[0].Result.(map[string]interface{})
	tools := result["tools"].([]interface{})
	if len(tools) != 13 {
		t.Fatalf("esperaba 13 herramientas, obtuve %d", len(tools))
	}
}

func TestServeToolsCallHappyPath(t *testing.T) {
	resps := runRequests(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"musubi_save_observation","arguments":{"topic_key":"t","content":"contenido"}}}`)
	if len(resps) != 1 || resps[0].Error != nil {
		t.Fatalf("esperaba éxito, obtuve %+v", resps)
	}
}

func TestServeParseError(t *testing.T) {
	resps := runRequests(t, `esto no es json`)
	if len(resps) != 1 || resps[0].Error == nil || resps[0].Error.Code != codeParseError {
		t.Fatalf("esperaba parse error, obtuve %+v", resps)
	}
}

func TestServeMethodNotFound(t *testing.T) {
	resps := runRequests(t, `{"jsonrpc":"2.0","id":4,"method":"metodo/inexistente"}`)
	if len(resps) != 1 || resps[0].Error == nil || resps[0].Error.Code != codeMethodNotFound {
		t.Fatalf("esperaba method not found, obtuve %+v", resps)
	}
}

func TestServeInvalidParams(t *testing.T) {
	resps := runRequests(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"musubi_save_observation","arguments":{"content":"sin topic"}}}`)
	if len(resps) != 1 || resps[0].Error == nil || resps[0].Error.Code != codeInvalidParams {
		t.Fatalf("esperaba invalid params, obtuve %+v", resps)
	}
}

func TestServeNotificationNoResponse(t *testing.T) {
	// Una notificación (sin id) a un método desconocido no debe producir respuesta.
	resps := runRequests(t, `{"jsonrpc":"2.0","method":"metodo/desconocido"}`)
	if len(resps) != 0 {
		t.Fatalf("esperaba 0 respuestas para notificación, obtuve %+v", resps)
	}
}
