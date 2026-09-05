package mcp

// fleet_otlp.go EMPUJA la telemetría de la flota a un receptor OTLP. Track «Control de flota», S11.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA TESIS, EN UNA LÍNEA: el empuje NO es un segundo camino de export. Es el MISMO camino con
// otra boca. Comparte con /metrics la selección de máquinas (devicesVisiblesParaMetricas), la
// tabla de series (seriesDeFlota) y el juego de labels (labelsDeFlota); lo único suyo es el sobre
// JSON, el POST y el lazo. Si este archivo tuviera su propio `for` sobre ListarDevices o su propia
// copia de la tabla, el diseño ya estaría mal: dos exportadores discrepan, y la discrepancia se
// descubre semanas después, cuando dos dashboards muestran cosas distintas.
//
// EL RIESGO NÚMERO UNO DE ESTE SLICE, ESCRITO ARRIBA DE TODO PORQUE COMPILA SIN QUEJARSE
//
// Un lazo de fondo no tiene request, así que no hay `principalFrom(ctx)` que valga: el empujador
// NACE SIN PRINCIPAL. Y con principal nil, proyectosVisibles marca federado=true y
// PuedeSobreDevice devuelve true incondicionalmente (es la confianza del stdio local) — o sea que
// un empujador descuidado exportaría la telemetría de TODOS los tenants a un endpoint externo,
// sin romper ninguna prueba existente y sin una línea de log.
//
// Por eso el empuje EXIGE un principal nombrado en la configuración, armarPayloadOTLP RECHAZA el
// nil en vez de tratarlo como «ve todo», y el servidor NO ARRANCA si ese principal no existe o no
// tiene ninguna concesión `metrics`. La confianza local vale para quien está sentado en la
// máquina; no para un lazo que manda datos afuera cada 30 segundos.
//
// CERO DEPENDENCIAS: el OTLP/JSON se emite a mano con encoding/json, igual que la traza de
// internal/memory/otel.go. Un SDK de OpenTelemetry para emitir 19 gauges sería una dependencia
// con transitivas para lo que entra en cien líneas de structs.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/fleet"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// nombreScopeOTLP e instanciaDelEmpuje son CONTRATO CON EL DESTINO, no perillas: por eso viven
// acá y no en la config. `service.instance.id` es además la marca que distingue lo empujado de lo
// scrapeado cuando el mismo Prometheus recibe las dos cosas (ver deploy/prometheus/prometheus.yml).
const (
	nombreScopeOTLP    = "musubi/fleet"
	instanciaDelEmpuje = "musubi-otlp-push"
)

// errEmpujeTransitorio y errEmpujePermanente clasifican el fallo del POST, con el mismo criterio
// que el cliente de sync (429/5xx se reintentan, el resto no). Acá no hay outbox ni backoff: el
// reintento es el próximo tick. Lo que cambia con la clasificación es EL LOG — un fallo permanente
// es un ESTADO que dura hasta que alguien toca una configuración, así que se avisa UNA vez; uno
// transitorio es un evento y se cuenta.
var (
	errEmpujeTransitorio = errors.New("fallo transitorio del empuje OTLP")
	errEmpujePermanente  = errors.New("fallo permanente del empuje OTLP")
)

// ── El sobre OTLP/JSON (subconjunto de metrics: sólo gauges) ────────────────────────────────

type otlpMetricsDoc struct {
	ResourceMetrics []otlpResourceMetrics `json:"resourceMetrics"`
}

type otlpResourceMetrics struct {
	Resource     otlpRecurso        `json:"resource"`
	ScopeMetrics []otlpScopeMetrics `json:"scopeMetrics"`
}

type otlpRecurso struct {
	Attributes []otlpAtributo `json:"attributes,omitempty"`
}

type otlpScopeMetrics struct {
	Scope   otlpAmbito   `json:"scope"`
	Metrics []otlpMetric `json:"metrics"`
}

