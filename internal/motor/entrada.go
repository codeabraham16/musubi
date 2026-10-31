package motor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────────────────────
// LA OTRA MITAD DEL PROTOCOLO, Y LA QUE CASI SE PASA POR ALTO.
//
// `traductor.go` traduce lo que SALE: los eventos de la Messages API a los doce eventos que
// el cliente espera. Este archivo traduce lo que ENTRA, y hace falta porque el transcript de
// pi NO es el de la Messages API — medido el 2026-10-28 contra pi-ai/src/types.ts:
//
//   - Tiene CUATRO roles propios: "system", "user", "assistant" y "toolResult". La Messages
//     API tiene dos, y el system prompt va en un campo aparte del request.
//   - El system prompt y las tools NO viajan en el cuerpo: viven DENTRO de los mensajes
//     "system" del transcript (`normalizeContext` los pliega ahí, transcript.ts:30). Un
//     servidor que lea `body.systemPrompt` no encuentra nada y sirve el turno sin prompt.
//   - Los nombres chocan justo donde más duele: pi dice `arguments` donde la API dice
//     `input`, `parameters` donde dice `input_schema`, `thinkingSignature` donde dice
//     `signature`, y `mimeType` donde dice `media_type`. Ninguno de esos errores falla: la
//     API rechaza el request con un 400 que habla de un campo que el cliente nunca escribió.
//
// POR QUÉ VIVE EN ESTE PAQUETE Y NO EN EL HANDLER. Es conocimiento del protocolo, igual que
// `eventos.go`, y tiene que poder probarse sin levantar un servidor HTTP. El handler queda
// siendo transporte: decodifica, llama acá, y bombea el SSE.
// ────────────────────────────────────────────────────────────────────────────────────────────

// PedidoProxy es el cuerpo que `streamProxy` POSTea, tal como sale del cliente
// (pi-agent-core/src/proxy.ts:153): `{model, context, options}`.
//
// LA RUTA NO ES NEGOCIABLE Y CONVIENE SABERLO ANTES DE DISEÑAR: el cliente hace
// `fetch(`${proxyUrl}/api/stream`)` con la ruta HARDCODEADA (proxy.ts:147). Sólo `proxyUrl`
// es configurable. O sea que el servidor no puede elegir otro camino, y en Musubi `/api/stream`
// ya estaba tomada por el feed de eventos en vivo — de ahí que la puerta del turno se reparta
// por MÉTODO (GET = feed, POST = turno) en vez de por ruta.
type PedidoProxy struct {
	Model   ModeloProxy   `json:"model"`
	Context ContextoProxy `json:"context"`
	Options OpcionesProxy `json:"options"`
}

// ModeloProxy es el `Model` de pi (types.ts:833 y :847) recortado a lo que un turno necesita.
// El cliente manda el objeto ENTERO —tarifas, ventana de contexto, modalidades, cabeceras—
// y leer sólo tres campos es deliberado: cada campo que este servidor interprete es un campo
// que lo ata a una versión del catálogo del cliente.
type ModeloProxy struct {
	ID string `json:"id"`

	// MaxTokens es el tope del MODELO, no del pedido. Es el piso que se usa cuando las
	// opciones no traen uno propio.
	MaxTokens int `json:"maxTokens"`
}

// ContextoProxy es el `TranscriptContext` (types.ts:793). Es un solo campo: el brand de
// TypeScript que lo acompaña es un símbolo fantasma y no serializa.
//
// Los mensajes quedan CRUDOS a propósito: el rol decide cómo se parsea cada uno, y
// adivinar una struct única para los cuatro roles obliga a inventar campos opcionales
// donde el protocolo tiene tipos distintos.
type ContextoProxy struct {
	Messages []json.RawMessage `json:"messages"`
}

