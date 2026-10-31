# Tareas — `motor`

## F1 · El traductor — CERRADO

- [x] Los 12 eventos del protocolo, con los nombres verificados contra el fuente de pi
      (`proxy.ts`), no contra la memoria: el `default` de `processProxyEvent` sólo hace
      `console.warn`, así que un `type` mal escrito pierde el evento **en silencio**.
- [x] El estado del turno: qué bloque está abierto en qué índice, el `usage` parcial, el cierre.
- [x] El corte en el primer `message_delta`.
- [x] 13 invariantes con test y sabotaje (ver `spec.md`).
- [x] Verificado contra el cliente REAL: 15 corridas de los dos fixtures, 0 throws y 0
      `console.warn`.

## F2 · El Lanzador (`claude -p`) — CERRADO

- [x] Lanzar, alimentar, bombear, cortar. Nueve caminos de salida.
- [x] El entorno del hijo no hereda la sesión (`CLAUDECODE`, `CLAUDE_CODE_`, `MUSUBI_`) ni la
      API key, salvo opt-in explícito.
- [x] El stderr acotado a los últimos 4 KB — es donde aparece el mensaje útil.
- [x] **La partición prefijo/cola**, que es el nudo del paquete. Verificada contra el binario
      real CON CONTRAPRUEBA: con la partición el modelo contesta el dato que sólo estaba en el
      prefijo; sin ella, no.
- [x] El `defer` de cosecha contra la fuga de goroutine por panic del consumidor, con un test
      que cuenta goroutines de `os/exec` en el stack dump.
- [x] 14 casos de sabotaje con el veredicto esperado, **todos por el test** y ninguno por
      compilación.
- [ ] Darle un test a `WaitDelay` con los guiones `nieto`/`nieto-sin-stderr`/`duerme`, **o
      quitarla**. Hoy está marcada en el comentario como guarda definida y desconectada a
      propósito, que es el gotcha `gotcha-guarda-definida-y-desconectada` de este repo.
- [ ] R1, sin solución conocida: si el prefijo termina en un turno de usuario, el CLI inyecta un
      `assistant: "No response requested."` sintético.

## F2b · El ClienteAPI (`/v1/messages`) — CERRADO

Entró porque el dueño eligió **los dos motores detrás de una interfaz**, con la selección por
configuración.

- [x] El request completo: `stream:true`, `anthropic-version`, `anthropic-beta` en una cabecera
      con comas, las tools crudas del cliente, el bloque `thinking`.
- [x] La validación de lo que la API rechazaría con un 400 que cuesta más diagnosticar que
      prevenir — **antes** de gastar el request.
- [x] El parser de SSE con las rarezas del formato: `data:` multilínea, comentarios/latidos,
      `[DONE]` de pasarela, y el despacho final sin línea vacía.
- [x] El `Timeout` del cliente HTTP en CERO con las esperas previas al stream acotadas.
- [x] El cierre del cuerpo como corte real del turno, con un test que lo mide del lado del
      servidor.
- [x] El error nunca filtra la credencial, con un test que lo verifica.
- [x] 20 casos de sabotaje (ver `spec.md`).

## F3 · El endpoint — CERRADO

- [x] `POST /api/stream` reusando `autenticarPersona` y el patrón SSE de
      `internal/mcp/http.go:497-525`. La ruta **no se eligió**: el cliente la tiene hardcodeada
      (`proxy.ts:147`) y lo único configurable es el host, así que con `/api/stream` ya tomada por
      el feed del panel, la salida fue despachar por MÉTODO.
- [x] 🔴 `rc.SetWriteDeadline(time.Time{})`. **Sin eso el stream muere a los 90 s** por el
      `WriteTimeout` del server, a mitad de un turno largo.
- [x] Cabeceras: `text/event-stream`, `no-store`, `keep-alive`, `X-Accel-Buffering: no`.
- [x] 🔴 **Y un `rc.Flush()` detrás del `WriteHeader`, que el plan no tenía.** `WriteHeader` sólo
      pone las cabeceras en el buffer del server; el cliente hace `await fetch(...)` y recién
      después mira `response.ok` (`proxy.ts:154`), así que sin el flush queda bloqueado todo lo
      que tarde el primer token. Medido: lo único que destrababa el buffer era el primer latido,
      **a los 20 s**.
- [x] Latido cada 20 s como `": latido\n\n"`, y corte por `r.Context().Done()`. El intervalo entra
      por PARÁMETRO a `bombearTurno`: una variable de paquete que el test bajara serviría hasta el
      día que dos tests corran en paralelo y se pisen el valor.
