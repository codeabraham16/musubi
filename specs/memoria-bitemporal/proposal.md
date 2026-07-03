---
artifact: proposal
schema_version: "1.0"
change: memoria-bitemporal
status: archived
---

# Propuesta — Invalidación bi-temporal del grafo de hechos

## Intención
El grafo de hechos (`musubi_save_fact` / tabla `relations`) sólo sabe **acumular**
tripletas sujeto-predicado-objeto: nunca las **retira**. Si hoy guardo
`(Ana, trabaja_en, Acme)` y mañana `(Ana, trabaja_en, Globex)`, el grafo se queda con
**las dos** como si ambas fueran verdad simultánea. `RecallFacts` las devuelve juntas y
el agente no tiene forma de saber cuál es la vigente. La memoria envejece mal: cuanto más
se usa, más contradicciones acumula.

Queremos que Musubi ejecute la **transición de verdad** de forma model-free: cuando llega
un hecho nuevo que **contradice** a uno viejo según una regla determinista, el viejo se
**invalida** (se cierra su ventana de validez) en vez de convivir con el nuevo. Y queremos
poder preguntar "¿qué era verdad en tal momento?" (consulta *point-in-time*). Es el
diferenciador de Zep/Graphiti, alcanzable **sin LLM** porque el juez de contradicción son
**reglas de cardinalidad por predicado**, no comprensión semántica.

## Alcance
- **Incluye:**
  - Modelo **bi-temporal** en `relations` (migración v5, columnas aditivas):
    `valid_from` / `valid_to` (tiempo del EVENTO: desde/hasta cuándo el hecho es verdad) e
    `invalidated_at` / `superseded_by` (tiempo de TRANSACCIÓN: cuándo Musubi supo que dejó
    de ser vigente y qué hecho lo reemplazó). "Verdad actual" = `invalidated_at IS NULL`.
  - **Juez de contradicción por cardinalidad**, model-free: un conjunto **configurable** de
    predicados *single-valued* (funcionales: a lo sumo un objeto vivo por sujeto, p. ej.
    `trabaja_en`, `estado_actual`, `vive_en`). Al guardar `(S, P, O_new)` con P
    single-valued, se invalida todo `(S, P, O_old)` vivo con `O_old != O_new`. Los
    predicados no declarados son *many-valued* (no invalidan nada; `conoce`, `visitó`).
  - `RecallFacts` filtra por defecto a la **verdad actual** (`invalidated_at IS NULL`) y
    acepta un parámetro **`as_of`** opcional para consulta point-in-time
    (`valid_from <= as_of AND (valid_to IS NULL OR valid_to > as_of)`).
  - `SaveFact` acepta un `valid_from` opcional (cuándo empezó a ser verdad el hecho; default
    = ahora), y **revive** un triplete exacto previamente invalidado si se re-afirma.
  - La tool `musubi_save_fact` reporta cuántos hechos invalidó; `musubi_recall_facts` expone
    `as_of`.
- **No incluye (explícito):**
  - **Parser de fechas en lenguaje natural** ("hace 3 días") para derivar `valid_from`. Es
    backlog; acá `valid_from` es ahora o un ISO explícito. No inferimos fechas de prosa.
  - **Invalidación semántica** (contradicción por negación/antónimos). Eso requiere LLM y
    queda —como hoy— para el juez del agente vía `musubi_judge` en las observaciones. Acá
    sólo cardinalidad, que es determinista.
  - Bi-temporalidad de las **observaciones** (prosa). Este cambio es del grafo de HECHOS;
    las observaciones ya tienen su propio supersede en `conflicts.go`.
  - Cardinalidad **inferida** automáticamente del historial. Los predicados single-valued se
    **declaran** en config (con un default conservador), no se adivinan.

## Enfoque
Principio: **cerrar ventana, no borrar**. Un hecho invalidado nunca se elimina; se le marca
`valid_to`/`invalidated_at` y `superseded_by`, de modo que la historia queda **auditable y
reversible** y la consulta point-in-time es un simple filtro por fecha. El juez de
contradicción es puramente aritmético/relacional (¿el predicado está en el set
single-valued? ¿existe otro objeto vivo para ese sujeto?), sin ningún modelo. Todo ocurre
dentro de la transacción que ya usa `SaveFact`, sobre la misma base SQLite single-writer.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/database.go` + `migrations.go` (migración v5: 4 columnas + índice +
    backfill `valid_from = created_at`).
  - `internal/memory/graph.go` (`SaveFact` invalida por cardinalidad; `RecallFacts` filtra
    verdad actual + `as_of`; revivir triplete).
  - `internal/config/config.go` (`SingleValuedPredicates []string` + default + merge).
  - `internal/mcp/methods.go` + `registry.go` (params/salida de `save_fact` y `recall_facts`;
    sin tools nuevas → conteo intacto).
  - Tests nuevos de graph.go (invalidación, many-valued no invalida, point-in-time, revivir).
- Compatibilidad: **aditivo y retrocompatible**. Las columnas nuevas son nullable; el
  backfill deja los hechos existentes como vigentes (`invalidated_at IS NULL`,
  `valid_from = created_at`). Con el set single-valued por default vacío o mínimo, el
  comportamiento por defecto es idéntico al actual salvo el filtrado de invalidados (que sin
  invalidaciones no cambia nada).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Declarar single-valued un predicado que en realidad admite varios → se pierde info | La invalidación es **reversible** (no DELETE); default de single-valued conservador y **configurable**; el hecho invalidado se puede revivir re-afirmándolo |
| El agente usa el mismo predicado con distinta capitalización/idioma | Normalización case-insensitive del predicado al comparar contra el set |
| Romper `RecallFacts` (ahora filtra) para quien esperaba ver todo | El filtro por defecto es "verdad actual", que es lo correcto; el histórico sigue disponible vía `as_of`; los tests fijan el contrato |
| `as_of` mal formado | Validar/parsear ISO; ante error, degradar a "verdad actual" con nota |
| Migración sobre bases grandes | Sólo `ADD COLUMN` + un `UPDATE` de backfill acotado; idempotente por `user_version` |

## Estrategia de rollback
Aditivo. Rollback = revertir el PR; las columnas quedan inertes (un binario viejo ignora
columnas de más y su `SaveFact`/`RecallFacts` siguen funcionando: nunca las referencian).
`user_version` puede quedar en v5 sin daño. No hay borrado de datos que deshacer: como sólo
cerramos ventanas (nunca DELETE), incluso los hechos invalidados siguen en la tabla.

## Criterio de éxito
1. Guardar `(S, P, O_new)` con P single-valued invalida el `(S, P, O_old)` previo vivo, y
   `RecallFacts(S)` devuelve sólo `O_new` — cubierto por test.
2. Con P many-valued, guardar `(S, P, O2)` **no** invalida `(S, P, O1)`; ambos vivos — test.
3. `RecallFacts(S, as_of=T)` con T anterior a la invalidación devuelve `O_old`
   (point-in-time) — test.
4. Re-afirmar un triplete exacto previamente invalidado lo revive (`invalidated_at` vuelve a
   NULL) — test.
5. Todo model-free, Go puro, sin deps nuevas; build + suite verdes; migración idempotente y
   retrocompatible.
