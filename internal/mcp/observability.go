package mcp

// Observabilidad del modo servicio (Track 4 / T4.4): health/readiness, métricas y
// correlation IDs. Todo stdlib + el uuid ya presente; cero dependencias nuevas.
//   - GET /healthz  -> liveness (200 si el proceso responde).
//   - GET /readyz   -> readiness (200 si el motor/DB responde; 503 si no).
//   - GET /metrics  -> contadores en formato texto Prometheus (auth si hay token).
// Cada request al MCP recibe un correlation ID (header X-Request-Id: el entrante si
// viene, o uno nuevo) que se devuelve en la respuesta.

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/google/uuid"
)

const headerRequestID = "X-Request-Id"

// httpMetrics son contadores en memoria del endpoint MCP, clasificados por resultado.
// Se exponen en /metrics. Lock-free (atomic) para no contender bajo carga.
type httpMetrics struct {
	ok           atomic.Int64
	clientError  atomic.Int64 // respuestas 4xx (incl. 401)
	unauthorized atomic.Int64 // subconjunto 401, útil para detectar fuerza bruta
	serverError  atomic.Int64 // respuestas 5xx
}

func (m *httpMetrics) record(status int) {
	switch {
	case status == http.StatusUnauthorized:
		m.unauthorized.Add(1)
		m.clientError.Add(1)
	case status >= 500:
		m.serverError.Add(1)
	case status >= 400:
		m.clientError.Add(1)
	default:
		m.ok.Add(1)
	}
}

// render escribe los contadores en formato de exposición texto de Prometheus.
func (m *httpMetrics) render() string {
	return "# HELP musubi_http_requests_total Requests al endpoint MCP por resultado.\n" +
		"# TYPE musubi_http_requests_total counter\n" +
		fmt.Sprintf("musubi_http_requests_total{result=\"ok\"} %d\n", m.ok.Load()) +
		fmt.Sprintf("musubi_http_requests_total{result=\"client_error\"} %d\n", m.clientError.Load()) +
		fmt.Sprintf("musubi_http_requests_total{result=\"unauthorized\"} %d\n", m.unauthorized.Load()) +
		fmt.Sprintf("musubi_http_requests_total{result=\"server_error\"} %d\n", m.serverError.Load())
}

// statusRecorder envuelve un ResponseWriter para capturar el código de estado emitido
// (necesario para clasificar la métrica). Default 200 si el handler no llama WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// withObservability envuelve el handler del MCP: asigna/propaga el correlation ID y
// registra la métrica por resultado. health/readyz/metrics no se envuelven (no son
// tráfico de aplicación).
func withObservability(m *httpMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(headerRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set(headerRequestID, rid)
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		m.record(sr.status)
	})
}

// healthzHandler responde liveness: el proceso está vivo y sirviendo.
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}

// readyzHandler responde readiness: sondea el motor con una lectura barata (GetMeta).
// 503 si el backend no responde, para que un orquestador no rutee tráfico todavía.
func (s *McpServer) readyzHandler(w http.ResponseWriter, _ *http.Request) {
	if _, _, err := s.engine.GetMeta("__readyz_probe__"); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}` + "\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ready"}` + "\n"))
}
