# Spec — `motor`

Cada invariante lleva el test que lo cubre y el resultado de haber **saboteado el código que ese
test dice cubrir**. La regla del repo es que el sabotaje ataque *el* invariante declarado, no una
validación cualquiera: la defensa en profundidad lo tapa y el test queda vacuo.

Dos trampas que esta spec documenta porque las dos aparecieron de verdad acá:

- **sabotaje VACUO** — el cambio no altera lo que el test observa, así que el verde es correcto y
  no informa. Un caso particular: ponerse rojo **por compilación**. Eso mide al compilador.
- **SITIO EQUIVOCADO** — el test mide bien, pero otra cosa; o el fixture **no llega** a la línea
  que el test dice cubrir. Encontrado dos veces en esta batería (A12, A13).

Y una medición previa que cambia cómo se lee todo esto: **la cobertura se mide ANTES de creerle a
un sabotaje.** En un trabajo anterior, 27/27 sabotajes resultaron ciertos **y ciegos** porque la
función que atacaban tenía 0,0 % de cobertura. Antes de esta batería: `go tool cover -func` sobre
el paquete, ninguna función de `api.go` en cero, **89,7 % de statements**.

## Invariantes del traductor (F1)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| I1 | Un request produce **un solo turno**: el traductor corta en el primer `message_delta`, y después no emite nada más. | `TestCortaEnElPrimerTurno` · `TestDespuesDelCierreNoEmiteNada` | ROJO |
| I2 | El cierre de un bloque lleva **el tipo del bloque que se abrió**. El cliente tira `Received text_end for non-text content` si no coincide. | `TestElCierreDeBloqueRespetaSuTipo` | ROJO |
| I3 | La `signature` del bloque `thinking` viaja en el evento de **cierre**, no en los deltas. | `TestLaFirmaDelThinkingSaleEnElEnd` | ROJO |
| I4 | Un delta de **largo 0 no se reenvía**: el cliente lo concatenaría igual, pero cada uno cuesta un frame SSE. | `TestElDeltaVacioNoSeReenvia` | ROJO |
| I5 | Dos bloques consecutivos **en el mismo índice no mezclan** su contenido. | `TestBloquesConsecutivosEnElMismoIndiceNoMezclan` | ROJO sólo rompiendo **las dos** guardas (ver nota) |
| I6 | Las claves JSON emitidas son **las del protocolo de pi**, no las de la Messages API. | `TestLasClavesSonLasDelProtocolo` (9 subtests) | ROJO |
| I7 | `contentIndex` se emite **también cuando vale 0**, que es el caso más frecuente. `omitempty` lo borraría justo ahí. | `TestLasClavesSonLasDelProtocolo` | ROJO |
| I8 | La razón de corte se traduce a la del cliente. | `TestRazonDeCorte` | ROJO |
| I9 | El `usage` se traduce a `input`/`output`/`cacheRead`/`cacheWrite`/`totalTokens`. | `TestUsoSeTraduceALosNombresDePi` | ROJO |
| I10 | Un `tool_use` cuyos argumentos quedaron **truncados** no emite JSON inválido. | `TestArgumentosTruncadosNoEmitenJSONInvalido` | ROJO |
| I11 | Cerrar un bloque que **no se abrió** es un error, no un cierre fantasma. | `TestCerrarUnBloqueQueNoSeAbrioEsError` | ROJO |
| I12 | Los eventos propios del harness (`system`, `assistant`, `user`, `result`) **se ignoran**: son redundantes con los `stream_event` o son el loop del hijo. | `TestLosEventosDelHarnessSeIgnoran` | ROJO |
| I13 | Una línea **ilegible no cierra el turno**: se saltea. Un turno perdido por un byte raro es peor que un hueco. | `TestUnaLineaIlegibleNoCierraElTurno` | ROJO |
| I14 | Un `error` del proveedor **a mitad del stream** se traduce a un evento `error` con su mensaje, en vez de ignorarse. Sin esto el stream queda abierto esperando un `message_delta` que no llega y el request se cuelga hasta el tope de tiempo. | `TestUnErrorEnMedioDelStreamCierraElTurno` (por la puerta de la API) | ROJO (A20) |

**Nota sobre I5 — el invariante lo sostienen DOS guardas redundantes**, y está documentado en los
comentarios de las dos: `t.bloques[idx] = b` en `bloqueAbre` (el bloque nuevo reemplaza al que
hubiera) y `delete(t.bloques, idx)` en `bloqueCierra`. **Romper una sola deja todo en verde**: el
invariante cae recién cuando se rompen las dos. El sabotaje incluye el caso que rompe ambas
(espera ROJO) y dos casos que **esperan verde** para dejar la redundancia asentada, porque si no
el próximo que mire va a concluir que el test no mide nada — y es falso.

