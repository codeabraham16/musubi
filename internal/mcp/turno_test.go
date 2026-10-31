package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/guiones"
	"musubi/internal/logx"
	"musubi/internal/motor"
)

// motorFalso sirve un turno guionado. No es un mock del protocolo: los eventos reales los
// produce el traductor del paquete motor, que tiene su propia batería. Acá lo que se mide es
// el TRANSPORTE — que los eventos salgan como SSE, en orden, y que un cierre raro se nombre.
type motorFalso struct {
	eventos  []motor.Evento
	err      error
	antesDe  func() // corre antes de emitir, para los tests que necesitan temporizar
	mu       sync.Mutex
	recibido motor.Pedido
}

func (m *motorFalso) Turno(ctx context.Context, p motor.Pedido, emitir func(motor.Evento) error) error {
	m.mu.Lock()
	m.recibido = p
	m.mu.Unlock()
	if m.antesDe != nil {
		m.antesDe()
	}
	for _, ev := range m.eventos {
		if err := emitir(ev); err != nil {
			return err
		}
	}
	return m.err
}

func (m *motorFalso) pedido() motor.Pedido {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recibido
}

// grabadorSSE es un ResponseWriter que además registra lo que `http.ResponseController` le
// pide. Hace falta porque el invariante más caro de este handler —limpiar la deadline de
// escritura— es invisible con un ResponseRecorder pelado: el controller no encuentra el método,
// devuelve ErrNotSupported, el handler lo ignora, y el test pasaría igual con la línea borrada.
type grabadorSSE struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
	// bitacora intercala en ORDEN lo que pasa por el writer: "deadline:cero", "deadline:plazo"
	// y "write". Reemplazó a dos booleanos (`deadlineLimpiada`, `deadlineConPlazo`) que no
	// alcanzaban para el invariante que hace falta —que ninguna escritura salga sin un plazo
	// armado justo antes—, porque un handler que pusiera un plazo al principio y después
	// escribiera diez veces daba los mismos valores que el correcto.
	bitacora []string
	flushes  int
	// avisoFlush, si está, se cierra en el PRIMER flush. Es la forma de esperar a que algo
	// salga al cable sin dormir un rato a ver si ya salió: la prueba del latido necesita
	// saber que el ticker escribió, y un poll con sleeps haría el test lento y flaky.
	avisoFlush  chan struct{}
	unFlushSolo sync.Once
}

func (g *grabadorSSE) SetWriteDeadline(t time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t.IsZero() {
		g.bitacora = append(g.bitacora, "deadline:cero")
	} else {
		g.bitacora = append(g.bitacora, "deadline:plazo")
	}
	return nil
}

// Write se intercepta sólo para anotar el orden. `fmt.Fprintf` hace UN Write por llamada —arma
// el string en su buffer y lo escribe de una—, así que un "write" en la bitácora es una
// escritura del handler y no un pedazo de ella.
func (g *grabadorSSE) Write(p []byte) (int, error) {
	g.mu.Lock()
	g.bitacora = append(g.bitacora, "write")
	g.mu.Unlock()
	return g.ResponseRecorder.Write(p)
}

func (g *grabadorSSE) libro() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.bitacora...)
}

func (g *grabadorSSE) Flush() {
	g.mu.Lock()
	g.flushes++
	g.mu.Unlock()
	g.ResponseRecorder.Flush()
	if g.avisoFlush != nil {
		g.unFlushSolo.Do(func() { close(g.avisoFlush) })
	}
}

func nuevoGrabador() *grabadorSSE {
	return &grabadorSSE{ResponseRecorder: httptest.NewRecorder()}
}

// eventosDelCuerpo parsea un cuerpo SSE y devuelve los eventos del protocolo. Descarta los
// comentarios (los latidos), igual que hace el cliente.
func eventosDelCuerpo(t *testing.T, cuerpo string) []motor.Evento {
	t.Helper()
	var out []motor.Evento
	for _, linea := range strings.Split(cuerpo, "\n") {
		if !strings.HasPrefix(linea, "data: ") {
			continue
		}
		var ev motor.Evento
		if err := json.Unmarshal([]byte(strings.TrimPrefix(linea, "data: ")), &ev); err != nil {
			t.Fatalf("una línea data: no parsea: %v (%q)", err, linea)
		}
		out = append(out, ev)
	}
	return out
}

func pedidoDeTurno(cuerpo string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/stream", strings.NewReader(cuerpo))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// letras es un io.Reader infinito de 'a'. Existe para que las pruebas del tope manden decenas
// de megas sin materializarlos: un `strings.Repeat` de 40 MB cuesta esa memoria DOS veces —el
// relleno y la concatenación que lo mete en el JSON— y lo que se mide no necesita que los bytes
// existan todos a la vez.
type letras struct{}

func (letras) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// pedidoDeTurnoConRelleno arma un pedido válido cuyo contenido son `n` bytes de relleno.
func pedidoDeTurnoConRelleno(n int64) *http.Request {
	cuerpo := io.MultiReader(
		strings.NewReader(`{"model":{"id":"m"},"context":{"messages":[{"role":"user","content":"`),
		io.LimitReader(letras{}, n),
		strings.NewReader(`"}]}}`),
	)
	r := httptest.NewRequest(http.MethodPost, "/api/stream", cuerpo)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// El cuerpo mínimo que manda el cliente real: `{model, context, options}`.
const cuerpoMinimo = `{"model":{"id":"claude-opus-5","maxTokens":64000},
	"context":{"messages":[{"role":"user","content":"hola"}]},
	"options":{"maxTokens":100}}`

// ─────────────────────────── el despacho por método ───────────────────────────

// Sabotaje T1: el POST se atiende aunque no haya motor (panic en el bombeo)
// Se pisa con T17 sobre el mismo `if s.motor == nil`, y son dos guardas: ésta mide QUÉ contesta
// (405 y no un panic del bombeo), T17 mide CUÁNDO (antes de pedir credencial).
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tif s.motor == nil {\n\t\t// EL MISMO 405 QUE ANTES DE QUE ESTO EXISTIERA, y el `Allow` dice sólo GET porque es\n\t\t// la verdad: sin motor configurado, POST no está disponible. Un 503 sería tentador,\n\t\t// pero diría «está pero caído» cuando en realidad nadie lo pidió.\n\t\tw.Header().Set(\"Allow\", \"GET\")\n\t\thttp.Error(w, \"method not allowed\", http.StatusMethodNotAllowed)\n\t\treturn\n\t}"
// arnes: a=""
// arnes: colision_ok="TestUnMotorApagadoRechazaAntesDeMirarLaCredencial"
//
// Sabotaje T2: el Allow anuncia POST aunque no haya motor
// Mismo `de` que T3 y `a` OPUESTO: es la otra mitad del mismo condicional. Ésta lo fuerza a
// anunciar POST siempre; T3 lo fuerza a no anunciarlo nunca. Una sola de las dos dejaría media
// línea sin medir.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\t\tpermitidos := \"GET\"\n\t\t\tif s.motor != nil {\n\t\t\t\tpermitidos = \"GET, POST\"\n\t\t\t}"
// arnes: a="\t\t\tpermitidos := \"GET, POST\""
// arnes: colision_ok="TestConMotorLaRutaAnunciaLosDosMetodos"
func TestSinMotorElPostDeTurnoContesta405YNoLoAnuncia(t *testing.T) {
	// 🔴 EL ESTADO DE TODA INSTALACIÓN DE HOY, y el comportamiento tiene que ser el MISMO que
	// antes de que este endpoint existiera. Un 503 sería tentable, pero diría «está pero
	// caído» cuando en realidad nadie lo configuró.
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/stream", "application/json", strings.NewReader(cuerpoMinimo))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, quiero 405", resp.StatusCode)
	}
	// El `Allow` dice la verdad de ESTE servidor: anunciar POST mandaría al cliente a
	// estrellarse contra el mismo 405 por otra puerta.
	if a := resp.Header.Get("Allow"); a != "GET" {
		t.Errorf("Allow = %q, quiero \"GET\" sin motor configurado", a)
	}

	// 🔴 Y EL MISMO INVARIANTE POR EL OTRO CAMINO, que son dos de verdad y el sabotaje lo
	// descubrió: un POST lo atiende `servirTurno`, que tiene su propio 405 con su propio
	// `Allow`; el bloque de `http.go` sólo corre para los métodos que NO son GET ni POST. Con
	// sólo el POST de arriba, el `Allow` de `http.go` quedaba sin medir en el estado SIN motor
	// —`TestConMotorLaRutaAnunciaLosDosMetodos` cubre el otro—, así que se podía cablear
	// "GET, POST" fijo y la suite entera seguía verde mintiéndole a cualquier cliente que
	// sondee con otro verbo.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/stream", nil)
	otra, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer otra.Body.Close()
	if otra.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: status = %d, quiero 405", otra.StatusCode)
	}
	if a := otra.Header.Get("Allow"); a != "GET" {
		t.Errorf("DELETE: Allow = %q, quiero \"GET\" sin motor configurado", a)
	}
}

// Sabotaje T3: con motor el Allow no anuncia POST (el cliente no sabe que existe)
// Mismo `de` que T2 y `a` OPUESTO: ver el ancla de T2. Ésta es la mitad CON motor.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\t\tpermitidos := \"GET\"\n\t\t\tif s.motor != nil {\n\t\t\t\tpermitidos = \"GET, POST\"\n\t\t\t}"
// arnes: a="\t\t\tpermitidos := \"GET\""
// arnes: colision_ok="TestSinMotorElPostDeTurnoContesta405YNoLoAnuncia"
func TestConMotorLaRutaAnunciaLosDosMetodos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(&motorFalso{})(s)
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, quiero 405", resp.StatusCode)
	}
	if a := resp.Header.Get("Allow"); a != "GET, POST" {
		t.Errorf("Allow = %q, quiero \"GET, POST\" con motor configurado", a)
	}
}

