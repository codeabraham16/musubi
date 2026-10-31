package motor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ClienteAPI es el otro motor: le habla a `/v1/messages` directo con `stream:true`.
//
// QUÉ GANA Y QUÉ CUESTA, comparado con el Lanzador. Gana el CONTRATO EXACTO: el cliente
// remoto controla su system prompt entero y su budget de thinking, y nada de lo que va en
// el request lo pone este proceso. Eso es lo que el Lanzador NO puede dar — el CLI agrega
// su `# Environment` con el cwd y la plataforma del host y dos bloques fijos de system, y
// la única bandera que lo quita (`--bare`) exige una API key, con lo cual deja de ser
// gratis—. Y gana no depender del formato INTERNO de una versión del CLI, que es lo más
// frágil del otro camino.
//
// CON LAS TOOLS LA VENTAJA SE ACHICÓ, y este comentario la exageraba: listaba «sus 24
// tools» como parte de lo que el Lanzador le mete encima, y desde que existe el espejo los
// DOS motores le dan al modelo el set exacto del pedido (`--tools ""` deja las nativas en
// cero). Lo que queda del lado del Lanzador es que las del cliente viajan con el prefijo
// `mcp__pi__` y hay que deshacerlo a la vuelta en los dos eventos que llevan nombre de
// tool. Ver el campo Tools de Pedido, donde está la medición.
//
// Cuesta consumo por token: acá no hay suscripción, se factura el request.
type ClienteAPI struct {
	// Clave es la credencial. Vacía = la de ClaveEnv (o ANTHROPIC_API_KEY).
	//
	// NO SE LOGUEA EN NINGÚN LUGAR DE ESTE PAQUETE, ni en los errores: un error de la API
	// se reporta con su cuerpo, que no la trae. Un log que la imprima la deja en el journal
	// del server para siempre.
	Clave string

	// ClaveEnv es la variable de donde sacar la credencial si Clave está vacía. Vacío =
	// ANTHROPIC_API_KEY. Se expone para que una instalación que ya tenga una pasarela
	// (LiteLLM y compañía) use la credencial que ya tiene, como hace `musubi calibrate`.
	ClaveEnv string

	// Endpoint es la URL de /v1/messages. Vacío = la de Anthropic. Se parametriza para
	// poder apuntar a una pasarela compatible o a un servidor de prueba.
	Endpoint string

	// HTTP es el cliente a usar. Nil = uno con el timeout en CERO a propósito: ver
	// clienteHTTP.
	HTTP *http.Client

	// Version es el valor de `anthropic-version`. Vacío = VersionAPIPorDefecto.
	Version string

	// Betas son los valores de `anthropic-beta`. Hacen falta para features que todavía no
	// son GA.
	Betas []string
}

// El ClienteAPI es el otro motor. Igual que con el Lanzador, esta línea hace que una firma
// que se desvíe falle acá y no en el handler.
var _ Motor = (*ClienteAPI)(nil)

const (
	// EndpointPorDefecto es /v1/messages de Anthropic.
	EndpointPorDefecto = "https://api.anthropic.com/v1/messages"

	// VersionAPIPorDefecto es el valor de `anthropic-version`. La API lo EXIGE: sin esa
	// cabecera el request se rechaza.
	VersionAPIPorDefecto = "2023-06-01"

	// MaxTokensPorDefecto es el tope de la respuesta cuando el pedido no trae uno. El
	// campo es OBLIGATORIO en el request, así que no hay forma de omitirlo.
	//
	// El valor es el que el CLI fija por su cuenta (medido: 64000), y la razón de copiarlo
	// es la paridad: un cliente que cambie de motor por config no tiene que ver que las
	// respuestas se le acortan.
	MaxTokensPorDefecto = 64000
)

