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
	"sort"
	"strings"
	"sync"
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

// toolStat son los contadores POR-TOOL (T17.5): volumen ok/error y latencia sumada, para ver qué
// tool concreta se llama más, cuál falla y cuál es la más lenta (el histograma global agrega todo).
// Lock-free; se crea perezosamente por nombre de tool en un sync.Map.
type toolStat struct {
	ok        atomic.Int64
	err       atomic.Int64
	count     atomic.Int64 // llamadas medidas (ok+error)
	sumMicros atomic.Int64 // latencia acumulada, en microsegundos (avg = sum/count)
}

// serverMetrics son los contadores/histogramas en memoria del servidor MCP, expuestos en
// /metrics. Lock-free (atomic) para no contender bajo carga. Incluye: resultado de requests
// HTTP, latencia + resultado de tools/call (agregado y POR-TOOL), rechazos de authz/cuota, y
// (al render, con cache) gauges de dominio del motor.
type serverMetrics struct {
	ok           atomic.Int64 // requests HTTP 2xx/3xx
	clientError  atomic.Int64 // respuestas 4xx (incl. 401)
	unauthorized atomic.Int64 // subconjunto 401, útil para detectar fuerza bruta
	serverError  atomic.Int64 // respuestas 5xx

	// DESGLOSE DE LOS 401 POR MOTIVO (A88). `unauthorized` solo dice «alguien falló» y con eso no
	// se puede decidir nada: un cliente propio al que le sacaron la variable de entorno y alguien
	// probando tokens producen el mismo número. Estos dos separan los dos mundos, y la cardinalidad
	// tiene techo (son dos series, no una por IP).
	authSinCredencial atomic.Int64 // request sin cabecera Authorization
	authDesconocida   atomic.Int64 // credencial presentada y no reconocida (o vencida)
	// authBloqueado cuenta los 429 del candado anti fuerza-bruta. Va aparte de los 401 porque
	// significa otra cosa: no es «tu credencial está mal», es «esta IP ya agotó sus intentos».
	authBloqueado atomic.Int64
	//
	// OJO AL COMPARAR CON DATOS ANTERIORES AL 2026-09-05: `unauthorized` cuenta RESPUESTAS 401, no
	// fallos de auth, y al mover el candado detrás del token (A88) el QUINTO fallo de cada ciclo
	// pasó de contestar 401 a contestar 429. O sea que `unauthorized` bajó ~20 % para el mismo
	// volumen de fallos, sin que nada mejorara. La serie continua y comparable de acá en adelante
	// es `musubi_auth_failures_total`, que cuenta el FALLO y no la respuesta: no le importa si el
	// candado ya estaba puesto. Cambiar un comportamiento y romper en silencio la serie que lo
	// medía es la forma más cara de arreglar algo.

	toolHist  latencyHistogram // latencia AGREGADA de cada tools/call (handler)
	toolOK    atomic.Int64     // tools/call que devolvieron resultado
	toolError atomic.Int64     // tools/call que devolvieron un RpcError
	toolStats sync.Map         // nombre de tool (string) -> *toolStat (desglose por-tool, T17.5)

	// Rechazos ANTES de ejecutar la tool (T17.5): antes eran invisibles en /metrics (la request
	// HTTP contaba como ok). Un pico de authz o quota es señal de abuso o de un cliente mal configurado.
	authzDenied   atomic.Int64 // tools/call negadas por rol (codeUnauthorized)
	quotaExceeded atomic.Int64 // tools/call negadas por cuota (codeQuotaExceeded)
	// motorDenied cuenta los frenazos del PRESUPUESTO DEL MOTOR, y va aparte de quotaExceeded
	// porque incluye un caso que no es un rechazo: cuando el recall se queda sin presupuesto,
	// DEGRADA al orden model-free y devuelve ok. Sin este contador, el sistema dejaría de usar el
	// juez sin que nadie pudiera enterarse.
	motorDenied atomic.Int64
	// execAllowDenied cuenta los exec frenados por la ALLOWLIST de la credencial (S10, I8-I10).
	// Va aparte de authzDenied porque significa otra cosa: authz es «no podés tocar esa máquina»
	// y esto es «podés, pero no ESE comando». Confundirlos haría que un token bien configurado
	// que choca contra su propia allowlist se lea como un intento de intrusión.
	execAllowDenied atomic.Int64
	// politicaStats cuenta las acciones del AUTO-HEAL por política y resultado (S10, I19).
	// Clave: "<politica>\x00<resultado>" -> *atomic.Int64.
	//
	// A PROPÓSITO SIN LA ETIQUETA DE MÁQUINA. El resto de las series de flota se filtran por la
	// credencial del scrape (ver renderFlota), pero las políticas son configuración del cerebro y
	// no cuelgan de ninguna concesión: etiquetar la máquina acá le entregaría el inventario de un
	// tenant a cualquier scraper. La alerta necesita saber QUE una política actúa en loop; CUÁL
	// máquina lo dice la bitácora, que sí está compuertada.
	politicaStats sync.Map

	gaugeCache domainGaugeCache // cache TTL de OperationalStats para no re-COUNT en cada scrape
}