// Sabotaje T4: el POST se come el GET del feed en vivo
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\tif r.Method == http.MethodPost {\n\t\t\ts.servirTurno(opt, w, r)\n\t\t\treturn\n\t\t}"
// arnes: a="\t\tif r.Method == http.MethodPost || r.Method == http.MethodGet {\n\t\t\ts.servirTurno(opt, w, r)\n\t\t\treturn\n\t\t}"
func TestElFeedEnVivoSigueAtendiendoElGetDeLaMismaRuta(t *testing.T) {
	// 🔴 LA REGRESIÓN QUE ESTE CAMBIO PODÍA CAUSAR. `/api/stream` ya estaba tomada por el
	// feed del panel, y el despacho por método se agregó ADENTRO de su handler: una condición
	// mal puesta le roba el GET al panel y el síntoma sería un panel en blanco, no un error.
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(&motorFalso{})(s)
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()

	// 🔴 TODA LECTURA DE ACÁ VA ACOTADA, Y NO ES PRUDENCIA DECORATIVA: el feed es un stream que
	// POR DISEÑO NO TERMINA NUNCA. Una versión anterior de este test hacía `io.ReadAll` del
	// cuerpo para el mensaje de error, y cuando la expectativa resultó equivocada el test no
	// salió rojo: colgó el PAQUETE ENTERO hasta el timeout (medido: 45 s). Un test que se
	// cuelga no informa, y encima acusa al paquete en vez de a sí mismo.
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Get(ts.URL + "/api/stream")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Fatal("el GET del feed se comió un 405: el despacho por método le robó la ruta al panel")
	}
	// 🔴 Afirmar sólo «no es 405» NO alcanza, y el sabotaje lo demostró: un GET desviado a
	// `servirTurno` no da 405 sino 400, porque un GET no trae cuerpo y el pedido no parsea. El
	// panel queda igual de en blanco y el test pasaba igual. Así que se afirma POSITIVAMENTE que
	// contestó EL FEED: su status, su Content-Type, y su primer frame.
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		var buf [256]byte
		n, _ := resp.Body.Read(buf[:]) // UN Read, no ReadAll: devuelve lo que ya llegó
		t.Fatalf("status = %d, Content-Type = %q (%s), quiero el 200 SSE del feed",
			resp.StatusCode, resp.Header.Get("Content-Type"), buf[:n])
	}
	// Y el primer frame del feed es su backlog, que el handler emite y flushea antes de quedarse
	// esperando (http.go:558). Es lo que distingue al feed de cualquier otro 200 de SSE —el
	// stream de un TURNO son `data:` sin nombre de evento— y leer exactamente estos bytes está
	// acotado: ya están en el cable.
	const marca = "event: backlog"
	primero := make([]byte, len(marca))
	if _, err := io.ReadFull(resp.Body, primero); err != nil {
		t.Fatalf("leyendo el primer frame del feed: %v (leído %q)", err, primero)
	}
	if string(primero) != marca {
		t.Errorf("el primer frame es %q, quiero %q: un GET que no llega al feed es un panel en blanco", primero, marca)
	}
}

// ─────────────────────────── el cuerpo del pedido ───────────────────────────

// Sabotaje T5: el error de pedido sale como texto pelado (el cliente pierde el motivo)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\terrorDeTurnoEnJSON(w, http.StatusBadRequest, err)"
// arnes: a="\t\thttp.Error(w, err.Error(), http.StatusBadRequest)"
//
// Sabotaje T6: el JSON de error usa otra clave que la que el cliente lee
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t_ = json.NewEncoder(w).Encode(map[string]string{\"error\": err.Error()})"
// arnes: a="\t_ = json.NewEncoder(w).Encode(map[string]string{\"message\": err.Error()})"
func TestUnPedidoIlegibleContestaJSONConLaClaveQueElClienteLee(t *testing.T) {
	// 🔴 LA FORMA DEL ERROR ES PARTE DEL CONTRATO. El cliente hace
	// `const body = await response.json(); throw new Error(body.error ?? ...)`
	// (proxy.ts:163). Un `http.Error` de texto pelado le deja el mensaje genérico y el motivo
	// real —el único dato útil— se pierde.
	casos := []struct {
		nombre string
		cuerpo string
		busca  string
	}{
		{"no es JSON", `{no`, "no se pudo leer"},
		{"sin model.id", `{"context":{"messages":[{"role":"user","content":"a"}]}}`, "model.id"},
		{"sin mensajes", `{"model":{"id":"m"},"context":{"messages":[]}}`, "context.messages"},
		{"rol desconocido", `{"model":{"id":"m"},"context":{"messages":[{"role":"raro"}]}}`, "rol desconocido"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			WithMotor(&motorFalso{})(s)
			g := nuevoGrabador()
			s.servirTurno(httpOptions{}, g, pedidoDeTurno(c.cuerpo))

			if g.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quiero 400", g.Code)
			}
			if ct := g.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, quiero application/json", ct)
			}
			var cuerpo map[string]string
			if err := json.Unmarshal(g.Body.Bytes(), &cuerpo); err != nil {
				t.Fatalf("la respuesta de error no es JSON: %v (%s)", err, g.Body.String())
			}
			if !strings.Contains(cuerpo["error"], c.busca) {
				t.Errorf("error = %q, esperaba que nombrara %q", cuerpo["error"], c.busca)
			}
		})
	}
}

// Sabotaje T7: el cuerpo rechaza los campos que este servidor no conoce
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tdec := json.NewDecoder(cuerpo)"
// arnes: a="\tdec := json.NewDecoder(cuerpo)\n\tdec.DisallowUnknownFields()"
func TestLosCamposQueEsteServidorNoConoceNoTumbanElPedido(t *testing.T) {
	// 🔴 AL REVÉS DE LO QUE HACE EL RESTO DEL REPO, Y A PROPÓSITO. `/api/flota` usa
	// DisallowUnknownFields porque Musubi escribe los dos lados. Este cuerpo lo escribe un
	// cliente AJENO que manda `temperature`, `samplingParams`, `cacheRetention`, `transport`,
	// `metadata` y el objeto `model` completo. Rechazar lo desconocido haría fallar TODOS los
	// pedidos reales, y volvería a fallar con cada versión nueva del cliente.
	m := &motorFalso{eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(`{
		"model":{"id":"claude-opus-5","name":"Opus","api":"anthropic-messages","provider":"anthropic",
		         "baseUrl":"https://api.anthropic.com","cost":{"input":3,"output":15},
		         "contextWindow":200000,"maxTokens":64000,"reasoning":true},
		"context":{"messages":[{"role":"user","content":"hola"}]},
		"options":{"maxTokens":100,"temperature":0.7,"samplingParams":{"topP":0.9},
		           "cacheRetention":"5m","transport":"sse","metadata":{"sessionId":"x"},
		           "maxRetryDelayMs":5000,"inventadoDelFuturo":true}}`))

	if g.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), quiero 200: un campo desconocido tumbó el pedido", g.Code, g.Body.String())
	}
	if p := m.pedido(); p.Modelo != "claude-opus-5" || p.MaxTokens != 100 {
		t.Errorf("el motor recibió %+v", p)
	}
}

// Sabotaje T8: el transcript traducido no llega al motor (se manda el pedido pelado)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\treturn motor.PedidoDesdeProxy(crudo)"
// arnes: a="\treturn motor.Pedido{Modelo: crudo.Model.ID, MaxTokens: 1}, nil"
func TestElTranscriptTraducidoLlegaAlMotor(t *testing.T) {
	// Cierra el circuito de las dos piezas nuevas: el traductor de entrada corre DENTRO del
	// handler, así que un error de cableado acá daría un motor que recibe un pedido vacío y
	// un turno que contesta sin contexto — sin que nada falle.
	m := &motorFalso{eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(`{
		"model":{"id":"claude-opus-5","maxTokens":64000},
		"context":{"messages":[
			{"role":"system","content":"sé breve","toolsAdded":[{"name":"leer","parameters":{"type":"object"}}]},
			{"role":"user","content":"qué hora es"}]},
		"options":{"reasoning":"low"}}`))

	if g.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", g.Code, g.Body.String())
	}
	p := m.pedido()
	if p.SystemPrompt != "sé breve" {
		t.Errorf("SystemPrompt = %q", p.SystemPrompt)
	}
	if p.Thinking != 2048 {
		t.Errorf("Thinking = %d, quiero 2048 (el budget de 'low' en la tabla de pi)", p.Thinking)
	}
	if !strings.Contains(string(p.Tools), `"input_schema"`) {
		t.Errorf("las tools llegaron sin traducir: %s", p.Tools)
	}
	if len(p.Mensajes) != 1 || p.Mensajes[0].Rol != "user" {
		t.Errorf("Mensajes = %+v", p.Mensajes)
	}
}

// ─────────────────────────── el stream ───────────────────────────

