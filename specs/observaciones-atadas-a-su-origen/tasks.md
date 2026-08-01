---
artifact: tasks
schema_version: "1.0"
change: observaciones-atadas-a-su-origen
status: draft
---

# Tareas — Observaciones atadas al estado que las originó

Orden pensado para que cada tarea sea cerrable sola y el build quede verde entre una y otra.

## Base

- [ ] **T1** — `internal/memory/database.go`: DDL de `observation_origins` en el bootstrap del
  esquema (PK compuesta `(observation_id, path)`, FK a `observations(id) ON DELETE CASCADE`,
  índice por `path`). Idempotente (`IF NOT EXISTS`), como el resto del esquema. (D1)
- [ ] **T2** — Test de que la FK cascadea de verdad: guardar observación + ancla, borrar la
  observación, y comprobar que el ancla se fue sola. Fija R14 y verifica que el pragma
  `foreign_keys(1)` está activo en el pool. (D1, R14)

## Escritura del ancla

- [ ] **T3** — `internal/memory/origins.go` (nuevo): `SaveObservationOrigins(tx, obsID, root,
  paths)` — normaliza con `NormalizeCodePath`, valida existencia (R3), aplica el tope de 10 (D6)
  y persiste (ruta, `FileFingerprint`). Errores que nombran la ruta ofensora. (R1–R4, D2, D6)
- [ ] **T4** — Enganchar T3 en el camino de guardado (`internal/memory/operations.go`,
  `SaveObservationTyped` y sus wrappers) **dentro de la misma tx** que la observación: si el ancla
  falla, no queda la observación guardada a medias. (R2, R3)
- [ ] **T5** — Tests de escritura: ancla feliz; ruta inexistente → error que la nombra y NADA
  guardado; 11 rutas → error; ruta absoluta dentro del proyecto → se normaliza a relativa.
  (R1–R4, D6)

## Derivación en el recall

- [ ] **T6** — `internal/memory/recall.go`: tipo `StaleOrigin{Path, Reason}` y campo
  `Stale []StaleOrigin \`json:"stale,omitempty"\`` en `RecallItem`. (D3)
- [ ] **T7** — `markStaleOrigins(items)`: UNA query `WHERE observation_id IN (...)`, memo
  `map[string]string` ruta→fingerprint por llamada, y clasificación `missing` (no existe) vs
  `changed` (difiere). Un error de E/S distinto de "no existe" NO marca. (R7–R9, R13, D4)
- [ ] **T8** — Llamarla en `Recall` **después** de rankear y recortar por presupuesto, antes del
  return; prefijar el gist con la advertencia. (R10, R12, D4)
- [ ] **T9** — Tests de derivación, uno por escenario de la spec: archivo cambiado → marca que lo
  nombra; archivo borrado → `missing`; archivo intacto → sin marca; observación sin anclas →
  `RecallItem` byte-idéntico al de hoy. (R5, R7–R10, R13)
- [ ] **T10** — Test de R11, el más importante: una observación marcada DEBE seguir apareciendo en
  la misma posición del ranking que tendría sin marca. Nada de ocultar ni despriorizar.
- [ ] **T11** — Test del caso real: observación con texto `PENDIENTE: gateway y bridge` anclada a
  su archivo; se toca el archivo; el recall la trae marcada sin intervención de nadie.

## Integridad y superficie

- [ ] **T12** — `internal/memory/doctor.go`: check `orphan_origins` (anclas sin observación) con su
  reparación, en la línea de `orphan_relations`. Test que lo pone en no-ok y lo cura. (R15)
- [ ] **T13** — `internal/mcp/methods.go`: arg `origin_paths []string` en
  `musubi_save_observation`, pasado al engine. `registry.go`: describirlo. Regenerar golden
  (`go test ./internal/mcp -run TestToolsListGolden -update`). (R1)
- [ ] **T14** — Test de D7/R6: una observación con anclas encolada al outbox NO lleva las rutas ni
  los fingerprints en su payload. Fija el comportamiento para que un refactor del sync no las
  arrastre.

## Cierre

- [ ] **T15** — `go build ./...`, `go vet ./internal/memory/ ./internal/mcp/`, `go test ./...`
  completo verde, y `musubi doctor` en ok incluyendo `orphan_origins`.
- [ ] **T16** — Actualizar la memoria `musubi-hard-delete-observaciones`: su afirmación *"NO hay
  foreign keys hacia observations, hay que limpiar a mano"* deja de ser cierta para esta tabla.
  Sin esto, un borrado manual futuro va a confundir a quien lo haga. (riesgo declarado en design)
- [ ] **T17** — `CHANGELOG.md`: entrada en `[Unreleased]`.