- [x] Un `error` propio emitido en el cierre anómalo: el cliente no tiene otra forma de saberlo.
- [x] El traductor de ENTRADA (`PedidoDesdeProxy`), que tampoco estaba en el plan: el transcript
      de pi **no es** el de la Messages API. Cinco renombres silenciosos y tres reglas que la API
      exige; ninguno falla ruidosamente.
- [x] **La selección de motor por configuración**, que es lo que cierra la decisión de tener dos.
      Nace apagada: los dos facturan distinto y el stream que sale es el mismo, así que elegir por
      omisión sería elegir cómo se gasta la plata de otro.
- [x] 18 tests verdes (`ok musubi/internal/mcp 3.282s`) y cobertura sin una sola función nueva en
      cero: `internal/motor` 88,9 %, `turno.go` 83,4 %.
- [x] **Dos defectos de TEST corregidos antes de saboteario**, los dos del patrón SITIO EQUIVOCADO:
      el del latido *pasaba* tardando 20,21 s en vez de fallar (ahora lleva reloj), y el de la fuga
      de goroutines medía que `bombearTurno` volviera — y vuelve por su `case <-ctx.Done()` pase lo
      que pase, así que era verde con la fuga intacta (ahora cuenta goroutines).
- [x] La batería de sabotaje de las 60 anclas (E1–E29, C1–C9, T1–T22) cierra en **60/60**, con base
      verde (18 tests en `./internal/mcp/`, 31 en `./internal/motor/`) y restauración verde en los
      dos paquetes. 53 dieron el veredicto esperado a la primera, y los otros 7 eran hallazgos, no
      ruido. Tres eran huecos de
      test reales (T22 no tenía test, T4 afirmaba sólo «no es 405» y un GET desviado da **400**, y
      E26 medía la mitad de su propio nombre); dos eran sabotajes mal apuntados, donde el verde era
      **correcto** (E8 atacaba defensa en profundidad —el filtro de la entrada ya descarta el
      vacío, y el del final protege una *sección* en `""`—, y E29 apuntaba a un test cuyo fixture
      no trae `thinkingBudgets`); y uno era VACUO (T14 dejaba el import `errors` huérfano, porque
      se usa en UNA sola línea de `turno.go`, así que el rojo lo firmaba el compilador).
- [x] 🔴 **El arnés falló dos veces por su propio instrumento, y las dos veces acusó al código.**
      Primero su `-timeout 180s` era más chico que los 275 s de `./internal/mcp/` entero, y el
      mensaje decía «no arranca en verde» (el mismo bug estaba en DOS sitios: la base verde y el
      chequeo de restauración). Arreglado midiendo sólo los tests que la batería observa. Después
      mi propio arreglo de T4 colgó el paquete 45 s: hacía `io.ReadAll` del cuerpo —para el
      *mensaje de error*— sobre un feed SSE que por diseño no termina nunca.
- [x] 🔴 **Y el séptimo hallazgo fue un test que medía A CARA O CRUZ: T21.** Pasó de `ROJO ok` a
      `VERDE MALO` sin que cambiara una línea del código bajo prueba, que es la firma de una carrera
      y no de un test mal apuntado. El doble volvía por su PROPIO `ctx.Done()`, así que al cancelar
      competía con `bombearTurno`: si llegaba primero a `hecho <- err`, el `select` del bombeo veía
      las DOS ramas listas y elegía al azar — la mitad de las veces drenaba `hecho` y no quedaba
      fuga. Flaky **hacia el verde**. Arreglado volviendo determinista el orden: el motor no mira el
      ctx, lo suelta el test, y el test lo suelta DESPUÉS de ver volver al bombeo. Verificado 5/5
      ROJO.
- [x] Pegar en `spec.md` las tres tablas con los veredictos medidos (E1–E29, C1–C9, T1–T22), antes
      de `## Verificación de punta a punta`, y sacar F3 de `## Lo que falta especificar`.

## F4 · El MCP espejo — CERRADO (la medición en vivo se corrió el 2026-10-29, con el OK del dueño)

- [x] **Las tres banderas, medidas contra el binario local con `--help`** (CLI 2.1.272, gratis):
      `--mcp-config` acepta «JSON files **or strings**» ⇒ el esquema de las tools lo sirve el espejo
      por HTTP y no toca el disco, y el config va a un **archivo** y no inline, porque lleva la
      credencial y el argv del hijo lo lee cualquiera de la máquina (ver `escribirConfig`);
      `--strict-mcp-config` («ignoring all other MCP configurations») ⇒ el hijo no hereda el
      `.mcp.json` del proyecto y no recursa sobre Musubi; `mcp add -t` lista `http` ⇒ el espejo
      vive dentro del proceso. Ver la tabla de `design.md`.
