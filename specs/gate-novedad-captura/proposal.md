# Proposal — gate-novedad-captura

## Intención

Cerrar **M4**: que la memoria que Musubi captura **solo** también pase por la detección de duplicados/relaciones. Hoy **no pasa por ninguna**.

`DetectRelations` se llama **únicamente** desde el handler MCP de `musubi_save_observation` (`methods.go:251/258`). Los **dos** caminos de captura **automática** la saltean por completo:

| Camino | Detección |
|---|---|
| `musubi_save_observation` — el agente guarda **explícito** | ✅ |
| **C3 — commits capturados** (`capture.go:191`) | ❌ **ninguna** |
| **C4 — error→fix** (`methods.go:1588`) | ❌ **ninguna** |

Su único dedup es `FindByContentHash`: **hash exacto del contenido**. Byte-idéntico dedupea; **cualquier otra redacción se guarda como memoria nueva e independiente**, sin marca ni relación.

Es justo la mitad "falso positivo" del problema original (*"que guarde ruido"*): la captura automática es la fuente de **mayor volumen** de memoria y es la que **menos** control tiene. Y ahora que #193 le dio ojos semánticos a `DetectRelations`, el arreglo es cablearla donde falta.

## El peligro de cablearlo ingenuo (y la restricción que sale de ahí)

`DetectRelations` **puede auto-supersede** (ocultar memoria) cuando: mismo `topic_key` + léxico alto + la nueva es más reciente.

**En la captura de commits, TODOS comparten `topic_key = "git-commit"`.** Ese `topic_key` es un **balde**, no un tema. Dos commits de mensaje parecido (`fix: typo en el README` … `fix: typo en el CHANGELOG`) tienen léxico alto y mismo "topic" ⇒ el más nuevo **auto-ocultaría** al anterior. **Pérdida de memoria real, automática y silenciosa** — y en el camino donde no hay ningún agente mirando.

De ahí la regla de este cambio, que es la misma del track:

> **En el camino automático, la detección NO auto-resuelve NUNCA: sólo rutea a `pending`.**

El agente juzga después (`musubi_judge`). Model-free **detecta y marca**; no decide qué se borra.

## Alcance

- `ConflictOptions.DetectOnly` (nuevo): fuerza **todos** los veredictos a `pending`; **jamás** auto-supersede ni auto-related. Es el modo del camino automático.
- Cablear `DetectRelations` con `DetectOnly` en:
  - **C3** — cada commit capturado (`captureCommits`).
  - **C4** — cada observación de error→fix (`methods.go:1588`).
- El camino explícito del agente (`musubi_save_observation`) **no cambia**: conserva su auto-resolve (ahí sí hay un `topic_key` real elegido por el agente, y el AND-gate de #193 ya lo protege).

## Fuera de alcance (explícito)

- **NO se descarta ni se saltea ningún guardado.** El gate de novedad **nunca** hace auto-NOOP: un duplicado semántico se **guarda igual** y queda **marcado** `pending`. Descartar en silencio sería pérdida de memoria — el modo de falla que el track más evita.
- No se toca el dedup por hash exacto (ése sí es un NOOP seguro: contenido byte-idéntico **es** lo mismo).
- No se toca `classifyCommit`, ni qué commits se capturan, ni la consolidación/retención.

## Estrategia de rollback

- `conflicts.enabled: false` ⇒ no corre nada (ya existe).
- `DetectOnly` es aditivo: sin él, el comportamiento de `DetectRelations` es exactamente el de hoy. Revertir el PR deja los caminos automáticos como estaban (sin detección).
- Sin migración de esquema. Las relaciones que crea son `pending`: **no ocultan nada**, sólo se surfacean.

## Riesgos

- **Costo por captura.** `DetectRelations` corre por cada commit capturado (FTS + búsqueda vectorial + coseno contra el pool). Acotado por `candidate_pool` (10), y la captura sólo corre cuando hay commits **nuevos** (no en cada turno). El binario de captura ya paga la carga de la tabla (~1.2s), así que el delta es marginal — **pero hay que medirlo**, no asumirlo.
- **Volumen de `pending`.** Si la captura genera muchas relaciones pendientes, el agente se satura. Mitiga: sólo entran pares que pasan un piso (léxico 0.3 **o** coseno 0.85), y los umbrales del coseno se calibraron en #193 sobre 77k pares.
- El error→fix (C4) es fire-and-forget: la detección no debe romper el `musubi_log_error` si falla (best-effort, como el resto del camino).
