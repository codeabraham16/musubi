package motor

import (
	"encoding/json"
	"fmt"
)

// --- La forma de entrada.
//
// El traductor tiene DOS puertas, y es la pieza que hace que los dos motores compartan
// todo lo demás:
//
//   - Linea: una línea de `claude -p --output-format stream-json
//     --include-partial-messages`. Los eventos que importan vienen ENVUELTOS como
//     {"type":"stream_event","event":{...}} y adentro traen el evento CRUDO de la Messages
//     API, sin una sola clave propia del CLI (verificado sobre capturas reales: los únicos
//     `event.type` que aparecen son los seis de la API). Los demás sobres (`system`,
//     `assistant`, `user`, `rate_limit_event`, `result`) son del harness y no se traducen,
//     salvo el `result` cuando hay que reportar un error.
//   - EventoAPI: el evento de la Messages API ya desenvuelto, como lo emite el SSE de
//     /v1/messages. Es la MISMA función, sin el sobre.
//
// Que la envoltura sea lo único distinto no es casualidad ni suerte: es la razón por la que
// cambiar de motor no cuesta el traductor.

type sobreClaude struct {
	Type    string          `json:"type"`
	Event   json.RawMessage `json:"event"`
	Subtype string          `json:"subtype"`
	IsError bool            `json:"is_error"`
	Result  string          `json:"result"`

	// Usage del `result` del harness. Es el único consumo disponible cuando el proceso
	// murió sin cerrar un turno; sin él, el error final declara cero tokens y el cliente
	// cree que el intento fallido salió gratis.
	Usage *usoCrudo `json:"usage"`
}

type bloqueCrudo struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`

	// Data es el payload opaco de un `redacted_thinking`. Viaja como la FIRMA del bloque
	// de thinking, no como su texto: el cliente lo guarda en `thinkingSignature`
	// (anthropic-messages.ts:750) y lo devuelve tal cual como `redacted_thinking.data` en
	// el request siguiente (:1446-1447). Perderlo le impide reconstruir el turno.
	Data string `json:"data"`
}

type deltaCrudo struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Thinking    string `json:"thinking"`
	Signature   string `json:"signature"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

type usoCrudo struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// errorCrudo es el cuerpo del evento `error` del stream de la Messages API
// ({"type":"error","error":{"type":"overloaded_error","message":"..."}}). Es el evento que
// llega cuando el proveedor se cae A MITAD de una respuesta.
type errorCrudo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type eventoCrudo struct {
	Type         string       `json:"type"`
	Index        *int         `json:"index"`
	ContentBlock *bloqueCrudo `json:"content_block"`
	Delta        *deltaCrudo  `json:"delta"`
	Usage        *usoCrudo    `json:"usage"`
	Error        *errorCrudo  `json:"error"`
	Message      *struct {
		Usage *usoCrudo `json:"usage"`
	} `json:"message"`
}

// --- El traductor

// bloqueAbierto es el estado que hay que llevar por índice de bloque. Sin esto no se puede
// traducir un `content_block_stop`, que en la Messages API NO dice de qué tipo de bloque
// es: viene pelado con su índice. Y errar ese tipo no degrada, ROMPE — el cliente tira
// `Received text_end for non-text content` (proxy.ts:294) y mata el stream.
type bloqueAbierto struct {
	kind      string // "text" | "thinking" | "tool_use"
	id        string // sólo tool_use
	nombre    string // sólo tool_use
	argumen   []byte // partial_json acumulado del tool_use
	firma     string // signature acumulada del thinking, o el `data` del redacted_thinking
	emitidoOK bool   // ya se emitió su *_start

	// indiceCliente es el índice DENSO con el que viaja este bloque, que no siempre es el
	// de la API: ver densificarIndice.
	indiceCliente int
}