type otlpAmbito struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type otlpMetric struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Gauge       otlpGauge `json:"gauge"`
}

type otlpGauge struct {
	DataPoints []otlpDataPoint `json:"dataPoints"`
}

// otlpDataPoint es UN punto. Las dos decisiones de forma que parecen menores y no lo son:
//
//   - AsDouble y AsInt son PUNTEROS. Con `float64` y omitempty, un `up 0` legítimo perdería su
//     campo y llegaría un punto SIN VALOR: exactamente el cero fantasma que este export existe
//     para no emitir, pero al revés. Con puntero, el 0 se escribe `"asDouble":0`.
//   - AsInt viaja como STRING, y TimeUnixNano también. Es la convención de OTLP/JSON para los
//     int64 —JSON no tiene enteros de 64 bits exactos— y es la misma que ya usa otlpVal.IntValue
//     en internal/memory/otel.go. Mandar timeUnixNano como número hace que Prometheus responda
//     400 y el empuje muera en silencio con la configuración perfecta.
type otlpDataPoint struct {
	Attributes   []otlpAtributo `json:"attributes,omitempty"`
	TimeUnixNano string         `json:"timeUnixNano"`
	AsDouble     *float64       `json:"asDouble,omitempty"`
	AsInt        *string        `json:"asInt,omitempty"`
}

type otlpAtributo struct {
	Key   string      `json:"key"`
	Value otlpValorKV `json:"value"`
}

type otlpValorKV struct {
	StringValue *string `json:"stringValue,omitempty"`
}

// atributoStr arma un atributo de texto. Todos los del empuje lo son: los cuatro labels de flota
// son texto y no hay ninguno más.
func atributoStr(clave, valor string) otlpAtributo {
	v := valor
	return otlpAtributo{Key: clave, Value: otlpValorKV{StringValue: &v}}
}

// atributosOTLP traduce labelsDeFlota a los attributes del punto. NO AGREGA NINGUNO: son los
// cuatro de siempre, en el mismo orden y con los mismos nombres que el exposition format. Un
// atributo de más (la versión del agente, la dirección) sería una etiqueta que la propia máquina
// controla, y con ella la cardinalidad de la serie deja de estar acotada por el cerebro.
func atributosOTLP(d fleet.Device) []otlpAtributo {
	labels := labelsDeFlota(d)
	out := make([]otlpAtributo, 0, len(labels))
	for _, kv := range labels {
		out = append(out, atributoStr(kv[0], kv[1]))
	}
	return out
}

// armarPayloadOTLP construye el sobre con lo que `p` —y sólo `p`— puede ver.
//
// `ahora` es UN SOLO RELOJ: el mismo con el que se decide `up` es el que sella todos los puntos.
// Con dos llamadas a time.Now() el sello y la decisión se separan por lo que tarde el barrido, y
// una máquina puede quedar marcada viva con un punto que dice otra cosa.
//
// Devuelve cuerpo nil (sin error) cuando no hay NADA que mandar: un sobre con cero métricas es un
// POST que no dice nada, y el gauge musubi_push_datapoints en 0 ya cuenta esa historia.
// atributosDeServicioOTLP son los mismos labels que usa el scrape, en el sobre de OTLP.
//
// Sale de labelsDeServicio y no de una segunda lista: dos juegos de labels para el mismo dato
// discrepan el día que alguien agrega uno, y la discrepancia se descubre semanas después cuando
// una consulta que cruza las dos salidas devuelve vacío.
func atributosDeServicioOTLP(sv fleet.Servicio, d fleet.Device) []otlpAtributo {
	kvs := labelsDeServicio(sv, d)
	out := make([]otlpAtributo, 0, len(kvs))
	for _, kv := range kvs {
		out = append(out, atributoStr(kv[0], kv[1]))
	}
	return out
}

