---
artifact: design
schema_version: "1.0"
change: hitl-interrupt-resume
status: archived
---

# Diseño — HITL: interrupt/resume durable

## Decisión 1 — `await` como campo de step + estado `waiting_input`
`WorkflowStep.Await string` (yaml `await`): el prompt para el humano; vacío = no pausa. Estado
`StepWaiting = "waiting_input"`, evento `EventStepWaiting = "step_waiting"`. Sin migración: el
campo viaja en el JSON de la definición ya persistida; estado/evento son strings.
**Rationale:** simetría con `when`/`repeat_while`/`compensate` (metadatos declarativos por
step) y con el patrón interrupt de LangGraph. El estado durable ya lo da SQLite.

## Decisión 2 — `waiting_input` NO es terminal → bloquea dependientes gratis
`terminalStep()` sigue siendo sólo `done||skipped`. Un `waiting_input` no satisface los
`needs` de sus dependientes, así que quedan bloqueados hasta la decisión. `persistRunStatusTx`
tampoco lo cuenta como terminal → el run no se cierra mientras haya un await pendiente.
**Rationale:** la semántica de gate cae sola del modelo de dependencias existente; cero código
de bloqueo nuevo.

## Decisión 3 — Pausa en `WorkflowReady`: evaluar `when`, luego `await`
En el loop de candidatos, tras la evaluación de `when` (que ya existe):
```
// when pasó (o no hay). Ahora: ¿es un gate humano?
if step.Await != "" {
    if run.StepStatus[id] != StepWaiting {
        run.StepStatus[id] = StepWaiting
        changed = true
        waiting = append(waiting, id)   // journalear step_waiting una vez
    }
    continue // NUNCA se ofrece como ready
}
ready = append(ready, id)
```
El `step_waiting` se journalea sólo para los recién marcados (los ya `waiting_input` no
re-journalean, aunque `ReadySteps` los siga devolviendo como candidatos). Se reutiliza la tx
`changed` que ya persiste skips: se agrega el journaleo de `step_waiting` (y sigue el de
`step_skipped`/`run_done`).
**Rationale:** `when` antes que `await` cumple R5 (un gate gated-out se salta, no pausa). El
guard `!= StepWaiting` da la idempotencia de R4 pese a que el scheduler corre en cada consulta.
**Descartado:** cambiar `ReadySteps` para excluir `waiting_input` (mezclaría la política de
gate humano con el cálculo puro de dependencias; mejor mantener `ReadySteps` model-free y
decidir el await en `WorkflowReady`, donde ya se persisten transiciones).

## Decisión 4 — `provide` es un método propio (no reusa `CompleteWorkflowStep`)
```go
func (e *DbEngine) ProvideWorkflowInput(runID, stepID, input, status string) (WorkflowRun, error)
```
Exige que el step esté en `waiting_input` (si no, error claro); valida `status` (done|failed,
default done); fija `StepStatus[step]=status`, `StepResults[step]=input`; en una tx persiste
(`persistRunStatusTx`) y journalea `step_completed` (payload `{status,result}`, uniforme para
saga/OTel) + `run_done` si el run quedó terminal.
**Rationale:** `CompleteWorkflowStep` exige que el step NO esté ya resuelto y aplica lógica de
`repeat_while`; un método propio mantiene la guarda "sólo un waiting_input se puede proveer" y
evita mezclar el loop del await con el de los steps normales. Journalear como `step_completed`
mantiene la uniformidad (la resolución de un gate es una compleción, con origen humano).
**Descartado:** reutilizar `CompleteWorkflowStep` (aflojaría su guarda de estado y arrastraría
la lógica de repeat_while a un gate).

## Decisión 5 — La lista `waiting` se deriva del snapshot + la def
```go
type AwaitingStep struct { StepID string `json:"step"`; Prompt string `json:"prompt"` }
func (e *DbEngine) WorkflowAwaiting(runID string) ([]AwaitingStep, error)
```
Recorre `run.Def.Steps`, y por cada uno con `StepStatus[id] == waiting_input` devuelve
`{id, Await}`. O(steps), siempre refleja el estado corriente.
**Rationale:** el snapshot ya tiene el estado; derivar del journal sería más caro y redundante.
El handler la incluye en las respuestas que devuelven `ready` (start/next/complete/resume) para
que el agente/humano vea qué está pausado y su prompt.

## Decisión 6 — `await` no se combina con `repeat_while`
Un step con `await` es un gate, no un loop. Si un step declarara ambos, `provide` lo completa
sin aplicar `repeat_while` (no re-abre). No se valida como error (para no romper), pero la
directiva/documentación aclara que no se combinan.
**Rationale:** semánticas distintas; combinarlas no tiene caso de uso claro y complicaría el
resolver.

## Contrato Go (resumen)
- `WorkflowStep.Await string` (yaml `await`, json omitempty).
- `StepWaiting = "waiting_input"`; `EventStepWaiting = "step_waiting"`.
- `WorkflowReady`: pausa steps con `await` (Decisión 3).
- `ProvideWorkflowInput(runID, stepID, input, status) (WorkflowRun, error)`.
- `WorkflowAwaiting(runID) ([]AwaitingStep, error)` + `AwaitingStep{StepID, Prompt}`.
- Interfaz `WorkflowStore` += `ProvideWorkflowInput`, `WorkflowAwaiting`.
- Handler: `action=provide` (run_id, step, input, status); `waiting` en las respuestas.

## Alternativas globales descartadas
- **Excluir `waiting_input` en `ReadySteps`** (Decisión 3): ensucia el cálculo de dependencias.
- **`provide` = `complete` con flag** (Decisión 4): afloja guardas.
- **Notificar al humano desde Musubi**: rompe local-first; el canal es del integrador.
