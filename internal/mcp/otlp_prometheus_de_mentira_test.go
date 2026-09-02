package mcp

// Un DOBLE del receptor OTLP de Prometheus: ingiere el sobre como lo ingiere Prometheus —con la
// normalización del nombre por unidad incluida— y contesta /api/v1/query. Existe para que el
// camino de punta a punta se ejercite SIEMPRE, y no sólo cuando alguien tiene un Prometheus 3.x
// encendido con --web.enable-otlp-receiver (que es casi nunca: ver fleet_otlp_real_test.go).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ PUEDE DECIR ESTE DOBLE Y QUÉ NO — LA PARTE HONESTA
//
// NO PUEDE decir que nuestra creencia sobre la especificación sea correcta. Codifica ESA MISMA
// creencia, así que si está equivocada, el doble se equivoca en silencio con nosotros. Para eso
// —y sólo para eso— sigue existiendo la prueba contra un Prometheus de verdad.
//
// SÍ PUEDE decir algo que hoy no dice nadie, y que es exactamente el agujero que el archivo real
// documenta con dos sabotajes medidos: una UNIDAD emitida en el payload que no está en las tablas
// de series. `TestNingunaUnidadRenombraLaSerieEnPrometheus` y `TestNingunaSerieCambiaDeNombre...`
// leen las TABLAS (seriesDeFlota / seriesDeServicio); las veinticuatro pruebas de sobre leen la
// FORMA. Si alguien hardcodea `Unit: "By"` dentro de armarPayloadOTLP —el sabotaje textual del
// encabezado de fleet_otlp_real_test.go— las tablas siguen impecables, el sobre sigue impecable,
// y la serie entra a Prometheus con otro nombre. Las veinticuatro quedan en verde. El doble no:
// renombra igual que el receptor y la consulta se va vacía.
//
// Y sostiene el resto de la cadena COMPUESTA, que hasta ahora sólo se ejercitaba por partes:
// armarPayloadOTLP → nuevoEmpujadorOTLP → POST → nombre y etiquetas consultables.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// pathOTLPDePrometheus es el path que atiende Prometheus con --web.enable-otlp-receiver. Está
// acá y no inline porque el doble contesta 404 en CUALQUIER otro, que es el síntoma exacto de un
// Prometheus sin el flag: si el empuje se configura contra otro path, esta prueba lo dice.
const pathOTLPDePrometheus = "/api/v1/otlp/v1/metrics"

// serieIngerida es una serie tal como quedó DEL OTRO LADO: con el nombre ya normalizado.
type serieIngerida struct {
	Nombre string
	Labels map[string]string
	Valor  string
}

// prometheusDeMentira acepta el empuje y deja las series consultables.
type prometheusDeMentira struct {
	*httptest.Server

	mu     sync.Mutex
	series []serieIngerida
	// quejas son los motivos por los que el doble rechazó o no entendió algo. Se acumulan en vez
	// de fallar en el handler porque un t.Fatalf desde la goroutine del servidor no falla la
	// prueba que lo miró: la deja colgada o la mata sin explicar nada.
	quejas []string
}

func (p *prometheusDeMentira) quejarse(formato string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quejas = append(p.quejas, fmt.Sprintf(formato, args...))
}

// revisarQuejas falla si el doble tuvo algo que decir. Se llama al final de cada prueba que lo
// usa: sin esto, un rechazo del doble se vería como «la serie no está», que manda a buscar el
// problema al lugar equivocado.
func (p *prometheusDeMentira) revisarQuejas(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, q := range p.quejas {
		t.Errorf("el receptor de mentira se quejó: %s", q)
	}
}