func armarPayloadOTLP(engine memory.StorageBackend, p *Principal, ahora time.Time,
	intervaloSonda time.Duration, versionCerebro string) (cuerpo []byte, puntos int, truncado bool, err error) {

	// EL RECHAZO DEL nil ES EL INVARIANTE CENTRAL DEL ARCHIVO (ver el encabezado). No se degrada
	// a «ve todo» como el stdio local: acá nadie está sentado en la máquina, y lo que hay del otro
	// lado es una salida de datos hacia afuera.
	if p == nil {
		return nil, 0, false, fmt.Errorf("el empuje OTLP no tiene principal: exportaría la telemetría de TODOS los proyectos. Declará `fleet.otlp.principal` en el config y ese principal en principals.yaml con `fleet: {metrics: [\"*\"]}`")
	}

	vistos, truncado := devicesVisiblesParaMetricas(engine, p)
	// Mismo criterio que el scrape: un error leyendo las ventanas no apaga las alertas de nadie.
	enMantenimiento, errMant := engine.DevicesEnMantenimiento(ahora)
	if errMant != nil {
		enMantenimiento = nil
	}
	sello := strconv.FormatInt(ahora.UnixNano(), 10)

	var metricas []otlpMetric
	for _, serie := range seriesDeFlota(ahora, intervaloSonda, versionCerebro, enMantenimiento) {
		var datos []otlpDataPoint
		for _, d := range vistos {
			v, ok := serie.Valor(d, d.UltimaMuestra)
			if !ok {
				// LA MISMA REGLA CENTRAL QUE EL SCRAPE: lo desconocido NO viaja como 0. En el
				// exposition format se omite la línea; acá se omite el punto.
				continue
			}
			punto := otlpDataPoint{Attributes: atributosOTLP(d), TimeUnixNano: sello}
			if serie.Entera {
				n := strconv.FormatInt(int64(v), 10)
				punto.AsInt = &n
			} else {
				valor := v
				punto.AsDouble = &valor
			}
			datos = append(datos, punto)
		}
		if len(datos) == 0 {
			// Una métrica sin ningún punto no se emite, igual que /metrics no emite HELP/TYPE sin
			// cuerpo: un nombre de serie sin datos es ruido que el receptor igual indexa.
			continue
		}
		puntos += len(datos)
		metricas = append(metricas, otlpMetric{
			Name: serie.Nombre, Description: serie.Ayuda, Unit: serie.Unidad,
			Gauge: otlpGauge{DataPoints: datos},
		})
	}

	// QUÉ CORRE ADENTRO de esas máquinas (A43). Mismas máquinas ya compuertadas, mismo sello de
	// tiempo: un empuje con dos relojes deja las series de servicio desalineadas de las de la
	// máquina donde corren, y cualquier consulta que las cruce da vacío.
	svs, truncadoSvs := serviciosVisiblesParaMetricas(engine, vistos)
	truncado = truncado || truncadoSvs
	for _, serie := range seriesDeServicio() {
		var datos []otlpDataPoint
		for _, e := range svs {
			v, ok := serie.Valor(e.sv, ahora)
			if !ok {
				continue
			}
			valor := v
			datos = append(datos, otlpDataPoint{
				Attributes:   atributosDeServicioOTLP(e.sv, e.d),
				TimeUnixNano: sello,
				AsDouble:     &valor,
			})
		}
		if len(datos) == 0 {
			continue
		}
		puntos += len(datos)
		metricas = append(metricas, otlpMetric{
			Name: serie.Nombre, Description: serie.Ayuda, Unit: serie.Unidad,
			Gauge: otlpGauge{DataPoints: datos},
		})
	}
	// EL TRUNCADO **NO** VIAJA POR ACÁ, Y ES DELIBERADO.
	//
	// La primera versión lo metía en este payload y rompió cuatro pruebas de una vez, todas
	// diciendo lo mismo: este sobre lleva TELEMETRÍA DE MÁQUINAS, con exactamente cuatro
	// atributos, y nada que no sea de una máquina. Tenían razón — `musubi_fleet_export_truncated`
	// no es un hecho de ninguna máquina, es del exportador: una métrica del cerebro, como
	// `musubi_tool_calls_total`.
	//
	// Y llega igual a Prometheus, porque el drop del scrape es `musubi_fleet_(device|service)_.*`
	// y esta serie no matchea: sale por /metrics como el resto de las métricas del cerebro. Por
	// eso su alerta vive en el grupo `musubi-brain` y no en el de flota.
	if len(metricas) == 0 {
		return nil, 0, truncado, nil
	}

	doc := otlpMetricsDoc{ResourceMetrics: []otlpResourceMetrics{{
		Resource: otlpRecurso{Attributes: []otlpAtributo{
			atributoStr("service.name", "musubi"),
			atributoStr("service.instance.id", instanciaDelEmpuje),
		}},
		ScopeMetrics: []otlpScopeMetrics{{
			Scope:   otlpAmbito{Name: nombreScopeOTLP},
			Metrics: metricas,
		}},
	}}}
	cuerpo, err = json.Marshal(doc)
	if err != nil {
		return nil, 0, truncado, fmt.Errorf("no se pudo serializar el payload OTLP: %w", err)
	}
	return cuerpo, puntos, truncado, nil
}

