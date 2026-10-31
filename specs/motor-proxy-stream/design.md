# Diseño — `internal/motor`

## La forma

```
                        ┌──────────────────────────────┐
   POST /api/stream ──▶ │  Motor (interfaz, motor.go)  │
   (pi / streamProxy)   │  Turno(ctx, Pedido, emitir)  │
                        └───────────┬──────────────────┘
                            ┌───────┴────────┐
                            ▼                ▼
                   ┌─────────────────┐  ┌──────────────────┐
                   │ Lanzador        │  │ ClienteAPI       │
                   │ lanzador.go     │  │ api.go           │
                   │ `claude -p`     │  │ POST /v1/messages│
                   └────────┬────────┘  └────────┬─────────┘
                            │  .Linea(bytes)     │  .EventoAPI(bytes)
                            └────────┬───────────┘
                                     ▼
                        ┌────────────────────────────┐
                        │ Traductor (traductor.go)   │
                        │ estado del turno + 12 ev.  │
                        └────────────┬───────────────┘
                                     ▼
                              emitir(Evento)
```

Una sola pieza tiene estado: el `Traductor`. Sabe qué bloque está abierto en qué índice, si el
turno ya cerró, y el `usage` parcial. Los dos motores son transporte.

## Por qué el traductor tiene DOS puertas y eso no es una duplicación

`Linea([]byte)` desenvuelve `{"type":"stream_event","event":<evento>}`, que es lo que el CLI
escribe en su stdout. `EventoAPI([]byte)` toma ese `<evento>` pelado, que es lo que manda el SSE
de `/v1/messages`.

**Son la misma función con y sin sobre**, y eso no es una suposición: se midió que los
`stream_event` del CLI son eventos de la Messages API **textuales**, sin una sola clave propia.
Ésa es la medición que hace que el segundo motor cueste un archivo. Si el CLI algún día agrega
una clave propia, la divergencia aparece en `Linea` y el resto del paquete no se entera.

## El corte: un turno por request

El protocolo de `streamProxy` espera UNA respuesta por request; el loop de tools lo maneja pi.
Los dos motores cortan en el **primer `message_delta`**, y el traductor es el que lo decide
(`Cerrado()`), no el transporte. En el Lanzador eso además impide que el hijo ejecute las tools:
el `tool_use` completo llega antes del `message_delta`.

## El ciclo de vida del Lanzador

Nueve caminos de salida, y el orden del `switch` es lo que los distingue:

```go
switch {
case errors.Is(errBombeo, ErrDelConsumidor): return errBombeo   // 1º: el consumidor manda
case cerrado:                                return nil
case errors.Is(motivo, context.DeadlineExceeded): ...           // el reloj
case errors.Is(motivo, context.Canceled):         ...           // el llamador
case errBombeo != nil:                       return errBombeo   // DESPUÉS del ctx
default:                                     ...                // proceso mudo
}
```

Tres decisiones que un test encontró, no el diseño:

1. **`ErrDelConsumidor` es un sentinel**, no un error pelado. Sin él, el error de `emitir` y el
   error de LECTURA son el mismo `error` y no se les puede dar prioridades distintas. Y hace
   falta que el del consumidor gane a `cerrado`, porque el traductor marca el turno cerrado
   ANTES de que su último evento se emita: si `cerrado` ganara, `Turno` devolvería `nil` por un
   turno que el cliente no recibió.
2. **`motivo := ctx.Err()` se lee ANTES de `cancelar()`.** Después, `ctx.Err()` dice `canceled`
   siempre y el vencimiento del plazo se vuelve indistinguible del corte propio.
3. **`case errBombeo != nil` va DESPUÉS de los dos del ctx**, y esto se descubrió fallando: el
   `context.AfterFunc` que cierra el pipe para destrabar la lectura produce un
   `file already closed` que ganaba en el switch y **tapaba justo el motivo que explica por qué
   se destrabó**.

