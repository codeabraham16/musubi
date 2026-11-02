# Tier 2 del envoltorio del recall — acortar el `id` del payload a 8 hex

Estado: 2a APROBADO por el dueño («Aplicar 2a ya», 2026-10-26), IMPLEMENTADO y VERIFICADO. El
**Hallazgo A** de la revisión adversarial (§3) —el id del recall es también PK exacta de tools vecinas
que no resolvían cortos— quedó **CERRADO**: el dueño eligió la opción (i) del fork y se mergeó en #772
(promote/discard resuelven la ref corta de 8 hex; `save_observation` con id= la rechaza). Con eso el
bloqueo se levantó. El 2026-11-02 la rama se rebasó sobre `main` (que ya incluye #772 y #779) y se
re-verificó sobre esa base: build + vet + tests de internal/mcp e internal/memory en VERDE juntos, y los
dos sabotajes del §4 corridos con deploy/pruebas/sabotaje.sh (borrar el loop → (a) rojo; ref del vecino →
(b) rojo por identidad), cada uno con motivo distinto y restauración bit-idéntica. La guarda del test
verifica IDENTIDAD por item (§4). Rama: `perf/recall-envoltorio-tier2` (sobre `main`).
Fecha: 2026-10-26, re-verificado 2026-11-02. Continúa `specs/recall-envoltorio/proposal.md` (Tier 1, en `main` vía #765).

## 1. Por qué (pilar y medición)

Pilar de ahorro de tokens (decisiones del dueño `c07b47cd` 2026-10-14 / `4e23c916` 2026-09-26):
el éxito se mide en tokens del AGENTE por unidad de trabajo, con **≤10 % de potencia sacrificada** y
**cero riesgo de perder info**.

La Fase 0 de medición (`003f0749`, 2026-10-24) ubicó el gasto: Bash+Read = 89 % del total y son
builtins de Claude Code (fuera del alcance de Musubi). De lo que SÍ es Musubi, `musubi_recall` es el
mayor (mediana ~4.888 tok/llamada, 472 llamadas / 45 d). Y su **envoltorio es el 72,7 % del payload**;
el gist útil, sólo el 27 %.

Desglose del payload (`1f616b89`, medido sobre la base real): claves JSON repetidas 36 %,
**id UUID 10,9 %**, topic_key 10,4 %, created_at 6,1 %, score 2,9 %, resto.

Tier 1 (ya en `main`) dejó `used_tokens` honesto y `created_at` a resolución de día, pero el
**`id` sigue viajando como UUID completo (36 chars)** en el payload del resultado del tool —
`RecallItem.ID`, `json:"id"` (internal/memory/recall.go:120), poblado en `empaquetar` con
`ID: c.id` (recall.go:481).

## 2. Qué propongo — y qué NO

### SÍ · 2a — acortar el `id` a 8 hex en el payload del recall

- **Cambio** (corrección de capa respecto de la propuesta original): un loop en el LÍMITE del tool
  (`toolRecall`, internal/mcp/methods.go), después de rerank y `markStaleOrigins` y antes del marshal:
  `for i := range res.Items { res.Items[i].ID = memory.IDCorto(res.Items[i].ID) }`.
  `IDCorto` ya existe en `main` (internal/memory/idcorto.go, de la palanca #1): un UUID canónico
  baja a sus primeros 8 hex; cualquier otra forma (`commit-…`, ids ya cortos) viaja verbatim.
  **NO en `empaquetar` (recall.go:481)**: ésa es la capa del API interno `Recall()`, y adentro de
  `Recall` el UUID entero lo necesitan `markStaleOrigins` (origins.go — marca ranciedad por id
  completo), la hidratación del grounding (`HydrateForGroundingCtx` vía `toolAsk`) y
  `recalleval/harness.go`. Acortar ahí rompió 5 tests en 2 paquetes (Q3/Q4 en memory, G1–G3 en mcp);
  acortar en el límite sólo toca el cable que ve el agente y deja el contrato interno intacto.
- **Ahorro**: el id baja de 36 a 8 chars (−78 % de ese campo) ≈ **−8,5 % del payload del recall**
  (~415 tok/llamada sobre la mediana; ~200k tok / 45 d en esta máquina), amplificado por la
  re-lectura del contexto turno a turno (el 68 % «releer contexto»). Es ganancia PURA.
- **Cero pérdida de info** (verificado en el código, no asumido):
  - `memory_expand → hydrateByIDs → esRefCorta` (expand.go:116,129,246) ya resuelve una ref corta
    de 8 hex, no sólo el id exacto. El agente expande igual.
  - Ante un prefijo AMBIGUO (expand.go:248-256) entran **TODAS** las que colisionan (superconjunto);
    el presupuesto no parte el grupo. Nunca se pierde la que el agente pidió, y `used_tokens` sigue
    siendo honesto (sube con el costo real).
  - **0 colisiones de prefijo de 8 hex en 2.105 UUIDs** del acervo local (medido 2026-10-26,
    scratchpad/colision.py). El caso ambiguo no ocurre hoy; el diseño lo cubre si algún día ocurre.
  - ⚠️ **ALCANCE de esta verificación: SÓLO la vía `memory_expand`.** El `id` del recall es de doble
    propósito: (a) handle opaco para `memory_expand` —que resuelve cortos— y (b) **PK exacta** que
    tools vecinas buscan directo. 2a acorta el id tratándolo como (a). La revisión adversarial
    (2026-10-26) encontró que la clase (b) NO fue enseñada a resolver cortos y que un consumidor (b)
    está VIVO en el flujo del subagente `musubi-tareas`. Es un hueco real de «cero pérdida», detallado
    y CONFIRMADO en código en §3 (Hallazgo A). Mientras A no se resuelva, «cero pérdida» NO está
    probado fuera de `memory_expand`.
- **Cero riesgo de potencia**: el `id` es un handle opaco para expandir, no contenido que el modelo
  razone. El gist, topic_key, edad y score quedan intactos. El modelo no pierde una sola señal.
- **Simetría / coherencia**: es EXACTAMENTE lo que la palanca #1 ya hace en el envoltorio del hook por
  turno (#770, desplegado en 0.145.0). 2a lo extiende al payload del tool, cerrando el otro sitio por
  donde todavía viaja el UUID entero.

### NO · 2b — compactar las claves JSON repetidas (36 %, el bocado más grande)

Matar las claves repetidas exige un formato tabular/posicional o claves de 1 char. Eso rompe a los
consumidores del JSON y **degrada la legibilidad para el modelo** (riesgo a potencia > 10 %) y la
garantía de «cero pérdida». Va contra el pilar. **NO-GO**, salvo que el dueño lo pida con otro criterio.

### NO · 2c — podar `topic_key` (10,4 %)

El agente lo usa como señal de tema/contexto. Podarlo pierde info útil. **NO.**

## 3. Riesgos abiertos — decisión del dueño

> **RESUELTO (2026-11-02):** el dueño eligió la opción **(i)** del fork de abajo y se implementó en
> **#772** (ya en `main`): los consumidores por-PK resuelven la ref corta de 8 hex y `save_observation`
> con id= la rechaza. El Hallazgo A queda cerrado y el bloqueo del PR, levantado. Lo que sigue en esta
> sección se conserva como registro de la decisión y del análisis que la fundó.

### Hallazgo A (CONFIRMADO en código, revisión adversarial 2026-10-26) — el id del recall también es PK exacta

El `id` que emite el recall tiene DOBLE propósito: handle opaco para `memory_expand` (resuelve cortos)
**y** clave primaria que tools vecinas buscan con igualdad exacta, sin resolver cortos. 2a lo acorta
pensándolo sólo como lo primero. Traza determinista de la regresión viva:

- El subagente `musubi-tareas` (cmd/musubi/subagente_tareas.go:109) tiene esta instrucción: «si otra
  nota —de la unidad o visible— cuenta cómo es ahora, pasá su id como `superseded_by`». Una nota
  **visible** se obtiene con `musubi_recall` → con 2a su `id` ya viaja **corto (8 hex)**.
- `toolDiscardProposal` (internal/mcp/methods_observation_quarantine.go) pasa ese `superseded_by` →
  `DescartarPropuestaCtx` → `DescartarPropuesta` (internal/memory/tareas_del_sistema.go:92), que valida
  `reemplazadaPor` con `SELECT COUNT(*) FROM observations WHERE id = ?` — **igualdad exacta**.
- La tabla está indexada por el UUID completo; `WHERE id = <8 hex>` **nunca** matchea. El scope
  (`obsInScope`, scope.go:256) NO frena antes: con un id inexistente da `sql.ErrNoRows` y devuelve
  `true` (lo deja pasar). O sea que la compuerta que dispara es el COUNT → **`ErrObservationNotFound`**,
  que el subagente recibe como `codeInvalidParams`.
- **Efecto:** el movimiento documentado del subagente (descartar una propuesta en cuarentena marcándola
  superseded por una nota visible) **falla**; la propuesta rancia queda viva. Es exactamente el modo de
  degradación que el proyecto llama «la memoria empieza a fallar». Viola el pilar **«cero riesgo de
  perder info»** fuera de la vía `memory_expand`.
- Alcance confirmado estrecho: el `id` de la propuesta en cuarentena sale de `musubi_work` (no de
  recall), así que NO se acorta; sólo rompe la rama `superseded_by`-desde-nota-visible. Otros
  consumidores por-PK del mismo patrón, a verificar antes de un fix: `save_observation id=`
  (operations.go:513, `ON CONFLICT(id) DO UPDATE` → un id corto crearía fila NUEVA, dup silencioso) y
  `promote` (registry.go, hoy dormida).

**Fork para el dueño** (A expande el alcance más allá del loop acotado que aprobaste como «2a ya»):

- **(i) Fase 2 — enseñar a los consumidores por-PK a resolver cortos** (lo que el naming `commit-1c6ed28e`
  ya anticipaba: «fase 1» = sólo `memory_expand`). `DescartarPropuestaCtx`/`PromoteObservationCtx`
  resuelven la ref corta igual que `memory_expand` (`esRefCorta`/`hydrateByIDs`), respetando scope y
  fallando si es ambigua; `save_observation id=` rechaza/avisa ante un id corto. Es el fix principista y
  el único que hace verdadero «cero pérdida», pero **toca código de producción nuevo** (discard/promote/
  save), fuera del loop aprobado. **Recomiendo que 2a NO se mergee sin esto**: si no, 2a introduce la
  regresión de arriba.
- **(ii) Documental** — dejar escrito que el id del recall es ref de 8 hex **sólo** para `memory_expand`,
  y reescribir la línea 109 del prompt del subagente para que NO pase ese id a `superseded_by`. Más
  barato, pero deja el id roto como PK para cualquier otro flujo futuro y cuesta tokens fijos en las
  descripciones de tools.
- **(iii) Gate** — 2a detrás de un flag de config, apagado hasta tener (i). Permite mergear la mecánica
  sin encender la regresión.

Mi recomendación: **(i) como prerequisito de 2a** (misma PR o una previa). La guarda del test
(`TestRecallEmiteIdCortoResolvible`, §4) ya quedó endurecida para IDENTIDAD y no depende de esta
decisión; pero el invariante «cero pérdida» no se sostiene fuera de `memory_expand` hasta (i).

### Hallazgo B (hipotético) — consumidor `musubi-body`

**Consumidor `musubi-body`** (cmd/musubi-body-ui/memoria.go:108, FUERA del repo, snapshot sin git;
vive —si vive— en `davantis-1`, hoy apagada → no verificable ahora). Lee campos del JSON del recall.
Si usa `id` asumiendo el UUID completo (como clave o para expandir), el id corto lo afectaría.

- **Mitigante 1**: `musubi-body` expandiría por el mismo resolvedor, que ya acepta 8 hex.
- **Mitigante 2 (el fuerte)**: la palanca #1 YA hace que el hook emita 8 hex. Si `musubi-body` lee
  ese bloque, ya convive con ids cortos. 2a **alinea** ambos lados (hook y payload en 8 hex); reduce
  una inconsistencia que ya existe, no crea una nueva.
- Es el mismo tipo de riesgo que el `score` (decisión `0b5e2437`): lo resuelve el dueño. Opciones:
  (i) aplicar 2a (y que `musubi-body` use el resolvedor corto, que ya existe);
  (ii) gate detrás de un flag de config hasta poder verificar `musubi-body` en `davantis-1`;
  (iii) esperar a tener `davantis-1` prendida para verificar antes de tocar nada.

## 4. Guardas (sabotaje-verificables, como Tier 1)

- Test nuevo (`TestRecallEmiteIdCortoResolvible`): cada `RecallItem.ID` del payload es una ref corta
  de 8 hex (a), y ese id corto RESUELVE por `musubi_memory_expand` y trae **SU** observación —no otra—
  (b). Son las dos mitades del invariante («ocurrió el acortado» + «cero pérdida»). La mitad (b) recorre
  TODOS los items y verifica IDENTIDAD por item: la siembra distingue cada nota con un marcador único
  «caso X» (`sembrar`, motor_sin_candado_test.go), y la guarda expande cada id corto y exige ese mismo
  marcador en el contenido. Una primera versión de (b) miraba sólo `Items[0]` y exigía una frase
  COMPARTIDA por todas las notas, y eso dejaba pasar dos trampas que la revisión adversarial encontró
  (ver abajo); el chequeo por item las cierra.
- **Sabotajes** (los cuatro verificados 2026-10-26 con deploy/pruebas/sabotaje.sh, cada uno ROJO por la
  mitad esperada y con un motivo DISTINTO —sin colisión de mensajes—):
  - Primario, borrar el loop de acortado en `toolRecall` ⇒ el UUID entero viaja ⇒ ROJO en (a),
    `envoltorio_recall_test.go:140` «el UUID entero viajó al agente».
  - S2, los ÚLTIMOS 8 hex en vez de los primeros (8 hex válidos que NO resuelven) ⇒ ROJO en (b):144,
    `expand=[]`.
  - S4, acortar bien sólo `Items[0]` y mal el resto ⇒ ROJO en (b) **en el item 1** — la trampa que la
    (b) vieja (sólo `Items[0]`) dejaba verde.
  - S6, darle a cada item la ref corta del VECINO (resuelve, pero a OTRA observación) ⇒ ROJO en (b):
    item 0 resolvió a «caso b», se esperaba «caso a» — la trampa que una (b) con frase compartida
    dejaba verde. Es la prueba de que (b) chequea IDENTIDAD, no mera resolubilidad.
  - Todos compilan y vetean limpio (no son rojos de build).
- Test: `used_tokens` sigue contando el payload real (con id corto) — el invariante de Tier 1 no se
  afloja.
- A/B del payload con `-overlay` HEAD vs árbol, byte-idéntico salvo el id (igual que en Tier 1).

## 5. Magnitud honesta

Esto es **pulido incremental limpio, no un salto grande**: `recall` es ~4,6 % de los tokens de
Musubi, que a su vez es una fracción del total dominado por Bash+Read (fuera de alcance, ya dictaminado
en Fase 0). El valor de 2a es que es **ahorro real sin ningún riesgo de potencia ni de info**, y suma
con la palanca #1. Si lo que se busca es un salto grande, el lever no está en Musubi.

## 6. Plan de implementación (SÓLO tras el OK del dueño)

1. `toolRecall` (internal/mcp/methods.go): loop `res.Items[i].ID = memory.IDCorto(...)` tras
   `markStaleOrigins`, antes del marshal. **NO en `empaquetar`** (ver §2a: rompe consumidores
   internos de `Recall()`).
2. Test de guarda (sección 4) en internal/mcp/envoltorio_recall_test.go (o nuevo).
3. `go test` de los paquetes tocados + `go vet`; A/B del payload.
4. Revisión adversarial (skill adversarial-review) — toca el formato del cable.
5. PR desde `perf/recall-envoltorio-tier2` a `main`; corte de release y deploy aparte, con el método
   de siempre (regla de oro: cada paso hacia afuera, con tu OK en el momento).
