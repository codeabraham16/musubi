# Changelog

Todos los cambios notables de Musubi se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

### Added
- **Operabilidad 24/7: métricas por-tool + contadores de rechazo + COUNT cacheado en `/metrics` (Track 17, T17.5).**
  Cierra los huecos de observabilidad que marcó la auditoría de cierre. **(1) Métricas por-tool:** el histograma de
  latencia era sólo agregado (no se veía QUÉ tool se llama más, cuál falla o cuál es la más lenta). Ahora, además del
  agregado, se emiten `musubi_tool_invocations_total{tool,result}` y `musubi_tool_latency_seconds_{sum,count}{tool}`
  (avg = sum/count), orden alfabético para un scrape determinista. **(2) Rechazos visibles:** las tools/call negadas
  por **rol** (authz) o **cuota** eran invisibles en `/metrics` (la request HTTP contaba como ok), ocultando abusos o
  clientes mal configurados; ahora `musubi_tool_rejections_total{reason="authz|quota"}` los cuenta. **(3) COUNT
  cacheado + con timeout:** los gauges de dominio re-ejecutaban los `COUNT` O(n) sobre `observations` en **cada**
  scrape; ahora se cachean con un TTL corto (15s) y los `COUNT` corren con un deadline (5s) para que una base lenta no
  cuelgue el scrape (best-effort: si vence, se omiten los gauges ese ciclo). Guards: `TestServerMetricsToolHistogram`
  (por-tool + rechazos), `TestDomainGaugeCacheTTL`.
- **`musubi embed backfill`: re-embeber el histórico (Track 17, T17.3).** Al encender la memoria semántica sobre una
  base con observaciones previas —o al cambiar de embedder— esas observaciones quedaban SIN vector de la procedencia
  actual y eran **invisibles** para el recall semántico para siempre; `WarnOnEmbedModelSwitch` avisaba del hueco pero
  no ofrecía remedio. El nuevo subcomando `EmbedBackfill` recorre las observaciones activas sin vector del modelo
  actual (sin fila en `embeddings` o con `model_id` distinto), las re-embebe con el embedder resuelto (mismo que
  serve/daemon), reconstruye el índice IVF una vez y actualiza la marca de modelo. Es **idempotente y resumible**
  (una fila ya re-embebida no se vuelve a listar). Sin semántica encendida ⇒ mensaje claro y salida. Guards:
  `TestEmbedBackfillReembedsHistory`, `TestEmbedBackfillSkipsEmptyVectors`.
- **Gate de calidad R@10 del recall semántico en CI (Track 17, T17.3c).** El harness `recalleval` medía léxico vs
  semántico con la tabla POTION real pero `TestSemanticVsLexicalReal` **sólo logueaba** el reporte (y se salteaba en
  CI): la calidad del recall no era un contrato defendido, sólo una medición de una vez. Ahora el test **asserta** un
  piso: híbrido **R@10 ≥ 0.80** (medido 0.833; léxico 0.750) y híbrido ≥ léxico (el win semántico debe ser aditivo).
  Nuevo job `recall-gate` en CI que **cachea** la tabla (~488MB, SHA-256 pinneado; sólo se baja en cache miss) y corre
  la evaluación con `MUSUBI_POTION_DIR`. Atrapa una regresión real (bug en el tokenizer Unigram, en el ranking híbrido
  o en la tabla) que degrade el recall — con el mismo molde de ratchet que el piso de cobertura y el `bench-guard`.

### Fixed
- **Procedencia de vector real por-modelo: `ollama`/`openai` ya no mezclan modelos en silencio (Track 17, T17.3).**
  El `model_id` que estampa la procedencia del vector salía de `Provider.Name()`, que para `ollama`/`openai` devolvía
  la **constante** `"ollama"`/`"openai"` — así, dos modelos distintos de **igual dimensión** bajo el mismo provider
  (p.ej. `nomic-embed-text` vs `mxbai-embed-large` a 768) compartían `model_id` y se **mezclaban** en la búsqueda por
  coseno, corrompiendo el recall en silencio (la única guarda previa, por dimensión, no los distinguía). Ahora
  `Name()` incluye el modelo (`"ollama:<model>"` / `"openai:<model>"`), de modo que la regla de homogeneidad los
  separa. `static` ya era correcto (incluía la tabla). *Nota:* tras actualizar, los vectores `ollama`/`openai` viejos
  quedan con la procedencia antigua y salen del recall hasta correr `musubi embed backfill` (arriba).

### Changed
- **DR off-host segura por default + dead-man's-switch + test de restore en CI (Track 17, T17.4).** Cierra el
  hallazgo **CRÍTICO** de la auditoría (perder el disco del cerebro central = perder toda la memoria compartida),
  que seguía abierto porque el backup off-host era un **no-op silencioso**. Tres cambios: **(1) fallo-cerrado** —
  `deploy/musubi-backup.sh` con `BACKUP_REMOTE` vacío ahora **falla** (exit≠0 ⇒ la unidad systemd queda `failed` y
  se ve en `systemctl status`) en vez de reportar "éxito" dejando el snapshot solo-local; el modo local-only se
  acepta **explícitamente** con `BACKUP_ALLOW_LOCAL_ONLY=1`. **(2) dead-man's-switch** — tras cada envío off-host
  exitoso el script deja una marca `.last_offhost`; un nuevo check de `musubi doctor` (`offhost_backup`) **avisa**
  (warning, no error; no afecta `readyz`) si esa marca envejece > 48h (el timer dejó de shipear). Marca ausente ⇒
  ok (no genera falsos positivos en máquinas de desarrollo sin timer). **(3) test de restore en CI** —
  `TestBackupToProducesRestorableSnapshot` toma un snapshot (`VACUUM INTO`), lo **restaura** como base nueva y
  verifica `integrity_check` + esquema + datos de las 3 familias (observación/hecho/código): "tenemos backups"
  pasa de afirmación no verificada a camino ejercitado en cada corrida. Verificado end-to-end con binario real
  (fallo-cerrado / escape hatch / envío + marca). *Nota de despliegue:* el servidor con `BACKUP_REMOTE` vacío
  empezará a fallar el timer hasta configurar un destino remoto o setear `BACKUP_ALLOW_LOCAL_ONLY=1`.

## [0.81.0] - 2026-07-10

### Fixed
- **Invalidación por cardinalidad cross-tenant del grafo de hechos — corrección de correctitud (Track 17, migración
  v14).** Con `UNIQUE(from_id, predicate, to_id)`, la invalidación por cardinalidad de un predicado **funcional**
  (single-valued: `works_at`, `estado_actual`…) cruzaba proyectos: en un cerebro central compartido, guardar
  `(Ana, works_at, Acme)` desde el proyecto A **cerraba la ventana** de `(Ana, works_at, Globex)` viva en el
  proyecto B (un tenant mutaba silenciosamente la verdad de otro). La migración v14 reconstruye `relations` con
  `UNIQUE(from_id, predicate, to_id, project_id)` (`project_id NOT NULL DEFAULT ''`, filas legacy → `''`), y la
  invalidación se acota **estrictamente** al proyecto de origen. Además el mismo triple ya puede coexistir entre
  proyectos (antes colisionaba en el `ON CONFLICT`).
- **Colisión cross-tenant de la memoria de código (`code_memory`) — corrección de correctitud (Track 17, migración
  v13).** `code_memory` tenía `PRIMARY KEY(path)`, así que en un cerebro central compartido dos proyectos con el
  mismo `path` (p.ej. `internal/auth.go`) **colisionaban** en el `ON CONFLICT(path)` y se **pisaban el gist** entre
  sí. La migración v13 reconstruye la tabla con `UNIQUE(path, project_id)` (`project_id NOT NULL DEFAULT ''`, filas
  legacy → `''`), de modo que cada proyecto tiene su propia entrada por archivo.

### Security
- **Aislamiento (parcial) de `musubi_insights` por proyecto (Track 17, T17.1c).** `insights` reportaba los counts de
  observations (`total`/`active`/`archived`) de **todos** los proyectos, filtrando el **volumen** de la memoria ajena.
  Ahora `InsightsCtx` acota esos counts al proyecto de la **credencial** (mismo `scopeClause`); `admin`/stdio ⇒
  federado. Es un aislamiento **parcial deliberado**: los hotspots de errores (`telemetry_logs`) y las decisiones de
  skills (`skill_decisions`) siguen federados porque sus tablas **no** tienen `project_id` (scopearlas requeriría otra
  migración; diferido, bajo riesgo). Con esto **todas las superficies de lectura respaldadas por `observations`/
  `relations`/`code_memory` quedan aisladas** — cierra el HIGH de cross-project bleed de la auditoría de cierre. Guard:
  `TestInsightsCtxScopesObservationCounts`.
- **Aislamiento del grafo de hechos (`recall_facts` / `entity_context` / `fact_path`) por proyecto (Track 17,
  T17.1b-4, migración v14).** La última superficie de lectura sin aislar: el recorrido del grafo devolvía hechos de
  **todos** los proyectos. Ahora `SaveFactFrom` atribuye la arista al proyecto de la **credencial** y un helper único
  (`liveFactFilter`) **pliega el scope de proyecto dentro del filtro bi-temporal** que comparten las tres superficies
  de traversal —BFS (`expandFrontier`), recall asociativo (PageRank) y camino más corto (`pathNeighbors`)—, de modo
  que las tres quedan scopeadas por un solo punto de cambio. `entity_context` acota además la parte de **prosa**
  (`observationGistsCtx`). Las **entidades** siguen siendo globales (se comparten los nodos; sólo las aristas se
  atribuyen). `recall_facts`/`entity_context` pasaron a ctx-aware y `save_fact` deriva el origen de la credencial;
  `admin`/stdio ⇒ federado. Guards: `TestFactsReadNoBleed`, `TestFactsCardinalityPerProject`,
  `TestFactPathProjectScope`, `TestFactsPageRankProjectScope`, `TestEntityContextProjectScope`,
  `TestMigrationV14RebuildsRelationsPreservingData`.
- **Aislamiento de `musubi_recall_code` por proyecto (Track 17, T17.1b-3).** Sobre la migración v13 (arriba):
  `SaveCodeMemoryFrom` atribuye el gist al proyecto de la **credencial** (no a un espacio global) y
  `GetCodeMemoryCtx` acota la lectura al proyecto del principal, prefiriendo su propia fila sobre la sin atribuir.
  `musubi_save_code`/`musubi_recall_code` pasaron a ctx-aware. `admin`/stdio ⇒ federado. Guard:
  `TestCodeMemoryProjectIsolationAndNoCollision`.