// ── El cliente ──────────────────────────────────────────────────────────────────────────────

// empujadorOTLP es el cliente saliente. Se construye UNA vez en el arranque, con su Timeout
// explícito: un http.Client sin Timeout espera para siempre, y este POST sale del proceso que
// atiende toda la memoria del equipo.
type empujadorOTLP struct {
	url   string
	token string
	http  *http.Client
}

// nuevoEmpujadorOTLP valida el destino y resuelve el secreto. Rechaza AL ARRANQUE, que es la hora
// barata de enterarse:
//
//   - un endpoint que no es http(s), o sin host;
//   - un http:// que no es loopback sin allow_insecure_token: ahí el bearer viaja en texto plano;
//   - una URL CON USERINFO (http://user:clave@host). El secreto entra por auth_token_env y nunca
//     por la URL, porque una URL termina en un log de diagnóstico el primer día malo;
//   - un auth_token_env declarado que apunta a una variable VACÍA: si no, el empuje sale sin
//     credencial y el destino responde 401 para siempre, con la configuración «puesta».
//
// El token se resuelve ACÁ y se guarda el VALOR, no el nombre: el mismo criterio que NewSyncClient.
func nuevoEmpujadorOTLP(cfg config.OTLPPushConfig) (*empujadorOTLP, error) {
	crudo := strings.TrimSpace(cfg.Endpoint)
	if crudo == "" {
		return nil, fmt.Errorf("fleet.otlp.endpoint está vacío: sin destino no hay nada que empujar")
	}
	u, err := url.Parse(crudo)
	if err != nil {
		return nil, fmt.Errorf("fleet.otlp.endpoint %q no es una URL válida: %v", crudo, err)
	}
	esquema := strings.ToLower(u.Scheme)
	if esquema != "http" && esquema != "https" {
		return nil, fmt.Errorf("fleet.otlp.endpoint tiene esquema %q: sólo http o https. Para Prometheus: http://127.0.0.1:9099/api/v1/otlp/v1/metrics", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("fleet.otlp.endpoint %q no tiene host", crudo)
	}
	if u.User != nil {
		return nil, fmt.Errorf("fleet.otlp.endpoint lleva usuario/contraseña en la URL: sacálos y poné el bearer en la variable de entorno que nombre `auth_token_env`. Una URL con credencial termina en un log de diagnóstico el primer día malo")
	}
	if esquema == "http" && !esLoopback(u.Hostname()) && !cfg.AllowInsecureToken {
		return nil, fmt.Errorf("fleet.otlp.endpoint %q es http:// y no es loopback: el bearer viajaría en texto plano por la red. Usá https, o seteá fleet.otlp.allow_insecure_token: true si el destino está detrás de un túnel que ya cifra", crudo)
	}
	token := ""
	if env := strings.TrimSpace(cfg.AuthTokenEnv); env != "" {
		token = os.Getenv(env)
		if token == "" {
			return nil, fmt.Errorf("fleet.otlp.auth_token_env nombra a %q y esa variable está vacía: el empuje saldría sin credencial y el destino contestaría 401 para siempre. Exportála, o sacá auth_token_env si el destino no autentica", env)
		}
	}
	return &empujadorOTLP{
		url:   u.String(),
		token: token,
		http:  &http.Client{Timeout: cfg.EffectiveTimeout()},
	}, nil
}

// esLoopback dice si el host es la propia máquina. `localhost` entra por nombre porque es lo que
// la gente escribe; el resto se resuelve como IP y no por DNS: preguntarle al resolver si un
// nombre apunta a loopback es una decisión de seguridad que depende de quién conteste.
// urlSinSecretos deja la URL mostrable: esquema, host y path, y el query string TAPADO ENTERO.
//
// La guarda de arriba rechaza el `usuario:clave@host` de la URL, y eso cubría el caso obvio. No
// cubre el otro, que es más común en receptores OTLP reales: el token como parámetro
// (`?api-key=...`). Ese endpoint es legítimo —hay servicios que sólo aceptan así— y por eso NO se
// rechaza; lo que no puede pasar es que la primera línea del journal del cerebro lo publique.
//
// Se tapa el query ENTERO y no sólo las claves que suenan a secreto: una lista de nombres
// sospechosos siempre le erra a la siguiente, y del otro lado de este log no hay nadie que
// necesite ver los parámetros.
func urlSinSecretos(crudo string) string {
	u, err := url.Parse(crudo)
	if err != nil {
		// Si no parsea, no se arriesga a mostrar la mitad de algo que no se entendió.
		return "(url ilegible)"
	}
	u.User = nil
	if u.RawQuery != "" {
		u.RawQuery = "[oculto]"
	}
	return u.String()
}

func esLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// enviar hace el POST y clasifica la respuesta.
//
// NUNCA LOGUEA NI DEVUELVE el header Authorization ni el CUERPO de la respuesta del destino. Lo
// segundo no es paranoia de manual: un proxy mal configurado en el medio puede devolver el request
// que le mandaron, y ese request lleva el bearer. El cuerpo se DRENA (para poder reusar la
// conexión) y se descarta sin mirarlo.
func (e *empujadorOTLP) enviar(ctx context.Context, cuerpo []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(cuerpo))
	if err != nil {
		return fmt.Errorf("%w: no se pudo construir el POST al receptor OTLP: %v", errEmpujePermanente, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: el receptor OTLP no contestó: %v", errEmpujeTransitorio, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		// El 404 tiene mensaje propio porque es EL error de este slice: Prometheus no acepta OTLP
		// por defecto, y sin el flag el POST devuelve 404 con la configuración del cerebro perfecta.
		return fmt.Errorf("%w: el receptor devolvió 404. Prometheus NO acepta OTLP por defecto: tiene que correr con --web.enable-otlp-receiver, y el path del endpoint tiene que ser /api/v1/otlp/v1/metrics", errEmpujePermanente)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: el receptor rechazó la credencial (HTTP %d). Revisá la variable que nombra fleet.otlp.auth_token_env", errEmpujePermanente, resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: el receptor devolvió HTTP %d; se reintenta en el próximo tick", errEmpujeTransitorio, resp.StatusCode)
	default:
		return fmt.Errorf("%w: el receptor devolvió HTTP %d", errEmpujePermanente, resp.StatusCode)
	}
}

