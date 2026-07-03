---
artifact: design
schema_version: "1.0"
change: saga-compensacion-lifo
status: archived
---

# Diseño — Saga: compensación LIFO

## Decisión 1 — `compensate` es una directiva de texto libre, sin validación de referencia
`WorkflowStep.Compensate string` es la descripción de cómo deshacer el step (p. ej. "borrar
el recurso creado en X", "revertir el commit"). No referencia otro step id ni se valida.
**Rationale:** model-free y flexible — la compensación es una instrucción para el agente, no
un nodo del DAG. Referenciar step ids ataría la compensación a la topología y pediría
validación; el texto libre es más simple y suficiente (el agente ya es el que ejecuta).
**Descartado:** `compensate` como step-id de un step de compensación (acopla topología, pide
validación, y no aporta sobre una directiva clara).

## Decisión 2 — El plan se DERIVA del journal en cada consulta (sin contador ni tabla)
`compensationPlan(run, journal)` recorre los eventos, arma el orden de completado (seq de los
`step_completed`, deduplicado y quedándose con el último por step), lo **invierte** (LIFO), y
filtra: `compensate != ""` AND `run.StepStatus[step] == done` AND sin evento
`step_compensated`. Devuelve `[]CompensationStep{StepID, Compensate}`.
**Rationale:** el mismo principio de OTel/derivar-no-guardar. El plan es una función pura del
journal + la definición → re-entrante e idempotente por construcción, sin estado extra que
mantener sincronizado. El costo (recorrer el journal) es trivial (runs acotados).
**Descartado:** un contador de "compensados restantes" o una columna de estado de
compensación por step (estado duplicado que puede desincronizarse del journal).

## Decisión 3 — Marcar `run_rollback` una sola vez, chequeando el journal
`WorkflowRollback` journalea `run_rollback` sólo si aún no existe ese evento para el run
(`SELECT ... WHERE event_type='run_rollback'`). Así re-llamar `rollback` recomputa el plan sin
duplicar el evento de inicio. El estado del run pasa a `compensating` (idempotente).
**Rationale:** re-entrancia limpia (R9) sin flags; el journal ya es la fuente de verdad de
"¿se inició el rollback?".
**Descartado:** un flag booleano en el snapshot (otro estado que sincronizar).

## Decisión 4 — Cierre a `compensated` cuando el plan queda vacío
Tanto `rollback` (si el plan nace vacío) como `compensated` (si tras marcar un step el plan se
vacía) DEBEN: fijar el run a `compensated` y journalear `run_compensated` (una sola vez,
chequeando que no exista). Todo en la misma tx que el `step_compensated`/`run_rollback`.
**Rationale:** el cierre es una consecuencia derivada del plan vacío, calculada en el mismo
punto donde el plan cambia; garantiza atomicidad snapshot+journal.

## Decisión 5 — Contrato Go
```go
type CompensationStep struct {
    StepID     string `json:"step"`
    Compensate string `json:"compensate"`
}
// Inicia la saga: marca compensating, journalea run_rollback (una vez), devuelve el plan LIFO.
func (e *DbEngine) WorkflowRollback(runID string) ([]CompensationStep, WorkflowRun, error)
// Marca un step como compensado (idempotente); devuelve el plan restante + run actualizado.
func (e *DbEngine) CompleteCompensation(runID, stepID string) ([]CompensationStep, WorkflowRun, error)
```
Estados: `RunCompensating = "compensating"`, `RunCompensated = "compensated"`.
Eventos: `EventRunRollback = "run_rollback"`, `EventStepCompensated = "step_compensated"`,
`EventRunCompensated = "run_compensated"`.
`WorkflowStep` gana `Compensate string yaml:"compensate" json:"compensate,omitempty"`.

## Decisión 6 — Persistencia atómica reutilizando el patrón del journal
Ambos métodos abren una tx, hacen el `UPDATE workflow_runs SET status=...` (nuevo helper
`setRunStatusTx(tx, runID, status)` o reutilizar un UPDATE directo) + `appendRunEvent(...)`
para cada evento, y commitean. Se reutiliza `appendRunEvent` (ya existe) y `WorkflowJournal`
para derivar el plan. `WorkflowRunStatus` para leer el snapshot (StepStatus + Def con el
nuevo campo).
**Rationale:** cero infraestructura nueva; el journal y el patrón de tx ya están.

## Decisión 7 — Validaciones y bordes
- `rollback`/`compensated` sobre run inexistente → error claro.
- `compensated(step)` de un step que no está en el plan vigente (no-done, sin compensate, o ya
  compensado) → **no-op idempotente** si ya estaba compensado; **error** si el step no existe
  o nunca fue compensable. (Distinguir "ya hecho" de "inválido".)
- Un run ya `compensated` que recibe `rollback` → plan vacío, no-op (no re-journalea).
**Rationale:** distinguir idempotencia (ya hecho, ok) de error (pedido inválido) es lo honesto.

## Alternativas globales descartadas
- **Estado de compensación por step en el snapshot** (Decisión 2): drift.
- **Rollback automático al primer `failed`** (fuera de alcance): la política es del agente.
- **Sub-DAG de compensación**: LIFO plano cubre la semántica saga estándar; un grafo de undo
  es sobre-ingeniería para el caso real.