// Sabotaje T9: sin Content-Type de SSE (el cliente no lo lee como stream)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tw.Header().Set(\"Content-Type\", \"text/event-stream\")"
// arnes: a=""
func TestUnTurnoSaleComoSSEConLasCabecerasDelProtocolo(t *testing.T) {
	m := &motorFalso{eventos: []motor.Evento{
		{Type: motor.EvStart},
		{Type: motor.EvTextStart},
		{Type: motor.EvTextDelta, Delta: "hola"},
		{Type: motor.EvTextEnd},
		{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{Input: 10, Output: 2}},
	}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	if g.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", g.Code, g.Body.String())
	}
	for cab, quiero := range map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-store",
		"Connection":        "keep-alive",
		"X-Accel-Buffering": "no",
	} {
		if tiene := g.Header().Get(cab); tiene != quiero {
			t.Errorf("%s = %q, quiero %q", cab, tiene, quiero)
		}
	}

	evs := eventosDelCuerpo(t, g.Body.String())
	quieroTipos := []string{motor.EvStart, motor.EvTextStart, motor.EvTextDelta, motor.EvTextEnd, motor.EvDone}
	if len(evs) != len(quieroTipos) {
		t.Fatalf("salieron %d eventos, quiero %d: %s", len(evs), len(quieroTipos), g.Body.String())
	}
	for i, q := range quieroTipos {
		if evs[i].Type != q {
			t.Errorf("evento %d = %q, quiero %q", i, evs[i].Type, q)
		}
	}
	// El usage del `done` tiene que llegar íntegro: es el contador del cliente.
	if evs[4].Usage == nil || evs[4].Usage.Input != 10 || evs[4].Usage.Output != 2 {
		t.Errorf("usage del done = %+v", evs[4].Usage)
	}
	// Cada evento se baja con Flush: sin eso el stream llega entero al final y deja de ser
	// un stream, que es todo el punto.
	g.mu.Lock()
	flushes := g.flushes
	g.mu.Unlock()
	if flushes < len(quieroTipos) {
		t.Errorf("hubo %d flushes para %d eventos: el stream se está bufferando", flushes, len(quieroTipos))
	}
}

// Sabotaje T10: la deadline de escritura del server queda puesta (muere a los 90 s)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t_ = rc.SetWriteDeadline(time.Time{})"
// arnes: a=""
func TestElTurnoLimpiaLaDeadlineDeEscrituraDelServer(t *testing.T) {
	// 🔴 SIN ESTO EL TURNO MUERE A LOS 90 s. El WriteTimeout del server está calibrado para
	// las respuestas cortas de /mcp (timeout + 30 s), y un turno con thinking largo lo pasa.
	// El síntoma no sería un error: el cliente ve el stream cortado a mitad de una frase, y
	// eso es imposible de diagnosticar desde el otro lado.
	//
	// ⚠️ ESTA PRUEBA EXIGÍA QUE NO HUBIERA NINGÚN PLAZO, Y ASÍ PROHIBÍA LA CURA DEL OTRO
	// AGUJERO. Decía, textual, que poner una deadline con plazo «es justo lo que hay que
	// evitar» — y confundía dos relojes distintos: el ABSOLUTO del server, que vence aunque el
	// stream salga perfecto y por eso se limpia, y el RODANTE de cada escritura, que es lo
	// único que evita que un cliente que no lee retenga el turno para siempre. Lo que se mide
	// ahora es lo que de verdad importa: **el primer movimiento sobre el reloj es borrar el
	// absoluto**, antes de cualquier escritura. Que después haya plazos es correcto, y tiene su
	// propia prueba más abajo.
	m := &motorFalso{eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	libro := g.libro()
	if len(libro) == 0 || libro[0] != "deadline:cero" {
		t.Errorf("el primer movimiento del handler fue %q (bitácora %v), quiero \"deadline:cero\": "+
			"si el absoluto del server no se borra ANTES de escribir, el turno se corta a los "+
			"90 s a mitad de una frase", primero(libro), libro)
	}
}

// primero y loAnterior sirven para que el mensaje de error se pueda leer sin indexar fuera de
// rango justo cuando la bitácora vino vacía, que es uno de los modos de falla.
func primero(xs []string) string {
	if len(xs) == 0 {
		return "(nada)"
	}
	return xs[0]
}

func loAnterior(xs []string, i int) string {
	if i <= 0 {
		return "(el principio del stream)"
	}
	return xs[i-1]
}

// 🔴 EL CLIENTE QUE NO LEE SE QUEDABA CON EL TURNO ENTERO. `escribir` no armaba ningún plazo, y
// el absoluto del server ya estaba borrado dos líneas antes, así que con los buffers TCP llenos
// el Write bloqueaba SIN TECHO: el bombeo dejaba de recibir del canal —que va sin buffer—, la
// goroutine del motor se quedaba dentro de `emitir`, y `Turno` no llegaba ni a sus defers.
// Medido con un plazo de turno de 3 s: a los 6,5 s seguían vivas `{watchCtx=1, Turno=2,
// Serve=1}` goroutines y un `musubi-mcp-*` en disco con su token, y lo único que los liberaba
// era el cancel del ctx — que un cliente que no lee pero tampoco cierra nunca dispara.
//
// LO QUE SE MIDE ACÁ ES QUE EL PLAZO SE ARMA, no que desbloquee. Que un deadline vencido corte
// un Write trabado lo hace `net.Conn`, y sobre un writer de test no hay socket que cortar: un
// grabador que bloqueara seguiría bloqueado con plazo y sin él. Misma frontera que T10, que mide
// que el absoluto se borre y no que el turno sobreviva noventa segundos.
//
// 🔑 SE EXIGE EL ORDEN, NO UN CONTEO. Un handler que armara un plazo al principio y después
// escribiera diez veces daría los mismos totales que el correcto, y es justo la variante rota:
// el plazo tiene que ser RODANTE o el primer Write trabado se come el resto del turno.
//
// Sabotaje T31: los eventos salen sin plazo (el cliente que no lee retiene el turno)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\tplazoDeEscritura()\n\t\tif _, err := fmt.Fprintf(w, \"data: %s\\n\\n\", b); err != nil {"
// arnes: a="\t\tif _, err := fmt.Fprintf(w, \"data: %s\\n\\n\", b); err != nil {"
//
// Sabotaje T32: el latido sale sin plazo (el camino que nadie mira retiene el turno)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tplazoDeEscritura()\n\t\t\tif _, err := fmt.Fprint(w, \": latido\\n\\n\"); err != nil {"
// arnes: a="\t\t\tif _, err := fmt.Fprint(w, \": latido\\n\\n\"); err != nil {"
//
// Sabotaje T33: el plazo se estira tanto que deja de ser un plazo
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="const plazoDeEscrituraDelTurno = 30 * time.Second"
// arnes: a="const plazoDeEscrituraDelTurno = 24 * time.Hour"
func TestCadaEscrituraDelTurnoLlevaSuPlazo(t *testing.T) {
	// VARIOS eventos, porque con uno solo no se distingue «un plazo por escritura» de «un
	// plazo y listo».
	m := &motorFalso{eventos: []motor.Evento{
		{Type: motor.EvTextDelta, Delta: "ho"},
		{Type: motor.EvTextDelta, Delta: "la"},
		{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}},
	}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	libro := g.libro()
	escrituras := 0
	for i, paso := range libro {
		if paso != "write" {
			continue
		}
		escrituras++
		if i == 0 || libro[i-1] != "deadline:plazo" {
			t.Errorf("la escritura %d salió después de %q y no de un plazo (bitácora %v): sin "+
				"plazo rodante, un cliente con los buffers llenos traba ese Write para siempre "+
				"y se queda con el turno, su hijo y su espejo", escrituras, loAnterior(libro, i), libro)
		}
	}
	if escrituras < len(m.eventos) {
		t.Errorf("salieron %d escrituras para %d eventos (bitácora %v): el test no ejercitó lo "+
			"que dice medir", escrituras, len(m.eventos), libro)
	}

	// Y EL LATIDO ES EL OTRO CAMINO AL CABLE, con su propia línea y su propio olvido posible.
	// Va por `bombearTurno` directo para no esperar los veinte segundos de la constante.
	lento := &motorFalso{
		antesDe: func() { time.Sleep(80 * time.Millisecond) },
		eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}},
	}
	g2 := nuevoGrabador()
	bombearTurno(context.Background(), lento, motor.Pedido{}, g2, http.NewResponseController(g2), time.Millisecond)

	libro2 := g2.libro()

	// EL CONTROL SE CUENTA POR LA BITÁCORA, NO POR EL TEXTO DEL LATIDO, y la diferencia la encontró
	// el censo de colisiones. Contarlo con `strings.Count(cuerpo, ": latido")` ataba este test a la
	// FORMA del latido, que es el invariante de OTRO (T19, sobre esta misma línea de `turno.go`).
	// Medido: con el latido emitido como evento `data:` en vez de comentario, este test caía con la
	// bitácora en 79 pares `deadline:plazo write` PERFECTOS — acusaba al plazo por un daño a la
	// forma, que es justo el «la otra, si cae, puede estar cayendo por el daño al corpus» del aviso.
	// La bitácora sí es de este test: el excedente de escrituras sobre los eventos son los latidos,
	// cualquiera sea su sintaxis.
	escrituras2 := 0
	for _, paso := range libro2 {
		if paso == "write" {
			escrituras2++
		}
	}
	if escrituras2 <= len(lento.eventos) {
		t.Fatalf("salieron %d escritura(s) para %d evento(s): el motor lento no dejó latir, así que "+
			"el test no ejercitó el camino del latido (bitácora %v)",
			escrituras2, len(lento.eventos), libro2)
	}
	for i, paso := range libro2 {
		if paso == "write" && (i == 0 || libro2[i-1] != "deadline:plazo") {
			t.Errorf("con %d escritura(s) para %d evento(s) —el resto son latidos—, una salió sin "+
				"plazo armado antes (bitácora %v): el latido escribe al mismo socket que los "+
				"eventos y se olvidó su línea", escrituras2, len(lento.eventos), libro2)
			break
		}
	}

	// EL VALOR, con literales propios: nombrar `plazoDeEscrituraDelTurno` en su propia cota la
	// vuelve vacua, que es el defecto que esta vuelta encontró dos veces.
	const pisoDelPlazo = 5 * time.Second
	const techoDelPlazo = 5 * time.Minute
	if plazoDeEscrituraDelTurno < pisoDelPlazo {
		t.Errorf("plazoDeEscrituraDelTurno = %v, por debajo del piso de %v: a esa escala una red "+
			"mala le corta el turno a un cliente que sí está leyendo",
			plazoDeEscrituraDelTurno, pisoDelPlazo)
	}
	if plazoDeEscrituraDelTurno > techoDelPlazo {
		t.Errorf("plazoDeEscrituraDelTurno = %v, por encima del techo de %v: el plazo existe para "+
			"que una escritura trabada no retenga el turno, y con ese valor lo retiene igual",
			plazoDeEscrituraDelTurno, techoDelPlazo)
	}
}