**El `defer` de cosecha no es higiene, es una fuga medida.** Si el consumidor panica, el panic
sube por `bombear` y se salta el `cmd.Wait()`. La goroutine `watchCtx` de os/exec queda bloqueada
para siempre en su send final (exec.go:881, canal sin buffer cuyo único receptor es `Wait`),
reteniendo el handle del proceso, el pipe y el transcript. En un servidor HTTP eso se **acumula**
turno a turno.

> ⚠️ Acá decía que se acumula «porque net/http recupera el panic por request». **Esa mitad es
> falsa**, y medida: el motor corre en la goroutine que larga `bombearTurno`, no en la del
> handler, y el recuperador de net/http (`(*conn).serve.func1`) no la alcanza — aparece en la
> pila de un panic del handler y está ausente en la de ésta. Mientras no hubo otro recover, un
> panic acá no se acumulaba: **mataba el proceso entero**. Lo que hoy sostiene la acumulación es
> el recover propio de `bombearTurno` (`internal/mcp/turno.go`), puesto por este mismo cambio.
> La frase costó una ronda de revisión, que la citó como premisa y concluyó lo contrario.

## La partición de la historia

`claude -p --input-format stream-json` **no interpreta su stdin como transcript**. Así que:

| Parte | Cómo viaja | Por qué |
|---|---|---|
| prefijo (todo menos el final) | `.jsonl` de sesión + `--resume <ruta absoluta>` | es la única forma en que el CLI lo lee como historia |
| cola (el último mensaje, más el `assistant` que lo precede si lo hay) | stdin | el par `tool_use`/`tool_result` tiene que viajar JUNTO |

Tres campos del JSONL con **falla silenciosa**: `model` con el id resuelto (con un alias caen los
bloques de thinking firmados), `isMeta:true` en la última línea del prefijo (sin él el modelo
recibe un «Continue from where you left off.») y `stop_reason:"tool_use"` cuando el turno pidió
una tool.

Residuo sin solución (R1): si el prefijo termina en un turno de usuario, el CLI inyecta un
`assistant: "No response requested."` sintético. No hay forma de sacarlo con lo probado.

## El ClienteAPI

Lo que lo hace distinto del Lanzador está en tres líneas:

- **`http.Client.Timeout` va en CERO, y no es un olvido.** Cubre la vida entera del request,
  **lectura del cuerpo incluida**: en un stream que dura lo que dura el turno, un timeout de
  cliente corta la respuesta a mitad y el cliente remoto ve un turno truncado sin ningún error
  que lo explique. El tope de un turno es trabajo del ctx del llamador. Lo que sí se acota es lo
  de ANTES del stream (`TLSHandshakeTimeout`, `ResponseHeaderTimeout`), que no crece con el largo
  de la respuesta.
- **`defer resp.Body.Close()` es lo que CORTA el turno.** El traductor para en el primer
  `message_delta` y volvemos con el cuerpo a medio leer: cerrarlo le dice al otro extremo que no
  mande más. Sin eso, cada turno deja una conexión colgada escribiendo bytes que nadie lee.
- **El parser de SSE acumula varias líneas `data:`** y las concatena con `\n` (spec), ignora los
  comentarios (`: algo`, que es como viajan los latidos de cualquier proxy), ignora un `[DONE]`
  de pasarela, y **despacha también al final** por si el stream termina sin la línea vacía: la
  API siempre la manda, pero perder el `message_delta` por un `\n` faltante sería tirar el turno
  entero.

Las **líneas `event:` se descartan a propósito**: el SSE manda el tipo dos veces y el `type` del
JSON es la única fuente de verdad. Leer las dos sería tener dos fuentes para el mismo dato.

## La asimetría entre los dos motores, que se achicó a una sola

`Pedido` declara `Tools` y `Thinking`. Esta sección decía que **los dos los honra sólo
`ClienteAPI`**, con este argumento: «`claude -p` enruta las tools que uno le declare por MCP como
`tool_use` común, pero las agrega **encima** de las 24 propias del CLI, y no hay forma de sacarle
las propias». **Las dos mitades eran falsas**, y no era un detalle de redacción: enrutaba a
facturar por API key un turno que la suscripción podía servir.

