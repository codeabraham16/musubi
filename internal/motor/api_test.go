package motor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseDeUnTurno es un turno completo en el formato que emite /v1/messages con `stream:true`:
// las líneas `event:` que el traductor descarta, los `data:` que sí lee, y la línea vacía
// que cierra cada evento.
const sseDeUnTurno = `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":120,"cache_read_input_tokens":15842}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Par"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ís"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

event: message_stop
data: {"type":"message_stop"}

`

// servidorSSE levanta un /v1/messages falso que contesta el cuerpo dado y guarda el request
// que recibió. Es el patrón de calibrate.go: sin esto no hay forma de probar el camino entero
// sin consumir cuota ni necesitar una credencial.
func servidorSSE(t *testing.T, cuerpo string) (*ClienteAPI, *http.Request, *[]byte) {
	t.Helper()
	var visto http.Request
	var payload []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = *r.Clone(context.Background())
		payload, _ = io.ReadAll(r.Body)
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, cuerpo)
	}))
	t.Cleanup(srv.Close)
	return &ClienteAPI{Clave: "sk-de-prueba", Endpoint: srv.URL, HTTP: srv.Client()}, &visto, &payload
}

func pedidoAPIDePrueba() Pedido {
	return Pedido{
		Modelo:       "claude-sonnet-5",
		SystemPrompt: "sos un motor",
		Mensajes:     []Mensaje{TextoDeUsuario("¿capital de Francia?")},
	}
}

// EL CAMINO ENTERO del segundo motor: SSE adentro, eventos del protocolo afuera. Lo que se
// pinea no es sólo que funcione, es que dé LO MISMO que el otro motor — la equivalencia de
// las dos puertas del traductor está probada aparte, y acá se verifica que el transporte no
// la ensucie.
func TestElClienteAPISirveUnTurnoCompleto(t *testing.T) {
	c, _, _ := servidorSSE(t, sseDeUnTurno)

	var evs []Evento
	var texto strings.Builder
	if err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs = append(evs, e)
		if e.Type == EvTextDelta {
			texto.WriteString(e.Delta)
		}
		return nil
	}); err != nil {
		t.Fatalf("Turno: %v", err)
	}

	quiero := []string{EvStart, EvTextStart, EvTextDelta, EvTextDelta, EvTextEnd, EvDone}
	if got := tipos(evs); !iguales(got, quiero) {
		t.Errorf("forma del turno:\n got %v\nwant %v", got, quiero)
	}
	if texto.String() != "París" {
		t.Errorf("texto: got %q, want %q", texto.String(), "París")
	}

	// EL USAGE FUSIONADO, que es el invariante que más fácil se rompe: los tokens de
	// entrada y de caché vienen SÓLO en el message_start, y el output sólo en el delta
	// final. Reportar el usage del delta pelado declararía 15.842 tokens de caché en cero.
	fin := evs[len(evs)-1]
	if fin.Usage == nil {
		t.Fatal("el done tiene que llevar usage")
	}
	if fin.Usage.Input != 120 || fin.Usage.CacheRead != 15842 || fin.Usage.Output != 7 {
		t.Errorf("usage: %+v", *fin.Usage)
	}
	if fin.Reason != ReasonStop {
		t.Errorf("reason: got %q, want %q", fin.Reason, ReasonStop)
	}
}

