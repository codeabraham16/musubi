---
artifact: design
schema_version: "1.0"
change: memoria-bitemporal
status: archived
---

# Diseño — Invalidación bi-temporal del grafo de hechos

## Decisión 1 — Cuatro columnas bi-temporales sobre `relations`, sin tabla de historia
Se agregan `valid_from`, `valid_to`, `invalidated_at`, `superseded_by` a la propia tabla
`relations`. No se crea una tabla de historial aparte: como **nunca borramos** (sólo cerramos
ventana), la fila invalidada ES su propio registro histórico. "Verdad actual" = filas con
`invalidated_at IS NULL`.
**Rationale:** una tabla de historia separada duplicaría el esquema y las escrituras sin
beneficio: la consulta point-in-time se resuelve con un filtro por fecha sobre la misma
tabla. Menos código, una sola fuente de verdad.
**Descartado:** tabla `relations_history` con triggers de copia (más superficie, más
sincronización que puede fallar).

## Decisión 2 — Dos ejes de tiempo con semántica explícita
- **Tiempo del evento** (`valid_from`, `valid_to`): desde/hasta cuándo el hecho es verdad *en
  el mundo*. `valid_to IS NULL` = sigue vigente.
- **Tiempo de transacción** (`invalidated_at`, `superseded_by`): cuándo Musubi *supo* que el
  hecho dejó de ser la verdad corriente, y qué hecho lo reemplazó. `invalidated_at IS NULL`
  = es la creencia actual.
En la invalidación por cardinalidad ambos se cierran a la vez (`valid_to = invalidated_at =
now`), porque el evento (cambió de trabajo) y el conocimiento (nos enteramos) coinciden. Se
mantienen separados igual para habilitar, sin otra migración, el caso futuro de un
`valid_from` histórico (evento pasado que registramos hoy).
**Rationale:** es el modelo de Zep/Graphiti; separar los ejes es lo que da la consulta
point-in-time correcta y la auditoría "qué creíamos y cuándo".

## Decisión 3 — Juez de contradicción = pertenencia a un set single-valued (config)
`SaveFact` decide si invalida con una única pregunta determinista: **¿el predicado
(normalizado, case-insensitive) está en `SingleValuedPredicates`?** Si sí, invalida los
`(S, P, O_old)` vivos con `O_old != O_new`. Si no, no toca nada.
Default **curado y chico**, sólo predicados claramente funcionales, en ES e inglés:
`trabaja_en, works_at, estado_actual, current_status, status, vive_en, lives_in,
ubicado_en, located_in, reporta_a, reports_to, asignado_a, assigned_to, pertenece_a,
belongs_to, prioridad, priority, version_actual, current_version, owner, responsable`.
Configurable: el usuario puede extender o vaciar.
**Rationale:** model-free y transparente. La cardinalidad es el único juez de contradicción
que no necesita entender el texto. Un default chico minimiza falsos positivos; y como la
invalidación es reversible (Decisión 5), un error se corrige re-afirmando.
**Descartado:** (a) inferir cardinalidad del historial (frágil, sorpresivo); (b) un flag
`single_valued` por llamada en la tool (traslada al agente una decisión de esquema que debe
ser estable y global); (c) hardcodear el set (no adaptable a otros dominios/idiomas).

## Decisión 4 — Orden de operaciones en la transacción: insertar/UPSERT primero, invalidar después
Dentro de la tx de `SaveFact`:
1. UPSERT del hecho nuevo `(S,P,O_new)` (revive si estaba invalidado — Decisión 5),
   obteniendo su `id`.
2. Si P es single-valued: `UPDATE relations SET valid_to=now, invalidated_at=now,
   superseded_by=:newID WHERE from_id=:S AND predicate=:P AND to_id != :O_new AND
   invalidated_at IS NULL` → devuelve el conteo de invalidados.