- **`Tools` lo honran los DOS.** `--tools ""` —documentada en el `--help` del propio binario, «Use
  "" to disable all tools»— deja las nativas en cero y **preserva** las que llegan por MCP, así
  que el Lanzador puede dejarle al modelo exactamente el set del pedido. Los números medidos no se
  copian acá a propósito: viven en el campo `Tools` de `Pedido` (`internal/motor/motor.go`), que es
  de donde salieron, y ese `24` es justamente lo que pasa cuando un número se replica en ocho
  lugares y después se mide uno solo.
- **`Thinking` sí queda.** El CLI decide su propio budget y no toma uno por bandera: pedirle 8000
  al Lanzador no hace nada — ni falla ni se cumple.

Y aparece una asimetría **nueva**, de forma y no de capacidad: por el espejo las tools viajan como
un servidor MCP, así que el CLI les pone el prefijo `mcp__pi__` antes de mostrárselas al modelo y
hay que deshacerlo en los dos eventos de vuelta (`sinElPrefijoDelEspejo`).

Está documentado **en el campo**, no en un README, porque el que lo va a leer es el que escriba
el `Pedido`.

## La puerta: por qué el endpoint despacha por MÉTODO y no por ruta

La ruta no se eligió. El cliente la tiene **hardcodeada** (`proxy.ts:147`) y lo único configurable
es el host; en Musubi `/api/stream` ya estaba tomada por el feed en vivo del panel. Las dos
alternativas eran pedirle al cliente que cambiara —o sea tocar el cuerpo, que desde esta terminal
no se toca— o despachar por método en el mismo handler. Se eligió lo segundo: `GET` sigue siendo
el feed, `POST` es el turno, y el `Allow` dice la verdad en los dos estados (sin motor no anuncia
`POST`, porque anunciarlo mandaría al cliente a estrellarse por otra puerta).

El 405 por motor apagado sale **antes** de mirar la credencial. No es un descuido del orden: sin
motor no hay nada que proteger, y pedir credencial para contestar un 405 le confirma a quien
sondea que ahí hay algo.

Y hay una pieza que el plan no tenía y que es de cara al usuario: **`WriteHeader` no baja las
cabeceras al cable**. Sólo las pone en el buffer del server. El cliente hace `await fetch(...)` y
recién entonces mira `response.ok`, así que sin un `rc.Flush()` propio queda bloqueado todo lo
que tarde el primer token — decenas de segundos con thinking largo, sin poder distinguir «está
pensando» de «no conectó». Medido: lo único que destrababa el buffer era el primer latido, a los
20 s, lo que lo volvía un defecto que se tolera en vez de uno que se ve.

El `http.ResponseWriter` **no es seguro para uso concurrente**, así que el motor corre en su propia
goroutine y el handler es el único que escribe: canal de eventos sin buffer (el motor no corre
adelante del cable) y canal de resultado con buffer 1 (si no, se filtra una goroutine por cada
cliente que se desconecta a mitad de turno).

## El traductor de ENTRADA, y el estado oculto que es el ORDEN

El transcript que manda pi **no es** el de la Messages API, y eso el plan no lo tenía. Son cinco
renombres silenciosos —`arguments`→`input`, `parameters`→`input_schema`,
`thinkingSignature`→`signature`, `{data,mimeType}`→`source{type,media_type,data}`, y el
`toolResult` que es un mensaje **user**— más tres reglas que la API exige: `input_schema`
obligatorio, los N resultados de un turno en UN mensaje, y el `redacted_thinking` como tipo propio
de bloque. Ninguno falla ruidosamente: la API contesta un 400 que habla de un campo que el cliente
nunca escribió.

De ahí sale la decisión menos obvia del archivo: **los bloques se REESCRIBEN, no se pasan tal
cual.** Un passthrough sería más corto y más rápido, y sería un 400 — el `TextContent` de pi puede
traer `textSignature` y el `ToolCall` trae `thoughtSignature`/`namespace`, campos que la API no
conoce y que hacen rechazar el request entero.

Y `mapaOrdenado[T]` existe porque **el orden es semántica, no presentación**. pi arma sus secciones
de system y su set de tools con un `Map` de JS, donde re-asignar una clave existente conserva su
posición y borrarla y reponerla la manda al final. Con un `map` de Go el system prompt saldría en
un orden distinto en cada corrida — una falla que no se ve como un bug sino como «el modelo
responde distinto a veces», que es la clase de defecto que nadie encuentra buscándolo.