// Sabotaje T12: un cierre anomalo no emite nada (el cliente no sabe por que corto)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tescribir(eventoDeCierreAnomalo(ctx, err))"
// arnes: a=""
//
// Sabotaje T15: el usage del cierre anomalo va nil (rompe el tipo del cliente)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tuso := motor.Usage{}\n\treturn motor.Evento{\n\t\tType:         motor.EvError,\n\t\tReason:       razon,\n\t\tErrorMessage: err.Error(),\n\t\tUsage:        &uso,\n\t}"
// arnes: a="\treturn motor.Evento{\n\t\tType:         motor.EvError,\n\t\tReason:       razon,\n\t\tErrorMessage: err.Error(),\n\t}"
func TestUnCierreAnomaloEmiteUnErrorPropioConElMotivo(t *testing.T) {
	// 🔴 EL CLIENTE NO TIENE OTRA FORMA DE SABERLO. El 200 ya salió con las cabeceras, así
	// que el status no se puede cambiar; y un stream que se corta sin evento terminal le deja
	// el fallback genérico de desconexión, que no dice nada. Este evento es la única traza
	// del motivo real que llega al otro lado.
	m := &motorFalso{
		eventos: []motor.Evento{{Type: motor.EvStart}, {Type: motor.EvTextDelta, Delta: "empecé"}},
		err:     fmt.Errorf("el proceso murió sin respuesta del modelo"),
	}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	evs := eventosDelCuerpo(t, g.Body.String())
	if len(evs) == 0 {
		t.Fatal("no salió ningún evento")
	}
	ultimo := evs[len(evs)-1]
	if ultimo.Type != motor.EvError {
		t.Fatalf("el último evento = %q, quiero %q", ultimo.Type, motor.EvError)
	}
	if ultimo.Reason != motor.ReasonError {
		t.Errorf("reason = %q, quiero %q", ultimo.Reason, motor.ReasonError)
	}
	if !strings.Contains(ultimo.ErrorMessage, "murió sin respuesta") {
		t.Errorf("errorMessage = %q, perdió el motivo real", ultimo.ErrorMessage)
	}
	// El `usage` del `error` es OBLIGATORIO en el protocolo (proxy.ts:44): omitirlo rompe el
	// tipo del otro lado, aunque el turno no haya llegado a reportar consumo.
	if ultimo.Usage == nil {
		t.Error("el evento error salió SIN usage, y el protocolo lo declara obligatorio")
	}
}

// Sabotaje T13: un turno que cerro bien agrega un segundo evento terminal
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\tcase err := <-hecho:\n\t\t\tif err == nil {"
// arnes: a="\t\tcase err := <-hecho:\n\t\t\tif false {"
func TestUnTurnoQueCierraBienNoAgregaUnSegundoEventoTerminal(t *testing.T) {
	// El `done` lo emite el traductor, no el handler. Si el handler agregara su propio evento
	// terminal, el cliente vería dos cierres para un turno y el segundo llegaría después de
	// que ya dio el turno por terminado.
	m := &motorFalso{eventos: []motor.Evento{
		{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}},
	}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	evs := eventosDelCuerpo(t, g.Body.String())
	if len(evs) != 1 || evs[0].Type != motor.EvDone {
		t.Errorf("salieron %d eventos (%s), quiero sólo el done del traductor", len(evs), g.Body.String())
	}
}

// Sabotaje T14: el cancel del cliente se reporta como falla del servidor
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\trazon := motor.ReasonError\n\tif errors.Is(ctx.Err(), context.Canceled) {\n\t\trazon = motor.ReasonAborted\n\t}"
// arnes: a="\trazon := motor.ReasonError\n\tif errors.Is(ctx.Err(), context.DeadlineExceeded) {\n\t\trazon = motor.ReasonAborted\n\t}"
func TestElCancelDelClienteSeNombraAbortadoYNoComoFalla(t *testing.T) {
	// La diferencia importa del lado del cliente: `aborted` es «vos lo cortaste» y no merece
	// un cartel de error, mientras que `error` es «se rompió». Mezclarlos hace que cada Esc
	// del usuario se vea como una falla del servidor.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ev := eventoDeCierreAnomalo(ctx, context.Canceled)
	if ev.Reason != motor.ReasonAborted {
		t.Errorf("con el ctx cancelado: reason = %q, quiero %q", ev.Reason, motor.ReasonAborted)
	}
	// Y la contraprueba, que es la mitad que un test de un solo caso no mide: sin cancel, el
	// mismo cierre tiene que salir como falla.
	sano := eventoDeCierreAnomalo(context.Background(), fmt.Errorf("se rompió"))
	if sano.Reason != motor.ReasonError {
		t.Errorf("sin cancel: reason = %q, quiero %q", sano.Reason, motor.ReasonError)
	}
}

// ─────────────────────────── auth y tope ───────────────────────────

// Sabotaje T16: el turno no pide credencial (la operacion mas cara, sin puerta)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tif opt.registry != nil {\n\t\tif _, ok := autenticarPersona(opt, w, r); !ok {\n\t\t\treturn\n\t\t}\n\t} else if opt.token != \"\" && !validBearer(r.Header.Get(\"Authorization\"), opt.token) {\n\t\tw.Header().Set(\"WWW-Authenticate\", \"Bearer\")\n\t\thttp.Error(w, \"unauthorized\", http.StatusUnauthorized)\n\t\treturn\n\t}"
// arnes: a=""
func TestElTurnoExigeLaMismaCredencialQueElFeed(t *testing.T) {
	// 🔴 SERVIR UN TURNO ES LA OPERACIÓN MÁS CARA DEL SERVIDOR: gasta la suscripción o los
	// tokens del usuario. Una puerta más floja acá que en el feed de LECTURA sería al revés
	// de como tiene que ser, y el costo de la diferencia lo paga el dueño del token.
	m := &motorFalso{eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)
	opt := httpOptions{token: "el-secreto"}

	sinToken := nuevoGrabador()
	s.servirTurno(opt, sinToken, pedidoDeTurno(cuerpoMinimo))
	if sinToken.Code != http.StatusUnauthorized {
		t.Errorf("sin token: status = %d, quiero 401", sinToken.Code)
	}
	if a := sinToken.Header().Get("WWW-Authenticate"); a != "Bearer" {
		t.Errorf("sin token: WWW-Authenticate = %q, quiero \"Bearer\"", a)
	}

	malToken := nuevoGrabador()
	rMal := pedidoDeTurno(cuerpoMinimo)
	rMal.Header.Set("Authorization", "Bearer otro")
	s.servirTurno(opt, malToken, rMal)
	if malToken.Code != http.StatusUnauthorized {
		t.Errorf("con token equivocado: status = %d, quiero 401", malToken.Code)
	}

	bien := nuevoGrabador()
	rBien := pedidoDeTurno(cuerpoMinimo)
	rBien.Header.Set("Authorization", "Bearer el-secreto")
	s.servirTurno(opt, bien, rBien)
	if bien.Code != http.StatusOK {
		t.Errorf("con el token correcto: status = %d (%s), quiero 200", bien.Code, bien.Body.String())
	}
}

// Sabotaje T17: el 405 por motor apagado pide credencial primero
// Se pisa con T1 sobre el mismo `if s.motor == nil`: T1 mide QUÉ contesta, ésta mide que lo
// conteste ANTES de `autenticarPersona` — sin la guarda el pedido cae al 401 y el cliente cree que
// le falta credencial para una ruta que no existe.
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tif s.motor == nil {\n\t\t// EL MISMO 405 QUE ANTES DE QUE ESTO EXISTIERA, y el `Allow` dice sólo GET porque es\n\t\t// la verdad: sin motor configurado, POST no está disponible. Un 503 sería tentador,\n\t\t// pero diría «está pero caído» cuando en realidad nadie lo pidió.\n\t\tw.Header().Set(\"Allow\", \"GET\")\n\t\thttp.Error(w, \"method not allowed\", http.StatusMethodNotAllowed)\n\t\treturn\n\t}\n\n"
// arnes: a=""
// arnes: colision_ok="TestSinMotorElPostDeTurnoContesta405YNoLoAnuncia"
func TestUnMotorApagadoRechazaAntesDeMirarLaCredencial(t *testing.T) {
	// El orden importa: sin motor no hay nada que proteger, y pedir credencial para contestar
	// un 405 le dice a quien sondea que ahí hay algo. El 405 es la respuesta de una ruta que
	// no existe para ese método, y así tiene que leerse.
	s := newTestServer(t, embedding.NoopProvider{})
	g := nuevoGrabador()
	s.servirTurno(httpOptions{token: "el-secreto"}, g, pedidoDeTurno(cuerpoMinimo))
	if g.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, quiero 405 (no 401)", g.Code)
	}
}

// 🔴 EL TOPE TIENE DOS LADOS Y LOS DOS SON LOAD-BEARING, así que los dos se miden acá. Su
// comentario ([turno.go:31-38](turno.go#L31-L38)) declara las dos cosas: que existe, porque uno
// infinito deja que un cliente roto se coma la memoria del proceso —y este proceso es el que
// tiene la memoria de todos los proyectos—, y que es GENEROSO, porque lo que viaja es un
// transcript completo con capturas en base64 y un tope apretado no se ve como un límite sino
// como un agente que deja de funcionar cuando la charla se pone larga.
//
// 🔑 LOS DOS LITERALES SON PROPIOS Y NINGUNO NOMBRA `topeDelPedidoDeTurno`. Esta prueba armaba
// el cuerpo con `topeDelPedidoDeTurno+1024`, o sea se medía contra la constante que vigila: bajá
// el tope a 1 MB y el relleno baja con él, el 400 sigue saliendo y el verde no informa nada. Es
// el mismo defecto que tenía el margen del idle del espejo, escrito de nuevo — una aserción que
// se deriva de su objetivo no lo mide, lo acompaña.
//
// Sabotaje T18: el cuerpo del pedido no tiene tope (un cliente roto come la memoria)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tcuerpo := http.MaxBytesReader(w, r.Body, topeDelPedidoDeTurno)"
// arnes: a="\tcuerpo := r.Body"
//
// Sabotaje T26: el tope sube tanto que deja de ser un tope
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="const topeDelPedidoDeTurno = 32 << 20"
// arnes: a="const topeDelPedidoDeTurno = 1 << 30"
// arnes: colision_ok="TestUnCuerpoGiganteSeCortaEnVezDeComerseLaMemoria"
//
// Sabotaje T27: el tope se aprieta y la charla larga deja de entrar
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="const topeDelPedidoDeTurno = 32 << 20"
// arnes: a="const topeDelPedidoDeTurno = 1 << 20"
// arnes: colision_ok="TestUnCuerpoGiganteSeCortaEnVezDeComerseLaMemoria"
//
// ⚠️ T26 Y T27 SE PISAN a propósito: atacan la MISMA línea en direcciones opuestas, que es
// justo lo que hace falta para encerrar un número entre un piso y un techo. Viven sobre la misma
// prueba, así que su `colision_ok` la nombra una vez y el censo valida que conteste de verdad.
func TestUnCuerpoGiganteSeCortaEnVezDeComerseLaMemoria(t *testing.T) {
	m := &motorFalso{eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}}}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	// EL TECHO. Por encima de cualquier tope sano, armado como el cuerpo real para que el
	// decodificador tenga que leerlo de verdad antes de rendirse.
	const porEncimaDeCualquierTopeSano = 40 << 20
	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurnoConRelleno(porEncimaDeCualquierTopeSano))
	if g.Code != http.StatusBadRequest {
		t.Errorf("un cuerpo de %d B contestó %d, quiero 400: sin tope efectivo un cliente roto "+
			"se come la memoria del proceso que guarda todos los proyectos",
			porEncimaDeCualquierTopeSano, g.Code)
	}

	// EL PISO. Una charla larga con capturas pegadas: tiene que ENTRAR. Esta mitad no existía,
	// y es la que convierte «hay un tope» en «hay un tope del tamaño que el servicio necesita».
	//
	// 🔑 SE EXIGE EL `done`, NO UN 200. `httptest.ResponseRecorder` nace con `Code` en 200, así
	// que un handler que no escribiera nada pasaría un `g2.Code != 200` sin haber corrido: la
	// aserción diría «no falló» donde hace falta que diga «funcionó». El evento terminal sólo
	// existe si el pedido se decodificó, el motor corrió y el bombeo escribió al cable.
	const unaCharlaLargaConCapturas = 8 << 20
	g2 := nuevoGrabador()
	s.servirTurno(httpOptions{}, g2, pedidoDeTurnoConRelleno(unaCharlaLargaConCapturas))
	evs := eventosDelCuerpo(t, g2.Body.String())
	if len(evs) != 1 || evs[0].Type != motor.EvDone {
		t.Errorf("un cuerpo de %d B (status %d) dio %d evento(s) %+v, quiero un solo `done`: el "+
			"tope quedó tan apretado que una conversación larga con capturas ya no entra, y eso "+
			"no se ve como un límite sino como un agente que deja de andar cuando la charla se "+
			"alarga", unaCharlaLargaConCapturas, g2.Code, len(evs), evs)
	}
}

