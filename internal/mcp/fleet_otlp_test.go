package mcp

// Pruebas del slice S11: el EMPUJE OTLP de la telemetría de la flota.
//
// Todo lo de acá abajo existe para sostener una frase: el empuje es el MISMO export que /metrics
// con otra boca, y actúa con la autoridad de un principal NOMBRADO. Un lazo de fondo no tiene
// request, así que el nil está siempre a un descuido de distancia — y con nil, el barrido federa y
// la compuerta por máquina dice que sí a todo.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// ── Andamios ────────────────────────────────────────────────────────────────────────────────

// principalDePrometheus es el principal típico del empuje: ve todos los proyectos, mira toda la
// flota y no puede tocar nada. Es el que documenta deploy/prometheus/prometheus.yml.
func principalDePrometheus() Principal {
	return Principal{
		Name: "prometheus", Role: RoleReader, Read: ReadAll,
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
}

// maquinaConMuestra enrola una máquina y le deja una muestra, sin pasar por HTTP.
func maquinaConMuestra(t *testing.T, s *McpServer, proyecto, nombre string, m fleet.Muestra, cuando time.Time) fleet.Device {
	t.Helper()
	enrolarDePrueba(t, s, proyecto, nombre)
	d, ok, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil || !ok {
		t.Fatalf("no se pudo releer la máquina %q: %v", nombre, err)
	}
	latir(t, s, d.ID, m, cuando)
	d.LastSeen = cuando
	return d
}

// prepararEmpuje arma un servidor con el empuje configurado contra `destino` y el registro dado.
// `ajustar` puede tocar la config antes de validarla (nil = los defaults).
func prepararEmpuje(t *testing.T, destino string, reg *PrincipalRegistry, ajustar func(*config.OTLPPushConfig)) *McpServer {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	cfg := config.OTLPPushConfig{Endpoint: destino, Principal: "prometheus", IntervalSeconds: 30}
	if ajustar != nil {
		ajustar(&cfg)
	}
	if err := s.ConfigurarFlota(config.FleetConfig{OTLP: cfg}); err != nil {
		t.Fatalf("ConfigurarFlota con empuje: %v", err)
	}
	if err := s.vincularRegistroDeFlota(reg); err != nil {
		t.Fatalf("vincularRegistroDeFlota: %v", err)
	}
	return s
}

// receptorDePrueba es un destino OTLP que guarda lo que le llega.
type receptorDePrueba struct {
	*httptest.Server
	cuerpos  chan []byte
	pedidos  atomic.Int64
	estado   int
	ecoAuth  bool // responde con el header Authorization en el cuerpo (para probar que no se filtra)
	bloqueo  chan struct{}
	recibido chan struct{}

	// EL SOBRE, no sólo el contenido (A49). Hasta acá el receptor leía el cuerpo y contaba
	// requests, y nada más: el verificador borró el `Content-Type` de `enviar` y la suite entera
	// quedó en verde, con Prometheus contestando 400 a cada POST en producción.
	mu    sync.Mutex
	sobre sobreRecibido
}

// sobreRecibido es lo que el destino ve ANTES de mirar el cuerpo.
type sobreRecibido struct {
	Metodo  string
	Path    string
	Headers http.Header
}

func (r *receptorDePrueba) ultimoSobre(t *testing.T) sobreRecibido {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sobre.Metodo == "" {
		t.Fatal("el receptor no recibió ningún request")
	}
	return r.sobre
}

// nuevoReceptor levanta un destino que responde `estado` y acumula los cuerpos recibidos.
func nuevoReceptor(t *testing.T, estado int) *receptorDePrueba {
	t.Helper()
	r := &receptorDePrueba{cuerpos: make(chan []byte, 16), estado: estado}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.sobre = sobreRecibido{Metodo: req.Method, Path: req.URL.Path, Headers: req.Header.Clone()}
		r.mu.Unlock()
		crudo, _ := io.ReadAll(req.Body)
		r.pedidos.Add(1)
		select {
		case r.cuerpos <- crudo:
		default:
		}
		if r.recibido != nil {
			select {
			case r.recibido <- struct{}{}:
			default:
			}
		}
		if r.bloqueo != nil {
			<-r.bloqueo
		}
		w.WriteHeader(r.estado)
		if r.ecoAuth {
			// Un proxy mal configurado que devuelve el request es real: el cuerpo de la respuesta
			// del destino puede traer el bearer de vuelta.
			fmt.Fprintf(w, "rechazado, tu header fue: %s", req.Header.Get("Authorization"))
		}
	}))
	t.Cleanup(r.Server.Close)
	return r
}

// ultimoCuerpo devuelve el último payload recibido (falla si no llegó ninguno).
func (r *receptorDePrueba) ultimoCuerpo(t *testing.T) []byte {
	t.Helper()
	select {
	case c := <-r.cuerpos:
		return c
	default:
		t.Fatal("el receptor no recibió ningún payload")
		return nil
	}
}

// puntoDePrueba es UN dataPoint ya desarmado, en el vocabulario del exposition format para poder
// compararlo con el scrape línea por línea.
type puntoDePrueba struct {
	Metrica  string
	Labels   string   // device="x",project="y",tier="A",os="linux"
	Claves   []string // las keys de los attributes, en orden
	Valor    float64
	EsEntero bool
	Sello    string
	Unidad   string
}

func comoObjeto(t *testing.T, v any, donde string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s no es un objeto JSON sino %T", donde, v)
	}
	return m
}

func comoLista(t *testing.T, v any, donde string) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("%s no es una lista JSON sino %T", donde, v)
	}
	return l
}