// OpcionesProxy son las `ProxySerializableStreamOptions` que el cliente serializa
// (proxy.ts:91). Se leen las que este servidor puede HONRAR; las demás —temperature,
// samplingParams, cacheRetention, transport, metadata— se ignoran en silencio y es correcto:
// ninguno de los dos motores las puede aplicar, y aceptarlas sería prometer algo que no pasa.
type OpcionesProxy struct {
	// MaxTokens del PEDIDO. Le gana al del modelo cuando viene.
	MaxTokens int `json:"maxTokens"`

	// Reasoning es el nivel de thinking que pidió el cliente ("minimal" | "low" | "medium"
	// | "high" | "xhigh" | "max"). Vacío = thinking apagado.
	Reasoning string `json:"reasoning"`

	// ThinkingBudgets son los presupuestos por nivel. Puntero para distinguir «el cliente
	// mandó presupuestos propios» de «no mandó nada», que es lo que decide si se usan los
	// de pi: un struct de ceros se leería como «presupuesto 0» y apagaría el thinking que
	// el cliente acaba de pedir.
	ThinkingBudgets *PresupuestosThinking `json:"thinkingBudgets"`
}

// PresupuestosThinking es el `ThinkingBudgets` de pi (types.ts:139).
type PresupuestosThinking struct {
	Minimal int `json:"minimal"`
	Low     int `json:"low"`
	Medium  int `json:"medium"`
	High    int `json:"high"`
}

// presupuestosPorDefecto son los de pi (simple-options.ts:71). Se copian en vez de inventar
// otros por la misma razón que MaxTokensPorDefecto copia el del CLI: un cliente que no manda
// presupuestos espera los de su propia librería, y que el servidor elija otros le cambia la
// respuesta sin avisarle.
var presupuestosPorDefecto = PresupuestosThinking{Minimal: 1024, Low: 2048, Medium: 8192, High: 16384}

// Los roles del transcript de pi (types.ts:652). "toolResult" no tiene equivalente en la
// Messages API: se convierte en un mensaje de USUARIO con bloques `tool_result`.
const (
	rolSystem     = "system"
	rolUsuario    = "user"
	rolAsistente  = "assistant"
	rolToolResult = "toolResult"
)

// PedidoDesdeProxy traduce el cuerpo de `streamProxy` al Pedido de este paquete.
//
// Lo que hace, en orden: reproduce el estado de los mensajes "system" para sacar el prompt y
// el set de tools vigentes, traduce cada mensaje restante a bloques de la Messages API, y
// fusiona los turnos consecutivos del mismo rol.
func PedidoDesdeProxy(p PedidoProxy) (Pedido, error) {
	modelo := strings.TrimSpace(p.Model.ID)
	if modelo == "" {
		return Pedido{}, errors.New("el pedido no trae model.id: sin modelo no hay turno que servir")
	}
	if len(p.Context.Messages) == 0 {
		return Pedido{}, errors.New("el pedido no trae context.messages")
	}

	prompt, tools, mensajes, err := reproducirTranscript(p.Context.Messages)
	if err != nil {
		return Pedido{}, err
	}
	if len(mensajes) == 0 {
		// Un transcript que SÓLO tiene mensajes "system" no es un turno: no hay nada que
		// responder. Los dos motores lo rechazarían más adelante con un error peor
		// (`pedido sin mensajes`, ya del lado del proveedor), así que se corta acá.
		return Pedido{}, errors.New("el transcript no tiene ningún turno de usuario ni de asistente")
	}

	tope := p.Options.MaxTokens
	if tope <= 0 {
		tope = p.Model.MaxTokens
	}

	return Pedido{
		Modelo: modelo,
		// EL MISMO ID EN LOS DOS CAMPOS, Y NO ES REDUNDANTE. `ModeloResuelto` existe porque
		// el Lanzador tiene que escribir un id COMPLETO en los turnos del asistente o el CLI
		// descarta los bloques de thinking firmados en silencio (ver su comentario en
		// motor.go). Los modelos de pi ya traen el id concreto en `model.id` —no un alias—,
		// así que declararlo acá es lo que hace que el thinking sobreviva al viaje de ida.
		ModeloResuelto: modelo,
		SystemPrompt:   prompt,
		Mensajes:       mensajes,
		MaxTokens:      tope,
		Tools:          tools,
		Thinking:       presupuestoDeThinking(p.Options),
	}, nil
}