// ─────────────────────────── el latido ───────────────────────────

func TestElLatidoViajaComoComentarioYNoComoEvento(t *testing.T) {
	// 🔴 POR QUÉ COMENTARIO Y NO EVENTO. El parser del cliente descarta toda línea que no
	// empiece con "data: " (proxy.ts:177), así que un latido-comentario no puede confundirse
	// con un evento del protocolo ni cuando el cliente cambie. Un latido mandado como
	// `data: {}` sí entraría al switch de eventos del cliente, y ahí el daño depende de su
	// versión — exactamente la clase de acoplamiento que conviene no tener.
	//
	// Acá se verifica la FORMA de la línea contra el mismo parseo que hace el cliente, en vez
	// de esperar 20 s a que el ticker dispare. El intervalo se mide aparte, abajo.
	g := nuevoGrabador()
	rc := http.NewResponseController(g)
	fmt.Fprint(g, ": latido\n\n")
	_ = rc.Flush()

	if evs := eventosDelCuerpo(t, g.Body.String()); len(evs) != 0 {
		t.Errorf("el latido se parseó como %d evento(s) del protocolo: %+v", len(evs), evs)
	}
	// Y la contraprueba: el mismo parseo SÍ tiene que ver un evento de verdad, o el test
	// anterior pasaría con un parser roto.
	fmt.Fprint(g, "data: {\"type\":\"done\"}\n\n")
	if evs := eventosDelCuerpo(t, g.Body.String()); len(evs) != 1 || evs[0].Type != motor.EvDone {
		t.Errorf("el parseo no vio el evento real: %+v", evs)
	}
}

// 🔴 EL VALOR DEL LATIDO NO LO MIRABA NADIE, NI EL CABLE QUE LO LLEVA. `bombearTurno` recibe el
// latido por parámetro —a propósito, para que un test pueda medir el camino sin esperar veinte
// segundos—, y todas las pruebas de arriba le pasan el suyo. El efecto lateral es que
// `latidoDelTurno` tiene UN solo call site real, [turno.go:106](turno.go#L106), y hasta acá
// ningún test lo recorría: la constante se podía mutar a cualquier cosa, y la línea que la pasa
// podía dejar de pasarla, sin que nada se pusiera rojo.
//
// SE MIDEN LAS DOS COSAS, porque fallan distinto:
//   - el CABLE: que `servirTurno` pase la constante y no un valor suelto. Se ve de punta a
//     punta: con un motor que tarda y la constante en veinte segundos, el cuerpo no puede
//     traer un solo latido.
//   - el VALOR: que la constante esté en una escala sana. Un latido por debajo del piso
//     inunda el socket con una escritura y un flush por nada mientras el motor piensa; uno por
//     encima del techo deja la conexión muda más de lo que un intermediario tolera, que es
//     exactamente lo que el latido existe para evitar.
//
// 🔑 El piso y el techo son literales PROPIOS: una aserción derivada de `latidoDelTurno` se
// mueve con ella y no mide nada (ver la prueba del tope, más arriba, donde ese defecto estaba
// vivo). Y no se sabotea la constante a cero aunque rompería la aserción, porque un
// `time.NewTicker(0)` PANIQUEA, y ese panic nace en la goroutine del handler —fuera del recover
// que cubre al motor— así que mataría el binario de test sin dejar un `--- FAIL`: la falla 5
// del arnés. Las dos mutaciones de abajo se quedan del lado seguro del cero.
//
// Sabotaje T28: servirTurno deja de pasar la constante y late a destiempo
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\tbombearTurno(r.Context(), s.motor, pedido, w, rc, latidoDelTurno)"
// arnes: a="\tbombearTurno(r.Context(), s.motor, pedido, w, rc, time.Millisecond)"
//
// Sabotaje T29: el latido se estira tanto que la conexión queda muda
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="const latidoDelTurno = 20 * time.Second"
// arnes: a="const latidoDelTurno = 5 * time.Minute"
// arnes: colision_ok="TestElLatidoSaleDeLaConstanteYNoDeUnValorSuelto"
//
// Sabotaje T30: el latido se acelera y escribe al socket por nada
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="const latidoDelTurno = 20 * time.Second"
// arnes: a="const latidoDelTurno = 20 * time.Nanosecond"
// arnes: colision_ok="TestElLatidoSaleDeLaConstanteYNoDeUnValorSuelto"
func TestElLatidoSaleDeLaConstanteYNoDeUnValorSuelto(t *testing.T) {
	// EL CABLE. El motor tarda lo suficiente para que un latido mal cableado dispare, y
	// muchísimo menos que la constante, para que el bien cableado no pueda.
	const loQueTardaElMotor = 60 * time.Millisecond
	m := &motorFalso{
		antesDe: func() { time.Sleep(loQueTardaElMotor) },
		eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}},
	}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	// LOS LATIDOS SE CUENTAN POR LA BITÁCORA Y NO POR SU TEXTO. Contarlos con
	// `strings.Count(cuerpo, ": latido")` ataba este test a la FORMA del latido, que es el
	// invariante de otro (T19): el día que la forma cambiara, este contador daría 0, la aserción
	// pasaría trivialmente y la guarda quedaría VACUA sin ponerse roja — que es peor que caerse.
	// El excedente de escrituras sobre los eventos son los latidos, cualquiera sea su sintaxis.
	escrituras := 0
	for _, paso := range g.libro() {
		if paso == "write" {
			escrituras++
		}
	}
	if escrituras > len(m.eventos) {
		t.Errorf("un turno de %v dejó %d escritura(s) para %d evento(s) —el resto son latidos—: "+
			"con la constante en %v no puede latir ni una vez, así que el valor que llega al "+
			"bombeo no es el de la constante",
			loQueTardaElMotor, escrituras, len(m.eventos), latidoDelTurno)
	}

	// EL VALOR, con literales propios.
	const pisoDelLatido = time.Second
	const techoDelLatido = time.Minute
	if latidoDelTurno < pisoDelLatido {
		t.Errorf("latidoDelTurno = %v, por debajo del piso de %v: cada latido es una escritura "+
			"y un flush sobre la conexión, y a esa escala son miles por segundo mientras el "+
			"motor piensa, por nada", latidoDelTurno, pisoDelLatido)
	}
	if latidoDelTurno > techoDelLatido {
		t.Errorf("latidoDelTurno = %v, por encima del techo de %v: el latido existe para que la "+
			"conexión no quede muda mientras el motor piensa, y con ese intervalo queda muda "+
			"más de lo que un intermediario tolera", latidoDelTurno, techoDelLatido)
	}
}