// puntosDelPayload desarma el sobre y, de paso, EXIGE SU FORMA: si timeUnixNano dejara de ser un
// string, o asInt dejara de serlo, o apareciera una métrica sin puntos, esta función falla. Por eso
// la usan todas las pruebas del archivo y no sólo la de la forma.
func puntosDelPayload(t *testing.T, cuerpo []byte) []puntoDePrueba {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(cuerpo, &doc); err != nil {
		t.Fatalf("el payload no es JSON válido: %v\n%s", err, cuerpo)
	}
	var out []puntoDePrueba
	for _, rmAny := range comoLista(t, doc["resourceMetrics"], "resourceMetrics") {
		rm := comoObjeto(t, rmAny, "resourceMetrics[]")
		for _, smAny := range comoLista(t, rm["scopeMetrics"], "scopeMetrics") {
			sm := comoObjeto(t, smAny, "scopeMetrics[]")
			for _, mAny := range comoLista(t, sm["metrics"], "metrics") {
				m := comoObjeto(t, mAny, "metrics[]")
				nombre, _ := m["name"].(string)
				unidad, _ := m["unit"].(string)
				g := comoObjeto(t, m["gauge"], "gauge de "+nombre)
				datos := comoLista(t, g["dataPoints"], "dataPoints de "+nombre)
				if len(datos) == 0 {
					t.Errorf("la métrica %q viaja con dataPoints vacío: una métrica sin puntos es un nombre de serie que el receptor indexa para nada", nombre)
				}
				for _, dAny := range datos {
					d := comoObjeto(t, dAny, "dataPoint de "+nombre)
					sello, ok := d["timeUnixNano"].(string)
					if !ok {
						t.Fatalf("timeUnixNano de %q es %T y tiene que ser un STRING: es un uint64 y JSON no lo aguanta como número; Prometheus responde 400 y el empuje muere en silencio", nombre, d["timeUnixNano"])
					}
					p := puntoDePrueba{Metrica: nombre, Sello: sello, Unidad: unidad}
					crudoInt, hayInt := d["asInt"]
					crudoDbl, hayDbl := d["asDouble"]
					switch {
					case hayInt && hayDbl:
						t.Fatalf("el punto de %q trae asInt Y asDouble a la vez", nombre)
					case hayInt:
						txt, ok := crudoInt.(string)
						if !ok {
							t.Fatalf("asInt de %q es %T y tiene que ser un STRING (convención OTLP/JSON para los int64)", nombre, crudoInt)
						}
						n, err := strconv.ParseInt(txt, 10, 64)
						if err != nil {
							t.Fatalf("asInt de %q no es un entero decimal: %q", nombre, txt)
						}
						p.Valor, p.EsEntero = float64(n), true
					case hayDbl:
						n, ok := crudoDbl.(float64)
						if !ok {
							t.Fatalf("asDouble de %q es %T y tiene que ser un NÚMERO JSON", nombre, crudoDbl)
						}
						p.Valor = n
					default:
						t.Fatalf("el punto de %q no tiene ni asInt ni asDouble: un punto sin valor es el cero fantasma al revés", nombre)
					}
					var partes []string
					for _, aAny := range comoLista(t, d["attributes"], "attributes de "+nombre) {
						a := comoObjeto(t, aAny, "attribute de "+nombre)
						clave, _ := a["key"].(string)
						valor := comoObjeto(t, a["value"], "value del attribute "+clave)
						txt, ok := valor["stringValue"].(string)
						if !ok {
							t.Fatalf("el attribute %q de %q no tiene stringValue", clave, nombre)
						}
						p.Claves = append(p.Claves, clave)
						partes = append(partes, clave+"="+citarLabel(txt))
					}
					p.Labels = strings.Join(partes, ",")
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// seriesDelPayload arma el mapa "serie{labels}" -> valor formateado, en el mismo vocabulario que
// el exposition format, para poder comparar los dos caminos de salida.
func seriesDelPayload(t *testing.T, cuerpo []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range puntosDelPayload(t, cuerpo) {
		out[p.Metrica+"{"+p.Labels+"}"] = formatearValor(p.Valor)
	}
	return out
}

// seriesDelScrape hace lo mismo con el cuerpo de /metrics, quedándose sólo con las de flota.
func seriesDelScrape(salida string) map[string]string {
	out := map[string]string{}
	for _, l := range strings.Split(salida, "\n") {
		if !strings.HasPrefix(l, "musubi_fleet_device_") {
			continue
		}
		i := strings.Index(l, "} ")
		if i < 0 {
			continue
		}
		out[l[:i+1]] = l[i+2:]
	}
	return out
}

// ── I1 · El empujador actúa con la autoridad de un principal nombrado, NUNCA con nil ────────

// EL RIESGO NÚMERO UNO DEL SLICE. Un lazo interno no tiene request, así que principalFrom devuelve
// nil; y con nil, proyectosVisibles marca federado=true y PuedeSobreDevice devuelve true
// incondicionalmente (es la confianza del stdio local). Un empujador descuidado exportaría la
// telemetría de TODOS los tenants a un endpoint externo, sin romper ninguna prueba existente.
//
// Sabotaje que la hace fallar: sacar el `if p == nil` de armarPayloadOTLP y dejar que siga —
// devuelve un payload con las máquinas de los dos proyectos en vez de un error.
func TestArmarPayloadConPrincipalNilNoExporta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)
	maquinaConMuestra(t, s, "cliente-acme", "server-acme", muestraSana(40, ahora), ahora)

	cuerpo, puntos, _, err := armarPayloadOTLP(s.engine, nil, ahora, 0, versionDePrueba)
	if err == nil {
		t.Fatalf("un principal nil produjo un payload de %d puntos en vez de un error:\n%s", puntos, cuerpo)
	}
	if len(cuerpo) != 0 || puntos != 0 {
		t.Errorf("además del error devolvió %d puntos y %d bytes de payload", puntos, len(cuerpo))
	}
	// El mensaje tiene que decir QUÉ hacer, no sólo qué pasó.
	if !strings.Contains(err.Error(), "fleet.otlp.principal") || !strings.Contains(err.Error(), "principals.yaml") {
		t.Errorf("el error no dice cómo arreglarlo: %v", err)
	}
}

// La tenencia gobierna el empuje igual que el scrape.
//
// Sabotaje: pasarle nil a armarPayloadOTLP desde empujarUnaVez (aparecen las dos máquinas).
func TestElEmpujeNoCruzaTenants(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)
	maquinaConMuestra(t, s, "cliente-acme", "server-acme", muestraSana(40, ahora), ahora)

	// Comodín de capacidades, pero acotado a `casa`.
	acotado := &Principal{
		Name: "prom", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	cuerpo, puntos, _, err := armarPayloadOTLP(s.engine, acotado, ahora, 0, versionDePrueba)
	if err != nil || puntos == 0 {
		t.Fatalf("no se armó el payload del principal acotado: %v (%d puntos)", err, puntos)
	}
	if strings.Contains(string(cuerpo), "server-acme") {
		t.Errorf("el empuje cruzó tenants:\n%s", cuerpo)
	}
	if !strings.Contains(string(cuerpo), "pc-gio") {
		t.Errorf("no exportó lo suyo:\n%s", cuerpo)
	}

	// Y la compuerta POR MÁQUINA, no sólo por proyecto: un principal con `metrics` sobre una sola
	// máquina no empuja la de al lado.
	unaSola := &Principal{
		Name: "prom", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"pc-gio"}},
	}
	maquinaConMuestra(t, s, "casa", "nas", muestraSana(40, ahora), ahora)
	cuerpo, _, _, err = armarPayloadOTLP(s.engine, unaSola, ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cuerpo), `"nas"`) {
		t.Errorf("el empuje sorteó la compuerta por máquina:\n%s", cuerpo)
	}
}

// EL TENANT SALE DE LA FILA, JAMÁS DE LO QUE LA MÁQUINA DECLARE.
//
// Una máquina puede mandar lo que quiera en el cuerpo del latido: no hay dónde aterrizar un
// `project`. El label `project` del punto sale de devices.project_id, que se fijó al enrolar.
//
// Sabotaje: hacer que labelsDeFlota tome el proyecto de algo que reporte la máquina (o agregar un
// campo `project` a cuerpoLatido y usarlo) — el punto saldría etiquetado con el tenant ajeno.
func TestElProyectoDeLaSerieSaleDeLaFilaYNoDeLoQueDeclaraLaMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")

	// El latido MIENTE: dice ser de otro tenant, con otro nombre y otro id.
	m := muestraDePrueba()
	txt, err := m.Serializar()
	if err != nil {
		t.Fatal(err)
	}
	mentira := `{"muestra":` + txt + `,"project":"cliente-acme","device":"server-acme","device_id":"robado"}`
	if code, cuerpo := postCon(t, ts.URL+fleetHeartbeatPath, tok, mentira); code != http.StatusOK {
		t.Fatalf("el latido falló: %d %s", code, cuerpo)
	}

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), time.Now(), 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range puntosDelPayload(t, cuerpo) {
		if !strings.Contains(p.Labels, `project="casa"`) || !strings.Contains(p.Labels, `device="pc-gio"`) {
			t.Fatalf("la serie se etiquetó con lo que declaró la máquina y no con su fila: %s{%s}", p.Metrica, p.Labels)
		}
	}
	if strings.Contains(string(cuerpo), "acme") || strings.Contains(string(cuerpo), "robado") {
		t.Errorf("algo de lo que declaró la máquina llegó al payload:\n%s", cuerpo)
	}
}

