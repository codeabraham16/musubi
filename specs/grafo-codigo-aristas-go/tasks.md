---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-aristas-go
status: draft
---

# Tareas — Grafo de código: aristas derivadas en Go (Track 20 · F1)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (≤ ~400 líneas).
Marcá `[x]` al completar.

## Capa 1 — Derivación pura (`internal/codeintel`)
- [ ] **T1** — Tipos `Node`, `Edge`, `PackageGraph` + constantes de `Kind` de nodo
  (`file/func/method/type/const/var/package`) y de arista (`IMPORTS/CONTAINS/CALLS`) y de
  proveniencia (`EXTRACTED`). Función `NodeKey(path, kind, name, recv)` estable y pura (D2, R7).
  · _archivos:_ `internal/codeintel/graph.go`
- [ ] **T2** — `ExtractImports(path, content) []Import` para `.go` (import specs + alias) con
  degradación sin pánico (R2, R5). · _archivos:_ `internal/codeintel/graph.go` · _depende de:_ T1
- [ ] **T3** — `DerivePackage(dir, files map[string]string, modulePath) PackageGraph`: nodos
  `file` + símbolos (reusar `ExtractSymbols`), aristas `CONTAINS` (file→símbolo, conf 1.0) e
  `IMPORTS` (file→package, `external=!hasPrefix(importPath, modulePath)`), cada fila etiquetada
  con `SrcPath` (R2, R3, D5). · _depende de:_ T2
- [ ] **T4** — Resolución `CALLS` intra-paquete: tabla de símbolos del paquete; por cada
  `ast.CallExpr` con callee sin calificar, arista `caller —CALLS→ callee` con `conf=1.0` si el
  match es único; **omitir** si es ambiguo o no resuelve; cross-paquete precisas se difieren
  (R4, D4). · _depende de:_ T3
- [ ] **T5** — Distinguir métodos de funciones homónimas en `NodeKey` (incluir receiver) y
  degradación de `.go` que no compila (parseo tolerante, parcial, sin panic) + no-Go ⇒
  `PackageGraph` vacío (R6, R7, R5). · _depende de:_ T4

## Capa 2 — Store (`internal/memory`)
- [ ] **T6** — Migración **v18** `code_graph`: `code_graph_nodes` + `code_graph_edges` con
  `project_id TEXT NOT NULL DEFAULT ''`, `UNIQUE(...project_id)` e índices scope+traversal;
  sumar la baseline (`initSchemaOn`) para bases frescas (R8, D7).
  · _archivos:_ `internal/memory/migrations.go` (+ baseline)
- [ ] **T7** — `internal/memory/codegraph.go`: tipos de fila + `UpsertPackageGraphFrom(originProjectID,
  dir, g)` en tx = **delete-by-source** (`WHERE src_path=? AND project_id=?`) + reinsert con
  `src_fingerprint` (D3, R11, R13, R14). · _depende de:_ T6
- [ ] **T8** — Lecturas scopeadas `GetCodeGraphNodeCtx` / `NeighborsCtx` (scoped vs federate,
  patrón `GetCodeMemoryCtx`); devuelven `src_fingerprint` para que MCP compare staleness (R9,
  R12, R16). · _depende de:_ T7

## Capa 3 — Disparo (`internal/mcp`, interno en F1)
- [ ] **T9** — `modulePath()`: leer la línea `module X` de `go.mod` una vez (helper model-free).
  · _archivos:_ `internal/mcp/methods_codegraph.go`
- [ ] **T10** — `RefreshCodeGraphForPackage(ctx, dir)`: listar `.go` del dir, leer contenido +
  `FileFingerprint`, `DerivePackage`, persistir scopeado (reusa `NormalizeCodePath`,
  `readProjectFile`, `scopedCtx`) (R13, R15, D6, D8). · _depende de:_ T8, T9
- [ ] **T11** — Enganche best-effort en el handler de `save_code`: si el path es `.go`, tras
  guardar el gist disparar `RefreshCodeGraphForPackage(dir(path))` sin bloquear ni fallar el
  guardado si el paquete no deriva (D8). · _depende de:_ T10

## Pruebas
- [ ] **T12** — codeintel golden: fixture de paquete Go multi-archivo ⇒ conjuntos
  `IMPORTS/CONTAINS/CALLS` esperados; método vs. función homónima; `external` correcto; archivo
  roto sin panic; no-Go vacío (cubre R2–R7). · _archivos:_ `internal/codeintel/graph_test.go`
- [ ] **T13** — store: `UpsertPackageGraphFrom` + **aislamiento por proyecto** (dos `project_id`,
  mismo path/símbolo, coexisten — regresión de v13); refresco delete-by-source deja hermanos
  intactos; `src_fingerprint` habilita STALE (cubre R8–R14).
  · _archivos:_ `internal/memory/codegraph_test.go`
- [ ] **T14** — mcp integración: `RefreshCodeGraphForPackage` sobre `internal/codeintel` puebla
  nodos/aristas esperados; el enganche de `save_code` dispara el refresco (cubre R13, R16, D8).
  · _archivos:_ `internal/mcp/methods_codegraph_test.go`

## Docs / cierre
- [ ] **T15** — `CHANGELOG.md` (`[Unreleased]`): grafo de código F1 (aristas Go derivadas,
  model-free, federation-ready) — nota de que aún no hay tool pública (F2).
- [ ] **T16** — `go build ./...` + `go test ./...` verdes, **sin cgo**; confirmar
  `user_version=18` tras migrar una base existente y que una base fresca crea las tablas.
- [ ] **T17** — Verificar contra la spec: cada `R1–R16` tiene cobertura (mapear R# → T#).

## Forecast de review
- Líneas estimadas: ~700–900 (codeintel ~350, memory ~250, mcp ~150, tests ~250).
- ¿Chained PRs recomendado? **Sí** — PR-A = Capa 1 (codeintel: T1–T5, T12) es autónoma y
  mergeable sola; PR-B = Capas 2–3 (store + disparo + integración). Divide el diff y aísla el
  riesgo de la migración de esquema en su propio PR.