// Sabotaje T19: el latido sale como evento en vez de comentario
//
// La línea del latido la cubren DOS guardas con motivos distintos, y por eso se contesta la
// colisión en vez de elegir una: ésta mira la FORMA (comentario SSE y no un evento `data:` que el
// cliente del protocolo no sabe leer), y T32 mira el PLAZO de esa misma escritura. La respuesta va
// acá y no en T32 porque la colisión es DIRECCIONAL: este `a` destruye el literal `": latido\n\n"`
// que el `de` de T32 contiene, mientras que el `a` de T32 lo conserva. El censo denunció una sola
// dirección; contestar la otra sería afirmar que alguien miró algo que nadie miró.
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tif _, err := fmt.Fprint(w, \": latido\\n\\n\"); err != nil {"
// arnes: a="\t\t\tif _, err := fmt.Fprint(w, \"data: {\\\"type\\\":\\\"latido\\\"}\\n\\n\"); err != nil {"
// arnes: colision_ok="TestCadaEscrituraDelTurnoLlevaSuPlazo"
//
// Sabotaje T20: el latido no se flushea (queda en el buffer, no late)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tif rc.Flush() != nil {\n\t\t\t\treturn\n\t\t\t}"
// arnes: a=""
func TestElLatidoNoEsperaAQueElMotorEmita(t *testing.T) {
	// Lo que se mide es que el latido NO dependa de que el motor emita: es justo cuando el
	// motor está callado —pensando— que la conexión parece muerta para los intermedios, y es
	// justo entonces cuando el latido tiene que salir.
	//
	// 🔴 Y DE PASO MIDE LA RAZÓN POR LA QUE EL MOTOR CORRE EN SU PROPIA GOROUTINE: un
	// `http.ResponseWriter` NO es seguro para uso concurrente. Si el motor escribiera desde la
	// suya mientras el ticker escribe desde ésta, sería una carrera de datos. Acá el ticker
	// escribe mientras el motor está vivo y bloqueado, que es la ventana donde eso pasaría.
	//
	// El intervalo entra por parámetro en vez de leer `latidoDelTurno`: medir el camino real
	// con la constante serían 20 s de espera, y bajarla con una variable de paquete sería una
	// carrera el día que dos tests de este archivo corran en paralelo.
	primerFlush := make(chan struct{})
	liberar := make(chan struct{})
	soltar := sync.OnceFunc(func() { close(liberar) })
	defer soltar() // que el motor nunca quede bloqueado para siempre, ni en el camino de fallo
	termino := make(chan struct{})

	m := &motorFalso{
		antesDe: func() { <-liberar },
		eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}},
	}
	g := nuevoGrabador()
	g.avisoFlush = primerFlush
	rc := http.NewResponseController(g)

	go func() {
		defer close(termino)
		bombearTurno(context.Background(), m, motor.Pedido{}, g, rc, 5*time.Millisecond)
	}()

	select {
	case <-primerFlush:
		// Algo salió al cable con el motor todavía callado, y lo único que puede haber
		// escrito es el ticker.
	case <-termino:
		t.Fatal("el bombeo terminó sin que saliera un solo latido")
	case <-time.After(5 * time.Second):
		t.Fatal("ningún latido en 5 s con un intervalo de 5 ms: el ticker no corre mientras el motor piensa")
	}

	soltar()
	select {
	case <-termino:
	case <-time.After(5 * time.Second):
		t.Fatal("el bombeo no volvió después de que el motor emitiera su done")
	}

	// El cuerpo se lee RECIÉN ACÁ: el canal `termino` da el happens-before contra las
	// escrituras de la otra goroutine. Leerlo antes sería la misma carrera que este test dice
	// prevenir.
	cuerpo := g.Body.String()
	if !strings.Contains(cuerpo, ": latido\n\n") {
		t.Fatalf("no hay ningún latido en el cuerpo: %q", cuerpo)
	}
	// Y el orden es el invariante: el latido tiene que estar ANTES del primer evento, porque
	// lo que se afirma es que no espera al motor. Si saliera después, este test pasaría con un
	// ticker que sólo late cuando ya hay tráfico — que es inútil.
	if iLatido, iEvento := strings.Index(cuerpo, ": latido"), strings.Index(cuerpo, "data: "); iEvento >= 0 && iLatido > iEvento {
		t.Errorf("el latido salió DESPUÉS del primer evento (latido en %d, evento en %d): %q", iLatido, iEvento, cuerpo)
	}
}

// Sabotaje T11: las cabeceras no se bajan al cable (el cliente espera el 1er token)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t_ = rc.Flush()\n\n\tbombearTurno("
// arnes: a="\n\tbombearTurno("
func TestLasCabecerasLleganAntesDelPrimerEvento(t *testing.T) {
	// 🔴 EL DEFECTO QUE ESTE TEST DESENTERRÓ, y que no se ve como un defecto. `WriteHeader`
	// sólo pone las cabeceras en el buffer del server: sin un `Flush` propio no salen al cable
	// hasta que algo más escriba. Del otro lado el cliente hace `await fetch(...)` y recién
	// entonces mira `response.ok` (proxy.ts:154), así que se queda bloqueado todo lo que tarde
	// el primer token — y con thinking largo son decenas de segundos en los que no puede
	// distinguir «pensando» de «no conectó».
	//
	// Medido: sin el flush del handler, lo único que destrababa el buffer era el primer LATIDO,
	// a los 20 s. Por eso acá hay un reloj: sin él el test pasaría igual, sólo tardando 20 s, y
	// un test que tarda en vez de fallar es un test que no mide nada.
	liberar := make(chan struct{})
	soltar := sync.OnceFunc(func() { close(liberar) })
	defer soltar()

	m := &motorFalso{
		antesDe: func() { <-liberar },
		eventos: []motor.Evento{{Type: motor.EvDone, Reason: motor.ReasonStop, Usage: &motor.Usage{}}},
	}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(m)(s)
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 30 * time.Second}))
	defer ts.Close()

	// El tope es lo que convierte «tarda 20 s» en ROJO, y está puesto donde separa las dos
	// cosas sin tocar ninguna: esto cuesta milisegundos y el defecto cuesta 20 s, así que
	// cualquier número entre medio detecta igual y lo único que cambia es la tolerancia a una
	// máquina cargada. Se eligió el extremo holgado por evidencia del propio repo: en la suite
	// completa del 2026-10-29, con tres paquetes pesados en paralelo, DOS tests con reloj
	// dieron falso rojo por contención (`TestCerebroSaludaInicializacionSinDiscar` tardó
	// 12,65 s, y `TestObservadorCorrerLimpioBajoRace` no vio terminar algo en 28 s). Un reloj
	// mide bien el defecto y se equivoca bajo carga: el margen es lo que paga esa diferencia.
	cl := &http.Client{Timeout: 5 * time.Second}
	inicio := time.Now()
	resp, err := cl.Post(ts.URL+"/api/stream", "application/json", strings.NewReader(cuerpoMinimo))
	if err != nil {
		t.Fatalf("el POST no recibió cabeceras en %s con el motor callado — las cabeceras SSE no se bajan al cable: %v", time.Since(inicio).Round(time.Millisecond), err)
	}
	defer resp.Body.Close()
	demora := time.Since(inicio)

	if resp.StatusCode != http.StatusOK {
		cuerpo, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, cuerpo)
	}
	// Las cabeceras llegaron con el motor TODAVÍA bloqueado: eso es lo que deja al cliente
	// empezar a leer antes del primer token.
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if demora > 2*time.Second {
		t.Errorf("las cabeceras tardaron %s con el motor callado: algo las está bufferando", demora.Round(time.Millisecond))
	}

	// Y que el stream siga sirviendo después del flush temprano: un flush mal puesto podría
	// dejar el cuerpo roto o cerrado.
	soltar()
	br := bufio.NewReader(resp.Body)
	vioDone := false
	for !vioDone {
		linea, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("leyendo el stream: %v", err)
		}
		if strings.HasPrefix(linea, "data: ") {
			var ev motor.Evento
			if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(linea), "data: ")), &ev); err != nil {
				t.Fatalf("una línea data: no parsea: %v (%q)", err, linea)
			}
			if ev.Type == motor.EvDone {
				vioDone = true
			}
		}
	}
}