// ptrPrincipal es azúcar para pasar un principal literal por puntero.
func ptrPrincipal(p Principal) *Principal { return &p }

// ── I2 · Sin principal válido, el servidor no arranca ───────────────────────────────────────

// De las dos formas de enterarse de que el empuje está mal configurado, «el servidor no arranca»
// es mucho más barata que «los gráficos están vacíos y nadie sabe desde cuándo».
//
// Sabotaje que la hace fallar: borrar la llamada a validarPrincipalDeEmpuje de
// vincularRegistroDeFlota — los tres casos arrancan y el empuje queda mudo para siempre.
func TestElEmpujeNoArrancaSinPrincipalNombrado(t *testing.T) {
	casos := []struct {
		nombre string
		cfg    config.OTLPPushConfig
		reg    *PrincipalRegistry
		enPuja string // un pedazo del mensaje que el operador tiene que leer
	}{
		{
			nombre: "endpoint sin principal",
			cfg:    config.OTLPPushConfig{Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics"},
			reg:    registroDePrueba(principalDePrometheus()),
			enPuja: "fleet.otlp.principal",
		},
		{
			nombre: "el principal no existe",
			cfg:    config.OTLPPushConfig{Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics", Principal: "prometheus"},
			reg:    registroDePrueba(),
			enPuja: "no existe en principals.yaml",
		},
		{
			nombre: "el principal no tiene ninguna concesión metrics",
			cfg:    config.OTLPPushConfig{Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics", Principal: "prometheus"},
			reg: registroDePrueba(Principal{
				Name: "prometheus", Role: RoleAdmin, Read: ReadAll, Write: WriteAny,
			}),
			enPuja: "concesión `metrics`",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			err := s.ConfigurarFlota(config.FleetConfig{OTLP: c.cfg})
			if err == nil {
				err = s.vincularRegistroDeFlota(c.reg)
			}
			if err == nil {
				t.Fatal("el servidor arrancó con el empuje mal configurado")
			}
			if !strings.Contains(err.Error(), c.enPuja) {
				t.Errorf("el error no le dice al operador qué hacer.\n  quería que contuviera: %s\n  dijo: %v", c.enPuja, err)
			}
		})
	}

	// Y el camino feliz sí arranca: una guarda que rechaza todo no prueba nada.
	s := newTestServer(t, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{OTLP: config.OTLPPushConfig{
		Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics", Principal: "prometheus",
	}}); err != nil {
		t.Fatalf("una configuración buena no arrancó: %v", err)
	}
	if err := s.vincularRegistroDeFlota(registroDePrueba(principalDePrometheus())); err != nil {
		t.Fatalf("una configuración buena no arrancó: %v", err)
	}
}

// El empuje APAGADO es el default y no valida nada: nadie tiene que declarar un principal para no
// empujar.
func TestSinEndpointElEmpujeNiSiquieraSeConfigura(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
		t.Fatalf("la config vacía tiene que seguir arrancando: %v", err)
	}
	if err := s.vincularRegistroDeFlota(registroDePrueba()); err != nil {
		t.Fatalf("sin empuje no hay principal que validar: %v", err)
	}
	if s.empujador != nil || s.empujeCfg.Activo() {
		t.Error("el empuje nació encendido: encender una salida de datos hacia afuera tiene que ser una decisión, no un default")
	}
	// Y RunEmpujeOTLP con el empuje apagado retorna en el acto en vez de quedarse en un ticker.
	listo := make(chan struct{})
	go func() { s.RunEmpujeOTLP(context.Background()); close(listo) }()
	select {
	case <-listo:
	case <-time.After(2 * time.Second):
		t.Fatal("RunEmpujeOTLP se quedó corriendo con el empuje apagado")
	}
}

// ── I3 · El empuje exporta exactamente lo mismo que el scrape ───────────────────────────────

// Si esto deja de valer, hay dos exportadores y un día muestran cosas distintas.
//
// Sabotaje que la hace fallar: darle al empujador su propio recorrido de devices sin
// PuedeSobreDevice, o su propia copia de la tabla de series (por ejemplo, sacarle una fila).
func TestElEmpujeYElScrapeExportanLasMismasSeriesYLosMismosValores(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	maquinaConMuestra(t, s, "cliente-acme", "server-acme", muestraSana(70, ahora), ahora)
	maquinaConMuestra(t, s, "casa", "nas", muestraSana(10, ahora.Add(-2*time.Hour)), ahora.Add(-2*time.Hour))
	enrolarDePrueba(t, s, "casa", "muda") // enrolada y sin latir nunca

	p := ptrPrincipal(principalDePrometheus())
	var b strings.Builder
	renderFlota(&b, s.engine, p, ahora, s.sondaIntervalo, versionDePrueba)
	delScrape := seriesDelScrape(b.String())

	cuerpo, puntos, _, err := armarPayloadOTLP(s.engine, p, ahora, s.sondaIntervalo, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	delPush := seriesDelPayload(t, cuerpo)

	if puntos != len(delPush) {
		t.Errorf("el conteo de puntos (%d) no coincide con los puntos del payload (%d)", puntos, len(delPush))
	}
	if len(delScrape) == 0 {
		t.Fatal("el scrape no exportó nada: la comparación sería vacía y verde")
	}
	for clave, valor := range delScrape {
		otro, hay := delPush[clave]
		if !hay {
			t.Errorf("el scrape exporta %s y el empuje NO", clave)
			continue
		}
		if otro != valor {
			t.Errorf("%s: el scrape dice %s y el empuje dice %s", clave, valor, otro)
		}
	}
	for clave := range delPush {
		if _, hay := delScrape[clave]; !hay {
			t.Errorf("el empuje exporta %s y el scrape NO", clave)
		}
	}
}

// ── I4 · Lo desconocido no se emite como cero, tampoco en OTLP ──────────────────────────────

// Sabotaje: devolver (0, true) en una fila de seriesDeFlota; o emitir la métrica con
// `"gauge":{"dataPoints":[]}` en vez de saltearla (puntosDelPayload lo caza).
func TestUnValorDesconocidoNoViajaComoCeroEnElPayload(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Muestra SIN cpu_pct, sin temp_c, sin mem_libre y sin procesos: la primera de un agente.
	maquinaConMuestra(t, s, "casa", "pc-gio", fleet.Muestra{
		Tomada: ahora, NumCPU: 4, MemTotal: 100, MemUsada: 25,
		DiscoTotal: 1000, DiscoUsado: 100, DiscoDisponible: 850,
	}, ahora)

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	presentes := map[string]bool{}
	for _, p := range puntosDelPayload(t, cuerpo) {
		presentes[p.Metrica] = true
	}
	for _, noQuiero := range []string{
		"musubi_fleet_device_cpu_percent",
		"musubi_fleet_device_temperature_celsius",
		"musubi_fleet_device_memory_free_bytes",
		"musubi_fleet_device_processes",
	} {
		if presentes[noQuiero] {
			t.Errorf("%s viajó en el payload sin haberse medido: un 0 entra al gráfico como una medición real", noQuiero)
		}
	}
	// Y lo que sí se midió está, incluido un `up` que vale 1: la regla es «lo desconocido no
	// viaja», no «lo que vale poco no viaja».
	for _, quiero := range []string{"musubi_fleet_device_up", "musubi_fleet_device_memory_used_bytes"} {
		if !presentes[quiero] {
			t.Errorf("falta la serie medida %s:\n%s", quiero, cuerpo)
		}
	}
}

// Un `up` en CERO tiene que viajar: es el valor del que vive MaquinaCaida. Con `float64` y
// omitempty en vez de un puntero, el 0 perdería su campo y llegaría un punto SIN VALOR.
//
// Sabotaje: cambiar AsDouble de *float64 a float64 con omitempty.
func TestUnUpEnCeroViajaConSuCero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Latió hace una hora: para su umbral está caída.
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora.Add(-time.Hour)), ahora.Add(-time.Hour))

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	visto := false
	for _, p := range puntosDelPayload(t, cuerpo) {
		if p.Metrica != "musubi_fleet_device_up" {
			continue
		}
		visto = true
		if p.Valor != 0 {
			t.Errorf("la máquina caída exportó up=%v", p.Valor)
		}
	}
	if !visto {
		t.Fatalf("el `up` en 0 no viajó; sin él MaquinaCaida no tiene de qué vivir:\n%s", cuerpo)
	}
	if !strings.Contains(string(cuerpo), `"asDouble":0`) {
		t.Errorf("el 0 no quedó escrito en el JSON (un punto sin valor es el cero fantasma al revés):\n%s", cuerpo)
	}
}