- [x] 🔴 **Una corrección de lectura casi mata la fase.** El párrafo de la ayuda que menciona
      `--mcp-config` junto a los directorios de `CLAUDE.md` es de **`--bare`**, no de
      `--setting-sources`. Si fuera de `--setting-sources`, el `--setting-sources ""` que el motor
      ya pasa habría apagado el `--mcp-config` y esto sería imposible. Se verificó con
      `grep -B 12` sobre la ayuda antes de escribir una línea.
- [x] `internal/motor/espejo.go`: servidor MCP efímero de un turno, en loopback, con la ruta como
      credencial (16 bytes de `crypto/rand`) y el 404 **antes** del chequeo de método.
- [x] **Vive en `internal/motor` y no en `internal/mcp`** aunque ahí ya haya un servidor MCP
      entero: ese paquete **importa** a este, así que colgarlo ahí sería un ciclo. El precio es un
      JSON-RPC mínimo; el premio es que el ciclo de vida del espejo sea el del turno.
- [x] El nombre `pi` es **aritmética**: `mcp__pi__` cuesta 9 de los 64 caracteres del tope, y el
      tope se valida al abrir, con el prefijo puesto.
- [x] La des-prefijación **no va en el traductor** —el camino que parecía obvio—: lo comparten los
      dos motores, el `ClienteAPI` nunca pone ese prefijo y pi declara sus propias tools MCP con
      forma `mcp__srv__tool`. Va envuelta en el `emitir` del `Lanzador`, y toca los **dos** eventos
      que llevan nombre de tool.
- [x] **El contador de `tools/call` tiene un LECTOR**, y eso no es cosmético: un número que nadie
      mira es una guarda definida y desconectada (el patrón ya tiene nombre en este repo). Lo lee
      un `defer` del `Turno` y sale por stderr. Lo que avisa es una **carrera**: `bombear` vuelve
      en cuanto el traductor se da por cerrado y el `message_delta` con `stop_reason: tool_use`
      sale antes de que el agente ejecute nada, así que en el camino normal el hijo nunca llega a
      pedirle al espejo que corra la tool.
- [x] `internal/motor/espejo_test.go` + los dos del argv en `lanzador_test.go`: **16 sabotajes,
      16/16 con el rojo esperado** (M1–M16), medidos al disco con un script que **lee las
      directivas `arnes:` de los propios archivos** — un script que repitiera los strings a mano
      podría estar midiendo algo distinto de lo que quedó escrito.
- [x] 🔴 **M4 enseñó algo que no se buscaba: ese invariante no lo custodia su test.** Sacar el
      corte que evita abrir un espejo cuando el pedido no trae tools tumba **13 tests de nivel
      superior** (medido con `go test -overlay` el 2026-10-29), porque un turno sin tools es el
      caso normal de cualquier turno de texto. Que sean trece es el hallazgo, no burocracia.
- [x] ⚠️ Doce de esos trece son colaterales y **no llevan `colision_ok`**: van en prosa en el
      bloque del sabotaje, arriba de `TestSinToolsNoHayEspejo` —que es el decimotercero y el
      único que muerde como custodio—. `colision_ok` responde por un ANCLA que un sabotaje le
      rompe a otro; estos doce caen por **comportamiento**, con su ancla intacta, así que
      declararlos sería contestar un aviso que nunca se emitió y el censo lo marcaría RANCIO.
- [x] 🔑 **Y el número de esa prosa se vence solo.** Decía «diez» y la medición dio doce: uno
      faltaba desde antes (`TestElLectorAvisaCuandoElHijoNuncaVioLasTools`) y otro lo agregó
      este mismo change-set (`TestUnTopeNegativoNoMataElTurnoEnElActo`, el custodio del tope
      negativo, que corre un turno sin tools). La cita a este bloque va por NOMBRE de test y no
      por número de línea, que es lo que ya se había desfasado una vez.
- [x] 🔴 **Dos testigos quedaron verdes a propósito, y eso es lo que separa un sabotaje que mide
      de uno vacuo.** M14 (mutar el `ToolCall` prestado en vez de copiarlo) **no** rompió el test
      del renombre, y M6 (token fijo) **no** rompió el de la ruta secreta: los pares de tests
      miden cosas distintas, y si un sabotaje los hubiera roto a los dos, uno de ellos sería de
      más.
- [x] `go test ./... -count=1` entero: **35 paquetes ok, 0 FAIL, exit 0** — la puerta que en F5
      destapó el guardián del cerebro.
