---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f2-consulta-hook
status: draft
---

# Tareas — Consultar el grafo de código (Track 20 · F2-A)

## Store (`internal/memory`)
- [ ] **T1** — `GraphInEdgesCtx` (callers) + tipo `GraphDegree`. · `codegraph.go`
- [ ] **T2** — `GraphImpactCtx` (BFS `CALLS` entrantes, acotado por maxDepth/maxNodes). · `codegraph.go`
- [ ] **T3** — `GraphStatsCtx`, `GraphTopByDegreeCtx`, `GraphEntryPointsCtx`, `ListGraphNodesForFileCtx`. · `codegraph.go`
- [ ] **T4** — Sumar las firmas nuevas a la interfaz `CodeGraphStore`. · `backend.go`

## MCP (`internal/mcp`)
- [ ] **T5** — `indexAllPackages` (WalkDir, skip dirs, refresca por paquete) + `toolCodegraphIndex`. · `methods_codegraph.go`
- [ ] **T6** — `annotateStale` + `toolCodeGraph` (por symbol/path: node+callees+callers+imports+stale). · `methods_codegraph.go`
- [ ] **T7** — `toolImpact` (callers transitivos) + `toolMap` (stats+god-nodes+entry points). · `methods_codegraph.go`
- [ ] **T8** — Registrar 4 tools (index mutating; 3 consulta readOnly) + actualizar golden. · `registry.go`, `testdata/toolslist.golden.json`

## Pruebas
- [ ] **T9** — store: in/impact/stats/top/entrypoints + aislamiento por proyecto. · `codegraph_test.go`
- [ ] **T10** — mcp: index puebla; code_graph callers/callees; impact transitivo; stale tras mutar; map. · `methods_codegraph_test.go`

## Cierre
- [ ] **T11** — CHANGELOG `[Unreleased]` (F2-A: index + tools de consulta; nota de que el hook es F2-B).
- [ ] **T12** — `go build`/`go test ./...` verdes sin cgo; golden consistente.

## Forecast
- ~600–800 líneas. Un PR (F2-A), stacked sobre F1 (#239). El hook (F2-B) va en su propio PR con opt-in de config.
