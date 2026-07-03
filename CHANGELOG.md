# Changelog

Todos los cambios notables de Musubi se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

## [0.60.0] - 2026-07-03

Track 13 — Ola A (cosechar el run journal). Frutos de observabilidad y robustez sobre el journal de v0.59.0.
Cuatro features, cada una dogfoodeada por el flujo SDD completo y **sin migración de esquema** (todo se apoya en
el journal `run_events` de v0.59.0): **export OpenTelemetry**, **saga (compensación LIFO)**, **HITL
(interrupt/resume durable)** y **gate de verificación + Reflexion**. `musubi_workflow` pasó de 8 a 13 acciones;
el catálogo sigue en 30 tools; todo aditivo y model-free.

### Added
- **Gate de verificación duro + Reflexion en workflows** (`musubi_workflow action=verify`): cierra el
  *verification-generation gap* (generar es fácil, verificar es el cuello de botella). Un step puede declarar
  `verify` (la directiva de qué chequear); al completarlo con `done` **no** queda hecho: entra en `verifying`
  (no terminal, bloquea a sus dependientes) hasta que un veredicto lo resuelva. `action=verify` (run_id, step,
  verdict `pass|fail`, reflexión en `result`): **pass** → `done` (uniforme: journalea `step_completed`);
  **fail** → registra la **reflexión** y, si queda presupuesto de intentos, **reabre** el step para un reintento
  informado (**Reflexion**); al agotarse (`max_iterations`, default 3), el step queda `failed` (el gate no se
  satisface). Las reflexiones acumuladas se devuelven para informar el reintento y quedan en el journal. Nuevo
  estado (`verifying`) y eventos (`step_verifying`, `step_reflection`). **Sin migración**. Model-free: Musubi
  impone la estructura del gate y registra; el veredicto lo produce el agente, idealmente con una lente
  adversarial (la skill `adversarial-review` lo fomenta) — adversarial > auto-chequeo.
- **HITL: interrupt/resume durable en workflows** (`musubi_workflow action=provide`): un step puede declarar
  `await` (un prompt), volviéndolo un **gate humano**. Al quedar listo, el run se **pausa** en él
  (`waiting_input`) en vez de ofrecerlo para ejecutar, bloquea a sus dependientes, y las respuestas lo surface en
  `waiting` con su prompt. Se reanuda con `action=provide` (run_id, step, input, status): `done` = aprobado (el
  `input` queda como resultado, los dependientes se destraban), `failed` = rechazado (siguen bloqueados). La
  espera es **durable** por construcción (estado + journal en SQLite): se puede proveer la decisión **en otra
  sesión** y el run continúa exactamente donde estaba (patrón interrupt/resume de LangGraph). Un gate con `when`
  falso se salta en vez de pausar. Nuevo estado de step (`waiting_input`) y evento de journal (`step_waiting`).
  **Sin migración**. Model-free: Musubi expone QUÉ espera y su prompt; el aviso al humano es del integrador.
- **Saga: compensación LIFO en workflows** (`musubi_workflow action=rollback` / `compensated`): el motor sabía
  avanzar un DAG pero no **deshacer**. Ahora un step puede declarar `compensate` (la directiva de cómo revertirlo);
  `action=rollback` inicia la **saga** y devuelve el plan de compensación en orden **LIFO** (inverso al de
  completado) de los steps completados con compensación; el agente ejecuta cada *undo* y reporta con
  `action=compensated` (run_id, step), que devuelve el plan restante; al vaciarse, el run queda `compensated`. El
  plan se **deriva del run journal** (re-entrante e idempotente: compensar dos veces un step es no-op; re-`rollback`
  recomputa lo que falta). Model-free: Musubi coordina QUÉ y EN QUÉ ORDEN; el agente ejecuta el undo real.
  Nuevos estados de run (`compensating`, `compensated`) y eventos de journal (`run_rollback`, `step_compensated`,
  `run_compensated`). **Sin migración** (el campo viaja en la definición ya persistida). El disparo es explícito
  (un step `failed` no fuerza rollback; la política es del agente).
- **Export OpenTelemetry del run journal** (`musubi_workflow action=otel`): exporta un run de workflow como una
  **traza OTLP/JSON** estándar (el run es un *trace*, cada step un *span*), lista para ingerir en cualquier
  collector (Jaeger, Grafana Tempo, etc.). La traza se **deriva** del journal en el momento del export (principio
  "derivar, no guardar-y-desfasar" — sin tabla de spans, sin migración, sin drift). IDs OTel **deterministas**
  (trace_id 16 bytes de `run_id`, span_id 8 bytes de `run_id`+`step_id`, por SHA-256 truncado): re-exportar da la
  misma traza. Status por step (`failed`→ERROR, `done`→OK, `skipped` marcado), atributos (`musubi.seq`,
  `event_type`, `result`, `workflow_id`), `service.name=musubi`. Model-free, Go puro, **sin el SDK de OTel** (el
  OTLP/JSON se emite a mano). Musubi sólo devuelve el JSON; el transporte al collector es del consumidor
  (local-first). Alinea con la dirección del servidor casero (Musubi como cerebro + orquestador observable).

## [0.59.0] - 2026-07-03

Track 13 — endurecimiento de los dos pilares (memoria + orquestación) con ingeniería SOTA, toda model-free.
Tres cambios, cada uno dogfoodeado por el flujo SDD completo: un **bugfix de liveness** en la pizarra (lease/TTL),
la **invalidación bi-temporal** del grafo de hechos (memoria que ya no envejece mal), y el **run journal
append-only** con idempotencia (cimiento de replay/observabilidad). Esquema evolucionado a la versión v6. El
catálogo sigue en 30 tools; todo aditivo y retrocompatible.

### Fixed
- **Bug de liveness en la pizarra multi-agente (`musubi_work`)**: una unidad que un sub-agente reclamaba y luego
  abandonaba (crash, timeout, sesión cerrada) quedaba en `claimed` **para siempre** — ningún otro agente podía
  retomarla y el batch nunca cerraba. Ahora cada claim toma un **lease con vencimiento (TTL)**: si el dueño no lo
  renueva, la unidad se recicla automáticamente en el próximo `claim` (reclamo *lazy*, sin proceso de fondo).

### Added
- **Run journal append-only + idempotencia por step** (Track 13, orquestación): el motor de workflows
  (`musubi_workflow`) sólo guardaba un **snapshot mutable**, sin idempotencia (un `complete` repetido
  sobrescribía en silencio) ni historia (no se podía auditar/exportar/replay). Ahora cada transición del run
  (arranque, step completado/saltado/reabierto, run cerrado) se registra en un **journal append-only**
  (`run_events`), escrito en la **misma transacción** que actualiza el snapshot — event-sourcing con read-model
  materializado, así journal y estado corriente nunca divergen. `complete` acepta una **`idempotency_key`**
  opcional: reintentar con la misma clave es un **no-op seguro** (no re-aplica ni duplica). Nueva acción
  `journal` (run_id) que devuelve la traza de eventos del run (`WorkflowJournal`). Es el cimiento estructural de
  replay/HITL/saga/observabilidad (OTel), que quedan habilitados para cambios futuros. Migración de esquema
  **v6** (tabla `run_events` con `UNIQUE(run_id, seq)` y `UNIQUE(run_id, idempotency_key)`), aditiva: el
  snapshot y su API siguen intactos.
- **Invalidación bi-temporal del grafo de hechos** (Track 13, memoria): hasta ahora `musubi_save_fact` sólo
  **acumulaba** tripletas y nunca retiraba ninguna, así que `(Ana, trabaja_en, Acme)` y `(Ana, trabaja_en,
  Globex)` convivían como si ambas fueran verdad. Ahora el grafo es **bi-temporal** (patrón Zep/Graphiti,
  model-free): para un predicado **funcional** (*single-valued*: `trabaja_en`, `estado_actual`, `vive_en`…,
  declarados en `graph.single_valued_predicates`), guardar un objeto nuevo **invalida** automáticamente el
  anterior por **cardinalidad** — sin LLM, sin entender el texto. El hecho viejo no se borra: se le cierra la
  ventana de validez (`valid_from`/`valid_to`, `invalidated_at`, `superseded_by`), de modo que la historia queda
  auditable. `musubi_recall_facts` devuelve por defecto sólo la **verdad actual** y acepta un parámetro **`as_of`**
  para consulta *point-in-time* ("qué era verdad en tal momento"). `musubi_save_fact` acepta un `valid_from`
  opcional y **revive** un hecho invalidado si se re-afirma. Migración de esquema **v5** (4 columnas aditivas +
  índice + backfill `valid_from = created_at`), retrocompatible. Los predicados *many-valued* (no declarados) no
  invalidan nada.
- **Lease/TTL + heartbeat + fencing token en `musubi_work`** (Track 13, orquestación): patrón *visibility timeout*
  (SQS) / lease (Chubby) sobre la pizarra, 100% model-free. Nuevo `action=heartbeat` para renovar el lease
  mientras el sub-agente trabaja; el `claim` devuelve un **fencing token** monótono que `heartbeat`/`complete`
  validan para bloquear al "worker zombie" (un agente expropiado que revive con un token viejo afecta 0 filas),
  incluso cuando dos agentes comparten el mismo id. Dead-letter automático (`failed`) tras `max_attempts` reclamos,
  para no reciclar indefinidamente una unidad que siempre falla. TTL y máximo de reintentos configurables
  (`multiagent.lease_ttl_seconds` = 300, `multiagent.max_attempts` = 5). Migración de esquema **v4** (columnas
  aditivas `owner_id`/`lease_expires_at`/`heartbeat_at`/`attempts`/`fencing_token` + índice), retrocompatible.
  Semántica *at-least-once* → el trabajo delegado debe ser idempotente.

## [0.58.0] - 2026-07-03

Release de dos hitos: **el pilar de orquestación/SDD elevado a co-igual de la memoria** (Track 12) y la
**inteligencia de cambios de código** (`musubi_detect_changes`). El catálogo de tools pasó de 27 a 30.

### Added
- **`musubi_detect_changes` — inteligencia de cambios de código (model-free, Go puro)**: nueva tool que corre
  `git diff` y, para cada archivo tocado, RE-DERIVA sus símbolos del contenido **actual** (`go/ast` para `.go`;
  escáner liviano para `.ts/.tsx/.js/.jsx/.py`) en vez de confiar en datos guardados — así el diff y los
  símbolos viven siempre en el mismo sistema de coordenadas y nunca se desalinean. Reporta, por archivo: el
  tipo de cambio, los símbolos afectados por los hunks, si su gist de memoria de código quedó *stale*
  (fingerprint) y qué observaciones/decisiones lo referencian. Es de solo-lectura y se engancha en la fase
  `verify` del flujo SDD para acotar qué verificar y qué decisión quedó potencialmente obsoleta. Nuevo paquete
  `internal/codeintel` (extractor de símbolos/imports + parser de diff unified), sin dependencias con cgo.