// presupuestoDeThinking resuelve el budget en tokens a partir del nivel pedido.
//
// EL CLAMP DE xhigh/max A high ES DE pi, NO UNA DECISIÓN DE ACÁ (`clampReasoning`,
// simple-options.ts:78): su tabla de presupuestos sólo tiene cuatro niveles, y los dos de
// arriba no tienen uno propio. Sin el clamp, un cliente que pide "max" cae en el cero del
// struct y se queda SIN thinking justo cuando pidió el máximo.
func presupuestoDeThinking(o OpcionesProxy) int {
	nivel := strings.TrimSpace(strings.ToLower(o.Reasoning))
	if nivel == "" {
		return 0
	}
	if nivel == "xhigh" || nivel == "max" {
		nivel = "high"
	}
	b := presupuestosPorDefecto
	if o.ThinkingBudgets != nil {
		b = *o.ThinkingBudgets
	}
	switch nivel {
	case "minimal":
		return b.Minimal
	case "low":
		return b.Low
	case "medium":
		return b.Medium
	case "high":
		return b.High
	}
	// Un nivel que no está en la tabla NO es un error del pedido: es un nivel que esta
	// versión no conoce. Apagar el thinking es la caída segura — pedirle a la API un budget
	// inventado sí tiraría el turno entero.
	return 0
}

// reproducirTranscript recorre los mensajes una sola vez y devuelve el prompt vigente, las
// tools vigentes y los turnos traducidos.
//
// UNA SOLA PASADA, Y NO ES POR VELOCIDAD. El orden importa en los dos ejes a la vez: las
// tools de un mensaje "system" valen desde ahí en adelante, y los turnos que vienen después
// ya las pueden usar. Separarlo en dos recorridos invita a que uno de los dos se desincronice.
func reproducirTranscript(crudos []json.RawMessage) (prompt string, tools json.RawMessage, mensajes []Mensaje, err error) {
	// El prompt se arma con las MISMAS reglas que `getCurrentSystemMessage` +
	// `getSystemMessageText` (transcript.ts:73 y text.ts:15): los `content` de cada mensaje
	// "system" se juntan con "\n\n", y las secciones se agregan después, también con "\n\n".
	var textos []string
	secciones := nuevoMapaOrdenado[string]()
	declaradas := nuevoMapaOrdenado[toolProxy]()

	for i, crudo := range crudos {
		var cab struct {
			Role string `json:"role"`
		}
		if e := json.Unmarshal(crudo, &cab); e != nil {
			return "", nil, nil, fmt.Errorf("mensaje %d: no es un objeto JSON: %w", i, e)
		}

		switch cab.Role {
		case rolSystem:
			var m systemProxy
			if e := json.Unmarshal(crudo, &m); e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (system): %w", i, e)
			}
			// `contentText` junta los bloques de texto de UN mensaje con "\n" —no con
			// "\n\n", que es el separador ENTRE mensajes (text.ts:6). Son dos separadores
			// distintos en el mismo prompt y confundirlos cambia el texto que ve el modelo.
			if t := m.Content.texto("\n"); t != "" {
				textos = append(textos, t)
			}
			for _, s := range m.Sections {
				if s.Borrar {
					secciones.borrar(s.Nombre)
					continue
				}
				secciones.poner(s.Nombre, s.Texto)
			}
			// LAS QUITADAS VAN ANTES QUE LAS AGREGADAS, y el orden es el de pi
			// (`getCurrentTools`, transcript.ts:62-63). Al revés, un mensaje que redefine
			// una tool —quitarla y volver a agregarla en el mismo paso— la perdería.
			for _, t := range m.ToolsRemoved {
				declaradas.borrar(t.Name)
			}
			for _, t := range m.ToolsAdded {
				declaradas.poner(t.Name, t)
			}

		case rolUsuario:
			var m struct {
				Content contenidoProxy `json:"content"`
			}
			if e := json.Unmarshal(crudo, &m); e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (user): %w", i, e)
			}
			bloques, e := bloquesDeEntrada(m.Content)
			if e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (user): %w", i, e)
			}
			mensajes = agregarMensaje(mensajes, rolUsuario, bloques)

		case rolAsistente:
			var m struct {
				Content []json.RawMessage `json:"content"`
			}
			if e := json.Unmarshal(crudo, &m); e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (assistant): %w", i, e)
			}
			bloques, e := bloquesDeAsistente(m.Content)
			if e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (assistant): %w", i, e)
			}
			mensajes = agregarMensaje(mensajes, rolAsistente, bloques)

		case rolToolResult:
			var m toolResultProxy
			if e := json.Unmarshal(crudo, &m); e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (toolResult): %w", i, e)
			}
			bloque, e := bloqueDeToolResult(m)
			if e != nil {
				return "", nil, nil, fmt.Errorf("mensaje %d (toolResult): %w", i, e)
			}
			// EL RESULTADO DE UNA TOOL ES UN MENSAJE DE USUARIO. No hay rol propio en la
			// Messages API, y `agregarMensaje` lo fusiona con el user de al lado — que es
			// justo lo que la API EXIGE: los N resultados de un turno con N tool_use van en
			// UN solo mensaje. Mandarlos en mensajes separados es un 400.
			mensajes = agregarMensaje(mensajes, rolUsuario, []json.RawMessage{bloque})

		default:
			// NO SE SALTEA, Y ES A PROPÓSITO. pi admite roles propios en los transcripts de
			// agente (`TranscriptMessages`, transcript.ts:40), pero lo que llega acá es un
			// `TranscriptContext` ya normalizado, donde los roles son esos cuatro. Un rol
			// desconocido significa que el contrato cambió, y descartar un turno en silencio
			// le cambia el contexto al modelo sin que nadie pueda notarlo: es exactamente la
			// clase de falla que este paquete viene evitando en el otro sentido.
			return "", nil, nil, fmt.Errorf("mensaje %d: rol desconocido %q", i, cab.Role)
		}
	}

	// LAS SECCIONES VAN DESPUÉS DEL CONTENIDO, no intercaladas: es el orden de
	// `getSystemMessageText` (text.ts:17), que primero renderiza el `content` y después
	// recorre `sections`. Y las vacías se filtran recién acá, al final, porque una sección
	// puede quedar en "" y la línea en blanco que dejaría se vería en el prompt.
	var limpias []string
	for _, p := range append(textos, secciones.valores()...) {
		if p != "" {
			limpias = append(limpias, p)
		}
	}
	prompt = strings.Join(limpias, "\n\n")

	if decl := declaradas.valores(); len(decl) > 0 {
		tools, err = toolsParaLaAPI(decl)
		if err != nil {
			return "", nil, nil, err
		}
	}
	// Un transcript SIN ningún mensaje "system" es válido: prompt vacío y sin tools. Es el
	// caso del turno pelado, y no hay nada que avisar.
	return prompt, tools, mensajes, nil
}