// domainGaugeCache cachea el resultado de OperationalStats por un TTL corto (T17.5): sin esto, un
// Prometheus scrapeando /metrics cada pocos segundos re-ejecuta los COUNT O(n) en CADA scrape. El
// cache los amortiza a lo sumo uno por ventana; combinado con el deadline del engine, /metrics deja
// de poder martillar (o colgar por) la base.
type domainGaugeCache struct {
	mu    sync.Mutex
	val   memory.OpStats
	at    time.Time
	valid bool
}

// domainGaugeTTL es la ventana del cache de gauges de dominio. Corto: los conteos cambian lento y
// un pequeño desfase en /metrics es irrelevante para operar.
const domainGaugeTTL = 15 * time.Second

// get devuelve el valor cacheado si sigue fresco.
func (c *domainGaugeCache) get() (memory.OpStats, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valid && time.Since(c.at) < domainGaugeTTL {
		return c.val, true
	}
	return memory.OpStats{}, false
}

// put guarda el valor recién computado con su marca de tiempo.
func (c *domainGaugeCache) put(st memory.OpStats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.val, c.at, c.valid = st, time.Now(), true
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

// recordTool registra la latencia y el resultado de un tools/call, AGREGADO y por-tool (T17.5).
// ok=false ⇒ el handler devolvió un RpcError. Barato (atomics + un LoadOrStore la 1ª vez por tool),
// seguro de llamar concurrentemente.
func (m *serverMetrics) recordTool(tool string, d time.Duration, ok bool) {
	m.toolHist.observe(d)
	if ok {
		m.toolOK.Add(1)
	} else {
		m.toolError.Add(1)
	}

	v, _ := m.toolStats.LoadOrStore(tool, &toolStat{})
	ts := v.(*toolStat)
	ts.count.Add(1)
	ts.sumMicros.Add(d.Microseconds())
	if ok {
		ts.ok.Add(1)
	} else {
		ts.err.Add(1)
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

	b.WriteString("# HELP musubi_auth_failures_total Rechazos de autenticación por motivo. Sin etiqueta de IP a propósito: la atribución va al log (A88).\n")
	b.WriteString("# TYPE musubi_auth_failures_total counter\n")
	fmt.Fprintf(&b, "musubi_auth_failures_total{motivo=\"%s\"} %d\n", motivoSinCredencial, m.authSinCredencial.Load())
	fmt.Fprintf(&b, "musubi_auth_failures_total{motivo=\"%s\"} %d\n", motivoCredencialDesconocida, m.authDesconocida.Load())
	b.WriteString("# HELP musubi_auth_lockouts_total Requests rechazadas con 429 por el candado anti fuerza-bruta.\n")
	b.WriteString("# TYPE musubi_auth_lockouts_total counter\n")
	fmt.Fprintf(&b, "musubi_auth_lockouts_total %d\n", m.authBloqueado.Load())

	b.WriteString("# HELP musubi_tool_calls_total Invocaciones de tools/call por resultado (agregado).\n")
	b.WriteString("# TYPE musubi_tool_calls_total counter\n")
	fmt.Fprintf(&b, "musubi_tool_calls_total{result=\"ok\"} %d\n", m.toolOK.Load())
	fmt.Fprintf(&b, "musubi_tool_calls_total{result=\"error\"} %d\n", m.toolError.Load())

	b.WriteString("# HELP musubi_tool_duration_seconds Latencia de tools/call (handler, agregado).\n")
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

	m.renderToolBreakdown(&b)
	m.renderRejections(&b)
	m.renderDomainGauges(&b, engine)
	return b.String()
}

// renderToolBreakdown emite el desglose POR-TOOL (T17.5): volumen ok/error y latencia sumada+contada
// (avg = sum/count) por nombre de tool. Orden estable (alfabético) para un scrape determinista.
func (m *serverMetrics) renderToolBreakdown(b *strings.Builder) {
	type row struct {
		name string
		st   *toolStat
	}
	var rows []row
	m.toolStats.Range(func(k, v interface{}) bool {
		rows = append(rows, row{k.(string), v.(*toolStat)})
		return true
	})
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	b.WriteString("# HELP musubi_tool_invocations_total Invocaciones de tools/call por tool y resultado.\n")
	b.WriteString("# TYPE musubi_tool_invocations_total counter\n")
	for _, r := range rows {
		fmt.Fprintf(b, "musubi_tool_invocations_total{tool=%q,result=\"ok\"} %d\n", r.name, r.st.ok.Load())
		fmt.Fprintf(b, "musubi_tool_invocations_total{tool=%q,result=\"error\"} %d\n", r.name, r.st.err.Load())
	}
	b.WriteString("# HELP musubi_tool_latency_seconds_sum Latencia acumulada de tools/call por tool (avg = sum/count).\n")
	b.WriteString("# TYPE musubi_tool_latency_seconds_sum counter\n")
	for _, r := range rows {
		fmt.Fprintf(b, "musubi_tool_latency_seconds_sum{tool=%q} %g\n", r.name, float64(r.st.sumMicros.Load())/1e6)
	}
	b.WriteString("# HELP musubi_tool_latency_seconds_count Llamadas de tools/call medidas por tool.\n")
	b.WriteString("# TYPE musubi_tool_latency_seconds_count counter\n")
	for _, r := range rows {
		fmt.Fprintf(b, "musubi_tool_latency_seconds_count{tool=%q} %d\n", r.name, r.st.count.Load())
	}
}

// renderRejections emite los rechazos de tools/call ANTES de ejecutar (authz por rol / cuota),
// que antes no se veían en /metrics (T17.5).
func (m *serverMetrics) renderRejections(b *strings.Builder) {
	b.WriteString("# HELP musubi_tool_rejections_total Tools/call rechazadas antes de ejecutar, por razón.\n")
	b.WriteString("# TYPE musubi_tool_rejections_total counter\n")
	fmt.Fprintf(b, "musubi_tool_rejections_total{reason=\"authz\"} %d\n", m.authzDenied.Load())
	fmt.Fprintf(b, "musubi_tool_rejections_total{reason=\"quota\"} %d\n", m.quotaExceeded.Load())
	fmt.Fprintf(b, "musubi_tool_rejections_total{reason=\"motor_quota\"} %d\n", m.motorDenied.Load())
	fmt.Fprintf(b, "musubi_tool_rejections_total{reason=\"fleet_allowlist\"} %d\n", m.execAllowDenied.Load())
	m.renderPoliticas(b)
}

// contarPolitica anota una acción de auto-heal.
// resultado: "ok" | "rechazada" | "sin_principal" | "error" | "mantenimiento".
func (m *serverMetrics) contarPolitica(politica, resultado string) {
	if m == nil {
		return
	}
	clave := politica + "\x00" + resultado
	v, _ := m.politicaStats.LoadOrStore(clave, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

// sembrarPoliticas crea EN CERO las series de cada política configurada.
//
// EL CERO Y EL SILENCIO NO SON LO MISMO, y hasta acá el código no cumplía lo que su propio
// comentario prometía: `politicaStats` declaraba desde S10 que la serie se emite aunque valga
// cero «una vez que hay políticas configuradas», pero nada la sembraba — el mapa nacía vacío,
// `renderPoliticas` cortaba, y la serie no existía hasta la PRIMERA acción.
//
// El costo era exacto y medible: las dos alertas que viven de esta serie son `increase(...)`, y
// un `increase()` sobre una serie ausente no devuelve nada. O sea que no podían distinguir «no
// actuó ninguna política» de «el cerebro dejó de exportar» — que es la distinción entera por la
// que existe el comentario. Se vio al reiniciar el cerebro después de configurar la primera
// política real: la serie que acababa de aparecer desapareció, y ningún log lo dijo.
//
// Se siembran los CUATRO resultados posibles y no sólo los que hoy miran las alertas: una alerta
// nueva sobre `error` se encontraría con el mismo agujero, y sembrar de menos lo dejaría abierto
// para la próxima.
func (m *serverMetrics) sembrarPoliticas(nombres []string) {
	if m == nil {
		return
	}
	for _, n := range nombres {
		for _, r := range []string{"ok", "rechazada", "sin_principal", "error", "mantenimiento"} {
			// LoadOrStore y no Store: sembrar NUNCA puede pisar un contador que ya viene
			// contando, o una recarga de configuración borraría la historia de la ventana.
			m.politicaStats.LoadOrStore(n+"\x00"+r, new(atomic.Int64))
		}
	}
}

// renderPoliticas emite el contador de acciones automáticas.
//
// SE EMITE AUNQUE VALGA CERO una vez que hay políticas configuradas, porque el silencio y el cero
// no son lo mismo: una serie AUSENTE hace que `rate()` no devuelva nada y la alerta de I19 no
// pueda distinguir «no actuó» de «el cerebro dejó de exportar». Es la misma trampa que
// FlotaSinTelemetria cierra un nivel más arriba.
func (m *serverMetrics) renderPoliticas(b *strings.Builder) {
	type fila struct {
		politica, resultado string
		n                   int64
	}
	var filas []fila
	m.politicaStats.Range(func(k, v interface{}) bool {
		partes := strings.SplitN(k.(string), "\x00", 2)
		if len(partes) == 2 {
			filas = append(filas, fila{partes[0], partes[1], v.(*atomic.Int64).Load()})
		}
		return true
	})
	if len(filas) == 0 {
		return
	}
	sort.Slice(filas, func(i, j int) bool {
		if filas[i].politica != filas[j].politica {
			return filas[i].politica < filas[j].politica
		}
		return filas[i].resultado < filas[j].resultado
	})
	b.WriteString("# HELP musubi_fleet_policy_actions_total Acciones de política automática (auto-heal), por política y resultado.\n")
	b.WriteString("# TYPE musubi_fleet_policy_actions_total counter\n")
	for _, f := range filas {
		fmt.Fprintf(b, "musubi_fleet_policy_actions_total{policy=%q,result=%q} %d\n", f.politica, f.resultado, f.n)
	}
}

// renderDomainGauges agrega los gauges de dominio si el motor los expone y responde OK. Usa un
// cache TTL (T17.5) para no re-ejecutar los COUNT O(n) en cada scrape. Best-effort: ante error se
// omiten (no rompe el scrape) y no se cachea (para reintentar al próximo).
func (m *serverMetrics) renderDomainGauges(b *strings.Builder, engine memory.StorageBackend) {
	p, ok := engine.(opStatsProvider)
	if !ok {
		return
	}
	st, cached := m.gaugeCache.get()
	if !cached {
		var err error
		st, err = p.OperationalStats()
		if err != nil {
			return
		}
		m.gaugeCache.put(st)
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
	b.WriteString("# HELP musubi_backup_offhost_age_seconds Antigüedad del último backup off-host exitoso (-1 si nunca/no configurado).\n")
	b.WriteString("# TYPE musubi_backup_offhost_age_seconds gauge\n")
	fmt.Fprintf(b, "musubi_backup_offhost_age_seconds %d\n", st.BackupOffhostAgeSec)
	b.WriteString("# HELP musubi_backup_local_age_seconds Antigüedad del último snapshot LOCAL (-1 si nunca). Dice si el timer corre; el de off-host dice si el backup sale de la máquina.\n")
	b.WriteString("# TYPE musubi_backup_local_age_seconds gauge\n")
	fmt.Fprintf(b, "musubi_backup_local_age_seconds %d\n", st.BackupLocalAgeSec)
}

// renderEmpuje emite las TRES series de auto-vigilancia del empuje OTLP (S11).
//
// POR QUÉ SALEN POR EL TIRÓN Y NO VIAJAN EN EL PROPIO EMPUJE: un mecanismo de monitoreo cuya
// única forma de avisar de su propia muerte es él mismo no avisa nunca. Si el POST no llega, un
// contador de fallos que viajara adentro del POST tampoco llega. Es el mismo punto ciego que
// FlotaSinTelemetria cierra un nivel más abajo.
//
// Se emiten SÓLO si el empuje está configurado —nadie necesita tres series en cero en un cerebro
// que no empuja— y, una vez configurado, se emiten SIEMPRE aunque valgan cero: el silencio y el
// cero no son lo mismo, y una serie ausente hace que `rate()` no devuelva nada. Mismo criterio
// que renderPoliticas.
//
// La excepción es last_success, que se OMITE mientras nunca haya habido un empuje aceptado. Un 0
// ahí sería el unix epoch, o sea «último éxito: hace 56 años» — que se lee como un bug del panel
// y no como «esto nunca funcionó», que es lo que realmente pasó.
func (s *McpServer) renderEmpuje(b *strings.Builder, ahora time.Time) {
	if !s.empujeCfg.Activo() {
		return
	}
	if ultimo := s.empujeUltimoExito.Load(); ultimo > 0 {
		b.WriteString("# HELP musubi_push_last_success_seconds Segundos desde el último empuje OTLP aceptado. AUSENTE si nunca hubo uno.\n")
		b.WriteString("# TYPE musubi_push_last_success_seconds gauge\n")
		fmt.Fprintf(b, "musubi_push_last_success_seconds %d\n", int64(ahora.Sub(time.Unix(ultimo, 0)).Seconds()))
	}
	b.WriteString("# HELP musubi_push_failures_total Empujes OTLP que no llegaron a destino.\n")
	b.WriteString("# TYPE musubi_push_failures_total counter\n")
	fmt.Fprintf(b, "musubi_push_failures_total %d\n", s.empujeFallos.Load())
	b.WriteString("# HELP musubi_push_datapoints Puntos que llevó el último empuje. Un 0 sostenido = el empujador corre y no exporta nada.\n")
	b.WriteString("# TYPE musubi_push_datapoints gauge\n")
	fmt.Fprintf(b, "musubi_push_datapoints %d\n", s.empujeDatapoints.Load())
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