- **`musubi_save_code` deriva símbolos automáticamente**: cuando no se pasa `symbols`, se extraen del contenido
  actual del archivo (anclados al mismo fingerprint), evitando el string manual que se desincronizaba. Si el
  llamador pasa `symbols` explícito, se respeta (compat hacia atrás).
- **Flujo SDD guiado — `musubi_sdd`** (Track 12 O1): genera por vos el workflow canónico de un cambio
  (`proposal→spec→design→tasks→implement→verify→archive`) sobre el motor DAG, sin escribir YAML, y guía fase
  por fase; al cerrar cada fase persiste su contrato de resultado en memoria (`sdd/<change>/<phase>`) para que
  las siguientes lo recuperen por referencia barata en vez de releer archivos. Resumible entre sesiones.
- **Estimador de ahorro por delegación — `musubi_work action=savings`** (Track 12 O2): estimación model-free
  de los tokens ahorrados al delegar en la pizarra vs. hacerlo inline (aislamiento de contexto), con
  parámetros configurables.
- **Sistema avanzado de creación de skills** (Track 12): validador de calidad model-free
  (`internal/skills/quality.go`) que puntúa una skill contra las best-practices de Agent Skills (description
  como disparador en 3ª persona ≤1024 chars, name sin reservadas, triggers acotados, rules con ejemplo) y
  bloquea el guardado si tiene errores; nueva tool **`musubi_author_skill`** (reporte scoreado sin guardar, o
  guardado tras pasar el gate; reporta el tier de confiabilidad de la fuente).
- **Skills cognitivas embebidas**: `sdd-flow`, `adversarial-review` y `designing-web-ui` (WCAG AA + escala de
  espaciado 4/8px), incluidas en el bundle de `musubi setup`.
- **Cerebro remoto self-hosted** (Track 12 S): soporte para apuntar el MCP a una instancia central de Musubi
  vía entrada remota con token por variable de entorno; incluye runbook de onboarding.

### Changed
- **Dashboard de la memoria**: nuevo pilar de orquestación (runs/batches) en el snapshot y la vista (Track 12
  O4), y barrido completo a un sistema de espaciado 4/8px + escala tipográfica (skill `designing-web-ui`).

## [0.57.0] - 2026-06-23

### Added
- **Auditoría UX del dashboard contra el skill `ui-ux-pro-max`** (Track 11): se aplicó el *pre-delivery
  checklist* del skill (reglas de accesibilidad, timing de animación y contraste). El dashboard ya cumplía la
  mayoría (focos visibles, teclado en el grafo, *skeleton*, cifras tabulares, formato locale, sin emojis como
  iconos); esta release cierra los gaps detectados.

### Changed
- **Movimiento reducido**: la barra de carga deja de animarse bajo `prefers-reduced-motion: reduce` y se
  acortan todas las transiciones — el movimiento es 100% opcional. El *placeholder* de carga pasa de un
  *shimmer* de texto (que con `color:transparent` podía dejar los números de los KPIs invisibles en algunos
  *frames*) a un simple atenuado por opacidad: la barra superior indeterminada es la única señal de carga y
  nunca oculta contenido.
- **Chip de filtro accesible**: el chip «dominio ✕» del panel de memorias pasa a ser un control de verdad
  (`role="button"`, `tabindex`, `aria-label`) y se puede limpiar el filtro con `Enter`/`Espacio` (antes era
  solo *click*).
- **Timing de micro-interacción**: el *count-up* de KPIs y gauge baja de 620 ms a **400 ms** (regla del skill:
  micro-interacciones ≤ 400 ms).
- **Reveal escalonado**: los nodos del grafo aparecen con *stagger* de 35 ms por nodo (más natural; bajo
  movimiento reducido aparecen al instante).
- **Contraste AA**: el color de texto secundario `--dim` sube a ~4.6:1 sobre el fondo (antes ~4.2:1) para
  cumplir el mínimo 4.5:1 de texto normal.

## [0.56.0] - 2026-06-23

### Added
- **Pulido visual + UX del dashboard** (Track 11): el dashboard local sube de nivel manteniendo la estructura,
  los datos en vivo y el coste **0 tokens**:
  - **Sistema visual refinado**: tokens de contraste/espaciado/radio/elevación, fondo con aura sutil de la
    marca, cabeceras de sección con barra de acento y KPIs con franja superior de color por métrica.
  - **Micro-interacciones**: los números de los KPIs y el gauge hacen *count-up* animado (easeOutCubic), el
    gauge tiñe su halo según el estado del presupuesto, y los nodos del grafo aparecen con un *pop* suave.
  - **Estados**: barra de carga indeterminada + *skeleton* shimmer mientras llega el primer snapshot (sin
    parpadeo brusco), estados vacíos más claros y *hover* de las tarjetas de memoria.
  - **Accesibilidad**: navegación por teclado del grafo (`Tab` + `Enter`/`Espacio`), `aria-label` y anillos de
    foco en los nodos, mejor contraste de texto y todo el movimiento bajo `prefers-reduced-motion`.
- **Path del proyecto en la cabecera**: el snapshot trae un campo `project` (nombre de la carpeta raíz) y el
  dashboard lo muestra («proyecto X»), para no confundir de qué workspace son los datos.

### Changed
- El grafo solo se re-dibuja cuando cambian los datos o el estado (expandido/filtro) — antes se re-renderizaba
  en cada *poll* de 4 s, re-animando los nodos y perdiendo el *hover*. Ahora una firma de render lo evita.

## [0.55.0] - 2026-06-23

### Added
- **Grafo de conocimiento interactivo** (Track 11): el mapa pasa de una «estrella» plana a un grafo de
  **dos niveles, vivo y explorable**:
  - **Drill-down**: cada dominio se abre en sus **sub-temas reales** (`roadmap` → `track-8`, `track-7`…);
    arranca con el más activo ya expandido. Clic en un dominio lo abre **y filtra** las memorias de abajo.
  - **Brillo por recencia**: los temas con actividad reciente brillan; los viejos se apagan.
  - **Hover** → tooltip con conteo, «última actividad hace X» y un ejemplo de memoria.
  - **Aristas curvas con peso** (grosor ∝ nº de memorias, opacidad ∝ recencia) + leyenda.
- **`DbEngine.TopicTree()`** (`internal/memory/topics.go`): arma el árbol dominio → temas de las
  observaciones activas, con conteo y última actividad por nodo (`DomainNode`/`TopicLeaf`). El snapshot de
  `export` ahora expone ese árbol en `graph.domains` (antes solo `{domain, count}`).

### Changed
- `graph.domains` del snapshot ahora es el árbol enriquecido (cada dominio trae `last_activity` y `topics`).
- Las memorias recientes del snapshot suben de 12 a 20 (mejor cobertura del filtro por dominio).

## [0.54.0] - 2026-06-23

### Added
- **Dashboard legible** (Track 11): el dashboard deja de ser solo métricas técnicas y suma contenido que un
  humano puede leer para familiarizarse con Musubi:
  - **«Lo que Musubi recuerda»**: las memorias reales del proyecto en lenguaje claro (tema + resumen + hace
    cuánto), no solo conteos.
  - **«Actividad reciente»**: una línea de tiempo cronológica de lo último que se guardó (la memoria
    «creciendo» mientras trabajás).
  - **Explicaciones**: cada sección técnica con una línea que la traduce a lenguaje claro + tooltips en los
    KPIs.
- **`DbEngine.RecentObservations(limit)`** (`internal/memory/operations.go`): devuelve las últimas
  observaciones NO archivadas en forma legible (`ObsCard`: tema, gist, fecha, importancia); cae al recorte
  del contenido si falta el gist. El snapshot de `export` ahora incluye el campo `recent`.

### Notes
- Frontend en `cmd/musubi/assets/dashboard.html` (data-driven). Tests: `TestRecentObservations` y la
  verificación de `recent` en `TestBuildExportSnapshot`.

## [0.53.0] - 2026-06-23

### Added
- **`musubi dashboard`** (UI local en vivo): nuevo subcomando que sirve una **interfaz web de solo lectura**
  de la memoria —salud, gobernador de tokens (gauge + barras por superficie + umbrales watch/over), checks y
  un **mapa de conocimiento** radial por dominio—. El HTML va **embebido en el binario** (`//go:embed`) y se
  actualiza solo (polling a `/api/snapshot`, que reusa el snapshot de `export`).
  - **Opt-in y cero tokens**: corre como proceso aparte, no se engancha a ningún hook ni inyecta nada al
    contexto del agente. Los datos van de SQLite al navegador, sin LLM en el medio.
  - **Solo loopback** (`127.0.0.1` por defecto, puerto `7777`): por diseño es de uso local; rechaza bind a
    interfaces públicas. Flags: `--addr <host:port>`, `--no-open` (no abrir el navegador).

### Notes
- `dashboard.go` (`runDashboard`, `dashboardHandler`, `isLoopbackAddr`, `openBrowser`) + asset embebido en
  `cmd/musubi/assets/dashboard.html` (data-driven: renderiza desde el JSON y hace polling). Tests:
  `TestDashboardSnapshotEndpoint`, `TestDashboardIndexServesHTML`, `TestIsLoopbackAddr`.

## [0.52.0] - 2026-06-23

### Added
- **`musubi export`** (observabilidad): nuevo subcomando CLI que vuelca un **snapshot JSON** del estado de
  la memoria —salud (`doctor`), insights, ledger de tokens (`tokens`) y un **mapa de conocimiento** por
  dominio de topic— en stdout o a un archivo (`--out <ruta>`). Read-only, model-free, una sola pasada.
  Es la fuente de datos estable para dashboards y observabilidad externa: reúne las mismas vistas que las
  tools MCP en un único documento con forma fija que consumen las UIs.
- **`DbEngine.TopicDomainCounts()`** (`internal/memory/topics.go`): agrega las observaciones activas por el
  **dominio** del topic (prefijo antes del primer `/`; `roadmap/track-7` → `roadmap`), ordenado por cantidad.
  Alimenta el mapa de conocimiento sin LLM (agregación SQL determinista).

### Notes
- `buildExportSnapshot` (`cmd/musubi/export.go`) compone el documento reusando `Diagnose`/`Insights`/
  `LedgerStatus().Budget`/`TopicDomainCounts`; sin duplicar lógica. Tests: `TestBuildExportSnapshot`,
  `TestTopicDomainCounts`.

## [0.51.0] - 2026-06-22

### Added
- **Brevedad del gobernador** (Track 9 / T9.5): nueva superficie por turno `turn_brevity` que inyecta una
  directiva para que el agente responda **conciso**, recortando los tokens de **SALIDA** (las respuestas
  del modelo). Cierra el arco del gobernador de tokens: medir (T9.1) → avisar (T9.3) → **reducir la salida**.
  Hasta ahora todas las superficies solo acotaban la **ENTRADA** (el contexto inyectado); esta toca el otro
  lado del presupuesto. Inspirada en la skill de comunidad `caveman`, pero nativa y atada al gobernador.
