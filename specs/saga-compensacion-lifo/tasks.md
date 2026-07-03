---
artifact: tasks
schema_version: "1.0"
change: saga-compensacion-lifo
status: archived
---

# Tareas — Saga: compensación LIFO

## Núcleo (workflow.go)
- [ ] T1 — `WorkflowStep`: campo `Compensate string` (yaml `compensate`, json omitempty). (R1)
- [ ] T2 — Consts: `RunCompensating`/`RunCompensated`; `EventRunRollback`/`EventStepCompensated`/
  `EventRunCompensated`. `CompensationStep{StepID, Compensate}` struct. (R2–R3, R6)
- [ ] T3 — `compensationPlan(run WorkflowRun, events []RunEvent) []CompensationStep`: orden de
  completado = seq de `step_completed` (dedup, último por step) → invertir (LIFO) → filtrar
  `Compensate!="" AND StepStatus==done AND sin step_compensated`. (R4–R5)
- [ ] T4 — `WorkflowRollback(runID) ([]CompensationStep, WorkflowRun, error)`: cargar run+journal;
  error si no existe; tx: status→compensating, `run_rollback` (si no existe ya), y si el plan
  nace vacío → status compensated + `run_compensated`; devolver plan. (R7–R9)
- [ ] T5 — `CompleteCompensation(runID, stepID) ([]CompensationStep, WorkflowRun, error)`:
  validar step (existe/compensable); si ya compensado → no-op; tx: `step_compensated`, y si el
  plan restante queda vacío → status compensated + `run_compensated`; devolver plan restante. (R10–R13)

## Interfaz + handler
- [ ] T6 — `backend.go`: `WorkflowStore` += `WorkflowRollback` y `CompleteCompensation`.
- [ ] T7 — `methods.go toolWorkflow`: acciones `rollback` (run_id) y `compensated` (run_id, step)
  → devuelven `{run, pending}`. `registry.go`: agregar al enum/descripción (sin tools nuevas).
  Regenerar golden. (R14)

## Tests
- [ ] T8 — `workflow_test.go` (o `saga_test.go`): plan LIFO abc→[c,b,a]; filtro sin-compensate/
  no-done; ejecución + cierre a compensated; doble compensación no-op; rollback vacío→compensated;
  re-entrancia sin duplicar run_rollback. (todos los R)

## Cierre
- [ ] T9 — `go build/vet/test ./...` verdes; golden regenerado; smoke del ciclo
  rollback→compensated×N→compensated. (R15)