## Invariantes del Lanzador (F2)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| L1 | El corte **mata el proceso** en vez de esperar a que termine: `claude -p` seguiría trabajando. | `TestElCorteMataElProceso` (mide tiempo < 4 s **y** la ausencia de una marca que el hijo deja si sobrevive) | ROJO |
| L2 | Si el **consumidor falla**, el turno vuelve con ese error y **no cuelga**. | `TestSiElConsumidorFallaElTurnoVuelve` | ROJO |
| L3 | El turno se corta por **tiempo** y el error lo dice, distinguible del corte propio. | `TestElTurnoSeCortaPorTiempoYLoDice` | ROJO |
| L4 | El **cancel del llamador** corta el turno. Es el caso del cliente que cierra la conexión. | `TestElCancelDelLlamadorCortaElTurno` | ROJO |
| L5 | Una **línea larga** del harness no tira el turno. El `system/init` real pasa los 64 KB del default de `bufio.Scanner`, así que el tope está en 16 MB. | `TestUnaLineaLargaDelHarnessNoTiraElTurno` (genera 200 KB en runtime) | ROJO |
| L6 | El **argv** lleva las banderas que habilitan el protocolo. Sin `--include-partial-messages` no hay deltas y el cliente no muestra nada; sin `--settings` con la atribución vacía, el hijo firma los commits como si fuera él. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` | ROJO |
| L7 | El **transcript viaja completo**, con los turnos del asistente y sus `tool_result`. Los bloques van crudos (`json.RawMessage`) para no perder nada al re-serializar. | `TestUnTranscriptDeDosTurnosConToolsSobreviveElViaje` (`entrada_test.go`) | ROJO |
| L8 | El entorno del hijo **no hereda la sesión ni la API key**: se sacan por prefijo `CLAUDECODE`, `CLAUDE_CODE_` y `MUSUBI_`, más `ANTHROPIC_API_KEY` salvo opt-in explícito. | `TestElEntornoDelHijoNoHeredaLaSesionNiLaAPIKey` | ROJO |
| L9 | El **stderr se acota** a los últimos 4 KB y se guarda el final: es donde aparece el mensaje útil («credit balance is too low»), y el principio es ruido. | `TestElStderrSeAcotaYGuardaElFinal` | ROJO |
| L10 | La historia se **parte**: el prefijo por `--resume <ruta absoluta>` y la cola por stdin, con el par `tool_use`/`tool_result` junto. | `TestLaHistoriaSePartePrefijoYCola` · `TestLaColaViajaPorStdinConSusBloquesIntactos` · `TestElPrefijoViajaComoArchivoDeSesion` | ROJO |
| L11 | El archivo de sesión es **absoluto** (el CLI lo resuelve contra su propio cwd, que el `Pedido` puede cambiar) y **se limpia** al terminar: lleva el transcript entero en claro. | `TestElArchivoDeSesionEsAbsolutoYSeLimpia` | ROJO |
| L12 | Un error del consumidor **en el último evento** no se reporta como turno servido. El traductor marca el turno cerrado ANTES de emitir el `done`, así que `cerrado` y el error conviven. | `TestUnErrorEnElUltimoEventoNoSeReportaComoTurnoServido` | ROJO |
| L13 | Un **panic del consumidor no deja la cosecha sin hacer**. Sin el `defer`, el panic se salta el `cmd.Wait()` y fuga una goroutine por turno. | `TestUnPanicDelConsumidorNoDejaLaCosechaSinHacer` (cuenta `os/exec.(*Cmd).watchCtx` en el stack dump) | ROJO |

**Nota sobre el ciclo de vida** — un `defer cancelar()` **no alcanza** cuando hay un `cmd.Wait()`
después: el defer corre al salir de la función, o sea después del Wait, y el Wait espera a que el
hijo termine solo. Medido con el proceso falso: **10 segundos de cuelgue** cada vez que el
consumidor fallaba. El cancel va **siempre y antes del Wait**, y el motivo del corte
(`context.DeadlineExceeded`) se mira **antes** de cancelar, porque después `ctx.Err()` dice
`canceled` siempre y el vencimiento del tope se vuelve indistinguible del corte propio.

**Nota sobre el orden del switch, que lo encontró un test fallando** — el `context.AfterFunc` que
cierra el pipe para destrabar la lectura produce un `read |0: file already closed`, y ese error
**ganaba en el switch y tapaba justo el motivo que explica por qué se destrabó**. Por eso
`case errBombeo != nil` va DESPUÉS de los dos casos del ctx, y por eso el error del consumidor
necesita el sentinel `ErrDelConsumidor`: sin él, el error de `emitir` y el de lectura son el mismo
`error` pelado y no hay forma de darles prioridades distintas.

**Nota sobre los huérfanos** — no hace falta un Job Object ni matar un árbol de procesos:
`claude.exe` es un binario nativo único, no un wrapper de node, y no deja nietos. Medido
muestreando por PID durante un turno real: **1 proceso, 0 huérfanos** cuatro segundos después del
corte.

**Nota sobre los tres campos de falla silenciosa del JSONL de sesión** — `model` con el id
**resuelto** (con un alias como `sonnet` los bloques de thinking firmados se descartan sin aviso),
`isMeta:true` en la última línea del prefijo (sin él el modelo recibe un «Continue from where you
left off.»), y `stop_reason:"tool_use"` cuando ese turno pidió una tool. Ninguno de los tres da
error: dan un turno distinto.

**Residuo R1, sin solución conocida** — si el prefijo termina en un turno de usuario, el CLI
inyecta un `assistant: "No response requested."` sintético. Ninguna de las 17 variantes probadas
lo evita.

## Invariantes del ClienteAPI (F2b)

Batería propia: **20 casos, 18 ROJO + 2 que esperan VERDE con su motivo**, ninguno vacuo. (El reparto
se cuenta de la tabla de abajo, que es la que manda: la versión previa de esta línea decía 17 + 3 y
no cerraba contra sus propias filas.)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| A1 | El request lleva `stream:true`. Sin eso la API contesta un mensaje entero de una vez y este paquete —que es todo deltas— no tiene nada que traducir. | `TestElRequestALaAPILlevaLoQueLaAPIExige` | ROJO |
| A2 | Lleva `anthropic-version`. La API lo **exige**. | idem | ROJO |
| A3 | Lleva `accept: text/event-stream`. | idem | ROJO |
| A4 | Las betas van en **UNA** cabecera separada por comas, no una por beta. | idem | ROJO |
| A5 | Las **tools del cliente viajan crudas**. Sin ellas el modelo no tiene nada que pedir y el agent loop remoto no arranca. | idem | ROJO |
| A6 | El `max_tokens` por defecto se aplica: el campo es **obligatorio** en el request. | idem | ROJO |
| A7 | `max_tokens > budget_tokens` se valida **antes de salir a la red**: el 400 de la API habla de los dos campos sin decir cuál cambiar. | `TestElPedidoSeValidaAntesDeGastarUnRequest` | ROJO |
| A8 | Un status distinto de 200 **no se parsea como si fuera el stream**. | `TestUnRechazoDeLaAPISaleConSuMensaje` | ROJO |
| A9 | El **cuerpo del rechazo llega al error**. Es lo único que dice por qué (saldo agotado, modelo inexistente); tirarlo deja un «status 400» que no se diagnostica. | idem | ROJO |
| A10 | Varias líneas `data:` de un mismo evento se **concatenan con `\n`** (spec de SSE). Un JSON partido en dos llegaría como dos JSON inválidos. | `TestElParseoDelSSEAguantaLasRarezasDelFormato` | ROJO |
| A11 | Los **latidos** (comentarios `: algo`) no cambian el turno. | idem | **VERDE esperado** (ver nota) |
| A12 | Un `[DONE]` de pasarela no se parsea como JSON. | idem | **VERDE esperado** (ver nota) |
| A13 | Un stream que termina **sin la línea vacía final** no pierde el último evento. Perder el `message_delta` por un `\n` faltante sería tirar el turno entero. | idem | ROJO |
| A14 | El error del consumidor **le gana a `cerrado`**: el traductor marca el turno cerrado antes de emitir el `done`, así que si `cerrado` ganara, `Turno` devolvería `nil` por un turno que el cliente no recibió. | `TestEnLaAPIElErrorDelConsumidorTambienLeGanaAlCierre` | ROJO |
| A15 | El **cancel a mitad del stream** se reporta como cancel del llamador, no como falla de lectura. | `TestEnLaAPIElCancelDelLlamadorNoParaceUnaFalla` | ROJO |
| A16 | El **cancel antes de las cabeceras** también. Es otro camino —el error vuelve del transporte, no del scanner— y en un servidor es el más frecuente de los dos: un cliente que se va antes del primer token. | `TestUnCancelAntesDeLasCabecerasTambienSeNombra` | ROJO |
| A17 | 🔴 El cliente HTTP **no le pone timeout al stream**. `http.Client.Timeout` cubre la lectura del cuerpo: cortaría la respuesta a mitad de turno y el cliente remoto vería un turno truncado sin ningún error que lo explique. Lo que sí se acota es lo de ANTES del stream. | `TestElClienteHTTPNoLePoneTimeoutAlStream` | ROJO |
| A18 | **El cuerpo se cierra, y eso es lo que corta el turno.** El traductor para en el primer `message_delta` y volvemos con el cuerpo a medio leer; sin cerrarlo, cada turno deja una conexión colgada escribiendo bytes que nadie lee. | `TestAlCerrarElTurnoSeCortaLaConexion` (lo mide **del lado del servidor**, contando escrituras) | ROJO |
| A19 | `cortarCampo` come **un** espacio tras los dos puntos, que es lo que la spec define como separador. | `TestCortarCampoRespetaElValor` | ROJO |
| A20 | = I14, por la puerta de la API. | `TestUnErrorEnMedioDelStreamCierraElTurno` | ROJO |

**Nota sobre A11 y A12 — las dos guardas son redundantes EN EFECTO, y está escrito en el código.**
Un `: latido` tampoco pasa el `cortarCampo(linea, "data")` que viene abajo, así que el latido se
descarta igual. Y un `[DONE]` sin guarda baja al traductor, no parsea, y se **saltea** por I13: lo
único que la guarda ahorra es una línea `motor: …` en stderr que se lee como un bug cuando no lo
es. **Lo único que cambian es el log, y eso el test no lo puede medir** sin inventar maquinaria
para observar stderr. Quedan en la batería, con el verde esperado y el motivo, para que el próximo
que la corra no concluya que el test está vacío.

**Nota sobre A12 y A13 — las dos eran SITIO EQUIVOCADO, y el sabotaje las encontró.** El `[DONE]`
estaba pegado al final del fixture, que es donde una pasarela lo pone… y ahí **la línea no se lee
nunca**, porque el turno cierra en el `message_delta` y `bombearSSE` vuelve en ese punto. Igual
A13: recortar el `\n` del final del fixture deja el `message_delta` con su línea vacía intacta, así
que el despacho de después del loop tampoco corría. Los dos fixtures ahora cortan **en el
`message_delta`**, que es el único lugar donde esos caminos se ejercitan.

## Invariantes del traductor de entrada (F3a)

El transcript que manda pi **no es** el de la Messages API, y eso el plan no lo tenía. Son cinco
renombres silenciosos y tres reglas que la API exige: nada de eso falla ruidosamente, la API
contesta un 400 que habla de un campo que el cliente nunca escribió.

Cobertura antes de esta batería (`go tool cover -func`): ninguna función de `entrada.go` en cero,
mínimo 75,0 % (`secciones.UnmarshalJSON`), **88,9 % del paquete**.

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| E1 | Un pedido **sin `model.id`** se rechaza antes de traducir. | `TestElPedidoDelProxySeValidaAntesDeTraducir` | ROJO |
| E2 | Un pedido **sin `context.messages`** se rechaza. | idem | ROJO |
| E3 | Un transcript de **puro `system`** no es un turno: no hay nada que responder, y los dos motores lo rechazarían después con un error peor, ya del lado del proveedor. | idem | ROJO |
| E4 | El id del modelo va en **los dos campos** (`Modelo` y `ModeloResuelto`). Sin el resuelto, el Lanzador escribe un alias en los turnos del asistente y el CLI **descarta los bloques de thinking firmados en silencio**. | `TestElIdDelModeloViajaEnLosDosCamposDelPedido` | ROJO |
| E5 | `options.maxTokens` le **gana** al del modelo; el del modelo es el techo del catálogo, no lo que el cliente pidió. | `TestElTopeDeTokensDelPedidoLeGanaAlDelModelo` | ROJO |
| E6 | Los bloques de texto **dentro** de un mensaje system se juntan con `"\n"` (`contentText`, text.ts:6). | `TestElSystemPromptSeReproduceComoLoHacePi` | ROJO |
| E7 | Los mensajes system **entre sí** se juntan con `"\n\n"` (`getCurrentSystemMessage`, transcript.ts:84). Son dos separadores distintos en el mismo prompt y confundirlos cambia el texto que ve el modelo. | idem | ROJO |
| E8 | Una sección **vacía no deja su hueco** en el prompt, y se filtra al final: una sección puede quedar en `""` después de redefinirse. | `TestUnSystemSinTextoNoDejaUnHuecoEnElPrompt` | ROJO tras agregar el caso de **sección** (ver nota) |
| E9 | Las secciones van **después** del contenido, no intercaladas (`getSystemMessageText`, text.ts:17). | `TestLasSeccionesVanDespuesDelContenidoYRespetanElOrdenDeMap` | ROJO |
| E10 | Una sección en **`null` se borra**. Es un tercer estado —ausente, con texto, presente-en-null— y un string vacío no alcanza. | `TestUnaSeccionEnNullSeBorraYAlVolverVaAlFinal` | ROJO |
| E11 | Las tools **quitadas se procesan antes** que las agregadas (`getCurrentTools`, transcript.ts:62). Al revés, un mensaje que redefine una tool —quitarla y reponerla en el mismo paso— la perdería. | `TestLasToolsQuitadasSeProcesanAntesQueLasAgregadas` | ROJO |
| E12 | 🔴 Re-asignar una clave **conserva su posición**; borrarla y reponerla la manda al final. Es la semántica del `Map` de JS, y un `map` de Go daría un prompt distinto en cada corrida — una falla que se ve como un modelo que «responde distinto a veces». | `TestUnaToolRedefinidaConservaSuPosicion` | ROJO |
| E13 | `input_schema` es **OBLIGATORIO**: una tool sin parámetros recibe `{"type":"object","properties":{}}`. Omitir el campo rechaza el request entero, o sea que **una sola** tool sin parámetros se llevaría a todas las demás. | `TestUnaToolSinParametrosRecibeUnEsquemaVacioYNoOmiteElCampo` | ROJO |
| E14 | El campo se llama `input_schema`, no `parameters`. | idem | ROJO |
| E15 | Un `content` que es un **string pelado** es un bloque de texto: el protocolo admite las dos formas. | `TestElContentComoStringPeladoEsUnBloqueDeTexto` | ROJO |
| E16 | Una imagen viaja como `source{type:"base64", media_type, data}`, no como `{data, mimeType}`. | `TestUnaImagenSeTraduceALaFuenteBase64DeLaAPI` | ROJO |
| E17 | 🔴 Los bloques se **REESCRIBEN**, no se pasan tal cual. El `TextContent` de pi puede traer `textSignature` y el `ToolCall` trae `thoughtSignature`/`namespace`: campos que la API no conoce y que hacen rechazar el request entero. Un passthrough «eficiente» acá es un 400. | `TestLosCamposPropiosDePiNoViajanALaAPI` | ROJO |
| E18 | Una llamada a tool usa `input`, no `arguments`. | `TestUnaLlamadaATootUsaInputYNoArguments` | ROJO |
| E19 | Una llamada **sin argumentos** lleva `{}`: `input` es obligatorio y tiene que ser un objeto. Pasa cuando el modelo mandó argumentos que no parseaban y pi dejó `{}`. | `TestUnaLlamadaSinArgumentosLlevaUnObjetoVacio` | ROJO |
| E20 | La firma del thinking viaja como `signature`, no `thinkingSignature`. Sin ella la API rechaza el turno; con el nombre de pi viaja como campo desconocido, que es el mismo rechazo por otra puerta. | `TestElThinkingViajaConSuFirmaYConElNombreDeLaAPI` | ROJO |
| E21 | Un thinking **redactado es otro tipo de bloque** (`redacted_thinking` con el payload en `data`), no el mismo con una bandera. Reenviarlo así es lo que deja seguir una conversación que pasó por el filtro de seguridad. | `TestUnThinkingRedactadoEsOtroTipoDeBloque` | ROJO |
| E22 | Un bloque de **tipo desconocido es un error**, no se saltea. | `TestUnBloqueDeTipoDesconocidoEsUnError` | ROJO |
| E23 | Un **rol desconocido es un error**. Lo que llega es un `TranscriptContext` ya normalizado: un rol nuevo significa que el contrato cambió, y descartar un turno en silencio le cambia el contexto al modelo sin que nadie pueda notarlo. | `TestElPedidoDelProxySeValidaAntesDeTraducir` | ROJO |
| E24 | Un `toolResult` **es un mensaje `user`**: no hay rol propio en la Messages API. | `TestUnResultadoDeToolEsUnMensajeDeUsuario` | ROJO |
| E25 | Los turnos **consecutivos del mismo rol se fusionan**, que es lo que la API EXIGE: los N resultados de un turno con N `tool_use` van en UN mensaje. Mandarlos separados es un 400. | `TestVariosResultadosConsecutivosVanEnUnSoloMensaje` · `TestUnResultadoQueSigueAUnUsuarioSeFusionaConEl` | ROJO |
| E26 | `is_error` sale **sólo cuando es true**: mandar `is_error:false` en cada resultado es ruido en cada turno de un agent loop. | `TestUnResultadoConErrorLoDeclaraYLosDemasNo` | ROJO tras agregar el resultado **sin** error (ver nota) |
| E27 | Un resultado **vacío igual cierra** su llamada: un `tool_use` sin respuesta hace que la API rechace el turno. | `TestUnResultadoVacioIgualCierraSuLlamada` | ROJO |
| E28 | El clamp `xhigh`/`max` → `high` es de pi (`clampReasoning`), y un nivel desconocido cae en 0 (thinking apagado), que es la caída segura. | `TestElBudgetDeThinkingSaleDelNivelYDeLaTabla` | ROJO |
| E29 | Los `thinkingBudgets` **del cliente le ganan** a la tabla por defecto. | `TestElBudgetDeThinkingSaleDelNivelYDeLaTabla` (subtest `budgets propios`) | ROJO tras re-anclar el sabotaje (ver nota) |

## Invariantes de la selección de motor (F3b)

Los dos motores existen y los dos están verificados, pero un paquete con dos implementaciones y
ningún selector no es una elección: es código que nadie puede usar.

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| C1 | Sin proveedor **no hay motor y tampoco hay error**: es el estado de toda instalación de hoy. Un error ahí haría fallar el arranque en cada máquina que no configuró un motor — o sea en todas. | `TestSinProveedorNoHayMotorYTampocoHayError` | ROJO |
| C2 | Un `provider` mal escrito es un **error**, no un apagado silencioso. Quien escribió `claude` QUISO prender el motor; apagarlo en silencio le deja un 405 sin ninguna pista y un yaml que «ya tiene un motor». | `TestUnProveedorMalEscritoEsUnErrorQueNombraLosValidos` | ROJO |
| C3 | Y el error **nombra los válidos**: es la diferencia entre una sesión de diagnóstico y una corrección de diez segundos. | idem | ROJO |
| C4 | El valor se **normaliza** (mayúsculas y espacios): lo escribe una persona a mano en un yaml, y rechazar `"  CLI "` sería pedantería que cuesta una sesión por un espacio que nadie ve. | `TestElProveedorSeLeeSinImportarMayusculasNiEspacios` | ROJO |
| C5 | `keep_api_key` **llega** al Lanzador. No es cosmético: el sentido de `cli` es facturar con la suscripción, y una API key que se filtra al hijo la hace facturar por API sin que se note desde el cliente. | `TestElProveedorCLIRecibeLoQueDiceElYaml` | ROJO |
| C6 | Un tope de turno en **0 queda en 0** para que gane el default del Lanzador. Si la fábrica «ayudara» con un número propio, el default dejaría de estar en un solo lugar. | `TestUnTopeDeTurnoEnCeroQuedaEnCeroParaQueGaneElDefault` | ROJO |
| C7 | 🔴 `api` sin credencial falla **AL CONSTRUIR**, y el momento es el invariante. Un motor que arranca «bien» y se cae recién cuando alguien le pide un turno convierte un error de configuración en un error de runtime, y el que lo ve es el cliente remoto, que no puede hacer nada. | `TestElProveedorAPISinCredencialFallaAlConstruir` | ROJO |
| C8 | La descripción del arranque dice si la variable **TIENE** algo, nunca qué tiene. | `TestLaDescripcionDelMotorNuncaImprimeElSecreto` | ROJO |
| C9 | La descripción del motor apagado **explica el 405**, que es el síntoma que se ve del otro lado. | `TestLaDescripcionDelMotorApagadoExplicaEl405` | ROJO |

## Invariantes del endpoint (F3c)

La ruta no se eligió: el cliente la tiene **hardcodeada** (`proxy.ts:147`) y lo único configurable
es el host. Y en Musubi `/api/stream` ya estaba tomada por el feed en vivo del panel, así que la
salida es despachar por **método**.

Cobertura antes de esta batería: ninguna función de `turno.go` en cero, mínimo 79,3 %
(`bombearTurno`), **83,4 % del paquete**.

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| T1 | Sin motor, el `POST` contesta **el mismo 405** que antes de que esto existiera. Un 503 sería tentador pero diría «está pero caído» cuando nadie lo pidió. | `TestSinMotorElPostDeTurnoContesta405YNoLoAnuncia` | ROJO |
| T2 | El `Allow` **no anuncia POST** cuando no hay motor: mandaría al cliente a estrellarse por otra puerta. | idem (caso `DELETE`) | ROJO |
| T3 | Con motor **sí lo anuncia**, o el cliente no tiene forma de saber que existe. | `TestConMotorLaRutaAnunciaLosDosMetodos` | ROJO |
| T4 | El `GET` sigue siendo **el feed en vivo**. El despacho por método no puede comerse la ruta que ya estaba. | `TestElFeedEnVivoSigueAtendiendoElGetDeLaMismaRuta` | ROJO tras afirmar el feed **positivamente** (ver nota) |
| T5 | 🔴 El error **antes de las cabeceras va como JSON**, porque es lo que el cliente parsea: `body.error ?? ...` (proxy.ts:163). Un `http.Error` de texto pelado le deja un mensaje genérico y el motivo real se pierde justo cuando era el dato útil. | `TestUnPedidoIlegibleContestaJSONConLaClaveQueElClienteLee` | ROJO |
| T6 | Y la clave es **`error`**, la que el cliente lee. | idem | ROJO |
| T7 | ⚠️ **NO** va `DisallowUnknownFields`, y es al revés del resto del repo. `/api/flota` lo usa porque Musubi escribe los dos lados; este cuerpo lo escribe un cliente AJENO que manda `temperature`, `samplingParams`, `cacheRetention`, `transport`, `metadata` y el objeto `model` completo. Rechazar lo desconocido haría fallar TODOS los pedidos reales, y volvería a fallar con cada versión nueva del cliente. | `TestLosCamposQueEsteServidorNoConoceNoTumbanElPedido` | ROJO |
| T8 | El transcript **traducido** llega al motor, no el pedido crudo. | `TestElTranscriptTraducidoLlegaAlMotor` | ROJO |
| T9 | Las cuatro cabeceras del protocolo SSE salen. | `TestUnTurnoSaleComoSSEConLasCabecerasDelProtocolo` | ROJO |
| T10 | 🔴 `SetWriteDeadline(time.Time{})`. **Sin eso el turno muere a los 90 s** por el `WriteTimeout` del server, que está calibrado para las respuestas cortas de `/mcp`. El síntoma no es un error: el cliente ve el stream cortado a mitad de una frase. | `TestElTurnoLimpiaLaDeadlineDeEscrituraDelServer` | ROJO |
| T11 | 🔴 **Las cabeceras se bajan al cable antes del primer evento.** `WriteHeader` sólo las pone en el buffer del server; sin un `Flush` propio no salen hasta que algo más escriba. Y el cliente hace `await fetch(...)` y recién entonces mira `response.ok` (proxy.ts:154), así que se queda bloqueado todo lo que tarde el primer token — decenas de segundos con thinking largo, sin poder distinguir «pensando» de «no conectó». **Medido: sin el Flush, lo único que destrababa el buffer era el primer latido (20 s).** | `TestLasCabecerasLleganAntesDelPrimerEvento` | ROJO |
| T12 | 🔴 Un **cierre anómalo se nombra** con un evento `error` propio: el status 200 ya salió con las cabeceras y no se puede cambiar, así que un stream que se corta sin evento terminal deja al cliente con su fallback genérico de desconexión, que no dice nada de lo que pasó. | `TestUnCierreAnomaloEmiteUnErrorPropioConElMotivo` | ROJO |
| T13 | Un turno que **cerró bien no agrega** un segundo evento terminal: el traductor ya emitió su `done`. | `TestUnTurnoQueCierraBienNoAgregaUnSegundoEventoTerminal` | ROJO |
| T14 | El **cancel del cliente se nombra `aborted`**, no `error`. La diferencia importa del otro lado: `aborted` no merece un cartel de error, y mezclarlos hace que cada Esc del usuario se vea como una falla del servidor. | `TestElCancelDelClienteSeNombraAbortadoYNoComoFalla` | ROJO cambiando el **centinela**, no borrando el bloque (ver nota) |
| T15 | El `usage` del cierre anómalo va en **cero y no nil**: el protocolo lo declara obligatorio (proxy.ts:44), así que omitirlo rompe el tipo del otro lado. Cero es honesto — ese camino se toma cuando el turno no llegó a reportar consumo. | `TestUnCierreAnomaloEmiteUnErrorPropioConElMotivo` | ROJO |
| T16 | El turno exige **la misma credencial que el feed**. Servir un turno es la operación más cara del servidor: una puerta más floja acá que en el feed de LECTURA sería al revés de como tiene que ser. | `TestElTurnoExigeLaMismaCredencialQueElFeed` | ROJO |
| T17 | El 405 por motor apagado sale **antes** de mirar la credencial: sin motor no hay nada que proteger, y pedir credencial para contestar un 405 le dice a quien sondea que ahí hay algo. | `TestUnMotorApagadoRechazaAntesDeMirarLaCredencial` | ROJO |
| T18 | El cuerpo tiene **tope** (32 MiB). Generoso porque lo que viaja es un transcript completo con imágenes en base64; infinito dejaría que un cliente roto se coma la memoria del proceso. | `TestUnCuerpoGiganteSeCortaEnVezDeComerseLaMemoria` | ROJO |
| T19 | 🔴 El latido va como **comentario SSE** (`: latido`), no como evento. El parser del cliente descarta toda línea que no empiece con `"data: "` (proxy.ts:177), así que un latido-comentario no puede confundirse con un evento del protocolo **ni cuando el cliente cambie**. | `TestElLatidoViajaComoComentarioYNoComoEvento` · `TestElLatidoNoEsperaAQueElMotorEmita` | ROJO |
| T20 | El latido **se flushea**: sin eso queda en el buffer y no late, que es lo contrario de para lo que existe. | `TestElLatidoNoEsperaAQueElMotorEmita` | ROJO |
| T21 | 🔴 El canal de resultado tiene **buffer 1**, para que la goroutine del motor pueda terminar aunque el handler ya se haya ido. Sin él se filtra **una goroutine por cada cliente que se desconecta** a mitad de un turno. | `TestElClienteQueSeVaNoDejaGoroutinesColgadas` | ROJO **5/5** tras volver determinista el orden (ver nota) |
| T22 | Y el `emitir` **mira el ctx**: devolver el error del ctx es lo que CORTA el turno desde adentro —el motor lo propaga, mata el proceso hijo o cierra el cuerpo del request— y es lo que impide que quede alguien generando tokens que nadie va a leer. | `TestElEmitirEsElQueCortaAlMotorQueNoMiraElCtx` | ROJO (test nuevo; ver nota) |

### Lo que la batería encontró: siete casos, y no todos eran lo mismo

De 60 sabotajes, 53 dieron el veredicto esperado a la primera. Los otros siete **no eran siete
veces el mismo problema**, y la distinción es lo único que evita perseguir fantasmas: un sabotaje
mal apuntado da verde **correcto** —el invariante SÍ está medido, en otro test— y «arreglar» el
test ahí sería trabajo sobre nada.

**Tres eran huecos de test reales** (el verde era un defecto del test, y los tres se arreglaron):

- **T22 — el invariante no tenía test.** El doble del test de T21 esperaba su **propio**
  `ctx.Done()`, así que se enteraba del cancel por otra vía y **no volvía a llamar a `emitir`
  nunca**: quitarle el chequeo del ctx al emisor no cambiaba nada observable. Hizo falta un motor
  **terco** —emite para siempre, ignora el ctx— y un test nuevo. El comentario de T21, que afirmaba
  cubrir esto, era falso y se corrigió.

  **Y después siguió en verde una vuelta más, por otro motivo.** Escribí el test pero dejé la
  ancla apuntando al test viejo, así que la batería volvió a correr el que no mide. Es lo que hace
  falta saber para leer su veredicto: **«VERDE MALO» nombra un síntoma con tres causas posibles**
  —el test no mide el invariante, el sabotaje apunta a otro sitio, o la ancla nombra el test
  equivocado— y las tres se ven exactamente igual en la salida. Confundirlas cuesta una vuelta por
  cada una.
- **T4 — la aserción era demasiado débil.** Afirmaba «no es 405», y un `GET` desviado a
  `servirTurno` **no da 405: da 400**, porque un GET no trae cuerpo y el pedido no parsea. El panel
  queda igual de en blanco y el test pasaba igual. Ahora afirma **positivamente que contestó el
  feed**: status 200, `Content-Type: text/event-stream`, y su primer frame `event: backlog` — que
  es lo que lo distingue de cualquier otro 200 de SSE, porque el stream de un TURNO son `data:`
  sin nombre de evento.

  **Y el primer intento de arreglarlo salió peor que el problema, lo que deja la lección más útil
  del tramo.** Afirmé el **503** de `live feed unavailable` razonando que un servidor de prueba no
  tendría feed armado; lo tiene, así que el status real es 200. Pero el test no salió rojo: el
  camino de diagnóstico hacía `io.ReadAll(resp.Body)` sobre un stream que **por diseño no termina
  nunca**, y el paquete entero colgó hasta el timeout — 45 s, y con la cuenta corriendo el panic
  acusa al paquete. **Una expectativa equivocada se volvió un cuelgue en vez de un rojo porque la
  lectura del diagnóstico no tenía cota.** Y la lectura del camino de error es justo la que uno
  escribe distraído, porque sólo corre si algo salió mal: mientras la expectativa acierta, el test
  se ve sano. Ahora el cliente lleva `Timeout` (que cubre también la lectura del cuerpo), el frame
  que se verifica se lee con `io.ReadFull` de exactamente sus bytes —acotado porque ya están en el
  cable: el handler los flushea en `http.go:558`— y el mensaje de error usa **un** `Read` en buffer
  fijo. `io.ReadAll(io.LimitReader(body, N))` tampoco servía: si el stream manda menos de N bytes y
  se calla, sigue esperando.

  Y el modo de leer el panic es lo que separa este caso del de contención: ahí el volcado decía
  `running tests: TestX (0s)` —cero segundos, nada colgaba, era el paquete entero pasándose por
  cuatro segundos—; acá dijo `(45s)`, que es el test mismo trabado. **El paréntesis es el dato.**
- **E26 — el fixture medía la mitad del nombre.** `…LoDeclaraYLosDemasNo` tenía un solo resultado
  **en error**, donde `is_error` sale `true` por los dos caminos: mandar el campo siempre con su
  valor pasaba igual. Lo que distingue es el resultado **sin** error, donde la clave tiene que
  estar ausente y no en `false`.

**Dos eran sabotajes mal apuntados** (el verde era correcto; se corrigió la batería, no el código):

- **E8 — defensa en profundidad.** `reproducirTranscript` filtra el vacío **al entrar**
  (`if t := m.Content.texto("\n"); t != ""`), así que `textos` nunca trae `""` y el filtro del
  final nunca veía el caso. Lo que ese filtro protege de verdad es una **sección** que quedó en
  `""` al redefinirse —el propio código lo declara en `entrada.go:630`: «un string vacío […]
  significa sección vacía, que se renderiza (y se filtra después)»— y eso no tenía test. Se agregó
  el caso, y con él el sabotaje mide lo que dice medir.
- **E29 — el nombre engañaba.** El sabotaje apuntaba a
  `TestSinThinkingBudgetsSeUsanLosDePiYNoCeros`, cuyo fixture **no trae** `thinkingBudgets`: con el
  `if` borrado da lo mismo. El invariante estaba medido todo el tiempo, en el subtest
  `budgets propios`.

**Uno era VACUO** — rojo del compilador, no del test: **T14**. `errors` se usa en **una sola
línea** de `turno.go`, así que borrar el bloque `if errors.Is(ctx.Err(), context.Canceled)` deja el
import huérfano. El reemplazo ahora **cambia el centinela** en vez de borrar el bloque
(`context.DeadlineExceeded`): los dos imports siguen usados y el invariante se rompe igual, porque
un cliente que se va produce `Canceled`.

**Y uno medía A CARA O CRUZ, que es el más instructivo de los siete: T21.** Este test ya venía
corregido de antes de la batería por SITIO EQUIVOCADO —su primera versión sólo esperaba que
`bombearTurno` volviera tras el cancel, y eso es verde **pase lo que pase** con las otras dos
goroutines, porque el bombeo tiene su propio `case <-ctx.Done()` y vuelve igual—, así que ahora
**cuenta las goroutines** antes y después, espera a que la cuenta baje y volca el dump de stacks al
fallar, que es lo que dice en qué línea quedó colgada. Con eso ya medía el invariante. Lo que no
medía era **siempre**: el test pasó de
`ROJO ok` en una vuelta a `VERDE MALO` en la siguiente **sin que cambiara una línea del código bajo
prueba**, y eso es la firma de una carrera, no de un test que mide otra cosa. El doble volvía por su
**propio** `ctx.Done()`: al cancelar, el motor y `bombearTurno` despertaban juntos, y si el motor
llegaba primero a `hecho <- err`, el `select` del bombeo veía **las dos** ramas listas (`hecho` y
`ctx.Done()`) y elegía al azar — la mitad de las veces drenaba `hecho`, no quedaba fuga, y el test
pasaba **con el sabotaje puesto**. Flaky **hacia el verde**, que es la dirección en que un test no
avisa nunca.

El arreglo no es esperar más ni reintentar: es volver **determinista el orden de los dos hechos**.
El motor ya no mira el ctx —espera a que lo suelte el TEST— y el test lo suelta **después** de ver
volver al bombeo, así que el motor intenta entregar su resultado cuando ya no hay nadie del otro
lado. Ésa es la única situación en la que el buffer de `hecho` hace diferencia: con buffer 1 el
envío entra y la goroutine termina; sin buffer queda bloqueada para siempre. Sin azar. **Verificado
corriendo el sabotaje cinco veces seguidas: 5/5 ROJO.**

La regla que queda: **un test de concurrencia cuyo veredicto depende de quién gane una carrera
reporta el defecto la mitad de las veces.** La partida del handler tiene que ser *precondición* del
retorno del motor, no una competidora suya — y si el que ordena los dos hechos no es el test, no los
ordena nadie.

## Invariantes del cableado (F5)

Los dos motores, la selección y el endpoint están medidos con 60 sabotajes, y ninguno de ellos mide
**si alguien lo llama**. Antes de F5, `motor.Desde` y `motor.DescribirMotor` tenían cero callers de
producción: el paquete existía, compilaba, pasaba sus tests, y `POST /api/stream` contestaba 405
para siempre porque `s.motor` nunca se poblaba. Es el modo de fallo que este repo ya tiene
catalogado —*se construye y no se enciende*— y el peor de todos para un test: nada se pone rojo,
porque no hay nada roto; lo que falta es una línea que nadie escribió.

Las guardas leen el **AST** de `main.go`, no su texto. El mismo día que se escribió esto, insertar
`WithMotor` en la lista de options partió el `de` de la guarda de `WithVersion`, que anclaba en la
option de al lado: su verde pasó a significar «no lo apliqué». Preguntarle al árbol sintáctico por
la función y por los argumentos de la llamada no se pudre cuando el vecino cambia.

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| W1 | El argumento de `WithMotor` es el motor **construido**, no un nil. `WithMotor` ignora el nil a propósito, así que `WithMotor(nil)` cablea la option, deja su línea en la lista y no enciende nada. | `TestSoloServeCableaElMotorDeTurnos` | ROJO |
| W2 | El error de `motor.Desde` es **fatal**, y se chequea en la sentencia **siguiente**. Entre medio hay veinte llamadas que reusan `err`: un chequeo que llega tarde no es el chequeo de esto. | `TestServeNoArrancaSiElMotorEstaMalConfigurado` | ROJO |
| W3 | `WithMotor` aparece en **una sola** construcción, y es la de `runServe`. | `TestSoloServeCableaElMotorDeTurnos` | ROJO |
| W4 | La línea de `DescribirMotor` sale **siempre**, también con el motor apagado — que es justo el caso en que el 405 sorprende. Colgarla de un `if` la hace desaparecer en el único caso que la necesita. | `TestServeDiceQueMotorQuedoAunqueEsteApagado` | ROJO |
| W5 | `runDaemon` **no llama** a `motor.Desde` en ningún lado. Un `provider: api` sin credencial es fatal al construir, así que un daemon que lo construyera le voltearía el arranque a cada sesión del agente por un servicio que un stdio no tiene por dónde servir. W3 no lo ve: mira los **argumentos** de `NewMcpServer`, y un motor construido y no cableado le pasa por debajo. | `TestElDaemonNoConstruyeElMotorDeTurnos` | ROJO |

**Lo que el compilador ya custodia no hace falta sabotearlo, y confundirlo cuesta un verde falso.**
`motorDeTurnos` tiene un solo consumidor, así que *borrar* `mcp.WithMotor(motorDeTurnos)` deja la
variable huérfana y el paquete no compila (`declared and not used: motorDeTurnos`). Un sabotaje así
pone en rojo a **todas** las pruebas del paquete: su rojo no dice nada sobre la guarda. Era la
primera versión de W1, y se cambió por el ataque al argumento, que es la mitad del invariante que el
compilador **no** ve.

⚠️ **`arnes -overlay` no les puede dar veredicto a estos cinco**, y un verde ahí no es un verde. Ese
modo reemplaza archivos para el *build* de Go, y estas guardas no corren `main.go`: lo **parsean del
disco**, así que ven el archivo sano. Los cinco salieron «candidato a rojo falso» y, en la misma
corrida, la posición 182 —`identidad_servicios_unix_test.go:44`, del repo y ajena a esto— salió
igual: es la **clase** de prueba, no estas guardas. El veredicto sale aplicando al disco: **ROJO
5/5**, cada sabotaje tumbando su prueba y con una prueba **testigo** del mismo paquete verde en los
cinco, que es lo que separa un rojo de la guarda de un rojo del paquete.

## Invariantes del MCP espejo (F4)

El CLI **no toma tools por bandera**: la única puerta es MCP. Así que para que el motor gratis
sirva un turno real —y todos los reales traen tools, porque el transcript de pi las plega en sus
mensajes `system`— hay que levantarle al hijo un servidor MCP efímero con las tools del cliente
adentro. Lo que hace que eso quepa en un archivo son tres banderas del binario local, medidas con
`--help`: `--mcp-config` acepta «JSON files **or strings**» (el esquema de las tools lo sirve el
espejo por HTTP y nunca toca el disco; el config sí va a un archivo, porque lleva la credencial y
el argv lo lee cualquiera),
`--strict-mcp-config` ignora cualquier otra configuración MCP (el hijo no hereda el `.mcp.json` del
proyecto, que es lo que haría a Musubi recursar sobre sí mismo), y `mcp add -t` lista `http` (el
espejo vive dentro de este proceso).

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| M1 | El esquema sale con el nombre de **MCP** (`inputSchema`), no con el del request (`input_schema`). No falla ruidosamente: el hijo toma la tool con el esquema ausente y el modelo la llama vacía. | `TestLasToolsDelPedidoSeTraducenAlFormatoMCP` | ROJO |
| M2 | Una tool sin parámetros lleva un esquema vacío **explícito**. `inputSchema` es obligatorio en MCP: omitirlo hace que el hijo rechace el servidor entero, así que una sola tool sin argumentos dejaría al turno sin ninguna. | `TestUnaToolSinParametrosLlevaUnEsquemaVacioYNoSeOmite` | ROJO |
| M3 | El tope de 64 caracteres se mide **con el prefijo que pone el CLI**. Medirlo sobre el nombre del cliente deja pasar nombres que el modelo rechaza, y el error del otro lado hablaría de un nombre que el cliente nunca escribió. | `TestElTopeDelNombreCuentaElPrefijoQuePoneElCLI` | ROJO |
| M4 | Un pedido sin tools **no abre espejo**. | `TestSinToolsNoHayEspejo` | ROJO (y 12 caídas colaterales, medidas con overlay el 2026-10-29 — ver abajo) |
| M5 | El listener está en **loopback**. Lo que hay detrás son las definiciones de tools del cliente: en todas las interfaces, se publican en la red. | `TestElEspejoEscuchaSoloEnLoopbackYConUnaRutaSecreta` | ROJO |
| M6 | El token de la ruta es **aleatorio** (16 bytes de `crypto/rand`). Un token predecible es lo mismo que no tener token, y en loopback lo alcanza cualquier proceso de la máquina. | `TestDosEspejosNoCompartenPuertoNiToken` | ROJO |
| M7 | **La ruta es la credencial**: cualquier otra da 404. | `TestLaRutaEquivocadaDa404SeaCualSeaElMetodo` | ROJO |
| M8 | El 404 sale **antes** de mirar el método, para que un sondeo no pueda distinguir «ruta mal» de «método mal» y usar la diferencia para encontrar la buena. | `TestLaRutaEquivocadaDa404SeaCualSeaElMetodo` | ROJO |
| M9 | Una **notificación** (sin `id`) no se contesta. `notifications/initialized` llega así, y responderle es un error de protocolo que algunos clientes reportan como servidor roto: un turno sin tools y sin motivo visible. | `TestUnaNotificacionNoSeContesta` | ROJO |
| M10 | El `id` vuelve **con su tipo**. El spec deja que sea número o string; re-tipearlo devuelve un id que no es el que mandó el hijo, y la respuesta queda huérfana. | `TestElIdVuelveConSuTipoOriginal` | ROJO (+2) |
| M11 | El camino imposible —un `tools/call` que llega al espejo, que no ejecuta nada— contesta con el **`-32603` de JSON-RPC** y no con un código inventado: el hijo distingue «error interno del servidor» de «método que no existe», y con el segundo reintentaría. | `TestUnToolsCallContestaErrorYQuedaContado` | ROJO |
| M12 | El config declara el transporte **`http`**. Con `stdio` el espejo tendría que ser un segundo binario. | `TestElConfigMCPDeclaraElEspejoPorHTTP` | ROJO |
| M13 | La des-prefijación toca los **dos** eventos que llevan nombre de tool. Olvidar uno es peor que no hacer nada: el bloque se abre con un nombre y se cierra con otro. | `TestElPrefijoDelCLISeDeshaceEnLosDosEventos` | ROJO |
| M14 | El envoltorio **copia** el `ToolCall` en vez de escribir adentro del que le prestaron. Hoy mutarlo sería inocuo; es el atajo que funciona hasta que alguien cachea el evento del otro lado, y entonces el defecto no se ve en este archivo. | `TestElEnvoltorioNoMutaElToolCallQueLePrestaron` | ROJO |
| M15 | `--strict-mcp-config` va en el argv. Sin él el hijo carga el `.mcp.json` del proyecto, y eso **está medido**: las sesiones que dejó el hijo en la medición en vivo traen el índice de auto-memoria del dueño y 135 prefijos de conectores que el pedido nunca declaró. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` | ROJO |
| M16 | `--allowedTools mcp__pi` va en el argv. Sin él cada tool del cliente se frena pidiendo un permiso que en headless nadie va a contestar. **Medido el 2026-10-29**: sin la bandera, en `-p` no interactivo la tool se **DENIEGA** —el espejo no recibe un solo `tools/call`— y el turno vuelve con el modelo diciendo que le falta un permiso (`lanzador.go:430-436`). ⚠️ **No muere por timeout**: eso era la hipótesis previa y había quedado pegada a la palabra «Medido», que es exactamente cómo una conjetura se lee después como un hecho. | `TestElArgvDeclaraElEspejoCuandoElPedidoTraeTools` | ROJO |
| M17 | `--tools ""` va en el argv. Sin él el hijo conserva las tools nativas del CLI —entre ellas el disco del host— que el cliente remoto no declaró y no sabe ejecutar. Es el que resolvió el punto que el plan le asignaba a `--disallowedTools`. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` | ROJO |
| M18 | `--no-session-persistence` va en el argv. La sesión que el motor fabrica para cada turno es un medio de transporte, no historia del usuario: sin la bandera queda escrita en el disco de quien corre el servidor. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` | ROJO |
| M19 | **Sin espejo, el argv declara un mapa MCP vacío** (`{"mcpServers":{}}`). `--strict-mcp-config` sin un `--mcp-config` que la acompañe queda suelta, y el hijo vuelve a heredar la configuración del host por el camino que M15 cierra. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` | ROJO |
| M20 | Las tools del espejo llevan el `_meta` de **always-load**. Sin él el CLI se las entrega al modelo DIFERIDAS: medido en vivo, el primer `tool_use` del turno pasaba a ser `ToolSearch` —una tool del CLI que el cliente no declaró— en vez de la del cliente. | `TestLasToolsDelEspejoSeDeclaranAlwaysLoad` | ROJO |
| M21 | La credencial del espejo viaja por **archivo**, no inline en el argv. `--mcp-config` acepta las dos formas, y la inline publica el token en la línea de comandos del hijo, que cualquier proceso de la máquina lee. | `TestElArgvDeclaraElEspejoCuandoElPedidoTraeTools` | ROJO (+1, legítima) |
| M22 | El archivo de config **se borra al cerrar el turno**. Es la credencial del espejo: sobrevivirle al turno la deja en el temporal del host hasta que alguien la encuentre. | `TestLaConfigDelEspejoViveEnUnArchivoPropioYSeBorraAlCerrar` | ROJO |
| M23 | Y nace con permisos **`0600`**. En el temporal compartido, legible por todos es lo mismo que no tener token. | `TestLaConfigDelEspejoViveEnUnArchivoPropioYSeBorraAlCerrar` | ROJO |
| M24 | Un nombre de tool fuera de `^[a-zA-Z0-9_-]$` **se rechaza**, no se normaliza. Medido: el CLI reemplaza cada carácter ilegal por `_` **en silencio**, el `init` muestra el servidor `connected`, y el cliente recibe un `tool_use` para un nombre que nunca declaró. Peor: `tool mala` y `tool/mala` colapsan las dos en `tool_mala` y el CLI deduplica — de tres tools el modelo vio dos. | `TestUnNombreFueraDelCharsetNoPasa` | ROJO |
| M25 | El cuerpo que manda el hijo entra con **tope de 1 MiB**. Es un proceso local, pero el que decide cuánto se parsea no puede ser el que manda. | `TestUnCuerpoGiganteNoSeParsea` | ROJO |
| M26 | El envoltorio que des-prefija **se instala** en el turno con tools. Sin él el cliente recibe el nombre que inventó el CLI y su agent loop se corta con una «tool desconocida» que no explica nada. | `TestElTurnoConToolsDelClienteLeSacaElPrefijoDelCLI` | ROJO |
| M27 | Un corte tardío **deja rastro** por stderr. El turno ya contestó, así que el cliente no se enteraría nunca de que el hijo siguió intentando ejecutar tools después del corte. | `TestElLectorDelContadorDelEspejoAvisaCuandoElHijoEjecuta` | ROJO |
| M28 | El `tools/call` se cuenta **antes** de la puerta de las notificaciones. El `id` lo pone quien intenta: no puede ser él el que decide si su intento queda registrado, u omitir un campo apaga el aviso. | `TestUnToolsCallSinIdSeCuentaIgual` | ROJO |
| M29 | Los `tools/list` servidos **se cuentan**, y es el fallo CONTRARIO al de M28: un turno con tools abre el espejo justamente para que el hijo lea el roster. Sin el contador, un `--mcp-config` que dejó de llegar se ve como un modelo que se volvió vago. | `TestElContadorDeListadosSoloSubeConUnToolsList` | ROJO |
| M30 | Y ese cero **avisa**. Es el único síntoma de un hijo que contestó sin ver una sola tool del cliente, que desde afuera es indistinguible de un modelo que miró sus tools y eligió no usar ninguna. | `TestElLectorAvisaCuandoElHijoNuncaVioLasTools` | ROJO |
| M31 | El aviso cuelga de `esp != nil`, no del contador a secas: un turno sin tools no abre espejo, y ahí cero listados es lo correcto. Sin la guarda grita en todos los turnos de texto y se vuelve ruido que nadie lee. | `TestElLectorAvisaCuandoElHijoNuncaVioLasTools` | ROJO |
| M32 | El listener tiene **los cuatro** timeouts. Había uno solo —`ReadHeaderTimeout`— con un comentario que decía que acotaba «la única forma de colgar esto»: era falso, dejaba sin plazo el cuerpo del request y la espera entre requests keep-alive. | `TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno` | ROJO (+1) |
| M33 | El `IdleTimeout` **le gana al plazo del turno** y no comparte escala con el `ReadTimeout`. Son plazos de cosas distintas —un request a medio mandar y una conexión en silencio—, y el silencio entre dos `tools/call` es el modelo pensando: con la escala del read, el listener le corta el keep-alive al hijo y el hijo lo ve como un servidor MCP caído. ⚠️ Poner sólo `ReadTimeout` habría sido **peor que no tocar nada**: con `IdleTimeout` en cero, net/http usa el valor del read para la espera entre requests. | `TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno` | ROJO (+1) |
| M34 | Un `Shutdown` que **expira** no cerró nada, y hay que seguirlo con `Close()`. Shutdown cierra los listeners y las conexiones ociosas, pero a las que están a mitad de un request sólo las ESPERA: al vencer el contexto devuelve su error y las deja vivas con su goroutine y su descriptor, dentro de un `musubi serve` que no se muere nunca. Y **no hace falta la credencial para provocarlo**: el plazo de lectura corre antes de que el handler mire la ruta. | `TestUnShutdownQueExpiraIgualCortaLaConexion` | ROJO |
| M35 | La goroutine que **sirve** el listener existe. Sin ella el espejo acepta por backlog del kernel y no contesta nunca: el hijo se queda esperando un servidor MCP que está abierto y mudo. | `TestUnShutdownQueExpiraIgualCortaLaConexion` | ROJO |
| M36 | Y sirve **este** espejo (`Handler: e`), no el mux global. Con el handler en nil, net/http cae en el `DefaultServeMux` y el espejo devuelve 404 a todo detrás de una ruta que parece correcta. | `TestUnShutdownQueExpiraIgualCortaLaConexion` | ROJO |
| M37 | El margen del minuto es **load-bearing**. El `+ margenDelIdle` existe para que el techo del turno gane SIEMPRE, pero la aserción vieja sólo pedía `IdleTimeout > DemoraPorDefecto`: cambiarlo por `+ time.Nanosecond` dejaba la suite entera en verde. | `TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno` | ROJO |
| M38 | El idle cuelga del plazo **real** del turno, no de la constante. Con el espejo colgado de `DemoraPorDefecto` y el turno de un `turn_timeout_seconds` que no está acotado, un plazo configurado por encima de los once minutos hacía que el idle cortara **antes** que el reloj del lanzador — justo lo que el comentario del código prometía que no pasa. | `TestElIdleDelEspejoCuelgaDelPlazoRealDelTurno` | ROJO |
| M39 | El lanzador **resuelve** el plazo en vez de quedarse con el crudo del yaml. Sin la llamada a `plazoDelTurno`, un `Espera` negativo llega como plazo ya vencido y el turno muere dentro de `cmd.Start()` sin lanzar el proceso: 0 eventos y un `context deadline exceeded` que no nombra la causa. | `TestUnTopeNegativoNoMataElTurnoEnElActo` | ROJO |
| M40 | Lo mismo en la fábrica, que además lo **anuncia**: sin la llamada, un `turn_timeout_seconds: -1` se describe como «tope por turno -1s» y la pista del arranque le miente al operador sobre el plazo que de verdad rige. | `TestElDiagnosticoInformaElTopeQueRigeYNoElDelYaml` | ROJO |
| M41 | Y en el cliente de la API, donde el negativo **viaja**: con la igualdad a cero, un `MaxTokens` de -1 se serializa tal cual en el request y la pasarela contesta 400; con `MinInt64`, viaja el mínimo de la máquina. | `TestUnMaxTokensNegativoNoViajaALaAPI` | ROJO |
| M42 | El resolvedor cura **todo** el dominio no positivo, no sólo el cero. Y el aislamiento se midió: con un `== 0 \|\| == -1s \|\| == -5s` —los tres puntos exactos que los tests viejos usaban— los dos custodios del negativo quedan VERDES y un `-2` sigue naciendo vencido. Es lo que distingue un dominio de un punto. | `TestElPlazoDelTurnoCuraTodoElDominioNoPositivo` | ROJO |
| M43 | El **valor** del defecto está anclado por igualdad. Las 30 menciones de `DemoraPorDefecto` en el paquete la usan todas como referencia y ninguna la compara contra un valor, así que bajarla a `1 * time.Second` dejaba el paquete entero en verde —medido: el único `--- FAIL` es este custodio— con todos los turnos largos muriendo por reloj. | `TestElDefectoDelPlazoSonDiezMinutos` | ROJO |

**43/43**, medidos al disco con un script que **lee las directivas `arnes:` de los propios archivos
de test**: un script que repitiera los strings a mano podría estar midiendo algo distinto de lo que
quedó escrito, y entonces el veredicto sería de otra cosa.

⚠️ **Y hay que declarar QUÉ cuenta, porque el script que esta frase cita devuelve otro número.**
`internal/motor` tiene hoy **81** directivas `arnes: de=`, no 43: son las **43** de la serie `M`
—41 contra el espejo y su lanzador (28 directivas en `espejo_test.go` y 13 en `lanzador_test.go`),
más M40 en la fábrica y M41 en el cliente de la API— más **29** de la serie `E` (el traductor de
entrada, en `entrada_test.go`) y **9** de la serie `C` (la fábrica, en `factory_test.go`), que son
de otras fases. Contar las directivas sin separar las series da 81 y parece refutar esta tabla: no
la refuta, cuenta otra cosa. Esta tabla es la serie `M`, y 43 + 29 + 9 = 81 cierra la aritmética.

Y el mismo script verifica que las etiquetas vayan de `M1` a `M43` **sin huecos ni repetidos**, que
es lo que esta tabla no puede probar de sí misma: un hueco significaría una directiva borrada sin
sacar su fila —la fila queda afirmando una cobertura que no existe— y un repetido, dos sabotajes
peleándose una etiqueta, con la fila describiendo a uno y el arnés corriendo el otro.

🔑 **M35 y M36 existen porque el panel preguntó por el listener, y tenía razón.** Todos los demás
tests del espejo llaman `ServeHTTP` a mano con un `httptest.Recorder` —que es lo correcto para
probar el protocolo— y por eso el `go Serve(ln)` y el `Handler: e` podían desaparecer sin que una
sola prueba se enterara. La cobertura la da una aserción de tres líneas en el único test de
`espejo_test.go` que disca un socket de verdad. ⚠️ Acá decía «el único test» a secas y era falso:
`llamarAlEspejo` (`lanzador_test.go:97`) hace dos `http.Post` REALES contra la URL del espejo desde
el proceso hijo, así que los tests del lanzador que pasan tools también atraviesan el listener. Y el sabotaje enseñó algo de arriba: **los dos ponen rojo también al aviso de
M30**, porque un hijo real no puede pedirle `tools/list` a un espejo que no sirve. Ese aviso no era
decoración: detecta la falla del listener en producción, que es donde no hay test.

🔴 **M4 enseñó algo que no se buscaba: ese invariante no lo custodia su test.** Sacar el corte que
evita abrir un espejo cuando el pedido no trae tools **rompe doce pruebas** del paquete —trece
`--- FAIL` de nivel superior contando a `TestSinToolsNoHayEspejo`, que no es colateral sino el
custodio de este mismo sabotaje y cuyo trabajo es morder—, porque un turno sin tools es el caso
normal de cualquier turno de texto: sin el corte, `abrirEspejo` le pasa un `nil` al parser de tools
y devuelve error antes de lanzar el proceso. Que sean doce es el hallazgo, no burocracia.

⚠️ Y esas doce **no llevan `colision_ok`**: van enumeradas en prosa en el comentario de
`TestSinToolsNoHayEspejo`, en `internal/motor/espejo_test.go`. La cita va por **nombre** del test y
no por número de línea: esta spec citaba `espejo_test.go:282-284` y ése es exactamente el desfase
que ya pasó una vez acá. Y decía «los diez `colision_ok` de ese sabotaje», que eran dos errores en
una frase — el número y el mecanismo. `colision_ok` responde por un ANCLA que este sabotaje le
rompe a otro; las doce caen por **comportamiento**, con su ancla intacta, así que declararlas sería
contestarle a un aviso que la herramienta nunca emitió, y el censo del arnés denuncia esa respuesta
como RANCIA.

**Y dos testigos quedaron verdes a propósito, que es lo que separa un sabotaje que mide de uno
vacuo.** M14 no rompió el test de M13, y M6 no rompió el de M5: cada par mide cosas distintas. Si un
sabotaje hubiera roto a los dos tests de su par, uno de ellos sería de más.

## Verificación de punta a punta

Tres tests detrás de `MOTOR_E2E=1`, porque consumen cuota. Son lo único que verifica que las
banderas del argv sigan existiendo en el binario y que el formato del stream no haya cambiado: el
stream de `claude -p` es una interfaz de CLI, no una API con contrato. **Cuando cambien las banderas
incondicionales o la forma del stream, estos tests son los que avisan.**

⚠️ **Y SÓLO ESO.** La frase decía «cuando el CLI cambie» a secas, y así prometía una alarma que no
existe: un grep de `Tools`, `espejo`, `allowedTools` o `mcp__` sobre `integracion_test.go` devuelve
**cero**. Nada repetible cubre `--allowedTools`, el prefijo `mcp__pi__`, el `_meta` de always-load
ni el renombre por el charset — es decir, justo lo que se descubrió midiendo en vivo y lo que más
chance tiene de cambiar sin aviso. Esa mitad vive hoy en mediciones manuales fechadas, no en un
test, y el día que el CLI la mueva nadie se va a enterar por acá.

| Test | Qué pincha | Corrida real |
|---|---|---|
| `TestE2EUnTurnoRealSeSirveYCorta` | un turno se sirve y se corta | `4.405s · 5 eventos · [start text_start text_delta text_end done]`, texto `"París"` |
| `TestE2ELaHistoriaLlegaCompletaAlModelo` | el dato que está **sólo en el prefijo** llega | contesta `"7391"`; **sin la partición contesta `"Listo."`** |
| `TestE2EElToolResultLlegaIntacto` | el `tool_result` del medio del transcript llega | contesta `"Zaragoza"`; **sin la partición contesta `"Desconocida"`**, que es el síntoma exacto del placeholder `[Tool result missing due to internal error]` |

Las dos contrapruebas son lo que distingue «anduvo» de «anduvo por esto».

## Lo que falta especificar

**F4 ya está arriba, con sus 36 invariantes.** Falta **F6** (el cableado de ThinkRail), que se
agrega a este documento cuando se cierre, con el mismo formato: invariante · test · sabotaje.
⛔ F6 no se toca desde esta terminal: el cuerpo se trabaja desde su propia sesión.

Y F5 **no era un subcomando**, que es como lo nombraba el plan: no hay un `musubi motor` nuevo. El
motor se construye y se cablea dentro de `musubi serve`, que es el único comando que sirve el mux
HTTP donde vive la ruta.
