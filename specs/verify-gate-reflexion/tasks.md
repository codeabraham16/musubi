---
artifact: tasks
schema_version: "1.0"
change: verify-gate-reflexion
status: archived
---

# Tareas — Gate de verificación duro + Reflexion

## Núcleo (workflow.go)
- [ ] T1 — `WorkflowStep.Verify string` (yaml `verify`, json omitempty). Consts `StepVerifying =
  "verifying"`, `EventStepVerifying`, `EventStepReflection`, `defaultVerifyAttempts = 3`. (R1–R2)
- [ ] T2 — `CompleteWorkflowStep`: al completar con `done`, si `step.Verify != ""` → `StepStatus
  = verifying` (precedencia sobre repeat_while), journalear `step_verifying` (payload result) en
  vez de `step_completed`; `failed` y steps sin verify sin cambios. (R4–R6)
- [ ] T3 — `VerifyWorkflowStep(runID, stepID string, pass bool, reflection string) (WorkflowRun,
  []string, error)`: exigir `verifying`; pass → done + `step_completed` + run_done; fail →
  `step_reflection` + (StepIters+1<max → reopen pending/iters++/`step_reopened`; else failed);
  todo en tx; devolver run + reflexiones. (R7–R11)
- [ ] T4 — `stepReflections(runID, stepID) ([]string, error)`: payloads de `step_reflection`
  del journal, en orden. (R12)

## Interfaz + handler
- [ ] T5 — `backend.go`: `WorkflowStore` += `VerifyWorkflowStep`.
- [ ] T6 — `methods.go toolWorkflow`: acción `verify` (run_id, step, verdict pass|fail,
  reflection vía `result`) → `{run, ready, reflections}`. Leer `verdict`. `registry.go`:
  describir `verify` + `verdict` + `verify` field (sin tools nuevas). Golden. (R13)

## Tests
- [ ] T7 — `verifygate_test.go`: complete-con-verify → verifying (no done, dependientes
  bloqueados); verify(pass) → done + step_completed + dependientes ready; verify(fail) →
  reflexión + reopen (pending) + reflexiones consultables; agotar intentos → failed; step sin
  verify → done directo; verify sobre no-verifying → error. (todos los R)

## Cierre
- [ ] T8 — `go build/vet/test ./...` verdes; golden regenerado; smoke del ciclo complete→
  verify(fail)→reopen→complete→verify(pass)→done. (R14)