- **Aislamiento de `musubi_conflicts` por proyecto (Track 17, T17.1b-2).** Extiende el aislamiento multi-tenant a
  la superficie de conflictos de memoria: antes `musubi_conflicts` devolvía las relaciones pendientes de TODOS los
  proyectos. Ahora `PendingObsRelationsCtx` hace `JOIN` a `observations` por el `source_id` y filtra por el
  `project_id` **derivado de la credencial** (mismo `scopeClause` que las demás superficies); `admin`/stdio ⇒
  federado. `musubi_conflicts` pasó a ctx-aware. Sin migración (aprovecha el `project_id` que ya vive en
  `observations`). Guard: `TestConflictsEnforcePrincipalScope`.
- **Redacción de TODO ingest al central: `save_fact` y `save_code` ya no escriben secretos crudos (Track 17, T17.2).**
  La auditoría de cierre encontró que la redacción forzada server-side (`forceRedact`) cubría **solo**
  `save_observation` — `save_fact` (subject/predicate/object) y `save_code` (gist/symbols) escribían contenido
  **crudo** al pozo compartido, recuperable por `recall_facts`/`recall_code`, mientras el `Threat_Model` lo declaraba
  falsamente como "redacta TODO ingest". Ahora un helper único (`redactIfForced`) pasa **las tres** tools por la
  redacción cuando el bind es no-loopback (el central). Además: en `save_observation` el contenido se redacta
  **ANTES** de computar el embedding (el vector at-rest ya no se deriva del secreto crudo) y el `topic_key` también
  se cubre. El `Threat_Model.md` se corrigió para reflejar el alcance real **y** advertir que la redacción es
  **best-effort heurística** (reduce, no garantiza; un secreto corto o de baja entropía puede escapar), no una
  garantía dura. Guard: `TestForceRedactCoversAllIngest`. En loopback local el contenido queda crudo (el dev lo necesita).
- **Atribución de escritura por credencial: se cierra el write-poisoning cross-tenant (Track 17, T17.1b-1).**
  Complementa T17.1a (aislamiento de LECTURA) con su contracara de ESCRITURA: `musubi_save_observation` confiaba en
  el `project_id` que declaraba el cliente, así que un `writer`/`reader` acotado a un proyecto podía atribuir una
  observación a OTRO proyecto (o dejarla sin atribuir, visible para todos), evadiendo el aislamiento recién
  cerrado. Ahora el origen se **deriva de la credencial** (`principalFrom(ctx)`): un principal no-admin siempre
  escribe atribuido a SU proyecto; se ignora el `project_id` de los args. El origen explícito se respeta solo para
  **admin/legacy** (ingest del central, para quien se diseñó la variante `*From`). `musubi_save_observation` pasó a
  ctx-aware (`countingSaveCtx`). Guard: `TestWriteAttributionFromPrincipal`.
- **Aislamiento multi-tenant: se cierra la fuga de CONTENIDO cross-project (Track 17, T17.1a).** La auditoría de
  cierre encontró que el scope por-credencial estaba cableado en UNA sola superficie de lectura (`musubi_recall`):
  las demás consultaban la memoria SIN filtro de proyecto, así que un principal acotado a un proyecto leía el
  contenido crudo de TODOS. Esta unidad cierra las 3 superficies que devuelven contenido completo —
  `musubi_search_keyword`, `musubi_search_semantic` y `musubi_memory_expand` (la fuga más grave: hidrataba por id
  arbitrario). Diseño de mínima superficie: un `ProjectScope` que viaja por el **contexto** (`WithProjectScope`/
  `projectScopeFrom`) y un helper SQL `scopeClause` centralizado (mismo criterio que `filterCandidatesByProject`
  del recall: el proyecto pedido + las filas sin atribuir); las funciones de lectura del engine lo aplican sin
  cambiar la firma de `StorageBackend` ni sus ~30 callers. El MCP deriva el scope de la credencial (`recallScopeFor`)
  y lo inyecta (`scopedCtx`); `musubi_memory_expand` pasó a ctx-aware. Ausencia de scope (stdio local / admin /
  legacy) ⇒ federado, comportamiento histórico bit-a-bit. Guards de no-bleed: `TestReadIsolationByProjectScope`
  (motor, las 3 funciones) + `TestReadSurfacesEnforcePrincipalScope` (e2e MCP). **Pendiente en T17.1b:** las
  superficies de metadata/grafo (`recall_facts`, `entity_context`, `recall_code`, `insights`, `conflicts`) y la
  atribución de ESCRITURA por credencial (poisoning).

### Added
- **README en inglés + cross-link ES↔EN (adopción por terceros, Track 16 / Producible F4).** Cierra la Fase 4.
  Toda la documentación estaba solo en español, así que un adoptante anglófono no tenía onboarding. Nuevo
  `README.en.md` — espejo fiel del README (instalación, inicio rápido, cómo funciona, capacidades, herramientas
  MCP, configuración, referencia de CLI, búsqueda semántica, desarrollo, roadmap; diagramas Mermaid con labels
  traducidos y anchors del TOC en inglés). Ambos READMEs llevan un selector de idioma cruzado en el encabezado.
- **CI cross-platform: validación en Windows y macOS (adopción por terceros, Track 16 / Producible F4).** Hasta
  ahora todos los jobs de CI corrían solo en `ubuntu-latest`; los binarios se cross-compilan para 6 targets pero
  nunca se *testeaban* fuera de Linux. Nuevo job `test-cross` con `strategy.matrix: [windows-latest, macos-latest]`
  que corre `go vet` + `go build` + `go test ./...` en cada uno. El job `test` de ubuntu sigue siendo el canónico
  (race + piso de cobertura + govulncheck). Sin `-race` en la matriz a propósito: evita depender de cgo/gcc en
  Windows (el driver SQLite es `modernc` puro Go, así que build y test no necesitan un compilador C).

### Changed
- **`musubi provision` ahora EXIGE `--brain` (adopción por terceros, Track 16 / Producible F4).** Antes `--brain`
  defaulteaba a `100.79.126.62:7717` — la IP del tailnet del AUTOR: un tercero que corría `musubi provision` sin
  flags terminaba sondeando/cableando contra la máquina del autor. Se eliminó ese default personal (constante
  `provision.DefaultBrain`) y ahora `provision` falla con un mensaje claro si falta `--brain`, apuntando a `musubi
  setup` para quien solo quiere setear el proyecto localmente sin un cerebro central. Mismo criterio en los scripts
  de deploy: `deploy/connect-brain-linux.sh` (`BRAIN_IP` requerido vía `${BRAIN_IP:?…}`) y
  `deploy/connect-brain-windows.ps1` (`-BrainIp` requerido con check explícito). Ningún archivo versionado apunta ya
  a infra del autor. (El `repoOwner`/catálogos siguen en `codeabraham16/musubi` — ese ES el repo público real.)

## [0.80.0] - 2026-07-09

### Added
- **`/metrics` accionable: latencia de tools + gauges de dominio (Track 16 / Producible F3.1).** Antes `/metrics`
  solo exponía 4 contadores de requests HTTP por resultado — un operador 24/7 no veía nada del dominio. Ahora,
  manteniendo cero dependencias (renderer Prometheus hecho a mano), agrega: (a) **histograma de latencia**
  `musubi_tool_duration_seconds` (buckets + `_sum` + `_count`, lock-free) y contador `musubi_tool_calls_total`
  {ok,error} por cada `tools/call`, instrumentado en el choke point `handleToolsCall` (cubre stdio y HTTP); (b)
  **gauges de dominio** pulled at scrape vía un accesor nuevo `DbEngine.OperationalStats()`: `musubi_observations`,
  `musubi_embeddings_active`, `musubi_vector_index_size`, `musubi_vector_index_trained`, `musubi_sync_outbox`
  {pending,sent,dead} y `musubi_sync_outbox_oldest_pending_age_seconds` (atraso del sync). Los gauges se exponen
  vía una interfaz opcional (`opStatsProvider`) type-asserted al render, así los backends de test que no la
  implementan no rompen el scrape. Las métricas viven en un `serverMetrics` compartido en el `McpServer`.
- **Benchmark de búsqueda vectorial a escala + guard de sublinealidad del IVF (Track 16 / Producible F3.3).** El
  único benchmark vectorial topaba en n=10 000 (justo el umbral donde el IVF se activa), así que el régimen donde
  el índice debe ganarle al full-scan quedaba sin medir ni proteger en CI. `BenchmarkSearchVector` ahora fuerza el
  entrenamiento síncrono del IVF (mide la ruta indexada de forma determinista, no el full-scan transitorio) y suma
  un caso de escala **n=100 000 opt-in** (env `MUSUBI_BENCH_SCALE`, porque sembrar 100k tarda minutos). Nuevo
  **bench-guard en CI** que corre `BenchmarkSearchVector` a n=1k y n=10k y verifica que la memoria por búsqueda
  crezca SUB-LINEALmente (`B/op(10k)/B/op(1k)` ≈ 3.7x medido, ~√10; umbral 6): una regresión que rompa el IVF y
  caiga a full-scan lo llevaría a ~lineal (~10x). Se mide `B/op` (determinista) y no wall-time, igual que el guard
  de `BenchmarkMaintain`.
- **Cuota de uso por-principal (Track 16 / Producible F3.2).** Cierra la Fase 3. Hasta ahora, una vez autenticado,
  un principal podía hacer llamadas ilimitadas: el único rate-limit era el lockout de auth por-IP (anti fuerza
  bruta del bearer). Nuevo `quotaLimiter` (ventana deslizante en memoria, model-free, espeja `authLimiter`) que
  limita las `tools/call` **por identidad de principal** por minuto, enforced en el choke point `handleToolsCall`
  (tras autorizar por rol, antes de tomar el lock — no serializa los rechazos). Superar la cuota devuelve el nuevo
  código `codeQuotaExceeded` (-32002; la credencial es válida, solo excedió el uso). Configurable con
  `service.quota_per_minute` (0 = sin límite, default). Solo aplica cuando hay principal (serve con registro); en
  stdio local (agente confiable, sin principal) no hay cuota. Distintos principals tienen cuotas independientes.

## [0.79.1] - 2026-07-09

