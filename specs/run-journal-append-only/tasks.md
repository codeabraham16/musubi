---
artifact: tasks
schema_version: "1.0"
change: run-journal-append-only
status: archived
---

# Tareas — Run journal append-only + idempotencia

## Esquema
- [ ] T1 — `migrations.go`: migración v6 `run_events` — `CREATE TABLE IF NOT EXISTS
  run_events(id PK autoinc, run_id, seq, step_id, event_type, payload, idempotency_key,
  created_at)` + `UNIQUE(run_id, seq)` + `UNIQUE(run_id, idempotency_key)` + índice por
  `(run_id, seq)`. Idempotente por user_version, sin tocar workflow_runs. (R1–R3)

## Núcleo (workflow.go)
- [ ] T2 — Helper `appendRunEvent(tx, runID, stepID, eventType, payload, idempKey string) error`
  (seq = MAX(seq)+1 por run; idempKey/stepID vacíos → NULL). (R5–R6)
- [ ] T3 — `persistRunStatus` → `persistRunStatusTx(tx, run)` (misma lógica allTerminal→done);
  actualizar llamadores. (R4)
- [ ] T4 — `StartWorkflowRun`: envolver en tx; si el INSERT creó fila (RowsAffected>0),
  appendear `run_started`. (R6)
- [ ] T5 — `CompleteWorkflowStep(runID, stepID, result, stepStatus, idempotencyKey)`: tx que
  (a) si idempotencyKey!="" y ya hay evento con esa clave → no-op, devolver estado actual;
  (b) aplicar lógica existente (status/result, repeat_while, cierre); (c) persistRunStatusTx;
  (d) appendear `step_completed` (+ `step_reopened` si re-abrió, + `run_done` si cerró). (R7–R9)
- [ ] T6 — `WorkflowReady`: cuando marca skips, envolver persist + un `step_skipped` por step
  en tx. (R6)
- [ ] T7 — `RunEvent` struct + `WorkflowJournal(runID) ([]RunEvent, error)` (ORDER BY seq). (R10)

## Interfaz + handlers
- [ ] T8 — `backend.go`: `WorkflowStore` — nueva firma de `CompleteWorkflowStep` +
  `WorkflowJournal`. `methods.go toolWorkflow`: `complete` lee `idempotency_key`; acción
  `journal` (run_id) → `WorkflowJournal`. `registry.go`: describir `idempotency_key` y la
  acción `journal` (sin tools nuevas → conteo intacto). Golden si aplica. (R11)

## Tests
- [ ] T9 — `workflow_test.go`: complete idempotente (2ª llamada no-op, result intacto, 1 solo
  evento); journal ordenado (run_started→completes→run_done); atomicidad; retrocompat sin key;
  skip/reopen journal-eados. (todos los R)

## Cierre
- [ ] T10 — `go build/vet/test ./...` verdes; golden regenerado si cambió; verificar que los
  tests de workflow existentes pasan sin cambios de aserción (retrocompat). (R13–R14)
