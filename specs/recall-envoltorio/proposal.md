# Propuesta — adelgazar el envoltorio de `musubi_recall`

**Estado:** propuesta, sin tocar código. Cada recorte se aprueba por separado (☐ al pie de cada uno).
**Fecha:** 2026-10-24 · **Origen:** Fase 0 del pivote de ahorro de tokens (auditoría del reel «Code Mode»).
**Pilar:** hacer al AGENTE eficiente sin perder potencia (decisiones `c07b47cd`, `4e23c916`; sacrificio máx 10%).

---

## 1. Qué medimos y por qué este es el lever

El reel «Code Mode» quedó confirmado pero resultó el lever equivocado para Musubi (las defs ya están 91%
diferidas; `ingest_url`/`fleet_*` casi no se usan). Midiendo dónde se van los tokens de verdad, el único
recorte **grande y de ganancia pura** que Musubi controla es el **envoltorio del resultado de `recall`**.

Medición sobre transcripts reales (2.909 `.jsonl`, ventana 2026-09-10→2026-10-24; scripts en
`scratchpad/{recall_args,recall_ratio,field_breakdown}.py`):

- **474 resultados de `recall`, 33.363 ítems** (70,4 por resultado).
- El **97%** de las llamadas pasa `token_budget` explícito (mediana 1.500, p90 3.000).
- **El resultado es 3,40× el presupuesto pedido** (p90 3,66; 100% de las llamadas lo superan).
- Desglose del payload: **gist 29,2%** · nombres JSON repetidos 28,0% · `topic_key` 11,8% · `id` 11,4% ·
  `score` 6,8% · `created_at` 6,7% · resto (`author`/`project_id`/`full_tokens`/`stale`/`provenance`) ~6%.

**La causa (código):** `internal/memory/recall.go:empaquetar` suma al presupuesto **sólo el texto del gist**
(`cost = EstimateTokens(gist)`, `gistMax=24`), pero cada `RecallItem` serializado lleva además `id`,
`topic_key`, `created_at`, `score`, `full_tokens` y las claves JSON repetidas. El gist —lo único que el
budget cuenta y lo que tiene valor— es el **29%** del payload; el **71% es envoltorio**.

**Nota de honestidad sobre el tamaño:** `recall` es ~7% de los tokens de resultado totales (el estimador
real de Musubi es `divJSON=2,6` chars/token, así que el payload real ≈ 3,8M tok/45d, no 2,48M). El 80%
sigue siendo **Bash**, que Musubi no puede tocar. Esto no es transformador; es el mejor recorte *de los que
Musubi controla*, y se amplifica porque el resultado del recall queda en contexto y **se re-lee cada turno**
(tu 68% de «releer contexto»).

**Qué NO proponer (ya hecho / no conviene):**
- *Quitar la indentación JSON* → **ya está** (`methods.go:jsonResult` serializa compacto; recuperó el ~28%
  de whitespace hace tiempo).
- *Recortar `precheck_codegraph`* → **no**: ya está acotado (10 símbolos/5 refs), inerte sin índice, y los
  comentarios lo justifican como 3× más barato que el `grep` que reemplaza. Es inversión, no desperdicio.
- *Tocar el gist, `id`, `topic_key`, `created_at`(día), `stale`* → son contenido/valor/correctitud. No se
  tocan salvo donde se diga explícito abajo.

---

## 2. Recortes propuestos (aprobás cada uno por separado)

Ahorros expresados como **% del payload de `recall`** y en **tok/día** (estimación real ÷2,6, promediada
sobre los ~85k tok/día de recall entre todos los agentes; **pre-multiplicador de re-lectura**, que lo
amplifica varias veces).

### Tier 1 — ganancia pura, riesgo bajo, listo para diff

#### Recorte 1.1 — dropear o gatear `score`
- **Qué:** dejar de serializar `score` en el camino model-free (hoy va salvo cuando el juez reordena, donde
  ya se omite). O gatearlo detrás de un flag de depuración opt-in.
- **Ahorro:** ~9% del payload (valor 6,8% + su clave en 33k ítems) ≈ **~8k tok/día**.
- **Riesgo:** BAJO, y además **mejora conducta**: el propio comentario en `recall.go:123-128` advierte que
  publicar `score` «invita a re-ordenar por él y deshacer, en silencio, un juicio que costó ~8,5 s». El
  agente no actúa sobre el puntaje crudo.
- **Blast radius:** `RecallItem.Score` (tag `omitempty`); goldens/tests en `internal/mcp/motor_con_freno_test.go`
  e `internal/memory/recall_test.go`. Consumidor = el modelo, no código Go.
- **Decisión:** ☐ aprobar  ☐ no  ☐ versión gateada (flag)

#### Recorte 1.2 — `created_at` a resolución de día
- **Qué:** emitir `created_at` como fecha (`2026-10-14`, ~10 chars) en vez de ISO8601 con hora
  (`2026-10-14T08:45:12Z`, ~20 chars). Se conserva el propósito documentado (la EDAD de la memoria).
- **Ahorro:** ~3,3% del payload ≈ **~3k tok/día**.
- **Riesgo:** BAJO. «Ayer vs. hace 8 meses» no necesita hora. *A verificar antes del diff:* que ningún
  consumidor ordene por `created_at` con precisión sub-día (no se encontró ninguno; el agente sólo lee la edad).
- **Blast radius:** `RecallItem.CreatedAt`; mismos tests que 1.1.
- **Decisión:** ☐ aprobar  ☐ no

**Subtotal Tier 1 ≈ 12% del payload ≈ ~11k tok/día (pre-multiplicador), sin perder ni una memoria.**

