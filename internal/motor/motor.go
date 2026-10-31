package motor

import (
	"context"
	"encoding/json"
)

// Motor sirve UN turno de modelo: recibe el transcript, emite los eventos del protocolo del
// proxy por `emitir`, y vuelve cuando el turno cerró o falló.
//
// POR QUÉ ES UNA INTERFAZ Y NO UNA FUNCIÓN SUELTA. Hay dos formas de conseguir ese turno, y
// ninguna domina a la otra:
//
//   - Lanzador (lanzador.go) corre el binario `claude` en modo headless. Factura contra la
//     SUSCRIPCIÓN del usuario, que es la razón por la que este paquete existe. Precio
//     medido el 2026-10-28 y ACOTADO el 2026-10-29: el CLI agrega su propio contexto al
//     prompt —el `# Environment` con el cwd y la plataforma del host, y dos bloques fijos
//     de system— y el cliente remoto no controla nada de eso. Sólo `--bare` lo quita, y
//     `--bare` exige ANTHROPIC_API_KEY, o sea que deja de ser gratis. En esta lista
//     figuraban también las tools nativas del CLI, y ésas SÍ se pudieron sacar: `--tools
//     ""` las deja en cero y preserva las del espejo (ver el campo Tools de Pedido).
//   - ClienteAPI (api.go) le habla a /v1/messages directo. Contrato exacto, cero
//     contaminación, nada que dependa del formato interno de una versión del CLI. Precio:
//     consumo por token.
//
// Las dos comparten TODO lo demás —el Traductor y los doce eventos del protocolo—, porque
// lo que el CLI reenvía son eventos de la Messages API envueltos, sin una sola clave propia.
// Esa medición es la que hace que esta interfaz cueste tan poco.
type Motor interface {
	Turno(ctx context.Context, p Pedido, emitir func(Evento) error) error
}

// Mensaje es un turno del transcript que se le manda al modelo. El contenido viaja como
// bloques CRUDOS de la Messages API (`{"type":"text",...}`, `{"type":"tool_use",...}`,
// `{"type":"tool_result",...}`) en vez de un string: el cliente ya los tiene armados y
// aplanarlos a texto perdería los tool results, que son justamente lo que hace avanzar el
// loop del agente remoto.
type Mensaje struct {
	Rol       string            // "user" | "assistant"
	Contenido []json.RawMessage // bloques tal como los mandó el cliente
}

// TextoDeUsuario arma el mensaje más simple posible. Es para los tests y para un pedido de
// una sola línea; el camino normal recibe los bloques ya formados.
func TextoDeUsuario(texto string) Mensaje {
	b, _ := json.Marshal(map[string]string{"type": "text", "text": texto})
	return Mensaje{Rol: "user", Contenido: []json.RawMessage{b}}
}

// TextoDeAsistente arma un turno del asistente con un solo bloque de texto. Para los tests
// de continuación, que necesitan un transcript con turnos de los dos lados.
func TextoDeAsistente(texto string) Mensaje {
	b, _ := json.Marshal(map[string]string{"type": "text", "text": texto})
	return Mensaje{Rol: "assistant", Contenido: []json.RawMessage{b}}
}