// Turno sirve un turno completo contra la Messages API: arma el request, consume el SSE y
// traduce cada evento con la MISMA puerta del traductor que usa el Lanzador.
//
// POR QUÉ ESTO ES CORTO. Lo que el CLI reenvía en sus `stream_event` son eventos de la
// Messages API TEXTUALES, sin una sola clave propia (medido el 2026-10-28). Así que el
// traductor ya sabía hablar este protocolo: lo único que cambia es la puerta de entrada —
// `EventoAPI` en vez de `Linea`, que es la misma función sin desenvolver el sobre— y de
// dónde salen los bytes. Esa medición es la que hace que el segundo motor cueste un archivo
// y no un paquete.
func (c *ClienteAPI) Turno(ctx context.Context, p Pedido, emitir func(Evento) error) error {
	if len(p.Mensajes) == 0 {
		return errors.New("pedido sin mensajes")
	}
	clave := c.clave()
	if clave == "" {
		return fmt.Errorf("falta la credencial: ni Clave ni la variable %s", c.nombreDeLaVariable())
	}
	cuerpo, err := cuerpoDelRequest(p)
	if err != nil {
		return err
	}

	destino := c.Endpoint
	if destino == "" {
		destino = EndpointPorDefecto
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(cuerpo))
	if err != nil {
		return fmt.Errorf("armar el request: %w", err)
	}
	version := c.Version
	if version == "" {
		version = VersionAPIPorDefecto
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("x-api-key", clave)
	req.Header.Set("anthropic-version", version)
	if len(c.Betas) > 0 {
		req.Header.Set("anthropic-beta", strings.Join(c.Betas, ","))
	}

	resp, err := c.clienteHTTP().Do(req)
	if err != nil {
		// El ctx cancelado llega como un error del transporte, y distinguirlo importa por
		// lo mismo que en el Lanzador: en un servidor es el caso más frecuente y no es una
		// falla.
		if errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("el llamador canceló el turno: %w", err)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("el turno se cortó por tiempo: %w", err)
		}
		return fmt.Errorf("pedirle el turno a %s: %w", destino, err)
	}
	// SE CIERRA SIEMPRE, y es lo que CORTA el turno. El traductor para en el primer
	// `message_delta` y volvemos de este loop con el cuerpo a medio leer: cerrarlo le dice
	// al otro extremo que no mande más. Sin esto, cada turno deja una conexión colgada
	// esperando bytes que nadie lee.
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorDeHTTP(resp)
	}

	tr := NuevoTraductor()
	cerrado, errBombeo := bombearSSE(resp.Body, tr, emitir)

	switch {
	case errors.Is(errBombeo, ErrDelConsumidor):
		// Igual que en el Lanzador, y por lo mismo: el traductor marca el turno como
		// cerrado ANTES de que su último evento se emita, así que un `emitir` que falle en
		// el `done` deja cerrado=true Y un error. Si `cerrado` ganara, el llamador creería
		// que sirvió un turno que el cliente no recibió.
		return errBombeo
	case cerrado:
		return nil
	case errors.Is(ctx.Err(), context.Canceled):
		return errors.New("el llamador canceló el turno")
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return errors.New("el turno se cortó por tiempo")
	case errBombeo != nil:
		return errBombeo
	default:
		// El stream terminó sin un `message_delta`. Es el equivalente del «proceso mudo»
		// del Lanzador: no hay respuesta y tampoco hay a quién echarle la culpa.
		return errors.New("el stream terminó sin cerrar el turno")
	}
}