Necesitamos el `id` del hecho nuevo para `superseded_by`, por eso el orden es insertar antes
de invalidar. `to_id != :O_new` evita que el hecho nuevo se invalide a sí mismo.
**Rationale:** un solo `UPDATE` masivo invalida todos los objetos viejos de una; atómico por
la transacción; single-writer garantiza que nadie lea un estado intermedio.

## Decisión 5 — Revivir vía UPSERT sobre la constraint única existente
La constraint `UNIQUE(from_id, predicate, to_id)` se conserva. El insert pasa a:
```
INSERT INTO relations (from_id, predicate, to_id, valid_from, created_at)
VALUES (?, ?, ?, :validFrom, now)
ON CONFLICT(from_id, predicate, to_id) DO UPDATE SET
  valid_from = :validFrom, valid_to = NULL, invalidated_at = NULL, superseded_by = NULL
```
Así, re-afirmar un triplete exacto lo **revive** (limpia su invalidación) y actualiza
`valid_from`. `Created` = true sólo si fue INSERT real (fila nueva); revivir cuenta como
"no created" pero sí reactiva.
**Rationale:** cumple R9 sin lógica extra de "buscar y decidir"; el UPSERT hace ambos casos.
**Descartado:** SELECT-then-branch (más código, condición de carrera evitada de todos modos
por single-writer, pero el UPSERT es más simple).

## Decisión 6 — `valid_from` explícito: ISO validado, fallback a now; revivir usa now
`SaveFact` acepta `validFrom string` opcional. Si viene, se valida como ISO
(`time.Parse` de un par de layouts comunes → si falla, se ignora y se usa now, sin error).
Al revivir un triplete (R9), `valid_from` se fija a **now** (nueva afirmación de vigencia),
salvo que el llamador pase un `validFrom` explícito.
**Rationale:** point-in-time útil sin abrir la puerta al parser de fechas NL (fuera de
alcance). Degradar silenciosamente ante ISO inválido evita romper el guardado por un typo.

## Decisión 7 — `RecallFacts` filtra por defecto; `as_of` como filtro parametrizado en el BFS
`RecallFacts(entity, maxHops, maxFacts, asOf string)`:
- Sin `asOf`: cada consulta de expansión agrega `AND r.invalidated_at IS NULL`.
- Con `asOf` válido: reemplaza ese predicado por
  `AND r.valid_from <= :asOf AND (r.valid_to IS NULL OR r.valid_to > :asOf)`.
El filtro se construye una vez y se inyecta en `expandFrontier` (que recibe el fragmento
WHERE + args), de modo que **cada salto** del BFS respeta el mismo criterio temporal. `asOf`
inválido → se ignora (verdad actual).
**Rationale:** point-in-time correcto exige filtrar en cada hop, no sólo en el nodo inicial;
pasar el filtro como parámetro mantiene una sola ruta de expansión.

## Firmas (contrato Go)
```go
// SaveFact ahora invalida por cardinalidad y revive; devuelve #invalidados.
func (e *DbEngine) SaveFact(subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error)
type SaveFactResult struct { Created bool `json:"created"`; Invalidated int `json:"invalidated"` }

// RecallFacts filtra verdad actual; asOf opcional para point-in-time.
func (e *DbEngine) RecallFacts(entity string, maxHops, maxFacts int, asOf string) (GraphResult, error)
```
El set `singleValued` se pasa desde el handler (que tiene la config), igual que el patrón
TTL/maxAttempts del lease. Los llamadores (handler MCP + tests) se actualizan.

## Config
`SingleValuedPredicates []string` en la sección de memoria/grafo de `config` (default de la
Decisión 3), con merge `len==0 -> default`. Predicados comparados tras `strings.ToLower` +
trim.

## Alternativas globales descartadas
- **Borrado físico del hecho viejo:** pierde auditoría y point-in-time; contradice "cerrar
  ventana, no borrar".
- **Invalidación semántica con LLM:** rompe model-free; ya cubierta (para prosa) por el juez
  del agente sobre observaciones.
- **Versionado por número en vez de fechas:** las fechas dan point-in-time natural y ordenan
  igual; un contador no responde "qué era verdad el martes".
