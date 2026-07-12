# Spec — gate-novedad-captura

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go` (modo `DetectOnly`), `cmd/musubi/capture.go` (C3), `internal/mcp/methods.go` (C4).

## Invariantes de seguridad (mandan sobre todo)

- **R0 — El camino AUTOMÁTICO no auto-oculta memoria NUNCA.** Con `DetectOnly`, `DetectRelations` NO DEBE producir ninguna relación con `status = resolved`, y NO DEBE llamar a `markSuperseded`. Todos los veredictos DEBEN ser `pending`.
  - *Razón:* en la captura de commits todos comparten `topic_key = "git-commit"` — un **balde**, no un tema. El auto-supersede (mismo topic + léxico alto + más nuevo) ocultaría un commit anterior por parecerse en el mensaje. Y no hay ningún agente en el loop para notarlo.
- **R1 — El gate de novedad NO DEBE descartar ni saltear ningún guardado.** Un duplicado semántico DEBE **guardarse igual** y quedar **marcado** con una relación `pending`. Un auto-NOOP silencioso sería **pérdida de memoria**.
  - El dedup por **hash exacto** (`FindByContentHash`) SÍ sigue siendo un NOOP válido: contenido byte-idéntico **es** lo mismo.

## Modo `DetectOnly`

- **R2** — `ConflictOptions.DetectOnly` (bool, default `false`). En `false`, `DetectRelations` DEBE comportarse **exactamente** como hoy (el camino explícito del agente no cambia).
- **R3** — En `true`, toda relación emitida DEBE tener `Relation = pending` y `Status = pending`, cualquiera sea la combinación de léxico y coseno.
- **R4** — En `true`, la `Confidence` DEBE conservar la señal más fuerte (para que el agente pueda priorizar qué mirar primero).
- **R5** — El criterio de **qué pares entran** (los pisos: léxico ≥ `similarity_floor` **o** coseno ≥ `cosine_floor`) NO DEBE cambiar entre los dos modos: `DetectOnly` sólo cambia el **veredicto**, no el **pool**.

## Cableado en los caminos automáticos

- **R6 (C3)** — `captureCommits` DEBE correr la detección con `DetectOnly` sobre cada observación de commit que **efectivamente guarde** (no sobre las deduplicadas por hash: no hay observación nueva que relacionar).
- **R7 (C4)** — El camino error→fix DEBE correr la detección con `DetectOnly` sobre la observación guardada.
- **R8 (best-effort)** — Un fallo de la detección NO DEBE romper la captura ni el `musubi_log_error`: se logea y se sigue. La captura es una red de seguridad; no puede ser ella la que rompa el turno.
- **R9** — Con `conflicts.enabled: false` la detección NO DEBE correr en ningún camino (comportamiento de hoy).

**Escenario M4.a (el duplicado automático queda marcado)** — *Given* un commit ya capturado y otro nuevo que dice lo mismo con otras palabras, *When* corre la captura, *Then* el nuevo se **guarda** y queda una relación **`pending`** contra el anterior (hoy: se guarda **sin ninguna marca**).

**Escenario M4.b (R0 — no se auto-oculta un commit)** — *Given* dos commits con mensajes muy parecidos (léxico alto) y el mismo `topic_key = "git-commit"`, y coseno alto, *When* corre la captura, *Then* **NINGUNO** queda `superseded`, y la relación es `pending`. (Sin `DetectOnly`, este caso auto-supersedería: es el bug que se evita.)

**Escenario M4.c (R1 — nada se descarta)** — *Given* un commit semánticamente duplicado de uno existente, *When* corre la captura, *Then* la observación **se guarda igual** (existe en la base) y NO se saltea.

**Escenario M4.d (el camino explícito no cambia)** — *Given* `musubi_save_observation` con `DetectOnly = false`, *When* hay un casi-duplicado con léxico y coseno altos y mismo topic, *Then* auto-resuelve `supersedes` como siempre.

## No-objetivos (verificables)

- NO se cambia `classifyCommit`, ni qué commits se capturan, ni la consolidación/retención.
- NINGUNA ruta automática auto-suprime, auto-fusiona ni descarta memoria.