// agregarMensaje agrega los bloques al último turno si es del mismo rol, o abre uno nuevo.
//
// LA FUSIÓN NO ES UNA OPTIMIZACIÓN: la Messages API quiere los resultados de todas las tools
// de un turno en UN mensaje de usuario, y un transcript de pi los trae como un
// `toolResult` por tool. Sin esto, un turno con dos tools se rechaza con un 400.
func agregarMensaje(ms []Mensaje, rol string, bloques []json.RawMessage) []Mensaje {
	if len(bloques) == 0 {
		// Un turno sin bloques no se puede mandar (la API pide `content` no vacío) y
		// tampoco aporta nada. Se descarta en vez de emitir un mensaje inválido.
		return ms
	}
	if n := len(ms); n > 0 && ms[n-1].Rol == rol {
		ms[n-1].Contenido = append(ms[n-1].Contenido, bloques...)
		return ms
	}
	return append(ms, Mensaje{Rol: rol, Contenido: bloques})
}

// ───────────────────────────── los mensajes, uno por uno ─────────────────────────────

type systemProxy struct {
	Content      contenidoProxy `json:"content"`
	Sections     secciones      `json:"sections"`
	ToolsAdded   []toolProxy    `json:"toolsAdded"`
	ToolsRemoved []refTool      `json:"toolsRemoved"`
}

type refTool struct {
	Name string `json:"name"`
}