### Fixed
- **`musubi embed pull` ahora cae a IPv4 cuando el IPv6 no tiene ruta (Track 16 / Producible, pulido de Fase 4).**
  En máquinas con IPv6 *configurado pero sin ruta real* (VPN que tuneliza sólo IPv4, red que anuncia IPv6 sin
  salida), la descarga de la tabla fallaba con `dial tcp [2600:…]:443: connect: network is unreachable` porque el
  cliente HTTP por default de Go no reintentaba por IPv4. Ahora el downloader usa un cliente que, ante un error de
  *red/host inalcanzable* (`ENETUNREACH`/`EHOSTUNREACH`), **reintenta forzando `tcp4`** — sin romper las redes
  IPv6-only (que aciertan en el primer intento) ni cambiar el camino feliz. Se detectó dogfooteando el despliegue
  de la Fase 2 en una laptop Linux con IPv6 roto.
- **El mensaje de éxito de `musubi embed pull` ya no manda a editar `config.yaml` de gusto.** Desde 16.2f la
  memoria semántica es *auto-ON* (`resolveEmbedder` detecta la tabla en la ubicación estándar y la enciende al
  reiniciar), pero el mensaje seguía diciendo "para activar, poné `provider: static`…" — heredado y engañoso.
  Ahora, si la tabla quedó en la ruta estándar del modelo default, informa que **se auto-detecta al reiniciar el
  daemon** (sin tocar config); sólo si quedó fuera (por `--out` o un modelo no-default) muestra las líneas de
  `config.yaml` a declarar.

### Security
- **Toolchain de Go a `1.26.5` en CI/release por `GO-2026-5856`** — leak de privacidad en *Encrypted Client Hello*
  de `crypto/tls`, presente en go1.26.4 y corregido en go1.26.5. El pin flotante `1.26.x` se había quedado en
  1.26.4 (retraso del manifest de `setup-go`), así que `govulncheck` empezó a marcar la stdlib; se fija **exacto a
  `1.26.5`** en los tres jobs de `ci.yml` y en `release.yml` para que los binarios publicados se compilen con la
  stdlib parcheada.

## [0.79.0] - 2026-07-08

### Added
- **Captura automática (C3/C4) con embeddings — cierra la Fase 2 (Track 16 / Producible 16.2e).** Las memorias
  auto-capturadas se guardaban con vector `nil`, así que quedaban FUERA del recall semántico (sólo participaban
  las guardadas por herramienta). Ahora, cuando la semántica está encendida, **C3** (commits nuevos, hook `Stop`
  vía `musubi capture`) y **C4** (par error→fix al resolver telemetría) generan su embedding: `runCapture`
  resuelve el embedder con la MISMA auto-detección + degradación elegante que `serve`/`daemon` (`resolveEmbedder`)
  y estampa la MISMA procedencia (`SetVectorModelID`, F2.2) para que los vectores sean homogéneos; C4 usa un
  helper best-effort en el MCP server. Best-effort en ambos: un fallo de embedding devuelve `nil` (ese ítem queda
  léxico) sin romper el turno ni el resolve. Con esto, TODA la memoria —capturada o guardada explícitamente—
  participa del recall semántico. Golden intacto.
- **Memoria semántica ON por default con auto-detección + degradación elegante (Track 16 / Producible 16.2f).**
  Cierra la Fase 2: la semántica se enciende sola cuando se puede y NUNCA rompe el arranque. El entrypoint
  (`serve`/`daemon`) ahora resuelve el embedder con `resolveEmbedder`: si no hay provider explícito (`none`/vacío)
  y existe una tabla en la ubicación estándar (`<workspace>/.musubi/embeddings/potion-multilingual-128M`, la que
  baja `musubi embed pull`), enciende `static` automáticamente; si no hay tabla —o si cargarla falla— cae a
  **recall léxico** en vez de abortar (antes un error de embeddings hacía `os.Exit`). **Medición del gate** (con
  la tabla real de POTION multilingüe, sobre el fixture dorado): la semántica es un **win aditivo** — `R@10`
  0.75→**0.83** (recupera ~1/3 de los relevantes del hueco de vocabulario) **sin regresión** en `R@1`/`R@5`/`MRR`.
  Test de medición repetible (`recalleval`, gated por `MUSUBI_POTION_DIR`). También: fix del flag `--out` de
  `embed pull` (el modelo posicional se extrae antes de parsear, así `embed pull <modelo> --out X` funciona) y
  `.musubi/embeddings/` va al `.gitignore` (tablas de cientos de MB, puro dato). Golden intacto.
- **`musubi embed pull` — descarga turnkey de la tabla de embeddings + carga plana (Track 16 / Producible 16.2d).**
  Hace la memoria semántica *lista para encender* sin pasos manuales. Nuevo comando **`musubi embed pull
  [modelo] [--out DIR] [--mirror URL]`** que baja una tabla estática (por default `potion-multilingual-128M`,
  ES+EN) con **checksum SHA-256 pinneado**, de forma **atómica** (baja a `.part`, verifica tamaño + hash, y sólo
  entonces renombra) e **idempotente** (si ya está con el checksum correcto, no re-descarga). La tabla es PURO
  DATO: se baja una vez en el setup y en runtime no corre ninguna red ni modelo (model-free at inference). El
  flag `--mirror` permite re-hostearla en infra propia (Forgejo/servidor del tailnet) manteniendo el checksum
  pinneado, así un mirror comprometido no puede colar otra tabla. Registro `embedding.KnownModels` con URLs y
  hashes verificados contra el oid LFS de la fuente. Además, `StaticProvider` ahora carga la tabla **PLANA** (un
  solo `[]float32` de vocab×dim en vez de ~500K slices): para la multilingüe (500K×256 ≈ 488 MB) evita cientos de
  miles de headers de slice y mejora la localidad de caché. Golden intacto.
- **Tokenizer Unigram/SentencePiece en Go puro — habilita tablas MULTILINGÜES (Track 16 / Producible 16.2c).**
  El `StaticProvider` sólo sabía tokenizar WordPiece BERT (tablas inglesas). Las tablas multilingües de
  model2vec/POTION (ES+EN reales, p. ej. `potion-multilingual-128M`) usan **Unigram/SentencePiece** —otro
  formato de `tokenizer.json`— así que no cargaban. Este PR agrega un tokenizer Unigram **bit-exacto vs
  HuggingFace, en Go puro y sin cgo**, reproduciendo todo el pipeline: normalizer con `precompiled_charsmap`
  (trie DARTS de SentencePiece) + reglas `Replace` + `Strip`, pre-tokenizer `Metaspace` (▁), y segmentación
  `Unigram` por Viterbi sobre ~500K piezas con log-probs. La única sutileza vs HF (recomposición de secuencias
  descompuestas por grapheme) se resuelve con `NFC` antes del charsmap, que da idéntico resultado para toda
  entrada realista. `static.go` se refactorizó a una interfaz `tokenizer` con dispatch por `model.type`
  (WordPiece | Unigram); el WordPiece existente no cambia de comportamiento. **Validado bit-exacto** contra el
  tokenizer real de POTION multilingüe (test gated por `MUSUBI_SPM_TESTDATA`; referencia `text→ids` en testdata)
  y con unit tests sintéticos del Viterbi/normalizer. Precede a 16.2d (traer la tabla). Golden intacto.
- **Contrato de vector + procedencia — regla de homogeneidad (Track 16 / Producible 16.2b).** El núcleo de
  ROBUSTEZ de la memoria semántica, hecho ANTES de encenderla (S1 de Track 15). Hasta ahora un vector no
  registraba QUÉ modelo lo produjo: al cambiar de embedder, los vectores viejos (otra procedencia) se comparaban
  por coseno con los nuevos y **corrompían el recall EN SILENCIO** cuando compartían dimensión (misma dim, otro
  espacio semántico ⇒ similitudes basura coladas al top). La única guarda previa era por dimensión (el
  dim-guard), que no cubre el borde same-dim; sólo había un *warning* (`WarnOnEmbedModelSwitch`) que recomendaba
  limpiar a mano. Ahora: migración v12 añade `embeddings.model_id`; cada engine estampa la **procedencia** de su
  embedder (`SetVectorModelID`, cableado en `serve`/`daemon` con `provider.Name()`) en todo vector que escribe; y
  la búsqueda exacta (full-scan y por-celda IVF) aplica la **regla de homogeneidad**: sólo compara vectores de la
  MISMA procedencia que el de consulta. Los de otro modelo quedan **excluidos automáticamente** (no se mezclan ni
  corrompen el ranking) — el warning pasa a ser informativo (re-embeber para recuperarlos). Aditiva y
  backward-compat: `''` = procedencia desconocida (legacy y engines sin embedder nombrado) sólo compara contra
  `''`, así que el comportamiento histórico —y todos los tests/bench sin `SetVectorModelID`— no cambian. Golden
  intacto.
- **Harness de calidad de recall (Track 16 / Producible 16.2a).** Primer paso de la Fase 2: una forma
  REPETIBLE y determinista de MEDIR qué tan bueno es el recall, para poder probar con números —no con fe— que
  encender la señal semántica mejora sobre el baseline léxico ANTES de cambiar el default (el audit fue tajante:
  *harness primero*). Nuevo paquete `internal/recalleval`, 100% model-free y sin red: métricas estándar de IR
  (`recall@k`, `MRR`, `nDCG@k`) como aritmética pura + un runner que siembra un motor de memoria temporal con un
  **fixture dorado versionado** (`testdata/golden.json`: 26 docs de memoria de dev ES/EN + 12 queries
  etiquetadas) y evalúa una o más configuraciones de recall sobre el mismo corpus. El fixture incluye a propósito
  queries de **hueco de vocabulario/traducción** (bug↔error, deploy↔despliegue, olvido↔decay) donde el léxico
  debería fallar y la semántica ganar. Baseline medido: **R@10 léxico = 0.75** (el léxico no encuentra el 25% de
  los relevantes ni en el top-10 → margen que la tabla POTION debe cerrar en 16.2c). El camino híbrido (con
  vector) queda ejercitado end-to-end con un embedder sintético para que la integración de la tabla real no
  descubra bugs tarde. Golden de MCP intacto.

## [0.78.0] - 2026-07-08

### Added
- **Hardening del borde del central — lockout + threat model + ACLs (Track 16 / Producible 16.1e).** Cierra la
  Fase 1. (1) **Lockout anti fuerza-bruta**: tras 5 fallos de auth desde una IP, el central la bloquea 60s
  (`authLimiter`, en memoria, model-free) — antes el adivinado online del bearer era ilimitado para cualquier
  peer del tailnet. (2) **Threat model documentado** (`docs/Threat_Model.md`): borde de confianza, activos,
  amenazas→mitigaciones y riesgos residuales — fija qué cubre WireGuard y qué no. (3) **Guía de ACLs de
  Tailscale**: la policy default es allow-all, así que se documenta cómo restringir el puerto del brain a
  dispositivos autorizados (defensa en profundidad, no confiar solo en el rango CGNAT). Cierra los hallazgos
  *low* de superficie HTTP, threat model y least-privilege de red (`audit/2026-07-08`). Golden intacto.
