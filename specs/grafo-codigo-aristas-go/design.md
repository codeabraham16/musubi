---
artifact: design
schema_version: "1.0"
change: grafo-codigo-aristas-go
status: draft
---

# Diseño técnico — Grafo de código: aristas derivadas en Go (Track 20 · F1)

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D1** | **Persistir el grafo derivado** (no derivar-al-vuelo en cada consulta), con `src_fingerprint` por fila y staleness comparada por valor. | (a) Derivar 100% al vuelo desde los archivos. (b) Cachear en memoria de proceso. | La federación lo exige: el cerebro central **no tiene el código fuente** del proyecto, así que un grafo puramente derivado-al-vuelo no puede verse desde la cabina CRM. Persistir + fingerprint concilia "derivar, no desfasar" (nace derivado) con "federable" (viaja como dato) y "no miente" (una desincronía se reporta STALE). |
| **D2** | **Id de nodo = clave string estable** `path#kind:name` (con receiver para métodos), NO el rowid. Las aristas referencian nodos por esa clave. | rowid entero + FK. | El refresco re-deriva (borra+reinserta) las filas de un archivo: con rowid, cada refresh cambiaría el id y rompería las aristas cross-archivo. La clave estable **desacopla las aristas del churn** y permite aristas "colgantes" hacia un símbolo aún no derivado (se resuelve cuando su archivo entra). |
| **D3** | **La arista/nodo es PROPIEDAD de su archivo origen** (`src_path`). Refresco = `DELETE ... WHERE src_path=? AND project_id=?` + reinsert, en una tx. | Diff fila-por-fila; borrado global y re-index total. | Mantiene el refresco **local al archivo cambiado** y elimina aristas stale por construcción, sin recomputar el repo entero. |
| **D4** | **CALLS = resolución intra-paquete** con tabla de símbolos del paquete (go/ast puro), `confidence=1.0` para match único; **CALLS cross-paquete precisas DIFERIDAS**. `IMPORTS` ya captura la dependencia cross-paquete. | `go/types`/`go/packages` para resolución exacta total. | `go/types` necesita que el paquete **typechequee** (build + toolchain + deps): frágil justo en el estado a medio editar que queremos soportar, pesado, y depende del `go` instalado (mal para el central). La resolución intra-paquete es go/ast puro, robusta en repos rotos, determinista y cubre "quién llama a esto" dentro del paquete. go/types queda como **slice futuro de upgrade de confianza**, no como bloqueo de F1. |
| **D5** | **Los paquetes importados son NODOS** (`kind=package`, flag `external`), arista `file —IMPORTS→ package`. | Guardar el import-path como string plano en la arista. | Convierte "¿qué importa al paquete X?" en una consulta de grafo uniforme (F2), y distingue in-project vs stdlib/terceros con `external`. |
| **D6** | **Reparto de capas idéntico a `detect_changes`**: `codeintel` deriva (puro, sin fs/db), `memory` persiste + lee scopeado + compara staleness **por valor** (sin fs), `mcp` aporta fs+fingerprint+git y dispara. | Meter la derivación en la capa MCP. | Reusa el patrón ya probado (el fingerprint lo computa MCP; el motor solo persiste y compara, ver `gistStale`). Hace la derivación **testeable con strings** (golden), sin tocar disco. |
| **D7** | **Esquema = migración v18**, aditiva; `project_id TEXT NOT NULL DEFAULT ''` sentinel + `UNIQUE(... , project_id)`; índices para lectura scopeada y recorrido. | Tablas ad-hoc `CREATE IF NOT EXISTS` fuera del versionado. | Sigue exactamente el patrón de tenancy de v13 (`code_memory`) y v14 (`relations`): sentinel no-nullable (SQLite trata cada NULL distinto en UNIQUE y rompería la dedup del upsert), legacy en `''` = espacio federado. |
| **D8** | **En F1 el poblado es un método INTERNO** (`RefreshCodeGraphForPackage`), enganchado oportunistamente al camino de `save_code` para archivos `.go` (cuando el agente ya guarda el gist, se deriva+persiste el grafo de su paquete). **Sin tool pública.** | Exponer ya una tool `musubi_index`. | Puebla el grafo como efecto de la rutina normal de dogfood, sin nueva acción del agente ni superficie de consulta prematura. El index deliberado de repo completo y las tools de consulta son **F2**. |