// nuevoPrometheusDeMentira levanta el doble y devuelve su URL base (sin path).
func nuevoPrometheusDeMentira(t *testing.T) *prometheusDeMentira {
	t.Helper()
	p := &prometheusDeMentira{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == pathOTLPDePrometheus:
			p.ingerir(w, req)
		case req.URL.Path == "/api/v1/query":
			p.consultar(w, req)
		default:
			// El 404 de un Prometheus sin --web.enable-otlp-receiver, y el de un path mal
			// configurado, se ven igual. Los dos tienen que llegar como 404 a `enviar`, que es
			// quien traduce ese código al mensaje que dice qué hacer.
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(p.Server.Close)
	return p
}

// ingerir hace lo que hace el receptor de Prometheus: valida el sobre y guarda las series con el
// nombre NORMALIZADO.
func (p *prometheusDeMentira) ingerir(w http.ResponseWriter, req *http.Request) {
	if ct := req.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		// El receptor real contesta 415 a un content-type que no es ni JSON ni protobuf. Sin esto
		// el doble aceptaría un POST desnudo y taparía justo la regresión de A49.
		p.quejarse("el POST llegó con Content-Type %q y el receptor OTLP sólo acepta application/json o application/x-protobuf", ct)
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	var doc struct {
		ResourceMetrics []struct {
			ScopeMetrics []struct {
				Metrics []struct {
					Name  string `json:"name"`
					Unit  string `json:"unit"`
					Gauge struct {
						DataPoints []struct {
							Attributes []struct {
								Key   string `json:"key"`
								Value struct {
									StringValue *string `json:"stringValue"`
								} `json:"value"`
							} `json:"attributes"`
							// Sellos y enteros se leen COMO VENGAN. Prometheus 3.1.0 tolera el
							// número donde la especificación pide string —está medido, y está
							// escrito en fleet_otlp_real_test.go—, así que un doble que los
							// rechazara estaría inventando una estrictez que el receptor real no
							// tiene. Esa desviación la caza puntosDelPayload, no esto.
							TimeUnixNano json.RawMessage `json:"timeUnixNano"`
							AsInt        json.RawMessage `json:"asInt"`
							AsDouble     *float64        `json:"asDouble"`
						} `json:"dataPoints"`
					} `json:"gauge"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}
	if err := json.NewDecoder(req.Body).Decode(&doc); err != nil {
		p.quejarse("el cuerpo no es un sobre OTLP/JSON legible: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, rm := range doc.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				nombre, err := nombreNormalizadoPorUnidad(m.Name, m.Unit)
				if err != nil {
					p.quejarse("%v", err)
					continue
				}
				for _, dp := range m.Gauge.DataPoints {
					// Los atributos del RECURSO no viajan como labels: Prometheus los pone en
					// target_info, no en la serie. Sólo los del punto son etiquetas, y por eso el
					// empuje tiene que poner device/project en el punto y no en el recurso.
					labels := map[string]string{}
					for _, a := range dp.Attributes {
						if a.Value.StringValue != nil {
							labels[a.Key] = *a.Value.StringValue
						}
					}
					valor, ok := valorDelPunto(dp.AsInt, dp.AsDouble)
					if !ok {
						p.quejarse("un punto de %q llegó sin asInt ni asDouble: el receptor no tiene qué guardar", m.Name)
						continue
					}
					p.guardar(serieIngerida{Nombre: nombre, Labels: labels, Valor: valor})
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

// valorDelPunto lee el valor tolerando el int como string (la convención de OTLP/JSON) y como
// número (lo que el receptor real acepta igual).
func valorDelPunto(asInt json.RawMessage, asDouble *float64) (string, bool) {
	if len(asInt) > 0 && string(asInt) != "null" {
		txt := strings.Trim(string(asInt), `"`)
		if n, err := strconv.ParseInt(txt, 10, 64); err == nil {
			return strconv.FormatInt(n, 10), true
		}
		return "", false
	}
	if asDouble != nil {
		return strconv.FormatFloat(*asDouble, 'g', -1, 64), true
	}
	return "", false
}

// nombreNormalizadoPorUnidad es LA regla que hace falta emular, y la única razón por la que este
// doble no es un eco de lo que ya prueba fleet_otlp_test.go: el receptor le agrega al nombre la
// forma canónica de la unidad cuando el nombre no termina ya en ella.
//
// La tabla es unidadCanonica, la misma que usa otlp_nombres_test.go y que se verificó contra el
// Prometheus de producción. Una unidad que no esté ahí NO se adivina: se queja, que es lo que
// tiene que pasar el día que alguien agregue una.
func nombreNormalizadoPorUnidad(nombre, unidad string) (string, error) {
	if unidad == "" {
		return nombre, nil
	}
	canon, conocida := unidadCanonica[unidad]
	if !conocida {
		return "", fmt.Errorf("la serie %q llegó con la unidad %q, que este doble no sabe expandir: agregala a unidadCanonica (en otlp_nombres_test.go) después de verificar contra un Prometheus de verdad qué le hace al nombre", nombre, unidad)
	}
	if strings.HasSuffix(nombre, "_"+canon) {
		return nombre, nil
	}
	return nombre + "_" + canon, nil
}

// guardar reemplaza la serie con esos mismos labels, como hace un TSDB con la muestra más nueva.
func (p *prometheusDeMentira) guardar(s serieIngerida) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, vieja := range p.series {
		if vieja.Nombre == s.Nombre && mismosLabels(vieja.Labels, s.Labels) {
			p.series[i] = s
			return
		}
	}
	p.series = append(p.series, s)
}

func mismosLabels(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// nombresIngeridos devuelve, sin repetir, los nombres con los que quedaron las series.
func (p *prometheusDeMentira) nombresIngeridos() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]bool{}
	for _, s := range p.series {
		out[s.Nombre] = true
	}
	return out
}

// selectorInstantaneo parte `metrica{k="v",k2="v2"}`. Es todo el PromQL que estas pruebas usan, y
// cualquier otra cosa se rechaza en vez de contestar vacío: un vacío se lee como «la serie no
// llegó» y manda a buscar el problema al lugar equivocado.
var selectorInstantaneo = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?$`)
var matcherIgual = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"$`)

func (p *prometheusDeMentira) consultar(w http.ResponseWriter, req *http.Request) {
	expr := strings.TrimSpace(req.URL.Query().Get("query"))
	partes := selectorInstantaneo.FindStringSubmatch(expr)
	if partes == nil {
		p.quejarse("no entendí la consulta %q: este doble sólo sabe selectores instantáneos con matchers de igualdad", expr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	quiero := map[string]string{}
	if partes[2] != "" {
		for _, crudo := range strings.Split(partes[2], ",") {
			m := matcherIgual.FindStringSubmatch(strings.TrimSpace(crudo))
			if m == nil {
				p.quejarse("no entendí el matcher %q de la consulta %q: sólo se soporta k=\"v\"", crudo, expr)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			quiero[m[1]] = m[2]
		}
	}

	type muestra struct {
		Metric map[string]string `json:"metric"`
		Value  []any             `json:"value"`
	}
	resultado := []muestra{}
	p.mu.Lock()
	for _, s := range p.series {
		if s.Nombre != partes[1] {
			continue
		}
		coincide := true
		for k, v := range quiero {
			if s.Labels[k] != v {
				coincide = false
				break
			}
		}
		if !coincide {
			continue
		}
		metric := map[string]string{"__name__": s.Nombre}
		for k, v := range s.Labels {
			metric[k] = v
		}
		resultado = append(resultado, muestra{Metric: metric, Value: []any{0, s.Valor}})
	}
	p.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data":   map[string]any{"resultType": "vector", "result": resultado},
	})
}

// ── Que el doble tenga dientes ──────────────────────────────────────────────────────────────

// UNA PRUEBA QUE NO PUEDE FALLAR NO CUENTA, Y UN DOBLE QUE NO PUEDE DECIR QUE NO TAMPOCO.
//
// Si nombreNormalizadoPorUnidad se volviera un `return nombre, nil`, el camino de punta a punta
// seguiría en verde con la unidad saboteada y el doble estaría certificando lo contrario de lo
// que dice el encabezado. Esta prueba le mete a mano el sobre que rompe —una unidad `By` sobre un
// nombre que no termina en `_bytes`— y exige que renombre.
//
// Sabotaje que la hace fallar: hacer que nombreNormalizadoPorUnidad devuelva el nombre tal cual.
func TestElReceptorOTLPDeMentiraRenombraLaSerieCuandoLaUnidadNoEstaEnElNombre(t *testing.T) {
	prom := nuevoPrometheusDeMentira(t)
	sobre := `{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"musubi_fleet_device_up","unit":"By",` +
		`"gauge":{"dataPoints":[{"attributes":[{"key":"device","value":{"stringValue":"pc-gio"}}],` +
		`"timeUnixNano":"1","asInt":"1"}]}}]}]}]}`
	resp, err := http.Post(prom.URL+pathOTLPDePrometheus, "application/json", strings.NewReader(sobre))
	if err != nil {
		t.Fatalf("no se pudo empujar al doble: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("el doble contestó %d a un sobre bien formado", resp.StatusCode)
	}

	if _, ok := consultarPrometheus(t, prom.URL, `musubi_fleet_device_up{device="pc-gio"}`); ok {
		t.Error("el doble dejó consultable `musubi_fleet_device_up` con unidad `By`: el receptor real la habría guardado como `musubi_fleet_device_up_bytes` y la regla que consulta el nombre declarado quedaría MUDA. Sin este renombrado el doble no puede cazar el sabotaje de la unidad")
	}
	if v, ok := consultarPrometheus(t, prom.URL, `musubi_fleet_device_up_bytes{device="pc-gio"}`); !ok || v != "1" {
		t.Errorf("el doble tenía que guardar la serie como `musubi_fleet_device_up_bytes` y devolvió (%q, %v)", v, ok)
	}
	prom.revisarQuejas(t)
}

// Y que el 404 del path equivocado llegue como 404: es el error insignia de este slice, y el doble
// tiene que poder producirlo para que la cadena entera se ejercite contra él.
//
// Sabotaje que la hace fallar: atender cualquier POST en el doble en vez de sólo pathOTLPDePrometheus.
func TestElReceptorOTLPDeMentiraContesta404EnElPathEquivocado(t *testing.T) {
	prom := nuevoPrometheusDeMentira(t)
	resp, err := http.Post(prom.URL+"/api/v1/otlp/metrics", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("no se pudo empujar al doble: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("un POST al path equivocado contestó %d y tiene que contestar 404: es el síntoma de un Prometheus sin --web.enable-otlp-receiver, y `enviar` lo traduce al único mensaje que dice qué hacer", resp.StatusCode)
	}
}