- **Redacción forzada server-side en el central (Track 16 / Producible 16.1d).** La redacción de secretos se
  disparaba por el VALOR del scope declarado por el cliente (`scope==shared`), así que un cliente podía escribir
  un secreto **crudo** en el cerebro compartido mandando `scope=local`. Ahora el central **redacta SIEMPRE**,
  independiente del scope declarado: un bind **no-loopback** (infra compartida) enciende `forceRedact`
  **fail-closed** (no se puede desactivar), y un bind loopback puede optar por `service.force_redact`. Con
  `forceRedact`, todo ingest se trata como `shared` ⇒ la redacción de C2 corre siempre. Cierra el hueco de
  ingest crudo del hallazgo de seguridad (`audit/2026-07-08`). Backward-compatible (stdio local y loopback sin
  el flag: sin cambios); golden intacto.
- **Enforcement del aislamiento por credencial (Track 16 / Producible 16.1c-3).** El cable que cierra la Fase 1:
  el scope del recall se **deriva del principal autenticado** (su `project_id` sale de la credencial, no lo
  auto-declara el cliente). `toolRecall` ahora acota el recall al proyecto del principal — un `reader`/`writer`
  con `project_id` **solo recupera memoria de su proyecto** (más la sin atribuir), mientras un `admin` ve
  **federado** (todos). Sin principal (stdio local) o sin `project_id` ⇒ sin scope (federado, histórico). Con
  esto el aislamiento de 16.1b se **activa automáticamente** por credencial: se cierra el hallazgo **high** de
  cross-project bleed. Lógica pura en `recallScopeFor(principal)`; enforcement e2e verificado (writer ve solo lo
  suyo, admin ve todo). Backward-compatible; golden intacto.
- **CLI `musubi token` — gestión del registro de principals (Track 16 / Producible 16.1c-2).** Hace usable la
  identidad por-principal sin computar hashes a mano: **`musubi token new --name X --project Y --role writer`**
  genera un token opaco (256 bits, prefijo `msb_`), guarda su **SHA-256** en `.musubi/principals.yaml` (nunca el
  token crudo) y lo imprime **una sola vez** para entregárselo al miembro; **`list`** muestra nombre/rol/proyecto
  (jamás el hash); **`revoke --name X`** da de baja. Rechaza nombres duplicados y roles inválidos; crea el
  archivo (600) si falta. El token generado **autentica de una** contra el registro (round-trip verificado).
  Runbook actualizado en `docs/Server_Brain_Onboarding.md`. Golden intacto.
- **Identidad por-principal — registro de tokens + autorización por rol (Track 16 / Producible 16.1c-1).**
  Cierra el core del hallazgo **high** _"un único bearer sin identidad/rotación/revocación/authz"_. El central
  puede cargar un **registro de principals** (`.musubi/principals.yaml` o `service.principals_file`) que mapea
  el **SHA-256** de cada token a `{name, project_id, role}` — credenciales **por-miembro revocables** (borrás la
  línea) en vez de un token compartido. El archivo guarda el **hash**, nunca el token crudo (un leak no da
  credenciales usables). En modo `serve`, cada request se autentica contra el registro y el principal viaja en el
  contexto; el dispatch aplica **authz por rol**: `reader` solo tools de lectura, `writer` lee+escribe, `admin`
  todo (deniega con `codeUnauthorized`). **Backward-compatible**: sin archivo de registro sigue el modo de un
  único bearer, y el `MUSUBI_TOKEN` legacy sigue válido como `admin`; el daemon stdio local no tiene principal
  (confianza local, acceso pleno). Runbook de alta/revocación en `docs/Server_Brain_Onboarding.md`. Golden
  intacto. (El CLI `musubi token new|revoke|list` y el enforcement `project_id`→recall llegan en 16.1c-2/16.1c-3.)
- **Aislamiento por proyecto en el recall + federación opt-in (Track 16 / Producible 16.1b).** Segundo paso de
  la Fase 1: el recall puede acotarse a un proyecto. `RecallOptions` suma `ProjectScope` y `Federate` — con
  scope y sin federate, el recall **descarta los candidatos de otros proyectos** (conserva el proyecto pedido y
  las filas sin atribuir); `Federate` los vuelve a ver todos (el opt-in del modelo "aislado + federación opt-in"
  elegido por el usuario). Implementado como **choke point único**: todos los pools (léxico, vectorial,
  co-ocurrencia) confluyen en `cands`, así que se filtra una sola vez —limpio y sin reescribir 11 queries—
  llevando el `project_id` del candidato en la fila. **Backward-compatible**: `ProjectScope` vacío ⇒
  comportamiento histórico (federado) bit-a-bit; el enforcement por defecto lo cableará la identidad (16.1c).
  Avanza el hallazgo **high** de cross-project bleed (`audit/2026-07-08`). Golden intacto.
- **Atribución multi-tenant — el central preserva el `project_id` de origen (Track 16 / Producible 16.1a).**
  Primer paso de la Fase 1 (cerebro multi-tenant). Antes, al ingerir una observación sincronizada, el central
  estampaba **su propio** `project_id` y descartaba el del proyecto de origen (`saveObservation` usaba siempre
  `e.projectID`, y `toolSaveObservation` ni leía el campo) — sin atribución no hay sobre qué aislar. Ahora el
  handler lee `project_id` del payload y lo **preserva**: nuevas variantes `SaveObservationTypedFrom` /
  `SaveObservationDedupedTypedFrom` estampan el proyecto de ORIGEN (`""` ⇒ el `project_id` del engine, así el
  guardado local no cambia). El sync client ya enviaba el `project_id`; ahora el central lo respeta. Cimiento del
  aislamiento por proyecto (16.1b). Backward-compatible: sin cambios en el recall todavía; golden intacto.

- **DR del cerebro central — backup consistente + off-host + runbook de restore (Track 16 / Producible 16.0b).**
  El nodo central es el único punto donde converge la memoria compartida de todos los proyectos; perder su
  `memory.db` sin backup off-host era irreversible. Ahora: (1) el backup usa **`VACUUM INTO`** en vez de copiar
  el archivo con `io.Copy` tras un `wal_checkpoint` — snapshot *transaccionalmente consistente* en un paso, sin
  lockear el daemon ni arriesgar un estado a medias por escrituras concurrentes; (2) nuevo comando **`musubi
  backup [--out <dir>]`** (puro-Go, no requiere `sqlite3` en el host) que imprime la ruta del snapshot; (3)
  `deploy/musubi-backup.sh` + un **timer systemd diario** (instalado por `install-musubi-brain.sh`) que shipa el
  snapshot **off-host** (`rsync`/`rclone`/`cp`) con retención; (4) **runbook de restore probado** en
  `docs/Server_Brain_Onboarding.md`. Cierra el hallazgo **crítico** «el central no tiene DR» de `audit/2026-07-08`.
- **Fuente única de versión + release verificable (Track 16 / Producible 16.0a).** La versión vivía en dos
  lugares que derivaron: el tag de git (vía `-ldflags -X main.version`) y `cmd/musubi/versioninfo.json` (el
  recurso de Windows), que quedó congelado en `0.57.0.0` con el proyecto en `0.78` porque el paso manual de
  regenerarlo se saltó ~20 releases. Ahora hay un archivo **`VERSION`** como fuente ÚNICA: un test
  (`TestVersioninfoMatchesVERSION`) falla si `versioninfo.json` diverge de `VERSION`, y `release.yml` **aborta
  el release** si el tag no coincide con `VERSION` y **regenera el `.syso`** desde `versioninfo.json` con
  `goversioninfo` pineado (`@v1.4.0`) — el `.exe` de Windows ya no puede reportar una versión vieja. Cierra el
  hallazgo *high* «release no cortada / versión con dos fuentes de verdad divergentes» de la auditoría
  (`audit/2026-07-08`).
- **Guarda de compatibilidad de esquema hacia adelante (Track 16 / Producible 16.0c).** Un binario viejo que
  abría una base migrada por uno más nuevo antes corría un no-op silencioso y operaba a ciegas sobre columnas/
  tablas que no conocía — riesgo de corrupción lógica en una flota mixta (laptop/PC/central con binarios de
  distinta versión). Ahora `applyMigrations` **falla-cerrado**: si el `user_version` de la base supera la última
  migración que este binario conoce, se niega a abrir con el error centinela **`ErrSchemaTooNew`** (sin degradar
  ni avanzar la versión), en vez de continuar. Cierra el hallazgo *medium* «sin guarda de compatibilidad de
  esquema en runtime» de la auditoría de producibilidad (`audit/2026-07-08`). Aditivo, golden intacto.
- **Captura automática C4 — capturar el par error→fix al resolver telemetría.** El par error→fix es *la
  memoria de código más valiosa*, y Musubi ya lo tenía en la tabla de telemetría (`musubi_log_error` guarda
  el error + el parche propuesto) pero moría ahí. Ahora, cuando se llama **`musubi_resolve_telemetry`** (el fix
  se confirmó), se **captura el par como memoria local** — `"Error en <file>: <mensaje> → Arreglado con:
  <parche>"` (`procedural`, deduplicada) — recuperable por recall. Model-free, best-effort (un fallo de la
  captura no rompe el resolve), y solo captura si hay un parche registrado (anti-ruido). Queda **local** (al
  compartir por `promote`, la redacción de C2 lo limpia). **Cierra el track de captura automática (C1 proactiva
  + C2 redacción + C3 commits + C4 error→fix).** Aditivo: sin tools nuevas, golden intacto.
- **PC auto-configurable P2 — `musubi provision` deja el proyecto seteado.** P1 conectaba la máquina al
  cerebro; ahora `provision` también **deja el proyecto 100% seteado como Musubi** — workspace `.musubi/`,
  skills cognitivas, templates SDD y los **4 hooks** (SessionStart con el priming de captura proactiva **C1**,
  UserPromptSubmit, PreToolUse, y **Stop** con la captura de commits **C3**) — reusando los helpers de `setup`.
  Consecuencia: una máquina recién provista tiene **la captura automática y la memoria de código funcionando
  de fábrica**, no solo la conexión al cerebro. Best-effort (un fallo del setup local no revierte la conexión
  ya lograda), idempotente, y `--dry-run` no muta. Aditivo: `setup` sin cambios, golden intacto.
