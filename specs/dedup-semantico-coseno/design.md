# Design — dedup-semantico-coseno

## El gate: una tabla de verdad, no un `if` anidado

El corazón del cambio es `decideRelation`. Hoy despacha sobre `sim` (léxico). Ahora despacha sobre **dos** señales, con `cos` **opcional** (`*float64`: `nil` = no hay vector ⇒ camino histórico).

|  | `cos` ausente | `cos < cosFloor` | `cosFloor ≤ cos < cosAuto` | `cos ≥ cosAuto` |
|---|---|---|---|---|
| **`lex ≥ auto`** | histórico (auto) | **pending** (degrada, R6) | **pending** (degrada, R6) | **auto** (R4) |
| **`floor ≤ lex < auto`** | pending | pending | pending | pending |
| **`lex < floor`** | ignorar | ignorar (R8) | **pending** (R5 ⚡) | **pending** (R5 ⚡) |

⚡ = **lo que hoy es invisible** y pasa a verse: el duplicado semántico escrito con otras palabras.

**Sólo UNA celda auto-resuelve, y exige las dos señales altas.** De ahí sale R0 gratis: como el auto-resolve pide `lex ≥ auto` (la condición de hoy) **más** `cos ≥ cosAuto`, el conjunto de auto-supresiones es por construcción un **subconjunto** del de hoy. No hay forma de que el coseno cree una supresión nueva — no es una promesa, es una propiedad estructural del gate.

Dentro de la celda "auto" se conserva **intacta** la lógica actual (mismo `topic_key` + más reciente ⇒ `supersedes`; mismo topic pero no más reciente ⇒ `pending`; cross-topic ⇒ `related`).

## Obtener el coseno por candidata

`DetectRelations` necesita `cos(src, cand)` para **cada** candidata del pool (no sólo para las que devuelve el vecino-más-cercano).

- **Decisión:** una sola query que trae los vectores de **todos** los ids del pool (`WHERE observation_id IN (...) AND model_id = ?`) y se computa `CosineSimilarity` en memoria contra el vector de `src`.
- **Por qué no reusar el ranking de `SearchObservations`:** devuelve sólo el top-N por coseno. Una candidata que vino del pool **léxico** puede no estar en ese top-N ⇒ me quedaría sin su coseno justo cuando lo necesito para el AND-gate (R6, degradar). El pool es chico (`candidate_pool`, default 10 + vecinos), así que traer los vectores y computar exacto es barato y **completo**.
- **`model_id = ?`** (la procedencia actual) no es un detalle: comparar por coseno vectores de **otro** modelo da números sin sentido. Es exactamente el contrato que N1/#192 volvió confiable — **este slice depende de aquél**.
- Falta el vector de `src` **o** el de la candidata ⇒ `cos = nil` ⇒ esa candidata se juzga por el camino histórico (R7). La degradación es **por par**, no global.

## M2 — el pool

`conflictCandidates` pasa a devolver la **unión**: los del FTS (como hoy, mismo orden y exclusiones) **+** los vecinos por coseno de `SearchObservations(srcVec, pool)`, deduplicados por id. Sin `srcVec`, devuelve exactamente el pool de hoy (R2).

Se reusan las exclusiones existentes (propia id, archived, superseded) en ambos caminos: `SearchObservations` ya re-filtra contra SQLite.

## Config

```go
// CosineFloor: piso de coseno para que una candidata SEMÁNTICA entre como pending (default 0.85).
// CosineAutoThreshold: coseno mínimo para que el coseno CORROBORE una auto-resolución (default 0.90).
CosineFloor         float64 `yaml:"cosine_floor"`
CosineAutoThreshold float64 `yaml:"cosine_auto_threshold"`
```

Se usa el patrón **ausente-vs-explícito** del repo (`presentBlockKeys`): ausente ⇒ default; explícito (incluido `0`) ⇒ se respeta. `cosine_floor: 0` ⇒ el coseno no participa ⇒ rollback (R9).

## Los umbrales salen de MEDIR, no de estimar

Sobre la memoria real (393 obs, **77.028 pares**):

- **Casi-duplicados** (Jaccard ≥ 0.7): coseno **0.991**.
- **No relacionados** (Jaccard < 0.3): p50 **0.601**, p95 0.737, p99 0.786, **máx 0.884**.

**La línea de base del coseno es altísima (~0.60).** Texto del mismo dominio comparte vocabulario y el mean-pooling lo amplifica. Consecuencia práctica: un umbral "razonable" a ojo es catastrófico.

| umbral | pares con `cos ≥ u` y `lex < 0.3` |
|---|---|
| 0.75 | **2.661** (ruido) |
| 0.80 | 450 |
| **0.85** | **46** ← señal plausible |
| 0.90 | 0 |

⚠️ **Esta escala NO es la del recall.** Ahí las sims corren 0.40-0.50 porque son *query* vs *documento* (la query es corta). Acá es *documento* vs *documento*. **Reusar el `vector_floor: 0.30` del recall habría marcado casi TODO como duplicado.** Dos pisos de coseno con nombres parecidos y escalas distintas: el riesgo está señalado en el código.

- `cosine_auto_threshold = 0.90`: los duplicados reales están en 0.99 ⇒ las auto-resoluciones de hoy **sobreviven** al AND-gate; y 0 falsos positivos en 76k pares.
- `cosine_floor = 0.85`: 46 pares (0.06%) ⇒ volumen manejable de `pending` para el agente.

## Alternativas descartadas

- **OR (coseno alto ⇒ auto-resolver):** **prohibido por R0.** Los estáticos no evalúan predicados: *"usamos X"* y *"ya NO usamos X"* tienen coseno alto ⇒ un OR **auto-ocultaría memoria contradictoria en silencio**. Es exactamente el modo de falla que el track más quiere evitar.
- **Fusionar léxico y coseno en un score único** (p. ej. promedio o RRF): mezcla dos escalas con semánticas distintas y, sobre todo, **destruye el invariante R0** — un coseno alto podría compensar un léxico bajo y disparar un auto-supersede. La separación en un AND-gate es lo que hace la seguridad *demostrable* en vez de *esperable*.
- **Auto-merge de duplicados semánticos:** fuera de alcance por diseño. Model-free rutea a pending; el juicio lo da el agente.
- **Reusar `defaultSimilarityFloor` (0.3) para el coseno:** la medición lo mata — 0.3 está muy por debajo de la mediana del ruido (0.60).