- [x] ✅ **Los tres puntos que esperaban una medición en vivo se midieron el 2026-10-29** contra el
      CLI 2.1.272, con el OK del dueño porque gasta cuota de su suscripción. Este ítem estuvo
      ABIERTO mientras el código ya tenía las tres respuestas escritas con su fecha, y así quedó la
      contradicción que encontró la revisión (ALCANCE-H4): **el documento era el rancio, no la
      spec.** Lo medido, con el lugar donde vive la evidencia:
    - **El prefijo exacto**: el modelo emite `mcp__<servidor>__<tool>` —en la corrida, un servidor
      de juguete de una sola tool dio `mcp__juguete__sumar_dos_numeros`—, así que con
      `nombreDelEspejo = "pi"` el `tool_use` llega como `mcp__pi__<tool>`
      (`internal/motor/espejo.go:446-451`).
    - **El permiso**: `--allowedTools` hace match **por prefijo de servidor**, así que `mcp__pi`
      autoriza a todas sin enumerarlas; y sin la bandera, en `-p` no interactivo la tool se
      DENIEGA —el espejo no recibe un solo `tools/call`— y el turno se le va al cliente pidiendo un
      permiso que en headless nadie contesta (`internal/motor/lanzador.go:430-436`, invariante M16).
    - **Las tools propias del CLI**: no se mitigaron con `--disallowedTools`, que era el candidato
      del plan, sino con **`--tools ""`** (invariante M17). Y el que sacaba a `ToolSearch` del
      PRIMER `tool_use` era otro: el `_meta` de always-load, sin el cual el CLI le entrega las
      tools del cliente **diferidas** al modelo (M20, `internal/motor/espejo.go:446-456`).
    - Sigue valiendo lo demás: un cliente que necesite su **system prompt entero** va por
      `ClienteAPI` — el CLI le prepende su propia identidad en todas las variantes probadas,
      `--bare` incluida (`internal/motor/lanzador.go:420-428`).
    - ⚠️ Y **no** «su set exacto de tools», que es lo que decía acá: era la mitad que este mismo
      arreglo retractaba, reintroducida al redactarlo. `--tools ""` más el espejo le dan al modelo
      el set exacto del cliente por los **dos** motores, y `internal/motor/motor.go:87-91` lo dice
      con lo medido: la versión vieja de esa frase enrutaba a facturar por API key un turno que la
      suscripción podía servir.

### Endurecimiento del listener y de los contadores (vuelta 2 de la revisión)

- [x] **Los cuatro timeouts del listener**, con el `IdleTimeout` colgado de `DemoraPorDefecto` para
      que el que corte un turno eterno sea el reloj del lanzador y no el transporte (M32, M33).
      ⚠️ La trampa: con `IdleTimeout` en cero, `net/http` usa el valor de `ReadTimeout` para la
      espera entre requests keep-alive, así que **poner sólo el read habría sido peor que no tocar
      nada**.
      🔴 **Y colgarlo de `DemoraPorDefecto` resultó ser el defecto que la vuelta 3 arregló** (M38):
      la constante y el plazo real del turno se despegan en cuanto alguien escribe
      `turn_timeout_seconds`, y con un plazo por encima de los once minutos el listener cortaba
      **antes** que el reloj del lanzador. Hoy cuelga de `espera`, el plazo que rige. Este ítem se
      deja como está porque describe lo que ESTA vuelta entregó —el hallazgo necesitaba que los
      cuatro timeouts existieran para poder verse— y lo corrige la vuelta de abajo, no un borrado.
- [x] **`Close()` después de un `Shutdown` que expira** (M34). Un Shutdown vencido no cerró nada: a
      las conexiones a mitad de request sólo las ESPERA, y al vencer el ctx las deja vivas con su
      goroutine y su descriptor. No hace falta la credencial para provocarlo —el plazo de lectura
      corre antes de que el handler mire la ruta—, así que es seguridad y no prolijidad.
- [x] 🔑 **Medirlo enseñó dónde apretar: hay que colgar el HANDLER, no el parser.** `net/http` emite
      `StateActive` **después** de que `readRequest` devuelve, así que una conexión bloqueada a mitad
      de los headers sigue contando como OCIOSA y Shutdown la cierra sin dudar. El primer intento
      volvió en **704 µs** y lo denunció la guarda del plazo que el test tiene justamente para que un
      verde vacuo no pase por medición. Lo que cuelga al handler: headers completos,
      `Content-Length: 200` y diecisiete bytes de cuerpo.
- [x] **El conteo del `tools/call` sale de detrás de la puerta de las notificaciones** (M28): el `id`
      lo pone quien intenta, y un aviso que se apaga omitiendo un campo no es un aviso.
- [x] **El contador de `tools/list` y su aviso** (M29, M30, M31): es el fallo CONTRARIO, y el que
      pasaba callado. Sin él, un `--mcp-config` que dejó de llegar se ve como un modelo que se volvió
      vago. El aviso cuelga de `esp != nil`, porque en un turno sin tools cero listados es correcto.