// ── El lazo ─────────────────────────────────────────────────────────────────────────────────

// RunEmpujeOTLP corre el empuje en su PROPIO ticker hasta que ctx se cancela. Pensada para su
// propia goroutine; bloquea hasta la cancelación. No-op si el empuje está apagado.
//
// TICKER PROPIO Y NO COLGADO DE RunFlotaScheduler: el intervalo de sondeo (5 min) gobierna el
// gasto de SSH y de él se deriva el umbral de «caído»; la cadencia del export es la del scrape
// (30 s). Atarlos repite el error que scheduler_flota.go documenta con el mantenimiento de la
// memoria — un número que alguien cambia por una razón y que apaga otra cosa en otro subsistema.
func (s *McpServer) RunEmpujeOTLP(ctx context.Context) {
	intervalo := s.empujeCfg.EffectiveInterval()
	if s.empujador == nil || !s.empujeCfg.Activo() || intervalo <= 0 {
		return
	}
	logx.Info("flota: empuje OTLP activo", "destino", urlSinSecretos(s.empujador.url),
		"cada", intervalo.String(), "principal", s.empujeCfg.Principal)
	t := time.NewTicker(intervalo)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.empujarUnaVez(ctx, time.Now())
		}
	}
}

// empujarUnaVez arma y manda UN payload.
//
// NO DEVUELVE ERROR A NADIE, y eso es la decisión: que Prometheus se caiga es un problema de
// monitoreo, no del plano de memoria. Cuenta, avisa y sigue. Tampoco toma dispatchMu ni ningún
// candado del servidor: un cerebro que se cuelga porque el sistema de métricas no contesta es
// peor que no tener métricas.
func (s *McpServer) empujarUnaVez(ctx context.Context, ahora time.Time) {
	if s.empujador == nil {
		return
	}
	// UN EMPUJE EN VUELO. Un destino más lento que el tick acumularía goroutines y payloads en
	// memoria en un proceso que vive días. Mismo patrón que flotaBusy.
	if !s.empujeBusy.CompareAndSwap(false, true) {
		logx.Warn("empuje OTLP: el anterior sigue en vuelo; se saltea este tick (¿el destino tarda más que el intervalo?)")
		return
	}
	defer s.empujeBusy.Store(false)

	// El principal se resuelve EN CADA TICK contra el snapshot vigente del registro, igual que las
	// políticas: guardarlo resuelto al arrancar dejaría al empujador exportando en nombre de una
	// credencial ya revocada. Y una política fantasma queda inerte (falla cerrada), pero un
	// empujador fantasma SIGUE MANDANDO DATOS.
	p, ok := s.principalDelEmpuje()
	if !ok {
		s.avisarUnaVez("empuje_sin_principal", func() {
			logx.Warn("empuje OTLP: el principal ya no está en principals.yaml; no se empuja nada (no se repite este aviso hasta que se resuelva)",
				"principal", s.empujeCfg.Principal)
		})
		s.empujeDatapoints.Store(0)
		s.empujeFallos.Add(1)
		return
	}
	s.avisosDados.Delete("empuje_sin_principal")

	// LA MISMA COMPROBACIÓN QUE EL ARRANQUE, PERO EN CADA TICK (A50).
	//
	// validarPrincipalDeEmpuje exige la concesión `metrics` y se NIEGA A ARRANCAR sin ella. Pero
	// principals.yaml se recarga en caliente cada 10 s, así que la concesión se puede ir DESPUÉS
	// de un arranque perfectamente válido — y ahí el empuje seguía corriendo, armando un payload
	// vacío y volviéndose por el `len(cuerpo) == 0` de más abajo SIN DEJAR RASTRO. La alerta
	// terminaba disparando (MusubiPushOTLPMudo, a los diez minutos, porque last_success envejece),
	// pero el log no decía por qué, y la causa —una línea que alguien sacó de un YAML— es
	// justamente la que no se deduce mirando el empuje.
	//
	// NO CUENTA UN FALLO: `musubi_push_failures_total` significa «no llegó a destino», y acá no se
	// intentó llegar. Ensuciar ese contador rompería MusubiPushOTLPNuncaLlego, que distingue «se
	// cayó» de «nunca anduvo» justamente por él.
	if len(p.Fleet[fleet.CapMetrics]) == 0 {
		s.avisarUnaVez("empuje_sin_concesion", func() {
			logx.Error("empuje OTLP: el principal existe pero ya NO tiene ninguna concesión `metrics` en su sección `fleet:`; no se exporta ni una máquina (no se repite este aviso hasta que se resuelva)",
				"principal", s.empujeCfg.Principal,
				"arreglo", "devolvele `fleet: {metrics: [\"*\"]}` en principals.yaml; las capacidades de flota NO se derivan del rol")
		})
		s.empujeDatapoints.Store(0)
		return
	}
	s.avisosDados.Delete("empuje_sin_concesion")

	cuerpo, puntos, truncado, err := armarPayloadOTLP(s.engine, p, ahora, s.sondaIntervalo, s.version)
	if err != nil {
		logx.Error("empuje OTLP: no se pudo armar el payload", "error", err)
		s.empujeDatapoints.Store(0)
		s.empujeFallos.Add(1)
		return
	}
	if truncado {
		// El push no tiene dónde poner un comentario que el parser ignore —el scrape sí—, así que
		// el aviso va al log. Una vez: es un estado, no un evento.
		s.avisarUnaVez("empuje_truncado", func() {
			logx.Warn("empuje OTLP: se barrieron los primeros proyectos y hay más; la telemetría del resto no se está empujando",
				"proyectos", proyectosParaExportar)
		})
	}
	s.empujeDatapoints.Store(int64(puntos))
	if len(cuerpo) == 0 {
		// Nada visible para ese principal: no se manda un sobre vacío. Pero SE DICE (A50). Este
		// es el tercer modo de quedarse mudo y el más difícil de ver de los tres: el principal
		// existe y tiene su concesión, sólo que apunta a proyectos donde no hay ni una máquina
		// —alguien renombró el proyecto, o el barrido todavía no vio a nadie—. Desde afuera es
		// idéntico a los otros dos: cero puntos, cero fallos, silencio.
		s.avisarUnaVez("empuje_vacio", func() {
			logx.Warn("empuje OTLP: el principal tiene concesión `metrics` pero no alcanza a NINGUNA máquina; no se manda un sobre vacío (no se repite este aviso hasta que vuelva a haber puntos)",
				"principal", s.empujeCfg.Principal,
				"concesion", strings.Join(p.Fleet[fleet.CapMetrics], ","))
		})
		return
	}
	s.avisosDados.Delete("empuje_vacio")

	if err := s.empujador.enviar(ctx, cuerpo); err != nil {
		s.empujeFallos.Add(1)
		if errors.Is(err, errEmpujePermanente) {
			// Un fallo permanente (404 sin el flag, 401 con el token mal) dura hasta que alguien
			// edite una configuración. Repetirlo cada 30 s son 2.880 líneas idénticas por día.
			s.avisarUnaVez("empuje_permanente:"+err.Error(), func() {
				logx.Error("empuje OTLP: fallo permanente (no se repite este aviso hasta que cambie)", "error", err)
			})
			return
		}
		logx.Warn("empuje OTLP: no se pudo entregar", "error", err)
		return
	}
	s.rearmarAvisos("empuje_permanente:")
	s.empujeUltimoExito.Store(ahora.Unix())
}

// principalDelEmpuje resuelve el principal configurado contra el registro VIGENTE.
func (s *McpServer) principalDelEmpuje() (*Principal, bool) {
	if s.buscarPrincipal == nil {
		return nil, false
	}
	return s.buscarPrincipal.porNombre(strings.TrimSpace(s.empujeCfg.Principal))
}

// rearmarAvisos vuelve a habilitar los avisos de una familia cuando la condición se resuelve. Sin
// esto, un 404 arreglado dejaría el aviso mudo para siempre y la próxima caída sería silenciosa.
func (s *McpServer) rearmarAvisos(prefijo string) {
	s.avisosDados.Range(func(k, _ any) bool {
		if clave, ok := k.(string); ok && strings.HasPrefix(clave, prefijo) {
			s.avisosDados.Delete(clave)
		}
		return true
	})
}
