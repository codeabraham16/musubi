---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f5-incremental-stale
status: archived
---

# Tareas — F5: incremental + poda + visibilidad

Checklist atómica, ordenada; cada una cerrable por separado.

- [ ] **T1 — Primitivas de memoria** (`internal/memory/codegraph.go`)
  - `GraphFileFingerprintsCtx(ctx) (map[string]string, error)`: `SELECT DISTINCT path,
    src_fingerprint FROM code_graph_nodes WHERE path != '' <scope>`.
  - `PruneGraphFilesFrom(origin string, paths []string) (int, error)`: tx que borra nodos
    (`path`) y aristas (`src_path`) por cada path; cuenta filas de nodos borradas; scopeado por
    `project_id` ('' ⇒ engine). Wrapper `PruneGraphFiles`.

- [ ] **T2 — Refactor del walk** (`internal/mcp/methods_codegraph.go`)
  - Extraer `walkGoTree() (dirs, fileKeys map[string]bool)` del cuerpo de `indexAllPackages`;
    `dirs` = directorios con `.go`, `fileKeys` = paths normalizados de cada `.go` en disco.
  - `indexAllPackages` pasa a usar `walkGoTree` (sin cambio de comportamiento).

- [ ] **T3 — Índice incremental** (`internal/mcp/methods_codegraph.go`)
  - `indexIncremental(ctx)`: clasifica `GraphFileFingerprintsCtx` vs `walkGoTree`+`FileFingerprint`
    → `dirtyDirs`, `ghostPaths`. Poda `ghostPaths` (origen por credencial, como refresh), marca
    dirty el dir de cada fantasma si aún existe, refresca los `dirtyDirs`. Devuelve
    `{packages, pruned, skipped, nodes, edges}`.
  - `toolCodegraphIndex`: parsear `{mode string}` best-effort; `incremental` → `indexIncremental`,
    resto → `indexAllPackages`.

- [ ] **T4 — Fix cgStale** (`internal/mcp/methods_codegraph.go`)
  - Archivo ausente/ilegible (`err != nil || cur == ""`) ⇒ `true`. Path vacío sigue `false`.

- [ ] **T5 — Visibilidad en map** (`internal/mcp/methods_codegraph.go`)
  - `graphFreshness(ctx) (stale, ghosts int)` sobre `GraphFileFingerprintsCtx`.
  - `toolMap` agrega `"stale"` y `"ghosts"`.

- [ ] **T6 — Registry + golden** (`internal/mcp/registry.go`, `testdata/toolslist.golden.json`)
  - `musubi_codegraph_index`: `inputSchema` con `mode` (enum `full|incremental`, default `full`);
    actualizar la descripción (mencionar el incremental barato).
  - Regenerar/actualizar el golden de la lista de tools.

- [ ] **T7 — Tests** (`internal/mcp/methods_codegraph_test.go`, `internal/memory/codegraph_test.go`)
  - Incremental: sin-cambios (0 packages), modificado (sólo su pkg, deja de estar stale), nuevo
    (aparece), borrado (podado, no en code_graph/impact, `pruned≥1`).
  - `cgStale` archivo faltante ⇒ `stale:true`.
  - `map` reporta `stale≥1` y `ghosts≥1`.
  - Aislamiento: `PruneGraphFilesFrom` de un proyecto no toca filas de otro.

- [ ] **T8 — Build/CHANGELOG**
  - `go build ./... && go vet ./... && go test ./...` verde.
  - Entrada en `CHANGELOG.md` `[Unreleased]`.
