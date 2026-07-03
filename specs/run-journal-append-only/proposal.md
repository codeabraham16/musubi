---
artifact: proposal
schema_version: "1.0"
change: run-journal-append-only
status: archived
---

# Propuesta — Run journal append-only + idempotencia por step

## Intención
El motor de workflows (`musubi_workflow` / tabla `workflow_runs`) guarda sólo un
**snapshot mutable** del estado: `step_status`/`step_results` se sobrescriben en cada
avance. Eso trae dos límites reales:
1. **No hay idempotencia.** Si un agente llama `complete` dos veces para el mismo step
   (reintento, doble entrega, red que reenvía), la segunda sobrescribe la primera sin que
   nadie lo note. En una orquestación multi-agente con reintentos esto corrompe resultados.
2. **No hay historia.** El snapshot no recuerda *qué pasó y cuándo*: no se puede auditar la
   secuencia de un run, ni exportar traza (OTel), ni construir después replay/HITL/saga/
   time-travel. La memoria de la orquestación es amnésica.

Queremos un **journal append-only** (`run_events`): cada transición del run (arranque,
step completado/saltado/reabierto, run cerrado) se **agrega** como un evento inmutable, con
una **clave de idempotencia** que hace que un `complete` repetido sea un **no-op** seguro.
El snapshot se conserva como *read-model* materializado (verdad corriente barata); el
journal es el registro de verdad histórico. Es el cimiento estructural que —según la
investigación de Track 13— multiplica: idempotencia hoy, y replay/HITL/saga/observabilidad
mañana, todo sin reescribir el motor.

## Alcance
- **Incluye:**
  - Tabla nueva `run_events(id, run_id, seq, step_id, event_type, payload, idempotency_key,
    created_at)` con `UNIQUE(run_id, seq)` y `UNIQUE(run_id, idempotency_key)` (migración v6).
  - Escritura del evento **en la misma transacción** que actualiza el snapshot, de modo que
    journal y read-model **nunca divergen** (event-sourcing con read-model materializado).
  - Eventos emitidos: `run_started`, `step_completed` (payload: status + result),
    `step_skipped`, `step_reopened` (loop `repeat_while`), `run_done`.
  - **Idempotencia por step**: `CompleteWorkflowStep` acepta una `idempotency_key` opcional;
    si ya existe un evento con esa clave para el run, la llamada es **no-op** y devuelve el
    estado actual (no re-aplica ni re-escribe).
  - Método `WorkflowJournal(runID)` + acción `journal` en `musubi_workflow` para leer la
    traza (auditoría/observabilidad).
- **No incluye (explícito):**
  - **Reemplazar el snapshot por un fold del journal.** El snapshot sigue siendo la verdad
    corriente (materializada en la misma tx). Recovery = leer el snapshot, no re-ejecutar el
    journal — Musubi no ejecuta código, así que el replay-por-re-ejecución no aplica.
  - **Replay/rollback interactivo, saga LIFO, time-travel, export OTel.** Son los frutos que
    este journal *habilita*, pero quedan para cambios posteriores. Acá se construye el cimiento.
  - **Journal de la pizarra (`work_units`) o de la memoria.** Este cambio es del motor de
    workflows; los otros subsistemas quedan fuera.

## Enfoque
Event-sourcing pragmático: el journal es la fuente de verdad **histórica** (append-only,
inmutable), y el snapshot `workflow_runs` es un **read-model** materializado que se actualiza
en la **misma transacción** que agrega el evento. Como `workflow_runs` vive en SQLite
single-writer, escribir snapshot + evento juntos es atómico y nunca divergen. La idempotencia
sale gratis de `UNIQUE(run_id, idempotency_key)`: un `complete` repetido con la misma clave o
bien choca con la constraint (y se trata como no-op) o se detecta antes con un SELECT. El
`seq` monótono por run da orden total para la auditoría y el futuro replay.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/database.go` + `migrations.go` (migración v6: tabla `run_events` + índices).
  - `internal/memory/workflow.go` (append de eventos en tx; idempotencia en
    `CompleteWorkflowStep`; `WorkflowJournal`; `RunEvent` struct).
  - `internal/memory/backend.go` (interfaz `WorkflowStore`: firma de `CompleteWorkflowStep`
    + `WorkflowJournal`).
  - `internal/mcp/methods.go` + `registry.go` (acción `journal`, param `idempotency_key`;
    sin tools nuevas → conteo intacto).
  - Tests nuevos de workflow.go (idempotencia, journal ordenado, eventos por transición).
- Compatibilidad: **aditivo**. El snapshot y su API siguen; `idempotency_key` es opcional
  (ausente → comportamiento actual). Runs viejos sin eventos siguen funcionando (el journal
  arranca vacío para ellos).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Snapshot y journal divergen | Se escriben en la **misma transacción**; single-writer garantiza atomicidad |
| Refactor de `CompleteWorkflowStep` a tx rompe la lógica de `repeat_while`/cierre | Conservar la lógica intacta; sólo mover la persistencia a una tx que además appendea; tests existentes de workflow deben seguir verdes |
| Colisión de `idempotency_key` legítimamente distinta entre runs | La constraint es por `(run_id, idempotency_key)`, no global |
| Crecimiento del journal | Append-only por diseño; poda/retención queda fuera de alcance (runs son acotados); documentar |
| `seq` con carreras | `MAX(seq)+1` dentro de la tx sobre single-writer; sin condición de carrera real |

## Estrategia de rollback
Aditivo. Rollback = revertir el PR; la tabla `run_events` queda inerte (ningún otro camino la
lee) y el snapshot sigue siendo autosuficiente. `user_version` puede quedar en v6 sin daño.
Ningún dato del snapshot se migra ni se pierde: el journal es una capa nueva al lado.

## Criterio de éxito
1. Dos `complete` del mismo step con la **misma** `idempotency_key` producen un solo cambio;
   el segundo es no-op y devuelve el estado ya aplicado — cubierto por test.
2. `WorkflowJournal(runID)` devuelve los eventos en orden de `seq`, cubriendo start →
   completes → done — test.
3. El snapshot resultante es idéntico al del motor actual para las mismas llamadas (sin
   `idempotency_key`), probando retrocompatibilidad — los tests existentes siguen verdes.
4. Snapshot y journal se escriben atómicamente (un fallo no deja uno sin el otro) — test.
5. Todo model-free, Go puro, sin deps nuevas; build + suite verdes; migración idempotente.