// Traductor convierte el stream de un turno de modelo en eventos del protocolo del proxy.
// Tiene estado (los bloques abiertos, el usage parcial y si el turno ya cerró), así que NO
// es seguro compartirlo entre requests: uno por request.
type Traductor struct {
	bloques   map[int]*bloqueAbierto
	cerrado   bool // ya se emitió el done/error: todo lo que sigue se descarta
	arrancado bool // ya se emitió el start

	// proximoIndice es el próximo índice denso a entregar. Ver densificarIndice.
	proximoIndice int

	// usoInicial es el usage del `message_start`, que es el ÚNICO lugar donde llegan los
	// tokens de entrada y de caché. El `message_delta` final puede traer sólo
	// `output_tokens`, y entonces reportar su usage pelado declararía `input: 0` teniendo
	// el dato guardado acá.
	usoInicial Usage
}

// NuevoTraductor devuelve un traductor listo para un turno.
func NuevoTraductor() *Traductor {
	return &Traductor{bloques: make(map[int]*bloqueAbierto)}
}

// Cerrado indica que el turno ya terminó: el llamador tiene que dejar de leer y matar el
// proceso. No es una optimización — `claude -p` es un agente con loop propio y seguiría
// emitiendo un segundo `message_start` que el cliente no espera.
func (t *Traductor) Cerrado() bool { return t.cerrado }

// Linea traduce una línea del stream de `claude -p` y devuelve los eventos que le
// corresponden (puede ser ninguno). Una línea ilegible NO aborta el turno: se devuelve el
// error para que el llamador lo registre y siga, porque el harness intercala líneas que no
// son del protocolo y una sola mal formada no debería tirar una respuesta entera.
func (t *Traductor) Linea(raw []byte) ([]Evento, error) {
	if t.cerrado {
		return nil, nil
	}
	var sobre sobreClaude
	if err := json.Unmarshal(raw, &sobre); err != nil {
		return nil, fmt.Errorf("línea no es JSON: %w", err)
	}

	switch sobre.Type {
	case "stream_event":
		return t.streamEvent(sobre.Event)
	case "result":
		// Sólo se llega acá si el proceso terminó SIN haber cerrado un turno (si el
		// turno cerró bien, t.cerrado ya es true y se descartó todo). O sea: un result
		// visible es siempre una salida anómala.
		return t.resultado(sobre), nil
	}
	return nil, nil
}

// EventoAPI traduce un evento de la Messages API ya desenvuelto: es la puerta del motor que
// le habla a /v1/messages directo, sin el sobre del CLI. Mismo cuerpo, mismos invariantes.
func (t *Traductor) EventoAPI(raw []byte) ([]Evento, error) {
	if t.cerrado {
		return nil, nil
	}
	return t.streamEvent(raw)
}

func (t *Traductor) streamEvent(raw json.RawMessage) ([]Evento, error) {
	var ev eventoCrudo
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, fmt.Errorf("stream_event ilegible: %w", err)
	}

	switch ev.Type {
	case "message_start":
		if t.arrancado {
			// Un segundo message_start significa que el loop de `claude -p` arrancó
			// otro turno sin que nadie lo cortara. Se ignora en vez de emitir un
			// segundo `start`, que dejaría al cliente reconstruyendo dos mensajes
			// sobre el mismo parcial.
			return nil, nil
		}
		t.arrancado = true
		if ev.Message != nil {
			t.usoInicial = traducirUso(ev.Message.Usage)
		}
		return []Evento{{Type: EvStart}}, nil

	case "content_block_start":
		return t.bloqueAbre(ev)

	case "content_block_delta":
		return t.bloqueDelta(ev)

	case "content_block_stop":
		return t.bloqueCierra(ev)

	case "message_delta":
		return t.turnoCierra(ev), nil

	case "error":
		// El proveedor se cayó a mitad del stream (un `overloaded_error`, un
		// `api_error`). SIN este case el evento caía al return de abajo y se ignoraba:
		// el stream seguía abierto sin que llegara nunca un `message_delta`, así que el
		// request se colgaba hasta el tope de tiempo —diez minutos por defecto— y el
		// cliente terminaba con el fallback genérico de desconexión en vez del motivo.
		return t.errorDeLaAPI(ev), nil
	}
	// message_stop, ping y cualquier evento nuevo de la API se ignoran: el cierre del turno
	// lo marca el message_delta, que es el único que trae stop_reason y usage.
	return nil, nil
}

