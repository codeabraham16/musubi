---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f2-consulta-hook
status: draft
---

# Especificación — Consultar el grafo de código (Track 20 · F2-A)

> **F2-A** = índice de repo + tools de consulta read-only. El **hook `PreToolUse`** que inyecta
> el subgrafo antes de Read/Grep es **F2-B** (requiere opt-in de config del usuario) y queda fuera.

## Requisitos

### Índice de repo completo
- **R1** — DEBE existir una tool `musubi_codegraph_index` que recorra el proyecto (raíz =
  `projectPath`), derive el grafo de **cada paquete Go** y lo persista, para poblar el grafo del
  repo ENTERO (F1 solo poblaba disperso vía `save_code`). DEBE saltar directorios ocultos
  (`.git`, `.musubi`), `vendor/` y `testdata/`. Devuelve un resumen `{packages, nodes, edges}`.
- **R2** — El índice DEBE reusar el derivado y el store de F1 (`DerivePackage` +
  `UpsertPackageGraphFrom`), atribuido al proyecto de la credencial (no deja filas sin atribuir).

### Tools de consulta (read-only, scopeadas)
- **R3** — `musubi_code_graph` DEBE aceptar `symbol` (node_key) o `path` y devolver el nodo, sus
  **callees** (aristas `CALLS` salientes), **callers** (`CALLS` entrantes), e **imports** del
  archivo, en forma compacta. Con `path`, DEBE devolver los símbolos que el archivo contiene.
- **R4** — `musubi_impact` DEBE aceptar `symbol` y devolver el **cierre transitivo de callers**
  (quién llama, directa o indirectamente) hasta `max_depth` (default acotado), model-free (BFS
  sobre aristas `CALLS` entrantes). Es "qué se rompe si cambio X".
- **R5** — `musubi_map` DEBE devolver un panorama del proyecto: conteos (`nodes`, `edges` por
  kind), los **god-nodes** (top-N por grado = callers+callees) y **entry points** (funcs sin
  callers y `func:main`). Sin parámetros obligatorios.
- **R6** — Toda tool de consulta DEBE **anotar staleness**: para cada nodo devuelto cuyo
  `src_fingerprint` guardado difiera del fingerprint ACTUAL del archivo, marca `stale=true`
  (cierra el gap R12 de F1). El cálculo del fingerprint vive en la capa MCP (como `gistStale`).
- **R7** — Las 3 tools de consulta DEBEN ser `readOnly=true` y respetar el scope de la credencial
  (no ven el grafo de otro proyecto; federado sin scope).

### Integración
- **R8** — Registrar las 4 tools DEBE actualizar el golden de `tools/list` de forma consistente;
  `go build` + `go test` DEBEN quedar verdes, model-free, **sin cgo**.

## Escenarios

### Escenario: index puebla el repo
- **Given** un proyecto Go con varios paquetes y el grafo vacío
- **When** se corre `musubi_codegraph_index`
- **Then** el grafo queda poblado (nodes/edges > 0) y `code_graph` sobre un símbolo real
  devuelve sus vecinos

### Escenario: callers y callees
- **Given** `Alpha` llama a `beta` en el mismo paquete (grafo indexado)
- **When** `musubi_code_graph {symbol: "...#func:Alpha"}`
- **Then** `beta` aparece en callees; y `code_graph` sobre `beta` lista a `Alpha` en callers

### Escenario: impacto transitivo
- **Given** `A→B→C` (CALLS) indexado
- **When** `musubi_impact {symbol: "...#func:C"}`
- **Then** el resultado incluye `B` y `A` (callers transitivos), acotado por `max_depth`

### Escenario: staleness
- **Given** un símbolo indexado y luego su archivo cambia en disco
- **When** `musubi_code_graph` sobre ese símbolo
- **Then** el nodo se devuelve con `stale=true`

### Escenario: aislamiento
- **Given** el mismo node_key indexado en los proyectos P1 y P2
- **When** un lector acotado a P1 consulta ese símbolo
- **Then** ve el nodo de P1, no el de P2

## Fuera de alcance
- **Hook `PreToolUse`** que responde antes de Read/Grep → **F2-B** (opt-in de config).
- Weld a memoria `EXPLICADO_POR` → **F3**. Aristas TS/Py → **F4**. Sync central + viz CRM.
- Ranking de centralidad sofisticado (PageRank): F2-A usa **grado** simple.

## Preguntas abiertas
- [ ] ¿`code_graph` por `path` devuelve también callers de cada símbolo o solo la lista de
      símbolos? (design: probable símbolos + imports, callers solo en la consulta por símbolo)
- [ ] Tope de `max_depth` de impact y tope de nodos devueltos (evitar explosión). (design)