- **Captura automática C3 — captura de commits (red de seguridad determinista).** Un hook **`Stop`**
  (`musubi capture --hook-mode`) que, al cerrar cada turno, captura los **commits nuevos** del repo como
  memoria **local**, sin depender del agente ni de un LLM — el mensaje de commit **es el "por qué"** destilado
  por el humano. Model-free: lee `git log` incremental desde el último HEAD capturado (guardado en meta,
  global al repo; la primera vez solo el HEAD, para no ingerir toda la historia), **clasifica por keyword**
  (fix/bug/security → alto; feat/refactor/perf → medio; y **omite** merge/wip/cortos y chore/docs/style/test/
  build/ci), y guarda subject + body + archivos tocados, **deduplicado**. **No-op silencioso** si no es un
  repo git, no hay commits nuevos, o todos son triviales. La captura es **local** (nunca comparte: un secreto
  de un diff no cruza; compartir pasa por `promote`, que C2 redacta). `setup` registra el hook `Stop`
  (idempotente). Cierra el track de captura automática (C1 proactiva + C2 redacción + C3 commits). Aditivo:
  sin tools nuevas, golden intacto.
- **Captura automática C2 — redacción de secretos en el borde a `shared` (más seguro que el SOTA).** Como la
  captura es **shared-by-default**, un secreto que el agente capture no debe terminar en el cerebro que ve
  todo el equipo. Nuevo paquete `internal/redact` (model-free, **sin dependencias nuevas**): `Redact(text)`
  combina **reglas por forma** (AWS/GitHub/Stripe/Google/JWT/PEM/bearer/`KEY=valor`, RE2) con un **catch-all
  de entropía de Shannon** para formatos desconocidos, respetando una allowlist de placeholders (y **sin
  tocar git SHAs**). La guarda se aplica **en el borde donde una observación se vuelve `shared`**
  (`saveObservation` con scope shared y `PromoteObservation`): el contenido se limpia ANTES de persistir, y
  como el outbox reconstruye el payload desde la fila, **nada sin redactar cruza al central por ninguna ruta**.
  La memoria **`local` queda intacta** (los secretos pueden vivir en tu propia máquina; se limpian solo al
  compartir). Ningún competidor (Mem0/Letta/Zep/Copilot) documenta redacción. Aditivo: sin deps, sin tools
  nuevas, golden intacto.
- **Captura automática C1 — captura proactiva (el cerebro aprende mientras trabajás).** Musubi ya
  RECUPERA memoria solo; ahora también **empuja a capturarla sola**. El hook SessionStart inyecta un
  bloque conciso (`startup_capture`) que instruye al agente a **guardar por su cuenta, sin que se lo
  pidan**, los aprendizajes durables — **decisiones** (el porqué), **gotchas**, **estado del trabajo**
  y **hechos de código** — con las tools correctas y con criterio de salencia (solo lo reusable/no-obvio,
  nada de trivialidades); además **desambigua "shared"** = memoria compartida del cerebro, NO un tag ni
  commit de git. El recordatorio por turno pasa a ser **prescriptivo** (nombra qué capturar, no solo el
  conteo). El bloque **respeta el hook silencioso**: viaja solo cuando el arranque ya tiene algo que
  decir. La extracción la hace el agente (que es el LLM), no Musubi — costo LLM cero, coherente con el
  diseño model-free. Es la Fase 1 del track de captura automática; la captura es **local** (compartir al
  cerebro llega en una fase posterior, detrás de la redacción de secretos). Aditivo: sin tools nuevas, golden intacto.
- **PC auto-configurable P1 — `musubi provision` (unir una máquina al cerebro).** Nuevo subcomando que
  lleva un equipo a estar **unido al cerebro central** en un comando, idempotente y cross-platform. El
  corazón es un **preflight de red VPN-agnóstico**: sonda dos caminos (un destino público de control por IP
  literal —sin DNS— y el cerebro en el tailnet) y clasifica el entorno en `Clean` / `SplitExcluded`
  (el runtime va directo y solo ve la malla) / `Tunneled` (el runtime está atrapado en el túnel y no ve la
  malla) / `Isolated`, con **guía accionable en prosa sin nombrar ningún producto de VPN**. Si el cerebro no
  es alcanzable, **frena el self-check y explica el paso faltante** en vez de fallar en silencio. Luego
  asegura Tailscale, aplica la **apertura del tailnet** (reglas de firewall `TS-Allow-Tailnet-In/Out` en
  Windows / allowlist de subred en Linux, idempotentes; si falta admin, instruye sin abortar), **cablea el
  `.mcp.json`** con las entradas `musubi` (local) y `musubi-cerebro` (remota, bearer por `${MUSUBI_TOKEN}` —
  el secreto nunca toca el archivo) preservando lo existente, y hace el **self-check reach + auth** contra el
  cerebro. También deja el bloque **`sync:`** en el `.musubi/config.yaml` (idempotente, preservando la config
  previa) para que el daemon LOCAL **suba solo la memoria `shared`** al cerebro (outbox de F2) — con
  `allow_insecure_token: true` porque el central es `http://` sobre el tailnet (WireGuard ya cifra); sin este
  paso el `.mcp.json` conectaba pero el auto-sync quedaba apagado. `--dry-run` diagnostica y muestra el plan
  sin mutar. Porta a Go la lógica probada en `deploy/connect-brain-*`. Aditivo: no agrega tools MCP (el golden no cambia).

### Changed
- **Cerebro híbrido — sync más robusto (offline-first de verdad).** Se corrigió una grieta de F2 que
  destapó una prueba real: un fallo **transitorio** del sync (cerebro central caído, VPN reconectando) que
  acumulaba `max_attempts` terminaba en **dead-letter permanente**, perdiendo memoria `shared` que sólo
  estaba temporalmente sin poder entregarse. Ahora un fallo transitorio (red/timeout/5xx/429) **nunca muere**:
  reintenta indefinidamente con backoff exponencial acotado; **sólo** un fallo permanente (4xx/params/auth)
  va a dead-letter. Además, dos tools nuevos le dan **ojos y una red de seguridad** al sync: **`musubi_sync_status`**
  (read-only) reporta cuántas observaciones están pendientes/enviadas/en dead-letter, la antigüedad de la más
  vieja pendiente y el último error; **`musubi_sync_requeue`** devuelve las que quedaron en dead-letter a la
  cola de envío (útil tras un corte). Aditivo y backward-compatible; con `sync.enabled=false` nada cambia.

### Added
- **Cerebro híbrido F2 — outbox durable + cliente de sync saliente (offline-first).** El conocimiento
  marcado `shared` (F1) ahora **viaja al cerebro central** por su cuenta. Cuando una observación se promueve
  o se guarda como `shared`, se encola una fila en una **tabla `outbox` durable** (migración v11, aditiva)
  **dentro de la misma transacción** que cambia el scope (*transactional outbox*: o quedan ambos o ninguno).
  Un **scheduler de drain** —arrancado en `daemon` y en `serve`, que **no toma el lock de dispatch**—
  reclama lotes con un `UPDATE … RETURNING` atómico (lease sobre `next_attempt_at`, con auto-recuperación
  de reclamos colgados) y los empuja al `musubi serve` central vía JSON-RPC `tools/call` →
  `musubi_save_observation` remoto, con el `id` de la observación como clave: la re-entrega es un no-op
  gracias al UPSERT `ON CONFLICT(id)` del receptor (**at-least-once con efecto exactly-once**). Es
  **offline-first**: si el central está caído la fila queda `pending` con *backoff* exponencial (jitter,
  tope) y drena sola al recuperarse; los errores permanentes (4xx) o el tope de reintentos van a
  *dead-letter* (`status='dead'`). Un **backfill** idempotente al abrir la DB siembra el outbox con las
  `shared` que ya existían de F1. El re-sync ante cambio de contenido se detecta por `content_hash`. Config
  nueva bajo `sync:` (`enabled` —**off por default**—, `central_url`, `auth_token_env` —el token **nunca**
  en el YAML, siempre por env var—, `drain_interval_seconds`, `batch_size`, `max_attempts`,
  `backoff_base/max_seconds`, `lease_seconds`, `allow_insecure_token`). Cero dependencias nuevas; el set de
  tools MCP no cambia; con `sync.enabled=false` el comportamiento es idéntico al de antes. Es la Fase 2 del
  track de 5 (F3 central multi-proyecto, F4 federated recall, F5 hardening).
- **Cerebro híbrido F1 — modelo de `scope` (local/shared) + `project_id` en la memoria.** Fundación del
  cerebro central compartido: cada observación lleva ahora un `scope` (`local`, default = comportamiento
  histórico; o `shared`, candidata a sincronizarse con el cerebro central en fases siguientes) y un
  `project_id` que la ata a su proyecto (migración v10, aditiva y backward-compatible). `musubi_save_observation`
  acepta un parámetro opcional `scope` (validado); un tool nuevo **`musubi_promote`** eleva una observación
  local a `shared` (idempotente). Internamente se **centralizó el predicado de visibilidad**
  (`archived = 0 AND superseded_by IS NULL`) en una única constante (`visibleObsPredicate`), refactorizando
  las queries de lectura sin cambiar el SQL — el *seam* para el filtrado por scope que viene. Todo aditivo:
  las bases y observaciones previas se comportan idéntico (0 regresiones). Es la Fase 1 de un track de 5
  (F2 sync offline-first, F3 central multi-proyecto, F4 federated recall, F5 hardening).
- **Dashboard-cerebro (`musubi dashboard`): la memoria como grafo neuronal 3D en vivo.** Nuevo backend
  `internal/memory/braingraph.go` que expone las observaciones activas como **neuronas** y las
  `observation_relations` como **sinapsis** (`DbEngine.BrainGraph`), read-only y model-free —saliencia
  `importance*exp(-age/30)+ln(1+heat)` computada en Go, cap top-N, sin aristas colgantes—. `musubi export`
  suma el campo `brain` al snapshot y `musubi dashboard` lo renderiza en un canvas: cerebro 3D con
  **spreading-activation real** (solo dispara con actividad entre polls), HUD glass (salud/tokens/
  orquestación/dominios/actividad) y polling de `/api/snapshot`. El volumen **se expande simétricamente con
  la población** (radio ∝ N^⅓, encuadre estable) y el render se **autorregula por FPS** (LOD por
  prominencia, bloom sin `ctx.filter`, gobernador de calidad) para sostener miles de neuronas. Loopback-only,
  0 tokens, proceso aparte.