// densificarIndice devuelve el índice con el que un bloque ANUNCIADO viaja al cliente. No
// es el de la API, y la diferencia importa: el cliente hace `partial.content[contentIndex] =
// bloque` (proxy.ts:266) sobre un array, así que un hueco queda como `null`. Si la API abre
// en el índice 0 un bloque que este traductor no anuncia —un tipo nuevo, un
// `web_search_tool_result`— y el texto llega en el índice 1, el cliente arma
// `content = [null, {...}]` y el consumidor de ese array se encuentra un agujero que nadie
// puso ahí. Entregando índices densos y propios, el cliente siempre ve 0,1,2…
func (t *Traductor) densificarIndice() int {
	idx := t.proximoIndice
	t.proximoIndice++
	return idx
}

func (t *Traductor) bloqueAbre(ev eventoCrudo) ([]Evento, error) {
	if ev.Index == nil || ev.ContentBlock == nil {
		return nil, fmt.Errorf("content_block_start sin index o sin content_block")
	}
	idx := *ev.Index

	// LA VALIDACIÓN VA ANTES DE TOCAR EL MAPA. Un tool_use inválido que se registrara
	// primero reemplazaría al bloque que estuviera abierto en ese índice, y entonces el
	// `content_block_stop` siguiente cerraría el inválido —que no emite nada— y el bloque
	// legítimo se quedaría abierto para siempre del lado del cliente.
	kind := ev.ContentBlock.Type
	if kind == "tool_use" || kind == "server_tool_use" {
		if ev.ContentBlock.ID == "" || ev.ContentBlock.Name == "" {
			// El cliente necesita los dos para armar el bloque (proxy.ts:330-336).
			// Sin ellos el toolcall_end no tiene a qué hacerle Object.assign.
			return nil, fmt.Errorf("tool_use en el índice %d sin id o sin name", idx)
		}
	}

	// El bloque se construye de cero y REEMPLAZA al que hubiera en ese índice, en vez de
	// reusarlo para ahorrarse una asignación. Importa porque un stream de varios turnos
	// reusa los índices de bloque, y un acumulador heredado pega los argumentos de dos
	// tool calls distintas y produce JSON inválido.
	//
	// Esto y el `delete` de bloqueCierra son DOS guardas del mismo invariante, y son
	// redundantes: medido por sabotaje, rompiendo una sola los tests quedan en verde, y el
	// invariante se cae recién cuando se rompen las dos. O sea que quien toque una de las
	// dos no va a ver nada fallar — el test que las cubre está en
	// TestBloquesConsecutivosEnElMismoIndiceNoMezclan.
	b := &bloqueAbierto{kind: kind, id: ev.ContentBlock.ID, nombre: ev.ContentBlock.Name}
	t.bloques[idx] = b

	switch kind {
	case "text":
		b.emitidoOK = true
		b.indiceCliente = t.densificarIndice()
		return []Evento{{Type: EvTextStart, ContentIndex: b.indiceCliente}}, nil

	case "thinking":
		b.emitidoOK = true
		b.indiceCliente = t.densificarIndice()
		return []Evento{{Type: EvThinkingStart, ContentIndex: b.indiceCliente}}, nil

	case "redacted_thinking":
		// El razonamiento viene cifrado: no hay texto que mostrar, pero el payload opaco
		// SÍ tiene que llegar, porque el cliente lo devuelve como `redacted_thinking.data`
		// en el request siguiente y sin él la API rechaza el turno. Va por el canal de la
		// firma, que es exactamente donde el cliente lo guarda, y el texto es el mismo
		// placeholder que usa pi para que la interfaz muestre que hubo razonamiento
		// (anthropic-messages.ts:746-752).
		b.emitidoOK = true
		b.firma = ev.ContentBlock.Data
		b.indiceCliente = t.densificarIndice()
		return []Evento{
			{Type: EvThinkingStart, ContentIndex: b.indiceCliente},
			{Type: EvThinkingDelta, ContentIndex: b.indiceCliente, Delta: razonamientoOculto},
		}, nil

	case "tool_use", "server_tool_use":
		b.emitidoOK = true
		b.indiceCliente = t.densificarIndice()
		return []Evento{{Type: EvToolCallStart, ContentIndex: b.indiceCliente, ID: b.id, ToolName: b.nombre}}, nil
	}
	// Un tipo de bloque que el protocolo no tiene (los hay nuevos cada tanto) se registra
	// pero no se anuncia: así su `stop` tampoco emite nada, el cliente nunca ve un bloque a
	// medio abrir, y no consume un índice denso.
	return nil, nil
}