- **Config `memory.brevity_mode`** (opt-in, default `off`):
  - `off` — no inyecta nada (sin cambios de comportamiento).
  - `lite` / `full` / `ultra` — fija el nivel de concisión; se inyecta **una vez por sesión** (la directiva
    persiste en contexto, no se repite turno a turno).
  - `auto` — solo dispara cuando el gasto de la sesión cruza `session_token_budget` (mismo umbral que la
    alerta proactiva), de modo que **bajo presupuesto su costo es cero**. Requiere `session_token_budget > 0`.
  - Un valor inválido degrada a `off`: un typo nunca enciende la directiva. Toda directiva **preserva exacto**
    el código, comandos, rutas, nombres de API, versiones y flags.

### Notes
- `buildBrevityNudge`/`brevityDirective` en `turn.go`; throttle por `session_id`+modo (`loop_brevity_injected`).
  La superficie se contabiliza en el ledger holístico como `turn_brevity`. Tests: `TestTurnBrevityManual…`,
  `TestTurnBrevityAuto…`, `TestTurnBrevityOffSilent`, `TestBrevityDirectiveLevelsDiffer`, `TestLoadBrevityMode…`.

## [0.50.0] - 2026-06-22

### Added
- **Pulido de la instalación y el `usage`** (Track 10 / T10.2): tres mejoras de UX del CLI surgidas de la
  auditoría de primera experiencia:
  - **Guardia anti "trampa del doble clic"**: si en el menú interactivo se elige instalar **local** en una
    carpeta que NO parece un proyecto (sin `go.mod`/`package.json`/`.git`/…, típico de hacer doble clic
    sobre el `.exe` en Descargas), Musubi avisa y pide confirmación explícita, sugiriendo la opción Global.
    En un proyecto real procede sin molestar.
  - **Aviso de fragilidad del modo local**: tras `setup` sin instalación global, si el `.mcp.json` queda
    referenciando el binario por ruta absoluta (sin `MUSUBI_BIN` ni `musubi` en el PATH), avisa que mover
    o borrar el binario rompe la carga, con un tip hacia el modo Global (ruta estable).
  - **`usage` agrupado y alineado**: el muro de texto pasa a secciones (Instalación, Servidor MCP,
    Memoria, Catálogo, Binario, Hooks) con columnas alineadas y headers en color.

### Notes
- Helpers `looksLikeProject` (heurística por manifiestos/`.git`), `isYes` (confirmación s/si/y/yes) y
  `confirmLocalDir`. El padding del `usage` se aplica ANTES de colorear, así el alineado no se descuadra
  con o sin ANSI. Tests: `TestLooksLikeProject`, `TestIsYes`.

## [0.49.0] - 2026-06-22

### Added
- **Consola de Windows en UTF-8 + color en el CLI** (Track 10 / T10.1, experiencia de instalación): al
  arrancar, Musubi inicializa la consola de Windows (`SetConsoleOutputCP(CP_UTF8)` + habilita
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING`) — 100% Go vía syscall a kernel32, sin CGo. **Arregla el mojibake
  del primer contacto**: en un cmd.exe fresco (codepage OEM 850/437) los `✓` y acentos que emite `setup`
  salían como basura (`✓`→`Ô£ô`, `Reabrí`→`ReabrÝ`). Ahora renderizan bien y se desbloquea el color ANSI.
  El menú de instalación por doble clic y la salida de `setup` ahora usan color (verde `✓`, headers en
  cyan, énfasis en negrita).

### Notes
- El color es **seguro por defecto**: solo se emite cuando stdout es una TERMINAL real, el VT está
  habilitado y `NO_COLOR` no está seteada. En los hooks, el daemon y los pipes/redirecciones (donde
  stdout es el canal JSON-RPC o una captura) la salida queda **en texto plano** — verificado que
  `setup` piped y `detect --hook-mode` no emiten ANSI y el JSON de hook sigue limpio. Archivos:
  `console_windows.go` / `console_other.go` (build-tagged) y `style.go` (helper de estilo memoizado por TTY).

## [0.48.0] - 2026-06-22

### Changed
- **Superficies por turno delta-aware: fase y conflictos solo se reinyectan al cambiar** (Track 9 / T9.4):
  el recordatorio de fase del pipeline (`turn_phase`) y el aviso de conflictos (`turn_conflicts`) se
  inyectaban **enteros cada turno**. Una simulación de sesión realista contra el ledger holístico
  (`footprint_test.go`) mostró que `turn_phase` era el costo que **más escala**: ~58 tok/turno **sin
  delta** → en una sesión de 40 turnos ≈ **2.300 tokens** repitiendo la misma línea, más que cualquier
  costo de arranque (que es one-time). Ahora ambos siguen el mismo principio que `turn_recall`: se
  inyectan completos **solo cuando cambian** (la fase al avanzar de fase/tarea; los conflictos al
  cambiar la cantidad) y callan mientras tanto (el agente ya los tiene en contexto). Medido: `turn_phase`
  232→58 (primera sesión) y 224→56 (establecida) sobre 4 turnos; el ahorro crece con la longitud de la sesión.

### Notes
- Helper `turnSurfaceChanged` (delta por superficie, con el `session_id` como prefijo para reiniciar al
  cambiar de sesión, igual que el delta del recall). Estado en meta `loop_phase_injected` /
  `loop_conflicts_injected`. Nuevo `footprint_test.go`: simula una primera sesión (proyecto nuevo: dispara
  cognitivo + generación de skills) y una establecida (perfilada) y reporta el footprint por superficie —
  auditoría reproducible que fundamentó esta decisión sobre datos, no intuición.

## [0.47.0] - 2026-06-22

### Added
- **Alerta proactiva del gobernador por turno** (Track 9 / T9.3): cuando el gasto acumulado de la sesión
  cruza el presupuesto blando (`memory.session_token_budget`), el hook por turno (UserPromptSubmit) inyecta
  **una** línea avisando —**una sola vez por sesión** (throttle por `session_id`), para no convertir el
  aviso en ruido—. Cierra el lazo del gobernador: T9.2 lo mostraba **si el agente consultaba**
  `musubi_tokens`; ahora el aviso es **proactivo**, con el desglose a un comando de distancia. Sigue siendo
  **blando** (no recorta nada) y model-free. Con `session_token_budget: 0` queda desactivado.

### Notes
- El aviso vive en `buildBudgetAlert` (lee el ledger ANTES de contabilizar el turno, así que puede atrasarse
  un turno respecto del cruce exacto: oportuno sin ser molesto) y se contabiliza como la superficie
  `budget_alert` del propio ledger. Throttle vía meta `loop_budget_alerted`. `turnOutput` recibe el
  presupuesto desde `cfg.Memory.SessionTokenBudget`.

## [0.46.0] - 2026-06-22

### Added
- **Gobernador de sesión: presupuesto blando de tokens + reporte** (Track 9 / T9.2): nueva opción
  `memory.session_token_budget` (default **8000**, `0` = sin techo) y `musubi_tokens` ahora devuelve el
  reporte del **gobernador**: total, presupuesto, **restante**, **% usado**, **estado** (`ok` <75% ·
  `watch` ≥75% · `over` ≥100%) y el **desglose por superficie ordenado por gasto** (cada una con su % del
  total). Sobre el ledger holístico de T9.1, esto convierte los números crudos en una señal accionable:
  de un vistazo se ve cuánto contexto consume Musubi y **qué superficie** lo domina. Es **blando**: no
  recorta nada (eso arriesgaría eficiencia); solo mide y reporta para que el gasto sea visible y acotable.

### Notes
- El estado/umbrales viven en `TokenLedger.Budget(budget)` (model-free, determinista, testeable). El
  presupuesto es del bloque `memory`; un `session_token_budget: 0` EXPLÍCITO se respeta (opt-out) y no se
  pisa con el default. La alerta PROACTIVA por turno (avisar al cruzar el techo sin que el agente consulte)
  queda para T9.3. Golden de `tools/list` regenerado por el cambio de descripción de `musubi_tokens`.

## [0.45.0] - 2026-06-22

### Changed
- **Ledger holístico de tokens: medir TODAS las superficies, no solo el recall** (Track 9 / T9.1): el
  ledger de tokens (`musubi_tokens`) ahora contabiliza **cada** superficie que inyecta contexto, no
  solo el priming y el recall por turno. Antes quedaban **invisibles** —y por lo tanto sin medir ni
  optimizar— el bloque cognitivo de arranque, las instrucciones de generación de skills, la salud, la
  fase del pipeline, el batch multi-agente, los conflictos, el recordatorio de captura y las dos
  superficies del PreToolUse (memoria de código y errores conocidos). El proyecto creció en superficies
  de contexto pero el ledger seguía mirando solo una: "no podés optimizar lo que no medís". Es el
  cimiento de la evolución del sistema de tokens (medir antes de optimizar, misma disciplina que Track 7).

### Notes
- La contabilidad se centraliza en el punto de **ensamblado** de cada hook (`assembleAccounted`), que
  estima el texto FINAL de cada bloque —header, ids y formato incluidos, que es la huella real que entra
  al contexto— en vez de que cada builder contabilice por su cuenta (la mayoría no lo hacía). Sigue siendo
  model-free y determinista (`EstimateTokens`). Nuevas superficies en el ledger: `startup_health`,
  `startup_cognitive`, `startup_skillgen`, `turn_phase`, `turn_batch`, `turn_conflicts`,
  `capture_reminder`, `precheck_code`, `precheck_telemetry` (se suman a `startup_priming`, `turn_recall`,
  `hydration`, `code_recall`). `startup_priming`/`turn_recall` pasan a medirse sobre el bloque final
  (antes solo el contenido de los gists, sub-reportando el header).

## [0.44.0] - 2026-06-22

### Changed
- **Mejor ranking del catálogo cosechado: tope de skills por repo** (Track 8 / T8.5): el cosechador
  (`musubi catalog harvest`) ahora acota cuántas skills aporta un mismo repo de GitHub (flag
  `--max-per-repo`, default **3**). Las estrellas que reporta el marketplace son del **repo**, no de
  la skill, así que un monorepo enorme y muy estrellado (ej. `openclaw/openclaw` con 379k) inundaba el
  top con skills mediocres y tapaba otras más enfocadas. Con el cap se conservan las N mejores de cada
  repo, dejando lugar a más variedad y relevancia. `--max-per-repo 0` desactiva el tope.

### Notes
- `HarvestMarketplace` aplica el cap sobre la lista ya ordenada por estrellas (se queda con las N de
  mayor ranking por repo). `repoKey` extrae `owner/repo` de la URL de GitHub. Tests: cap por repo,
  modo sin tope, y extracción de `repoKey`.

## [0.43.1] - 2026-06-22

### Fixed
- **`updatedAt` del marketplace tolera número o string** (Track 8): el endpoint de skillsmp
  devuelve `updatedAt` a veces como string (`"1781667763"`) y a veces como número JSON
  (`1781667763`). El struct lo esperaba string, así que una sola entrada con formato numérico
  hacía fallar el decode de **toda la respuesta de esa seed** → en la cosecha real se perdían
  seeds enteras (Go y Node.js, las más importantes). Ahora un tipo tolerante (`flexString`)
  normaliza ambos a string. Detectado al generar el catálogo inicial de producción.
- **El Action de cosecha baja el binario del release en vez de `go install`** (`deploy/musubi-skills/`):
  el `go.mod` declara el módulo como `musubi` (no la URL de GitHub), así que `go install
  github.com/codeabraham16/musubi/cmd/musubi@latest` falla ("module declares its path as: musubi").
  El workflow ahora descarga `musubi-linux-amd64` del último release con `gh release download`.
  Detectado al correr el Action central por primera vez.

## [0.43.0] - 2026-06-22

### Added
- **`musubi_discover_skills` lee un catálogo estático por default** (Track 8 / T8.4, cierra el ciclo):
  el descubrimiento ya **no pega a la API del marketplace en vivo** salvo como fallback. Sirve desde un
  catálogo **curado y estático** (`marketplace-index.json` publicado por el cosechador central),
  cacheado con TTL → **cero rate limit para el usuario** (el límite de 50/día deja de aplicar). Si el
  catálogo no está configurado o no está disponible, cae con gracia a la API en vivo (transición sin
  fricción mientras el archivo aún no existe). La respuesta incluye `"source": "catalog" | "live"`.
- Config `sourcing.marketplace_catalog_url` (default: el `marketplace-index.json` en el repo
  `musubi-skills`). `skillsource.FetchMarketplaceCatalog` (lee el catálogo estático) y
  `skillsource.FilterMarketplaceSkills` (filtra local por query: algún término en nombre/desc/id,
  preservando el orden por estrellas).
- **Workflow del cosechador central** en `deploy/musubi-skills/` (`harvest.yml` + `README.md`): un
  GitHub Action listo para copiar al repo `musubi-skills` que corre `musubi catalog harvest`
  semanalmente (con `SKILLSMP_API_KEY` como secret) y publica el catálogo. Es lo que hace que un solo
  cosechador toque la API y todos los usuarios lean el archivo estático.

### Notes
- Con esto el plan de "las 3 palancas" queda cerrado: API key (T8.1) + caché (T8.2) son el pipeline de
  ingesta que alimenta el catálogo cosechado (T8.3) que se sirve estático (T8.4). El modo live persiste
  como fallback y para `marketplace_catalog_url` vacío.
- Tests: `discover_skills` desde catálogo estático (no toca la API live) y fallback a live cuando el
  catálogo falla; `FetchMarketplaceCatalog` (parseo + error no-fatal) y `FilterMarketplaceSkills`.

## [0.42.0] - 2026-06-22

### Added
- **Cosechador del marketplace** (Track 8 / T8.3, Palanca 3): nuevo subcomando
  **`musubi catalog harvest`** que arma un **catálogo estático** de Agent Skills del marketplace,
  curado por *seeds* (stacks/keywords) y estrellas. La idea del trayecto: en vez de que cada usuario
  pegue a la API en vivo (y choque con el rate limit de 50/día anónimo), un cosechador central corre
  de vez en cuando y publica este JSON; el descubrimiento lo leerá de un archivo (cero rate limit,
  llega en T8.4). No se mirrorea el 1.7M: se cura un subconjunto por relevancia y popularidad.
  Flags: `--seeds a,b,c` (default: Go, Python, Node.js, Rust, …), `--top N` por seed, `--min-stars N`,
  `--out ruta`, `--api-key-env NOMBRE` (default `SKILLSMP_API_KEY`; vacío ⇒ tier anónimo), `--url`.
- **`skillsource.HarvestMarketplace`**: núcleo cosechable y testeable — recibe un `fetch` inyectable
  (sin acoplar a la red), consulta cada seed, **deduplica por id** (gana la de más estrellas), filtra
  por `min-stars` y ordena por estrellas desc (desempate estable por id). Best-effort: una seed que
  falla se omite con warn y la cosecha sigue. `MarketplaceCatalog` es el formato de salida
  (`version`, `generated`, `seeds`, `skills`); el timestamp lo setea el CLI (núcleo determinista).

### Notes
- El cosechador usa **solo metadatos de skillsmp** en esta etapa (id/name/description/githubUrl/stars);
  la validación/enriquecimiento contra GitHub como fuente de verdad queda para un PR siguiente. El
  `discover_skills` sigue en vivo por ahora; T8.4 lo conmuta a leer el catálogo estático por default.
- Un ejemplo del formato vive en `internal/skillsource/testdata/marketplace-index.example.json`
  (validado por test). Escritura **atómica** (temp + rename) reusando el patrón de `catalog merge`.

## [0.41.0] - 2026-06-22

### Added
- **Caché de sourcing con TTL** (Track 8 / T8.2): las respuestas de red del sourcing de skills
  —catálogo curado (`musubi_search_skills`) y marketplace (`musubi_discover_skills`)— se cachean en
  memoria con TTL = `sourcing.cache_seconds` (default 3600s). Una query repetida sale del caché en vez
  de pegar de nuevo a la red: como la query de descubrimiento sin argumentos se deriva del stack y es
  **estable**, esto convierte N llamadas en 1 fetch + (N-1) hits locales, **preservando el rate limit**
  del marketplace (el tier anónimo es de 50/día). Es además la base de ingesta del futuro cosechador
  del catálogo (un harvest re-consulta lo mismo entre corridas; el caché le ahorra presupuesto de API).
  Solo se cachean fetches exitosos (un error transitorio reintenta). `cache_seconds: 0` lo desactiva.

### Notes
- El caché (`sourcingCache`) es seguro para concurrencia: las tools de sourcing son read-only y se
  despachan en paralelo bajo RLock, así que el caché se protege con su propio mutex (limpieza perezosa
  de entradas vencidas). Tests: hit/miss, expiración, modo inerte, y que dos `discover_skills` con la
  misma query pegan al marketplace una sola vez.

## [0.40.0] - 2026-06-22

### Added
- **`musubi_discover_skills`** (Track 8 / T8.1, tool nº27): descubre **Agent Skills** (formato
  SKILL.md) de la comunidad en un marketplace externo (por defecto skillsmp.com, ~1.7M skills
  indexadas de GitHub público), **filtradas por el stack del proyecto**. El marketplace tiene escala
  pero no conoce tu proyecto; Musubi aporta la pieza que falta: si no pasás `query`, la deriva del
  stack detectado (ecosistemas + frameworks). Es un canal **separado** del catálogo curado
  (`musubi_search_skills`) y deliberadamente **solo de descubrimiento**: devuelve metadatos + el
  `githubUrl` de cada skill para que el usuario los **revise e instale por su cuenta**. Musubi nunca
  baja, ejecuta ni instala el SKILL.md (contenido no confiable de GitHub arbitrario; el propio
  marketplace avisa "revisá el código antes de instalar"). Read-only.
- **`skillsource.FetchMarketplaceSkills`**: cliente del endpoint de búsqueda del marketplace
  (`GET /api/v1/skills/search`), con el mismo patrón que `FetchCatalog` (timeout por contexto,
  backstop anti-DoS de tamaño, degradación graciosa). Acota `limit` a [1,100], ordena por estrellas
  y, si hay API key, la envía como `Authorization: Bearer` (sube el rate limit; sin key usa el tier
  anónimo). Omite entradas sin `id` o sin `githubUrl`.
- Config: `sourcing.marketplace_enabled` (bool, **default false: opt-in**), `sourcing.marketplace_url`
  (default `https://skillsmp.com`) y `sourcing.marketplace_api_key_env` (NOMBRE de la env var con la
  API key; el secreto no se guarda en el yaml, mismo criterio que `embedding.api_key_env`).

