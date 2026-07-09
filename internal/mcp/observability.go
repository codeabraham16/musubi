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
	"strings"
	"sync/atomic"
	"time"

	"musubi/internal/memory"

	"github.com/google/uuid"
)

const headerRequestID = "X-Request-Id"

// toolBuckets son los límites (en segundos) del histograma de latencia de tools/call. Cubren
// desde sub-milisegundo (recall léxico chico) hasta decenas de segundos (embedding + save o un
// mantenimiento pesado). Fijos y ordenados: el render los acumula en formato Prometheus.
const numToolBuckets = 12

var toolBuckets = [numToolBuckets]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// latencyHistogram es un histograma Prometheus MÍNIMO (buckets acumulativos + _sum + _count),
// lock-free. Cada observación cae en UN bucket (el menor cuyo límite no supera); el render
// acumula (le = "menor o igual"). La suma se guarda en microsegundos para no necesitar un
// atomic de float. Lo que excede el último límite se refleja en el bucket +Inf (== count).
type latencyHistogram struct {
	buckets   [numToolBuckets]atomic.Int64
	count     atomic.Int64
	sumMicros atomic.Int64
}

func (h *latencyHistogram) observe(d time.Duration) {
	h.count.Add(1)
	h.sumMicros.Add(d.Microseconds())
	sec := d.Seconds()
	for i := 0; i < numToolBuckets; i++ {
		if sec <= toolBuckets[i] {
			h.buckets[i].Add(1)
			return
		}
	}
	// Cae en +Inf: no incrementa ningún bucket finito (el render lo deriva de count).
}

// serverMetrics son los contadores/histogramas en memoria del servidor MCP, expuestos en
// /metrics. Lock-free (atomic) para no contender bajo carga. Incluye: resultado de requests
// HTTP, latencia + resultado de tools/call, y (al render) gauges de dominio del motor.
type serverMetrics struct {
	ok           atomic.Int64 // requests HTTP 2xx/3xx
	clientError  atomic.Int64 // respuestas 4xx (incl. 401)
	unauthorized atomic.Int64 // subconjunto 401, útil para detectar fuerza bruta
	serverError  atomic.Int64 // respuestas 5xx

	toolHist  latencyHistogram // latencia de cada tools/call (handler)
	toolOK    atomic.Int64     // tools/call que devolvieron resultado
	toolError atomic.Int64     // tools/call que devolvieron un RpcError
}

