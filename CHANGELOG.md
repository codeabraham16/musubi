# Changelog

Todos los cambios notables de Musubi se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

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

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.43.0...HEAD
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
