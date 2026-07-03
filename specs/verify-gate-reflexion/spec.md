---
artifact: spec
schema_version: "1.0"
change: verify-gate-reflexion
status: archived
---

# Especificación — Gate de verificación duro + Reflexion

## Requisitos

### Modelo
- **R1** — `WorkflowStep` DEBE aceptar un campo opcional `verify` (string): la directiva de qué
  verificar. Vacío = el step no tiene gate. Sin migración (viaja en el JSON de la definición).
- **R2** — DEBE existir el estado de step `verifying` y los eventos `step_verifying`,
  `step_verified`, `step_reflection` (en la tabla `run_events` existente).
- **R3** — Un step `verifying` NO DEBE ser terminal: DEBE **bloquear** a sus dependientes hasta
  que la verificación pase.

### Entrada a verificación (`CompleteWorkflowStep`)
- **R4** — Al completar con `done` un step cuyo `verify` no está vacío, el motor DEBE ponerlo en
  `verifying` (no `done`) y journalear `step_verifying` (payload con el resultado producido). El
  resultado producido DEBE conservarse en `step_results`.
- **R5** — Completar un step **con** `verify` pero con status `failed` NO DEBE entrar a
  verificación: DEBE quedar `failed` directamente (un fallo no se verifica).
- **R6** — Un step **sin** `verify` DEBE completarse como hoy (done directo).

### Resolución (`action=verify`)
- **R7** — `VerifyWorkflowStep(runID, stepID, pass bool, reflection string)` DEBE exigir que el
  step esté en `verifying`; si no, DEBE devolver error claro.
- **R8** — Con `pass=true`, el step DEBE quedar `done`; el motor DEBE journalear `step_completed`
  (payload status=done + el resultado) para que dependientes/saga/OTel lo vean uniforme, y
  cerrar el run si corresponde.
- **R9** — Con `pass=false`, el motor DEBE journalear `step_reflection` (payload = la reflexión).
  Si el número de intentos de verificación previos es **menor** que el máximo, DEBE **reabrir**
  el step (`pending`, incrementando el contador de intentos) para otro intento (Reflexion).
- **R10** — Con `pass=false` y el presupuesto de intentos **agotado**, el step DEBE quedar
  `failed` (el gate no se satisface) y NO DEBE reabrirse.
- **R11** — El máximo de intentos DEBE ser configurable por step (reusar `max_iterations`) con
  un default sano cuando no se declara.

### Reflexiones consultables
- **R12** — Las reflexiones de un step (los `step_reflection` del journal) DEBEN ser
  recuperables, para que el reintento sea **informado**. `verify` (fail→reopen) DEBE poder
  devolver las reflexiones acumuladas del step.

### Tool / integración
- **R13** — `musubi_workflow` DEBE exponer `verify` (run_id, step, verdict pass|fail,
  reflection). El conteo de tools MCP NO DEBE cambiar (acción nueva).
- **R14** — Build + suite verdes; model-free (sin LLM), Go puro, sin dependencias ni migración.

## Escenarios

### Escenario: el gate bloquea `done`
- **Given** un run `a → check(verify) → b`, con `a` done
- **When** se completa `check` con `done`
- **Then** `check` queda `verifying` (no `done`); `b` NO está ready (bloqueado)

### Escenario: verify pass → done
- **Given** `check` en `verifying`
- **When** se llama `verify(check, pass, "")`
- **Then** `check` queda `done`; el journal tiene `step_completed`; `b` aparece ready

### Escenario: verify fail → reflexión + reopen
- **Given** `check` en `verifying`, con máximo 3 intentos
- **When** se llama `verify(check, fail, "faltó cubrir el caso X")`
- **Then** el journal tiene `step_reflection`; `check` vuelve a `pending` (reabierto); `b`
  sigue bloqueado; una consulta de reflexiones del step devuelve "faltó cubrir el caso X"

### Escenario: agotar el presupuesto → failed
- **Given** `check` con máximo 2 intentos, ya reabierto y re-completado, y falla la verificación
  por segunda vez
- **When** se llama `verify(check, fail, "sigue mal")`
- **Then** `check` queda `failed` (no se reabre); `b` permanece bloqueado

### Escenario: step sin verify no cambia
- **Given** un step sin `verify`
- **When** se completa con `done`
- **Then** queda `done` directo (sin pasar por `verifying`)

### Escenario: verify sobre step no-verificando es error
- **Given** un step que no está en `verifying`
- **When** se llama `verify` sobre él
- **Then** devuelve error claro

## Fuera de alcance
- Ejecutar la verificación / emitir el veredicto (lo hace el agente).
- Forzar/medir adversarialidad o sesgo del juez.
- Combinar `verify` con `repeat_while`.
- Migración de esquema.

## Preguntas abiertas
- [ ] ¿El contador de intentos reutiliza `StepIters` o usa uno propio? (design; probable:
      `StepIters`, ya existe y cuenta reaperturas)
- [ ] ¿`verify(pass)` journalea `step_verified` además de `step_completed`, o sólo el segundo?
      (design; probable: sólo `step_completed` para uniformidad; el paso por verifying ya
      quedó en `step_verifying`)
- [ ] ¿Las reflexiones se devuelven siempre o sólo en el fail→reopen? (design; probable: en el
      reopen, y disponibles vía el journal en cualquier momento)