// EL REQUEST. Tres de estos campos los EXIGE la API y el resto define qué turno se sirve:
// errarle a cualquiera no da un turno distinto, da un 400.
func TestElRequestALaAPILlevaLoQueLaAPIExige(t *testing.T) {
	c, req, payload := servidorSSE(t, sseDeUnTurno)
	c.Betas = []string{"beta-uno", "beta-dos"}

	p := pedidoAPIDePrueba()
	p.MaxTokens = 2048
	p.Thinking = 1024
	p.Tools = json.RawMessage(`[{"name":"Read","description":"lee","input_schema":{"type":"object"}}]`)
	if err := c.Turno(context.Background(), p, func(Evento) error { return nil }); err != nil {
		t.Fatalf("Turno: %v", err)
	}

	if got := req.Header.Get("x-api-key"); got != "sk-de-prueba" {
		t.Errorf("x-api-key: got %q", got)
	}
	// `anthropic-version` es OBLIGATORIA: sin esa cabecera la API rechaza el request.
	if got := req.Header.Get("anthropic-version"); got != VersionAPIPorDefecto {
		t.Errorf("anthropic-version: got %q, want %q", got, VersionAPIPorDefecto)
	}
	if got := req.Header.Get("accept"); got != "text/event-stream" {
		t.Errorf("accept: got %q", got)
	}
	// Las betas van en UNA cabecera separada por comas, no una por beta.
	if got := req.Header.Get("anthropic-beta"); got != "beta-uno,beta-dos" {
		t.Errorf("anthropic-beta: got %q", got)
	}

	var cuerpo struct {
		Model     string            `json:"model"`
		MaxTokens int               `json:"max_tokens"`
		Stream    bool              `json:"stream"`
		System    string            `json:"system"`
		Tools     []json.RawMessage `json:"tools"`
		Thinking  *struct {
			Type         string `json:"type"`
			BudgetTokens int    `json:"budget_tokens"`
		} `json:"thinking"`
		Messages []struct {
			Role    string            `json:"role"`
			Content []json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(*payload, &cuerpo); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v\n%s", err, *payload)
	}
	if cuerpo.Model != "claude-sonnet-5" || cuerpo.MaxTokens != 2048 {
		t.Errorf("model/max_tokens: %q/%d", cuerpo.Model, cuerpo.MaxTokens)
	}
	// Sin `stream:true` la API contesta un mensaje completo de una vez, y este paquete
	// entero —que es todo deltas— no tendría nada que traducir.
	if !cuerpo.Stream {
		t.Error("falta stream:true")
	}
	if cuerpo.System != "sos un motor" {
		t.Errorf("system: got %q", cuerpo.System)
	}
	if len(cuerpo.Tools) != 1 || !strings.Contains(string(cuerpo.Tools[0]), `"name":"Read"`) {
		t.Errorf("las tools del cliente tienen que viajar crudas: %v", cuerpo.Tools)
	}
	if cuerpo.Thinking == nil || cuerpo.Thinking.Type != "enabled" || cuerpo.Thinking.BudgetTokens != 1024 {
		t.Errorf("thinking: %+v", cuerpo.Thinking)
	}
	if len(cuerpo.Messages) != 1 || cuerpo.Messages[0].Role != "user" || len(cuerpo.Messages[0].Content) != 1 {
		t.Errorf("messages: %+v", cuerpo.Messages)
	}

	// Sin thinking el bloque NO va: mandarlo con budget 0 es un 400.
	c2, _, payload2 := servidorSSE(t, sseDeUnTurno)
	if err := c2.Turno(context.Background(), pedidoAPIDePrueba(), func(Evento) error { return nil }); err != nil {
		t.Fatalf("Turno: %v", err)
	}
	if strings.Contains(string(*payload2), "thinking") {
		t.Errorf("un pedido sin thinking no debe declararlo: %s", *payload2)
	}
	// Y el max_tokens por defecto tiene que estar: el campo es obligatorio.
	if !strings.Contains(string(*payload2), `"max_tokens":64000`) {
		t.Errorf("falta el max_tokens por defecto: %s", *payload2)
	}
}

// LO QUE SE VALIDA ANTES DE SALIR A LA RED. Cada uno de estos sería un 400 de la API, y un
// 400 cuesta un viaje, cuota y un mensaje que no dice cuál campo arreglar.
func TestElPedidoSeValidaAntesDeGastarUnRequest(t *testing.T) {
	casos := []struct {
		nombre string
		p      Pedido
		dice   string
	}{
		{
			nombre: "sin modelo",
			p:      Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
			dice:   "sin modelo",
		},
		{
			// La API exige max_tokens > budget_tokens, y su 400 habla de los dos campos
			// sin decir cuál cambiar.
			nombre: "el budget de thinking no cabe en max_tokens",
			p: Pedido{
				Modelo: "m", MaxTokens: 1000, Thinking: 1000,
				Mensajes: []Mensaje{TextoDeUsuario("hola")},
			},
			dice: "budget de thinking",
		},
		{
			nombre: "un rol que la API no acepta",
			p: Pedido{
				Modelo:   "m",
				Mensajes: []Mensaje{{Rol: "system", Contenido: []json.RawMessage{[]byte(`{}`)}}},
			},
			dice: "no es user ni assistant",
		},
		{
			nombre: "un mensaje sin contenido",
			p:      Pedido{Modelo: "m", Mensajes: []Mensaje{{Rol: "user"}}},
			dice:   "sin contenido",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := cuerpoDelRequest(c.p)
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if !strings.Contains(err.Error(), c.dice) {
				t.Errorf("el error no nombra el arreglo (%q): %v", c.dice, err)
			}
		})
	}

	// Un pedido sin mensajes se rechaza en Turno, antes incluso de armar el cuerpo.
	c, _, _ := servidorSSE(t, sseDeUnTurno)
	if err := c.Turno(context.Background(), Pedido{Modelo: "m"}, func(Evento) error { return nil }); err == nil {
		t.Error("un pedido sin mensajes debe rechazarse")
	}
}