type toolProxy struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type toolResultProxy struct {
	ToolCallID string         `json:"toolCallId"`
	ToolName   string         `json:"toolName"`
	Content    contenidoProxy `json:"content"`
	IsError    bool           `json:"isError"`
}

// contenidoProxy es un `content` de pi, que en el MISMO campo puede ser un string o un
// arreglo de bloques (types.ts:557 y :574). No es una rareza del formato: el camino corto
// —un mensaje de una línea— manda el string pelado.
type contenidoProxy struct {
	texto1  string
	esTexto bool
	bloques []json.RawMessage
}

func (c *contenidoProxy) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		if err := json.Unmarshal(b, &c.texto1); err != nil {
			return err
		}
		c.esTexto = true
		return nil
	}
	return json.Unmarshal(b, &c.bloques)
}

// texto extrae y junta SÓLO los bloques de texto, igual que `contentText` (text.ts:6). Un
// bloque de imagen no aporta texto y se saltea; es lo que hace que un system prompt con una
// imagen pegada no quede con un hueco raro en el medio.
func (c contenidoProxy) texto(sep string) string {
	if c.esTexto {
		return c.texto1
	}
	var ts []string
	for _, b := range c.bloques {
		var bl struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(b, &bl) == nil && bl.Type == "text" {
			ts = append(ts, bl.Text)
		}
	}
	return strings.Join(ts, sep)
}

// bloquesDeEntrada traduce el contenido de un mensaje de usuario (o de un tool_result) a
// bloques de la Messages API. Acepta texto e imágenes, que es lo que pi admite ahí.
func bloquesDeEntrada(c contenidoProxy) ([]json.RawMessage, error) {
	if c.esTexto {
		if c.texto1 == "" {
			return nil, nil
		}
		return []json.RawMessage{bloqueDeTexto(c.texto1)}, nil
	}
	var out []json.RawMessage
	for _, b := range c.bloques {
		var cab struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(b, &cab); err != nil {
			return nil, fmt.Errorf("bloque ilegible: %w", err)
		}
		switch cab.Type {
		case "text":
			var bl struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(b, &bl); err != nil {
				return nil, fmt.Errorf("bloque de texto: %w", err)
			}
			// SE REESCRIBE EN VEZ DE PASARLO TAL CUAL, y eso vale para todos los bloques de
			// este archivo. El `TextContent` de pi puede traer `textSignature`, el `ToolCall`
			// trae `thoughtSignature` y `namespace`: campos que la Messages API no conoce y
			// que la hacen rechazar el request entero. Un passthrough «eficiente» acá es un
			// 400 que habla de un campo que el cliente nunca escribió.
			out = append(out, bloqueDeTexto(bl.Text))
		case "image":
			var bl struct {
				Data     string `json:"data"`
				MimeType string `json:"mimeType"`
			}
			if err := json.Unmarshal(b, &bl); err != nil {
				return nil, fmt.Errorf("bloque de imagen: %w", err)
			}
			img, err := json.Marshal(map[string]any{
				"type": "image",
				// `media_type`, no `mimeType`: la API lo nombra distinto y un campo mal
				// nombrado acá no se ignora, hace fallar el request.
				"source": map[string]string{"type": "base64", "media_type": bl.MimeType, "data": bl.Data},
			})
			if err != nil {
				return nil, fmt.Errorf("serializar la imagen: %w", err)
			}
			out = append(out, img)
		default:
			return nil, fmt.Errorf("bloque de entrada de tipo %q", cab.Type)
		}
	}
	return out, nil
}

