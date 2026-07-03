---
artifact: design
schema_version: "1.0"
change: run-journal-append-only
status: archived
---

# Diseño — Run journal append-only + idempotencia por step

## Decisión 1 — Journal append-only + snapshot como read-model materializado (misma tx)
`run_events` es la fuente de verdad histórica (inmutable, append-only); `workflow_runs`
sigue siendo el read-model de "estado corriente", materializado **en la misma transacción**
que agrega el evento. Nunca se leen los eventos para reconstruir el estado en caliente: se
lee el snapshot (barato, O(1)). El journal existe para idempotencia, auditoría y el futuro
replay/observabilidad.
**Rationale:** es event-sourcing pragmático. Reescribir el estado como fold del journal sería
un cambio grande y riesgoso sin beneficio inmediato (Musubi no re-ejecuta código, así que el
recovery correcto es leer el último estado, no re-aplicar eventos). Mantener ambos en una tx
sobre SQLite single-writer garantiza que jamás divergen.
**Descartado:** snapshot-derivado-del-journal (rewrite mayor, fuera de alcance) y journal
best-effort fuera de tx (puede divergir del snapshot).

## Decisión 2 — Idempotencia por SELECT previo dentro de la tx (no por captura de constraint)
`CompleteWorkflowStep(runID, stepID, result, stepStatus, idempotencyKey)`: si
`idempotencyKey != ""`, al inicio de la tx se hace
`SELECT COUNT(*) FROM run_events WHERE run_id=? AND idempotency_key=?`. Si > 0 → **no-op**:
rollback de la tx (nada que escribir) y se devuelve el estado actual del run. La constraint
`UNIQUE(run_id, idempotency_key)` queda como **red de seguridad** (defensa en profundidad),
no como mecanismo primario.
**Rationale:** el SELECT explícito es más legible y devuelve el estado actual de forma
natural; parsear el error de conflicto de SQLite es frágil y acopla al driver. La constraint
igual protege ante una carrera imposible (single-writer) o un bug futuro.
**Descartado:** `INSERT ... ON CONFLICT DO NOTHING` + revisar RowsAffected (mezcla la señal de
idempotencia con la de inserción; menos claro para devolver "el estado ya aplicado").

## Decisión 3 — Todas las escrituras del motor pasan por una tx que appendea
Se introduce un helper:
```go
func appendRunEvent(tx *sql.Tx, runID, stepID, eventType, payload, idempKey string) error
```
que calcula `seq = MAX(seq)+1` para el run e inserta el evento (idempKey vacío → NULL).
- `StartWorkflowRun`: envuelve el INSERT del run en una tx; si creó fila nueva (RowsAffected>0)
  appendea `run_started`. Reabrir un run existente (ON CONFLICT DO NOTHING) no re-emite.
- `CompleteWorkflowStep`: tx que (a) chequea idempotencia (Decisión 2); (b) aplica la lógica
  de estado EXISTENTE (status/result, `repeat_while` re-abre, cierre del run); (c)
  `persistRunStatusTx(tx, run)` actualiza el snapshot; (d) appendea `step_completed` (con la
  idempKey) y, si el step se re-abrió, `step_reopened`, y si el run quedó done, `run_done`.
- `WorkflowReady`: cuando marca steps `skipped` por `when` falso, envuelve el persist + un
  `step_skipped` por cada uno en una tx.
`persistRunStatus` se refactoriza a `persistRunStatusTx(tx, run)` (misma lógica de
"allTerminal → done"); los llamadores le pasan la tx.
**Rationale:** una sola vía de escritura por transición garantiza que snapshot y journal se
muevan juntos. La lógica de negocio (repeat_while, cierre) no cambia: sólo se relocaliza su
persistencia dentro de una tx.
**Descartado:** appendear los eventos fuera de la tx del snapshot (viola R4).

## Decisión 4 — Orden y contenido de los eventos de `CompleteWorkflowStep`
En un `complete` que además cierra el run y no re-abre: se emite `step_completed` y luego
`run_done` (dos eventos, `seq` consecutivos). Si el step se re-abre por `repeat_while`: se
emite `step_completed` y luego `step_reopened` (el run no cierra). `run_done` es un **evento
separado**, no un flag, para que el journal sea autocontenido y el futuro consumidor (OTel,
replay) no tenga que inferir el cierre.
Payload de `step_completed` = JSON `{"status": <done|failed>, "result": <result>}`.
**Rationale:** eventos discretos y autocontenidos son la base correcta para replay/traza.

## Decisión 5 — `RunEvent` struct e interfaz
```go
type RunEvent struct {
    Seq       int    `json:"seq"`
    StepID    string `json:"step_id,omitempty"`
    EventType string `json:"event_type"`
    Payload   string `json:"payload,omitempty"`
    CreatedAt string `json:"created_at"`
}
func (e *DbEngine) WorkflowJournal(runID string) ([]RunEvent, error)   // ORDER BY seq
// interfaz WorkflowStore:
CompleteWorkflowStep(runID, stepID, result, stepStatus, idempotencyKey string) (WorkflowRun, error)
WorkflowJournal(runID string) ([]RunEvent, error)
```
El handler MCP `complete` lee `idempotency_key` opcional del payload; nueva acción `journal`
llama `WorkflowJournal`. Los llamadores de `CompleteWorkflowStep` (handler + tests) se
actualizan a la nueva firma.

## Decisión 6 — Constraint y NULLs
`UNIQUE(run_id, idempotency_key)`: en SQLite, múltiples filas con `idempotency_key IS NULL`
NO violan un UNIQUE (los NULL se consideran distintos), así que los eventos sin clave
(run_started, step_skipped, run_done, y completes sin key) coexisten sin choque. `step_id` es
NULL para eventos de run (run_started/run_done).

## Alternativas globales descartadas
- **Cola/broker externo para eventos:** rompe local-first; el journal en SQLite alcanza.
- **Un evento por llamada sin distinguir tipo:** pierde la semántica que replay/OTel necesita.
- **Reconstruir el snapshot desde el journal en cada lectura:** O(n) por lectura y un rewrite
  del motor; el read-model materializado es O(1) y de bajo riesgo.
