# Propuesta — adelgazar el envoltorio de `musubi_recall`

**Estado:** **CERRADO el 2026-11-13.** Tier 1 completo, Tier 2.1 completo, §3 opción B completa, y el
Tier 3.1 se hizo en otra forma (id corto, ver `tier2-id-corto.md`). Tier 2.2 y 3.2 quedan ABIERTOS y se
des-recomiendan al pie (§6). Las casillas de abajo están marcadas con lo que REALMENTE pasó.
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
- **Decisión:** ☑ **aprobar (dropear, sin flag)** — hecho el 2026-11-13, commit del track
  `perf/el-payload-del-recall-sin-los-campos-que-nadie-lee`. Se destrabó al medir que el único
  consumidor (musubi-body) está EN PAUSA desde el 2026-09-24. La guarda 1.1 de
  `envoltorio_recall_test.go` quedó INVERTIDA, no borrada.

#### Recorte 1.2 — `created_at` a resolución de día
- **Qué:** emitir `created_at` como fecha (`2026-10-14`, ~10 chars) en vez de ISO8601 con hora
  (`2026-10-14T08:45:12Z`, ~20 chars). Se conserva el propósito documentado (la EDAD de la memoria).
- **Ahorro:** ~3,3% del payload ≈ **~3k tok/día**.
- **Riesgo:** BAJO. «Ayer vs. hace 8 meses» no necesita hora. *A verificar antes del diff:* que ningún
  consumidor ordene por `created_at` con precisión sub-día (no se encontró ninguno; el agente sólo lee la edad).
- **Blast radius:** `RecallItem.CreatedAt`; mismos tests que 1.1.
- **Decisión:** ☑ **aprobar** — en main. Guarda 1.2 en `envoltorio_recall_test.go` (10 chars, sin hora).

**Subtotal Tier 1 ≈ 12% del payload ≈ ~11k tok/día (pre-multiplicador), sin perder ni una memoria.**

### Tier 2 — moderado, pide tu criterio (toca legibilidad o señal)

#### Recorte 2.1 — dropear `full_tokens`
- **Qué:** quitar `full_tokens` («costo de hidratar el contenido completo»). Su clave es larga
  (`"full_tokens":`, 14 chars) y va en cada ítem, así que pesa más que su valor.
- **Ahorro:** ~5,8% del payload (valor 1,1% + clave) ≈ **~5k tok/día**.
- **Riesgo:** MEDIO. Es la señal con la que el agente decide si vale la pena expandir un gist. Quitarla
  puede empujar a expandir más (o menos) a ciegas → posible roce con la regla de «no perder potencia».
  Alternativa: conservarlo sólo cuando supere un umbral (los gist caros, que son los que importan).
- **Decisión:** ☑ **dropear** — hecho el 2026-11-13, mismo commit que 1.1. El riesgo MEDIO que anotaba
  este ítem («es la señal con la que el agente decide si expandir») resultó **inexistente**: medido, el
  campo no tuvo lector NUNCA, ni en musubi ni en musubi-body. La señal que el ítem temía perder no
  estaba siendo usada por nadie.

#### Recorte 2.2 — acortar las claves más largas
- **Qué:** nombres de campo más cortos para los 2-3 más caros (`topic_key`→`tk`, `created_at`→`ts`,
  `provenance`→`prov`). El resto (`id`, `gist`) se dejan por legibilidad.
- **Ahorro:** parte del 28% de claves; estimado ~5-8% del payload ≈ **~5-7k tok/día**.
- **Riesgo:** MEDIO. El agente lee estos campos POR NOMBRE; claves crípticas pueden degradar su uso (la
  regla del 10%). Se mitiga acortando sólo las más largas y dejando las semánticas (`gist`, `id`). **Rompe
  todos los goldens/tests que afirman la forma del JSON.**
- **Decisión:** ☐ **ABIERTO, y se des-recomienda** — ver §6: hoy `topic_key` es el 20,0 % del payload
  (el 2º después del gist), así que el ahorro subió; pero la palanca de verdad ya no es el ítem.

### Tier 3 — mayor payoff, mayor costo/riesgo (sólo si querés empujar)

#### Recorte 3.1 — `id` como ordinal de sesión
- **Qué:** devolver un índice corto por resultado (`1`, `2`, …) en vez del UUID de 36 chars, y que
  `memory_expand` acepte el ordinal (mapeado server-side a la sesión) o el UUID.