// bloquesDeAsistente traduce el contenido de un turno del asistente: texto, thinking y
// llamadas a tools. Es el camino por donde viaja la historia que hace avanzar un agent loop.
func bloquesDeAsistente(crudos []json.RawMessage) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for _, b := range crudos {
		var cab struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(b, &cab); err != nil {
			return nil, fmt.Errorf("bloque ilegible: %w", err)
		}
		switch cab.Type {
		case "text":
			var bl struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(b, &bl); err != nil {
				return nil, fmt.Errorf("bloque de texto: %w", err)
			}
			out = append(out, bloqueDeTexto(bl.Text))

		case "thinking":
			var bl struct {
				Thinking  string `json:"thinking"`
				Signature string `json:"thinkingSignature"`
				Redacted  bool   `json:"redacted"`
			}
			if err := json.Unmarshal(b, &bl); err != nil {
				return nil, fmt.Errorf("bloque de thinking: %w", err)
			}
			// UN THINKING REDACTADO ES OTRO TIPO DE BLOQUE, no el mismo con una bandera. La
			// API lo devuelve como `redacted_thinking` con el payload cifrado en `data`, y
			// reenviarlo así es lo que deja seguir una conversación que pasó por el filtro
			// de seguridad. Mandarlo como `thinking` con texto vacío lo rechaza.
			if bl.Redacted {
				rd, err := json.Marshal(map[string]string{"type": "redacted_thinking", "data": bl.Signature})
				if err != nil {
					return nil, fmt.Errorf("serializar el thinking redactado: %w", err)
				}
				out = append(out, rd)
				continue
			}
			// `signature`, no `thinkingSignature`. Es la firma criptográfica del bloque: sin
			// ella la API rechaza el turno en una conversación con thinking, y con el nombre
			// de pi viaja como un campo desconocido, que es el mismo rechazo por otra puerta.
			th, err := json.Marshal(map[string]string{"type": "thinking", "thinking": bl.Thinking, "signature": bl.Signature})
			if err != nil {
				return nil, fmt.Errorf("serializar el thinking: %w", err)
			}
			out = append(out, th)

		case "toolCall":
			var bl struct {
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(b, &bl); err != nil {
				return nil, fmt.Errorf("bloque de tool: %w", err)
			}
			args := bl.Arguments
			if len(bytes.TrimSpace(args)) == 0 || bytes.Equal(bytes.TrimSpace(args), []byte("null")) {
				// `input` es obligatorio y tiene que ser un objeto. Un `arguments` ausente
				// pasa cuando el modelo mandó argumentos que no parseaban: pi deja `{}` y
				// anota el motivo en `argumentsError`. El objeto vacío es la réplica fiel de
				// ese turno; inventar argumentos sería peor.
				args = json.RawMessage(`{}`)
			}
			tu, err := json.Marshal(map[string]any{
				"type": "tool_use",
				"id":   bl.ID,
				"name": bl.Name,
				// `input`, no `arguments`. El mismo renombre que el protocolo de salida hace
				// al revés en `ToolCall.Arguments` (eventos.go), y por el mismo motivo: son
				// dos vocabularios para el mismo dato.
				"input": args,
			})
			if err != nil {
				return nil, fmt.Errorf("serializar la llamada a tool: %w", err)
			}
			out = append(out, tu)

		default:
			return nil, fmt.Errorf("bloque de asistente de tipo %q", cab.Type)
		}
	}
	return out, nil
}

// bloqueDeToolResult arma el `tool_result` que cierra una llamada a tool.
func bloqueDeToolResult(m toolResultProxy) (json.RawMessage, error) {
	contenido, err := bloquesDeEntrada(m.Content)
	if err != nil {
		return nil, err
	}
	if len(contenido) == 0 {
		// Una tool que no devolvió nada igual tiene que cerrar su `tool_use`, o el turno
		// queda con una llamada sin respuesta y la API lo rechaza. Se manda un texto vacío
		// explícito en vez de un `content` vacío, que también es inválido.
		contenido = []json.RawMessage{bloqueDeTexto("")}
	}
	campos := map[string]any{
		"type":        "tool_result",
		"tool_use_id": m.ToolCallID,
		"content":     contenido,
	}
	if m.IsError {
		// `is_error` sólo cuando es true: la API acepta el campo ausente, y mandar
		// `is_error:false` en cada resultado es ruido en cada turno de un agent loop.
		campos["is_error"] = true
	}
	b, err := json.Marshal(campos)
	if err != nil {
		return nil, fmt.Errorf("serializar el resultado de %q: %w", m.ToolName, err)
	}
	return b, nil
}

func bloqueDeTexto(t string) json.RawMessage {
	// No puede fallar: dos strings. El error se descarta acá para que los llamadores no
	// tengan que propagar un error imposible en cada bloque de texto.
	b, _ := json.Marshal(map[string]string{"type": "text", "text": t})
	return b
}