// razonamientoOculto es el texto que ve el usuario cuando el modelo razonó pero el
// razonamiento viene cifrado. Es literalmente el de pi (anthropic-messages.ts:749) para que
// las dos vías muestren lo mismo.
const razonamientoOculto = "[Reasoning redacted]"

func (t *Traductor) bloqueDelta(ev eventoCrudo) ([]Evento, error) {
	if ev.Index == nil || ev.Delta == nil {
		return nil, fmt.Errorf("content_block_delta sin index o sin delta")
	}
	idx := *ev.Index
	b := t.bloques[idx]
	if b == nil {
		return nil, fmt.Errorf("content_block_delta en el índice %d sin su start", idx)
	}
	if !b.emitidoOK {
		// El bloque existe pero nunca se anunció (tipo desconocido). Emitir su delta
		// sería mandar un `text_delta` sin su `text_start`, y eso NO degrada: el cliente
		// tira `Received text_delta for non-text content` (proxy.ts:352) y el catch de
		// 233-243 lo deja con stopReason=error y content vacío — o sea que un bloque que
		// no entendimos se lleva puesto el turno entero.
		return nil, nil
	}
	d := ev.Delta
	salida := b.indiceCliente

	switch {
	case d.Text != "":
		return []Evento{{Type: EvTextDelta, ContentIndex: salida, Delta: d.Text}}, nil

	case d.Thinking != "":
		return []Evento{{Type: EvThinkingDelta, ContentIndex: salida, Delta: d.Thinking}}, nil

	case d.Signature != "":
		// La firma NO es un delta de contenido: viene en su propio evento (medido: 740
		// caracteres de una sola vez) y el protocolo la quiere en el `thinking_end`.
		// Se acumula y se emite al cerrar.
		b.firma += d.Signature
		return nil, nil

	case d.PartialJSON != "":
		b.argumen = append(b.argumen, d.PartialJSON...)
		return []Evento{{Type: EvToolCallDelta, ContentIndex: salida, Delta: d.PartialJSON}}, nil
	}
	// Un delta vacío no se emite. El primer `partial_json` de cada tool call viene con
	// cadena vacía (medido), y reenviarlo sería un evento que no mueve nada en el cliente.
	return nil, nil
}

func (t *Traductor) bloqueCierra(ev eventoCrudo) ([]Evento, error) {
	if ev.Index == nil {
		return nil, fmt.Errorf("content_block_stop sin index")
	}
	idx := *ev.Index
	b := t.bloques[idx]
	if b == nil {
		// Cerrar un bloque que nunca se abrió sería adivinar su tipo, y adivinar mal
		// hace que el cliente tire una excepción y pierda la respuesta entera. Se
		// reporta y no se emite nada.
		return nil, fmt.Errorf("content_block_stop en el índice %d sin su start", idx)
	}
	// Saca el bloque del mapa: no acumula los cerrados durante un turno largo, y es la
	// segunda de las dos guardas redundantes del invariante que explica bloqueAbre. Sacar
	// SÓLO esta línea no rompe ningún test (medido), así que el comentario de allá es el
	// que avisa.
	delete(t.bloques, idx)
	if !b.emitidoOK {
		return nil, nil
	}
	salida := b.indiceCliente

	switch b.kind {
	case "text":
		return []Evento{{Type: EvTextEnd, ContentIndex: salida}}, nil
	case "thinking", "redacted_thinking":
		return []Evento{{Type: EvThinkingEnd, ContentIndex: salida, ContentSignature: b.firma}}, nil
	case "tool_use", "server_tool_use":
		args, errArgs := argumentos(b.argumen)
		return []Evento{{
			Type:         EvToolCallEnd,
			ContentIndex: salida,
			ToolCall: &ToolCall{
				Type:           "toolCall",
				ID:             b.id,
				Name:           b.nombre,
				Arguments:      args,
				ArgumentsError: errArgs,
			},
		}}, nil
	}
	return nil, nil
}

