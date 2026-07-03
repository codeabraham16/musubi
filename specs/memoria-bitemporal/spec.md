---
artifact: spec
schema_version: "1.0"
change: memoria-bitemporal
status: archived
---

# Especificación — Invalidación bi-temporal del grafo de hechos

## Requisitos

### Esquema (migración v5)
- **R1** — La migración DEBE agregar a `relations` las columnas: `valid_from` (DATETIME),
  `valid_to` (DATETIME), `invalidated_at` (DATETIME) y `superseded_by` (INTEGER, id de la
  relación que la reemplazó). DEBE ser aditiva (`ADD COLUMN`).
- **R2** — La migración DEBE backfillear `valid_from = created_at` para las filas
  existentes, dejándolas vigentes (`valid_to`, `invalidated_at`, `superseded_by` en NULL).
- **R3** — La migración DEBE crear un índice que soporte la búsqueda de hechos vivos por
  sujeto+predicado (p. ej. `(from_id, predicate, invalidated_at)`).
- **R4** — La migración DEBE ser idempotente vía el runner de `user_version`.

### Cardinalidad (configuración)
- **R5** — DEBE existir un conjunto **configurable** de predicados *single-valued*
  (`config`: `SingleValuedPredicates []string`) con un default conservador. La comparación
  contra ese conjunto DEBE ser **case-insensitive** (y sobre el predicado normalizado).
- **R6** — Un predicado NO declarado single-valued DEBE tratarse como *many-valued*: guardar
  otro objeto para el mismo (sujeto, predicado) NO DEBE invalidar nada.

### Guardado con invalidación (`SaveFact`)
- **R7** — Al guardar `(S, P, O_new)` con P single-valued, el sistema DEBE invalidar toda
  relación viva `(S, P, O_old)` con `O_old != O_new`: fijar `valid_to = now`,
  `invalidated_at = now` y `superseded_by = id(nuevo hecho)`. NO DEBE borrar la fila.
- **R8** — El hecho nuevo DEBE quedar vivo con `valid_from = now` (o el `valid_from`
  explícito provisto) e `invalidated_at IS NULL`.
- **R9** — Si el triplete exacto `(S, P, O)` ya existe pero está **invalidado**, re-afirmarlo
  DEBE **revivirlo** (`invalidated_at = NULL`, `valid_to = NULL`, `valid_from = now`), en vez
  de crear un duplicado o no hacer nada.
- **R10** — `SaveFact` DEBE aceptar un `valid_from` opcional (marca de tiempo desde la cual
  el hecho es verdad). Ausente → `now`. NO se infieren fechas de texto libre.
- **R11** — `SaveFact` DEBE reportar cuántas relaciones invalidó (además de si creó una nueva).
- **R12** — La invalidación por cardinalidad DEBE ocurrir dentro de la **misma transacción**
  que inserta el hecho nuevo (o todo o nada).

### Recuperación (`RecallFacts`)
- **R13** — Por defecto, `RecallFacts` DEBE devolver únicamente hechos **vigentes**
  (`invalidated_at IS NULL`).
- **R14** — `RecallFacts` DEBE aceptar un parámetro `as_of` opcional (marca ISO). Con `as_of`
  presente, DEBE devolver los hechos que eran válidos en ese instante:
  `valid_from <= as_of AND (valid_to IS NULL OR valid_to > as_of)`. La expansión del grafo
  (BFS multi-hop) DEBE respetar ese mismo filtro en cada salto.
- **R15** — Un `as_of` mal formado NO DEBE causar error: DEBE degradar a "verdad actual" (R13).

### Tool / integración
- **R16** — `musubi_save_fact` DEBE exponer el `valid_from` opcional y reportar el conteo de
  invalidados en su salida. `musubi_recall_facts` DEBE exponer el `as_of` opcional. NO DEBE
  cambiar el conteo de tools MCP (sin tools nuevas).
- **R17** — El build y la suite completa DEBEN quedar verdes; todo model-free (sin LLM), Go
  puro, sin dependencias con cgo.

## Escenarios

### Escenario: predicado single-valued invalida al anterior
- **Given** el set single-valued incluye `trabaja_en`, y existe vivo `(Ana, trabaja_en, Acme)`
- **When** se guarda `(Ana, trabaja_en, Globex)`
- **Then** `(Ana, trabaja_en, Acme)` queda invalidado (`invalidated_at` seteado,
  `superseded_by` = id del nuevo) y `RecallFacts(Ana)` devuelve sólo `trabaja_en Globex`

### Escenario: predicado many-valued no invalida
- **Given** `conoce` NO está en el set single-valued, y existe `(Ana, conoce, Beto)`
- **When** se guarda `(Ana, conoce, Carla)`
- **Then** ambos quedan vivos y `RecallFacts(Ana)` devuelve `conoce Beto` y `conoce Carla`

### Escenario: consulta point-in-time
- **Given** `(Ana, trabaja_en, Acme)` válido desde T0, invalidado en T1 por `Globex`
- **When** se llama `RecallFacts(Ana, as_of=T0.5)` (entre T0 y T1)
- **Then** devuelve `trabaja_en Acme` (lo que era verdad en ese momento), no `Globex`

### Escenario: revivir un triplete invalidado
- **Given** `(Ana, trabaja_en, Acme)` fue invalidado por `Globex`
- **When** más tarde se re-afirma `(Ana, trabaja_en, Acme)`
- **Then** `(Ana, trabaja_en, Acme)` vuelve a estar vivo (`invalidated_at IS NULL`) y `Globex`
  queda invalidado (por ser single-valued y distinto)

### Escenario: retrocompatibilidad del backfill
- **Given** una base con hechos previos a la migración (sin columnas bi-temporales)
- **When** se aplica la migración v5 y luego se hace `RecallFacts`
- **Then** todos los hechos previos aparecen como vigentes (`valid_from = created_at`,
  `invalidated_at IS NULL`), sin cambios de comportamiento observable

## Fuera de alcance
- Parser de fechas en lenguaje natural para `valid_from`.
- Invalidación por contradicción semántica (negación/antónimos) — requiere LLM; queda para
  el juez del agente sobre observaciones.
- Bi-temporalidad de observaciones (prosa).
- Inferencia automática de cardinalidad desde el historial.

## Preguntas abiertas
- [ ] ¿El default de `SingleValuedPredicates` es vacío (opt-in total) o un set curado ES+EN?
      (design; criterio: minimizar falsos positivos vs. utilidad inmediata; probable: set
      curado y chico de predicados claramente funcionales)
- [ ] ¿`valid_from` explícito se pasa como ISO string y se valida, o sólo se acepta `now`?
      (design; probable: ISO validado, fallback a now)
- [ ] ¿La revivencia (R9) resetea `valid_from` a now o conserva el original? (design;
      probable: now, porque es una nueva afirmación de vigencia)