// ── I5 · Los labels son los cuatro, y salen de la fila ──────────────────────────────────────

// Sabotaje: agregar `agent_version` a atributosOTLP; o renombrar `device` a `hostname` (las 12
// reglas de alerta siguen evaluándose, no fallan, y no disparan nunca).
func TestElPayloadNoTomaLabelsDelAutorreporte(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)
	if err := s.engine.ActualizarAutoreporte(d.ID, "0.108.0-flota", "100.114.63.7"); err != nil {
		t.Fatal(err)
	}

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	for _, prohibido := range []string{"0.108.0-flota", "100.114.63.7", "agent_version", "hostname", "address"} {
		if strings.Contains(string(cuerpo), prohibido) {
			t.Errorf("%q llegó al payload: la cardinalidad de la serie la acota el cerebro, no lo que reporte la máquina\n%s", prohibido, cuerpo)
		}
	}
	esperadas := []string{"device", "project", "tier", "os"}
	for _, p := range puntosDelPayload(t, cuerpo) {
		if len(p.Claves) != len(esperadas) {
			t.Fatalf("%s tiene %d attributes (%v); son EXACTAMENTE cuatro", p.Metrica, len(p.Claves), p.Claves)
		}
		for i, quiero := range esperadas {
			if p.Claves[i] != quiero {
				t.Fatalf("%s: el attribute %d es %q y tiene que ser %q — el orden es canónico y lo comparte con el exposition format", p.Metrica, i, p.Claves[i], quiero)
			}
		}
	}
}

// ── I6 e I7 · El empujador no frena al cerebro, y no se solapa ──────────────────────────────

// Un cerebro que se cuelga porque el sistema de métricas no contesta es peor que no tener
// métricas.
//
// Sabotaje: envolver empujarUnaVez en s.dispatchMu.Lock(); o quitarle el Timeout al http.Client.
func TestUnPrometheusColgadoNoFrenaNingunaTool(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	destino.bloqueo = make(chan struct{})
	destino.recibido = make(chan struct{}, 1)
	// LIFO: primero se destraba el handler, después se espera al empuje, y recién ahí cierra el
	// httptest (que espera a las requests en vuelo y si no se destrabaran quedaría colgado).
	enVuelo := make(chan struct{})
	t.Cleanup(func() { <-enVuelo })
	t.Cleanup(func() { close(destino.bloqueo) })

	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), func(c *config.OTLPPushConfig) {
		c.TimeoutSeconds = 5
	})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	go func() { s.empujarUnaVez(context.Background(), ahora); close(enVuelo) }()
	<-destino.recibido // el POST está en vuelo y el destino no va a contestar

	inicio := time.Now()
	if _, e := call(t, s, "musubi_save_observation", map[string]any{
		"topic_key": "flota", "content": "una escritura mientras el empuje cuelga",
	}); e != nil {
		t.Fatalf("la tool falló mientras el empuje estaba colgado: %+v", e)
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Errorf("la tool tardó %s con el destino OTLP colgado: el empuje está frenando al cerebro", d)
	}
}

// UN EMPUJE EN VUELO A LA VEZ. Un destino más lento que el tick acumularía goroutines y payloads
// en un proceso que vive días.
//
// Sabotaje: reemplazar el CompareAndSwap de empujeBusy por un `go func()` por tick (llegan dos
// requests).
func TestDosTicksDeEmpujeNoSeSolapan(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	destino.bloqueo = make(chan struct{})
	destino.recibido = make(chan struct{}, 1)
	enVuelo := make(chan struct{})
	t.Cleanup(func() { <-enVuelo })
	t.Cleanup(func() { close(destino.bloqueo) })

	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), func(c *config.OTLPPushConfig) {
		c.TimeoutSeconds = 5
	})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	go func() { s.empujarUnaVez(context.Background(), ahora); close(enVuelo) }()
	<-destino.recibido

	// El segundo tick cae con el primero todavía en vuelo: se saltea y no manda nada.
	s.empujarUnaVez(context.Background(), ahora)
	if n := destino.pedidos.Load(); n != 1 {
		t.Errorf("el destino recibió %d requests; el segundo tick tenía que saltearse", n)
	}
}

// ── I8 · Revocar al principal del empuje lo apaga en el acto ────────────────────────────────

// Una política fantasma queda inerte (falla cerrada); un EMPUJADOR fantasma sigue mandando datos.
// Por eso el principal se resuelve en cada tick y no se guarda resuelto al arrancar.
//
// Sabotaje: guardar el *Principal resuelto en el struct al arrancar y reusarlo en cada tick.
func TestRevocarAlPrincipalDelEmpujeLoApagaEnElActo(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	s.empujarUnaVez(context.Background(), ahora)
	if n := destino.pedidos.Load(); n != 1 {
		t.Fatalf("el primer empuje no llegó (%d requests)", n)
	}

	// Alguien lo saca de principals.yaml entre dos ticks. El registro se recarga en caliente.
	s.buscarPrincipal = registroDePrueba()
	s.empujarUnaVez(context.Background(), ahora.Add(30*time.Second))
	s.empujarUnaVez(context.Background(), ahora.Add(60*time.Second))
	if n := destino.pedidos.Load(); n != 1 {
		t.Errorf("el empujador siguió mandando datos con el principal revocado (%d requests)", n)
	}
	// Y avisa UNA sola vez: el aviso es un ESTADO que dura hasta que alguien edite el archivo.
	if _, avisado := s.avisosDados.Load("empuje_sin_principal"); !avisado {
		t.Error("no avisó que el principal ya no está: un empuje mudo se ve igual que todo tranquilo")
	}
	if n := s.empujeFallos.Load(); n != 2 {
		t.Errorf("la señal sí se cuenta siempre: esperaba 2 fallos, hubo %d", n)
	}
}

// ── I9 · El secreto entra por referencia y no se escribe en ningún lado ─────────────────────