// argumentos devuelve el partial_json acumulado si es JSON válido; si no, `{}` MÁS el texto
// del problema, que es lo que el protocolo pide para este caso exacto.
//
// Por qué no basta con `{}`: el cliente hace `Object.assign` del toolCall que llega sobre el
// que venía reconstruyendo delta por delta (proxy.ts:358), así que un `arguments:{}` pisa su
// parcial y borra la evidencia de que los argumentos estaban truncados. Su única guarda
// —`if (toolCall.argumentsError) throw` en validation.ts:318-319— mira un campo que nunca
// llegaba, y entonces la tool se validaba y ejecutaba CON UN OBJETO VACÍO en vez de pedirle
// al modelo que la llamara de nuevo. Es alcanzable de verdad: el CLI manda max_tokens:64000
// y un tool call cortado por el tope sale con el JSON a medias.
//
// El texto imita el de pi (json-parse.ts:117) para que las dos vías le reporten lo mismo al
// modelo.
func argumentos(acumulado []byte) (json.RawMessage, string) {
	if len(acumulado) == 0 {
		return json.RawMessage("{}"), ""
	}
	if !json.Valid(acumulado) {
		return json.RawMessage("{}"), fmt.Sprintf(
			"los argumentos no son un objeto JSON válido\n\nReceived arguments:\n%s", acumulado)
	}
	return json.RawMessage(acumulado), ""
}

// turnoCierra traduce el message_delta, que es el ÚNICO evento que trae stop_reason y el
// usage final. Marca el turno como cerrado: a partir de acá el llamador corta.
func (t *Traductor) turnoCierra(ev eventoCrudo) []Evento {
	t.cerrado = true
	uso := t.usoFinal(ev.Usage)

	bruto := ""
	if ev.Delta != nil {
		bruto = ev.Delta.StopReason
	}
	razon, ok := razonDeCorte(bruto)
	if !ok {
		// Una razón que el protocolo no admite en `done` (un refusal, un `sensitive`) se
		// reporta como error CON su nombre, en vez de disfrazarse de "stop". El cliente
		// tiene que poder distinguir «terminó» de «no terminó y no sabemos» — y pi mismo
		// hace lo propio: su mapStopReason TIRA ante una razón que no conoce
		// (anthropic-messages.ts:1686-1688).
		return []Evento{{
			Type:         EvError,
			Reason:       ReasonError,
			ErrorMessage: fmt.Sprintf("stop_reason no soportado por el protocolo: %q", bruto),
			Usage:        &uso,
		}}
	}
	return []Evento{{Type: EvDone, Reason: razon, Usage: &uso}}
}

// usoFinal fusiona el usage del `message_start` con el del `message_delta`. El del delta
// gana campo por campo, pero sólo donde trae un número: la API puede mandar ahí sólo
// `output_tokens`, y en ese caso los tokens de entrada y de caché siguen siendo los del
// arranque. Sin la fusión, un turno que consumió 15.842 tokens de caché leída se reportaba
// con cacheRead en cero y el contador del cliente subdeclaraba el consumo real.
func (t *Traductor) usoFinal(u *usoCrudo) Usage {
	final := t.usoInicial
	if u != nil {
		del := traducirUso(u)
		if del.Input > 0 {
			final.Input = del.Input
		}
		if del.Output > 0 {
			final.Output = del.Output
		}
		if del.CacheRead > 0 {
			final.CacheRead = del.CacheRead
		}
		if del.CacheWrite > 0 {
			final.CacheWrite = del.CacheWrite
		}
	}
	final.TotalTokens = final.Input + final.Output + final.CacheRead + final.CacheWrite
	return final
}