// SIN CREDENCIAL, el error tiene que nombrar la VARIABLE que falta. Un «unauthorized» de la
// API después del viaje no dice dónde poner la clave, y el nombre es configurable.
func TestSinCredencialElErrorNombraLaVariable(t *testing.T) {
	c := &ClienteAPI{Endpoint: "http://127.0.0.1:1", ClaveEnv: "MI_CLAVE_QUE_NO_EXISTE"}
	err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err == nil {
		t.Fatal("se esperaba error")
	}
	if !strings.Contains(err.Error(), "MI_CLAVE_QUE_NO_EXISTE") {
		t.Errorf("el error tiene que nombrar la variable: %v", err)
	}
	// Y NUNCA la credencial: un error que la imprima la deja en el journal del server.
	c2 := &ClienteAPI{Endpoint: "http://127.0.0.1:1", Clave: "sk-secreto-123"}
	err2 := c2.Turno(context.Background(), pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err2 == nil {
		t.Fatal("se esperaba error de conexión")
	}
	if strings.Contains(err2.Error(), "sk-secreto-123") {
		t.Errorf("🔴 el error filtró la credencial: %v", err2)
	}
}

// EL ERROR DE LA API SALE CON SU MENSAJE. El cuerpo es lo único que dice por qué —saldo
// agotado, modelo inexistente, un campo inválido— y tirarlo deja un «status 400» imposible
// de diagnosticar.
func TestUnRechazoDeLaAPISaleConSuMensaje(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: must be greater than thinking.budget_tokens"}}`)
	}))
	defer srv.Close()

	c := &ClienteAPI{Clave: "k", Endpoint: srv.URL, HTTP: srv.Client()}
	err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err == nil {
		t.Fatal("se esperaba error")
	}
	for _, quiero := range []string{"400", "invalid_request_error", "must be greater than"} {
		if !strings.Contains(err.Error(), quiero) {
			t.Errorf("el error no trae %q: %v", quiero, err)
		}
	}

	// Un cuerpo que no es el JSON de la API igual tiene que llegar: una pasarela puede
	// contestar un HTML de nginx, y perderlo deja el status pelado.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "<html>502 desde el proxy</html>")
	}))
	defer srv2.Close()
	c2 := &ClienteAPI{Clave: "k", Endpoint: srv2.URL, HTTP: srv2.Client()}
	err2 := c2.Turno(context.Background(), pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err2 == nil || !strings.Contains(err2.Error(), "502 desde el proxy") {
		t.Errorf("el cuerpo del rechazo tiene que llegar: %v", err2)
	}
}