## Enfoque de implementación

Tres capas, espejando `detect_changes`:

**1. `internal/codeintel` (derivación pura, model-free, sin fs/db)**
- Tipos nuevos `Node` y `Edge` (ver contratos).
- `ExtractImports(path, content) []Import` — nuevo; import specs del AST de Go (con alias).
- `DerivePackage(dir string, files map[string]string, modulePath string) PackageGraph` —
  la unidad de derivación es el **paquete** (directorio), porque resolver CALLS intra-paquete
  necesita la tabla de símbolos de todos sus archivos. Emite:
  - un nodo `file` por archivo y nodos `func/method/type/const/var` (reusando `ExtractSymbols`);
  - aristas `CONTAINS` (file→símbolo), file-locales, `confidence=1.0`;
  - aristas `IMPORTS` (file→package), con `external = !startsWith(importPath, modulePath)`;
  - aristas `CALLS` (func/method→func/method) resueltas contra la tabla de símbolos del
    paquete: match único ⇒ `confidence=1.0`; nombre no resuelto o ambiguo ⇒ **se omite** (no se
    inventa). Cross-paquete precisas: **diferidas** (la dependencia ya vive en `IMPORTS`).
  - Cada nodo/arista queda etiquetado con su `src_path`.
- Degradación: archivo que no compila ⇒ parseo tolerante (`parser.ParseFile` recovery), se
  emite lo resuelto; nunca panic. Extensión no-Go ⇒ `PackageGraph` vacío.

**2. `internal/memory/codegraph.go` (store, con db, sin fs)** — espeja `codemem.go`:
- `UpsertPackageGraph(ctx, originProjectID, dir string, fps map[path]fingerprint, g PackageGraph) error`
  en una tx: por cada `src_path` del paquete, `DELETE` sus nodos/aristas de ese `project_id` y
  reinserta los nuevos con su `src_fingerprint`. (Delete-by-source de D3.)
- `GetCodeGraphNodeCtx` / `NeighborsCtx` (lectura scopeada, scoped vs federate como
  `GetCodeMemoryCtx`) — mínimo lo que necesiten los tests de F1; la superficie rica es F2.
- La **staleness NO se computa acá** (el motor no tiene fs): el store devuelve `src_fingerprint`;
  la capa MCP compara contra el fingerprint actual (igual que `gistStale`).

**3. `internal/mcp` (fs + fingerprint + disparo; interno en F1)**
- `RefreshCodeGraphForPackage(ctx, dir)`: lista los `.go` del directorio, lee su contenido y
  `FileFingerprint`, deriva con `codeintel.DerivePackage`, persiste con el store, scopeado por
  la credencial. Reusa `NormalizeCodePath` y `readProjectFile`.
- Enganche oportunista en el handler de `save_code`: si el path es `.go`, tras guardar el gist
  dispara `RefreshCodeGraphForPackage(dir(path))` (best-effort, no bloquea el guardado).
- `modulePath` se lee una vez del `go.mod` (primera línea `module X`), model-free.

## Contratos / interfaces

```go
// internal/codeintel — derivación pura
type Node struct {
    Key       string // estable: "internal/mcp/methods_detect.go#func:toolDetectChanges" | "pkg:go/ast"
    Kind      string // file|func|method|type|const|var|package
    Name      string // símbolo o import-path
    Path      string // archivo origen ("" para package externo)
    StartLine int
    EndLine   int
    External  bool   // package fuera del módulo (stdlib/terceros)
}
type Edge struct {
    FromKey    string
    ToKey      string
    Kind       string  // IMPORTS|CONTAINS|CALLS
    Confidence float64 // [0,1]
    Provenance string  // "EXTRACTED" en F1
    SrcPath    string  // archivo que "posee" la arista
}
type PackageGraph struct { Nodes []Node; Edges []Edge }

func ExtractImports(path, content string) []Import
func DerivePackage(dir string, files map[string]string, modulePath string) PackageGraph
```

