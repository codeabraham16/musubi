package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// REGRESIÓN (auditoría 2026-07-26, #9): /metrics gateaba SÓLO por el token legacy (opt.token). En el
// setup multi-tenant recomendado (principals.yaml ⇒ registry, sin token legacy) opt.token quedaba ""
// y /metrics caía ABIERTO en el bind del tailnet, exponiendo uso por tool, profundidad de outbox, etc.
// Ahora exige un principal válido cuando hay registry.
func TestMetricsRequiresAuthWithRegistry(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone, hash: hashToken("good-token")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	defer ts.Close()

	// Sin credencial ⇒ 401 (antes daba 200 con el cuerpo de métricas).
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/metrics sin credencial debe dar 401, obtuve %d", resp.StatusCode)
	}

	// Con un token válido del registro ⇒ 200.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics con bearer: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/metrics con token válido debe dar 200, obtuve %d", resp2.StatusCode)
	}
}
