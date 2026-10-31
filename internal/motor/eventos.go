// Package motor sirve un TURNO DE MODELO a un cliente remoto que trae su propio loop de
// agente. Nace para que Musubi pueda ser el servidor del `streamProxy` de pi-agent-core
// (el motor que usa ThinkRail), y así alimentar a ese cliente con la suscripción del
// usuario en vez de una API key.
//
// POR QUÉ EXISTE, Y POR QUÉ NO ES UN PROXY MÁS. Un cliente de pi puede apuntar su
// `streamFn` a `streamProxy(model, context, {proxyUrl, authToken})`, que hace
// `POST {proxyUrl}/api/stream` y consume SSE. Del otro lado, Musubi lanza el binario
// `claude` en modo headless (`claude -p --output-format stream-json
// --include-partial-messages`) y TRADUCE su salida a los eventos que ese cliente espera.
// La diferencia con pegarle a la Messages API desde código propio no es técnica, es de
// permiso: el binario es la app del usuario y la suscripción aplica, mientras que un
// cliente de terceros que se hace pasar por `claude-cli` recibe un 400.
//
// EL NUDO QUE RESUELVE EL TRADUCTOR. `claude -p` NO es un proveedor de un turno: es un
// agente con su propio loop, y medido el 2026-10-28 una sola corrida emitió DOS
// `message_start` (ejecutó una tool y volvió a llamar al modelo). El protocolo del proxy
// espera UNA respuesta por request. Por eso el traductor corta en el primer
// `message_delta` y descarta lo que venga después: el turno completo del modelo ya llegó
// ahí, antes de que el harness ejecute nada. El corte NO se puede delegar a `--max-turns 1`,
// que significa «un turno de USUARIO» y alcanza a ejecutar las tools del turno.
//
// MODEL-FREE Y OPT-IN. Nada de este paquete corre si nadie levanta el servidor: sin él,
// Musubi se comporta igual que hoy. No es un camino crítico de la memoria.
package motor

import (
	"encoding/json"
	"fmt"
)

// Los nombres de los eventos del protocolo, tal como los discrimina el `switch` de
// processProxyEvent en pi-agent-core/src/proxy.ts:261. Son constantes y no un string suelto
// porque un typo acá NO falla: el cliente cae en el `default` de ese switch, hace
// console.warn y devuelve undefined — o sea el evento se pierde EN SILENCIO.
const (
	EvStart         = "start"
	EvTextStart     = "text_start"
	EvTextDelta     = "text_delta"
	EvTextEnd       = "text_end"
	EvThinkingStart = "thinking_start"
	EvThinkingDelta = "thinking_delta"
	EvThinkingEnd   = "thinking_end"
	EvToolCallStart = "toolcall_start"
	EvToolCallDelta = "toolcall_delta"
	EvToolCallEnd   = "toolcall_end"
	EvDone          = "done"
	EvError         = "error"
)

// Las razones de corte del conjunto CERRADO que el protocolo admite. En proxy.ts:36 el
// `done` está tipado `Extract<StopReason, "stop" | "length" | "toolUse">` y el `error`
// `Extract<StopReason, "aborted" | "error">`: un valor fuera de esos conjuntos no es un
// dato raro, es un tipo roto del otro lado.
const (
	ReasonStop    = "stop"
	ReasonLength  = "length"
	ReasonToolUse = "toolUse"
	ReasonAborted = "aborted"
	ReasonError   = "error"
)

// Usage es el consumo de tokens con los nombres que usa pi, NO los de la Messages API.
// La traducción es obligatoria y es un lugar fácil de errar: el stream de `claude -p`
// habla `input_tokens`/`cache_read_input_tokens`, y el cliente espera
// `input`/`cacheRead`. Un campo mal nombrado llega como cero y el contador del cliente
// miente sin avisar.
type Usage struct {
	Input       int  `json:"input"`
	Output      int  `json:"output"`
	CacheRead   int  `json:"cacheRead"`
	CacheWrite  int  `json:"cacheWrite"`
	TotalTokens int  `json:"totalTokens"`
	Cost        Cost `json:"cost"`
}

