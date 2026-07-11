# Tareas — C5.1 Atribución (`author`)

> SDD · fase **tasks** · slice **C5.1** · cada tarea cerrable de forma independiente.

- [ ] **T1 — Migración v16.** En `internal/memory/migrations.go`: agregar migración
  `author_observations` = `ALTER TABLE observations ADD COLUMN author TEXT NOT NULL DEFAULT ''`
  (propia tx, aditiva). Subir `latestSchemaVersion()` 15 → 16.
- [ ] **T2 — Tripwires de esquema.** Actualizar el/los test(s) que asertan la versión
  (`migration_v15_test.go` / `outbox_test.go` u otros) de 15 → 16, y agregar
  `migration_v16_test.go`: valida columna presente + filas viejas con `author=''`.
- [ ] **T3 — Motor: threading de `author`.** En `internal/memory/operations.go`:
  extender `saveObservation(..., originProjectID, author string, embedding)` para persistir
  `author` en el INSERT y **excluirlo del UPDATE**; extender
  `SaveObservationTypedFrom` y `SaveObservationDedupedTypedFrom` con el param `author`; los
  wrappers legacy (`SaveObservation`, `SaveObservationTyped`, deduped) pasan `""`.
- [ ] **T4 — Interfaz backend.** En `internal/memory/backend.go`: actualizar las firmas de
  `SaveObservationTypedFrom` y `SaveObservationDedupedTypedFrom` en `ObservationStore`.
- [ ] **T5 — Campo `Author` en structs + lectura.** Agregar `Author string` a `Observation`
  (y a `SearchResult`/gist result si aplica); incluir `author` en los `SELECT` del recall /
  `memory_expand` / get, exponiéndolo cuando no está vacío.
- [ ] **T6 — Helper + handler.** En `internal/mcp/`: `authorFrom(p *Principal) string`
  (devuelve `p.Name` salvo nil / legacy-admin ⇒ `""`); `toolSaveObservation` deriva
  `author := authorFrom(principalFrom(ctx))` y lo pasa a `SaveObservationDedupedTypedFrom`.
- [ ] **T7 — Tests de comportamiento.**
  - `author` sellado: guardado con principal `davantis` ⇒ obs con `author="davantis"`.
  - anti-spoofing: el handler ignora cualquier `author` del payload (no existe como input).
  - backward-compat: sin principal / stdio ⇒ `author=""`, scope `local`, sin cambios.
  - legacy-admin ⇒ `author=""`.
- [ ] **T8 — Build + gofmt + lint local.** `go build ./...`, `go test ./internal/memory ./internal/mcp`,
  gofmt (ojo CRLF Windows: verificar con `tr -d '\r' | gofmt`).

## Orden sugerido
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8. (Schema primero, motor, interfaz, structs, handler, tests, verificación.)

## Fuera de C5.1 (siguientes slices)
Team-mode config (C5.2) · auto-recall federado (C5.3) · dedup/decay a escala (C5.4) ·
`author` en facts/code/telemetry · filtro `author=` en recall.
