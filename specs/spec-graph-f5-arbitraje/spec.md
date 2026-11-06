---
artifact: spec
schema_version: "1.0"
change: spec-graph-f5-arbitraje
status: draft
---

# Especificación — F5: arbitraje de conflictos spec↔observación

## Requisitos

Cada requisito es verificable y atómico. El "sistema" es el motor de memoria
(`internal/memory`) más el puente de proyección (`internal/mcp`).

- **R1 (detección spec→creencia)** — Al proyectar o re-proyectar una spec
  `shared`, el sistema **DEBE** correr la detección de relaciones con la
  proyección como *source*, de modo que una observación humana que la contradice
  quede como relación `pending` surfaceada para `musubi_judge`.

- **R2 (nunca auto-oculta)** — Una relación proyección-de-spec↔creencia **DEBE**
  nacer `pending`. El sistema **NO DEBE** auto-resolverla ni auto-ocultar la
  creencia: la contradicción semántica entre "usamos X" y una spec que dice "Z"
  no es deducible léxicamente, así que no cumple la compuerta AND
  (léxico≥auto ∧ mismo topic) que habilita el auto-supersede.

- **R3 (guard de seguridad: la creencia nunca oculta la spec)** — El sistema
  **NO DEBE** aplicar el efecto `supersedes` (marcar `superseded_by`) cuando el
  *target* es una proyección de spec, venga el veredicto del camino automático o
  de `musubi_judge`. Ocultar una proyección es fútil (se regenera del FS) y
  además invierte la autoridad.

- **R4 (la spec cruza dominios)** — La detección **NO DEBE** descartar un par por
  dominios ajenos (`dominiosAjenos`) cuando uno de los lados es una proyección de
  spec. Una spec **PUEDE** contradecir una creencia de cualquier `topic`, igual
  que un commit del registro histórico.

- **R5 (autoritativa, protegida en dos capas)** — Una proyección de spec participa
  en la detección como *source* (es el disparo de R1: desde la spec recién
  proyectada se surfacean las creencias que la contradicen). Como *target*, en
  cambio, una creencia **NO DEBE** poder desplazarla, y la protección es doble e
  independiente: (a) **no nace** ninguna relación que tome la proyección como
  *target* — `complementaryPair` la trata como registro protegido (igual que un
  `historicalRecord` o el registro declarado) y el loop hace `continue`; (b) aun si
  tal relación existiera por otra vía (p. ej. un `INSERT` directo), su efecto
  `supersedes` se **rechaza** en `markSuperseded` (R3). Cada capa tiene su sabotaje
  aislado. (Decisión del MVP léxico-puro: la proyección se comporta EXACTO como un
  commit del registro histórico, no como una observación append-only que igual
  admite ser *target* de una relación.)

- **R6 (aislamiento por tenant)** — La detección disparada por una proyección
  **NO DEBE** crear relaciones contra observaciones de otro tenant; reusa el
  guard de `projectID` ya presente en `DetectRelations`.

- **R7 (no-regresión)** — Para pares obs↔obs que **no** involucran una proyección
  de spec, el comportamiento **DEBE** ser idéntico al previo a F5. El predicado
  nuevo sólo matchea el namespace reservado (`spec-proj:` / topic `spec/<id>`).

- **R8 (alcance shared-only)** — Sólo las specs `shared`, ya materializadas por
  F4a, entran en F5. Las specs locales **NO** se materializan ni se arbitran en
  este MVP.

## Escenarios

### Escenario: una spec actualizada contradice una creencia vieja (R1, R2)
- **Given** una spec `shared` `s1` proyectada como observación `spec-proj:<t>:s1`
  (topic `spec/s1`) y una observación humana `o1` del mismo tenant que afirma lo
  contrario
- **When** `s1` se re-proyecta tras un cambio de su `SPEC.md`
- **Then** nace una relación `pending` con `source = proyección(s1)` y
  `target = o1`, surfaceada para `musubi_judge`, y `o1` permanece visible
  (`superseded_by IS NULL`)

