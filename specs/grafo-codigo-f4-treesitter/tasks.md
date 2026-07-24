---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f4-treesitter
status: draft
---

# Tareas — Aristas TS/JS/Python vía tree-sitter (Track 20 · F4)

- [x] **T1** — `go get gotreesitter@v0.47.0`; de-risquear la API con un smoke (borrado).
- [x] **T2** — `treesit_on.go` (`//go:build treesitter`): languageFor + derivePolyglotFile (defs/imports/CALLS intra-archivo) + helpers.
- [x] **T3** — `treesit_off.go` (`//go:build !treesitter`): no-ops (default lean).
- [x] **T4** — Pase polyglot en `DerivePackage` (merge por addNode/addEdge).
- [x] **T5** — Tests: `treesit_test.go` (tagged, TS/Py), `treesit_off_test.go` (default vacío), ajustar `TestDerivePackage_UnsupportedExtEmpty`.
- [x] **T6** — CHANGELOG (build opt-in + nota `go mod tidy`); verificar default lean (gotreesitter no linkeado) + ambos builds verdes.

## Forecast
- ~260 líneas. Un PR (F4) desde `main`. Dep tag-gated: `go mod tidy` solo con `-tags treesitter`.