// El destino responde 401 y DEVUELVE EL HEADER en el cuerpo, que es lo que hace un proxy mal
// configurado en el medio. Ni el token ni el cuerpo de la respuesta pueden terminar en el error
// (que es exactamente lo que se logea).
//
// Sabotaje: volcar resp.Body en el error, o incluir e.token en el mensaje.
func TestElErrorDelEmpujeNoLlevaLaCredencialNiElCuerpoDelDestino(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusUnauthorized)
	destino.ecoAuth = true
	const secreto = "msb_un_secreto_que_no_puede_aparecer"
	t.Setenv("MUSUBI_OTLP_TOKEN_TEST", secreto)

	emp, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: destino.URL, Principal: "prometheus", AuthTokenEnv: "MUSUBI_OTLP_TOKEN_TEST",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = emp.enviar(context.Background(), []byte(`{"resourceMetrics":[]}`))
	if err == nil {
		t.Fatal("un 401 tiene que ser un error")
	}
	if strings.Contains(err.Error(), secreto) {
		t.Errorf("el error lleva la credencial: %v", err)
	}
	if strings.Contains(err.Error(), "tu header fue") {
		t.Errorf("el error lleva el cuerpo de la respuesta del destino, que puede ser el eco del request: %v", err)
	}
	// Y le dice al operador dónde mirar.
	if !strings.Contains(err.Error(), "auth_token_env") {
		t.Errorf("el error no dice qué revisar: %v", err)
	}
}

// Un destino http:// que NO es loopback haría viajar el bearer en texto plano. Fail-closed.
//
// Sabotaje: quitar la comprobación de esquema/loopback de nuevoEmpujadorOTLP.
func TestUnDestinoRemotoSinTLSNoArranca(t *testing.T) {
	_, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: "http://prometheus.ajeno.example/api/v1/otlp/v1/metrics", Principal: "prometheus",
	})
	if err == nil {
		t.Fatal("un http:// remoto arrancó: el bearer viajaría en claro")
	}
	if !strings.Contains(err.Error(), "allow_insecure_token") {
		t.Errorf("el error no dice cómo optar explícitamente: %v", err)
	}
	// Loopback sí, sin ninguna perilla: ahí no hay red por la que filtrarse.
	if _, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics", Principal: "prometheus",
	}); err != nil {
		t.Errorf("el destino loopback tiene que arrancar sin perillas: %v", err)
	}
	// Y con la perilla puesta, el remoto también: la guarda no puede ser un muro sin puerta.
	if _, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: "http://prometheus.ajeno.example/api/v1/otlp/v1/metrics", Principal: "prometheus",
		AllowInsecureToken: true,
	}); err != nil {
		t.Errorf("con allow_insecure_token tiene que arrancar: %v", err)
	}
}

// Una URL con userinfo es un secreto que termina en un log de diagnóstico el primer día malo.
//
// Sabotaje: sacar el `if u.User != nil` de nuevoEmpujadorOTLP.
func TestUnaURLConUserinfoSeRechaza(t *testing.T) {
	_, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: "https://prom:clave-secreta@ejemplo.com/api/v1/otlp/v1/metrics", Principal: "prometheus",
	})
	if err == nil {
		t.Fatal("una URL con usuario y contraseña arrancó")
	}
	if strings.Contains(err.Error(), "clave-secreta") {
		t.Errorf("el error que rechaza la credencial en la URL la IMPRIME: %v", err)
	}
	if !strings.Contains(err.Error(), "auth_token_env") {
		t.Errorf("el error no dice por dónde entra el secreto: %v", err)
	}
	// Y un auth_token_env que apunta a una variable vacía tampoco arranca: si no, el empuje sale
	// sin credencial y el destino contesta 401 para siempre con la configuración «puesta».
	if _, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint: "http://127.0.0.1:9099/api/v1/otlp/v1/metrics", Principal: "prometheus",
		AuthTokenEnv: "MUSUBI_VARIABLE_QUE_NO_EXISTE_EN_ESTE_TEST",
	}); err == nil {
		t.Error("arrancó con auth_token_env apuntando a una variable vacía")
	}
}

// ── I10 · El empuje no lleva las métricas del servidor ──────────────────────────────────────

// Esas series están detrás de auth desde la auditoría 2026-07-26 #9, y
// musubi_fleet_policy_actions_total NO lleva etiqueta de máquina a propósito: empujarlas a un
// store sin credencial deshace esa corrección por la otra puerta.
//
// Sabotaje: agregarle al empujador el render de metrics.render(s.engine).
func TestElEmpujeNoLlevaLasMetricasDelServidor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	prohibidos := []string{
		"musubi_tool_", "musubi_rejections_", "musubi_db_", "musubi_outbox_", "musubi_sync_",
		"musubi_http_", "musubi_observations", "musubi_fleet_policy_actions_total",
	}
	for _, p := range puntosDelPayload(t, cuerpo) {
		if !strings.HasPrefix(p.Metrica, "musubi_fleet_device_") {
			t.Errorf("el payload lleva %q, que no es telemetría de una máquina", p.Metrica)
		}
		for _, mal := range prohibidos {
			if strings.HasPrefix(p.Metrica, mal) {
				t.Errorf("el payload lleva una métrica del SERVIDOR: %q", p.Metrica)
			}
		}
	}
}

// ── I11 · El sobre tiene la forma de la especificación ──────────────────────────────────────

// Sabotaje: emitir timeUnixNano como número, o asInt como número — Prometheus contesta 400 y el
// empuje muere en silencio. puntosDelPayload lo caza por el tipo JSON, no por el valor.
func TestElSobreOTLPTieneLaFormaDeLaEspecificacion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)

	cuerpo, puntos, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	if puntos == 0 {
		t.Fatal("el payload salió vacío: la prueba de forma no tendría nada que mirar")
	}
	// La estructura anidada, nivel por nivel (puntosDelPayload ya exige el resto).
	var doc map[string]any
	if err := json.Unmarshal(cuerpo, &doc); err != nil {
		t.Fatal(err)
	}
	rms := comoLista(t, doc["resourceMetrics"], "resourceMetrics")
	if len(rms) != 1 {
		t.Fatalf("esperaba UN resourceMetrics, hay %d", len(rms))
	}
	rm := comoObjeto(t, rms[0], "resourceMetrics[0]")
	recurso := comoObjeto(t, rm["resource"], "resource")
	atributos := map[string]string{}
	for _, aAny := range comoLista(t, recurso["attributes"], "resource.attributes") {
		a := comoObjeto(t, aAny, "resource.attribute")
		clave, _ := a["key"].(string)
		valor := comoObjeto(t, a["value"], "value")
		txt, _ := valor["stringValue"].(string)
		atributos[clave] = txt
	}
	if atributos["service.name"] != "musubi" {
		t.Errorf("service.name = %q", atributos["service.name"])
	}
	// La marca que distingue lo empujado de lo scrapeado cuando el mismo Prometheus recibe las dos
	// cosas. Si cambia, la receta de deploy/prometheus/prometheus.yml deja de aplicar.
	if atributos["service.instance.id"] != instanciaDelEmpuje {
		t.Errorf("service.instance.id = %q, esperaba %q", atributos["service.instance.id"], instanciaDelEmpuje)
	}
	sms := comoLista(t, rm["scopeMetrics"], "scopeMetrics")
	if len(sms) != 1 {
		t.Fatalf("esperaba UN scopeMetrics, hay %d", len(sms))
	}
	ambito := comoObjeto(t, comoObjeto(t, sms[0], "scopeMetrics[0]")["scope"], "scope")
	if ambito["name"] != nombreScopeOTLP {
		t.Errorf("el scope es %q, esperaba %q", ambito["name"], nombreScopeOTLP)
	}
	// Y los sellos son decimales puros: nada de notación científica ni de comillas adentro.
	for _, p := range puntosDelPayload(t, cuerpo) {
		if _, err := strconv.ParseUint(p.Sello, 10, 64); err != nil {
			t.Fatalf("timeUnixNano %q de %s no es un uint64 decimal", p.Sello, p.Metrica)
		}
	}
}