```sql
-- migración v18: code_graph
CREATE TABLE IF NOT EXISTS code_graph_nodes (
    project_id      TEXT NOT NULL DEFAULT '',
    node_key        TEXT NOT NULL,
    kind            TEXT NOT NULL,
    name            TEXT NOT NULL,
    path            TEXT NOT NULL DEFAULT '',
    start_line      INTEGER NOT NULL DEFAULT 0,
    end_line        INTEGER NOT NULL DEFAULT 0,
    external        INTEGER NOT NULL DEFAULT 0,
    src_fingerprint TEXT NOT NULL DEFAULT '',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, node_key)
);
CREATE TABLE IF NOT EXISTS code_graph_edges (
    project_id      TEXT NOT NULL DEFAULT '',
    from_key        TEXT NOT NULL,
    to_key          TEXT NOT NULL,
    kind            TEXT NOT NULL,           -- IMPORTS|CONTAINS|CALLS
    confidence      REAL NOT NULL DEFAULT 1.0,
    provenance      TEXT NOT NULL DEFAULT 'EXTRACTED',
    src_path        TEXT NOT NULL DEFAULT '',
    src_fingerprint TEXT NOT NULL DEFAULT '',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, from_key, to_key, kind)
);
CREATE INDEX IF NOT EXISTS idx_cg_nodes_scope ON code_graph_nodes(project_id, path);
CREATE INDEX IF NOT EXISTS idx_cg_edges_from  ON code_graph_edges(project_id, from_key);
CREATE INDEX IF NOT EXISTS idx_cg_edges_to    ON code_graph_edges(project_id, to_key);
CREATE INDEX IF NOT EXISTS idx_cg_edges_src   ON code_graph_edges(project_id, src_path);
```

## Trade-offs
- **Se gana:** un grafo de código real, consultable y **federable**, model-free y Go-puro; base
  lista para F2 (consulta/impacto) y F3 (weld a memoria). `IMPORTS`/`CONTAINS` exactos desde el día 1.
- **Se cede:** precisión de CALLS **cross-paquete** (diferida a un slice con `go/types`); la
  incrementalidad es a **granularidad de paquete** (no archivo) — más simple y correcta, y como
  los paquetes Go son un directorio chico, el costo de re-derivar es bajo. La staleness sí es
  por-archivo (cada fila lleva su `src_fingerprint`).
- **Riesgo aceptado:** nodos `package` externos pueden quedar huérfanos si un archivo deja de
  importarlos (no se GC en F1). Inofensivo; un barrido de huérfanos es trivial y queda para después.

## Plan de pruebas
- **`internal/codeintel` (golden, sin fs):** un fixture de paquete Go multi-archivo (2–3 `.go`)
  como `map[path]content`; asertar los conjuntos `IMPORTS`/`CONTAINS`/`CALLS`; método vs. función
  homónima distinguidos por `Key`; `external` correcto (stdlib vs in-module); archivo con error de
  sintaxis ⇒ sin panic, derivación parcial; `.ts`/`.css` ⇒ `PackageGraph` vacío.
- **`internal/memory` (db temporal):** `UpsertPackageGraph` + lectura scopeada; **aislamiento**:
  dos `project_id` con el mismo `path`/símbolo coexisten y no se pisan (regresión directa del bug
  de v13); refresco **delete-by-source** deja intactas las filas de archivos hermanos; el
  `src_fingerprint` devuelto permite marcar STALE tras cambiar el contenido.
- **`internal/mcp` (integración):** `RefreshCodeGraphForPackage` sobre un paquete real y chico del
  propio repo (p. ej. `internal/codeintel`) puebla nodos/aristas esperados; el enganche de
  `save_code` para un `.go` dispara el refresco; `go build` + `go test ./...` verdes; sin cgo.
