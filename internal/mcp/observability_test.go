package mcp

// Tests de observabilidad del modo servicio (Track 4 / T4.4): health, readiness,
// métricas y correlation IDs.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

func TestHealthz(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok"`) {
		t.Fatalf("healthz: status=%d body=%q", resp.StatusCode, body)
	}
}

func TestReadyz(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "ready") {
		t.Fatalf("readyz: status=%d body=%q (motor debería responder GetMeta)", resp.StatusCode, body)
	}
}

func TestMetricsCountsRequests(t *testing.T) {
	ts := newHTTPTestServer(t)
	// Generar tráfico: un request OK al MCP.
	postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := string(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics: status=%d", resp.StatusCode)
	}
	if !strings.Contains(text, "musubi_http_requests_total") {
		t.Fatalf("metrics no expone el contador esperado:\n%s", text)
	}
	if !strings.Contains(text, `musubi_http_requests_total{result="ok"} 1`) {
		t.Errorf("esperaba 1 request ok contado, métricas:\n%s", text)
	}
}

func TestMetricsRequiresAuthWhenTokenSet(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, token: "tok", loopbackOnly: true}))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/metrics") // sin bearer
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("metrics sin token: status=%d, esperaba 401", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics con token: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("metrics con token: status=%d, esperaba 200", resp2.StatusCode)
	}
}

func TestCorrelationIDHeader(t *testing.T) {
	ts := newHTTPTestServer(t)

	// Sin header entrante: el server genera uno.
	resp, _ := postMCPRaw(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if got := resp.Header.Get(headerRequestID); got == "" {
		t.Error("esperaba un X-Request-Id generado en la respuesta")
	}
	resp.Body.Close()

	// Con header entrante: se propaga (echo) tal cual.
	resp2, _ := postMCPRaw(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "corr-123")
	if got := resp2.Header.Get(headerRequestID); got != "corr-123" {
		t.Errorf("X-Request-Id entrante no se propagó: got %q, esperaba corr-123", got)
	}
	resp2.Body.Close()
}

// postMCPRaw hace un POST a /mcp con un X-Request-Id opcional y devuelve la respuesta cruda.
func postMCPRaw(t *testing.T, baseURL, body, reqID string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, baseURL+mcpHTTPPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if reqID != "" {
		req.Header.Set(headerRequestID, reqID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	return resp, err
}
