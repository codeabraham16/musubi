package mcp

// Tests del transporte HTTP (Track 4 / T4.2): mismo dispatch que stdio, sobre HTTP,
// con superficie de red mínima y segura (loopback-only, sin SSE, sin cross-origin).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

func newHTTPTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true}))
	t.Cleanup(ts.Close)
	return ts
}

func postMCP(t *testing.T, baseURL, body string) (*http.Response, JsonRpcResponse) {
	t.Helper()
	resp, err := http.Post(baseURL+mcpHTTPPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var jr JsonRpcResponse
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &jr)
	}
	return resp, jr
}

func TestHTTPToolsList(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, jr := postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, esperaba 200", resp.StatusCode)
	}
	if jr.JsonRpc != "2.0" || jr.Error != nil {
		t.Fatalf("respuesta inesperada: %+v", jr)
	}
	m, ok := jr.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result no es objeto: %T", jr.Result)
	}
	tools, ok := m["tools"].([]interface{})
	if !ok || len(tools) != 34 {
		t.Fatalf("esperaba 34 tools por HTTP, obtuve %v (%d)", ok, len(tools))
	}
}

func TestHTTPInitializeAndToolCall(t *testing.T) {
	ts := newHTTPTestServer(t)

	_, init := postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	res := init.Result.(map[string]interface{})
	if res["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion por HTTP inesperado: %v", res["protocolVersion"])
	}

	_, save := postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"musubi_save_observation","arguments":{"topic_key":"http/t","content":"guardado por HTTP"}}}`)
	if save.Error != nil {
		t.Fatalf("tools/call por HTTP devolvió error: %+v", save.Error)
	}
}

func TestHTTPNotificationReturns202(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, _ := postMCP(t, ts.URL, `{"jsonrpc":"2.0","method":"tools/list"}`) // sin id = notificación
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("notificación: status = %d, esperaba 202", resp.StatusCode)
	}
}

func TestHTTPParseAndMethodErrors(t *testing.T) {
	ts := newHTTPTestServer(t)

	_, parseErr := postMCP(t, ts.URL, `esto no es json`)
	if parseErr.Error == nil || parseErr.Error.Code != codeParseError {
		t.Errorf("esperaba parse error, obtuve %+v", parseErr.Error)
	}

	_, notFound := postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":9,"method":"metodo/inexistente"}`)
	if notFound.Error == nil || notFound.Error.Code != codeMethodNotFound {
		t.Errorf("esperaba method not found, obtuve %+v", notFound.Error)
	}
}

func TestHTTPGetIsMethodNotAllowed(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, err := http.Get(ts.URL + mcpHTTPPath)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp status = %d, esperaba 405 (SSE reservado)", resp.StatusCode)
	}
}

func TestHTTPRejectsCrossOrigin(t *testing.T) {
	ts := newHTTPTestServer(t)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+mcpHTTPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST con Origin: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Origin cross-site: status = %d, esperaba 403", resp.StatusCode)
	}
}

func TestListenAndServeHTTPRejectsNonLoopback(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := s.ListenAndServeHTTP(ctx, config.ServiceConfig{Enabled: true, Addr: "0.0.0.0:7717", RequestTimeoutSeconds: 60})
	if err == nil {
		t.Fatal("esperaba error al intentar bind no-loopback (0.0.0.0)")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Errorf("el error debe mencionar loopback, obtuve: %v", err)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:7717": true,
		"127.0.0.1":      true,
		"localhost:7717": true,
		"[::1]:7717":     true,
		"0.0.0.0:7717":   false,
		":7717":          false,
		"192.168.1.5:80": false,
		"example.com:80": false,
	}
	for host, want := range cases {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, esperaba %v", host, got, want)
		}
	}
}
