package main

import (
	"encoding/json"
	"fmt"
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

	h := dashboardHandler(engine, 8000, "proyecto-demo", nil)
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

	h := dashboardHandler(engine, 0, "", nil)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("esperaba 200 en /, obtuve %d", rr.Code)
	}
	// El HTML es la cáscara del dashboard WebGL: incluye MUSUBI y carga el bundle
	// three.js. El fetch a /api/snapshot vive DENTRO del bundle, no en el HTML.
	if !strings.Contains(rr.Body.String(), "MUSUBI") || !strings.Contains(rr.Body.String(), "/dashboard.bundle.js") {
		t.Error("el HTML servido debe ser la cáscara del dashboard (con MUSUBI y el <script> del bundle)")
	}

	// El bundle WebGL se sirve como JS y trae los DOS fetch del flujo nuevo.
	rrb := httptest.NewRecorder()
	h.ServeHTTP(rrb, httptest.NewRequest(http.MethodGet, "/dashboard.bundle.js", nil))
	if rrb.Code != http.StatusOK {
		t.Fatalf("esperaba 200 en /dashboard.bundle.js, obtuve %d", rrb.Code)
	}
	if ct := rrb.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("el bundle debe servirse como javascript, obtuve %q", ct)
	}
	// El contrato cambió A PROPÓSITO: el front dejó de sondear /api/snapshot —que arrastraba el
	// grafo entero cada 5 s y por eso obligaba a caparlo a 300 neuronas— y ahora pide el pulso,
	// más el grafo aparte y sólo cuando cambia. Este test declara el contrato NUEVO.
	if !strings.Contains(rrb.Body.String(), "/api/pulse") {
		t.Error("el bundle debe contener el fetch a /api/pulse (el sondeo liviano)")
	}
	if !strings.Contains(rrb.Body.String(), "/api/graph") {
		t.Error("el bundle debe contener el fetch a /api/graph (el grafo, aparte del sondeo)")
	}
	if strings.Contains(rrb.Body.String(), "/api/snapshot") {
		t.Error("el bundle NO debe sondear /api/snapshot: ese acoplamiento es el que obligaba al tope")
	}
	// La lente "código" hace hover-fetch del weld: su URL literal sobrevive a la minificación.
	if !strings.Contains(rrb.Body.String(), "/api/explained") {
		t.Error("el bundle debe contener el fetch a /api/explained (weld on-hover de la lente código)")
	}

	// Rutas desconocidas: 404 (no servir el HTML para cualquier path).
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/otra", nil))
	if rr2.Code != http.StatusNotFound {
		t.Errorf("una ruta desconocida debe dar 404, obtuve %d", rr2.Code)
	}
}

