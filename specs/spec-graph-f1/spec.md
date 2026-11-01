---
artifact: spec
schema_version: "1.0"
change: spec-graph-f1
status: draft
---

# Especificación — spec-graph · F1 (core + 7 tools + SKILL + regla)

> Vocabulario RFC 2119: **DEBE** / **NO DEBE** / **DEBERÍA** / **PUEDE**. Cada requisito es verificable
> y atómico, y está pensado para tener su test saboteable en la fase `tasks`/`implement`.

## Requisitos

### Detección y esquema
- **R1** — Un archivo markdown DEBE reconocerse como spec **si y sólo si** su frontmatter parsea y tiene
  `id` y `type` ambos presentes y no vacíos. Un `proposal.md`/`spec.md` del diario SDD (frontmatter
  `artifact`/`schema_version`/`change`/`status`, sin `id`+`type`) **NO DEBE** reconocerse como spec.
- **R2** — El esquema de frontmatter de una spec DEBE ser: `id` (string, requerido), `type` (enum,
  requerido), `status`, `title`, `parent` (id), `depends-on` (lista de ids), `references` (lista),
  `implements` (lista), `covers` (lista), `tags` (lista). Este esquema DEBE ser **independiente** del
  `schema_version:"1.0"` de las plantillas SDD (no se unifican).
- **R3** — `type` DEBE pertenecer al vocabulario cerrado de 5 valores, en este orden jerárquico:
  `goal-and-requirements`, `architecture-design`, `module-design`, `submodule-design`, `task-spec`.
  Un `type` fuera del vocabulario DEBE ser reportado como error por `musubi_spec_validate` (R12), pero
  **NO DEBE** impedir que el archivo se reconozca como spec (R1 manda).
- **R4** — El esquema **NO DEBE** incluir `anchors` en F1. La clave se difiere a F2; el parseo/edición
  lossless (R5) DEBE preservar cualquier clave desconocida que ya exista en un archivo, para que el
  esquema sea forward-compatible.

### Edición lossless
- **R5** — `create`/`update` DEBEN editar el frontmatter de forma **lossless**: el cuerpo en prosa DEBE
  quedar byte-a-byte idéntico, y los comentarios y el orden de claves del frontmatter DEBEN preservarse;
  sólo las claves efectivamente modificadas cambian.
- **R6** — Las tools **NO DEBEN** editar la prosa del cuerpo salvo en `create` (que escribe el cuerpo
  inicial). `update` opera **sólo** sobre claves de frontmatter; la prosa se edita con el `write`/`edit`
  normal, no con estas tools.

### Grafo e índice
- **R7** — El grafo DEBE construir aristas de **árbol** desde `parent` (cada spec con a lo sumo un padre)
  y aristas de **DAG** desde `depends-on`, `references` e `implements`.
- **R8** — Para cada arista hacia adelante, el grafo DEBE exponer su **arista inversa** consultable
  (p. ej. «quién depende de X», «hijos de X»).
- **R9** — El grafo DEBE detectar y marcar **`id` duplicados** (dos o más archivos que declaran el mismo
  `id`). La identidad de nodo es el `id`, no la ruta.
- **R10** — El índice DEBE **derivarse del filesystem** (no hay estado paralelo persistido que pueda
  divergir) y DEBE revalidarse por `(mtimeMs, size)`: una entrada cuyo `(mtime, size)` no cambió DEBE
  reusar el parseo cacheado; una que cambió DEBE reparsearse. El índice **NO DEBE** quedar rancio.

### Resolución de rutas (seguridad)
- **R11** — `resolveSpecPath` DEBE: (a) normalizar la ruta a **NFC**; (b) **rechazar** una ruta cuyo
  recorrido atraviese un **symlink**; (c) **rechazar** dos puntos/ADS de Windows (`nombre:flujo`,
  truco de drive/stream); (d) **rechazar** cualquier ruta que escape de la raíz permitida (`..`).

### Las 7 tools y su clasificación
- **R12** — DEBEN existir exactamente 7 tools `musubi_spec_*`:
  - `musubi_spec_grep` — busca specs (read-only), devuelve lista (id+ruta) aunque esté vacía (`[]`, no `null`).
  - `musubi_spec_get` — devuelve una spec por `id` o ruta: frontmatter parseado + cuerpo (read-only).
  - `musubi_spec_graph` — devuelve nodos + aristas hacia adelante e inversas, con scope/profundidad opcional (read-only).
  - `musubi_spec_validate` — reporta problemas estructurales: `type` desconocido, `parent`/`depends-on`/
    `references`/`implements` colgantes (target inexistente), `id` duplicados, falta de `id`/`type`,
    frontmatter malformado (read-only). En F1 **NO DEBE** reportar ranciedad (es F2).
  - `musubi_spec_create` — crea un `SPEC.md` con frontmatter + cuerpo inicial (write).
  - `musubi_spec_update` — edición lossless de frontmatter de una spec existente (write).
  - `musubi_spec_delete` — elimina una spec (write).