- [x] **La rama de error de `crypto/rand.Read` es inalcanzable en este Go y ningún test la puede
      cubrir**, así que queda documentada en el código en vez de buscada como hueco: `go doc` dice
      *«It never returns an error, and always fills b entirely»* y que **crashea el programa
      irrecuperablemente** si el lector del sistema falla. La guarda se conserva igual —falla
      cerrada, cuesta una línea—, pero el modo de falla real es un crash, no un error envolvible.
- [x] ✅ **ALCANCE-H7 cerrado midiendo** (el panel lo había rebajado a INFO su propio autor, y
      valía más que eso): **ninguna prueba marcaba el `go Serve(ln)` ni el `Handler: e`**. Todas
      llaman `ServeHTTP` a mano con un `httptest.Recorder` —que es lo correcto para probar el
      protocolo—, así que el servidor real podía dejar de servir sin que nada se pusiera rojo. Lo
      cubre una aserción de tres líneas en el único test de `espejo_test.go` que disca un socket
      —«el único» a secas era falso: `llamarAlEspejo` hace dos `http.Post` reales desde el proceso
      hijo, así que los tests del lanzador con tools también atraviesan el listener—, y lo miden M35
      (borrar el `Serve` deja al espejo aceptando por backlog y mudo: rojo en 10 s por el
      `SetReadDeadline`) y M36 (`Handler: nil` cae en el `DefaultServeMux` y devuelve 404 a todo).
- [x] 🔑 **Y el sabotaje devolvió un dato de arriba: M35 y M36 ponen rojo también al aviso de M30**,
      porque un hijo real no puede pedirle `tools/list` a un espejo que no sirve. El aviso no era
      decoración: es lo que detecta esta falla **en producción**, que es donde no hay tests.
- [x] 🔴 **M11 se reasignó, y el motivo es una lección de método: mover una línea de producción
      puede secuestrar el ancla de otro sabotaje.** Al mover `e.llamadas.Add(1)`, el `de` de M11
      siguió existiendo —misma indentación— pero apuntando a la línea nueva, así que M11 y M28
      borraban lo mismo: **una sola guarda contada dos veces**. Lo cazó el censo del arnés. M11 pasó
      a atacar el `Code: -32603` del camino imposible, que no tenía quien lo custodiara: se eliminó
      la colisión **y se ganó cobertura**.
- [x] 🔑 **`colision_ok` va en CADA directiva que pisa, no una vez por par.** Medido: con la clave
      sólo en una del par, el aviso de la otra seguía saliendo. Cada directiva responde por el daño
      que hace ELLA. (Admite varios nombres en una clave separados por espacios; repetir la clave la
      deja ROTA, y eso sólo se ve después del `git add`, porque el censo deriva de `git ls-files`.)
- [x] Censo del arnés al disco: **2153 directivas mecanizadas, 0 ROTAS, 0 colisiones**, cobertura
      99,4 % sobre 2166 anclas en 419 archivos (`arnes -validar -paquete ./internal/motor`).

### Los dos plazos del cable (vuelta 3 de la revisión)

- [x] **El margen del minuto era load-bearing y no lo medía nadie** (M37). El comentario declaraba
      que el minuto de más existe «para que el techo del turno gane SIEMPRE», pero la aserción sólo
      pedía `IdleTimeout > DemoraPorDefecto`: cambiar `+ time.Minute` por `+ time.Nanosecond`
      dejaba **la suite entera en verde**. La cura no es la aserción sino que cada cota lleve su
      literal propio con el porqué (`pisoDelMargen`).
- [x] **El idle cuelga del plazo REAL del turno** (M38), no de la constante: `abrirEspejo` recibe el
      plazo, con el default resuelto adentro porque un cero daría un idle de **un minuto pelado**,
      peor que el defecto que este parámetro arregla.
- [x] **Cada escritura al cable lleva su write-deadline rodante** (T26–T33), en los **dos** caminos
      que llegan al socket (los eventos y el latido), desde un ayudante único porque dos copias
      derivan. ⚠️ La distinción que costó el agujero: **limpiar el deadline ABSOLUTO no es quedarse
      sin plazo** — son dos relojes sobre la misma conexión, y el test que custodiaba el primero
      **prohibía explícitamente la cura del segundo** con un `t.Error("…es justo lo que hay que
      evitar")`. Un test puede prohibir el arreglo de un defecto.

### El plazo del turno en un solo lugar, y su valor anclado (vuelta 4 de la revisión)

