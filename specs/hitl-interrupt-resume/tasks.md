---
artifact: tasks
schema_version: "1.0"
change: hitl-interrupt-resume
status: archived
---

# Tareas — HITL: interrupt/resume durable

## Núcleo (workflow.go)
- [ ] T1 — `WorkflowStep.Await string` (yaml `await`, json omitempty). Consts `StepWaiting =
  "waiting_input"`, `EventStepWaiting = "step_waiting"`. `AwaitingStep{StepID, Prompt}`. (R1–R2)
- [ ] T2 — `WorkflowReady`: tras evaluar `when`, si `step.Await != ""` y el status no es ya
  `waiting_input` → marcar `waiting_input`, `changed=true`, agregar a `waiting`; `continue` (no
  ready). Journalear `step_waiting` por cada nuevo, en la tx `changed` existente. (R3–R5)
- [ ] T3 — `ProvideWorkflowInput(runID, stepID, input, status) (WorkflowRun, error)`: exigir
  `waiting_input` (error claro si no); validar status (done|failed, default done); fijar
  result/status; tx `persistRunStatusTx` + `step_completed` (payload) + `run_done` si terminal. (R7–R9)
- [ ] T4 — `WorkflowAwaiting(runID) ([]AwaitingStep, error)`: steps con status `waiting_input`
  + su `Await`. (R6)

## Interfaz + handler
- [ ] T5 — `backend.go`: `WorkflowStore` += `ProvideWorkflowInput`, `WorkflowAwaiting`.
- [ ] T6 — `methods.go toolWorkflow`: acción `provide` (run_id, step, input, status) → `{run,
  ready, waiting}`; incluir `waiting` en las respuestas de start/next/complete/resume. Leer
  `input`. `registry.go`: describir `provide` + `await` + `input` (sin tools nuevas). Golden. (R11)

## Tests
- [ ] T7 — `hitl_test.go`: pausa en await (no ready, waiting_input, dependientes bloqueados);
  provide=done reanuda (dependientes ready); provide=failed bloquea; gate con when-falso se
  salta (no pausa); durabilidad (relectura fresca provee y continúa); provide sobre no-esperando
  → error; step_waiting journaleado una sola vez. (todos los R)

## Cierre
- [ ] T8 — `go build/vet/test ./...` verdes; golden regenerado; smoke del ciclo pausa→provide→
  reanudar. (R12)