## Por qué la selección nace apagada

`motor.provider` sin valor devuelve `(nil, nil)`: ningún motor y ningún error. Es el estado de toda
instalación de hoy, y un error ahí haría fallar el arranque en cada máquina que no configuró un
motor, o sea en todas. Pero un valor **mal escrito** sí es error: quien puso `claude` QUISO prender
el motor, y apagarlo en silencio le deja un 405 sin pista y un yaml que «ya tiene un motor».

Que nazca apagada no es prudencia genérica: los dos motores **facturan distinto** —el Lanzador
contra la suscripción, el `ClienteAPI` contra la API— y el stream que sale por el cable es el
mismo. Elegir por omisión sería elegir cómo se gasta la plata de otro sin que se note en el
resultado. Por eso también `api` sin credencial falla **al construir** y no al primer turno: un
error de configuración que se manifiesta como error de runtime lo ve el cliente remoto, que no
puede hacer nada con él.

## F4 · El MCP espejo, y por qué el Lanzador sin él no sirve para el caso real

Sin esta fase el Lanzador sirve turnos **sin tools**, y el cliente real siempre manda tools: el
transcript de pi las trae plegadas dentro de sus mensajes `system` (`toolsAdded`/`toolsRemoved`), así
que un agent loop contra el Lanzador se queda con un modelo que no tiene nada que pedir. El
`ClienteAPI` no tiene el problema —las manda en el request y listo—, pero es el motor que **cobra por
token**, y la razón de existir de este paquete es el otro: el que factura contra la suscripción.

**El CLI no toma tools por bandera. Las toma por MCP**, y eso ya está medido: enruta como `tool_use`
común lo que uno le declare por ahí. Lo que esta fase resuelve es cómo declararlas sin tocar el disco
del usuario ni dejar un proceso de más.

Lo verificado contra el binario local (**CLI 2.1.272**, `claude --help`):

| Bandera | Lo que dice, textual | Para qué sirve acá |
|---|---|---|
| `--mcp-config <configs...>` | «Load MCP servers from JSON files **or strings**» | acepta las dos formas, y el diseño eligió el **archivo**: el JSON lleva la credencial del espejo y el argv de un proceso lo lee cualquiera de la máquina. El esquema de las tools del cliente igual nunca toca el disco —lo sirve el espejo por HTTP—, pero el config sí: un directorio `musubi-mcp-*` por turno (ver `escribirConfig`) |
| `--strict-mcp-config` | «Only use MCP servers from `--mcp-config`, ignoring all other MCP configurations» | el hijo no hereda el `.mcp.json` del proyecto, que es el que haría a Musubi recursar sobre sí mismo |
| `-t, --transport` de `mcp add` | «Transport type (**stdio, sse, http**)» | hay transporte HTTP, así que el espejo puede vivir **dentro del proceso** de Musubi |

Y una corrección de lectura que casi cambió el diseño: en la ayuda, el párrafo que menciona
`--mcp-config` junto a «CLAUDE.md dirs» pertenece a **`--bare`**, no a `--setting-sources`. Si fuera
de `--setting-sources`, el `--setting-sources ""` que el Lanzador ya pasa habría apagado también el
`--mcp-config` y toda esta fase sería imposible. `--setting-sources` dice sólo «Comma-separated list
of setting sources to load (user, project, local)»: no toca las banderas explícitas.

**El espejo vive en el Lanzador, no en `internal/mcp`.** Es la decisión de forma de esta fase. El
handler del turno (F3) está en `internal/mcp`, que **importa** `internal/motor`; colgar el espejo ahí
obligaría al motor a importar al mcp y eso es un ciclo. El Lanzador levanta su propio
`net/http.Server` en `127.0.0.1:0` por turno, le pasa la URL al hijo y lo cierra en el `defer`: el
ciclo de vida del espejo es exactamente el del turno, no hay puerto fijo que reservar, no hay estado
global que dos turnos concurrentes se puedan pisar, y el precio es un JSON-RPC mínimo
(`initialize`, `tools/list`, `tools/call`) en vez de reusar el servidor MCP que el repo ya tiene.