// LA UNIDAD NO PUEDE RENOMBRAR LA SERIE. El receptor OTLP de Prometheus normaliza el nombre con la
// unidad: a un gauge con unidad "1" le agrega `_ratio`, y `musubi_fleet_device_up` llegaría como
// `musubi_fleet_device_up_ratio` — con las 12 reglas de alerta evaluándose y sin disparar nunca.
// La regla que lo evita: o no se declara unidad, o el nombre ya la lleva en el sufijo.
//
// Sabotaje que la hace fallar: ponerle Unidad "1" a musubi_fleet_device_up (que es lo que dice la
// especificación de OTLP para lo adimensional, y es justo lo que rompe acá).
func TestNingunaUnidadRenombraLaSerieEnPrometheus(t *testing.T) {
	sufijos := map[string]string{"By": "_bytes", "s": "_seconds", "Cel": "_celsius", "%": "_percent"}
	for _, serie := range seriesDeFlota(time.Now(), 0, versionDePrueba) {
		if serie.Unidad == "" {
			continue
		}
		if serie.Unidad == "1" {
			t.Errorf("%s declara unidad \"1\": Prometheus le agregaría el sufijo `_ratio` al normalizar y la serie cambiaría de nombre en el camino. Dejala vacía.", serie.Nombre)
			continue
		}
		sufijo, conocida := sufijos[serie.Unidad]
		if !conocida {
			t.Errorf("%s declara la unidad %q, que no está en la tabla de sufijos conocidos: verificá cómo la normaliza Prometheus antes de usarla", serie.Nombre, serie.Unidad)
			continue
		}
		if !strings.HasSuffix(serie.Nombre, sufijo) {
			t.Errorf("%s declara unidad %q pero su nombre no termina en %q: la normalización del receptor se lo va a agregar y la serie va a llegar con OTRO nombre", serie.Nombre, serie.Unidad, sufijo)
		}
	}
}

// ── I12 · Un solo reloj ─────────────────────────────────────────────────────────────────────

// Sabotaje: llamar a time.Now() adentro del bucle de puntos, o dos veces (una para decidir `up` y
// otra para sellar).
func TestElPayloadUsaUnSoloReloj(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)
	maquinaConMuestra(t, s, "casa", "nas", muestraSana(50, ahora), ahora)

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	quiero := strconv.FormatInt(ahora.UnixNano(), 10)
	for _, p := range puntosDelPayload(t, cuerpo) {
		if p.Sello != quiero {
			t.Fatalf("%s{%s} lleva el sello %s y el reloj del export es %s: el instante con el que se decide `up` tiene que ser el que sella los puntos",
				p.Metrica, p.Labels, p.Sello, quiero)
		}
	}
}

// ── I13 · La falla del empuje se ve desde el tirón ──────────────────────────────────────────

// Un mecanismo de monitoreo cuya única forma de avisar de su propia muerte es él mismo no avisa
// nunca: si el POST no llega, un contador que viajara adentro del POST tampoco llega.
//
// Sabotaje: emitir musubi_push_last_success_seconds en 0 cuando nunca hubo un empuje exitoso (un
// «último éxito hace 56 años» se lee como un bug del panel y no como «esto nunca funcionó»).
func TestLaFallaDelEmpujeSeVeDesdeMetrics(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusInternalServerError)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	var antes strings.Builder
	s.renderEmpuje(&antes, ahora)
	if strings.Contains(antes.String(), "musubi_push_last_success_seconds") {
		t.Errorf("se exportó la fecha del último empuje sin que nunca hubiera habido uno:\n%s", antes.String())
	}
	if !strings.Contains(antes.String(), "musubi_push_failures_total 0") {
		t.Errorf("el contador de fallos no se emite en cero: `rate()` no puede distinguir «no falló» de «dejó de exportar»:\n%s", antes.String())
	}

	s.empujarUnaVez(context.Background(), ahora)
	s.empujarUnaVez(context.Background(), ahora)
	var despues strings.Builder
	s.renderEmpuje(&despues, ahora)
	if !strings.Contains(despues.String(), "musubi_push_failures_total 2") {
		t.Errorf("los fallos no se ven desde /metrics:\n%s", despues.String())
	}

	// Y con el destino sano, la serie del último éxito aparece.
	destino.estado = http.StatusOK
	s.empujarUnaVez(context.Background(), ahora)
	var sano strings.Builder
	s.renderEmpuje(&sano, ahora.Add(90*time.Second))
	if !strings.Contains(sano.String(), "musubi_push_last_success_seconds 90") {
		t.Errorf("después de un empuje aceptado falta (o miente) la antigüedad del último éxito:\n%s", sano.String())
	}
	if !strings.Contains(sano.String(), "musubi_push_datapoints ") {
		t.Errorf("falta el conteo de puntos del último empuje: un 0 sostenido es «el lazo corre y no exporta nada»:\n%s", sano.String())
	}
}

// Las tres series salen POR EL ENDPOINT REAL, no sólo por la función: un render que nadie cablea
// es exactamente el estado en el que estuvo /metrics antes de que alguien lo scrapeara.
//
// Sabotaje: sacar la llamada a s.renderEmpuje del handler de /metrics en http.go.
func TestLasSeriesDelEmpujeSalenPorElMetricsDeVerdad(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ts := servidorHTTP(t, s)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer token-de-una-persona")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	crudo, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(crudo), "musubi_push_failures_total") {
		t.Errorf("/metrics no expone la auto-vigilancia del empuje:\n%s", crudo)
	}
}

// ── I14 · El truncado se anuncia ────────────────────────────────────────────────────────────

// El push no tiene dónde poner un comentario que el parser ignore —el scrape sí—, así que el
// aviso va al log. Truncar en silencio es dejar media flota sin exportar sin que nadie lo sepa.
//
// Sabotaje: truncar sin avisar (borrar el avisarUnaVez de empujarUnaVez).
func TestUnBarridoTruncadoSeAnuncia(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	for i := 0; i < proyectosParaExportar+1; i++ {
		proyecto := fmt.Sprintf("proyecto-%03d", i)
		d, err := s.engine.AltaDevice(fleet.Device{
			Name: "maquina", ProjectID: proyecto, Tier: fleet.TierAgente,
			Caps: []fleet.Cap{fleet.CapMetrics}, OS: "linux",
		}, fmt.Sprintf("token-de-prueba-%03d", i))
		if err != nil {
			t.Fatalf("alta de la máquina %d: %v", i, err)
		}
		latir(t, s, d.ID, muestraSana(40, ahora), ahora)
	}

	_, _, truncado, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	if !truncado {
		t.Fatalf("con %d proyectos el barrido tenía que dar truncado", proyectosParaExportar+1)
	}
	s.empujarUnaVez(context.Background(), ahora)
	if _, avisado := s.avisosDados.Load("empuje_truncado"); !avisado {
		t.Error("se truncó el barrido y no se avisó: media flota sin exportar y nadie enterado")
	}
}

// ── La clasificación del fallo ──────────────────────────────────────────────────────────────

