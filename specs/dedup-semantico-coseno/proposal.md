# Proposal — dedup-semantico-coseno

## Intención

Cerrar **M2** + **M1/Q4** del track Semantic Hardening: que la detección de relaciones/duplicados (`conflicts.go`) **vea y juzgue con semántica**, sin ganar el poder de auto-suprimir de más.

Hoy `DetectRelations` es **100% léxico** y falla en las dos direcciones:

- **Falso negativo (M2 — el pool es ciego).** `conflictCandidates` busca candidatas **sólo por FTS**. Un duplicado escrito con **otras palabras** (sinónimos) nunca entra al pool ⇒ **nunca se detecta**. Es invisible, no "mal juzgado".
- **Falso negativo (M1/Q4 — el veredicto es léxico).** El veredicto usa sólo `Similarity` (Jaccard de trigramas). Un duplicado semántico da Jaccard **bajo** ⇒ cae bajo el piso ⇒ **se ignora**.

## La restricción de seguridad que manda sobre el diseño

**Los embeddings estáticos NO evalúan predicados.** *"Usamos NordVPN"* y *"ya NO usamos NordVPN"* tienen coseno **alto**: el coseno mide *de qué se habla*, no *qué se afirma*. Por eso el coseno **NUNCA** puede auto-ocultar memoria (`markSuperseded`).

De ahí el **AND-gate** (no un OR):

- **Auto-resolver** (supersede/related) exige **léxico ALTO Y coseno ALTO** — el coseno sólo **corrobora**, nunca decide solo.
- Si sólo **una** señal es alta (p. ej. duplicado semántico con otras palabras) ⇒ **`pending`**, lo juzga el agente.

**Invariante:** el coseno sólo **agrega condiciones** al auto-resolve ⇒ el conjunto de auto-supresiones es un **subconjunto** del de hoy. **El coseno no puede auto-suprimir nada nuevo**; sólo puede volver *visible* (pending) lo que hoy es invisible, o volver *más estricta* una auto-resolución existente. Es exactamente la regla de oro del track: *model-free amplía/filtra pools y rutea a pending; el juicio abstracto se delega al agente.*

## Umbrales: CALIBRADOS CON DATOS, no estimados

Medido sobre la memoria real (**393 observaciones, 77.028 pares**):

| | coseno |
|---|---|
| **Casi-duplicados** (Jaccard ≥ 0.7) | **0.991** |
| **No relacionados** (Jaccard < 0.3) | p50 **0.601** · p95 0.737 · p99 0.786 · **máx 0.884** |

**El piso del coseno es altísimo:** pares *no relacionados* ya dan **0.60 de mediana** — texto del mismo dominio (todo sobre Musubi) comparte vocabulario, y el mean-pooling lo amplifica. Un umbral ingenuo habría sido un desastre:

| umbral | pares con coseno ≥ umbral y Jaccard < 0.3 |
|---|---|
| 0.75 | **2.661** (ruido puro) |
| 0.80 | 450 |
| **0.85** | **46** ← candidatos plausibles |
| 0.90 | 0 |

⚠️ Ojo: **esta escala NO es la del recall.** Ahí las sims corren 0.40-0.50 porque son *query* vs *documento*; acá es *documento* vs *documento*, y la línea de base es ~0.60. Reusar el `vector_floor: 0.30` del recall habría marcado **todo** como duplicado.

**Defaults elegidos:** `cosine_floor = 0.85` (sobre el máximo del ruido, 0.884... justo en el borde ⇒ ver riesgos) y `cosine_auto_threshold = 0.90` (0 falsos positivos en 76k pares; los duplicados reales están en 0.99, así que las auto-resoluciones de hoy **sobreviven** al AND-gate).

## Alcance

- **M2:** `conflictCandidates` suma un **pool vectorial** (unión con el FTS) ⇒ los duplicados por sinonimia entran al pool.
- **M1/Q4:** el veredicto computa **léxico y coseno**; AND-gate para auto-resolver; una sola señal alta ⇒ `pending`.
- **Sin vectores** (embedder apagado, o falta el vector de la procedencia actual) ⇒ comportamiento **bit-idéntico al histórico** (léxico puro). Es el interruptor de rollback.
- Config: `conflicts.cosine_floor` y `conflicts.cosine_auto_threshold`.

## Fuera de alcance (explícito)

- **Auto-merge / auto-borrado** de duplicados: NO. El dedup semántico **rutea a pending**; fusionar o suprimir lo decide el agente (`musubi_judge`).
- Cambiar el Jaccard léxico, la consolidación (`consolidate.go`) o la retención.
- MMR (M6), pesos tuneables (M7), gate de novedad en la captura (M4).

## Estrategia de rollback

- `cosine_floor: 0` (o embedder apagado) ⇒ el coseno no participa ⇒ comportamiento histórico exacto.
- Sin migración de esquema, sin cambio de datos. El AND-gate sólo puede **reducir** auto-resoluciones, nunca crear supresiones nuevas ⇒ revertir es seguro por construcción.

## Riesgos

- **`cosine_floor = 0.85` está a 0.034 del máximo del ruido observado (0.884).** El margen es fino: puede colar algún `pending` de más. Es el lado seguro del error (un pending de más cuesta la atención del agente; una auto-supresión de más cuesta **memoria perdida**), pero hay que decirlo. Ajustable por config.
- La calibración sale de **UNA** base (este repo, 393 obs, dominio homogéneo). En un corpus más diverso la línea de base del coseno bajaría y `0.85` sería **conservador** (menos detecciones), no peligroso.
- Muestra chica de casi-duplicados (n=2 con Jaccard ≥ 0.7): el `0.991` es sólido en dirección pero no en precisión. No es crítico: el AND-gate sólo usa ese valor para **no romper** auto-resoluciones existentes, y 0.99 ≫ 0.90.
- El pool vectorial agrega candidatas ⇒ más pares evaluados por save (costo acotado por `candidate_pool`).
