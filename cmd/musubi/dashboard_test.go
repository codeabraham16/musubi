package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"musubi/internal/memory"
)

func TestDashboardSnapshotEndpoint(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()
	if err := engine.SaveObservation("a", "roadmap/x", "contenido", nil); err != nil {
		t.Fatal(err)
	}

	h := dashboardHandler(engine, 8000, "proyecto-demo")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("esperaba Content-Type JSON, obtuve %q", ct)
	}
	var snap exportSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("el snapshot no es JSON válido: %v", err)
	}
	if snap.Insights.Observations.Active != 1 {
		t.Errorf("esperaba 1 observación activa, obtuve %d", snap.Insights.Observations.Active)
	}
	if snap.Health.Status == "" {
		t.Error("el snapshot debe incluir el estado de salud")
	}
	if snap.Project != "proyecto-demo" {
		t.Errorf("esperaba project=proyecto-demo, obtuve %q", snap.Project)
	}
}

func TestDashboardIndexServesHTML(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	h := dashboardHandler(engine, 0, "")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("esperaba 200 en /, obtuve %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "MUSUBI") || !strings.Contains(rr.Body.String(), "/api/snapshot") {
		t.Error("el HTML servido debe ser el dashboard (con MUSUBI y el fetch a /api/snapshot)")
	}

	// Rutas desconocidas: 404 (no servir el HTML para cualquier path).
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/otra", nil))
	if rr2.Code != http.StatusNotFound {
		t.Errorf("una ruta desconocida debe dar 404, obtuve %d", rr2.Code)
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	for _, ok := range []string{"127.0.0.1:7777", "localhost:80", "[::1]:9000", "127.0.0.5:1"} {
		if !isLoopbackAddr(ok) {
			t.Errorf("isLoopbackAddr(%q) debería ser true", ok)
		}
	}
	for _, bad := range []string{":7777", "0.0.0.0:7777", "192.168.1.5:80", "example.com:80", "noport"} {
		if isLoopbackAddr(bad) {
			t.Errorf("isLoopbackAddr(%q) debería ser false (no expone a la red)", bad)
		}
	}
}
