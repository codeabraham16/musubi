---
artifact: design
schema_version: "1.0"
change: verify-gate-reflexion
status: archived
---

# Diseño — Gate de verificación duro + Reflexion

## Decisión 1 — `verify` como campo + estado `verifying` no-terminal (patrón HITL)
`WorkflowStep.Verify string` (yaml `verify`): la directiva de qué verificar; vacío = sin gate.
`StepVerifying = "verifying"`, no terminal (como `waiting_input`), así bloquea a los
dependientes gratis. Eventos `EventStepVerifying`, `EventStepReflection`. Sin migración.
**Rationale:** reutiliza el patrón de gate no-terminal recién validado en HITL; la
verificación ocurre ANTES de que el step cuente como hecho, evitando que downstream corra
sobre salida no verificada. Cero código de bloqueo nuevo (cae del modelo de dependencias).

## Decisión 2 — `verify` tiene precedencia sobre `repeat_while` en `CompleteWorkflowStep`
Al completar con `done`, se busca el step una vez: si `Verify != ""` → entra a `verifying`
(no aplica `repeat_while`); si no y `RepeatWhile != ""` → la lógica de loop actual. Un
`failed` no verifica (queda failed). Un step sin `verify` se completa done como hoy.
**Rationale:** son semánticas distintas y no se combinan (R de "fuera de alcance"); dar
precedencia a `verify` es simple y explícito. `failed` no se verifica porque un fallo ya es
terminal negativo.

## Decisión 3 — Journaleo diferenciado
- `complete(done)` de un step con `verify` → journalea `step_verifying` (payload `{result}`),
  NO `step_completed` (el step aún no está hecho).
- `verify(pass)` → journalea `step_completed` (payload `{status:done, result}`) — **uniforme**:
  dependientes/saga/OTel ven la compleción igual que cualquier otra. No se emite un
  `step_verified` separado (sería redundante; el paso por verificación ya quedó registrado en
  `step_verifying` + la ausencia de reflexión final).
- `verify(fail)` → journalea `step_reflection` (payload = la reflexión).
**Rationale:** mantener `step_completed` como el único evento de "hecho" preserva la
uniformidad que saga/OTel ya explotan; los eventos de verificación son metadatos adicionales.

## Decisión 4 — `VerifyWorkflowStep` con presupuesto por `StepIters`
```go
func (e *DbEngine) VerifyWorkflowStep(runID, stepID string, pass bool, reflection string) (WorkflowRun, []string, error)
```
Exige `verifying` (error si no). En tx:
- `pass`: `StepStatus=done`; `persistRunStatusTx`; journalea `step_completed` (+`run_done` si
  terminal). Devuelve run, reflexiones vacías.
- `!pass`: journalea `step_reflection`. `max = step.MaxIterations` (o `defaultVerifyAttempts=3`
  si ≤0). Si `StepIters[step]+1 < max` → **reabrir** (`StepStatus=pending`, `StepIters++`,
  journalea `step_reopened`); si no → `StepStatus=failed` (gate no satisfecho). `persistRunStatusTx`.
  Devuelve run + **todas las reflexiones acumuladas** del step (para el reintento informado).
El contador reutiliza `StepIters` (que ya cuenta reaperturas de `repeat_while`); un step con
`verify` no usa `repeat_while`, así que no hay colisión. La condición `StepIters+1 < max`
implementa "max intentos totales" (max=2 ⇒ 2 intentos; el 2º fail → failed).
**Rationale:** un solo contador; presupuesto acotado que garantiza terminación; las
reflexiones vienen del journal (fuente de verdad), no de estado nuevo.

## Decisión 5 — Reflexiones consultables desde el journal
```go
func (e *DbEngine) stepReflections(runID, stepID string) ([]string, error) // payloads de step_reflection
```
Deriva del journal los `step_reflection` del step, en orden. `verify(fail→reopen)` las
devuelve; el agente también puede verlas con `action=journal` en cualquier momento.
**Rationale:** derivar-no-guardar; el journal ya acumula las reflexiones.

## Decisión 6 — Contrato e integración
- `WorkflowStep.Verify string` (yaml `verify`, json omitempty).
- `StepVerifying = "verifying"`; `EventStepVerifying = "step_verifying"`;
  `EventStepReflection = "step_reflection"`; `defaultVerifyAttempts = 3`.
- Interfaz `WorkflowStore` += `VerifyWorkflowStep`.
- Handler: `action=verify` (run_id, step, verdict = `pass|fail`, reflection vía `result`) →
  `{run, ready, reflections}`. El `verdict` se lee de un arg nuevo; la reflexión del `result`.

## Alternativas globales descartadas
- **Verificación en una sola llamada (self-check dentro de `complete`)**: sería auto-chequeo,
  no adversarial; el two-phase (producir → verificar) permite que verifique otro agente/lente.
- **Contador propio en vez de `StepIters`**: estado extra sin beneficio; `StepIters` alcanza.
- **`step_verified` separado**: redundante con `step_completed`.
- **Ejecutar/medir la verificación en el server**: rompe model-free.