// Cost viaja en el protocolo porque el cliente lo lee para mostrarlo, pero Musubi lo
// emite en CERO a propósito: el stream de `claude -p` no trae tarifas, y el catálogo de
// pi sustituye 0 en silencio cuando no hay precio declarado (openrouter-api.ts:40-57), o
// sea que «cero» y «no sabemos» ya eran indistinguibles del otro lado. Inventar una
// tarifa acá sería peor que no declararla: el consumo real se mide por `Usage`, que sí
// es exacto.
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// ToolCall es el bloque de llamada a tool que viaja dentro de un `toolcall_end`. El
// cliente hace `Object.assign` de esto sobre el bloque que venía armando (proxy.ts:358),
// así que los nombres tienen que coincidir con los de pi-ai: `arguments`, no `input`.
type ToolCall struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`

	// ArgumentsError describe por qué los argumentos no se pudieron parsear, y es la
	// ÚNICA señal con la que el cliente evita ejecutar la tool con un objeto vacío: su
	// validación arranca con `if (toolCall.argumentsError) throw`
	// (pi-ai/src/utils/validation.ts:318-319). Vacío cuando los argumentos están bien,
	// así que lleva omitempty: un `argumentsError:""` también disparaba la guarda.
	ArgumentsError string `json:"argumentsError,omitempty"`
}

// Evento es un ProxyAssistantMessageEvent (proxy.ts:23-46) en su forma de cable. Es una
// sola struct con omitempty en vez de doce tipos porque lo que cruza la red es un objeto
// JSON plano discriminado por `type`, y mantener doce structs invita a que una se
// desincronice del switch del cliente.
//
// CUIDADO CON omitempty EN LOS ÍNDICES: `contentIndex` es 0 para el primer bloque, que es
// el caso más común, y con omitempty el campo desaparecería justo ahí, dejando al cliente
// escribiendo en `partial.content[undefined]`. El tag SÍ lo lleva —hace falta para los
// eventos donde el índice no significa nada, como el `start` y el `done`— y el caso del
// cero se resuelve en MarshalJSON, que lo reinyecta para los eventos que sí lo llevan. O
// sea: la respuesta a este peligro NO es el tag, es el método de abajo.
type Evento struct {
	Type string `json:"type"`

	// ContentIndex con omitempty + la reinyección de MarshalJSON: ver el comentario de
	// arriba. No aplica al `start`, al `done` ni al `error`, donde el cliente lo ignora.
	ContentIndex int `json:"contentIndex,omitempty"`

	Delta string `json:"delta,omitempty"`

	// ContentSignature es la firma criptográfica de un bloque de thinking. Viene en su
	// propio delta (medido: 740 caracteres en un solo `delta.signature`) y se emite
	// recién en el `thinking_end`, que es donde el protocolo la espera.
	ContentSignature string `json:"contentSignature,omitempty"`

	ID       string `json:"id,omitempty"`
	ToolName string `json:"toolName,omitempty"`

	ToolCall *ToolCall `json:"toolCall,omitempty"`

	Reason       string `json:"reason,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`

	// Usage es puntero para que un evento que no lo lleva no emita un objeto de ceros
	// que el cliente tomaría por consumo real. En `done` y `error` es obligatorio.
	Usage *Usage `json:"usage,omitempty"`

	ProviderThinkingLevel string `json:"providerThinkingLevel,omitempty"`
}

// llevaIndice marca los eventos cuyo `contentIndex` es significativo aunque valga 0, para
// que el encoder no lo omita. MarshalJSON lo reinyecta para esos, en vez de confiar en
// omitempty, que no distingue «cero» de «ausente».
func (e Evento) llevaIndice() bool {
	switch e.Type {
	case EvTextStart, EvTextDelta, EvTextEnd,
		EvThinkingStart, EvThinkingDelta, EvThinkingEnd,
		EvToolCallStart, EvToolCallDelta, EvToolCallEnd:
		return true
	}
	return false
}

// MarshalJSON emite el evento con `contentIndex` presente incluso cuando es 0. Go no
// tiene forma de pedir «omitempty salvo en este caso», y el bloque 0 es justamente el
// más frecuente: sin esto, el primer bloque de texto de CADA respuesta llegaría sin
// índice y el cliente lo escribiría en una posición indefinida.
func (e Evento) MarshalJSON() ([]byte, error) {
	type plano Evento // alias sin métodos, para no recursar
	bruto, err := json.Marshal(plano(e))
	if err != nil {
		return nil, fmt.Errorf("serializar evento %q: %w", e.Type, err)
	}
	if !e.llevaIndice() || e.ContentIndex != 0 {
		return bruto, nil
	}
	// Reinyecta el índice cero. Se hace sobre el mapa y no con un string replace para
	// no depender del orden de las claves que elija el encoder.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bruto, &m); err != nil {
		return nil, fmt.Errorf("reinyectar contentIndex en %q: %w", e.Type, err)
	}
	m["contentIndex"] = json.RawMessage("0")
	return json.Marshal(m)
}