// Sabotaje T21: el canal de resultado sin buffer (fuga una goroutine por cliente)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\thecho := make(chan error, 1)"
// arnes: a="\thecho := make(chan error)"
func TestElClienteQueSeVaNoDejaGoroutinesColgadas(t *testing.T) {
	// 🔴 UNA GOROUTINE POR CLIENTE QUE SE DESCONECTA ES UNA FUGA QUE NO SE VE. El canal de
	// eventos va sin buffer, así que el motor queda bloqueado en `emitir` cuando nadie lee; y
	// el canal de resultado SÍ tiene buffer para que pueda terminar aunque el handler ya se
	// haya ido. Sacar cualquiera de las dos cosas deja una goroutine viva por turno abortado.
	//
	// 🔴 Y SE CUENTAN LAS GOROUTINES, no sólo que el bombeo vuelva. Es la diferencia entre
	// medir el invariante y medir otra cosa: `bombearTurno` vuelve por su `case <-ctx.Done()`
	// PASE LO QUE PASE con las otras dos goroutines, así que un test que sólo lo espera queda
	// verde con la fuga intacta. Lo que hay que ver es que el proceso vuelva a la cuenta de
	// antes — que es lo que no pasa cuando el canal de resultado pierde su buffer.
	//
	// 🔴 Y EL ORDEN DE LOS DOS HECHOS ES PARTE DE LA MEDICIÓN — sin eso este test medía A CARA O
	// CRUZ. La versión anterior usaba un motor que volvía por su PROPIO `ctx.Done()`: al
	// cancelar, el motor y el bombeo despertaban juntos, y si el motor llegaba primero a
	// `hecho <- err`, el `select` del bombeo veía las DOS ramas listas y elegía al azar — la
	// mitad de las veces drenaba `hecho` y no quedaba fuga. Medido: la batería de sabotaje lo
	// dio ROJO una vuelta y VERDE la siguiente, sin que cambiara una línea del código bajo
	// prueba. Acá el motor NO mira el ctx: espera a que lo suelte el TEST, y el test lo suelta
	// después de ver volver al bombeo, así que el motor intenta entregar su resultado cuando ya
	// no hay nadie del otro lado — que es la única situación en la que el buffer hace diferencia.
	//
	// Lo que este test NO mide: que `emitir` mire el ctx. Ese invariante es de
	// `TestElEmitirEsElQueCortaAlMotorQueNoMiraElCtx`, y hasta que ese test existió el sabotaje
	// que le quita el ctx al emisor quedaba VERDE.
	enMarcha := make(chan struct{})
	suelte := make(chan struct{})
	termino := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	g := nuevoGrabador()
	rc := http.NewResponseController(g)

	antes := runtime.NumGoroutine()
	go func() {
		defer close(termino)
		// Un latido largo a propósito: lo que se mide es el cancel, y un ticker corto metería
		// escrituras de latido en medio que no tienen nada que ver con lo que se afirma.
		bombearTurno(ctx, &motorQueVuelveCuandoSeLePide{enMarcha: enMarcha, suelte: suelte},
			motor.Pedido{}, g, rc, time.Minute)
	}()

	<-enMarcha // el motor ya emitió: el turno está en marcha
	cancel()

	select {
	case <-termino:
	case <-time.After(5 * time.Second):
		t.Fatal("el bombeo no volvió tras el cancel: hay una goroutine colgada")
	}

	// EL HANDLER YA SE FUE, y recién ahora el motor intenta entregar su resultado: es el momento
	// exacto para el que existe el buffer del canal, y el único en que su ausencia se ve.
	close(suelte)

	// La cuenta se espera en vez de leerse de una: el scheduler no garantiza que una goroutine
	// que ya puede terminar haya terminado, y un chequeo inmediato sería flaky en la dirección
	// MALA — fallando cuando no hay fuga. El tope es largo porque lo que se afirma es que la
	// cuenta baja, no cuándo.
	limite := time.Now().Add(5 * time.Second)
	ahora := runtime.NumGoroutine()
	for ahora > antes && time.Now().Before(limite) {
		runtime.Gosched()
		ahora = runtime.NumGoroutine()
	}
	if ahora > antes {
		// El dump dice EN QUÉ LÍNEA quedó colgada, que es el dato que convierte esto en una
		// corrección y no en una sesión de diagnóstico.
		buf := make([]byte, 1<<16)
		buf = buf[:runtime.Stack(buf, true)]
		t.Fatalf("quedaron %d goroutine(s) de más (%d -> %d) tras abortar el turno:\n%s", ahora-antes, antes, ahora, buf)
	}
}

// motorQueVuelveCuandoSeLePide avisa que el turno está EN MARCHA — que es el caso que de verdad
// puede dejar una goroutine bloqueada — y después espera a que lo suelte el TEST, sin mirar el
// ctx. Que el momento de volver lo decida el test es lo que hace DETERMINISTA la medición: un
// motor que vuelve por su propio `ctx.Done()` corre una carrera con el bombeo al despertar del
// cancel, y el resultado de esa carrera decide si la fuga se ve o no.
type motorQueVuelveCuandoSeLePide struct {
	enMarcha chan struct{}
	suelte   chan struct{}
}

func (m *motorQueVuelveCuandoSeLePide) Turno(ctx context.Context, p motor.Pedido, emitir func(motor.Evento) error) error {
	if err := emitir(motor.Evento{Type: motor.EvStart}); err != nil {
		return err
	}
	close(m.enMarcha)
	<-m.suelte // NO mira el ctx a propósito: el momento de volver lo decide el test
	return context.Canceled
}

// motorTerco emite para siempre y NO mira el ctx. Es el único motor que pone a prueba de verdad
// que `emitir` sea el que corta: `motorQueVuelveCuandoSeLePide` emite UNA vez y después espera al
// test, así que nunca vuelve a llamar a `emitir` y no puede delatar a un emisor que ignora el ctx.
type motorTerco struct {
	aviso chan struct{}
	una   sync.Once
}

func (m *motorTerco) Turno(ctx context.Context, p motor.Pedido, emitir func(motor.Evento) error) error {
	for {
		// Delta vacío a propósito: el handler escribe cada evento al grabador y esto emite
		// hasta que lo corten, así que lo que se gasta en el camino conviene que sea poco.
		if err := emitir(motor.Evento{Type: motor.EvTextDelta}); err != nil {
			return err
		}
		m.una.Do(func() { close(m.aviso) })
		// Cooperativo, no prudente: el `Gosched` le da aire al scheduler para que el cancel
		// del test llegue pronto, y NO mira el ctx — si lo mirara, este motor dejaría de ser
		// el motor terco que el invariante necesita.
		runtime.Gosched()
	}
}