// EL PARSEO DEL SSE, con las rarezas que el formato permite y que un parser ingenuo rompe.
func TestElParseoDelSSEAguantaLasRarezasDelFormato(t *testing.T) {
	// Un evento con DOS líneas `data:` se concatena con \n (spec de SSE). La API manda una
	// sola, pero un JSON partido en dos llegaría como dos JSON inválidos.
	partido := "event: message_start\n" +
		`data: {"type":"message_start",` + "\n" +
		`data: "message":{"usage":{"input_tokens":5}}}` + "\n\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}` + "\n\n"

	c, _, _ := servidorSSE(t, partido)
	var evs []Evento
	if err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs = append(evs, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno con data en dos líneas: %v", err)
	}
	if got := tipos(evs); !iguales(got, []string{EvStart, EvDone}) {
		t.Errorf("un data partido en dos líneas no se rearmó: %v", got)
	}
	if evs[len(evs)-1].Usage == nil || evs[len(evs)-1].Usage.Input != 5 {
		t.Errorf("se perdió el usage del message_start: %+v", evs[len(evs)-1].Usage)
	}

	// Los latidos del SSE viajan como comentarios (`: algo`) y los manda cualquier proxy.
	// Parsearlos como datos tiraría el turno.
	conLatidos := ": latido\n\n" + strings.Replace(sseDeUnTurno, "event: content_block_stop", ": otro latido\nevent: content_block_stop", 1)
	c2, _, _ := servidorSSE(t, conLatidos)
	var evs2 []Evento
	if err := c2.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs2 = append(evs2, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno con latidos: %v", err)
	}
	if len(evs2) != 6 {
		t.Errorf("los latidos cambiaron el turno: %v", tipos(evs2))
	}

	// El `[DONE]` no lo manda la Messages API (cierra con message_stop), pero sí una
	// pasarela al estilo OpenAI, y parsearlo como JSON sería un error inútil en el log.
	//
	// ⚠️ VA ANTES DEL message_delta A PROPÓSITO. Pegado al final —que es donde una pasarela lo
	// pone— esta línea NO SE LEE NUNCA: el turno cierra en el message_delta y bombearSSE
	// vuelve ahí mismo. O sea que el fixture «natural» no ejercita la guarda ni un poco, y la
	// cobertura miente diciendo que sí.
	conDone := strings.Replace(sseDeUnTurno, "event: message_delta",
		"data: [DONE]\n\nevent: message_delta", 1)
	c3, _, _ := servidorSSE(t, conDone)
	var evs3 []Evento
	if err := c3.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs3 = append(evs3, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno con [DONE]: %v", err)
	}
	// El marcador no produce evento y no tira el turno. La guarda ADEMÁS evita un error
	// inútil en el log, y eso el test no lo puede medir: ver la nota de la spec.
	if len(evs3) != 6 {
		t.Errorf("el [DONE] cambió el turno: %v", tipos(evs3))
	}

	// Un stream que termina SIN la línea vacía final deja el último evento sin despachar. La
	// API siempre la manda; perder el message_delta por un \n faltante sería tirar el turno
	// entero.
	//
	// ⚠️ EL CORTE VA EN EL message_delta, NO AL FINAL DEL FIXTURE, y la diferencia es que el
	// test mida o no mida. Recortar el `\n` del final deja el message_delta con su línea
	// vacía intacta: el turno cierra ahí, el despacho de después del loop no corre, y romperlo
	// dejaba este test VERDE. Lo encontró el sabotaje A13.
	hasta := strings.Index(sseDeUnTurno, `data: {"type":"message_delta"`)
	if hasta < 0 {
		t.Fatal("el fixture ya no tiene un message_delta: este caso dejó de medir")
	}
	sinCorte := sseDeUnTurno[:hasta] +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`
	c4, _, _ := servidorSSE(t, sinCorte)
	var evs4 []Evento
	if err := c4.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs4 = append(evs4, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno sin la línea vacía final: %v", err)
	}
	if len(evs4) == 0 || evs4[len(evs4)-1].Type != EvDone {
		t.Errorf("sin el \\n final se perdió el cierre: %v", tipos(evs4))
	}
}

// EL ERROR DEL PROVEEDOR A MITAD DEL STREAM. La API puede mandar un `event: error` con la
// respuesta ya empezada (sobrecarga, corte). Sin tratarlo, el request se queda esperando un
// `message_delta` que no va a llegar.
func TestUnErrorEnMedioDelStreamCierraElTurno(t *testing.T) {
	conError := `data: {"type":"message_start","message":{"usage":{"input_tokens":9}}}

data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}