- [x] **Los diez reparos de prosa de los informes del panel**, con las cinco filas que faltaban en
      la tabla de sabotajes (M37–M41). 🔑 Y una lección de método que costó dos correcciones
      propias: **un grep de prosa en markdown miente por dos motivos a la vez** —el `**` se mete
      ENTRE las palabras, así que un patrón de dos palabras contiguas no casa nunca, y un en dash
      (`–`, 3 bytes en UTF-8) no es un guion, así que `.` casa un byte—. Con eso declaré que una
      frase del CHANGELOG no existía cuando sí existía. **Una autocorrección es una afirmación
      nueva y necesita su propia medición.**
- [x] **Cuatro custodios para lo que la vuelta 3 dejó sin cubrir**: el dominio negativo entero del
      `MaxTokens` de la API, el tope positivo que el diagnóstico de la fábrica anuncia, el crudo del
      yaml **en cualquier formato** (la guarda vieja buscaba la subcadena `"-1s"`, que la ata al
      `Duration.String()`) y la única línea de log de producción que el change-set agrega —el
      detalle del panic del motor, que el comentario del `recover` prometía y nadie medía—.
      Verificados por mutación: **5 de 7 mutantes eran invisibles al custodio viejo y visibles al
      nuevo**, y uno de los 7 lo fabriqué para refutar mi propio resultado, porque el primero que
      elegí ponía rojo al viejo por su *otra* aserción y no aislaba nada.
- [x] 🔑 **El plazo del turno estaba escrito TRES veces** —el lanzador, la fábrica que lo describe y
      el espejo que le cuelga el idle— y la del espejo ya había estado en `== 0` mientras las otras
      dos decían `<= 0`: los dos relojes del mismo turno resueltos con criterios opuestos. Quedó en
      `plazoDelTurno`, con **el contraejemplo** en su comentario, porque «no positivo ⇒ defecto» NO
      es una regla general del repo: `config.CognitionConfig.EffectiveMotorQuotaPerHour` usa la
      semántica opuesta para el negativo —cero ⇒ default, negativo ⇒ **sin límite**— y copiar la
      regla a ciegas allá convertiría un opt-out explícito en el default.
- [x] **Los sabotajes de M39 y M40 cambiaron de forma con la extracción, y había que reescribirlos**:
      citaban el texto literal de la guarda que dejó de existir, así que `arnes -validar` los habría
      marcado ROTAS. Hoy atacan «este sitio no usa el resolvedor», que es el defecto real que la
      extracción previene — más específico que el anterior, que era el mismo cambio escrito dos
      veces en dos archivos.
- [x] 🔴 **El VALOR del tope no lo custodiaba nadie** (M43), y es el hallazgo del juez adversarial.
      Las 30 menciones de `DemoraPorDefecto` en el paquete la usan todas como *referencia*
      —`.String()`, `> DemoraPorDefecto`, pasarla como argumento— y ninguna la compara contra un
      valor: con el defecto en `1 * time.Second` **el único `--- FAIL` del paquete entero es el
      custodio nuevo** (medido). Un test que mide un PREDICADO sobre una constante no custodia su
      VALOR.
- [x] **Y el resolvedor cura el dominio, no un punto** (M42). El aislamiento se midió: con un
      `== 0 || == -1s || == -5s` —los tres puntos exactos que los tests viejos usaban— los dos
      custodios del negativo quedan **VERDES** y un `-2` sigue naciendo con el contexto vencido.
- [x] Censo del arnés al disco: **81 directivas `arnes: de=` en `internal/motor`** = 9 de la serie
      `C` + 29 de la `E` + **43 de la `M`**, con la serie `M` de M1 a M43 **sin huecos ni
      repetidos** y repartida en 28 contra `espejo.go`, 13 contra `lanzador.go`, una contra
      `factory.go` y una contra `api.go`. La aritmética cierra y el script la verifica sola.

### Los cuatro tests que la vuelta 4 dejó sin arnés (vuelta 5 de la revisión)

