---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f2b-hook
status: draft
---

# Tareas — El hook que responde antes de leer (Track 20 · F2-B)

- [x] **T1** — Extender `codeStore` con las 3 lecturas de grafo (Ctx). · `precheck.go`
- [x] **T2** — `codegraphHookEnabled()` (env var) + `codeGraphMessage` + helpers (nombres, cotas). · `precheck.go`
- [x] **T3** — Gatear la superficie en `precheckOutput` + ledger `precheck_codegraph`. · `precheck.go`
- [x] **T4** — Fake +grafo y 2 tests (ON inyecta / OFF no). · `precheck_test.go`
- [x] **T5** — CHANGELOG (opt-in `MUSUBI_CODEGRAPH_HOOK`) + build/test verdes sin cgo.

## Forecast
- ~180 líneas. Un PR (F2-B) desde `main` (ya con F1-F3).