// TestDashboardCodeLens valida el backend de la lente "código": el snapshot trae el grafo de
// código y /api/explained devuelve las memorias que explican un símbolo (weld on-hover).
func TestDashboardCodeLens(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	nodes := []memory.GraphNode{
		{Key: "pkg/a.go#func:Hub", Kind: "func", Name: "Hub", Path: "pkg/a.go", SrcFingerprint: "1"},
		{Key: "pkg/a.go#func:Leaf", Kind: "func", Name: "Leaf", Path: "pkg/a.go", SrcFingerprint: "1"},
	}
	edges := []memory.GraphEdge{
		{FromKey: "pkg/a.go#func:Hub", ToKey: "pkg/a.go#func:Leaf", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "pkg/a.go", SrcFingerprint: "1"},
	}
	if err := engine.UpsertPackageGraph([]string{"pkg/a.go"}, nodes, edges); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservation("o1", "arq/hub", "Decisión sobre Hub en pkg/a.go: por rendimiento.", nil); err != nil {
		t.Fatal(err)
	}

	h := dashboardHandler(engine, 0, "demo", nil)

	// El snapshot trae el grafo de código.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	var snap exportSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("snapshot inválido: %v", err)
	}
	if len(snap.Code.Nodes) != 2 || len(snap.Code.Edges) != 1 {
		t.Errorf("el snapshot debería traer el grafo de código (2 nodos, 1 arista), got %d nodos / %d aristas", len(snap.Code.Nodes), len(snap.Code.Edges))
	}

	// El weld on-hover: /api/explained del símbolo devuelve la decisión que lo menciona.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/explained?symbol=pkg/a.go%23func:Hub", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("esperaba 200 en /api/explained, obtuve %d", rr2.Code)
	}
	var exp []memory.CodeExplain
	if err := json.Unmarshal(rr2.Body.Bytes(), &exp); err != nil {
		t.Fatalf("respuesta de /api/explained inválida: %v", err)
	}
	found := false
	for _, x := range exp {
		if x.TopicKey == "arq/hub" {
			found = true
		}
	}
	if !found {
		t.Errorf("/api/explained debería soldar la decisión arq/hub, got %+v", exp)
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

// TestDashboardGrafoSinTope: /api/graph trae el grafo ENTERO por defecto. Es la razón de ser
// del endpoint — el snapshot lo servía capado a 300 porque viajaba en cada sondeo, y con más
// de 300 memorias eso significaba que la pantalla nunca podía mostrar el acervo completo.
func TestDashboardGrafoSinTope(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	const total = 420 // por encima del default de 300, para que un tope se note
	for i := 0; i < total; i++ {
		if err := engine.SaveObservation(fmt.Sprintf("o%03d", i), "dom/x", "contenido", nil); err != nil {
			t.Fatal(err)
		}
	}
	h := dashboardHandler(engine, 8000, "demo", nil)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/graph?lens=memory", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rr.Code)
	}
	var g memory.BrainGraph
	if err := json.Unmarshal(rr.Body.Bytes(), &g); err != nil {
		t.Fatalf("no es JSON válido: %v", err)
	}
	if len(g.Neurons) != total || g.Truncated {
		t.Errorf("el grafo tiene que venir entero: %d de %d, truncated=%v", len(g.Neurons), total, g.Truncated)
	}

	// Y el ETag tiene que evitar retransmitirlo: es lo que hace barato re-pedirlo.
	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Fatal("falta el ETag: sin él el cliente re-baja el grafo entero cada vez")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/graph?lens=memory", nil)
	req.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusNotModified || rr2.Body.Len() != 0 {
		t.Errorf("con If-None-Match esperaba 304 y 0 bytes, obtuve %d con %d bytes", rr2.Code, rr2.Body.Len())
	}

	// Un limit explícito sigue capando: la superficie no pierde la capacidad de acotar.
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/graph?lens=memory&limit=50", nil))
	var g3 memory.BrainGraph
	if err := json.Unmarshal(rr3.Body.Bytes(), &g3); err != nil {
		t.Fatal(err)
	}
	if len(g3.Neurons) != 50 || !g3.Truncated || g3.TotalNeurons != total {
		t.Errorf("con limit=50 esperaba 50 de %d y truncated: obtuve %d de %d trunc=%v",
			total, len(g3.Neurons), g3.TotalNeurons, g3.Truncated)
	}
}

// TestDashboardPulseEsChico: el pulso es lo que se pide cada 5 s, así que su tamaño es su
// contrato. Con 420 memorias el snapshot arrastra el grafo entero y el pulso no.
func TestDashboardPulseEsChico(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	for i := 0; i < 420; i++ {
		if err := engine.SaveObservation(fmt.Sprintf("o%03d", i), "dom/x", "contenido de relleno para que pese", nil); err != nil {
			t.Fatal(err)
		}
	}
	h := dashboardHandler(engine, 8000, "demo", nil)

	rp := httptest.NewRecorder()
	h.ServeHTTP(rp, httptest.NewRequest(http.MethodGet, "/api/pulse", nil))
	if rp.Code != http.StatusOK {
		t.Fatalf("pulse: esperaba 200, obtuve %d", rp.Code)
	}
	rs := httptest.NewRecorder()
	h.ServeHTTP(rs, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))

	if rp.Body.Len() >= rs.Body.Len() {
		t.Errorf("el pulso tiene que ser MÁS CHICO que el snapshot: pulse=%d snapshot=%d",
			rp.Body.Len(), rs.Body.Len())
	}
	var p dashboardPulse
	if err := json.Unmarshal(rp.Body.Bytes(), &p); err != nil {
		t.Fatalf("el pulso no es JSON válido: %v", err)
	}
	if p.Counts.Neurons != 420 {
		t.Errorf("el pulso lleva el conteo REAL aunque no lleve el grafo: esperaba 420, obtuve %d", p.Counts.Neurons)
	}
	if p.GraphVersion == "" {
		t.Error("sin graph_version el cliente no puede saber cuándo re-bajar el grafo")
	}
	// Lo que NO tiene que llevar: el grafo. Si algún día se cuela, el sondeo vuelve a pesar.
	if strings.Contains(rp.Body.String(), `"neurons":[{`) {
		t.Error("el pulso NO debe llevar el array de neuronas: ese era el problema que resuelve")
	}
}