// bombearSSE consume el cuerpo `text/event-stream` y traduce cada `data:` por la puerta de
// la API. Devuelve si el turno cerró y el primer error que impida seguir.
//
// LAS LÍNEAS `event:` SE IGNORAN A PROPÓSITO. El SSE manda el tipo dos veces —en la línea
// `event:` y dentro del JSON del `data:`— y la segunda es la única que importa: el traductor
// discrimina por el `type` del cuerpo. Leer la del sobre sería tener dos fuentes de verdad
// para el mismo dato, y el día que difieran no habría forma de saber cuál vale.
func bombearSSE(r io.Reader, tr *Traductor, emitir func(Evento) error) (cerrado bool, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), topeDeLinea)

	// Un evento SSE puede traer VARIAS líneas `data:`, que por spec se concatenan con \n.
	// En la práctica la API manda una sola, pero el acumulador cuesta tres líneas y evita
	// que un día un JSON partido en dos llegue como dos JSON inválidos.
	var datos []string

	despachar := func() (bool, error) {
		if len(datos) == 0 {
			return false, nil
		}
		crudo := strings.Join(datos, "\n")
		datos = datos[:0]
		// La Messages API no lo manda (cierra con `message_stop`), pero una pasarela compatible
		// al estilo OpenAI sí.
		//
		// LO QUE AHORRA ES UNA LÍNEA DE LOG, no un turno, y esa es toda su ambición: sin la
		// guarda, el `[DONE]` baja al traductor, no parsea, y sale un `motor: …` por stderr que
		// se lee como un bug cuando no lo es. El turno sigue igual —el evento ilegible se
		// saltea por el invariante I13—, así que SACARLA DEJA EL TEST EN VERDE y el verde es
		// correcto. Anotado en la spec como sabotaje que espera verde.
		if crudo == "[DONE]" {
			return false, nil
		}
		evs, errEv := tr.EventoAPI([]byte(crudo))
		if errEv != nil {
			// Igual que en el Lanzador: se registra y se sigue. Un evento nuevo que este
			// traductor no conozca no puede tirar un turno que ya estaba casi completo.
			fmt.Fprintf(os.Stderr, "motor: %v\n", errEv)
		}
		if err := emitirEventos(evs, emitir); err != nil {
			return tr.Cerrado(), err
		}
		return tr.Cerrado(), nil
	}

	for sc.Scan() {
		linea := strings.TrimRight(sc.Text(), "\r")

		// La línea vacía cierra el evento: es ahí donde se despacha lo acumulado.
		if linea == "" {
			cerrado, err := despachar()
			if err != nil {
				return cerrado, err
			}
			if cerrado {
				return true, nil
			}
			continue
		}
		// Comentario SSE: es como viajan los latidos de cualquier proxy.
		//
		// ES REDUNDANTE EN EFECTO, y conviene saberlo antes de sacarla: una línea que arranca
		// con `:` tampoco pasa el `cortarCampo(linea, "data")` de abajo, así que el latido se
		// descarta igual. Queda por dos razones —declara la intención, y protege el día que
		// acá se lea un segundo campo— y con una consecuencia medida: SACARLA DEJA EL TEST EN
		// VERDE, y el verde es correcto. Está anotado en la spec como sabotaje que espera
		// verde, porque si no el próximo que lo corra va a concluir que el test no mide nada.
		if strings.HasPrefix(linea, ":") {
			continue
		}
		if valor, ok := cortarCampo(linea, "data"); ok {
			datos = append(datos, valor)
		}
		// Cualquier otro campo (`event:`, `id:`, `retry:`) se descarta: ver el comentario
		// de la función.
	}
	if err := sc.Err(); err != nil {
		return tr.Cerrado(), fmt.Errorf("leer el stream: %w", err)
	}
	// Un stream que termina sin la línea vacía final deja el último evento sin despachar.
	// La API siempre la manda, pero perder el `message_delta` por un \n faltante sería un
	// turno entero tirado.
	cerrado, err = despachar()
	if err != nil {
		return cerrado, err
	}
	return tr.Cerrado(), nil
}

// cortarCampo parte una línea SSE en campo y valor. Por spec el espacio que sigue a los dos
// puntos es parte del separador y se come UNO solo: comer todos rompería un valor que
// arranque con espacios.
func cortarCampo(linea, campo string) (string, bool) {
	if !strings.HasPrefix(linea, campo) {
		return "", false
	}
	resto := linea[len(campo):]
	if !strings.HasPrefix(resto, ":") {
		return "", false
	}
	return strings.TrimPrefix(resto[1:], " "), true
}

// emitirEventos manda los eventos al consumidor marcando sus errores con ErrDelConsumidor.
// Lo comparten los dos motores porque el orden del switch de cada Turno depende de ese
// sentinel: sin él, un error del consumidor y un error de lectura son el mismo `error`
// pelado y no hay forma de darles prioridades distintas.
func emitirEventos(evs []Evento, emitir func(Evento) error) error {
	for _, e := range evs {
		if err := emitir(e); err != nil {
			return fmt.Errorf("%w: emitir %s: %w", ErrDelConsumidor, e.Type, err)
		}
	}
	return nil
}

// errorDeHTTP arma el error de una respuesta que no es 200, con el cuerpo que la API manda.
// El cuerpo es lo ÚNICO que dice por qué —`credit balance is too low`, `max_tokens`
// inválido, un modelo que no existe— y tirarlo deja un «status 400» que no se puede
// diagnosticar.
func errorDeHTTP(resp *http.Response) error {
	cuerpo, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	texto := strings.TrimSpace(string(cuerpo))

	// La API contesta {"type":"error","error":{"type":…,"message":…}}. Si se puede leer, el
	// mensaje solo es mucho más claro que el JSON entero.
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(cuerpo, &env) == nil && env.Error.Message != "" {
		return fmt.Errorf("la API rechazó el turno (%s): %s: %s", resp.Status, env.Error.Type, env.Error.Message)
	}
	if texto == "" {
		return fmt.Errorf("la API rechazó el turno (%s) sin decir por qué", resp.Status)
	}
	return fmt.Errorf("la API rechazó el turno (%s): %s", resp.Status, texto)
}