// Pedido es un turno a servir: el transcript completo más los parámetros del modelo.
type Pedido struct {
	// Modelo es lo que el cliente pidió. Puede ser un id completo
	// ("claude-sonnet-5") o un alias que el proveedor resuelva ("sonnet").
	Modelo string

	// ModeloResuelto es el id COMPLETO del modelo, y existe por un gotcha medido que no
	// avisa de ninguna forma: el Lanzador tiene que escribir el modelo en cada turno del
	// asistente que le pasa al CLI, y si ahí va un ALIAS el CLI DESCARTA los bloques de
	// thinking firmados en silencio. A/B de un factor el 2026-10-28: con el id resuelto se
	// conservan; con "sonnet", caen; sin el campo, caen. Vacío = se usa Modelo, que es
	// correcto cuando el cliente ya mandó el id completo.
	ModeloResuelto string

	SystemPrompt string
	Mensajes     []Mensaje

	// Directorio donde corre el proceso. Importa porque el modelo ve rutas relativas a
	// él; vacío = el del proceso padre. Sólo lo usa el Lanzador.
	Dir string

	// MaxTokens es el tope de la respuesta. Sólo lo usa ClienteAPI, donde el campo es
	// OBLIGATORIO en el request; el CLI lo fija por su cuenta (medido: 64000). Cero =
	// MaxTokensPorDefecto.
	MaxTokens int

	// Tools son las definiciones de tools del cliente, crudas, tal como las manda. Es lo
	// que hace que un agent loop remoto funcione: sin ellas el modelo no tiene nada que
	// pedir.
	//
	// LOS DOS MOTORES LE DAN AL MODELO EL SET EXACTO DEL CLIENTE, y este comentario decía lo
	// contrario. La versión vieja afirmaba, como si estuviera medido, que el Lanzador «agrega
	// [las tools del cliente] ENCIMA de las 24 propias del CLI, y no hay forma de sacarle las
	// propias», y mandaba a ClienteAPI a quien necesitara su set exacto. Era falso, y costaba
	// plata: enrutaba a facturar por API key un turno que la suscripción podía servir.
	//
	// Medido el 2026-10-29 contra el 2.1.272, leyendo el evento `system/init` del propio CLI:
	//
	//	sin banderas                      -> 42 tools y 4 servidores MCP (12 son conectores del host)
	//	--strict-mcp-config               -> 30 tools y 1 servidor (se van los conectores)
	//	--strict-mcp-config --tools ""    -> sólo las del espejo (en la corrida, 1: la del pedido)
	//
	// LO QUE NO SE MIDIÓ NO SE ESCRIBE COMO SI SÍ. Este bloque desglosaba el 42 en «24 nativas
	// + ToolSearch + 12 conectores», y ese 24 no salió del `init`: venía arrastrado del
	// comentario que este mismo párrafo refuta, con lo cual la corrección reponía el número
	// falso una línea más abajo. Del `init` salen los dos totales y los conectores; cuántas de
	// las 30 son nativas no se contó, y de la resta no se deduce porque `--tools ""` también
	// se lleva `ToolSearch`. Si hace falta el número, se vuelve a leer el `init` —cuesta cero
	// cuota— en vez de deducirlo.
	//
	// `--tools ""` está documentada en el `--help` del binario («Use "" to disable all
	// tools») y PRESERVA las tools que llegan por MCP, así que el Lanzador puede dejar al
	// modelo con exactamente las del pedido. Ver argumentosCLI en lanzador.go.
	//
	// LA ASIMETRÍA QUE SÍ QUEDA, y es otra: por el espejo las tools viajan como un servidor
	// MCP, así que el CLI les pone el prefijo `mcp__pi__` antes de mostrárselas al modelo y
	// hay que sacárselo en los dos eventos de vuelta (ver sinElPrefijoDelEspejo). ClienteAPI
	// no necesita nada de eso porque las manda tal cual.
	Tools json.RawMessage

	// Thinking es el budget de tokens de razonamiento. Cero = apagado.
	//
	// ⚠️ LA OTRA ASIMETRÍA: sólo lo usa ClienteAPI. El CLI decide su propio budget y no
	// toma uno por bandera, así que pedirle 8000 al Lanzador no hace nada — ni falla ni se
	// cumple. Declarado acá igual para que el día que la bandera exista no haya que cambiar
	// la interfaz.
	Thinking int
}

// modeloParaElTranscript devuelve el id que va escrito en los turnos del asistente. Ver el
// comentario de ModeloResuelto: errarle no da un error, da un turno sin su thinking.
func (p Pedido) modeloParaElTranscript() string {
	if p.ModeloResuelto != "" {
		return p.ModeloResuelto
	}
	return p.Modelo
}

// tieneToolUse dice si un mensaje del asistente pidió una tool. Hace falta para declarar su
// `stop_reason` en el transcript: un turno con tool_use que se declare "end_turn" le dice al
// modelo que la conversación había terminado cuando en realidad estaba esperando un
// resultado.
func (m Mensaje) tieneToolUse() bool {
	for _, b := range m.Contenido {
		var cab struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &cab) == nil && (cab.Type == "tool_use" || cab.Type == "server_tool_use") {
			return true
		}
	}
	return false
}
