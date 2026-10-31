# Propuesta — `motor`: servir turnos de modelo al protocolo de `streamProxy`

## El problema

Musubi tiene memoria, orquestación y un cerebro central, pero **no tiene un cuerpo**: no hay
una interfaz donde todo eso se use. ThinkRail (el IDE basado en `pi`) sí tiene cuerpo, y su
motor (`pi-agent-core`) ya sabe hacer todo lo que falta: el loop de tools, la compactación del
contexto, el árbol de sesiones, el renderizado del transcript.

`pi-agent-core` obtiene los turnos del modelo a través de una función inyectable, `streamFn`.
Una de las implementaciones que ya trae es `streamProxy(model, context, {proxyUrl, authToken})`,
que hace `POST {proxyUrl}/api/stream` y consume la respuesta como SSE. **Ese es un punto de
entrada que no hay que inventar: ya existe y ya está cableado.**

## Lo que se propone

Que Musubi sirva ese endpoint. Un `POST /api/stream` que recibe el transcript que pi venía
armando, consigue un turno del modelo, traduce el stream al protocolo de eventos que pi consume,
y lo devuelve por SSE.

El resultado es que pi deja de hablarle a la API de Anthropic y le habla a Musubi, sin que pi
cambie una línea: Musubi queda **en el medio** de cada turno, que es exactamente el lugar desde
donde puede inyectar memoria, aplicar políticas y registrar lo que pasó.

## Dos motores, una interfaz

La propuesta original decía «`claude -p` y no la API directa», porque la cuota de la suscripción
ya está pagada y una API key sería un segundo gasto para hacer lo mismo. **Medir el camino mostró
que ninguno de los dos domina al otro**, y el dueño decidió que entren los dos detrás de una
interfaz (`Motor`), con la selección por configuración:

| | `Lanzador` (`claude -p`) | `ClienteAPI` (`/v1/messages`) |
|---|---|---|
| Factura | la **suscripción** ya paga | consumo por token |
| System prompt | el del CLI **más** el pedido | sólo el del cliente |
| Tools | exactamente las declaradas (`--tools ""`) | exactamente las declaradas |
| Thinking | el budget lo decide el CLI | lo decide el cliente |
| Contrato | el formato **interno** de una versión del CLI | la API versionada |

> ⚠️ La fila de **Tools** decía «las 24 del CLI **encima** de las declaradas», y era falso: `--tools ""`
> apaga las nativas y **preserva** las que llegan por MCP, así que los dos motores le dan al modelo el
> set exacto del cliente. Medido el 2026-10-29 contra el CLI 2.1.272. La única asimetría de capacidad
> que queda es **Thinking**. Los números viven en el campo `Tools` de `Pedido`
> (`internal/motor/motor.go`), que es el único lugar donde se escriben.

El costo de agregar el segundo motor resultó ser **un archivo y no un paquete**, y eso tiene una
causa medible: lo que el CLI reenvía en sus `stream_event` son eventos de la Messages API
**textuales**, sin una sola clave propia. Así que el traductor ya hablaba los dos protocolos; lo
único que cambia es la puerta de entrada (`Linea` desenvuelve el sobre del CLI, `EventoAPI` toma
el cuerpo pelado) y de dónde salen los bytes.

## El nudo del Lanzador, que es lo que hace esto difícil

`claude -p` **no es un proveedor de modelo: es un agente con su propio loop.** En una corrida
real emitió DOS `message_start`: produjo un `tool_use`, lo ejecutó él mismo, y siguió. El
protocolo de `streamProxy` espera UNA respuesta por request, porque el loop de tools lo maneja
pi del otro lado.

La solución es **cortar en el primer `message_delta`**. Eso resuelve dos cosas con una sola
decisión:

1. Deja exactamente un turno por request, que es lo que el protocolo pide.
2. **Impide que el hijo ejecute las tools.** El turno entero (incluido el `tool_use` con sus
   argumentos completos) llega ANTES del `message_delta`; cuando cortamos, el hijo todavía no
   empezó a ejecutar nada. El tool call sale en el stream y lo ejecuta pi, que es quien tiene
   el permiso del usuario y el estado de la sesión.

Esto NO se delega a `--max-turns 1`: esa bandera cuenta turnos de **usuario**, y con ella el
hijo alcanza a ejecutar las tools antes de parar.

## El segundo nudo, que apareció al medir

`claude -p --input-format stream-json` **no interpreta su stdin como un transcript.** Cada línea
`user` que entra es un **prompt nuevo**; las líneas `assistant` se inyectan por delante del
primero; y un `tool_result` se reemplaza por el texto literal `[Tool result missing due to
internal error]`. Mandarle la historia por stdin, que es lo que parecía obvio, le hacía perder la
conversación entera.

La historia se parte: el **prefijo** viaja como un `.jsonl` de sesión que se pasa por
`--resume <ruta absoluta>`, y la **cola** (el último mensaje más el turno del asistente que lo
precede, si lo hay) por stdin. El par `tool_use`/`tool_result` tiene que viajar junto por stdin.
Tres campos de ese JSONL fallan **en silencio** si se escriben mal: el `model` con el id resuelto
(con un alias caen los bloques de thinking firmados), el `isMeta:true` de la última línea del
prefijo (sin él el modelo recibe un «Continue from where you left off.») y el
`stop_reason:"tool_use"` cuando el turno pidió una tool.

## Alcance

Entra:

- La traducción del stream al protocolo de `streamProxy`, con las dos puertas (F1).
- El ciclo de vida del proceso hijo: lanzar, partir la historia, bombear, cortar (F2).
- El motor que le habla a `/v1/messages` directo, detrás de la misma interfaz (F2b).
- El endpoint `POST /api/stream` con auth y SSE (F3).
- Un servidor MCP espejo que *declare* las tools que pi trae en su `TranscriptContext`, para que
  el modelo las pueda llamar aunque las ejecute pi (F4).
- El subcomando que lo levanta (F5).
- El cableado del lado de ThinkRail (F6).

No entra:

- Tocar `pi-agent-core`. Si hace falta cambiar pi, la vía elegida era otra.
- Proveedores que no sean los dos de arriba. La interfaz queda, un tercero no.
- Cualquier cosa que el cliente no consuma: el protocolo manda, y lo que `proxy.ts` no lee no
  se emite.

## Riesgos conocidos al proponer

- **El `default` del switch de `processProxyEvent` sólo hace `console.warn`.** Un `type` mal
  escrito no falla: pierde el evento en silencio. El contrato de nombres hay que verificarlo
  contra el fuente, no contra la memoria.
- **Formato no versionado** del lado del Lanzador. El stream de `claude -p` es una interfaz de
  CLI, no una API con contrato. Puede cambiar en una actualización. Mitigación: tests de punta a
  punta contra el binario real que avisen cuando cambie.
- **El `usage` no trae tarifas.** `cost` se emite en cero a propósito, y queda documentado donde
  se emite para que nadie lo lea como un gasto medido.
- **La contaminación del Lanzador no se puede sacar.** El CLI agrega su `# Environment` con el
  cwd y la plataforma del host, sus tools nativas y dos bloques fijos de system. La única bandera
  que lo quita (`--bare`) exige `ANTHROPIC_API_KEY`, con lo cual deja de ser gratis — y ése es
  justamente el motivo por el que el segundo motor existe.
  **Medido después, en F4 (2026-10-29): el riesgo quedó a medias.** Las tools nativas SÍ se sacan,
  con `--tools ""`; el `# Environment` y los dos bloques de system no. Y el «24» que esta línea
  decía era un número inventado al proponer: nunca se midió. Ver `tasks.md` F4.