### Tier 2 — moderado, pide tu criterio (toca legibilidad o señal)

#### Recorte 2.1 — dropear `full_tokens`
- **Qué:** quitar `full_tokens` («costo de hidratar el contenido completo»). Su clave es larga
  (`"full_tokens":`, 14 chars) y va en cada ítem, así que pesa más que su valor.
- **Ahorro:** ~5,8% del payload (valor 1,1% + clave) ≈ **~5k tok/día**.
- **Riesgo:** MEDIO. Es la señal con la que el agente decide si vale la pena expandir un gist. Quitarla
  puede empujar a expandir más (o menos) a ciegas → posible roce con la regla de «no perder potencia».
  Alternativa: conservarlo sólo cuando supere un umbral (los gist caros, que son los que importan).
- **Decisión:** ☐ dropear  ☐ umbral  ☐ no

#### Recorte 2.2 — acortar las claves más largas
- **Qué:** nombres de campo más cortos para los 2-3 más caros (`topic_key`→`tk`, `created_at`→`ts`,
  `provenance`→`prov`). El resto (`id`, `gist`) se dejan por legibilidad.
- **Ahorro:** parte del 28% de claves; estimado ~5-8% del payload ≈ **~5-7k tok/día**.
- **Riesgo:** MEDIO. El agente lee estos campos POR NOMBRE; claves crípticas pueden degradar su uso (la
  regla del 10%). Se mitiga acortando sólo las más largas y dejando las semánticas (`gist`, `id`). **Rompe
  todos los goldens/tests que afirman la forma del JSON.**
- **Decisión:** ☐ aprobar (lista corta)  ☐ no

### Tier 3 — mayor payoff, mayor costo/riesgo (sólo si querés empujar)

#### Recorte 3.1 — `id` como ordinal de sesión
- **Qué:** devolver un índice corto por resultado (`1`, `2`, …) en vez del UUID de 36 chars, y que
  `memory_expand` acepte el ordinal (mapeado server-side a la sesión) o el UUID.
- **Ahorro:** ~11% del payload (el `id` entero) ≈ **~9k tok/día**.
- **Riesgo:** ALTO/MEDIO. Introduce estado de sesión (mapeo ordinal→UUID) y cambia el contrato de
  `memory_expand`. Feature real, con su propia superficie de prueba.
- **Decisión:** ☐ explorar en un spec aparte  ☐ no

#### Recorte 3.2 — serialización tabular (cabecera + filas)
- **Qué:** emitir los ítems como una tabla compacta (claves una vez en la cabecera) en vez de un objeto por
  ítem. Mata casi todo el 28% de claves repetidas.
- **Ahorro:** ~20%+ del payload ≈ **~17k tok/día**.
- **Riesgo:** ALTO. Cambia el formato que lee el modelo (riesgo de legibilidad/potencia) y rompe todo
  golden/consumidor. Es un cambio de contrato, no un ajuste.
- **Decisión:** ☐ explorar en un spec aparte  ☐ no

---

## 3. Arreglo aparte — el `token_budget` miente

Independiente de los recortes: la tool anuncia `token_budget` como *«Techo de tokens del resultado»*, pero
sólo acota el gist, e informa `used_tokens` **3,4× por debajo** de lo que realmente entró al contexto. Eso
afecta cómo **cada agente presupuesta su contexto** (pide 1.500 creyendo que techa su costo, gasta ~5.100).

- **Opción A (honesto):** contar el ítem serializado completo contra el budget. *Consecuencia:* para el
  mismo número pedido entran ~3,4× menos ítems → **es un recorte de cobertura** (roce con la regla del 10%);
  los callers tendrían que subir sus budgets para compensar. No es gratis en potencia.
- **Opción B (barato):** dejar el empaquetado como está pero reportar `used_tokens` real (medir el JSON
  final), y documentar en la tool que el budget acota el gist, no el payload. Cero cambio de conducta, sólo
  deja de mentir el número.
- **Recomendación:** B. Hace el número honesto sin recortar cobertura; los recortes de la §2 bajan el
  payload de verdad.
- **Decisión:** ☐ A  ☐ B  ☐ no

---

## 4. Blast radius y plan de verificación (cuando apruebes)

- **Superficies tocadas:** `internal/memory/recall.go` (`RecallItem`, `empaquetar`). El cambio alcanza
  **recall Y priming de arranque** (ambos salen de `empaquetar` — 15 callers, 5 de producción incluidos
  `DbEngine.Recall`, `PrimeContext`, `PrimeContextCtx`).
- **Goldens/tests a regenerar:** `internal/mcp/motor_con_freno_test.go`, `internal/memory/recall_test.go`,
  `proyecto_de_origen_test.go`, `origins_recall_test.go`, `observation_quarantine_test.go`.
- **Sin consumidor de producción del JSON:** lo lee el modelo; `ContentHash` ya es `json:"-"` y la inyección
  diferencial usa el struct en memoria. Baja el riesgo de los Tier 1/2.
- **Medición del antes/después:** re-correr `scratchpad/field_breakdown.py` sobre una sesión nueva tras el
  cambio y comparar tok/ítem.
- **Regla del 10%:** para 2.1/2.2/3.x, validar que la conducta del agente (expandir lo correcto, ubicar la
  memoria) no se degrada — con una prueba de conducta, no sólo bytes.

## 5. Qué NO voy a hacer sin tu OK explícito

Escribir código, regenerar goldens, commitear o desplegar. Esta propuesta es sólo el plan. Decime qué
recortes aprobás (por tier o por ítem) y recién ahí armo el diff para que lo revises antes de aplicar.