### Escenario: una creencia no puede desplazar a una proyección de spec (R3, R5)
- **Given** una creencia `o2` del mismo tenant que contradice a `proyección(s1)`
- **When** corre la detección con `o2` como *source* (la proyección sería el *target*)
- **Then** (capa a) NO nace ninguna relación con `proyección(s1)` como *target*:
  `complementaryPair` la protege y el loop saltea el par
- **And** (capa b) si una relación con `proyección(s1)` como *target* existiera por
  otra vía y se resolviera como `supersedes`, el efecto se rechaza:
  `proyección(s1).superseded_by` sigue `NULL`, la spec sigue en el recall, y la
  relación NO queda resuelta (permanece `pending`)

### Escenario: la spec contradice una creencia de otro dominio (R4)
- **Given** `proyección(s1)` (topic `spec/s1`) y una creencia `o3` de un `topic`
  de dominio distinto, contradictoria
- **When** corre la detección disparada por `s1`
- **Then** el par NO se descarta por `dominiosAjenos`; nace la relación `pending`

### Escenario: aislamiento por tenant (R6)
- **Given** `proyección(s1)` en el tenant A y una creencia contradictoria `o6` en
  el tenant B
- **When** corre la detección disparada por `s1`
- **Then** NO nace ninguna relación entre `proyección(s1)` y `o6`

### Escenario: par obs↔obs ajeno a specs no cambia (R7)
- **Given** dos observaciones `o4`, `o5`, ninguna de ellas proyección de spec
- **When** corre la detección
- **Then** el resultado es idéntico al previo a F5 (la suite de conflictos
  existente sigue verde sin tocarse)

## Fuera de alcance

- Specs **locales** (no `shared`): hoy no se materializan como observación.
- La dirección inversa **creencia nueva → spec vigente** (chequear contra specs al
  guardar una observación).
- Cualquier **auto-resolución** spec↔creencia.
- **Migración de esquema**, tabla nueva o discriminador cross-store.
- Materializar specs por fuera de la proyección unidireccional de F4a.

## Preguntas abiertas

Las tres quedaron resueltas en la fase design (ver `design.md` → "Hallazgo que
fija el diseño"); se conservan acá con su resolución para no perder el porqué.

- [x] **Embeddings de la proyección.** F4a escribe la proyección con `emb=nil`.
  Si la detección por coseno necesita un vector persistido para encontrar la
  creencia contradictoria, R1 no dispara en la práctica apoyándose sólo en el
  piso léxico. Decidir en design: ¿se computa un embedding para la proyección al
  cablearla por `DetectRelations`, o el piso léxico alcanza para el MVP?
  → **Resuelta (design D6, hallazgo #1): piso léxico, MVP léxico-puro.** La
  proyección va `emb=nil` a propósito (el seam `write` de la spec honra
  `candado_no_cruza_la_red`), así que ningún par spec↔creencia tiene coseno; F5
  es léxico por construcción. El coseno queda para F5.1 (computar el vector en un
  seam `read`). Peor caso del falso negativo léxico: una relación de menos, nunca
  memoria oculta de más.
- [x] **Dónde vive el guard de R3.** ¿En `markSuperseded` (más abajo, cubre todo
  camino: auto y `musubi_judge`) o en `ResolveObsRelation`/`decideRelation`?
  Preferencia: `markSuperseded`, por profundidad — un solo punto que ningún
  camino puede saltear.
  → **Resuelta (design D4): en `markSuperseded`**, el punto único por el que pasan
  el auto-path y `ResolveObsRelation`. Más una segunda capa en `complementaryPair`
  (R5a) que impide que la relación siquiera nazca. Dos capas, cada una con su
  sabotaje aislado.
- [x] **Señal de `specProjection`.** ¿Matchear por prefijo de id (`spec-proj:`) o
  por topic (`spec/<id>`)? Elegir la más estable y la que no colisione con una
  observación humana que casualmente use ese topic.
  → **Resuelta (design D1): por prefijo de id `spec-proj:`**, la señal que F4a
  reservó para esto (`spec_projection.go:17-19`). El topic `spec/<id>` lo podría
  usar por casualidad una nota humana, que quedaría sobre-protegida.
