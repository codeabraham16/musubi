---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f2-consulta-hook
status: draft
---

# Diseño técnico — Consultar el grafo de código (Track 20 · F2-A)

## Decisiones de arquitectura

| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| D1 | **Index = tool MCP** `musubi_codegraph_index` que camina `projectPath` con `filepath.WalkDir`, junta dirs con `.go` y llama `refreshCodeGraphForPackage` por dir. | Subcomando CLI; index automático en background. | El agente puede dispararlo directo (MCP); reusa el trigger de F1 sin duplicar lógica. CLI queda para después. |
| D2 | **Staleness en la capa MCP** (no en el store): tras leer nodos, comparar `src_fingerprint` guardado vs `FileFingerprint` actual (como `gistStale`). | Computar staleness en SQL. | El motor no tiene fs (principio de F1/detect_changes); mantiene el store puro. |
| D3 | **Impact = BFS model-free** sobre aristas `CALLS` entrantes, con `max_depth` y tope de nodos. | Recursive CTE en SQL; sin límite. | Simple, determinista, acotado (evita explosión). Reusa `GraphInEdgesCtx`. |
| D4 | **Map = grado simple** (callers+callees por nodo) + entry points sintácticos (`func:main`, funcs sin callers). | PageRank/centralidad. | F2-A quiere panorama barato; el ranking sofisticado no aporta lo suficiente todavía. |
| D5 | **Salida compacta** (node_key + kind + line + stale; aristas como pares de keys), sin cuerpos. | Devolver contenido. | Es la palanca de tokens: el agente navega estructura sin leer archivos. |

## Enfoque de implementación

**`internal/memory/codegraph.go` (store, nuevas lecturas scopeadas):**
- `GraphInEdgesCtx(ctx, toKey)` — callers (aristas entrantes), simétrico de `GraphOutEdgesCtx`.
- `GraphImpactCtx(ctx, key, maxDepth, maxNodes)` — BFS sobre `CALLS` entrantes; devuelve el
  conjunto de node_keys alcanzados (sin el origen), acotado.
- `GraphStatsCtx(ctx)` — conteos: nodos totales, aristas por kind.
- `GraphTopByDegreeCtx(ctx, n)` — top-N node_keys por (out+in) grado (dos GROUP BY unidos).
- `GraphEntryPointsCtx(ctx)` — funcs/methods sin aristas `CALLS` entrantes + `func:main`.
- `ListGraphNodesForFileCtx(ctx, path)` — símbolos de un archivo (para `code_graph` por path).
- Todas usan `scopeClause`/patrón scopeado ya existente.

**`internal/mcp/methods_codegraph.go` (handlers + index):**
- `indexAllPackages(ctx)` — `WalkDir` desde `projectPath`, set de dirs con `.go` (skip
  `.git`/`.musubi`/`vendor`/`testdata`/ocultos), `refreshCodeGraphForPackage` por dir, acumula
  `{packages, nodes, edges}`.
- `toolCodegraphIndex` (mutating), `toolCodeGraph`, `toolImpact`, `toolMap` (read-only).
- `annotateStale(nodes)` — helper: por nodo, `FileFingerprint(projectPath, node.Path)` != stored
  ⇒ `stale=true`.
- Normaliza `symbol` vs `path`: si el arg trae `#` es node_key; si es un path, se normaliza y se
  listan sus símbolos.

**`internal/mcp/registry.go`:** 4 `toolEntry` nuevas (index mutating; las 3 de consulta `readOnly:true`).
Actualizar `internal/mcp/testdata/toolslist.golden.json`.

## Contratos / interfaces
```go
// store
GraphInEdgesCtx(ctx, toKey string) ([]GraphEdge, error)
GraphImpactCtx(ctx, key string, maxDepth, maxNodes int) ([]string, error)
GraphStatsCtx(ctx) (nodes int, edgesByKind map[string]int, err error)
GraphTopByDegreeCtx(ctx, n int) ([]GraphDegree, error) // {Key, Degree}
GraphEntryPointsCtx(ctx) ([]string, error)
ListGraphNodesForFileCtx(ctx, path string) ([]GraphNode, error)
```
Salidas MCP (JSON compacto): `code_graph` → `{node, stale, callees[], callers[], imports[]}`;
`impact` → `{symbol, depth, callers[]}`; `map` → `{nodes, edges{IMPORTS,CONTAINS,CALLS}, god_nodes[], entry_points[]}`.

## Trade-offs
- Gana: navegación/impacto/panorama sin leer archivos, model-free, scopeado, con staleness real.
- Cede: index es on-demand (no automático); `map` usa grado (no centralidad fina); el hook (F2-B)
  —la mayor palanca— queda para un opt-in de config aparte.

## Plan de pruebas
- store: in/impact/stats/top/entrypoints sobre un grafo sembrado; aislamiento por proyecto.
- mcp: `index` sobre un proyecto temporal multi-paquete puebla y `code_graph`/`impact`/`map`
  responden lo esperado; `code_graph` marca `stale` tras mutar un archivo; golden de tools/list.