// El 404 es EL error de este slice: Prometheus no acepta OTLP por defecto. El mensaje tiene que
// nombrar el flag, o alguien pierde una tarde con la configuración del cerebro perfecta.
//
// Sabotaje: devolver un error genérico de «HTTP 404» sin nombrar --web.enable-otlp-receiver.
func TestUn404DiceQueFaltaElFlagDeProm(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusNotFound)
	emp, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{Endpoint: destino.URL, Principal: "prometheus"})
	if err != nil {
		t.Fatal(err)
	}
	err = emp.enviar(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("un 404 tiene que ser un error")
	}
	if !strings.Contains(err.Error(), "--web.enable-otlp-receiver") {
		t.Errorf("el 404 no nombra el flag que falta: %v", err)
	}
	if !strings.Contains(err.Error(), "/api/v1/otlp/v1/metrics") {
		t.Errorf("el 404 no nombra el path correcto: %v", err)
	}
}

// ── A50 · Los TRES modos de quedarse mudo, y por qué ninguno contaba un fallo ────────────────

// capturarLog redirige logx a un buffer mientras dura la prueba y devuelve el buffer.
//
// LA PRUEBA MIRA LA LÍNEA DE LOG Y NO UN CONTADOR, y eso es a propósito: el aviso de A50 no
// cuenta un fallo —no llegó a intentar entregar nada— así que el log ES el efecto observable.
// Una prueba que mirara sólo `avisosDados` quedaría en verde con la línea borrada.
func capturarLog(t *testing.T) *strings.Builder {
	t.Helper()
	var b strings.Builder
	t.Cleanup(logx.Capturar(&b))
	return &b
}

// El arranque EXIGE la concesión `metrics` y se niega a arrancar sin ella; pero principals.yaml se
// recarga en caliente cada 10 s, así que se la pueden sacar DESPUÉS de un arranque válido. Antes de
// A50 el empuje seguía corriendo, armaba un payload vacío y volvía sin dejar rastro.
//
// Sabotaje: borrar el bloque `if len(p.Fleet[fleet.CapMetrics]) == 0` de empujarUnaVez.
func TestSacarleLaConcesionMetricsEnCalienteLoDiceEnElLog(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	s.empujarUnaVez(context.Background(), ahora)
	if n := destino.pedidos.Load(); n != 1 {
		t.Fatalf("el primer empuje no llegó (%d requests)", n)
	}
	if p := s.empujeDatapoints.Load(); p == 0 {
		t.Fatalf("el empuje bueno no dejó puntos: la prueba no puede distinguir el después del antes")
	}
	fallosAntes := s.empujeFallos.Load()

	// Alguien le saca la sección `fleet:` al principal. El principal SIGUE EXISTIENDO — este es el
	// caso que TestRevocarAlPrincipalDelEmpujeLoApagaEnElActo no cubre.
	log := capturarLog(t)
	sinConcesion := principalDePrometheus()
	sinConcesion.Fleet = map[fleet.Cap][]string{}
	s.buscarPrincipal = registroDePrueba(sinConcesion)

	s.empujarUnaVez(context.Background(), ahora.Add(30*time.Second))
	s.empujarUnaVez(context.Background(), ahora.Add(60*time.Second))

	if n := destino.pedidos.Load(); n != 1 {
		t.Errorf("siguió empujando sin la concesión `metrics` (%d requests)", n)
	}
	texto := log.String()
	// LA FRASE TIENE QUE SER LA DE ESTE CASO Y NO LA DEL OTRO. El aviso de `empuje_vacio` también
	// dice «concesión `metrics`», así que buscar eso deja pasar el sabotaje: sin el bloque de la
	// concesión el empuje cae en la rama de abajo, avisa OTRA cosa, y la prueba quedaba en verde.
	if !strings.Contains(texto, "ya NO tiene ninguna concesión") {
		t.Errorf("el log no nombra la causa; el operador ve MusubiPushOTLPMudo y nada más.\nlog:\n%s", texto)
	}
	if strings.Contains(texto, "no alcanza a NINGUNA máquina") {
		t.Errorf("avisó el caso equivocado: la concesión no está VACÍA DE PROYECTOS, no existe.\nlog:\n%s", texto)
	}
	if !strings.Contains(texto, "prometheus") {
		t.Errorf("el log no nombra al principal que hay que arreglar en principals.yaml.\nlog:\n%s", texto)
	}
	// UNA sola vez: es un ESTADO que dura hasta que alguien edite el archivo, no un evento. Dos
	// ticks, una línea.
	if n := strings.Count(texto, "ya NO tiene ninguna concesión"); n != 1 {
		t.Errorf("avisó %d veces en dos ticks; a 30 s son 2.880 líneas idénticas por día", n)
	}
	// NO cuenta un fallo: `musubi_push_failures_total` significa «no llegó a destino» y acá ni se
	// intentó. Ensuciarlo rompería MusubiPushOTLPNuncaLlego, que separa «se cayó» de «nunca anduvo».
	if n := s.empujeFallos.Load(); n != fallosAntes {
		t.Errorf("contó %d fallos nuevos: no llegar a intentar no es fallar en entregar", n-fallosAntes)
	}
	// Y el gauge deja de mentir: sin esto sigue publicando el último conteo bueno para siempre.
	if p := s.empujeDatapoints.Load(); p != 0 {
		t.Errorf("musubi_push_datapoints quedó en %d con el empuje mudo: el gauge cuyo HELP dice "+
			"«un 0 sostenido = el empujador corre y no exporta nada» nunca llega a mostrar un 0", p)
	}
}

// El gauge de puntos se quedaba con el último conteo BUENO en todos los caminos de salida
// temprana. Su propio HELP declara que un 0 sostenido es la firma de que el empujador corre sin
// exportar — y no había forma de que llegara a valer 0 en esa situación.
//
// Sabotaje: quitar `s.empujeDatapoints.Store(0)` de la rama `if !ok` de principalDelEmpuje.
func TestElGaugeDePuntosNoSeQuedaConElUltimoConteoBueno(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	s.empujarUnaVez(context.Background(), ahora)
	buenos := s.empujeDatapoints.Load()
	if buenos == 0 {
		t.Fatalf("el empuje bueno no dejó puntos: la prueba no distingue el después del antes")
	}

	s.buscarPrincipal = registroDePrueba() // desaparece de principals.yaml
	s.empujarUnaVez(context.Background(), ahora.Add(30*time.Second))

	if p := s.empujeDatapoints.Load(); p != 0 {
		t.Errorf("musubi_push_datapoints siguió publicando %d puntos sin principal; "+
			"un tablero que mira ese gauge ve el empuje sano mientras está muerto", p)
	}
}