- **Ahorro:** ~11% del payload (el `id` entero) ≈ **~9k tok/día**.
- **Riesgo:** ALTO/MEDIO. Introduce estado de sesión (mapeo ordinal→UUID) y cambia el contrato de
  `memory_expand`. Feature real, con su propia superficie de prueba.
- **Decisión:** ☑ **hecho, pero NO como lo propone este ítem** — no se usó un ordinal de sesión (que
  habría metido estado de sesión). Se emite un **id corto de 8 hex, resolvible sin estado**, y
  `memory_expand` acepta las dos formas. Spec y medición en `tier2-id-corto.md`. En main desde el
  2026-11-02, impuesto en toda la flota el 11-09.

#### Recorte 3.2 — serialización tabular (cabecera + filas)
- **Qué:** emitir los ítems como una tabla compacta (claves una vez en la cabecera) en vez de un objeto por
  ítem. Mata casi todo el 28% de claves repetidas.
- **Ahorro:** ~20%+ del payload ≈ **~17k tok/día**.
- **Riesgo:** ALTO. Cambia el formato que lee el modelo (riesgo de legibilidad/potencia) y rompe todo
  golden/consumidor. Es un cambio de contrato, no un ajuste.
- **Decisión:** ☐ **ABIERTO, y se des-recomienda** — ver §6.

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
- **Decisión:** ☑ **B** — en main. `used_tokens` ahora mide el JSON final, y la guarda 3 de
  `envoltorio_recall_test.go` lo ata con dos cotas (supera la suma de gists y coincide con
  `EstimateTokens` del texto ±2). **Y la §6 explica por qué B no alcanzó.**

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

## 5. Qué se hizo, y con qué OK

Cada recorte se aplicó con una aprobación explícita y puntual (AskUserQuestion), no con una autorización
general del track. El Tier 1.1 + 2.1 + `author` salieron juntos el 2026-11-13 tras la respuesta «Esos dos +
author (−24,3 %)».

---

## 6. Cierre medido — el ahorro por ítem es real y NO se traduce en ahorro por llamada

Script: `specs/recall-envoltorio/scripts/` (se movieron al repo justamente porque los de `scratchpad/` que
citaba la §1 ya se perdieron). Ventanas limpias: la vieja termina antes del escalón del 10-27, y la nueva
arranca el 11-09, el día en que `len(id)` quedó parejo en toda la flota — **medir desde el merge (11-02) da
−5,1 % y es una ventana contaminada**, porque las sesiones ya abiertas siguieron con el binario viejo una
semana.

| | tok/ítem | ítems/llamada | `token_budget` pedido (mediana) | tok/resultado | ratio payload/budget |
|---|---|---|---|---|---|
| antes (09-12→10-26, 231 llamadas) | 114,6 | 63,7 | 1.500 | 7.303 | 4,87× |
| después (11-09→11-13, 36 llamadas) | **105,3** | 79,7 | **1.800** | 8.387 | **4,66×** |
| | **−8,1 %** | +25 % | +20 % | **+15 %** | −4,3 % |

**El dato incómodo, y es el hallazgo del track:** el ítem adelgazó un 8,1 % y el payload por llamada
**subió** un 15 %. No es que el cambio fallara: los callers pasaron a pedir budgets más grandes (mediana
1.500→1.800, p90 3.000→3.500), entraron un 25 % más de ítems, y se comieron el margen y más. La métrica que
aísla el cambio de la mezcla de llamadas es la última columna, el **ratio payload/budget**, invariante a que
el caller pida más — y ésa sí bajó.

**Por eso la §3-B no alcanzó.** Reportar `used_tokens` honesto dejó de mentir el número, pero el caller
sigue pidiendo contra un budget que acota **el gist**, no el payload: pide 1.800 creyendo techar su costo y
entran ~8.400. Mientras ese factor siga en 4,66×, cada token que se le saca al ítem vuelve como un ítem más.

**Dónde está la palanca que queda** (en orden, y ninguna es «adelgazar más el ítem»):

