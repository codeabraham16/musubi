---
artifact: spec
schema_version: "1.0"
change: hitl-interrupt-resume
status: archived
---

# Especificación — HITL: interrupt/resume durable

## Requisitos

### Modelo
- **R1** — `WorkflowStep` DEBE aceptar un campo opcional `await` (string): el prompt para el
  humano. Vacío = el step no pausa. NO DEBE haber migración (viaja en el JSON de la definición).
- **R2** — DEBE existir el estado de step `waiting_input` y el evento de journal `step_waiting`
  (en la tabla `run_events` existente).
- **R3** — Un step `waiting_input` NO DEBE ser terminal: DEBE **bloquear** a sus dependientes
  (sus `needs` no se consideran satisfechos hasta que se resuelva).

### Pausa en el scheduler
- **R4** — En `WorkflowReady`, un step candidato cuyo `when` pasa (o no tiene) y que declara
  `await` NO DEBE devolverse en `ready`: DEBE marcarse `waiting_input`, persistirse y
  journalearse `step_waiting` **una sola vez** (idempotente entre llamadas).
- **R5** — La evaluación de `when` DEBE ocurrir **antes** de pausar: un step con `await` cuyo
  `when` es falso DEBE saltarse (`skipped`), no pausar.
- **R6** — `WorkflowReady`/las respuestas del tool DEBEN poder surface los steps en
  `waiting_input` con su prompt `await` (lista `waiting`), para que el agente/humano sepa qué
  espera.

### Reanudación (`action=provide`)
- **R7** — `ProvideWorkflowInput(runID, stepID, input, status)` DEBE exigir que el step esté en
  `waiting_input`; si no, DEBE devolver error claro.
- **R8** — DEBE fijar `step_results[step] = input` y `step_status[step] = status` (done por
  defecto; done|failed válidos), journalear `step_completed` (payload status+result, uniforme
  con el resto) en tx, y recalcular el estado del run (cerrar si corresponde).
- **R9** — Tras `provide` con `done`, los dependientes del step DEBEN destrabarse
  (aparecer en `ready` en la próxima consulta); con `failed`, DEBEN permanecer bloqueados.

### Durabilidad
- **R10** — El estado `waiting_input` y el journal DEBEN persistir: una relectura fresca del
  run (otra sesión) DEBE ver el step en espera y poder proveerlo y continuar.

### Tool / integración
- **R11** — `musubi_workflow` DEBE exponer `provide` (run_id, step, input, status). El conteo
  de tools MCP NO DEBE cambiar (acción nueva).
- **R12** — Build + suite verdes; model-free (sin LLM), Go puro, sin dependencias ni migración.

## Escenarios

### Escenario: pausa en un step await
- **Given** un run `a → gate → b` donde `gate` declara `await: "Aprobar?"`, y `a` está done
- **When** se consulta `next` (WorkflowReady)
- **Then** `ready` NO contiene `gate`; el run marca `gate` como `waiting_input`; la respuesta
  surface `gate` en `waiting` con su prompt; `b` NO está ready (bloqueado por gate)

### Escenario: provide=done reanuda
- **Given** el run con `gate` en `waiting_input`
- **When** se llama `provide(gate, "ok, aprobado", done)`
- **Then** `gate` queda `done` con result "ok, aprobado"; `b` aparece en `ready`

### Escenario: provide=failed bloquea
- **Given** el run con `gate` en `waiting_input`
- **When** se llama `provide(gate, "rechazado", failed)`
- **Then** `gate` queda `failed`; `b` permanece bloqueado (no ready)

### Escenario: gate con `when` falso no pausa
- **Given** un `gate` con `await` y `when` que evalúa falso
- **When** se avanza el run
- **Then** `gate` se salta (`skipped`), no pasa a `waiting_input`

### Escenario: durabilidad entre sesiones
- **Given** un run con un step en `waiting_input`, persistido
- **When** se recarga el estado del run desde SQLite (relectura fresca) y se llama `provide`
- **Then** el run reanuda correctamente (el step se completa y los dependientes se destraban)

### Escenario: provide sobre step no-esperando es error
- **Given** un step que no está en `waiting_input`
- **When** se llama `provide` sobre él
- **Then** devuelve error claro

## Fuera de alcance
- Notificación/entrega al humano (email/Slack/push).
- Timeouts de espera (auto-resolución).
- Validación de esquema del input (texto libre).
- Migración de esquema.

## Preguntas abiertas
- [ ] ¿`provide` reutiliza `CompleteWorkflowStep` internamente o es un método propio? (design;
      probable: método propio que exige `waiting_input`, para no aflojar la guarda de complete)
- [ ] ¿La lista `waiting` se deriva del snapshot (steps en waiting_input + su Await) o del
      journal? (design; probable: del snapshot + la def, es O(1) y siempre refleja el estado)
- [ ] ¿Un step `await` puede también tener `repeat_while`? (design; probable: no se combina;
      el await es un gate, no un loop)