// toolsParaLaAPI traduce las declaraciones de tools de pi al formato del request.
func toolsParaLaAPI(ts []toolProxy) (json.RawMessage, error) {
	type toolAPI struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	out := make([]toolAPI, 0, len(ts))
	for _, t := range ts {
		esquema := t.Parameters
		if len(bytes.TrimSpace(esquema)) == 0 || bytes.Equal(bytes.TrimSpace(esquema), []byte("null")) {
			// `input_schema` es OBLIGATORIO. Una tool sin parámetros es legítima —pi la
			// declara con un schema vacío— pero omitir el campo rechaza el request entero,
			// o sea que una sola tool sin parámetros tiraría el turno completo.
			esquema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, toolAPI{
			Name:        t.Name,
			Description: t.Description,
			// `input_schema`, no `parameters`. Segundo renombre de la ida; los cuatro, en :23-25.
			InputSchema: esquema,
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("serializar las tools: %w", err)
	}
	return b, nil
}

// ───────────────────────────── el orden, que acá es semántica ─────────────────────────────

// secciones es el `sections` de un mensaje "system": un objeto JSON cuyo ORDEN DE CLAVES
// importa, porque las secciones se renderizan en ese orden después del `content`.
//
// POR QUÉ NO ES UN map[string]*string. Un mapa de Go no tiene orden, y recorrerlo da un
// prompt distinto en cada corrida — una falla que no se ve en un test de una clave y se ve
// como un modelo que «responde distinto a veces». El `Record` de pi es un objeto JSON y
// mantiene el orden del texto; esto lo preserva leyendo los tokens a mano.
type secciones []seccion

type seccion struct {
	Nombre string
	Texto  string
	// Borrar marca el `null` del protocolo, que QUITA la sección (types.ts:564). Es un
	// tercer estado: ausente, con texto, y presente-en-null. Un string vacío no alcanza —
	// significa «sección vacía», que se renderiza (y se filtra después), no «sin sección».
	Borrar bool
}

func (s *secciones) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return errors.New("sections no es un objeto")
	}
	for dec.More() {
		clave, err := dec.Token()
		if err != nil {
			return err
		}
		nombre, ok := clave.(string)
		if !ok {
			return errors.New("sections tiene una clave que no es un string")
		}
		var valor *string
		if err := dec.Decode(&valor); err != nil {
			return fmt.Errorf("sections[%q]: %w", nombre, err)
		}
		if valor == nil {
			*s = append(*s, seccion{Nombre: nombre, Borrar: true})
			continue
		}
		*s = append(*s, seccion{Nombre: nombre, Texto: *valor})
	}
	return nil
}

// mapaOrdenado reproduce el `Map` de JavaScript, y la diferencia con un mapa de Go cambia el
// resultado: re-asignar una clave que YA EXISTE conserva su posición, mientras que borrarla y
// volver a ponerla la manda al final. pi arma el prompt y el set de tools con `Map`
// (transcript.ts:75 y :59), así que un `append` ciego pondría una sección redefinida dos
// veces, y un mapa de Go las devolvería en cualquier orden.
type mapaOrdenado[T any] struct {
	orden    []string
	porClave map[string]T
}

func nuevoMapaOrdenado[T any]() *mapaOrdenado[T] {
	return &mapaOrdenado[T]{porClave: make(map[string]T)}
}

func (m *mapaOrdenado[T]) poner(k string, v T) {
	if _, existe := m.porClave[k]; !existe {
		m.orden = append(m.orden, k)
	}
	m.porClave[k] = v
}

func (m *mapaOrdenado[T]) borrar(k string) {
	if _, existe := m.porClave[k]; !existe {
		return
	}
	delete(m.porClave, k)
	for i, o := range m.orden {
		if o == k {
			m.orden = append(m.orden[:i], m.orden[i+1:]...)
			break
		}
	}
}

func (m *mapaOrdenado[T]) valores() []T {
	out := make([]T, 0, len(m.orden))
	for _, k := range m.orden {
		out = append(out, m.porClave[k])
	}
	return out
}