- [x] **Lo que había que mecanizar eran cuatro, y el recuerdo decía cuatro por casualidad.** El censo al disco dice que `internal/motor` tiene **79 de 142** tests de nivel superior sin directiva propia: los cuatro de esta vuelta son los que la vuelta 4 **agregó**, no los únicos sin cobertura del arnés. Los otros 75 no entran acá —el ítem de las filas sin directiva ya está en el plan siguiente—, y decirlo importa porque «mecanizamos los que faltaban» habría sido falso.
- [x] **El cuarto no estaba en este paquete.** `TestElDetalleDelPanicDelMotorVaAlLog` vive en `internal/mcp/turno_test.go`, porque el `logx.Error` del recover lo escribe el handler del turno. Es otra serie de etiquetas (`T`, no `M`) y otro censo, y ponerlo en la tabla `M` de la spec habría dejado una fila apuntando a un archivo de otro paquete — justo lo que el verificador de huecos no puede ver.
- [x] 🔑 **Un sabotaje que no compila da un rojo que no es cobertura**, y la primera medición cayó entera en esa trampa: tres de los cuatro mutantes dejaban una variable o un import sin usar, el paquete no compilaba y los cuatro custodios se ponían rojos con sus siete hermanas. El veredicto decía «muerde» y lo que pasaba era otra cosa. La sonda ahora corre `go vet` sobre cada mutante **antes** de mirar un test, y el corte de T34 cambió de «quitar la traza» a `len(debug.Stack())` por eso: conserva el uso del import, logra el mismo daño y deja medible lo que el test promete.
- [x] **El aislamiento de los cuatro, medido**: cada custodio ROJO y las **siete** hermanas VERDES, con el detalle de cuál deja viva a cuál en la tabla de la spec. Es lo que distingue un corte propio de «elegí el que ya tenía dueño».
- [x] **La colisión la denunció el arnés, no la predije yo.** Se escribieron las cuatro directivas y `arnes -validar` denunció **una sola**, en sus dos sentidos: M44 ↔ M41, sobre el mismo `if` de `api.go`. Declarada de los dos lados con el motivo de cada una, porque la detección es direccional y cada directiva responde por el daño que hace ella. M45 y M46 comparten el `de` de la línea del `Sprintf` y **no** colisionan: las dos sólo insertan, así que tras aplicar una el ancla de la otra sigue apareciendo una vez.
- [x] Censo al disco después de escribirlas: **84 directivas en `internal/motor`** = 9 `C` + 29 `E` + **46 `M`** (28 `espejo.go` + 13 `lanzador.go` + 3 `factory.go` + 2 `api.go`), serie `M` de M1 a M46 sin huecos ni repetidos; y **34 de la serie `T`** en `turno_test.go`, de T1 a T34, con 31 contra `turno.go` y 3 contra `http.go`. `arnes -validar`: **2175 mecanizadas · 0 ROTAS · 0 colisiones sin contestar · 99,4 %**.

## F5 · El cableado en `musubi serve` — CERRADO

- [x] **No es un subcomando**, que es como lo nombraba el plan. No hay un `musubi motor` nuevo: el
      motor se construye y se cablea dentro de `runServe`, el único comando que sirve el mux HTTP
      donde vive la ruta. Un subcomando aparte habría necesitado su propio servidor para servir una
      ruta que ya está servida.
- [x] `motor.Desde(cfg.Motor)` **antes** de construir el servidor, con el error **fatal**: un
      proveedor mal escrito o una credencial ausente son errores de CONFIGURACIÓN, y el único que
      los puede arreglar es quien editó el yaml — no el cliente remoto, que sólo vería un 405.
- [x] `mcp.WithMotor(motorDeTurnos)` en la lista de options de `runServe`, y **no** en `runDaemon`:
      un `provider: api` sin credencial le voltearía el arranque a cada sesión del agente por un
      servicio que un stdio no tiene por dónde servir.
- [x] Una línea de arranque **siempre**, también apagado, por `motor.DescribirMotor` — que nombra la
      variable de la credencial y nunca su valor.
- [x] `cmd/musubi/motor_cableado_test.go`: 4 guardas sobre el **AST** de `main.go` y 5 sabotajes
      (W1–W5) en `spec.md`, **ROJO 5/5** medido al disco con la reversión verificada por hash.
- [x] 🔴 **El censo cazó un defecto que yo había introducido**, y es la razón de que estas guardas
      lean el AST y no el texto. Insertar `mcp.WithMotor(motorDeTurnos), ` en la lista de options
      partió el `de` de `internal/mcp/fleet_prometheus_version_test.go:222`, que anclaba en
      `mcp.WithUsageLedger(…), mcp.WithVersion(version))` porque `mcp.WithVersion(version))` solo
      aparece en dos construcciones y no identifica una. **Su verde pasó a significar «no lo
      apliqué».** Re-anclado al vecino nuevo, con el porqué escrito ahí; de rebote las colisiones
      del censo bajaron de 17 a 0, porque el `de` corto ya no se pisa con los otros de `main.go`.
- [x] 🔴 **La primera versión de W1 era VACUA y el overlay la delató.** Borrar la option deja
      `motorDeTurnos` huérfana y el paquete no compila (`declared and not used`), así que su rojo
      ponía en rojo al paquete entero y no decía nada sobre la guarda. Lo que el compilador ya
      custodia no hace falta sabotearlo: W1 pasó a atacar el **argumento**, que es la mitad del
      invariante que el compilador no ve.