// pensamientoAPI es el bloque `thinking` del request.
type pensamientoAPI struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// mensajeAPI es un turno del transcript en el formato del request. El contenido viaja crudo:
// son los mismos bloques que el cliente mandó, que es justo el punto de este motor.
type mensajeAPI struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

// cuerpoAPI es el request de /v1/messages. Los tags son explícitos y en snake_case porque es
// un DTO que cruza la red: un campo mal nombrado NO falla, la API lo ignora y el turno sale
// con otro comportamiento.
type cuerpoAPI struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream"`
	Messages  []mensajeAPI    `json:"messages"`
	System    string          `json:"system,omitempty"`
	Tools     json.RawMessage `json:"tools,omitempty"`
	Thinking  *pensamientoAPI `json:"thinking,omitempty"`
}

// cuerpoDelRequest serializa el Pedido al cuerpo de /v1/messages, validando lo que la API
// rechazaría con un 400 que cuesta más diagnosticar que prevenir.
func cuerpoDelRequest(p Pedido) ([]byte, error) {
	if p.Modelo == "" {
		return nil, errors.New("pedido sin modelo: la Messages API lo exige")
	}
	tope := p.MaxTokens
	if tope <= 0 {
		tope = MaxTokensPorDefecto
	}

	c := cuerpoAPI{
		Model:     p.Modelo,
		MaxTokens: tope,
		Stream:    true,
		System:    p.SystemPrompt,
		Tools:     p.Tools,
	}
	if p.Thinking > 0 {
		// La API EXIGE max_tokens > budget_tokens, y el 400 que da si no se cumple habla
		// de los dos campos sin decir cuál cambiar. Se valida acá para que el error nombre
		// el arreglo.
		if tope <= p.Thinking {
			return nil, fmt.Errorf("max_tokens (%d) tiene que ser mayor que el budget de thinking (%d)", tope, p.Thinking)
		}
		c.Thinking = &pensamientoAPI{Type: "enabled", BudgetTokens: p.Thinking}
	}

	for i, m := range p.Mensajes {
		if m.Rol != "user" && m.Rol != "assistant" {
			return nil, fmt.Errorf("mensaje %d: rol %q no es user ni assistant", i, m.Rol)
		}
		if len(m.Contenido) == 0 {
			return nil, fmt.Errorf("mensaje %d: sin contenido", i)
		}
		c.Messages = append(c.Messages, mensajeAPI{Role: m.Rol, Content: m.Contenido})
	}

	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serializar el request: %w", err)
	}
	return b, nil
}

func (c *ClienteAPI) nombreDeLaVariable() string {
	if c.ClaveEnv != "" {
		return c.ClaveEnv
	}
	return "ANTHROPIC_API_KEY"
}

func (c *ClienteAPI) clave() string {
	if c.Clave != "" {
		return c.Clave
	}
	return os.Getenv(c.nombreDeLaVariable())
}

// clienteHTTP devuelve el cliente a usar.
//
// 🔴 EL TIMEOUT VA EN CERO, Y NO ES UN OLVIDO. `http.Client.Timeout` cubre la vida ENTERA
// del request, LECTURA DEL CUERPO INCLUIDA: en un stream que dura lo que dura el turno, un
// timeout de cliente corta la respuesta a mitad de camino y el cliente remoto ve un turno
// truncado sin ningún error que lo explique. El tope de un turno es trabajo del ctx que
// trae el llamador, que es el único que sabe cuánto está dispuesto a esperar.
//
// Lo que sí se acota es lo que pasa ANTES del stream: conectar, el handshake TLS y la espera
// de las cabeceras. Eso no crece con el largo de la respuesta.
func (c *ClienteAPI) clienteHTTP() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	// no habla con el cerebro: este cliente disca `Endpoint`, que es el /v1/messages de Anthropic
	// o una pasarela compatible, y nunca el cerebro central — contra api.anthropic.com el nombre
	// TLS del tailnet no tiene nada que verificar.
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}