// errorDeLaAPI traduce el evento `error` del stream. Cierra el turno: después de que el
// proveedor falla no llega ningún `message_delta`, así que esperarlo es colgarse.
func (t *Traductor) errorDeLaAPI(ev eventoCrudo) []Evento {
	t.cerrado = true
	msg := "el proveedor cortó el stream"
	if ev.Error != nil {
		if ev.Error.Message != "" && ev.Error.Type != "" {
			msg = fmt.Sprintf("%s: %s", ev.Error.Type, ev.Error.Message)
		} else if ev.Error.Message != "" {
			msg = ev.Error.Message
		} else if ev.Error.Type != "" {
			msg = ev.Error.Type
		}
	}
	// El usage parcial va igual: los tokens de entrada ya se consumieron y se cobraron
	// aunque la respuesta no llegara.
	uso := t.usoFinal(nil)
	return []Evento{{Type: EvError, Reason: ReasonError, ErrorMessage: msg, Usage: &uso}}
}

// razonDeCorte mapea el stop_reason de la Messages API al conjunto cerrado que el `done`
// del protocolo admite (proxy.ts:36). Devuelve ok=false para todo lo demás.
//
// El mapeo sigue al de pi (anthropic-messages.ts:1664-1690) a propósito, para que un cliente
// no vea una respuesta distinta según de qué lado le hablen.
func razonDeCorte(bruto string) (string, bool) {
	switch bruto {
	case "end_turn", "stop_sequence":
		return ReasonStop, true
	case "tool_use":
		return ReasonToolUse, true
	case "max_tokens":
		return ReasonLength, true
	case "pause_turn":
		// El turno quedó en pausa a mitad de una tool de servidor y la API espera que se
		// reenvíe tal cual. Para el cliente eso es un corte limpio: "stop is good enough
		// -> resubmit" (anthropic-messages.ts:1680-1681). Reportarlo como error lo dejaba
		// terminando en falla donde su propio proveedor termina bien.
		return ReasonStop, true
	}
	return "", false
}

// resultado traduce el `result` final del harness, al que sólo se llega cuando el proceso
// murió sin cerrar un turno. Siempre es un error desde la vista del cliente: no hubo
// respuesta del modelo.
func (t *Traductor) resultado(s sobreClaude) []Evento {
	t.cerrado = true
	msg := s.Subtype
	if msg == "" {
		msg = "el proceso terminó sin respuesta del modelo"
	}
	if s.Result != "" {
		msg += ": " + s.Result
	}
	if !s.IsError && s.Subtype != "" {
		// El harness dice que salió bien y sin embargo no cerró un turno. Vale aclararlo:
		// el mensaje a secas se lee como un crash y manda a buscar la falla al lado
		// equivocado.
		msg += " (el proceso no reportó error: el turno nunca llegó a cerrarse)"
	}
	// El usage del `result` es el único consumo disponible en este camino. Se reporta
	// igual: un intento que falló a mitad ya gastó los tokens de entrada.
	uso := t.usoFinal(s.Usage)
	return []Evento{{Type: EvError, Reason: ReasonError, ErrorMessage: msg, Usage: &uso}}
}

// traducirUso pasa los nombres de la Messages API a los de pi. TotalTokens se calcula
// acá porque el stream no lo trae y el cliente lo muestra tal cual.
func traducirUso(u *usoCrudo) Usage {
	if u == nil {
		return Usage{}
	}
	return Usage{
		Input:       u.InputTokens,
		Output:      u.OutputTokens,
		CacheRead:   u.CacheReadInputTokens,
		CacheWrite:  u.CacheCreationInputTokens,
		TotalTokens: u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
	}
}
