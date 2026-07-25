---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f5-incremental-stale
status: archived
---

# Diseño — F5: incremental + poda + visibilidad

## Decisiones

### D1 — Reconciliar por fingerprint guardado, sin git ni cursor de commit
El grafo ya persiste `src_fingerprint` por fila. El estado "anterior" del índice **es** el
grafo mismo: se lista `path → src_fingerprint` de `code_graph_nodes` y se compara con
`FileFingerprint` del disco. Clasificación por archivo:
- **modificado**: en el grafo y en disco, fp distinto → su paquete es *dirty*.
- **nuevo**: `.go` en disco no presente en el grafo → su paquete es *dirty*.
- **fantasma**: en el grafo, ausente en disco → *prune* (+ su paquete, si el dir sigue, es *dirty*
  para limpiar aristas colgantes que lo referencian).
- **sin cambio**: fp igual → se salta.

**Rationale:** cero estado nuevo, cero dependencia de git (funciona en un checkout sin `.git`),
y reusa exactamente el `FileFingerprint`/`NormalizeCodePath` que ya usa `cgStale`. **Alternativa
descartada:** `GitRunner.Diff` contra un ref base — requiere un cursor "último SHA indexado"
persistido y un repo git válido; más piezas, menos puro.

### D2 — La unidad de refresco es el PAQUETE (dir), no el archivo
Un archivo cambiado marca *dirty* a su directorio y se re-deriva el paquete entero con el
`refreshCodeGraphForPackage` existente (que ya hace delete-by-source + reinsert por archivo).
**Rationale:** la resolución de `CALLS` es intra-paquete (necesita la tabla de símbolos de todos
los archivos del dir); re-derivar el paquete es la unidad correcta y ya está implementada.
**Alternativa descartada:** refrescar sólo el archivo — dejaría CALLS mal resueltas dentro del
paquete.

### D3 — La poda es una primitiva nueva, simétrica al delete-by-source
`PruneGraphFilesFrom(origin, paths)` borra en una tx los nodos (`WHERE project_id=? AND path=?`)
y aristas (`WHERE project_id=? AND src_path=?`) de cada path fantasma — el mismo SQL que ya corre
`UpsertPackageGraphFrom` antes de reinsertar, pero sin reinsertar. Scopeada por `project_id`
(R6). **Alternativa descartada:** un GC global que borre nodos sin aristas — más caro y borra
cosas legítimas (entry points, tipos).

### D4 — Aristas colgantes: se limpian re-derivando el paquete del fantasma, no con cascada
Al podar `b.go` puede quedar una arista `a.go → b.go` (su `src_path` es `a.go`, no se borra). Se
resuelve marcando *dirty* el dir del fantasma (si aún existe) para re-derivar `a.go`, donde la
llamada ya no resuelve a un símbolo del paquete y la arista desaparece. Si el dir entero
desapareció, `refreshCodeGraphForPackage` falla al leerlo (no-op) y basta la poda de filas.
**Rationale:** consistente con el invariante F1 (aristas sólo derivadas); una arista que apunta a
un nodo inexistente ya se maneja hoy (el `to_key` no resuelve). No se persiguen cascadas.

### D5 — Argumento del tool: `mode:"full"|"incremental"`, default `full`
String extensible (deja lugar a futuros modos) en vez de un booleano. Ausente o desconocido ⇒
`full` (R9, no rompe callers). El handler parsea best-effort (ignora error de unmarshal → full).

### D6 — `cgStale`: archivo ausente/ilegible ⇒ stale
`FileFingerprint` que devuelve error o "" pasa de significar *fresco* a significar *stale*
(fantasma). **Rationale:** correctitud — mostrar código borrado como fresco es el bug; ante la
duda, marcar stale es la señal honesta. Nodos sin path (paquetes externos) siguen nunca-stale.

### D7 — Visibilidad: conteos a granularidad de ARCHIVO en `map` y en el resumen del índice
`graphFreshness(ctx) → (stale, ghosts int)` recorre `path → fp` del grafo y cuenta archivos con
fp distinto (stale) y ausentes (ghosts). `map` suma `stale`/`ghosts`; el resumen del índice suma
`packages` (refrescados), `pruned`, y en incremental `skipped`. Granularidad de archivo (no de
nodo) por ser barata y suficiente para "conviene re-indexar". **Rationale:** una sola pasada de
`stat` sobre los paths del grafo, reutilizable por map y por el freshness del índice.

## Contrato (Go)

**internal/memory/codegraph.go** (nuevas primitivas, scopeadas):
```go
// GraphFileFingerprintsCtx devuelve path → src_fingerprint de los archivos presentes en el
// grafo (nodos con path != ''), acotado a la credencial. Base de la reconciliación y del freshness.
func (e *DbEngine) GraphFileFingerprintsCtx(ctx context.Context) (map[string]string, error)

// PruneGraphFilesFrom borra nodos (por path) y aristas (por src_path) de los paths dados, en una
// tx, atribuido al project_id de origen ('' ⇒ el del engine). Simétrico al delete-by-source.
func (e *DbEngine) PruneGraphFilesFrom(originProjectID string, paths []string) (pruned int, err error)
```

**internal/mcp/methods_codegraph.go**:
```go
// walkGoTree recorre el proyecto una vez y devuelve el set de dirs con .go y el set de file-keys
// normalizados presentes en disco (refactor de indexAllPackages para reusar en incremental).
func (s *McpServer) walkGoTree() (dirs map[string]bool, fileKeys map[string]bool)

// indexIncremental reconcilia el grafo con el working tree: clasifica stored vs disco, poda
// fantasmas y refresca sólo los dirs dirty. Devuelve {packages, pruned, skipped, nodes, edges}.
func (s *McpServer) indexIncremental(ctx context.Context) (map[string]interface{}, error)

// graphFreshness cuenta archivos stale (fp distinto) y ghosts (ausentes) del grafo.
func (s *McpServer) graphFreshness(ctx context.Context) (stale, ghosts int)

// cgStale (modificado): archivo ausente/ilegible ⇒ true.
// toolCodegraphIndex (modificado): parsea {mode}, despacha full|incremental.
// toolMap (modificado): agrega "stale" y "ghosts" al resultado.
```

**internal/mcp/registry.go**: `musubi_codegraph_index` gana `inputSchema` con `mode`
(enum full|incremental, default full) y su descripción menciona el modo incremental barato.
Regenerar `internal/mcp/testdata/toolslist.golden.json`.

## Alternativas globales descartadas
- **Auto-refresh en el hook precheck**: metería latencia y I/O sorpresa en cada lectura; F5 es
  tool-driven explícito. Deuda documentada.
- **Marcar stale en la DB (columna)**: el stale se DERIVA del fingerprint en cada lectura
  (model-free, siempre veraz); persistirlo abriría desincronización. Se mantiene derivado.
