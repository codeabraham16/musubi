---
artifact: proposal
schema_version: "1.0"
change: saga-compensacion-lifo
status: archived
---

# Propuesta — Saga: compensación LIFO en workflows

## Intención
El motor de workflows (`musubi_workflow`) sabe avanzar un DAG, pero **no sabe deshacer**.
Si un run de varios steps con efectos secundarios (crear recursos, escribir, postear) falla
a mitad de camino, no hay forma coordinada de **revertir** lo ya hecho: el agente queda con
un estado a medias y sin guía de qué deshacer ni en qué orden. Es el patrón **saga** clásico:
una transacción larga sin locks se vuelve consistente compensando los pasos completados en
**orden inverso** (LIFO) cuando algo falla.

Queremos que un step pueda declarar **cómo se compensa** (una directiva de undo), y que Musubi,
al iniciarse un rollback, calcule y **coordine** la secuencia de compensaciones en orden LIFO
—derivada del run journal— journaleando cada paso. Model-free como siempre: Musubi decide
QUÉ compensar y EN QUÉ ORDEN; el agente ejecuta la compensación real.

## Alcance
- **Incluye:**
  - Campo `compensate` en `WorkflowStep` (directiva de cómo deshacer ese step; vacío = sin
    compensación).
  - Estados de run nuevos: `compensating`, `compensated`.
  - `action=rollback` (run_id): inicia la saga — deriva del journal la lista de steps
    completados **con** compensación y **aún no compensados**, en orden **LIFO** (inverso al
    de completado), marca el run `compensating`, journalea `run_rollback` y devuelve el plan.
  - `action=compensated` (run_id, step): el agente reporta que ejecutó la compensación de un
    step; se journalea `step_compensated`; cuando no quedan pendientes, el run pasa a
    `compensated` (journalea `run_compensated`). Re-entrante e idempotente.
  - Eventos nuevos en el journal (mismos que ya heredan OTel): `run_rollback`,
    `step_compensated`, `run_compensated`.
- **No incluye (explícito):**
  - **Rollback automático al fallar un step.** El disparo es **explícito** (`action=rollback`):
    un step `failed` no siempre implica deshacer todo (el agente puede reintentar o seguir).
    Musubi ofrece el mecanismo; la política es del agente.
  - **Ejecutar la compensación.** Musubi coordina y journalea; la acción real de undo la
    corre el agente (Musubi es model-free, no ejecuta código ni efectos).
  - **Compensaciones con dependencias propias / sub-DAG de compensación.** El orden es LIFO
    plano (inverso al de completado), que es la semántica saga estándar. Un grafo de
    compensación queda fuera.
  - **Migración de esquema.** El campo `compensate` viaja dentro de la definición del run (ya
    persistida como JSON); los estados/eventos nuevos son strings en columnas que ya existen.

## Enfoque
La secuencia de compensación se **deriva del run journal** (el mismo principio que OTel):
el orden de completado es el orden `seq` de los eventos `step_completed`; LIFO = ese orden
invertido. Un step entra al plan si (a) su `compensate` no está vacío, (b) su estado actual
es `done`, y (c) no tiene aún un evento `step_compensated`. Así el plan es siempre una función
pura del journal + la definición, re-entrante (re-llamar `rollback` recomputa lo que falta) e
idempotente (compensar dos veces el mismo step es no-op). Todo se journalea en la misma
transacción que ya usa el motor, dentro de SQLite single-writer.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/workflow.go` (`WorkflowStep.Compensate`; consts de estado/evento;
    `CompensationStep`; `WorkflowRollback`; `CompleteCompensation`).
  - `internal/memory/backend.go` (interfaz `WorkflowStore`: 2 métodos nuevos).
  - `internal/mcp/methods.go` + `registry.go` (acciones `rollback` y `compensated`; sin tools
    nuevas → conteo intacto).
  - Tests nuevos de workflow.go (plan LIFO, filtro por compensación/estado, idempotencia,
    cierre del run).
- Compatibilidad: **puramente aditivo**. Sin migración. Definiciones sin `compensate` no
  cambian de comportamiento; `rollback` sobre un run sin steps compensables devuelve un plan
  vacío y cierra como `compensated` de inmediato.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| El agente compensa en el orden equivocado | Musubi **impone** el orden LIFO derivado del journal; el plan viene ya ordenado |
| Compensar un step reabierto (`repeat_while`) o no-done | Filtro: sólo steps con estado actual `done` entran al plan |
| Doble compensación de un step | Idempotencia: un step con evento `step_compensated` ya no reaparece en el plan |
| Rollback iniciado dos veces | Re-entrante: `run_rollback` se journalea una vez; el plan se recomputa según lo que falte |
| Scope creep a ejecutar el undo | Línea roja: Musubi coordina/journalea; el efecto lo hace el agente |

## Estrategia de rollback
Aditivo y sin estado nuevo persistente (sin migración). Revertir el PR quita las 2 acciones y
el campo `compensate` (que un binario viejo ignora dentro del JSON de la definición). Los runs
existentes no se ven afectados; el journal de compensación es sólo lectura adicional.

## Criterio de éxito
1. Un run con steps `a→b→c` (todos con `compensate`) completado hasta `c`, tras `rollback`
   devuelve el plan `[c, b, a]` (LIFO) — cubierto por test.
2. Steps sin `compensate` o no-`done` **no** entran al plan — test.
3. `compensated` de cada step lo saca del plan; al vaciarse, el run queda `compensated` — test.
4. Compensar dos veces el mismo step es no-op (idempotente); re-`rollback` recomputa lo que
   falta — test.
5. Todo model-free, Go puro, sin deps ni migración; build + suite verdes; 30 tools intactas.