### Notes
- **Por qué opt-in y solo descubrimiento**: indexar 1.7M SKILL.md de GitHub arbitrario es contenido
  no confiable. Mantenerlo apagado por defecto y limitar a *recomendar + enlazar* (nunca instalar)
  preserva las invariantes de Musubi: local-first (degradación graciosa, red opcional), model-free y
  el modelo de confianza "revisá antes de instalar". No se mergea al gate de aplicabilidad (Hermes):
  el marketplace no expone triggers/capabilities, así que se trata como canal aparte.
- Tests: parseo/mapeo del adapter, armado del request (path, query escapada, `limit` acotado,
  `Authorization` con/sin key), degradación (HTTP≠200, JSON inválido, `success=false`); a nivel tool:
  deshabilitado→guía, query derivada del stack, query explícita con prioridad, marketplace caído→texto.

## [0.39.0] - 2026-06-22

### Changed
- **Mantenimiento ~9× más rápido y 18× menos memoria a escala** (Track 7 / T7.1): un harness de
  benchmarks de escala (`internal/memory/bench_test.go`) reveló que `Maintain` escalaba de forma
  cuadrática (10k observaciones: **37.5s y 3.27 GB**), y el profiler ubicó el cuello real en
  `Consolidate`: el conteo de solapamiento de trigramas reconstruía un `map[int]int` por cada
  observación (el 56% del tiempo se iba en `mapassign`). Como los índices de canónicos son densos, se
  reemplazó ese mapa por un **slice reutilizado** (`overlap []int` + lista de tocados para resetear en
  O(tocados)). Resultado, **a igualdad de resultado** (mismos tests): Maintain 10k baja a **3.97s y
  181 MB** (9.4× / 18×). La super-linealidad asintótica residual (las postings de trigramas crecen con
  n) queda para T7.2 como problema de *set-similarity-join*, con sus propios tests de equivalencia.

### Added
- **`(*ivfIndex).RemoveBatch(ids)`**: saca un lote de observaciones del índice vectorial bajo un único
  `Lock`, agrupando por celda y filtrando cada celda tocada una sola vez (O(celdas tocadas) en vez de
  O(borrados × celda) del loop de `Remove`). Idempotente con ids ausentes o repetidos; deja el índice
  en el mismo estado que llamar `Remove` uno por uno (test de equivalencia). La consolidación, el decay
  y la purga del mantenimiento lo usan en lugar del loop, para no re-tomar el lock por cada id cuando
  hay embeddings. La correctitud del recall ya la garantiza el re-filtro SQL del engine.
- **Job de CI `bench-guard`**: corre `BenchmarkMaintain` a 1k y 10k y falla si la **memoria asignada**
  escala de forma cuadrática (ratio B/op(10k)/B/op(1k) > 20). Se mide memoria y no tiempo a propósito:
  es determinista y estable en runners compartidos. Atrapa una regresión al patrón O(n²) sin falsos
  positivos por ruido de scheduler.

