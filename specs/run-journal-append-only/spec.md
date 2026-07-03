---
artifact: spec
schema_version: "1.0"
change: run-journal-append-only
status: archived
---

# Especificación — Run journal append-only + idempotencia por step

## Requisitos

### Esquema (migración v6)
- **R1** — La migración DEBE crear la tabla `run_events` con: `id` (PK autoincrement),
  `run_id` (TEXT NOT NULL), `seq` (INTEGER NOT NULL), `step_id` (TEXT), `event_type`
  (TEXT NOT NULL), `payload` (TEXT), `idempotency_key` (TEXT), `created_at` (DATETIME
  DEFAULT CURRENT_TIMESTAMP).
- **R2** — La tabla DEBE tener `UNIQUE(run_id, seq)` (orden total por run) y
  `UNIQUE(run_id, idempotency_key)` (idempotencia; múltiples NULL permitidos por SQLite).
- **R3** — La migración DEBE ser idempotente vía el runner de `user_version` y NO DEBE
  tocar `workflow_runs` ni sus datos.

### Append en la misma transacción
- **R4** — Cada evento DEBE agregarse en la **misma transacción** que actualiza el snapshot
  `workflow_runs`, de modo que un fallo deje AMBOS sin cambios (nunca uno sin el otro).
- **R5** — El `seq` de un evento DEBE ser monótono creciente por `run_id`
  (`MAX(seq)+1` dentro de la tx). El primer evento de un run DEBE tener `seq = 1`.
- **R6** — Los tipos de evento DEBEN cubrir las transiciones: `run_started` (al crear un run
  nuevo), `step_completed` (payload con `status` y `result`), `step_skipped` (al saltar por
  `when` falso), `step_reopened` (al re-abrir por `repeat_while`), `run_done` (al cerrarse el run).

### Idempotencia (`CompleteWorkflowStep`)
- **R7** — `CompleteWorkflowStep` DEBE aceptar una `idempotencyKey` opcional. Si ya existe un
  evento con esa `(run_id, idempotency_key)`, la llamada DEBE ser **no-op**: NO DEBE mutar el
  snapshot ni agregar un evento nuevo, y DEBE devolver el estado actual del run sin error.
- **R8** — Con `idempotencyKey` vacío, `CompleteWorkflowStep` DEBE comportarse como hoy
  (sin dedup), y aun así DEBE registrar el evento `step_completed` (auditoría).
- **R9** — El evento `step_completed` DEBE llevar la `idempotency_key` provista (para que la
  próxima llamada con la misma clave sea detectada como duplicada).

### Lectura del journal
- **R10** — DEBE existir `WorkflowJournal(runID)` que devuelva los eventos del run en orden
  ascendente de `seq`, cada uno con `{seq, step_id, event_type, payload, created_at}`.
- **R11** — `musubi_workflow` DEBE exponer una acción `journal` (run_id) que devuelva esa
  traza, y `complete` DEBE aceptar `idempotency_key`. El conteo de tools MCP NO DEBE cambiar.

### Compatibilidad
- **R12** — Los runs creados antes de la migración (sin eventos) DEBEN seguir operando; su
  journal simplemente arranca vacío y se puebla desde el próximo evento.
- **R13** — El snapshot resultante de una secuencia de llamadas sin `idempotencyKey` DEBE ser
  idéntico al del motor actual (los tests existentes de workflow DEBEN seguir verdes).
- **R14** — El build y la suite completa DEBEN quedar verdes; todo model-free (sin LLM), Go
  puro, sin dependencias con cgo.

## Escenarios

### Escenario: complete idempotente
- **Given** un run con un step `build` listo, y se llama `complete(build, "ok", done, key="k1")`
- **When** se vuelve a llamar `complete(build, "otro", done, key="k1")`
- **Then** el segundo es no-op: `step_results[build]` sigue siendo `"ok"`, y el journal tiene
  un solo evento `step_completed` para esa clave

### Escenario: journal ordenado y completo
- **Given** un run de 2 steps `a → b` que se arranca y se completan ambos
- **When** se llama `WorkflowJournal(runID)`
- **Then** devuelve, en orden de `seq`, al menos: `run_started`, `step_completed(a)`,
  `step_completed(b)` y `run_done`

### Escenario: atomicidad snapshot + evento
- **Given** un `complete` válido
- **When** se aplica
- **Then** el snapshot (`step_status`/`step_results`) y el evento `step_completed` quedan
  ambos presentes; no existe un estado donde el snapshot avanzó pero el evento falta

### Escenario: retrocompatibilidad sin idempotency_key
- **Given** la misma secuencia de `start`/`complete` sin claves de idempotencia
- **When** se ejecuta contra el motor con journal
- **Then** el `step_status`/`step_results`/`status` final es idéntico al del motor sin journal
  (los tests existentes pasan sin cambios de aserción)

### Escenario: step saltado y reabierto quedan en el journal
- **Given** un run con un step con `when` falso (se salta) y otro con `repeat_while` (se reabre)
- **When** se avanza el run
- **Then** el journal contiene `step_skipped` para el primero y `step_reopened` para el segundo

## Fuera de alcance
- Reemplazo del snapshot por un fold del journal.
- Replay/rollback interactivo, saga LIFO, time-travel, export OTel.
- Poda/retención del journal.
- Journal de la pizarra (`work_units`) o de la memoria.

## Preguntas abiertas
- [ ] ¿La idempotencia se detecta con un SELECT previo o se apoya en la constraint UNIQUE
      (capturando el error de conflicto)? (design; criterio: claridad vs. atomicidad;
      probable: SELECT previo dentro de la tx, más legible que parsear el error)
- [ ] ¿`run_done` se emite como evento separado o como flag en el último `step_completed`?
      (design; probable: evento separado, para que el journal sea autocontenido)
- [ ] ¿`WorkflowReady` (que persiste skips) también appendea en tx, o los skips se journal-ean
      de forma best-effort? (design; probable: en tx, por consistencia)
