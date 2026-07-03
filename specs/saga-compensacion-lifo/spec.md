---
artifact: spec
schema_version: "1.0"
change: saga-compensacion-lifo
status: archived
---

# Especificación — Saga: compensación LIFO

## Requisitos

### Modelo
- **R1** — `WorkflowStep` DEBE aceptar un campo opcional `compensate` (string): la directiva
  de cómo deshacer ese step. Vacío = el step no tiene compensación.
- **R2** — DEBEN existir los estados de run `compensating` y `compensated` (además de los
  actuales running/done/aborted). NO DEBE haber migración de esquema (el campo viaja en el
  JSON de la definición ya persistida; los estados son strings).
- **R3** — DEBEN existir los tipos de evento de journal `run_rollback`, `step_compensated` y
  `run_compensated`, registrados en la tabla `run_events` existente.

### Plan de compensación (derivado, LIFO)
- **R4** — El plan de compensación de un run DEBE derivarse del journal: el orden de completado
  es el orden `seq` de los eventos `step_completed`; el plan es ese orden **invertido** (LIFO).
- **R5** — Un step DEBE entrar al plan si y sólo si: (a) su `compensate` no está vacío, (b) su
  estado actual en el snapshot es `done`, y (c) NO tiene ya un evento `step_compensated`.
- **R6** — Cada entrada del plan DEBE llevar el `step` y su directiva `compensate`.

### `action=rollback`
- **R7** — `WorkflowRollback(runID)` DEBE: marcar el run como `compensating` (si no lo está),
  journalear `run_rollback` (una sola vez por run), y devolver el plan de compensación vigente
  (R4–R6) junto con el run.
- **R8** — Si el plan resultante está **vacío** (no hay nada que compensar), el run DEBE
  quedar directamente `compensated` (journalear `run_compensated`).
- **R9** — `rollback` DEBE ser re-entrante: llamarlo de nuevo DEBE recomputar el plan según lo
  que aún falte, sin duplicar el evento `run_rollback`.

### `action=compensated`
- **R10** — `CompleteCompensation(runID, stepID)` DEBE validar que `stepID` pertenece al run y
  está en el plan vigente; si no, DEBE devolver error claro.
- **R11** — DEBE journalear `step_compensated` para ese step (en tx). Compensar dos veces el
  mismo step DEBE ser **no-op idempotente** (no re-journalea, no falla).
- **R12** — Cuando tras un `compensated` el plan queda vacío, el run DEBE pasar a
  `compensated` y journalear `run_compensated` (en la misma tx).
- **R13** — DEBE devolver el plan restante (LIFO) + el run actualizado.

### Tool / integración
- **R14** — `musubi_workflow` DEBE exponer `rollback` (run_id) y `compensated` (run_id, step).
  El conteo de tools MCP NO DEBE cambiar (acciones nuevas, no tools).
- **R15** — Build + suite verdes; model-free (sin LLM), Go puro, sin dependencias ni migración.

## Escenarios

### Escenario: plan LIFO
- **Given** un run `a→b→c` con `compensate` en los tres, completado hasta `c`
- **When** se llama `rollback`
- **Then** el plan es `[c, b, a]` (inverso al completado) y el run queda `compensating`

### Escenario: filtro por compensación y estado
- **Given** un run donde `a` tiene `compensate`, `b` no, y `c` fue reabierto (no está `done`)
- **When** se llama `rollback`
- **Then** el plan contiene sólo `a` (b no tiene compensación; c no está done)

### Escenario: ejecución de compensaciones y cierre
- **Given** un plan `[c, b, a]` y el run `compensating`
- **When** se llama `compensated(c)`, `compensated(b)`, `compensated(a)`
- **Then** cada llamada saca el step del plan; tras `a`, el run queda `compensated` y el
  journal tiene `run_compensated`

### Escenario: doble compensación es no-op
- **Given** `c` ya compensado
- **When** se llama `compensated(c)` otra vez
- **Then** es no-op (no error, no evento duplicado) y `c` no reaparece en el plan

### Escenario: rollback sin nada que compensar
- **Given** un run sin ningún step con `compensate`
- **When** se llama `rollback`
- **Then** el plan es vacío y el run queda directamente `compensated`

### Escenario: rollback re-entrante
- **Given** un run con plan `[c, b, a]`, ya se compensó `c`
- **When** se vuelve a llamar `rollback`
- **Then** el plan recomputado es `[b, a]` y no se duplica el evento `run_rollback`

## Fuera de alcance
- Rollback automático al fallar un step (disparo explícito; política del agente).
- Ejecución real de la compensación (la corre el agente).
- Sub-DAG de compensación con dependencias (LIFO plano).
- Migración de esquema.

## Preguntas abiertas
- [ ] ¿`compensate` puede referenciar otro step id, o es siempre una directiva de texto libre?
      (design; probable: texto libre, más simple y model-free; sin validación de referencia)
- [ ] ¿Un run en estado `compensated` puede volver a `rollback`? (design; probable: no-op,
      devuelve plan vacío)
- [ ] ¿La completitud del plan se recomputa por consulta o se guarda un contador? (design;
      probable: recomputar del journal, coherente con derivar-no-guardar)