1. **§3 opción A** — contar el ítem serializado completo contra el budget. Es lo único que cierra el lazo:
   vuelve el número un techo de verdad. Tiene el costo que ya anotaba la §3 —para el mismo número pedido
   entran ~4,7× menos ítems, o sea es un recorte de cobertura— así que necesita una prueba de conducta y
   probablemente que los callers suban sus budgets a propósito, no por accidente.
2. **Por qué los callers pidieron más.** Subió un 20 % en seis semanas y nadie lo decidió: hay que medir si
   fue un caller nuevo, una skill, o el hook de arranque.
3. Tier 2.2 / 3.2 (claves cortas, tabular) sólo después de 1: hoy ahorrarían más que cuando se escribió
   esta propuesta —`topic_key` es el 20,0 % del payload— pero sin el techo arreglado el ahorro se recicla
   en más ítems, igual que el de este track.

### Dos mediciones independientes, y por qué sus porcentajes NO coinciden

El −8,1 % de la tabla sale de comparar dos ventanas reales. Existe **otra** medición del mismo cambio,
del 2026-11-03 (obs. `b561ac75`), hecha con un método distinto y mejor en un aspecto: atribuye cada
carácter del payload byte-exacto y corre un **contrafactual controlado sobre los mismos 249 ítems**, con
el estimador de `digest.go` replicado y validado (`est(raw) == used_tokens` del server, 0,00 % de error en
5 payloads). Da **−7,7 %**. Dos métodos independientes a 0,4 puntos: el número aguanta.

**Pero sus desgloses por campo se leen como si uno mintiera, y ninguno lo hace.** Es el denominador:

| campo | obs. `b561ac75` (11-03) | `cierre_envoltorio.py` (11-13) |
|---|---|---|
| `score` | 6,7 % | 10,8 % |
| `full_tokens` | 1,1 % | 6,9 % |
| `author` | 3,8 % | 6,6 % |

La del 11-03 cuenta **sólo el valor** y agrupa todas las claves JSON aparte (`KEYS` 25,3 % + `struct`
6,3 %). La del 11-13 le carga a cada campo **su valor más su clave**, que es lo que de verdad se ahorra al
dropearlo. Por eso `full_tokens` salta de 1,1 % a 6,9 %: el valor es un entero corto y la clave
`"full_tokens":` mide 14 caracteres. No hay que elegir una: **la propia obs. `b561ac75` ya hace la cuenta
de dropear** —«A') dropear score = 9,5 %»— y ahí coincide con el 10,8 % de acá, dentro de lo que explica la
ventana distinta. Al comparar las dos, mirá si el número incluye la clave.

### Lo que esa medición tenía y este cierre no

- **`project_id` es 7,5 % del payload y es CONSTANTE** (`"musubi"` en 249/249 ítems de un recall
  mono-proyecto). No se dropea —tiene lector vivo, el hook lo usa en `marcaDeProyecto`— pero se puede
  **factorizar al envelope** con un gate «si es único en el resultado → al envelope». Misma idea para
  `author`, que acá se dropeó en vez de factorizarse, y se pudo porque además no tenía lector.
- **El Tier 2a sólo alcanzó al 54 % de los ítems.** `IDCorto` acorta el UUID canónico de 36 chars; el 46 %
  restante son ids verbatim —`commit-<sha>` de 23 chars, `sdd/<change>/<phase>` de hasta 50— que quedan
  largos POR DISEÑO. Por eso el `id` cayó ~5 puntos y no los ~8 que proyectaba la §2. Sub-palanca abierta:
  acortar también los `commit-<sha>` a 8 hex.

Las dos siguen sujetas al rebote de arriba: suman bytes al ítem, y mientras el budget acote el gist, el
hueco se llena con otro ítem. Van después del §3-A, no antes.

**Desglose del ítem hoy** (ventana nueva, antes del recorte del 11-13): gist 30,6 % · `topic_key` 20,0 % ·
`score` 10,8 % · `created_at` 9,1 % · `project_id` 7,5 % · `full_tokens` 6,9 % · `author` 6,6 % · `id` 5,9 %
· `provenance` 2,3 % · `stale` 0,2 %. El recorte del 11-13 se lleva 24,3 % de eso: proyecta **~80 tok/ítem**
y un ratio de ~3,6×. Se re-mide cuando el binario nuevo esté parejo en la flota, con el mismo criterio de
ventana limpia que esta tabla — **no desde el día del merge.**