- **Scripts de despliegue del cerebro central en `deploy/`** (`install-musubi-brain.sh` +
  `connect-brain-linux.sh` / `connect-brain-windows.ps1`): montan Musubi como daemon MCP sobre HTTP
  (`musubi serve`) en un servidor Linux y conectan cada dispositivo cliente, en **un comando por
  máquina**. El de servidor es idempotente —binario+checksum, `restorecon` de SELinux, workspace,
  bloque `service:`, token que **no se regenera** al re-correr, unit systemd, `tailscale0` en la zona
  `trusted` del firewall, y verificación de `/readyz`+`tools/list`—. Los de cliente hacen el onboarding
  (Tailscale, allowlist de NordVPN, entrada remota `musubi-cerebro` en el `.mcp.json` con el token por
  referencia `${MUSUBI_TOKEN}`, y verificación con auth). Codifican el runbook de
  `docs/Server_Brain_Onboarding.md`.

### Changed
- **`backupDB()` migrado a `VACUUM INTO`**: el backup del auto-heal del `doctor` ahora es un snapshot
  consistente y compactado en vez de una copia cruda del archivo.

## [0.77.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (d) — **el recordatorio de captura cuenta las tres superficies**.
Cierra el Frente #3 y la auditoría. Correctness del loop dirigido, model-free, sin migración.

### Fixed
- **El recordatorio de captura ya no da falsos positivos con `save_fact`/`save_code`**: `buildCaptureReminder`
  usaba `CountObservations` como señal de "se guardó algo" entre turnos, así que persistir un **hecho**
  (`musubi_save_fact`) o un **gist de código** (`musubi_save_code`) no reiniciaba el contador y el nudge saltaba
  igual —aun cuando el propio texto sugería `musubi_save_fact`—. Ahora la señal deriva de un nuevo
  `CountSavedItems()` que suma las tres superficies (`observations` + `relations` + `code_memory`) en una sola
  query; es un total monótono ante cualquier save nuevo. La lógica de umbral/turnos/session-scoping no cambia.

## [0.76.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (c) — **delta del run en `musubi_workflow`**. Las acciones
incrementales dejan de re-serializar la definición inmutable del workflow en cada respuesta. Model-free, sin
cambios de esquema ni de estado persistido.

### Changed
- **Las respuestas incrementales de `musubi_workflow` omiten `definition`**: cada acción (`complete`,
  `provide`, `verify`, `rollback`, `abort`, `compensated`) devolvía el `run` COMPLETO, incluido el DAG entero
  (`definition`: todos los steps con títulos y directivas `verify`/`await`/`compensate`) — que **no cambia tras
  `start`**. En un run de varios pasos era el mayor bloque repetido del payload. Ahora esas acciones devuelven
  una vista `run` sin `definition` (conserva `run_id`/`workflow_id`/`status`/`step_status`/`step_results`/
  `step_iters`); el snapshot completo —con `definition`— sigue disponible en `start`, `status` y `resume` (los
  puntos donde el caller no tiene estado previo). Solo cambia la SERIALIZACIÓN de la respuesta: el estado en
  SQLite y la capa de memoria quedan intactos.

## [0.75.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (b) — **búsqueda gist-first**. `musubi_search_semantic` y
`musubi_search_keyword` dejan de serializar la `Observation` completa por hit (el mayor costo de tokens
model-facing recurrente que quedaba) y devuelven titulares acotados por presupuesto. Model-free, sin migración.

### Changed
- **`musubi_search_semantic` / `musubi_search_keyword` son gist-first**: antes ambas devolvían el objeto
  `Observation` COMPLETO (contenido full × N hits) en cada llamada. Ahora devuelven por hit
  `{id, topic_key, gist, similarity?, full_tokens}` —el titular extractivo en lugar del contenido— con el
  payload total acotado por un presupuesto de tokens (`searchGistBudget`, top-1 garantizado). El contenido
  completo se hidrata bajo demanda por `id` con `musubi_recall`/`musubi_memory_expand`. `similarity` solo
  aparece en la búsqueda semántica; `full_tokens` informa el costo de hidratar. Sin nuevos parámetros de
  schema (el `limit` existente sigue acotando la cantidad). Modelado en la capa MCP: las queries de memoria
  y el esquema quedan intactos.

## [0.74.0] - 2026-07-04

Auditoría del sistema de tokens, 3ª tanda — dos de los tres frentes que quedaban: **relevancia del recall por turno**
y **adelgazar el schema de tools** (costo fijo por turno). Ambos model-free y sin perder eficacia.

### Fixed
- **El recall por turno filtra stopwords** (relevancia): la superficie MÁS caliente (recall en cada
  UserPromptSubmit) corría un MATCH de FTS **crudo** —`el`/`de`/`la`/`the`/`of` incluidos— que diluía el OR y dejaba
  que la recencia volcara el orden, colando memorias tangenciales-pero-recientes. Ahora usa un nuevo flag
  `RankedFTS` que descarta stopwords (es/en) y tokens de 1 runa antes de armar la query (con fallback seguro si todo
  era ruido). **Opt-in**: el recall del tool `musubi_recall` queda bit-a-bit igual; solo cambia el recall por turno.

### Changed
- **Descripciones de tools más densas** (−~625 tok/turno de costo FIJO): las 5 mega-descripciones
  (`musubi_workflow`, `musubi_work`, `musubi_debate`, `musubi_sdd`, `musubi_author_skill`) embebían el protocolo
  completo paso-a-paso, pagado en el schema cada turno. Se recortó el racional y la verbosidad redundante
  **preservando cada action y feature con su trigger→action→params** (la respuesta de la tool guía las features
  avanzadas cuando aplican). El schema de las 31 tools bajó de ~30.1k a ~27.6k chars. Sigue en 31 tools.