- [x] 🔴 **El `de` de W5 arrastraba dos vecinos para ser único** —las tres options finales del
      daemon— y el censo lo denunció como colisión triple contra `daemon_agente_test.go` y
      `agente_plugin_test.go`. Re-anclado a `maintCtx, stopMaint := …`, que es único, exclusivo de
      `runDaemon` y de nadie más. Censo final: **0 colisiones**, 2117 directivas apuntando a un
      literal único.
- [x] ⚠️ **`arnes -overlay` no les puede dar veredicto a estos cinco, y un verde ahí no es un
      verde.** Ese modo reemplaza archivos para el *build* de Go; estas guardas parsean `main.go`
      del **disco** y ven el archivo sano. Los cinco salieron «candidato a rojo falso» y, en la
      misma corrida, la posición 182 —`identidad_servicios_unix_test.go:44`, del repo y ajena a
      esto— salió igual: es la **clase** de prueba. El veredicto sale al disco, con una prueba
      **testigo** del mismo paquete que tiene que seguir verde — es lo que separa un rojo de la
      guarda de un rojo del paquete.
- [x] **Los 60 sabotajes de F3 se portaron al arnés del repo**, que es donde se podren
      ruidosamente: vivían en un script del scratchpad, que se borra. Censo 2065 → 2125 anclas
      (+60 exactas), 0 rotas, y `TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre` sin una queja.
      Los 38 de `internal/motor` dan **38/38 ROJO bajo overlay** — más fuerte que el script,
      porque prueba que el rojo no depende de escribir el disco.
- [x] Pegar en `spec.md` la tabla de invariantes de F5 (W1–W5) antes de
      `## Verificación de punta a punta`, y corregir `## Lo que falta especificar`.
- [x] 🔴 **`go test ./...` entero destapó un guardián del repo que ningún paquete mío corría.**
      `TestTodoClienteDelCerebroPasaPorElConstructor` vive en `cmd/musubi` pero **barre los 433
      `.go` del repo**, y acusó a `internal/motor/api.go` por armarse un `http.Client` a mano sin
      declarar a quién le habla. Era un defecto real y mío: `ClienteAPI` disca `Endpoint`
      (api.anthropic.com o una pasarela compatible), nunca el cerebro, así que le faltaba la marca
      `no habla con el cerebro:` pegada al literal. Puesta, el censo del guardián pasó de 14 a **15
      clientes declarados ajenos** y la enumeración de su `t.Fatalf` se actualizó para no envejecer
      mintiendo. **Correr sólo los paquetes que tocás no alcanza cuando la guarda que te juzga vive
      en otro**: el verde de `./cmd/musubi -run …` y de `./internal/motor` convivía con este rojo.

## F6 · El cableado de ThinkRail — PENDIENTE

- [ ] `streamFn: streamProxy(...)` apuntando a Musubi.
- [ ] ⛔ No se toca el cuerpo desde esta terminal: va cuando el dueño lo pida.

## Entrega

- [x] `specs/motor-proxy-stream/{proposal,spec,design,tasks}.md`.
- [x] Entrada en CHANGELOG `[Unreleased]` — **ocho** viñetas en `### Added`, contadas al disco: el
      paquete, el endpoint, el cableado en `serve`, el MCP espejo, su endurecimiento (M21–M27), el
      del listener con los dos contadores (M28–M36), los dos plazos del cable (M37–M38 y T26–T33) y
      la mecanización de los cuatro custodios de la vuelta 4 (M44–M46 y T34).
      ⚠️ Acá decía **seis** y enumeraba seis: la séptima se agregó en la vuelta 3 y el conteo se
      quedó atrás. La vuelta 4 no suma una octava —lo suyo entra en la cuarta, que es la que habla
      de los sabotajes de la pieza—, así que el número de viñetas y el de vueltas no coinciden y no
      tienen por qué. **El `VERSION` no se toca acá**: en este repo el bump
      va en su propio `chore(release): cortar …` (así entró `087e65b0`, que sólo sumó 3 líneas al
      CHANGELOG, contra `58ed22cd`, que fue el corte). Y la rama está por encima del corte de
      0.147.0, así que NO aplica el gotcha del release-cut que mete la viñeta dentro de la
      versión ya tagueada.
- [x] El golden de `tools/list` **no hace falta**: F4 no agrega una tool al servidor MCP de
      Musubi, levanta un servidor MCP **aparte** —el espejo— cuyo único cliente es el hijo. El
      registry del repo no se tocó, y la batería entera lo confirma.
- [ ] El debate adversarial de la vuelta siguiente (el previo cerró 3-0 a `no_real` sobre el
      estado anterior del paquete; sobre un tally cerrado hay que abrir un debate NUEVO).
- [ ] Push y merge: **sólo con el OK explícito del dueño**, y el push exige
      `musubi receipt emit`.
