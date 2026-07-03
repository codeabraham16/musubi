---
artifact: proposal
schema_version: "1.0"
change: hitl-interrupt-resume
status: archived
---

# Propuesta — HITL: interrupt/resume durable (gates de aprobación)

## Intención
El motor de workflows avanza el DAG de forma autónoma: en cuanto un step está listo, se
ofrece para ejecutarse. Pero muchos flujos reales necesitan una **pausa para un humano**:
"aprobá el deploy a prod antes de seguir", "confirmá el borrado", "revisá esta fase SDD antes
de implementar". Hoy no hay forma de que un step **espere una decisión externa** y que esa
espera sea **durable** (sobreviva a reinicios/compactaciones): el estado ya vive en SQLite,
pero el motor no sabe pausar.

Queremos que un step pueda declarar que **requiere input/aprobación** (`await`): al quedar
listo, en vez de ofrecerse como ready, el run se **pausa** en ese step (`waiting_input`) y
surface el prompt para el humano. Más tarde, alguien **provee** la decisión/entrada
(`action=provide`) y el run **continúa** exactamente donde estaba. Durable por construcción
(el estado y el journal ya persisten). Es el patrón interrupt/resume de LangGraph, model-free.

## Alcance
- **Incluye:**
  - Campo `await` en `WorkflowStep` (el prompt/pregunta para el humano; vacío = sin pausa).
  - Estado de step nuevo: `waiting_input`.
  - El scheduler (`WorkflowReady`): cuando un step con `await` queda listo (y su `when` pasa),
    en vez de devolverlo como ready lo marca `waiting_input`, lo persiste y journalea
    `step_waiting`. Un step `waiting_input` **bloquea** a sus dependientes (no es terminal).
  - `action=provide` (run_id, step, input, status): resuelve un step en espera — fija su
    resultado a `input` y su estado a `done` (aprobado) o `failed` (rechazado), journalea
    `step_completed` (uniforme con el resto: saga/OTel lo ven igual) y reanuda el run.
  - Las respuestas del tool que devuelven `ready` DEBEN además surface los steps `waiting`
    (con su prompt), para que el agente/humano sepa qué está pausado.
- **No incluye (explícito):**
  - **Notificación/entrega al humano** (email, Slack, push). Musubi expone QUÉ espera y su
    prompt; el canal de aviso es del integrador. Local-first, sin clientes de red.
  - **Timeouts de espera** (auto-aprobar/auto-rechazar tras X). El await es indefinido hasta
    que se provee; un TTL de aprobación queda como follow-up.
  - **Formularios/validación de esquema del input.** `input` es texto libre (la decisión o el
    dato); validarlo es del agente.
  - **Migración de esquema.** El campo `await` viaja en la definición ya persistida; el estado
    y el evento nuevos son strings.

## Enfoque
El estado y el journal **ya son durables** (SQLite, resumible) — esa es justamente la base
del diferencial de Musubi, así que "interrupt/resume durable" es sobre todo enseñarle al
scheduler a **no ofrecer** un step con `await` y a **pausar** el run en él. La reanudación es
el flujo normal: `provide` completa el step (como un `complete` con un origen humano) y el
motor recalcula ready. Como un step en `waiting_input` no es terminal, sus dependientes
quedan bloqueados hasta la decisión — semántica de gate correcta y sin estado nuevo que
sincronizar. Todo se journalea en la misma tx que el motor ya usa.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/workflow.go` (`WorkflowStep.Await`; `StepWaiting`; `EventStepWaiting`;
    lógica de pausa en `WorkflowReady`; `ProvideWorkflowInput`; helper de steps en espera).
  - `internal/memory/backend.go` (interfaz `WorkflowStore`: `ProvideWorkflowInput`).
  - `internal/mcp/methods.go` + `registry.go` (acción `provide`; surface de `waiting` en las
    respuestas; sin tools nuevas → conteo intacto).
  - Tests nuevos (pausa en await, bloqueo de dependientes, provide done/failed, reanudación,
    durabilidad tras relectura).
- Compatibilidad: **puramente aditivo**. Sin migración. Definiciones sin `await` no cambian de
  comportamiento (nunca pausan).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Un step en espera se ofrece por error como ready | El scheduler lo excluye explícitamente y lo marca `waiting_input`; test lo fija |
| `provide` sobre un step que no está esperando | Error claro ("el step no está en espera"); sólo un `waiting_input` se puede proveer |
| El run parece "colgado" sin señal de qué espera | Las respuestas surface la lista `waiting` con el prompt de cada step |
| Reanudación no durable | El estado (`waiting_input`) y el journal ya persisten en SQLite; una relectura fresca reanuda igual — cubierto por test |
| Scope creep a notificaciones | Línea roja: Musubi expone el estado; el aviso al humano es del integrador |

## Estrategia de rollback
Aditivo y sin estado nuevo persistente (sin migración). Revertir el PR quita la acción
`provide` y el campo `await` (que un binario viejo ignora en el JSON de la def); un step con
`await` bajo un binario viejo simplemente no pausaría. Los runs existentes no se afectan.

## Criterio de éxito
1. Un step con `await` que queda listo NO aparece en `ready`; el run lo marca `waiting_input`
   y surface su prompt — cubierto por test.
2. Los dependientes de un step en espera quedan bloqueados hasta que se provee — test.
3. `provide(step, input, done)` reanuda el run (el step queda `done` con result=input, los
   dependientes se destraban); `provide(..., failed)` lo deja `failed` — test.
4. Una relectura fresca del run (otra "sesión") ve el `waiting_input` y puede proveer y
   continuar — durabilidad, test.
5. Todo model-free, Go puro, sin deps ni migración; build + suite verdes; 30 tools intactas.