- **R13** — La clasificación en `registry.go` DEBE ser: `grep`/`get`/`graph`/`validate` **read-only**
  (toman RLock); `create`/`update`/`delete` **write** (toman Lock). Marcar read-only algo que muta
  **NO DEBE** ocurrir (es un lost-update que `-race` no detecta).
- **R14** — El golden de `tools/list` (`internal/mcp`) DEBE regenerarse para incluir las 7 tools y la
  suite `TestToolsListGolden` DEBE quedar en verde.
- **R15** — `create` DEBE fallar si el `id` ya existe (duplicado) o si la ruta escapa de la raíz (R11).

### SKILL, regla y no-funcionales
- **R16** — DEBE existir una SKILL `spec-graph` y una **regla always-on de 4 líneas** inyectada como
  sección de system-prompt.
- **R17** — El subsistema `internal/specgraph` y las 7 tools DEBEN ser **model-free** (ningún LLM en el
  camino) y **sin cgo**; el build DEBE pasar con `GOWORK=off` y la suite con `-race`.
- **R18** — El cambio DEBE ser **aditivo**: **NO DEBE** requerir migración de datos ni alterar el diario
  `specs/<cambio>/` existente (que sigue siendo `task-spec`).

## Escenarios

### Escenario: discriminar spec de diario SDD
- **Given** un `SPEC.md` con `id: auth-core` y `type: module-design`, y un `specs/x/proposal.md` con
  `artifact: proposal` pero sin `id`/`type`
- **When** el índice escanea ambos
- **Then** el primero entra al grafo como spec; el segundo **no** aparece como spec

### Escenario: round-trip lossless
- **Given** un `SPEC.md` con un comentario en el frontmatter, claves en cierto orden y un cuerpo en prosa
- **When** `musubi_spec_update` cambia sólo `status: draft` → `status: active`
- **Then** el cuerpo queda byte-a-byte idéntico, el comentario y el orden de las demás claves se preservan,
  y sólo la línea de `status` cambió

### Escenario: id duplicado
- **Given** dos archivos que declaran `id: auth-core`
- **When** `musubi_spec_validate` corre
- **Then** reporta un error de `id` duplicado nombrando ambas rutas

### Escenario: arista colgante
- **Given** una spec con `depends-on: [no-existe]`
- **When** `musubi_spec_validate` corre
- **Then** reporta una referencia colgante hacia `no-existe`

### Escenario: índice no rancio por mtime/size
- **Given** una spec ya indexada
- **When** su archivo cambia en disco (nuevo `mtime`/`size`) y se vuelve a consultar
- **Then** el índice reparsea esa entrada y refleja el cambio sin reinicio

### Escenario: resolveSpecPath rechaza escape por symlink
- **Given** una ruta de spec cuyo componente es un symlink que apunta fuera de la raíz
- **When** `resolveSpecPath` la resuelve
- **Then** la rechaza con error, sin leer ni escribir fuera de la raíz

### Escenario: resolveSpecPath rechaza dos puntos/ADS de Windows
- **Given** una ruta con `nombre:flujo` (ADS / truco de stream)
- **When** `resolveSpecPath` la resuelve
- **Then** la rechaza con error

### Escenario: clasificación read-only honesta
- **Given** las 7 tools registradas
- **When** se despacha `musubi_spec_get` concurrente con `musubi_spec_update`
- **Then** `get` toma RLock y `update` toma Lock; ninguna tool read-only muta estado

## Fuera de alcance
- Anclaje `anchors` y ranciedad automática (F2). `validate` en F1 **no** reporta ranciedad.
- Recall semántico sobre cuerpos de spec y cruce con `musubi_code_graph`/`musubi_impact` + overlay en
  `musubi_map` (F3).
- Federación de gists de spec al cerebro central por outbox (F4).
- Arbitraje de conflictos spec↔observación por `musubi_judge`/`supersedes` (F5).
- Tocar o migrar el diario `specs/<cambio>/`.

## Preguntas abiertas
- [ ] **Raíz de descubrimiento del índice:** ¿desde dónde escanea `SPEC.md` (raíz del repo, con ignore de
  `vendor/`/`node_modules/`, o una raíz configurable)? → decidir en design.
- [ ] **Mecanismo exacto de inyección de la regla always-on:** ¿qué hook/ruta la emite como sección de
  system-prompt (cache-friendly)? → design.
- [ ] **Superficie de `musubi_spec_grep` en F1:** ¿sólo campos de frontmatter, o también substring
  literal del cuerpo (model-free)? El recall semántico es F3. → design.
- [ ] **`musubi_spec_update` y el cuerpo:** confirmar que es frontmatter-only y que la prosa va por
  `write`/`edit` (R6). → design.
