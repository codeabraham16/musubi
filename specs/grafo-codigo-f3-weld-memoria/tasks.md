---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f3-weld-memoria
status: draft
---

# Tareas — Soldar el grafo de código a la memoria (Track 20 · F3)

- [ ] **T1** — `symbolNameFromKey` + `explainedBy` (FTS scopeada por nombre/path, dedup). · `methods_codegraph.go`
- [ ] **T2** — `toolCodeContext` (nodo + callees/callers + explained_by). · `methods_codegraph.go`
- [ ] **T3** — Registrar `musubi_code_context` (readOnly). · `registry.go`
- [ ] **T4** — Clasificar en read_concurrency (wantReadOnly) + barrido de aislamiento (read_surface_class, marker web/topic). · tests
- [ ] **T5** — Regenerar golden (`-update`) + contadores 40→41 (http/server/dispatch). · tests
- [ ] **T6** — Test E2E: code_context devuelve estructura + explained_by; símbolo sin nodo. · `methods_codegraph_test.go`
- [ ] **T7** — CHANGELOG + `go build`/`go test ./...` verdes sin cgo.

## Forecast
- ~200–300 líneas. Un PR (F3), stacked sobre F2 (#240).