### Notes
- Frente que queda de la auditoría (#3): cachear `gist_tokens` (necesita migración), `search_semantic`/`keyword`
  gist-first con budget, delta en las respuestas de `musubi_workflow`, y `capture_reminder` contando todas las
  superficies de guardado. Documentado en `audit/2026-07-04-token-system`.

## [0.73.0] - 2026-07-04

Auditoría del sistema de tokens, 2ª tanda — **precisión del estimador** (los hallazgos #8/#9). Ambos son puro win,
model-free y 100% bajo control del server: mejoran la exactitud de la estimación de tokens SIN sacrificar recall. El
estimador versionado recomputa la columna `tokens` de todas las filas al abrir el motor (aplica al reiniciar).

### Fixed
- **Estimación por-segmento del markdown** (#8): antes, un solo fence ` ``` ` en una observación clasificaba **todo**
  el blob como código (`/3.4`), sobre-estimando ~12–17% y haciendo que el recall empaquetara **menos memoria de la
  que cabía**. Ahora `EstimateTokens` separa los bloques de código (entre fences) de la prosa y estima cada parte con
  su divisor. Recupera budget de recall real. JSON estructural se sigue estimando como blob completo.
- **Peso de caracteres no-ASCII** (#9): los acentos/emoji se contaban por runa y se dividían por el divisor de prosa
  (`/4`), **sub-estimando** la prosa acentuada — dirección insegura para un presupuesto, y todo el corpus es en
  español. Ahora los no-ASCII no-CJK se cuentan más densos (`divNonASCII=2.0`, ~0.5 tok/char), restaurando el sesgo
  conservador. La calibración opt-in descuenta esta contribución fija al ajustar los divisores por tipo.

### Notes
- El estimador pasa a `v3-seg-nonascii`: al reiniciar, recomputa `tokens`/`gist` de todas las filas una vez
  (idempotente). Pendientes mayores de la auditoría aún abiertos: adelgazar el schema de tools (~7.500 tok/turno,
  con el asterisco del prompt-caching client-side) y el floor de relevancia del recall por turno. Sigue en 31 tools.

## [0.72.0] - 2026-07-04

Auditoría del sistema de ahorro de tokens (4 agentes anclados en código + verificación adversarial) → **bundle de
quick-wins**: menos tokens sin sacrificar una gota de recall. El veredicto fue "sano ~8.5/10; el desperdicio está
concentrado, no es arquitectónico". Este release ataca 5 de los hallazgos de mayor ROI y riesgo casi nulo.

### Changed
- **Respuestas JSON de las tools compactas** (`jsonResult`: `MarshalIndent`→`Marshal`): la indentación era ~**28%**
  de whitespace puro en cada payload estructurado (recall, tokens, workflow, search, doctor…) que el cliente MCP
  parsea igual. −28% en toda respuesta JSON de tool.
- **`content_hash` fuera del payload model-facing** (`RecallItem`, `json:"-"`): eran 64 hex (~25 tokens) por item de
  maquinaria server-side (la inyección diferencial la consume in-process en Go) que viajaban al modelo sin que los
  use. Se conserva como campo Go; deja de serializarse.

### Fixed
- **`turn_batch` sin delta guard**: era el único bloque por turno que se re-inyectaba **cada turno** mientras había
  un batch activo (~53 tok/turno). Ahora usa el mismo `turnSurfaceChanged` que los otros bloques: solo emite cuando
  el progreso del batch cambió.
- **El recall por turno ignoraba los toggles semánticos**: la superficie MÁS caliente (recall en cada
  UserPromptSubmit) corría léxico puro, sin Stemming/Cooccurrence/GraphCentrality —los puentes model-free que la tool
  `musubi_recall` sí usa (Tracks 14/B4)—. Ahora se propagan desde `memory.*`: **mismos tokens, más relevancia**.
- **Metaclaves de captura no session-scoped**: `loop_obs_seen`/`loop_turns_since_save` sangraban entre sesiones (una
  sesión nueva heredaba el contador de la anterior y podía disparar el nudge de captura sin actividad propia). Ahora
  llevan el `session_id` como sufijo, igual que el estado delta.

### Notes
- Diferido de este bundle (necesita señal nueva en el recall + más superficie de test): floor de relevancia (no
  inyectar recencia disfrazada en prompts genéricos). Documentado en `audit/2026-07-04-token-system`. Pendientes
  mayores de la auditoría: adelgazar el schema de tools (~7.500 tok fijos/turno) y precisión del estimador
  (segmentación de markdown, peso no-ASCII). Sigue en 31 tools.

## [0.71.0] - 2026-07-04

Track 15, S1 (cierre) — **guard de cambio de modelo de embedding**. Con la Capa 2 (StaticProvider) ya es fácil
alternar tablas de embedding; si dos modelos comparten dimensión, sus vectores no son comparables por coseno pero el
`dim-guard` existente no los distingue (mezcla silenciosa que degrada el recall). Este release cierra ese borde con
la opción proporcionada: **visibilidad**, no maquinaria pesada.

### Added
- **Aviso de cambio de modelo de embedding** — al arrancar, si el modelo activo (`Provider.Name()`) cambió respecto
  del último run **y hay vectores previos de otro modelo**, se logea un warning claro (con conteo y acción sugerida:
  limpiar/re-embeber si el cambio fue same-dimension). Registra el modelo activo en `meta` para el próximo arranque.
  **Sin migración, sin cambiar el recall, no-op sin embedder.** Cubre el borde same-dim que el `dim-guard`
  (CosineSimilarity falla si dim≠, IVF descarta la dimensión minoritaria) no alcanza. Descartada la provenance
  per-row completa (columna `model_id` + filtro) por sobre-ingeniería para una realidad de un embedder por proceso.
  Cierra el backlog de Track 15 (S3 multilingüe = elección de asset sin código; Capa 1 y TLogic diferidos por
  decisión de ROI). Sigue en 31 tools.

## [0.70.0] - 2026-07-04

Track 15, Capa 2 — **semántica model-free _at inference_**. La auditoría dejó como frontera de fondo que Musubi, por
ser model-free, no "entiende": su recall combina señales léxicas/estructurales pero no capta sinonimia real
(`deploy`≈`despliegue`) salvo que un embedder externo esté configurado. Este release da esa capacidad **sin runtime
de modelo y sin cgo**: un provider que genera embeddings con una **tabla estática** token→vector (formato
model2vec/POTION) + mean-pooling — cero forward pass de red neuronal.

### Added
- **`StaticProvider` (embedding.provider=`static`)** — embeddings por lookup en una tabla estática destilada
  (model2vec/POTION) + mean-pool + L2-normalize, con un **WordPiece BERT propio bit-exacto** (BertNormalizer con
  strip-accents por NFD, greedy longest-match, `[UNK]`). Cae directo en el pipeline ya existente (tabla `embeddings`
  + índice IVF + coseno + fusión RRF) — **cero cambios en memory/mcp**. La tabla la aporta el usuario en
  `embedding.static_path` (bring-your-own-table: `model.safetensors` + `tokenizer.json`); **off por defecto**
  (`NoopProvider`), feature 100% aditiva. Bit-exactitud validada contra model2vec (12 strings EN/ES/acentos/
  puntuación, cosine 1.000000). Claim honesto: **"model-free _at inference_"** — la tabla se destiló offline de un
  sentence-transformer (misma categoría que servir vectores GloVe), **no** "model-free absoluto". Única dep nueva:
  `golang.org/x/text` (NFD del strip-accents). Sigue en 31 tools.

### Notes
- Diferido con criterio: provenance/homogeneidad de vector por `model_id` (el dim-guard existente ya cubre el switch
  de modelos de distinta dimensión), default multilingüe (`potion-multilingual-128M`), y bundling/auto-download del
  asset (hoy bring-your-own-path).

## [0.69.0] - 2026-07-04

Track 14, #2 — **2ª ola de semántica model-free**: stemming query-time por prefijo. Ataca el miss de recall más
común (morfológico): sin esto, buscar "deploy" no encontraba una memoria que dice "deploys" o "deployment", porque
el FTS matchea tokens exactos.

### Added
- **Stemming por prefijo en el recall** (sin dependencia, sin re-indexar): con el flag on, cada término de la query
  se reduce a una raíz con un stemmer **liviano y conservador** (recorta un sufijo de flexión ES+EN dejando raíz
  ≥4 runas; términos <5 quedan intactos) y se matchea por **prefijo FTS** (`"deploy"*`), atrapando las variantes de
  sufijo (`deploy`/`deploys`/`deployment`, `casa`/`casas`). Fiel a la identidad: **cero dependencia nueva** (se
  descartó Snowball para no romper la disciplina de 3 deps), **sin re-indexado ni migración**, model-free y
  determinista. Config `memory.recall_stemming` (default ON; `false` desactiva); off por zero-value preserva el
  match exacto histórico bit-a-bit. Honesto: cubre variantes de **sufijo**, no cambios de raíz (`despliegue`↔
  `desplegar` — eso requeriría un stemmer completo). Segunda ola de #2 tras la co-ocurrencia/PRF. Sigue en 31 tools.

## [0.68.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), #2 — **primer slice de semántica model-free** en el recall. La auditoría marcó
como gap estratégico que, sin embedder externo, la única señal de contenido era léxica (FTS token-exact): "deploy"
no encontraba una memoria que dice "despliegue". Este release agrega un **puente de vocabulario derivado del
corpus**, sin LLM ni modelo, manteniendo el determinismo.

### Added
- **Recall por co-ocurrencia / pseudo-relevance feedback (PRF)** — 6ª señal RRF opcional: tras el recall léxico,
  toma los top resultados (pseudo-relevantes), cosecha los términos que **co-ocurren** con la query en ellos
  (aparecen en ≥2 de esos docs, excluyendo la query y stopwords) y corre un 2º FTS con esos términos para **traer
  observaciones con vocabulario distinto** que la query original no encontró (el puente `deploy`↔`despliegue`). La
  "semántica" se **deriva del corpus** — no se importa de un modelo: pura tokenización + conteo + FTS, determinista.
  Realización **index-free** de la co-ocurrencia (sin índice global persistido, sin tabla, sin migración). Config
  `memory.recall_cooccurrence` (default ON; se desactiva con `false`); off por zero-value preserva el recall
  histórico bit-a-bit. Honesto: el valor es corpus-dependiente (con memoria escasa degrada a no-op). Primer paso de
  #2; quedan olas futuras (stemming EN+ES, LSA/SVD, índice global de co-ocurrencia). Sigue en 31 tools.

## [0.67.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), ola de endurecimiento — A2 (limpieza de código muerto, #4) + A3 (blindaje de
tests, #5).

### Added
- **Fuzzing sobre los parsers model-free** (primeros fuzz tests del repo, cerrando el hueco "cero fuzzing" de la
  auditoría): `FuzzSimilarity` (Jaccard de trigramas — invariantes [0,1] + simetría + no-NaN), `FuzzEvalCondition`
  (parser de expresiones `when`/`repeat_while` — determinismo + no-panic), `FuzzBuildFTSQuery` (constructores de
  query FTS — tolerancia a puntuación/unicode/bytes nulos). ~50–100k ejecuciones por fuzzer sin panics.
- **Test de concurrencia REAL del claim de la pizarra** (`TestClaimWorkUnitConcurrentNoDoubleClaim`): N agentes en
  goroutines compiten por M unidades; verifica que ninguna se reclama dos veces y que se reclaman exactamente las M
  (antes la "atomicidad" sólo se probaba en secuencial). Se apoya en el `UPDATE...RETURNING` bajo el write-lock de
  SQLite (`busy_timeout`); CI lo corre con `-race`.

### Removed
- **Cruft genuino eliminado**: `writeMCPConfig` (envoltorio duplicado de `writeMCPConfigAt`, sólo lo usaba su
  test) e `internal/codeintel/imports.go` completo (`ExtractImports` y helpers, usado únicamente por su propio
  test, sin ningún feature que lo consumiera). Al auditar el "código muerto" que marcó la auditoría se distinguió
  cruft de **andamiaje intencional**: se PRESERVARON `bootstrap.RemoteEntry`/`MergeRemoteMCPServer` (groundwork
  documentado del home-server: apuntar clientes al `musubi serve` central sobre la VPN) y `FakeRunner` (falso de
  git usado por los tests; `deadcode` lo marca sólo porque analiza desde `main`, no desde los tests). Borrar
  groundwork por "reducir superficie" habría destruido trabajo planeado; se removió sólo lo genuinamente muerto.

## [0.66.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), A1 — **modelo de fallo del motor de workflows**. La auditoría profunda encontró
un bug funcional latente: `RunAborted` estaba declarado pero nunca se usaba, y un step `failed` dejaba el run en
`running` para siempre (run zombie) con sus dependientes bloqueados, sin forma de abortarlo. Este release cierra ese
hueco: el estado del run ahora se **deriva** correctamente de los estados de sus steps, y hay un abort explícito.

### Fixed
- **Un run wedgeado por un step fallido ya no queda zombie**: si un step queda `failed` y bloquea todo progreso
  posible, el run transiciona a un estado terminal `failed` (con evento `run_failed` en el journal), en vez de
  quedar `running` indefinidamente. La transición es **derivada y model-free** (`computeRunStatus`): mientras haya
  progreso posible —una rama independiente en curso, un gate humano/verify sin resolver, un step con `when` que
  podría saltarse— el run **no** se marca failed (sin falsos-fallo). El happy-path (`run_done`) queda idéntico.

### Added
- **`musubi_workflow action=abort`** (run_id, razón opcional en `result`): aborta explícitamente un run atascado o
  no deseado → estado terminal `aborted` (evento `run_aborted`), y deja de despachar steps. Idempotente; falla si el
  run ya concluyó con éxito (`done`/`compensated`). Un run `failed`/`aborted` **todavía se puede compensar** con
  `rollback` (saga LIFO de los steps completados). Un run terminal (done/failed/aborted/compensated) no despacha más
  steps. Sin migración (los estados nuevos fluyen por la columna `status` existente). Sigue en 31 tools.

## [0.65.0] - 2026-07-04

`musubi setup` ahora **refresca las skills cognitivas manejadas** cuando el binario las actualiza, **sin pisar las
que el usuario editó**. Antes, `writeCognitiveSkills` saltaba cualquier archivo existente, así que un update de una
skill (p. ej. `adversarial-review` → `musubi_debate`) nunca llegaba a los repos ya inicializados — había que copiar
el `.yaml` a mano a cada repo. Ahora cada skill lleva su propia prueba de integridad y la propagación es un
`musubi setup`.

### Changed
- **Refresh de skills manejadas por checksum**: cada skill cognitiva que escribe Musubi lleva un `managed_checksum`
  (sha256 de su contenido canónico, CRLF-agnóstico). En el próximo `setup`, Musubi lo usa para decidir de forma
  determinista: si el archivo sigue **exactamente** como Musubi lo escribió (checksum coincide) → lo **refresca** a
  la versión nueva; si el usuario lo **editó** (checksum no coincide, o el archivo no parsea) → lo **preserva**. Un
  archivo legacy idéntico a la versión actual se **adopta** (gana el checksum, sin cambiar contenido). **Regla de
  oro (safety): ante la mínima duda, preservar** — Musubi nunca pisa trabajo del usuario. Idempotente: un `setup`
  sin cambios no reescribe ni reporta nada. `setup` informa qué skills actualizó. Campo `managed_checksum` opcional
  (omitempty), no afecta el loader ni el gate de calidad; solo aplica a las skills cognitivas (no a las escritas a
  mano ni a las de auto-discovery). Cierra el hueco de propagación que obligaba a copiar skills a mano a los repos.

## [0.64.1] - 2026-07-04

Cierra el lazo de v0.64.0: la skill cognitiva **`adversarial-review`** ahora USA el subsistema `musubi_debate` en
vez de describir el patrón como prosa sobre la pizarra. Así el determinismo y la persistencia que agregó el debate
se aprovechan de verdad en la revisión adversarial (y en la fase verify del flujo SDD).

### Changed
- **`adversarial-review` cableada a `musubi_debate`**: la revisión adversarial pasa de coordinar escépticos por la
  pizarra (`musubi_work`) con conteo de mayoría "a mano" a orquestar un **debate estructurado**: `open` (rounds=2,
  quorum=mayoría) → cada escéptico (un LENTE: correctitud/seguridad/repro/contrato) postea su refutación con `post`
  → `advance` habilita una 2ª ronda de **crítica cruzada** (cada uno ve y rebate las refutaciones ajenas) → `vote`
  (real|no_real) → `tally` da el **veredicto por mayoría DETERMINISTA y persistido**. no_consensus (empate/sin
  quórum) ⇒ se defiere el juicio a `musubi_judge`. El veredicto y las posturas quedan reproducibles. Solo cambia la
  plantilla de la skill (model-free); ninguna tool nueva.

## [0.64.0] - 2026-07-04

Debate multi-agente (**Society of Minds**) como subsistema ejecutable y determinista, model-free — reabriendo C3,
que en Track 13 se había descartado como subsistema. Un audit del código (con evidencia file:line) confirmó que el
andamiaje del debate se compone solo PARCIALMENTE de las primitivas existentes: la skill `adversarial-review` ya lo
simula como PROSA para el LLM, pero faltan tres mecanismos estructurales para tenerlo como topología ejecutable
(fan-out/rondas parametrizados, agregador N-ario, unidad multi-postura). Este release provee los dos que son
model-free —posturas atribuidas por ronda (crítica cruzada persistida) y tally determinista— y deja el juicio
semántico donde corresponde: en el LLM. **Primer incremento del catálogo desde hace varias olas: 30 → 31 tools**
(un subsistema genuinamente nuevo justifica su tool propia, como `musubi_work` y `musubi_workflow`). Migración v9.

### Added
- **`musubi_debate` — debate multi-agente model-free** (acciones `open` / `post` / `advance` / `vote` / `tally` /
  `status`): Musubi NO razona — estructura las rondas, PERSISTE las posturas atribuidas por agente y ronda (crítica
  cruzada reproducible) y CUENTA los votos; los sub-agentes (LLM) producen las posturas, las críticas y los votos.
  Ciclo: `open` (topic, rounds, quorum opcional) → N sub-agentes postean con `post` → `advance` cierra la ronda y
  devuelve las posturas previas como material de crítica para la siguiente → `vote` → `tally`. El **tally es 100%
  determinista**: gana el `choice` con el máximo ESTRICTO de votos que alcance el quórum → el debate se cierra con
  ese ganador; empate, bajo quórum o sin votos ⇒ `no_consensus` (sigue abierto: se puede `advance`+re-votar, o
  deferir el juicio a `musubi_judge`). El juicio SEMÁNTICO (elegir/sintetizar) se queda en el LLM. Migración v9
  (`debates`, `debate_postures` con `UNIQUE(debate_id,round,agent)`, `debate_votes` con `UNIQUE(debate_id,agent)`,
  `ON DELETE CASCADE`). Subsistema aislado y aditivo: no toca recall/work/workflow. Multi-Agent Debate / Society of
  Minds. **El catálogo pasa de 30 a 31 tools** (incremento deliberado).

## [0.63.0] - 2026-07-03

Track 13 — B4 (memoria más inteligente, cierre). **Centralidad de grafo como 5ª señal RRF del recall**, la última
pieza de la receta HippoRAG que faltaba, dogfoodeada por el flujo SDD completo con verificación adversarial;
model-free / Go-puro / aditiva. Hallazgo de scoping: la fusión RRF del recall **ya era híbrida** (keyword FTS +
recencia + frecuencia + semántica vectorial coseno, T5.7 R2) — "B4 = FTS + semántico vía RRF" ya estaba entregado.
Lo único que faltaba de HippoRAG era la señal de **centralidad de grafo**, que hoy solo corría sobre el grafo de
**hechos** (`recall_facts`), no sobre observaciones. Catálogo en 30 tools; sin migraciones (todo derivado al vuelo).

### Added
- **Centralidad de grafo en el recall de observaciones** (5ª señal RRF, config `memory.recall_graph_centrality`,
  **default ON**): una observación que es **hub** de un cluster relacionado (muchas `related`/`supersedes`/
  `conflicts_with` en `observation_relations`) sube en el ranking aunque el FTS/vector no la priorizara
  (*spreading activation*, estilo HippoRAG). Se computa por **Personalized PageRank** sobre el grafo de relaciones
  vivo (ambas puntas no archivadas ni superseded, no dirigido), sembrado uniformemente en el pool de candidatos ya
  recuperado y **rerank-only** (no incorpora candidatos nuevos, a diferencia del pool vectorial). **DERIVE-not-store**:
  se deriva al vuelo, sin tabla de scores. Reutiliza el kernel de power-iteration de PageRank (extraído a
  `pprPowerIteration`, compartido con `recall_facts`; equivalencia one-hot verificada). El `zero-value` de código
  preserva el comportamiento histórico **bit-a-bit** (equivalencia probada); se activa por config (double-default,
  patrón de `decay_reinforcement_k`). Se desactiva con `recall_graph_centrality: false`.

## [0.62.0] - 2026-07-03

Track 13 — Ola C (orquestación avanzada). **Contract-Net bidding** sobre la pizarra multi-agente, model-free
y aditivo, dogfoodeado por el flujo SDD completo con verificación adversarial. C1 (pipelines declarativos PDL/SAMMO)
resultó **ya cubierto** — los workflows de Musubi ya son datos declarativos (defs YAML en `.musubi/workflows/`,
DAG, condicionales, loops, expresiones). C3 (debate topologies) se **descartó como subsistema**: el patrón se
compone con las primitivas existentes (verify-gate + Reflexion, pizarra multi-agente, `musubi_judge`) sin agregar
framework. Catálogo en 30 tools; una migración aditiva (v8).

### Added
- **Contract-Net bidding en la pizarra multi-agente** (`musubi_work` acciones `bid` / `award` / `bids`): cuando
  los sub-agentes difieren en aptitud, en vez de asignar por *claim* de orden de llegada (first-come), la unidad
  se **anuncia** y los agentes **ofertan** (`bid`, un score donde **mayor = mejor** aptitud/confianza, que produce
  el propio agente — model-free); el orquestador revisa con `bids` y **adjudica** con `award` a la mejor oferta.
  La adjudicación **reusa la maquinaria de lease/TTL/fencing** existente: la unidad queda `claimed` por el ganador
  y sigue el flujo `heartbeat`/`complete` normal. Determinista (desempate por antigüedad y agente), atómico
  (`UPDATE ... RETURNING` guardado por `status='open'` — un doble `award` es no-op). Coexiste con el claim
  first-come (el orquestador elige el protocolo por unidad). Migración v8 (`work_bids`, con `ON DELETE CASCADE`
  al limpiar el batch). Contract-Net (Smith, 1980).

## [0.61.0] - 2026-07-03

Track 13 — Ola B (memoria más inteligente). Cuatro features sobre el pilar de memoria, cada una dogfoodeada por
el flujo SDD completo con verificación adversarial, todas **model-free / Go-puro / aditivas**: recall asociativo
por **Personalized PageRank**, **tipo de memoria** (mem_type) con olvido diferenciado, **refuerzo Ebbinghaus** del
olvido (heat) y **consultas de camino** en el grafo. El catálogo sigue en 30 tools. Una sola migración aditiva
(v7, `mem_type`); todo lo demás se deriva al vuelo. B4 (RRF hybrid) queda para una ola futura por riesgo.

### Added
- **Recall asociativo por Personalized PageRank** (`musubi_recall_facts rank=pagerank`): el BFS de vecindad, al
  cortar por `max_facts`, dejaba los hechos en orden de inserción (arbitrario) y perdía los relevantes a 2+ saltos.
  El nuevo modo corre **PPR** personalizado a la entidad semilla sobre el grafo de hechos y devuelve primero los
  más relevantes por cercanía asociativa multi-hop (score de un hecho = suma del PageRank de sus extremos). Power
  iteration pura (damping 0.85, hasta 200 iteraciones, corte por tolerancia L1), grafo no dirigido, masa
  conservada (nodos colgantes reinyectan al restart). Compone con lo bi-temporal: `rank=pagerank` + `as_of` da
  **PageRank point-in-time**. Default (`rank=''`/`bfs`) intacto (equivalencia byte-idéntica). **Sin migración**
  (se deriva de `relations`). HippoRAG.
- **Tipo de memoria (`mem_type`) con olvido diferenciado** (`musubi_save_observation mem_type=...`): cada
  observación puede declararse `semantic` (conocimiento estable), `episodic` (eventos puntuales) o `procedural`
  (cómo hacer algo) — enum model-free que aporta el agente. El tipo **modula el olvido**: episódico se enfría antes
  (peso de saliencia 0.6), semántico neutro (1.0), procedural más durable (1.5); sin tipo = 1.0 (idéntico a antes).
  Un guardado sin tipo **preserva** la clasificación existente (solo un tipo no vacío la cambia). Migración v7
  aditiva (`ADD COLUMN mem_type`). LangMem.
- **Refuerzo Ebbinghaus del olvido (heat)**: la vida media de la recencia deja de ser fija — cada acceso (repaso)
  la **alarga**, así las memorias frecuentemente accedidas ("calientes") resisten el archivado (spacing effect):
  `vida_media_efectiva = vida_media · (1 + K · ln(1+accesos))`. `K` es `maintenance.decay_reinforcement_k`
  (default 0.5, activo en el daemon; `K=0` reproduce exactamente el olvido previo). Clamp defensivo: nunca acelera
  el olvido. **Sin migración** (usa `access_count`). MemoryOS.
- **Consultas de camino en el grafo** (`musubi_recall_facts to=<entidad>`): responde "¿cómo se conecta X con Y?"
  devolviendo el **camino más corto** (cadena de hechos, en orden) entre dos entidades. BFS no dirigido con
  reconstrucción por predecesores; acotado por `max_hops`; compone con lo bi-temporal (`as_of` → camino
  point-in-time). **Sin migración** (se deriva de `relations`).

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

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.78.0...HEAD
[0.78.0]: https://github.com/codeabraham16/musubi/compare/v0.77.0...v0.78.0
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