// Sabotaje T22: el emitir no mira el ctx (el motor sigue generando para nadie)
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tselect {\n\t\t\tcase eventos <- ev:\n\t\t\t\treturn nil\n\t\t\tcase <-ctx.Done():"
// arnes: a="\t\t\tselect {\n\t\t\tcase eventos <- ev:\n\t\t\t\treturn nil\n\t\t\tcase <-make(chan struct{}):"
func TestElEmitirEsElQueCortaAlMotorQueNoMiraElCtx(t *testing.T) {
	// 🔴 EL INVARIANTE QUE EL TEST DE LA FUGA NO MIDE, y que el sabotaje encontró: un motor
	// bien educado se entera del cancel por su propio ctx, pero el que importa acá es el que
	// NO lo mira —un proceso hijo que sigue escupiendo stdout, un stream HTTP que sigue
	// llegando— y para ése la ÚNICA salida es que `emitir` devuelva error. Sin el
	// `case <-ctx.Done()` del emisor, ese motor queda bloqueado escribiendo a un canal que
	// nadie lee, para siempre, una goroutine por cada cliente que se fue a mitad de turno.
	emitio := make(chan struct{})
	termino := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	g := nuevoGrabador()
	rc := http.NewResponseController(g)

	antes := runtime.NumGoroutine()
	go func() {
		defer close(termino)
		bombearTurno(ctx, &motorTerco{aviso: emitio}, motor.Pedido{}, g, rc, time.Minute)
	}()

	<-emitio // el motor ya está en su bucle: el turno está en marcha
	cancel()

	select {
	case <-termino:
	case <-time.After(5 * time.Second):
		t.Fatal("el bombeo no volvió tras el cancel")
	}

	// Misma espera acotada que en el test de la fuga, y por el mismo motivo: el scheduler no
	// garantiza que una goroutine que ya puede terminar haya terminado.
	limite := time.Now().Add(5 * time.Second)
	ahora := runtime.NumGoroutine()
	for ahora > antes && time.Now().Before(limite) {
		runtime.Gosched()
		ahora = runtime.NumGoroutine()
	}
	if ahora > antes {
		buf := make([]byte, 1<<16)
		buf = buf[:runtime.Stack(buf, true)]
		t.Fatalf("quedaron %d goroutine(s) de más (%d -> %d): el motor que ignora el ctx sigue bloqueado en emitir:\n%s", ahora-antes, antes, ahora, buf)
	}
}

// ─────────────────────────── el panic del motor ───────────────────────────

// El valor del panic hace de sonda del filtrado: si el handler lo mandara al otro lado, este
// texto aparecería en el cuerpo SSE. Está escrito como lo que un panic real arrastra —algo de
// la máquina que no tiene por qué salir de ella— para que el custodio del filtrado no esté
// midiendo una cadena inocua que a nadie le importaría ver.
const panicDelMotor = "defecto del motor leyendo C:/Proyectos/Musubi/.musubi/config.yaml"

// motorQuePanica emite un evento y recién DESPUÉS paniquea. El orden importa: así el custodio
// mide el caso real y no el más cómodo — cuando el motor se rompe, el 200 y las cabeceras ya
// salieron al cable, el status ya no se puede cambiar, y la única forma de nombrar la falla es
// un evento del propio stream.
type motorQuePanica struct{}

func (motorQuePanica) Turno(ctx context.Context, p motor.Pedido, emitir func(motor.Evento) error) error {
	if err := emitir(motor.Evento{Type: motor.EvStart}); err != nil {
		return err
	}
	panic(panicDelMotor)
}

// Sabotaje T24: el panic se reporta como un turno que cerró bien (el cliente no se entera)
// Mismo `de` que T25, y son dos guardas distintas sobre esa línea, no una contada dos veces:
// ésta mide que la falla se NOMBRE, T25 que el valor del panic NO viaje. Con `nil` por el canal
// el select lee el cierre como bueno y vuelve sin evento terminal, que es justo el silencio que
// `eventoDeCierreAnomalo` existe para evitar.
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\t\thecho <- errors.New(\"defecto interno del motor\")"
// arnes: a="\t\t\t\thecho <- nil"
// arnes: colision_ok="TestElValorDelPanicNoViajaAlCliente"
func TestUnPanicDelMotorSeNombraComoFallaDelTurno(t *testing.T) {
	// 🔴 EL PANIC SE DEGRADA AL ERROR DE UN TURNO, y el cliente tiene que ENTERARSE. El 200 ya
	// salió con las cabeceras, así que el status no se puede cambiar; un stream que se corta sin
	// evento terminal le deja el fallback genérico de desconexión, que no dice nada. La otra
	// mitad del invariante —que el proceso siga vivo— la mide
	// `TestElPanicDelMotorNoMataAlProcesoDelServidor`, que necesita un proceso aparte.
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(motorQuePanica{})(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	evs := eventosDelCuerpo(t, g.Body.String())
	if len(evs) == 0 {
		t.Fatal("no salió ningún evento: el panic se comió el stream entero")
	}
	if evs[0].Type != motor.EvStart {
		t.Errorf("el primer evento = %q, quiero %q: se perdió lo que ya había salido antes del panic", evs[0].Type, motor.EvStart)
	}
	ultimo := evs[len(evs)-1]
	if ultimo.Type != motor.EvError {
		t.Fatalf("el último evento = %q, quiero %q: el turno que paniqueó cortó sin nombrar la falla", ultimo.Type, motor.EvError)
	}
	if ultimo.Reason != motor.ReasonError {
		t.Errorf("reason = %q, quiero %q: un panic no es un cancel del cliente", ultimo.Reason, motor.ReasonError)
	}
	// Mismo motivo que en el cierre anómalo común: el protocolo declara el `usage` OBLIGATORIO
	// en el `error` (proxy.ts:44), y un panic no es excusa para romperle el tipo al cliente.
	if ultimo.Usage == nil {
		t.Error("el evento error del panic salió SIN usage, y el protocolo lo declara obligatorio")
	}
}

// Sabotaje T25: el valor del panic viaja al cliente dentro del evento de error
// Mismo `de` que T24: el motivo de las dos está ahí. Ésta es la mitad del FILTRADO.
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\t\thecho <- errors.New(\"defecto interno del motor\")"
// arnes: a="\t\t\t\thecho <- fmt.Errorf(\"defecto interno del motor: %v\", r)"
// arnes: colision_ok="TestUnPanicDelMotorSeNombraComoFallaDelTurno"
func TestElValorDelPanicNoViajaAlCliente(t *testing.T) {
	// 🔴 UN VALOR DE PANIC ARRASTRA LO QUE TENGA A MANO: una ruta del disco, un fragmento del
	// prompt, el pedazo de un token. El detalle sirve para arreglar el defecto, y el lugar para
	// eso es el log del server —que lo lee quien corre el servicio— y no un evento que termina
	// pintado en la UI y guardado en el transcript del otro lado. Es la misma decisión que toma
	// el recover de `Dispatch` (server.go:712): contesta «error interno inesperado» y deja el
	// valor en el log.
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(motorQuePanica{})(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	cuerpo := g.Body.String()
	if strings.Contains(cuerpo, panicDelMotor) {
		t.Errorf("el valor del panic salió al cliente: %q aparece en el cuerpo SSE\n%s", panicDelMotor, cuerpo)
	}
	// Y el cliente IGUAL tiene que recibir algo, o este test se cumpliría borrando el evento:
	// filtrar no es callarse, y el silencio es el defecto que mide el test de al lado.
	evs := eventosDelCuerpo(t, cuerpo)
	if len(evs) == 0 || evs[len(evs)-1].Type != motor.EvError {
		t.Fatalf("no salió el evento de error: %s", cuerpo)
	}
	if evs[len(evs)-1].ErrorMessage == "" {
		t.Error("el evento de error salió sin mensaje: el cliente no tiene con qué distinguirlo de un corte")
	}
}

// EL DETALLE DEL PANIC LLEGA AL LOG: el mensaje, el valor y la traza. Es la contracara del test
// de arriba y hace falta que sean dos: ése mide que el valor NO salga al cliente, y se cumpliría
// igual si el detalle no fuera a ninguna parte. El comentario de `turno.go:200-202` lo dice como
// promesa —«un arreglo que cambie "crash con traza" por "log sin traza" deja el defecto más
// difícil de encontrar que estaba»— y nadie la medía: la línea del `logx.Error` es la ÚNICA de
// log de producción que este change-set agrega, y medido el 2026-10-29 se la podía dejar sin la
// traza, o sin el valor del panic, con el paquete compilando y los tres tests del recover en
// verde. Sin la traza no se sabe QUÉ motor rompió, que es lo único que un operador necesita.
func TestElDetalleDelPanicDelMotorVaAlLog(t *testing.T) {
	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	defer restaurar()

	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(motorQuePanica{})(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	registro := log.String()
	if !strings.Contains(registro, "panic en el motor del turno") {
		t.Errorf("el log no nombra la falla: %s", registro)
	}
	if !strings.Contains(registro, panicDelMotor) {
		t.Errorf("el log no tiene el valor del panic (%q), que es lo que sirve para arreglarlo: %s",
			panicDelMotor, registro)
	}
	// El sitio del panic sale de la traza. Sin ella el log dice que algo rompió y no dónde, que
	// es el retroceso que el comentario del recover declara que hay que evitar.
	if !strings.Contains(registro, "motorQuePanica") {
		t.Errorf("el log no tiene la traza: falta el sitio del panic: %s", registro)
	}
	// Control: que el detalle vaya al log no puede cumplirse mandándolo TAMBIÉN al cliente.
	if strings.Contains(g.Body.String(), panicDelMotor) {
		t.Error("el valor del panic salió al cliente además del log")
	}
}

// La marca que imprime el ayudante si LLEGÓ a volver del turno. Hace falta una marca y no basta
// el exit code: un hijo que muere antes de arrancar también sale distinto de cero, y los dos se
// leerían igual.
const marcaDeSupervivencia = "EL PROCESO SOBREVIVIO AL PANIC DEL MOTOR"

// TestAyudanteDelPanicDelMotor NO es un test: es el hijo del custodio de abajo. Corre un turno
// que paniquea por la ruta de producción y, si el proceso queda vivo, imprime la marca.
func TestAyudanteDelPanicDelMotor(t *testing.T) {
	if os.Getenv("GO_AYUDANTE_PANIC_DEL_MOTOR") != "1" {
		t.Skip("ayudante, no es un test")
	}
	s := newTestServer(t, embedding.NoopProvider{})
	WithMotor(motorQuePanica{})(s)

	g := nuevoGrabador()
	s.servirTurno(httpOptions{}, g, pedidoDeTurno(cuerpoMinimo))

	fmt.Println(marcaDeSupervivencia)
}

// Sabotaje T23: el defer no llama a recover (el panic se lleva el proceso entero, como antes)
// No se escribe BORRANDO el defer: sin él quedan `logx` y `debug` sin usar, el paquete no
// compila, y un build roto no mide ningún invariante — se pone rojo con cualquier cosa que se
// toque (es la falla 5 de `deploy/pruebas/sabotaje.sh`). Neutralizar el `recover()` deja el
// archivo compilando y cambia sólo el comportamiento, que es lo único que se quiere medir.
// arnes: archivo="internal/mcp/turno.go"
// arnes: de="\t\t\tif r := recover(); r != nil {"
// arnes: a="\t\t\tif r := any(nil); r != nil {"
func TestElPanicDelMotorNoMataAlProcesoDelServidor(t *testing.T) {
	// 🔴 ESTA GOROUTINE NO ES LA DEL HANDLER, y el `recover` de `net/http` no la cubre: medido
	// con dos brazos en la misma corrida, `net/http.(*conn).serve.func1` aparece en la pila de
	// un panic del handler y está AUSENTE en la de ésta. Sin el recover del motor, un defecto en
	// CUALQUIER turno se lleva puesto al `musubi serve` entero, con los demás turnos en vuelo
	// adentro y sus hijos `claude` quedando huérfanos, gastando cuota que ya nadie va a leer.
	//
	// 🔴 Y POR QUÉ EN UN PROCESO APARTE, que es lo que hace que esto sea una guarda y no una
	// esperanza: un panic sin recover mata al binario de test, así que medido DESDE ADENTRO el
	// sabotaje no produce `--- FAIL: <nombre>` sino un exit code pelado — exactamente la señal
	// que `deploy/pruebas/sabotaje.sh` se niega a aceptar, porque «la guarda funcionó» y «rompí
	// el build» se leen igual. Con el turno en un hijo, el que reporta el rojo es este test.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := guiones.HerramientaCtx(ctx, t, os.Args[0],
		"-test.run=^TestAyudanteDelPanicDelMotor$",
		// El TestMain de este paquete exige un `-test.timeout` suficiente y el hijo NO hereda el
		// del padre. `0` es «sin límite», que es lo que deja mudo a ese guard; el techo real del
		// hijo lo pone el contexto de arriba, que además lo mata si se cuelga.
		"-test.timeout=0")
	cmd.Env = append(os.Environ(), "GO_AYUDANTE_PANIC_DEL_MOTOR=1")
	salida, err := cmd.CombinedOutput()

	// El cuelgue se separa de la muerte ANTES de culpar al panic: un hijo que el contexto mató
	// sale distinto de cero igual, y leerlo como «se cayó por el panic» sería acusar al arreglo
	// de algo que no hizo.
	if ctx.Err() != nil {
		t.Fatalf("el hijo no terminó en un minuto, así que no se puede decir si sobrevivió:\n%s", salida)
	}
	if err != nil {
		t.Fatalf("EL PROCESO MURIÓ por un panic de UN turno (%v): se llevó puesto al servidor "+
			"entero, con los demás turnos en vuelo adentro.\n%s", err, salida)
	}
	if !strings.Contains(string(salida), marcaDeSupervivencia) {
		t.Fatalf("el hijo salió con 0 pero nunca llegó a imprimir %q, así que no volvió del "+
			"turno:\n%s", marcaDeSupervivencia, salida)
	}
}