`
	c, _, _ := servidorSSE(t, conError)
	var evs []Evento
	if err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs = append(evs, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno: %v", err)
	}
	fin := evs[len(evs)-1]
	if fin.Type != EvError {
		t.Fatalf("el último evento tiene que ser error: %v", tipos(evs))
	}
	if !strings.Contains(fin.ErrorMessage, "Overloaded") {
		t.Errorf("el mensaje del proveedor tiene que llegar al cliente: %q", fin.ErrorMessage)
	}
}

// EL MISMO INVARIANTE QUE EL LANZADOR, y está acá porque los dos motores lo resuelven en su
// propio switch: el traductor marca el turno como cerrado ANTES de emitir su último evento,
// así que un `emitir` que falle en el `done` deja cerrado=true Y un error. Si `cerrado`
// ganara, Turno devolvería nil y el llamador creería que sirvió un turno que el cliente no
// recibió entero.
func TestEnLaAPIElErrorDelConsumidorTambienLeGanaAlCierre(t *testing.T) {
	roto := errors.New("el transporte se cayó")
	c, _, _ := servidorSSE(t, sseDeUnTurno)

	err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		if e.Type == EvDone {
			return roto
		}
		return nil
	})
	if err == nil {
		t.Fatal("un turno cuyo done no se pudo entregar no se sirvió: no puede volver nil")
	}
	if !errors.Is(err, roto) || !errors.Is(err, ErrDelConsumidor) {
		t.Errorf("el error tiene que viajar envuelto y marcado: %v", err)
	}
}

// EL CANCEL DEL LLAMADOR, igual que en el Lanzador: en un servidor es el caso MÁS frecuente
// —un cliente que cierra la conexión— y tiene que decir eso, no parecer una falla del
// proveedor.
func TestEnLaAPIElCancelDelLlamadorNoParaceUnaFalla(t *testing.T) {
	// El servidor manda el arranque y después se queda callado, para que el cancel llegue
	// con el stream abierto.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// El cuerpo se drena por lo que explica TestUnCancelAntesDeLasCabecerasTambienSeNombra:
		// sin eso el handler no se entera nunca de que el cliente se fue.
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{}}}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancelar := context.WithCancel(context.Background())
	c := &ClienteAPI{Clave: "k", Endpoint: srv.URL, HTTP: srv.Client()}
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancelar()
	}()

	arranque := time.Now()
	err := c.Turno(ctx, pedidoAPIDePrueba(), func(Evento) error { return nil })
	if tardo := time.Since(arranque); tardo > 5*time.Second {
		t.Errorf("tardó %s: ignoró el cancel", tardo.Round(time.Millisecond))
	}
	if err == nil {
		t.Fatal("un turno que no se sirvió no puede volver nil")
	}
	if !strings.Contains(err.Error(), "canceló") {
		t.Errorf("el error tiene que decir que lo cortó el llamador: %v", err)
	}
}

// EL CANCEL QUE LLEGA ANTES DE LAS CABECERAS es OTRO camino de salida, y en un servidor es
// el más frecuente de los dos: un cliente que se va antes del primer token. Acá el error
// vuelve del transporte —de Do()— y no del scanner, así que lo resuelve un `if` distinto; si
// sólo estuviera cubierto el del medio del stream, este diría «pedirle el turno a …» y una
// desconexión normal se leería como una falla del proveedor.
func TestUnCancelAntesDeLasCabecerasTambienSeNombra(t *testing.T) {
	// El servidor acepta la conexión y no contesta NADA.
	//
	// ⚠️ EL `io.Copy(io.Discard, r.Body)` NO ES DECORATIVO, y colgó este test 274 s con 0 s de
	// CPU antes de estar acá. El `http.Server` de Go arranca la lectura de fondo que detecta al
	// cliente desconectado RECIÉN cuando el cuerpo del request llega a EOF
	// (`registerOnHitEOF`, net/http/server.go). Un handler que no drena el cuerpo nunca se
	// entera de que el cliente se fue: `r.Context()` no se cierra nunca, el handler queda
	// colgado y `httptest.Server.Close()` lo espera para siempre — y cuelga el paquete entero,
	// no sólo este test. Un servidor de verdad lee el cuerpo, así que drenarlo además es lo
	// fiel.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := &ClienteAPI{Clave: "k", Endpoint: srv.URL, HTTP: srv.Client()}
	ctx, cancelar := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancelar()
	}()
	err := c.Turno(ctx, pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err == nil {
		t.Fatal("un turno que no se sirvió no puede volver nil")
	}
	if !strings.Contains(err.Error(), "canceló") {
		t.Errorf("el error tiene que decir que lo cortó el llamador: %v", err)
	}

	// Y el plazo vencido dice OTRA cosa, porque el arreglo es otro: subirle el tiempo, no
	// buscar a un cliente que no se fue.
	ctx2, cancelar2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelar2()
	err2 := c.Turno(ctx2, pedidoAPIDePrueba(), func(Evento) error { return nil })
	if err2 == nil || !strings.Contains(err2.Error(), "tiempo") {
		t.Errorf("un plazo vencido tiene que nombrar el tiempo: %v", err2)
	}
}

// cortarCampo come UN solo espacio tras los dos puntos, que es lo que la spec de SSE define
// como separador. Comerlos todos corrompería un valor que arranque con espacios, y un prefijo
// sin los dos puntos NO es ese campo: `datos: x` no es un `data`.
func TestCortarCampoRespetaElValor(t *testing.T) {
	casos := []struct {
		linea string
		valor string
		es    bool
	}{
		{`data: {"a":1}`, `{"a":1}`, true},
		{`data:{"a":1}`, `{"a":1}`, true}, // sin espacio: válido por spec
		{"data:   hola", "  hola", true},  // UN espacio es separador; los otros son el valor
		{"data", "", false},               // sin los dos puntos no es el campo
		{"datos: x", "", false},           // otro campo que arranca igual
		{": latido", "", false},           // un comentario no es un data
	}
	for _, c := range casos {
		valor, es := cortarCampo(c.linea, "data")
		if es != c.es || valor != c.valor {
			t.Errorf("cortarCampo(%q): got (%q,%v), want (%q,%v)", c.linea, valor, es, c.valor, c.es)
		}
	}
}

// 🔴 EL TIMEOUT DEL CLIENTE HTTP VA EN CERO, y no es un olvido: http.Client.Timeout cubre la
// vida ENTERA del request, lectura del cuerpo incluida. En un stream que dura lo que dura el
// turno, un timeout de cliente corta la respuesta a mitad de camino y el cliente remoto ve un
// turno truncado sin ningún error que lo explique. El tope de un turno es trabajo del ctx que
// trae el llamador.
func TestElClienteHTTPNoLePoneTimeoutAlStream(t *testing.T) {
	c := &ClienteAPI{}
	if got := c.clienteHTTP().Timeout; got != 0 {
		t.Errorf("Timeout del cliente: got %s, want 0 — cortaría el stream a mitad del turno", got)
	}

	// Lo que sí se acota es lo de ANTES del stream, que no crece con el largo de la
	// respuesta. Sin esto, un endpoint que acepta la conexión y no contesta cuelga el turno
	// hasta que el ctx del llamador lo corte.
	tr, ok := c.clienteHTTP().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport: %T", c.clienteHTTP().Transport)
	}
	if tr.ResponseHeaderTimeout == 0 || tr.TLSHandshakeTimeout == 0 {
		t.Errorf("las esperas previas al stream sí van acotadas: header=%s tls=%s",
			tr.ResponseHeaderTimeout, tr.TLSHandshakeTimeout)
	}

	// Un cliente provisto por el llamador se respeta tal cual: es su decisión.
	mio := &http.Client{Timeout: time.Second}
	if (&ClienteAPI{HTTP: mio}).clienteHTTP() != mio {
		t.Error("el cliente del llamador no se reemplaza")
	}
}

// EL STREAM SE CORTA DE VERDAD. El traductor para en el primer `message_delta` y volvemos con
// el cuerpo a medio leer: cerrarlo le dice al otro extremo que no mande más. Sin eso, cada
// turno deja una conexión colgada escribiendo bytes que nadie lee.
func TestAlCerrarElTurnoSeCortaLaConexion(t *testing.T) {
	// El servidor manda el turno y después sigue escribiendo para siempre. `escrituras`
	// cuenta hasta dónde llegó: si el cliente cierra, el write falla y el loop termina.
	fin := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, sseDeUnTurno)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		escrituras := 0
		for {
			if _, err := io.WriteString(w, "data: {\"type\":\"ping\"}\n\n"); err != nil {
				break
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			escrituras++
			if escrituras > 100000 {
				break // el cliente no cortó: se corta acá para no colgar el test
			}
		}
		fin <- escrituras
	}))
	defer srv.Close()

	c := &ClienteAPI{Clave: "k", Endpoint: srv.URL, HTTP: srv.Client()}
	var evs []Evento
	if err := c.Turno(context.Background(), pedidoAPIDePrueba(), func(e Evento) error {
		evs = append(evs, e)
		return nil
	}); err != nil {
		t.Fatalf("Turno: %v", err)
	}
	// El turno salió completo y NO incluyó nada de lo que vino después del cierre.
	if got := tipos(evs); got[len(got)-1] != EvDone || len(got) != 6 {
		t.Errorf("el turno no cerró en el done: %v", got)
	}

	select {
	case escrituras := <-fin:
		if escrituras > 100000 {
			t.Error("el servidor nunca vio el cierre: la conexión quedó colgada")
		}
	case <-time.After(10 * time.Second):
		t.Error("el servidor sigue escribiendo: el cuerpo no se cerró")
	}
}

// UN max_tokens NEGATIVO NO VIAJA. cuerpoDelRequest existe para atrapar justo lo que la API
// rechazaría con un 400 que cuesta más diagnosticar que prevenir, y un negativo es ese caso:
// el cero cae al defecto, pero el -1 —que sale de un yaml o de un pi mal poblado— se serializa
// tal cual, gasta el request y vuelve con un error del otro lado. La guarda de thinking de más
// abajo SÍ lo atrapa, pero sólo cuando hay thinking: sin él no hay nada entre el -1 y la red.
//
// Sabotaje M41: un max_tokens negativo se serializa tal cual y viaja a la API
// arnes: archivo="internal/motor/api.go"
// arnes: de="\ttope := p.MaxTokens\n\tif tope <= 0 {"
// arnes: a="\ttope := p.MaxTokens\n\tif tope == 0 {"
// arnes: colision_ok="TestNingunMaxTokensNegativoViajaALaAPI"
// El de abajo pisa a este en el mismo `if`, y las dos son guardas de verdad: esta mide el -1
// que sale del yaml, la otra el dominio entero. Medido el 2026-10-29: con la guarda curando
// `== 0 || == -1` este test queda VERDE y el otro ROJO, o sea que cubren cosas distintas.
func TestUnMaxTokensNegativoNoViajaALaAPI(t *testing.T) {
	cuerpo, err := cuerpoDelRequest(Pedido{
		Modelo:    "m",
		MaxTokens: -1,
		Mensajes:  []Mensaje{TextoDeUsuario("hola")},
	})
	if err != nil {
		t.Fatalf("cuerpoDelRequest: %v", err)
	}
	var visto struct {
		MaxTokens int `json:"max_tokens"`
	}
	if err := json.Unmarshal(cuerpo, &visto); err != nil {
		t.Fatal(err)
	}
	if visto.MaxTokens <= 0 {
		t.Fatalf("max_tokens viajó en %d: la API contesta 400 y el turno se pierde", visto.MaxTokens)
	}
	if visto.MaxTokens != MaxTokensPorDefecto {
		t.Errorf("max_tokens = %d, quiero el defecto %d", visto.MaxTokens, MaxTokensPorDefecto)
	}
}

// NINGÚN max_tokens NEGATIVO VIAJA, y el plural es el invariante. El test de arriba mide el -1
// porque es el valor que sale de un yaml escrito a mano, pero la guarda no dice «el -1 cae al
// defecto», dice «cualquier valor que la API rechazaría cae al defecto»: una guarda que curara
// sólo el -1 dejaría pasar el -2 de un `pi` mal poblado, que se serializa tal cual y vuelve con
// el mismo 400 — medido con una sonda de mutación el 2026-10-29, y el paquete entero quedaba
// verde. El dominio se recorre entero porque un test de PUNTO no distingue «cura el cero y el
// negativo» de «cura el cero y el -1».
//
// Sabotaje M44: la guarda del tope cura el cero y el -1, pero deja pasar el resto del dominio
// arnes: archivo="internal/motor/api.go"
// arnes: de="\ttope := p.MaxTokens\n\tif tope <= 0 {"
// arnes: a="\ttope := p.MaxTokens\n\tif tope == 0 || tope == -1 {"
// arnes: colision_ok="TestUnMaxTokensNegativoNoViajaALaAPI"
func TestNingunMaxTokensNegativoViajaALaAPI(t *testing.T) {
	for _, caso := range []struct {
		nombre string
		tope   int
	}{
		{"el -1 que sale de un yaml a mano", -1},
		{"un -2, que el test de punto no cubre", -2},
		{"el -7 de un pi mal poblado", -7},
		{"el negativo del propio defecto", -MaxTokensPorDefecto},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			cuerpo, err := cuerpoDelRequest(Pedido{
				Modelo:    "m",
				MaxTokens: caso.tope,
				Mensajes:  []Mensaje{TextoDeUsuario("hola")},
			})
			if err != nil {
				t.Fatalf("cuerpoDelRequest: %v", err)
			}
			var visto struct {
				MaxTokens int `json:"max_tokens"`
			}
			if err := json.Unmarshal(cuerpo, &visto); err != nil {
				t.Fatal(err)
			}
			// La igualdad y no un `> 0`: el valor del defecto es parte del invariante, y una cota
			// deja pasar cualquier positivo —incluido un 1 que mata el turno igual de rápido—.
			if visto.MaxTokens != MaxTokensPorDefecto {
				t.Errorf("MaxTokens=%d viajó como %d, quiero el defecto %d",
					caso.tope, visto.MaxTokens, MaxTokensPorDefecto)
			}
		})
	}
}