### Notes
- `bench_test.go` usa datasets sintéticos deterministas (seed fija), sin red ni embeddings reales, solo
  stdlib: mide cómo escala el motor (save, recall léxico/híbrido, FTS, vector, Maintain, prime) sin deps
  nuevas. Es la base de medición de Track 7.

## [0.38.0] - 2026-06-20

### Changed
- **`.mcp.json` y hooks portables** (sobreviven a formateos, cambios de usuario y clones en otra
  máquina): `musubi setup` ya no hardcodea la ruta absoluta del binario ni del proyecto para Claude
  Code. El `command` del server se escribe como `${MUSUBI_BIN:-<ruta>}` (resoluble por la env var
  `MUSUBI_BIN`, con la ruta actual como fallback) y se **omite** `MUSUBI_HOME`: el daemon toma la raíz
  del proyecto de `CLAUDE_PROJECT_DIR`, que Claude Code inyecta automáticamente en el entorno del
  server. Los hooks invocan `musubi` por PATH cuando está instalado global. Resultado: el `.mcp.json`
  se vuelve commiteable y no se rompe al reinstalar o mover el proyecto. Cursor y otros agentes que no
  expanden `${VAR}` mantienen rutas absolutas (`AgentTarget.PortableConfig`).
- El instalador **global** (doble-clic, `install.ps1`, `install.sh`) ahora exporta `MUSUBI_BIN` con la
  ruta del binario instalado, además del PATH: al reinstalar tras un formateo, **todos** los proyectos
  con `.mcp.json` portable vuelven a resolver el binario sin tocar ninguno.

### Added
- `workspaceDir` resuelve la raíz con la cadena `MUSUBI_HOME → CLAUDE_PROJECT_DIR → cwd`.
- `AgentTarget.PortableConfig` distingue agentes que soportan config portable (Claude Code) de los que
  no (Cursor).

### Notes
- Tests: `.mcp.json` portable vs absoluto; `workspaceDir` con `CLAUDE_PROJECT_DIR` y su prioridad.

## [0.37.0] - 2026-06-19

### Added
- **`musubi_insights`** (Track 6 / T6.4, cierra Track 6): tool read-only que resume de un vistazo lo
  que Musubi aprendió del proyecto — tamaño de la memoria (observaciones totales / activas /
  archivadas), **hotspots** de archivos con más errores no resueltos, decisiones de skills
  (aceptadas / rechazadas por su decisión más reciente, last-write-wins), último mantenimiento y
  **salud** del ciclo. Es la cara "dashboard" de la observabilidad activa: todo agregación
  SQL/aritmética determinista, sin LLM.
- `(*DbEngine).Insights` + `InsightsReport` (en la interfaz `Insighter` de `StorageBackend`). La tool
  cuenta como tool nº26, clasificada **read-only** (corre concurrente bajo RLock).

### Notes
- Tests: `TestInsights` (observaciones activas/archivadas, errores+hotspots, decisiones last-wins);
  guard de clasificación read-only y golden de `tools/list` actualizados.

## [0.36.0] - 2026-06-19

### Added
- **Surfacing proactivo de errores conocidos** (Track 6 / T6.3): el hook `precheck` (PreToolUse Read)
  ahora, ANTES de que el agente lea un archivo, también surfacea los **errores no resueltos** que
  Musubi tiene registrados de ESE archivo (telemetría), con su `id` y el fix sugerido. "Este archivo
  ya te dio este error, este fue el fix" — sin que el agente lo pida. Se combina con el aviso de
  memoria de código existente; acotado a los 3 errores más recientes para no inundar el contexto.
  - Reusa `GetUnresolvedTelemetryLogsForFiles` (T6.2). El hook sigue siendo best-effort y model-free.

### Changed
- `precheckOutput` se refactorizó en `codeMemoryMessage` + `telemetryMessage` (combina ambas
  superficies); el interfaz `codeStore` del hook ahora también lee telemetría por archivo.

### Notes
- Test: `TestPrecheckSurfacesKnownErrors` (surfacea error + id + fix sugerido).

## [0.35.0] - 2026-06-19

