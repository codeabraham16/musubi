---
artifact: tasks
schema_version: "1.0"
change: deteccion-de-cambios-de-codigo
status: draft
---

# Tareas — Detección de cambios de código

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (≤ ~400 líneas).

## Implementación — Componente 1: `internal/codeintel` (base, sin deps del motor)
- [ ] T1 — Tipos base + dispatcher: `Symbol`, `Import`, `FileDiff`, `LineRange`, y
  `ExtractSymbols(path, content)` que despacha por extensión (nunca panic; `[]` si no soporta).
  · _archivos:_ `internal/codeintel/symbols.go`
- [ ] T2 — Extractor Go vía `go/ast` tolerante (FuncDecl con receiver→method, GenDecl
  type/const/var top-level; rangos desde `fset.Position`). · _archivos:_ `internal/codeintel/symbols.go` · _depende de:_ T1
- [ ] T3 — Extractor brace-lang (JS/TS/JSX/TSX): regex ancladas + cierre por conteo de llaves.
  · _archivos:_ `internal/codeintel/symbols.go` · _depende de:_ T1
- [ ] T4 — Extractor Python: regex `def`/`class` + cierre por des-indentación.
  · _archivos:_ `internal/codeintel/symbols.go` · _depende de:_ T1
- [ ] T5 — `ExtractImports(path, content)` (go/ast para Go; regex para el resto).
  · _archivos:_ `internal/codeintel/imports.go` · _depende de:_ T1
- [ ] T6 — `ParseUnifiedDiff(gitOut)` → `[]FileDiff` con rangos del lado nuevo + change_type;
  salta binarios; y `SymbolsInRanges(syms, ranges)` (solape inclusivo).
  · _archivos:_ `internal/codeintel/diff.go` · _depende de:_ T1
- [ ] T7 — `Runner` interface + impl real (`git --no-pager diff --no-color [--staged] [ref]`)
  + `FakeRunner` para tests. · _archivos:_ `internal/codeintel/git.go` · _depende de:_ T1

## Implementación — Componente 2: handler MCP
- [ ] T8 — `toolDetectChanges(raw)`: args `{ref?, staged?}` → diff → símbolos solapados →
  `gist_stale` por fingerprint → `related_memory` por keyword de path+símbolos → `DetectReport`.
  · _archivos:_ `internal/mcp/methods_detect.go` · _depende de:_ T6, T7

## Implementación — Componente 3: wiring
- [ ] T9 — Registrar `musubi_detect_changes` (read-only) en el registry; conteo 29→30.
  · _archivos:_ `internal/mcp/registry.go` · _depende de:_ T8
- [ ] T10 — `toolSaveCode`: auto-derivar `symbols` con `codeintel` cuando el llamador no los
  pasa (compat: si los pasa, respetar). · _archivos:_ `internal/mcp/methods.go` · _depende de:_ T2
- [ ] T11 — Directiva de la fase `verify` referencia `musubi_detect_changes`.
  · _archivos:_ `internal/memory/sdd.go` · _depende de:_ T9

## Pruebas
- [ ] T12 — `symbols_test.go`: corpus Go/TS/Py table-driven, **corrimiento de líneas** (R1,R2,R3)
  y **archivo que no compila** (degrada sin panic, R4,R5). · _archivos:_ `internal/codeintel/symbols_test.go`
- [ ] T13 — `diff_test.go` + `overlap_test.go`: fixtures added/modified/deleted/renamed/binario/
  multi-hunk (R7,R8) y solapes de borde (R11). · _archivos:_ `internal/codeintel/*_test.go`
- [ ] T14 — `methods_detect_test.go`: `FakeRunner` + archivos temp → report esperado
  (símbolos, gist_stale, related_memory), y verifica read-only (R11-R15).
  · _archivos:_ `internal/mcp/methods_detect_test.go`
- [ ] T15 — Test de `save_code` sin/con symbols (R16) + actualizar golden `tools/list` 29→30
  (server_test/http_test/dispatch_concurrent_test) (R18). · _archivos:_ `internal/mcp/*_test.go`

## Docs / cierre
- [ ] T16 — `CHANGELOG.md` sección `[Unreleased]`: nueva tool `musubi_detect_changes`.
- [ ] T17 — Verificar contra la spec: cada R1–R18 con cobertura; build + `go test ./...` verdes.

## Forecast de review
- Líneas estimadas: ~700–900 (codeintel ~400, handler ~150, wiring ~80, tests ~250).
- ¿Chained PRs? **Sí, recomendado:** PR-A = `internal/codeintel` + sus tests (T1–T7, T12, T13,
  autocontenido, sin tocar MCP); PR-B = handler + wiring + integración (T8–T11, T14–T17).