**Un listener en loopback es alcanzable por cualquier proceso local**, y lo que expone son las
definiciones de tools del cliente. Por eso la URL lleva un token aleatorio en la ruta y el servidor
contesta 404 a cualquier otra: una capability URL, que es lo mismo que hace el resto del repo cuando
no hay dónde guardar una credencial. Vive lo que vive el turno.

**`tools/call` NUNCA se debería llamar, y ahí es donde el diseño se gana el nombre de espejo.** El
turno se corta en el `tool_use` —el cliente remoto es el que ejecuta—, así que el hijo no llega a
invocar nada. Pero «no debería» no es «no puede»: si el CLI la llama, devolver un resultado vacío
haría que el modelo siga razonando sobre una tool que nadie corrió. Contesta un error de JSON-RPC
que lo dice, y queda contado para que se vea si alguna vez pasa.

**El renombre que hay que deshacer en el camino de vuelta.** El CLI expone lo que viene por MCP como
`mcp__<servidor>__<tool>` (es el formato que este repo ya parsea en `cmd/musubi/uso_agente.go:156`),
así que el `tool_use.name` que sale en el stream **no** es el nombre que el cliente declaró, y el loop
del cliente busca por el suyo. Es el **único renombre de la VUELTA**, y ahí está lo que este párrafo
decía mal: lo llamaba «el cuarto renombre del mismo viaje, después de `arguments`/`input`,
`parameters`/`input_schema` y `thinkingSignature`/`signature`», que contaba tres de los **cuatro** de
la ida —falta `mimeType`/`media_type`, y los cuatro los enumera `internal/motor/entrada.go:23-25`— y
además lo metía en un viaje que va al revés: ésos los hace el traductor de entrada sobre lo que manda
el cliente, y éste lo hace el CLI sobre lo que vuelve del modelo. Es el más traicionero de todos
porque no falla: el cliente recibe un `tool_use` con un nombre que no tiene en su tabla y lo trata
como una tool desconocida.

Por eso el nombre del servidor espejo es **corto**: el tope de un nombre de tool es 64 caracteres, y
el prefijo se los come. `mcp__pi__` cuesta 9; `mcp__cliente__` cuesta 14. Con el prefijo largo, una
tool del cliente de 51 caracteres pasa a ser inválida sin que nadie haya escrito nada mal.

## Lo que falta

**F6, el cableado de ThinkRail.** Nada más. La medición en vivo de F4 —que era lo otro que
faltaba acá— **se corrió el 2026-10-29** con el OK del dueño, porque gasta cuota de su
suscripción, y cerró los tres puntos que esperaban un turno real: el `tool_use` llega como
`mcp__pi__<tool>`; `--allowedTools` hace match **por prefijo de servidor**, así que `mcp__pi`
autoriza a todas sin enumerarlas, y sin la bandera la tool se DENIEGA en silencio en `-p` no
interactivo; y las tools propias del CLI se acotan con **`--tools ""`**, no con el
`--disallowedTools` que proponía el plan. Los números y dónde vive cada evidencia están en F4 de
`tasks.md`.

**El patrón «se construye y no se enciende» quedó cerrado con F5**, que no era el subcomando que el
plan nombraba: no hay un `musubi motor` nuevo. El motor se construye en `runServe` y entra al
servidor por una option, porque `serve` es el único comando que sirve el mux HTTP donde vive la
ruta. Con eso `motor.Desde` y `motor.DescribirMotor` dejaron de tener cero callers de producción, y
hay cuatro guardas sobre el AST de `main.go` para que no vuelvan a tenerlos sin que nada se ponga
rojo.

Lo que **sí** sigue abierto del encendido es el otro extremo: el cliente. La ruta existe, contesta y
está medida, pero el único consumidor real —ThinkRail— todavía apunta a la API de Anthropic, y eso
es F6. ⛔ No se toca el cuerpo desde esta terminal: va cuando el dueño lo pida.