func (m *serverMetrics) record(status int) {
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

// recordTool registra la latencia y el resultado de un tools/call. ok=false ⇒ el handler
// devolvió un RpcError. Barato (atomics), seguro de llamar concurrentemente.
func (m *serverMetrics) recordTool(d time.Duration, ok bool) {
	m.toolHist.observe(d)
	if ok {
		m.toolOK.Add(1)
	} else {
		m.toolError.Add(1)
	}
}

// opStatsProvider lo implementa el motor real (*memory.DbEngine) para exponer gauges de
// dominio. Se type-asserta al render: si el backend no lo implementa (fakes de test), los
// gauges se omiten y /metrics sigue sirviendo el resto.
type opStatsProvider interface {
	OperationalStats() (memory.OpStats, error)
}

// render escribe todas las métricas en formato de exposición texto de Prometheus. engine, si
// implementa opStatsProvider y responde sin error, agrega los gauges de dominio (tamaño de la
// base, estado del índice vectorial, profundidad del outbox de sync).
func (m *serverMetrics) render(engine memory.StorageBackend) string {
	var b strings.Builder

	b.WriteString("# HELP musubi_http_requests_total Requests al endpoint MCP por resultado.\n")
	b.WriteString("# TYPE musubi_http_requests_total counter\n")
	fmt.Fprintf(&b, "musubi_http_requests_total{result=\"ok\"} %d\n", m.ok.Load())
	fmt.Fprintf(&b, "musubi_http_requests_total{result=\"client_error\"} %d\n", m.clientError.Load())
	fmt.Fprintf(&b, "musubi_http_requests_total{result=\"unauthorized\"} %d\n", m.unauthorized.Load())
	fmt.Fprintf(&b, "musubi_http_requests_total{result=\"server_error\"} %d\n", m.serverError.Load())

	b.WriteString("# HELP musubi_tool_calls_total Invocaciones de tools/call por resultado.\n")
	b.WriteString("# TYPE musubi_tool_calls_total counter\n")
	fmt.Fprintf(&b, "musubi_tool_calls_total{result=\"ok\"} %d\n", m.toolOK.Load())
	fmt.Fprintf(&b, "musubi_tool_calls_total{result=\"error\"} %d\n", m.toolError.Load())

	b.WriteString("# HELP musubi_tool_duration_seconds Latencia de tools/call (handler).\n")
	b.WriteString("# TYPE musubi_tool_duration_seconds histogram\n")
	var cum int64
	for i := 0; i < numToolBuckets; i++ {
		cum += m.toolHist.buckets[i].Load()
		fmt.Fprintf(&b, "musubi_tool_duration_seconds_bucket{le=\"%g\"} %d\n", toolBuckets[i], cum)
	}
	total := m.toolHist.count.Load()
	fmt.Fprintf(&b, "musubi_tool_duration_seconds_bucket{le=\"+Inf\"} %d\n", total)
	fmt.Fprintf(&b, "musubi_tool_duration_seconds_sum %g\n", float64(m.toolHist.sumMicros.Load())/1e6)
	fmt.Fprintf(&b, "musubi_tool_duration_seconds_count %d\n", total)

	renderDomainGauges(&b, engine)
	return b.String()
}

// renderDomainGauges agrega los gauges de dominio si el motor los expone y responde OK.
// Best-effort: ante error se omiten (no rompe el scrape).
func renderDomainGauges(b *strings.Builder, engine memory.StorageBackend) {
	p, ok := engine.(opStatsProvider)
	if !ok {
		return
	}
	st, err := p.OperationalStats()
	if err != nil {
		return
	}
	trained := 0
	if st.VectorIndexTrained {
		trained = 1
	}
	b.WriteString("# HELP musubi_observations Observaciones visibles en la base.\n")
	b.WriteString("# TYPE musubi_observations gauge\n")
	fmt.Fprintf(b, "musubi_observations %d\n", st.Observations)
	b.WriteString("# HELP musubi_embeddings_active Observaciones visibles con embedding.\n")
	b.WriteString("# TYPE musubi_embeddings_active gauge\n")
	fmt.Fprintf(b, "musubi_embeddings_active %d\n", st.ActiveEmbeddings)
	b.WriteString("# HELP musubi_vector_index_size Vectores vivos en el índice IVF.\n")
	b.WriteString("# TYPE musubi_vector_index_size gauge\n")
	fmt.Fprintf(b, "musubi_vector_index_size %d\n", st.VectorIndexSize)
	b.WriteString("# HELP musubi_vector_index_trained 1 si el IVF tiene centroides (si no, recall = full-scan).\n")
	b.WriteString("# TYPE musubi_vector_index_trained gauge\n")
	fmt.Fprintf(b, "musubi_vector_index_trained %d\n", trained)
	b.WriteString("# HELP musubi_sync_outbox Filas del outbox de sync por estado.\n")
	b.WriteString("# TYPE musubi_sync_outbox gauge\n")
	fmt.Fprintf(b, "musubi_sync_outbox{state=\"pending\"} %d\n", st.OutboxPending)
	fmt.Fprintf(b, "musubi_sync_outbox{state=\"sent\"} %d\n", st.OutboxSent)
	fmt.Fprintf(b, "musubi_sync_outbox{state=\"dead\"} %d\n", st.OutboxDead)
	b.WriteString("# HELP musubi_sync_outbox_oldest_pending_age_seconds Antigüedad de la pendiente más vieja (atraso del sync).\n")
	b.WriteString("# TYPE musubi_sync_outbox_oldest_pending_age_seconds gauge\n")
	fmt.Fprintf(b, "musubi_sync_outbox_oldest_pending_age_seconds %d\n", st.OutboxOldestAgeSec)
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
func withObservability(m *serverMetrics, next http.Handler) http.Handler {
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