// El tercer modo, y el más difícil de ver: el principal existe Y tiene su concesión, pero la
// concesión apunta a proyectos donde no hay ni una máquina. Desde afuera es idéntico a los otros
// dos —cero puntos, cero fallos, silencio— y el arreglo es completamente distinto.
//
// Sabotaje: borrar el bloque `avisarUnaVez("empuje_vacio", ...)` de empujarUnaVez.
func TestUnEmpujeQueNoAlcanzaNingunaMaquinaLoDice(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	// La concesión es real y no está vacía: apunta a un proyecto que no existe.
	perdido := principalDePrometheus()
	perdido.Fleet = map[fleet.Cap][]string{fleet.CapMetrics: {"proyecto-que-alguien-renombro"}}
	s := prepararEmpuje(t, destino.URL, registroDePrueba(perdido), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	log := capturarLog(t)
	s.empujarUnaVez(context.Background(), ahora)
	s.empujarUnaVez(context.Background(), ahora.Add(30*time.Second))

	if n := destino.pedidos.Load(); n != 0 {
		t.Fatalf("mandó %d sobres vacíos: el escenario de la prueba no es el que dice", n)
	}
	texto := log.String()
	if !strings.Contains(texto, "no alcanza a NINGUNA máquina") {
		t.Errorf("se quedó mudo sin decirlo.\nlog:\n%s", texto)
	}
	// Dice a QUÉ apunta la concesión: sin eso el operador tiene el síntoma y no el archivo.
	if !strings.Contains(texto, "proyecto-que-alguien-renombro") {
		t.Errorf("el log no dice a qué apunta la concesión que no alcanza a nadie.\nlog:\n%s", texto)
	}
	if n := strings.Count(texto, "no alcanza a NINGUNA máquina"); n != 1 {
		t.Errorf("avisó %d veces en dos ticks; es un estado, no un evento", n)
	}
}

// El aviso se REARMA cuando el problema se resuelve. Un «avisar una vez» que no se rearma
// convierte el segundo incidente en silencio total, que es peor que el ruido que evita.
//
// Sabotaje: quitar `s.avisosDados.Delete("empuje_sin_concesion")` (o el de "empuje_vacio").
func TestElAvisoDelEmpujeMudoSeRearmaCuandoVuelveLaConcesion(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	sinConcesion := principalDePrometheus()
	sinConcesion.Fleet = map[fleet.Cap][]string{}
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	log := capturarLog(t)
	s.buscarPrincipal = registroDePrueba(sinConcesion)
	s.empujarUnaVez(context.Background(), ahora) // avisa

	s.buscarPrincipal = registroDePrueba(principalDePrometheus())
	s.empujarUnaVez(context.Background(), ahora.Add(30*time.Second)) // se arregla: empuja
	if n := destino.pedidos.Load(); n != 1 {
		t.Fatalf("no se recuperó al devolverle la concesión (%d requests)", n)
	}

	s.buscarPrincipal = registroDePrueba(sinConcesion)
	s.empujarUnaVez(context.Background(), ahora.Add(60*time.Second)) // vuelve a romperse: avisa DE NUEVO

	if n := strings.Count(log.String(), "ya NO tiene ninguna concesión"); n != 2 {
		t.Errorf("avisó %d veces en dos incidentes separados por una recuperación; "+
			"el segundo incidente pasó en silencio", n)
	}
}

// ── A49 · El SOBRE del empuje, no sólo lo que va adentro ─────────────────────────────────────

// Hasta A49 ninguna prueba miraba el método, el path ni un solo header del POST: el receptor leía
// el cuerpo y contaba requests. El verificador borró el `Content-Type` de `enviar` y las cuatro
// suites quedaron en verde — mientras Prometheus, del otro lado, contesta 400 a cada POST y nada
// se pone rojo porque un 400 cae en el `default` que sí cuenta un fallo... media hora después de
// que alguien mire el contador.
//
// Existe una prueba contra un Prometheus DE VERDAD (fleet_otlp_real_test.go) que sí ejercita el
// cable entero, pero es opt-in y no corre en CI. Ésta es la de todos los días.
//
// Sabotaje que la hace fallar: quitar `req.Header.Set("Content-Type", "application/json")`.
// Sabotaje que la hace fallar: cambiar http.MethodPost por http.MethodPut (o MethodGet).
func TestElEmpujeMandaUnPOSTDeJSONAlPathQueSeConfiguro(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL+"/api/v1/otlp/v1/metrics",
		registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	s.empujarUnaVez(context.Background(), ahora)
	if n := destino.pedidos.Load(); n != 1 {
		t.Fatalf("el empuje no llegó (%d requests)", n)
	}
	sobre := destino.ultimoSobre(t)

	if sobre.Metodo != http.MethodPost {
		t.Errorf("el empuje salió como %s; OTLP/HTTP es POST y cualquier otra cosa da 405", sobre.Metodo)
	}
	// EL PATH VIAJA ENTERO. Un `enviar` que arme la URL a mano en vez de usar la configurada
	// manda al host correcto y al endpoint equivocado — que es el 404 que el propio código de
	// error del 404 explica, y el modo de falla más caro de este slice.
	if sobre.Path != "/api/v1/otlp/v1/metrics" {
		t.Errorf("el POST fue a %q y no al path configurado: Prometheus contesta 404 en cualquier otro", sobre.Path)
	}
	if ct := sobre.Headers.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; sin `application/json` el receptor OTLP contesta 400 a cada POST", ct)
	}
	// Sin `auth_token_env` NO se manda un Authorization vacío: un `Bearer ` pelado es peor que la
	// ausencia del header —algunos receptores lo toman como un intento fallido de autenticar y
	// contestan 401 en vez de aceptar la request anónima.
	if a := sobre.Headers.Get("Authorization"); a != "" {
		t.Errorf("salió un Authorization (%q) sin auth_token_env configurado", a)
	}
	// Y el cuerpo es el sobre OTLP, no cualquier JSON: se usa `ultimoCuerpo`, que hasta A49 estaba
	// DEFINIDA Y SIN LLAMAR — el helper para mirar el payload existía y ninguna prueba lo usaba.
	var sobreOTLP struct {
		ResourceMetrics []struct {
			ScopeMetrics []struct {
				Metrics []struct {
					Name string `json:"name"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}
	if err := json.Unmarshal(destino.ultimoCuerpo(t), &sobreOTLP); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v", err)
	}
	if len(sobreOTLP.ResourceMetrics) == 0 {
		t.Fatal("el cuerpo no trae resourceMetrics: el sobre no es OTLP")
	}
	var nombres []string
	for _, rm := range sobreOTLP.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				nombres = append(nombres, m.Name)
			}
		}
	}
	if len(nombres) == 0 {
		t.Fatal("el sobre OTLP llegó sin una sola métrica adentro")
	}
	for _, n := range nombres {
		if !strings.HasPrefix(n, "musubi_") {
			t.Errorf("métrica %q sin el prefijo musubi_: se mezclaría con las del propio Prometheus", n)
		}
	}
}

// El bearer viaja en el header y NUNCA en la URL. La URL de un destino termina en un log de
// diagnóstico el primer día malo —`urlSinSecretos` existe justamente por eso— así que si el
// secreto viajara ahí, taparlo en el log no alcanzaría.
//
// Sabotaje que la hace fallar: quitar el `req.Header.Set("Authorization", ...)` de enviar.
func TestElBearerDelEmpujeViajaEnElHeaderYNoEnLaURL(t *testing.T) {
	const variable = "MUSUBI_TEST_OTLP_TOKEN_A49"
	t.Setenv(variable, "s3cr3t0-del-empuje")

	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL+"/api/v1/otlp/v1/metrics",
		registroDePrueba(principalDePrometheus()), func(c *config.OTLPPushConfig) {
			c.AuthTokenEnv = variable
		})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", muestraSana(40, ahora), ahora)

	s.empujarUnaVez(context.Background(), ahora)
	sobre := destino.ultimoSobre(t)

	if a := sobre.Headers.Get("Authorization"); a != "Bearer s3cr3t0-del-empuje" {
		t.Errorf("Authorization = %q; el destino contesta 401 para siempre y el mensaje manda a "+
			"revisar la variable, que está bien puesta", a)
	}
	if strings.Contains(sobre.Path, "s3cr3t0") {
		t.Error("el secreto viajó en el path: cualquier access log del destino lo guarda en claro")
	}
	// Y el log del propio cerebro no lo repite: urlSinSecretos custodia la otra mitad.
	if u := urlSinSecretos(s.empujador.url); strings.Contains(u, "s3cr3t0") {
		t.Errorf("urlSinSecretos dejó pasar el token: %q", u)
	}
}
