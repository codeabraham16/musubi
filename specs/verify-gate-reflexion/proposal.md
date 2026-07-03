---
artifact: proposal
schema_version: "1.0"
change: verify-gate-reflexion
status: archived
---

# Propuesta — Gate de verificación duro + Reflexion (bounded)

## Intención
Generar es fácil; **verificar es el cuello de botella** (el "verification-generation gap"). En
el motor de workflows, un step se marca `done` en cuanto el agente lo reporta — sin ninguna
garantía de que su salida sea correcta. Los flujos de calidad quieren lo contrario: un step
crítico **no debe darse por hecho hasta verificarse**, y si la verificación falla, el agente
debe **reintentar informado por qué falló** (Reflexion), con un presupuesto acotado de intentos
antes de rendirse.

Queremos un **gate de verificación duro**: un step puede declarar `verify` (la directiva de
qué chequear). Al completarlo, en vez de quedar `done` entra en `verifying` (no terminal:
bloquea a sus dependientes). Una acción de verificación resuelve: **pass** → `done`; **fail** →
se registra la **reflexión** (por qué falló) y, si quedan intentos, el step se **reabre** para
otro intento informado; si se agotan, el step **falla duro** (el gate no se satisface).
Model-free: Musubi impone la estructura del gate y acumula las reflexiones; el agente produce
el veredicto y la reflexión — idealmente con una **lente adversarial/independiente** (que la
directiva y la skill `adversarial-review` fomentan), porque adversarial > auto-chequeo.

## Alcance
- **Incluye:**
  - Campo `verify` en `WorkflowStep` (la directiva de qué verificar; vacío = sin gate).
  - Estado de step `verifying` (no terminal) + eventos `step_verifying`, `step_verified`,
    `step_reflection`.
  - En `CompleteWorkflowStep`: si un step con `verify` se completa con `done`, entra en
    `verifying` (no `done`), journaleando `step_verifying`.
  - `action=verify` (run_id, step, verdict, reflection): resuelve el gate — `pass` marca el
    step `done` (uniforme: journalea `step_completed`); `fail` journalea `step_reflection`
    (la reflexión) y, si `attempts < max`, **reabre** el step (Reflexion); si se agotan, lo
    marca `failed`.
  - Presupuesto de intentos acotado (reusa el contador de iteraciones existente; default sano).
  - Las reflexiones acumuladas de un step consultables (para que el reintento sea informado).
- **No incluye (explícito):**
  - **Ejecutar la verificación / emitir el veredicto.** Musubi impone el gate y registra; el
    veredicto y la reflexión los produce el agente (model-free). No hay LLM en el server.
  - **Forzar adversarialidad semántica.** Musubi estructura (exige un veredicto de verificación
    separado de la producción); que el verificador sea otro agente/lente es responsabilidad del
    integrador (lo fomenta la skill adversarial-review). No se mide el sesgo del juez aquí.
  - **Combinar `verify` con `repeat_while`** en el mismo step (semánticas de loop distintas).
  - **Migración de esquema.** `verify` viaja en el JSON de la definición; estado/eventos son strings.

## Enfoque
Se reutiliza el patrón de **gate no-terminal** recién construido para HITL: un step en
`verifying` no es terminal, así que bloquea a sus dependientes hasta el veredicto — la misma
mecánica que evita que downstream corra sobre salida no verificada (clave: la verificación
ocurre ANTES de que el step cuente como hecho, como el reopen de `repeat_while` pero
gobernado por un veredicto externo). La Reflexion es un `repeat_while` invertido: en vez de
"repetir mientras la condición sea verdadera", es "reabrir si la verificación falla y quedan
intentos", registrando la reflexión para informar el próximo intento. Todo se journalea en la
misma tx que el motor ya usa; el contador de intentos reutiliza `StepIters`.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/workflow.go` (`WorkflowStep.Verify`; `StepVerifying`; eventos;
    `CompleteWorkflowStep` entra a `verifying`; `VerifyWorkflowStep`; helper de reflexiones).
  - `internal/memory/backend.go` (interfaz `WorkflowStore`: `VerifyWorkflowStep` + reflexiones).
  - `internal/mcp/methods.go` + `registry.go` (acción `verify`; sin tools nuevas → conteo intacto).
  - Tests nuevos (gate bloquea done, pass→done, fail→reflexión+reopen, agotar intentos→failed,
    dependientes bloqueados durante verifying).
- Compatibilidad: **puramente aditivo**. Sin migración. Steps sin `verify` no cambian.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Un step en `verifying` se ofrece como ready o cuenta como done | `verifying` no es terminal (como waiting_input); test lo fija |
| Loop infinito de reflexión | Presupuesto acotado (`StepIters` < max, default sano); al agotarse → `failed` |
| `verify` sobre un step que no está verificando | Error claro; sólo un `verifying` se resuelve |
| Downstream corre sobre salida no verificada | El gate no-terminal bloquea dependientes hasta el pass |
| Confusión con `repeat_while` | Documentar que no se combinan; `verify` es gate de calidad, `repeat_while` es loop de condición |

## Estrategia de rollback
Aditivo y sin estado nuevo persistente (sin migración). Revertir el PR quita la acción
`verify` y el campo `verify` (que un binario viejo ignora en el JSON de la def); un step con
`verify` bajo un binario viejo se completaría `done` sin gate. Runs existentes no se afectan.

## Criterio de éxito
1. Un step con `verify` completado con `done` queda `verifying` (no `done`) y bloquea a sus
   dependientes — cubierto por test.
2. `verify(pass)` lo marca `done`, uniforme (journalea `step_completed`); los dependientes se
   destraban — test.
3. `verify(fail)` con intentos restantes registra la reflexión y **reabre** el step; el
   reintento puede consultar las reflexiones acumuladas — test.
4. Agotar el presupuesto de intentos deja el step `failed` (gate no satisfecho) — test.
5. Todo model-free, Go puro, sin deps ni migración; build + suite verdes; 30 tools intactas.