### Changed
- **Telemetría relevante en `musubi_resolve_skills`** (Track 6 / T6.2): en vez de devolver TODA la
  telemetría no resuelta, ahora devuelve solo los errores de los **archivos que el agente está
  tocando** (`modified_files`), matcheando por ruta completa o por nombre base (tolera prefijos y
  separadores `\`/`/` distintos). El error que viste antes en *este* archivo se surfacea; el ruido del
  resto no.

### Added
- `GetUnresolvedTelemetryLogsForFiles(files)` en el motor (+ interfaz `TelemetryStore`): lookup de
  errores no resueltos por archivo, reusable por el hook proactivo (T6.3).
- `TestGetUnresolvedTelemetryLogsForFiles`: match por ruta/basename, exclusión de resueltos, vacío.

## [0.34.0] - 2026-06-19

### Changed
- **`musubi_search_skills` aprende de las decisiones** (Track 6 / T6.1, abre la observabilidad
  activa): el listado de candidatos ahora **excluye las skills que el usuario ya rechazó**
  (`musubi_log_skill_decision` con `decision: rejected`). Cierra el lazo de aprendizaje pasivo: hasta
  ahora `skill_decisions` se escribía pero nadie la consumía, así que una skill rechazada se
  re-proponía en cada sesión.
  - **Last-write-wins**: una skill rechazada y luego aceptada vuelve a proponerse. Matchea por `id`
    (slug), la misma clave que `log_skill_decision`. Best-effort: si la lectura de decisiones falla,
    el listado se devuelve sin filtrar (nunca rompe la búsqueda).

### Added
- `TestExcludeRejectedSkills` (+ caso sin decisiones): valida la exclusión y el last-write-wins.

## [0.33.0] - 2026-06-19

### Added
- **Persistencia del índice IVF (arranque caliente)** (Track 5 / T5.8, cierra Track 5): el índice
  vectorial se serializa a un snapshot binario `<db>.vindex` (magic + dim + centroides, `encoding/binary`
  stdlib) tras cada rebuild. Al arrancar, si el snapshot es válido se **restauran los centroides y se
  reasignan los vectores activos saltando k-means** (el costo caro), en vez de re-entrenar desde cero.
  - El `.vindex` es un **caché derivado y reconstruible**: ante cualquier problema (ausente, corrupto,
    o incompatible) se cae al rebuild normal — nunca panic ni bloqueo de arranque, nunca compromete
    correctness (el engine re-filtra y re-rankea exacto).
  - **Endurecido por revisión adversarial** (16 agentes, 0 críticos/altos): escritura **atómica**
    (tmp + `os.Rename`, sin `.vindex` truncado ante crash); **guard de `k`** que descarta el snapshot
    si la cantidad de centroides diverge >2× de la natural para el `n` actual (dataset que cambió de
    tamaño entre sesiones → evita degradar el recall con `NProbe` fijo); validación de dim (drift de
    modelo) y de cotas (archivo corrupto no dispara asignaciones gigantes).

### Notes
- Tests: `TestVectorIndexWarmStart` (warm-start == rebuild), `TestVectorIndexWarmStartRejectsStaleK`,
  `TestVectorIndexWarmStartDimMismatch`, `TestIndexSnapshotRoundTrip`, `TestReadIndexSnapshotRejectsCorrupt`.
- Limitación conocida documentada: el snapshot no detecta un cambio de modelo de embeddings de la
  misma dimensión (se refresca en el próximo rebuild; agregar un fingerprint cruzaría la capa
  "model-free" del motor). `scoreCandidates`/`targetCentroidCount` ahora compartidos para no divergir.

## [0.32.0] - 2026-06-19

### Added
- **Recall híbrido** (Track 5 / T5.7 R2, la pieza de mayor impacto de la ola): cuando hay un proveedor
  de embeddings, `musubi_recall` suma un **pool de candidatos por similitud vectorial** (coseno) al
  pool léxico (FTS), **unidos por id** (union, no intersección), y agrega una **4ta señal RRF** por
  rango vectorial. Así una consulta como "fixed N+1 query" puede recuperar "database performance
  regression" aunque no compartan palabras. La query se embebe en la capa MCP (best-effort: si el
  embedder falla, el recall sigue 100% léxico).
- `augmentWithVectorPool` + `candidatesByIDs` en el motor; `RecallOptions.QueryVector`.

### Changed
- `scoreCandidates` suma el término vectorial detrás de `vecRank` (mismo patrón que `lexRank`).
  **Sin proveedor de embeddings (`NoopProvider`) el comportamiento es idéntico al histórico** —
  `QueryVector` vacío ⇒ `vecRank` nil ⇒ recall 100% léxico.

### Notes
- Tests: `TestRecallHybridUnionViaVector` (el pool vectorial trae una obs sin match léxico),
  `TestScoreCandidatesVectorSignal`. Cierra T5.7 (el slice de mayor impacto y riesgo de Track 5).

## [0.31.0] - 2026-06-19

### Changed
- **Recall multi-pool** (Track 5 / T5.7 R1, prepara el recall híbrido): `recallCandidates` devuelve
  ahora el ranking keyword (`lexRank`, id→posición) por separado, y `scoreCandidates` toma mapas de
  rank por pool en vez de derivar el rango keyword del orden del slice. Un candidato ausente de un
  pool simplemente no suma ese término RRF. Esto deja listo unir la señal vectorial (R2) sin
  ambigüedad de rangos.
  - **Bit-idéntico al histórico** con `NoopProvider` (solo el pool léxico): toda la batería de tests
    de recall existente pasa sin cambios de comportamiento. `lexRank` nil (fallback por recencia)
    omite el término keyword igual que antes.

### Added
- `TestScoreCandidatesLexRankEquivalence`: garantiza que `lexRank` por orden de slice == el viejo
  `keywordMeaningful=true`, y que nil / id ausente omite el término keyword.

## [0.30.0] - 2026-06-19

### Changed
- **FTS ponderado por IDF-aproximado** (Track 5 / T5.6, abre la ola de recall): nueva
  `buildFTSQueryRanked` que descarta el ruido que diluye el `OR` del `MATCH` — stopwords (lista
  determinista es/en) y tokens de una sola runa (p. ej. la `N` y el `1` de `N+1`) — pero **preserva
  entidades cortas** significativas (`Go`, `DB`, `API`). Si la consulta es toda ruido, cae a
  `buildFTSQuery` para no perder recall. Proxy de IDF determinista, sin LLM.
  - Adoptada en `conflictCandidates` (detección de conflictos) y `EntityContext` (grafo): menos
    ramas `OR`, candidatos más limpios. El path de `musubi_recall` se mantiene en `buildFTSQuery`
    hasta el recall híbrido (T5.7), para no calibrar el RRF sobre un pool que aún cambia.

### Added
- `TestBuildFTSQueryRanked`: descarta stopwords y tokens de 1 runa, preserva `Go`/`DB`/`API`,
  fallback no vacío ante consulta toda de ruido.

## [0.29.0] - 2026-06-19

### Changed
- **Olvido reversible** (Track 5 / T5.5, cierra la ola de autonomía): la consolidación de
  casi-duplicados ahora **archiva** el duplicado (soft-delete: `archived=1` + `archived_at` +
  `superseded_by` al canónico) en vez de **borrarlo físicamente**. Queda oculto del recall pero
  **recuperable**; el borrado definitivo lo hace `PurgeArchived` tras el período de gracia de
  retención (que limpia relaciones y embeddings). Así una fusión por falso positivo de trigramas no
  pierde datos.
- **Decay paginado**: el olvido escanea por **keyset paginado** (`id > lastID`) en vez de cargar todo
  el set activo en RAM, acotando la memoria en bases grandes. La saliencia se sigue computando en Go
  con la **misma fórmula** (no se movió a SQL): el conjunto archivado es **idéntico** al histórico,
  sin riesgo de regresión por diferencias de float/timestamps.

### Added
- **Protección por importancia en el decay**: `maintenance.decay_protect_importance` (float, default 0
  = off). Las observaciones con `importance >=` a ese valor (conocimiento deliberado: decisiones,
  arquitectura) **no se auto-archivan** por más viejas/frías que estén. Nota: Musubi no tiene columna
  `type`; la protección usa `importance`, la señal de "conocimiento deliberado" del esquema real.
- Tests: `TestDecayPaginationEquivalence` (paginado == una-pasada, garantía de no-regresión),
  `TestDecayProtectsHighImportance`, `TestConsolidateSoftDeletesDuplicate`.

## [0.28.0] - 2026-06-19

### Added
- **Auto-curación en el ciclo de mantenimiento** (Track 5 / T5.4): el scheduler de fondo ahora también
  se auto-cura. Tras cada mantenimiento corre `AutoHeal`: diagnostica y **repara automáticamente solo
  los checks de bajo riesgo** (`fts_consistency`, `missing_digests`, `orphan_relations`) en modo apply
  (con backup previo). `db_integrity` y `schema_migrations` quedan **fuera a propósito**: se reportan,
  no se auto-aplican.
- **Salud surfaceada en el arranque**: `AutoHeal` persiste el último `DiagnoseReport` (post-repair) en
  meta (`last_health`); el hook `SessionStart` lo lee (lectura barata, no re-diagnostica) e inyecta una
  advertencia con los problemas **no auto-reparables** si la base no está sana. Si está sana, silencioso.
- `(*DbEngine).AutoHeal` (+ en la interfaz `Doctor`), `buildHealthContext` en el hook de arranque.
- Tests: `TestAutoHealRepairsLowRisk`, `TestHealthContextSurfacesIssues`.

## [0.27.0] - 2026-06-19

### Added
- **Trigger de mantenimiento por volumen de saves** (Track 5 / T5.3): además del ticker temporal de
  T5.2, el daemon dispara ahora un mantenimiento tras `maintenance.auto_after_saves` saves
  (observaciones / hechos / código), para que una sesión intensa no espere al próximo tick. Es
  **opt-in**: `0` = desactivado (default).
  - El disparo es **async** (goroutine): el handler de save ya sostiene el write-lock de `dispatchMu`,
    así que correr el ciclo inline lo re-entraría (deadlock); la goroutine toma el lock al liberarse.
    Respeta el throttle (`MaintenanceDue`) y mantiene **un solo ciclo en vuelo** (`atomic.Bool` CAS);
    el contador es un `atomic.Int64` que se resetea al disparar.
  - Nuevo campo de config `maintenance.auto_after_saves` (int, default 0).
- `TestAutoMaintainAfterSaves`: verifica que cruzar el umbral dispara el mantenimiento y que por
  debajo no.

## [0.26.0] - 2026-06-19

### Added
- **Scheduler de auto-mantenimiento de fondo** (Track 5 / T5.2, corazón de la ola de autonomía): el
  daemon corre ahora el ciclo de mantenimiento (consolidar + olvidar + purgar + compactar) de forma
  recurrente vía un `time.Ticker`, no solo una vez al arrancar. Un daemon long-running se mantiene
  solo, sin necesidad de reinicio.
  - La corrida de arranque pasó a una goroutine best-effort: un `VACUUM` grande ya **no bloquea** el
    primer pedido del daemon.
  - El ticker y la corrida de arranque se **serializan contra el dispatch de tools** tomando el
    write-lock del server (`dispatchMu`, de T4.5) y respetan el throttle de T5.1 (`MaintenanceDue`).
    El ciclo se detiene limpio en el shutdown (cancelación de contexto por señal o EOF de stdin).
  - Métodos nuevos del server: `RunScheduledMaintenance` (una corrida throttled, bajo lock) y
    `RunMaintenanceScheduler` (loop por ticker hasta cancelar el contexto).
- `TestMaintenanceSchedulerRunsAndStops` (corre bajo `-race` en CI: ticker + dispatch concurrente de
  lecturas y escrituras contra el lock exclusivo del mantenimiento) y
  `TestRunScheduledMaintenanceThrottle`.

## [0.25.0] - 2026-06-19

### Changed
- **Throttle + `force` en `musubi_maintain`** (Track 5 / T5.1, abre la ola de autonomía del daemon):
  la tool consulta ahora el throttle del auto-mantenimiento (`MaintenanceDue`) antes de correr. Si el
  último mantenimiento fue hace menos del intervalo configurado (`maintenance.auto_interval_hours`),
  devuelve un no-op informativo (`{skipped: true, reason, last_maintenance}`) en vez de re-disparar
  consolidación + VACUUM. Pasá `force: true` para ignorar el throttle (mantenimiento on-demand
  explícito). Tras correr, marca `last_maintenance`.
  - Protege contra que un agente dispare el ciclo en loop, y establece el contrato `force` que
    reusará el scheduler de fondo (T5.2). `auto_interval_hours: 0` ⇒ sin throttle (siempre corre).
- `musubi_doctor` expone ahora `last_maintenance` para visibilidad del estado del ciclo, sin cambiar
  el contrato `DiagnoseReport` (el campo se suma; los existentes se preservan).

### Added
- `TestMaintainThrottleAndForce` y `TestDoctorExposesLastMaintenance`: guardas del throttle, del
  override por `force` y de la visibilidad de `last_maintenance`.

## [0.24.0] - 2026-06-19

### Changed
- **Concurrencia de lectura en el transporte HTTP** (Track 4 / T4.5): el dispatch ahora usa un
  `sync.RWMutex` y clasifica cada tool por si muta estado. Las **7 tools de solo-lectura**
  (`search_semantic`, `search_keyword`, `recall_facts`, `entity_context`, `conflicts`,
  `detect_stack`, `search_skills`) corren **concurrentes entre sí** (RLock); las que mutan toman el
  lock exclusivo (serializadas, sin lost-updates de read-modify-write). Se removió la serialización
  global del handler HTTP: peticiones de lectura concurrentes ya no se encolan detrás de una sola.
  - La clasificación es **fail-safe**: una tool es de-escritura por defecto; solo se marca
    `readOnly` tras verificar que no escribe DB, ni índice, ni ledger, ni hace `bumpAccess`. (Por eso
    `recall`/`memory_expand`/`recall_code` quedan como escritura: bumpean acceso o registran tokens.)
  - El modo stdio (un goroutine) no cambia: el RWMutex queda siempre libre, costo nulo.

### Added
- `TestToolReadOnlyClassification`: congela el conjunto exacto de tools de solo-lectura y es un guard
  de regresión contra marcar como `readOnly` una tool que muta (bug RMW que `-race` no detecta).
  `TestConcurrentReadDispatch`: dispara tools de lectura en paralelo (corre bajo `-race` en CI).

## [0.23.0] - 2026-06-19

### Added
- **Modo servicio: observabilidad** (Track 4 / T4.4, **cierra el track de modo servicio**). Endpoints
  operativos en el transporte HTTP, todo stdlib (+ el `uuid` ya presente), cero dependencias nuevas:
  - **`GET /healthz`** — liveness (200 si el proceso responde). Sin auth.
  - **`GET /readyz`** — readiness: sondea el motor con una lectura barata (`GetMeta`); 200 si responde,
    503 si no, para que un orquestador no rutee tráfico hasta que la DB esté lista. Sin auth.
  - **`GET /metrics`** — contadores en formato texto Prometheus (`musubi_http_requests_total` por
    resultado: ok / client_error / unauthorized / server_error). Detrás de auth si hay token (datos
    operativos); abierto en loopback sin token.
  - **Correlation IDs**: cada request al MCP recibe un `X-Request-Id` (el entrante si viene, o uno
    nuevo) que se devuelve en la respuesta, para trazar peticiones extremo a extremo.

## [0.22.0] - 2026-06-19

### Added
- **Modo servicio: autenticación, bind remoto y TLS** (Track 4 / T4.3). Habilita exponer el
  servidor MCP más allá de loopback, de forma segura:
  - **Bearer token** (`service.auth_token_env`): nombra una variable de entorno con el token (nunca
    en el YAML, patrón de `embedding.api_key_env`). Si hay token, todo request exige
    `Authorization: Bearer <token>`, comparado en **tiempo constante** (`crypto/subtle`).
  - **Gating de bind**: un `service.addr` **no-loopback exige token** — `musubi serve` se niega a
    arrancar si no lo hay. El bind loopback puede seguir sin auth (default de desarrollo) con la
    defensa anti DNS-rebinding (Host + Origin) ya existente.
  - **TLS opcional** (`service.tls_cert_file` + `service.tls_key_file`): si ambos están, sirve HTTPS.
    Un bind remoto sin TLS **avisa** que el token viaja en texto plano (no bloquea: un proxy que
    termina TLS es válido).
  - La defensa anti DNS-rebinding (Host loopback + Origin local) aplica solo en modo loopback; en
    remoto el token es el gate (los checks de Host romperían clientes legítimos).
- Tests: auth requerido/aceptado/rechazado, `resolveServiceAuth` (matriz loopback × token), y
  `validBearer` (prefijo/trim/constant-time). Cero dependencias nuevas (`crypto/subtle`, stdlib).

### Security
- Endurecimientos fail-closed (de una revisión de seguridad adversarial de la superficie remota):
  - `auth_token_env` nombrada pero con la env var vacía/ausente ahora es **error de arranque** (antes
    deshabilitaba la auth en silencio, contra la intención del operador).
  - Config TLS medio-seteada (solo `tls_cert_file` o solo `tls_key_file`) es **error** (antes
    degradaba a HTTP en texto plano en silencio).
  - Bind remoto con token pero **sin TLS** ahora **falla** salvo `service.allow_insecure_token: true`
    explícito (para deploys con un proxy que termina TLS). Antes solo avisaba.
  - Piso de TLS pineado explícitamente a 1.2 (`tls.Config{MinVersion}`).

## [0.21.0] - 2026-06-19

### Added
- **Modo servicio: transporte HTTP** (Track 4 / T4.2). Nuevo subcomando `musubi serve` que expone
  el servidor MCP sobre HTTP (`POST /mcp`, JSON-RPC 2.0) además del stdio por defecto. Mismo dispatch,
  mismas tools, misma config del motor — corre sobre el seam `Dispatch` de v0.20.0.
  - **Opt-in y seguro por defecto**: bloque de config `service:` con `enabled: false` por defecto; un
    workspace existente sin ese bloque no abre ningún puerto. `musubi serve` se niega a arrancar sin
    `service.enabled: true` (o un `--addr host:port` / `--enable` explícito).
  - **Solo loopback en este release**: bind a `127.0.0.1:7717` por defecto; un `addr` no-loopback es
    error de arranque (la autenticación y el bind remoto llegan en el próximo slice). Defensa de
    superficie: validación de `Host` loopback + rechazo de `Origin` cross-site (mitiga DNS-rebinding),
    techo de body (4 MiB), y timeouts de lectura/escritura/idle contra slow-loris.
  - **Concurrencia serializada**: las peticiones HTTP se serializan sobre un mutex (línea base segura,
    sin riesgo de read-modify-write en el motor). La concurrencia real es un slice posterior, tras la
    auditoría RMW; el seam `Dispatch` ya la deja lista.
  - `GET /mcp` (upgrade SSE) reservado (405): Musubi no emite mensajes server-initiated todavía.
  - **Cero dependencias nuevas**: todo `net/http` + stdlib.
- Tests del transporte HTTP (`http_test.go`): tools/list, initialize, tool-call, notificación→202,
  errores parse/method, `GET`→405, rechazo cross-origin, rechazo de bind no-loopback, y la tabla de
  `isLoopbackHost`.

## [0.20.0] - 2026-06-19

### Changed
- **Seam de dispatch** (Track 4 / T4.1, **abre el track de modo servicio**): se extrajo
  `(*McpServer).Dispatch(ctx, req) (JsonRpcResponse, bool)` del viejo `handleRequest`. Ahora el
  dispatch **devuelve** la respuesta en vez de escribirla a un campo compartido `s.out`; cada
  transporte serializa su propia escritura (`writeResponse(out, resp)`). Esto **elimina el único
  hazard de memoria** del servidor (la mutación de `s.out` + `send`) y deja a `Dispatch` seguro para
  llamarse concurrentemente — el prerequisito para los transportes de red de Track 4 (HTTP en v0.21.0).
  - El modo stdio (`musubi daemon`) queda **idéntico en comportamiento**: un goroutine, secuencial,
    60s por request, shutdown graceful. Solo cambió la plomería interna.
  - `Dispatch` lee únicamente estado fijado en `NewMcpServer` (registro de tools, motor, embedder,
    config) y no muta nada compartido; los handlers no escriben campos del servidor.

### Added
- Test de concurrencia `TestDispatchConcurrentSafe`: 64 goroutines disparan lecturas y escrituras
  en paralelo contra un servidor + motor compartidos (saves que ejercitan el `Add` al índice IVF y
  el rebuild en background, búsquedas que toman el RLock, `tools/list`). Corre bajo `-race` en CI
  como red de seguridad permanente de la concurrencia.

## [0.19.0] - 2026-06-19

### Added
- **Interfaz `StorageBackend`** (Track 3 / T3.2): el contrato completo que un backend de memoria
  debe cumplir para servir a la app (servidor MCP + CLI). `*memory.DbEngine` (SQLite local-first,
  puro Go, model-free) es la implementación de referencia; un backend alternativo —p.ej. el modo
  servicio de Track 4— implementa la misma interfaz **sin que los consumidores cambien**. Es el seam
  de extensibilidad de Track 3.
  - Compuesta de interfaces de rol chicas (idioma Go: "interfaces chicas, compuestas") —
    `ObservationStore`, `GraphStore`, `RelationStore`, `WorkStore`, `WorkflowStore`, `LedgerStore`,
    `MetaStore`, `PhaseStore`, `Maintainer`, `Doctor`, `Calibrator`, etc. — para que cada consumidor
    dependa solo del subconjunto que usa.
  - `internal/mcp` ahora depende de `memory.StorageBackend`, no de `*memory.DbEngine` concreto.
    Esto **desacopla el layer MCP del motor** y habilita tests de handlers en aislamiento con un
    backend falso (ver `TestStorageBackendSeam_ConflictsViaFake`).
  - Aserción en tiempo de compilación `var _ StorageBackend = (*DbEngine)(nil)`: agregar un método al
    contrato que el motor no implemente —o cambiar una firma— rompe la compilación de inmediato.

### Fixed
- El test golden de `tools/list` ahora normaliza el fin de línea (CRLF→LF) antes de comparar: era
  frágil en working trees de Windows con `git autocrlf` (el repo guarda LF pero el checkout deja CRLF).
  CI (Linux) no se veía afectado; el fix lo hace robusto en cualquier entorno.

## [0.18.0] - 2026-06-19

### Added
- **Registro de tools map-based** (Track 3 / T3.1, **abre el track de velocidad y extensibilidad**).
  Agregar una herramienta MCP exigía mantener sincronizados TRES lugares (el schema en `tools/list`,
  un `case` en el switch de `tools/call`, y un conteo manual en los tests). Ahora cada tool es una
  sola `toolEntry` (`internal/mcp/registry.go`) que liga su schema con su handler; `tools/list` itera
  el registro en orden y `tools/call` resuelve por mapa en O(1). **Agregar una tool = una entrada**.
  Las firmas que no usan el `context` del request se adaptan con `noCtx` sin tocar el cuerpo del handler.
- Test **golden** del catálogo (`TestToolsListGolden` + `testdata/toolslist.golden.json`): congela la
  salida JSON exacta de `tools/list` (nombres, descripciones, schemas y orden) — el refactor quedó
  probado byte-idéntico. Test de **consistencia estructural** (`TestRegistryConsistency`): garantiza que
  la lista de schemas y el mapa de dispatch sean siempre el mismo conjunto (sin tools sin handler ni
  handlers huérfanos).
- **CI endurecido**: `golangci-lint` (gate con `.golangci.yml`: linters estándar + preset de
  manejo de errores idiomático), **piso de cobertura** (CI falla si baja de 70%), `govulncheck`
  (escaneo de vulnerabilidades) y **Dependabot** (módulos Go + GitHub Actions). Antes el CI solo
  corría `vet`/`build`/`test -race`.

### Changed
- El dispatch de `tools/call` pasó de un `switch` de 25 ramas a una búsqueda por mapa
  (`s.toolIndex[name]`); la lista de `tools/list` pasó de un slice hand-mantenido a la iteración del
  registro. Comportamiento idéntico (verificado con el golden + verificación adversarial del binding
  nombre→handler contra el baseline).

### Fixed
- Limpieza de lint: eliminado el `const charsPerToken` muerto; mensajes de error de Ollama en
  minúscula (ST1005); comentarios de paquete en `memory`, `skills`, `mcp` y el comando `musubi`.

## [0.17.0] - 2026-06-19

### Added
- **Retención y compactación de memoria** (Track 1 / T1.3, **cierra el track de cimientos de datos**).
  Acota el crecimiento perpetuo de la base y reclama espacio, manteniéndose local-first y model-free:
  - **Purga dura** (`PurgeArchived`): borra DEFINITIVAMENTE las observaciones archivadas cuyo
    `archived_at` supera la ventana de retención (`maintenance.purge_archived_after_days`, default 90),
    en una transacción que limpia embeddings (FK CASCADE), relaciones semánticas y punteros
    `superseded_by`. El olvido (decay) solo marcaba `archived` sin borrar nunca; esto las elimina.
  - **Compactación física** (`Compact`): `wal_checkpoint(TRUNCATE)` + `PRAGMA optimize` siempre, y
    `VACUUM` tras una purga que borró filas (`maintenance.vacuum`, default true).
  - **`engine.Maintain`** centraliza el ciclo (consolidar → olvidar → purgar → compactar); lo comparten
    el subcomando `maintain`, el auto-mantenimiento del daemon y la tool MCP `musubi_maintain`.
  - Columna `archived_at` (migración v3): la ventana de retención cuenta **desde el archivado**
    (período de gracia), no desde el último acceso.
  - Índice `idx_obs_archived` (migración v2) — primera migración post-baseline, sobre el framework de v0.15.0.

### Changed
- **Consolidación O(n²) → ~O(n)**: índice invertido de trigramas + bucket de igualdad exacta, en vez de
  comparar cada observación contra todos los canónicos. Resultado idéntico al algoritmo previo (verificado
  con un test diferencial); escala a bases grandes.
- Tuning explícito del pool de conexiones SQLite (`SetMaxOpenConns`/`Idle`/`ConnMaxIdleTime`).
- Hidratación de observaciones (`expand.go`) ahora respeta el `context` del caller (variantes `…Ctx`),
  en vez de un `context.Background()` interno que ignoraba el deadline.

### Fixed
- La purga (hard-delete irreversible) **ya no se habilita por un upgrade silencioso**: un config sin bloque
  `maintenance` queda con la purga desactivada; solo se activa con el campo explícito.
- `Decay` trocea su `UPDATE … IN (…)` (antes podía superar el tope de parámetros y abortar el ciclo de
  mantenimiento en bases grandes).
- Al consolidar una observación que era fuente de un `supersede`, los punteros `superseded_by` se
  re-apuntan al canónico (la observación ocultada sigue oculta, no reaparece en el recall).

## [0.16.0] - 2026-06-19

### Added
- **Índice vectorial IVF para búsqueda semántica a escala** (Track 1 / T1.2). Reemplaza el
  full-scan O(n) de la búsqueda semántica (que cargaba y deserializaba **todos** los embeddings
  por query y se degradaba a ~10k observaciones) por un índice invertido por centroides k-means,
  **model-free y en Go puro** (sin dependencias nuevas, sin CGo). Diseño elegido por un panel
  multi-agente (IVF sobre HNSW/SQ8) y validado con verificación adversarial:
  - **No retiene vectores en RAM**: solo centroides + la membresía de cada celda (ids). Footprint
    residente ~10-90 MB incluso a 1M de observaciones; los vectores se cargan de SQLite **solo**
    para las celdas sondeadas.
  - **Exacto por debajo del umbral**: con menos de `exact_threshold` embeddings (o índice sin
    entrenar, o dimensión incompatible) la búsqueda es el full-scan exacto de siempre. Por encima,
    el IVF solo **acota** candidatos y el ranking final sigue siendo coseno **exacto**, re-filtrado
    `archived=0 AND superseded_by IS NULL` contra SQLite: el índice nunca compromete la correctitud
    (a lo sumo, el recall entre rebuilds). Test de regresión exige **recall@10 ≥ 0.92**.
  - k-means++ (sembrado D²) + reseed de centroides muertos; manejo de drift de dimensión
    (entrena con la dim mayoritaria); updates incrementales (`Add`/`Remove`) y re-entrenamiento
    throttled en segundo plano.
  - Bloque de config `vector_index` (`enabled`, `exact_threshold`, `nprobe`, `rebuild_*`, `kmeans_*`).

### Changed
- `internal/memory`: `SearchObservations` ahora despacha entre el camino IVF y el full-scan exacto
  (conservado intacto como `searchExactFullScan`). `saveObservation` mantiene el índice al día
  post-commit; `Decay` y la marca de superseded lo sincronizan.
- Lifecycle del `DbEngine`: `Close()` espera a las tareas de índice en segundo plano antes de
  cerrar la base (evita use-after-close del `*sql.DB`).

## [0.15.0] - 2026-06-19

### Added
- **Esquema versionado con migraciones** (`PRAGMA user_version`): runner que aplica las
  migraciones pendientes, **cada una en su propia transacción** (DDL + bump de versión atómicos;
  si una falla, rollback y la versión no avanza). La migración `baseline` encapsula el esquema
  histórico completo + las columnas de eficiencia de memoria; es idempotente sobre bases
  preexistentes (una base v0.14 solo avanza su `user_version` sin reescribir nada). Track 1 (T1.1)
  del rumbo de escalabilidad perpetua: habilita cambios de esquema NO aditivos (renames, tipos,
  tablas nuevas con backfill) de forma ordenada y resumible, que antes no tenían camino de upgrade.

### Changed
- `internal/memory/database.go`: el esquema (`initSchema`/`migrateObservations`) se refactorizó
  sobre una interfaz `execQuerier` (satisfecha por `*sql.DB` y `*sql.Tx`) para que la migración
  baseline corra dentro de una transacción. Los métodos previos se conservan como wrappers (sin
  cambio de comportamiento para el auto-repair del doctor ni los tests). Los backfills que dependen
  de la versión del estimador de tokens siguen como pasos idempotentes post-migración.

## [0.14.0] - 2026-06-18

### Added
- Soporte multi-agente en `musubi setup`: `--agent <claude|cursor>` registra el servidor MCP
  en la config del agente (`.mcp.json` para Claude, `.cursor/mcp.json` para Cursor). Abstracción
  `AgentTarget` + detección de agentes presentes en el proyecto. Los hooks siguen siendo
  específicos de Claude Code. Track B del roadmap.

## [0.13.0] - 2026-06-18

### Added
- **Motor de orquestación DAG (model-free)** — tool `musubi_workflow` (`start`/`next`/`complete`/`status`/`resume`).
  Musubi define el grafo (`.musubi/workflows/<id>.yaml`), persiste el estado del run en SQLite
  (tabla `workflow_runs`, **resumible entre sesiones**) y devuelve los steps listos; el agente
  ejecuta. Un step queda listo cuando todas sus `needs` están `done` o `skipped`. Tracks A1+A2.
- Control de flujo en workflows: un step puede llevar `when` (expresión model-free, ej.
  `step.build.result contains ok`); si es falsa el step se salta (`skipped`), expresando
  gate/if_then/switch sin tipos de step separados. Evaluador de expresiones seguro (sin eval).
- `musubi_workflow action=resume` para retomar un run en otra sesión (estado + steps listos).
- Loops en workflows: un step con `repeat_while` (+ `max_iterations`, cota anti-infinito) se
  re-ejecuta mientras la condición sea verdadera. Tracks A3.
- `musubi_workflow action=validate` (valida una definición sin correrla) y `action=list`
  (lista los runs con su progreso). Con esto Track A (motor DAG) queda completo.
- Templates de artefactos SDD (`proposal`/`spec`/`design`/`tasks`) versionados: `musubi setup`
  los deja en `.musubi/templates/sdd/`. Scaffold con `schema_version`, idempotente.
- `docs/Roadmap_spec-kit_adoption.md`: plan de orquestación DAG, multi-agente y templates SDD
  (inspirado en spec-kit, adaptado a local-first/model-free).

## [0.12.0] - 2026-06-18

### Added
- Skill cognitiva `audit-structure-flow` en el bundle de arranque: cada `musubi setup`
  la escribe en `.musubi/skills/`. Audita estructura y flujo del codebase (organización,
  acoplamiento, capas, ciclos, código muerto, propagación de context/errores) con
  hallazgos priorizados. También publicada en el catálogo de skills (#47, #48).
- VERSIONINFO del `.exe` reproducible: `cmd/musubi/versioninfo.json` + `go:generate`
  como única fuente de verdad (antes se editaban los `.syso` a mano) (#43).
- README con banner SVG animado y diagramas Mermaid (arquitectura, auto-descubrimiento,
  loop por turno) (#45).

### Changed
- Higiene de estructura (sin cambio de comportamiento): eliminado el paquete huérfano
  `internal/telemetry`; `methods.go` partido (1386→1073) extrayendo el catálogo de tools;
  `main.go` partido (601→207) a `setup.go` e `install.go` (#46).
- Más cobertura de tests en `cmd/musubi` (helpers de setup, calibrate, doctor, catalog) (#44).

## [0.11.0] - 2026-06-18

### Added
- Proveedor de embeddings `openai`: usa la API de OpenAI o cualquier servidor
  compatible con su esquema (LM Studio, vLLM, LocalAI…). La API key se lee de una
  env var (`api_key_env`, default `OPENAI_API_KEY`) y nunca se guarda en el yaml.
- `LICENSE` (MIT), este `CHANGELOG.md` y `CONTRIBUTING.md`.
- Plantillas de issue/PR en `.github/` y badges de CI, release y licencia en el README.

### Changed
- Hardening de robustez: propagación de `context.Context` con timeouts en la capa
  de memoria/embeddings, chequeo de `rows.Err()`, graceful shutdown del daemon
  (SIGINT/SIGTERM), recuperación de panics en los handlers JSON-RPC y validación
  del campo `jsonrpc`.
- Cobertura de tests: `internal/mcp` a 75.8% y `cmd/musubi` a 45.6%.

### Fixed
- `extract_deps`: parseo correcto de dependencias tipo `pydantic[extras]>=2.0`.

## [0.10.0] - 2026-06-16

### Added
- Memoria de código automática: hook `PreToolUse(Read)` que muestra el gist de un
  archivo antes de leerlo (#40).
- Gists de archivos con frescura por fingerprint, model-free (#39).

## [0.9.1] - 2026-06-16

### Changed
- Fin de la doble inyección priming↔turno: el priming siembra el delta (#38).
- Documentado el sistema de eficiencia de tokens; `calibrate` es opcional y gratis.

### Added
- Test de auditoría del footprint de tokens de Musubi (#37).

## [0.9.0] - 2026-06-16

### Added
- Calibración opt-in del estimador de tokens contra `count_tokens`, con
  contabilidad del priming (#36).

## [0.8.0] - 2026-06-16

### Added
- Núcleo de eficiencia de tokens: estimador calibrado + ledger + inyección delta,
  todo model-free (#35).

## [0.7.3] - 2026-06-16

### Fixed
- Resueltos los hallazgos BAJO de la auditoría completa (#34).

## [0.7.2] - 2026-06-16

### Fixed
- Hardening: arreglados los 9 hallazgos ALTO/MEDIO de la auditoría multi-agente (#33).

## [0.7.1] - 2026-06-16

### Changed
- Hardening de la capa de orquestación (auditoría multi-agente) (#31).

## [0.7.0] - 2026-06-16

### Added
- Multi-agente: pizarra compartida (`musubi_work`) para orquestar sub-agentes,
  model-free (#30).

## [0.6.0] - 2026-06-16

### Added
- Loop dirigido + pipeline por fases (`musubi_phase`) para orquestación model-free (#29).

## [0.5.0] - 2026-06-16

### Added
- Resolución de conflictos semánticos entre observaciones, model-free (#28).
- `musubi doctor` con auto-repair (y backup).

## [0.4.0] - 2026-06-15

### Changed
- Mejoras internas y bump de VERSIONINFO del `.exe` (#27).

## [0.3.1] - 2026-06-15

### Fixed
- VERSIONINFO del `.exe` actualizada (#25).

## [0.3.0] - 2026-06-15

### Added
- Auto-update del binario: comando `musubi update` + aviso de versión nueva al
  arrancar el daemon (#24).

## [0.2.4] - 2026-06-14

### Added
- Doble clic en `Musubi.exe` muestra el menú de instalación (local/global) (#18).

## [0.2.3] - 2026-06-14

### Fixed
- Reducción de falsos positivos de antivirus: VERSIONINFO en el `.exe` +
  checksums SHA-256 en las releases (#17).

## [0.2.2] - 2026-06-14

### Changed
- La release publica el binario de Windows como `Musubi.exe` (#16).

## [0.2.1] - 2026-06-14

### Added
- Icono embebido en el `.exe` de Windows (#15).

## [0.2.0] - 2026-06-14

### Added
- Instalador con elección de alcance: local al repo o global en la PC (#13).

## [0.1.0] - 2026-06-13

### Added
- Distribución inicial: instaladores de una línea, workflow de release y setup
  por doble clic.
- Servidor MCP en Go con memoria persistente local-first sobre SQLite (FTS5 +
  búsqueda semántica opcional vía Ollama), resolución dinámica de skills y
  telemetría de errores.

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.44.0...HEAD
[0.44.0]: https://github.com/codeabraham16/musubi/compare/v0.43.1...v0.44.0
[0.43.1]: https://github.com/codeabraham16/musubi/compare/v0.43.0...v0.43.1
[0.43.0]: https://github.com/codeabraham16/musubi/compare/v0.42.0...v0.43.0
[0.42.0]: https://github.com/codeabraham16/musubi/compare/v0.41.0...v0.42.0
[0.41.0]: https://github.com/codeabraham16/musubi/compare/v0.40.0...v0.41.0
[0.40.0]: https://github.com/codeabraham16/musubi/compare/v0.39.0...v0.40.0
[0.39.0]: https://github.com/codeabraham16/musubi/compare/v0.38.0...v0.39.0
[0.17.0]: https://github.com/codeabraham16/musubi/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/codeabraham16/musubi/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/codeabraham16/musubi/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/codeabraham16/musubi/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/codeabraham16/musubi/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/codeabraham16/musubi/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/codeabraham16/musubi/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/codeabraham16/musubi/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/codeabraham16/musubi/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/codeabraham16/musubi/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/codeabraham16/musubi/compare/v0.7.3...v0.8.0
[0.7.3]: https://github.com/codeabraham16/musubi/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/codeabraham16/musubi/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/codeabraham16/musubi/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/codeabraham16/musubi/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/codeabraham16/musubi/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/codeabraham16/musubi/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/codeabraham16/musubi/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/codeabraham16/musubi/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/codeabraham16/musubi/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/codeabraham16/musubi/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/codeabraham16/musubi/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/codeabraham16/musubi/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/codeabraham16/musubi/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/codeabraham16/musubi/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/codeabraham16/musubi/releases/tag/v0.1.0
