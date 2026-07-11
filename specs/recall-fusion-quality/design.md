# Design — recall-fusion-quality

## Q1 — Piso de coseno

**Decisión:** agregar `VectorFloor float64` a `RecallOptions`; `augmentWithVectorPool` recibe el piso y saltea los `SearchResult` con `Similarity < floor` antes de poblar `vecRank`/`missing`.

- **Firma:** `augmentWithVectorPool(ctx, cands, queryVec, limit, floor float64)`.
- **Filtrado:** en el loop sobre `results`, `if float64(r.Similarity) < floor { continue }`. Como `results` viene ordenado por `Similarity` desc (operations.go:359), el `continue` de los de baja sim no altera el rango relativo de los que sobreviven (R5) — `vecRank` sigue asignando 0,1,2… a los supervivientes en orden.
- **floor <= 0** ⇒ la guarda no filtra nada ⇒ histórico bit-a-bit (R3).
- **Config:** nuevo campo `VectorFloor float64` en `config.MemoryConfig` (yaml `vector_floor`), default **0.30** (conservador: descarta la cola claramente irrelevante <0.30 sin tocar la banda relevante 0.40-0.50 medida; la calibración fina es Q5, otro cambio). La capa MCP lo pasa a `RecallOptions.VectorFloor` como ya hace con Stemming/Cooccurrence/GraphCentrality.
- **Rationale del default 0.30, no 0:** el defecto medido es inyectar 50 vecinos con peso pleno; un piso conservador ON entrega valor real ya. `vector_floor: 0` en config revierte.

## Q2 — Degradación elegante

**Decisión:** un helper `isFTSCorruption(err) bool` (match model-free sobre el mensaje: contiene "corrupt", "malformed" o "database disk image") + tratamiento en `Recall`.

- En `Recall`, tras `recallCandidates`: si `err != nil`, `if isFTSCorruption(err) { logx.Warn(...); cands, lexRank = nil, nil } else { return RecallResult{}, err }`.
- Con `cands=nil` y `lexRank=nil`, el flujo sigue: `augmentWithVectorPool` (si hay QueryVector) suma el pool vectorial; si no, `len(cands)==0` ⇒ retorna vacío sin error (R9). El fallback por recencia NO aplica acá (ya se intentó FTS); es aceptable: el objetivo es no ABORTAR, y el pool vectorial cubre el caso semántico (R8).
- **Telemetría:** `logx.Warn("recall: FTS corrupto, degradando a pool no-léxico", "error", err)`. Señal visible, no silenciosa (mitiga el riesgo de enmascarar).

## Impacto en tests / golden

- Q1 con default 0.30 cambia el ranking del recall HÍBRIDO ⇒ regenerar golden (`TestToolsListGolden` NO se afecta; los golden de recall/token-audit sí pueden moverse). El recall léxico puro queda intacto (R4).
- Tests nuevos: piso alto/bajo (Q1.a/b) sobre `augmentWithVectorPool`; degradación (Q2.a) con un FTS inducido a error + un QueryVector; propagación (Q2.b) con un error no-corrupción.

## Alternativas descartadas

- **Reusar `defaultSimilarityFloor=0.3` de conflicts.go:** NO — ese piso es sobre `Similarity` (Jaccard de trigramas, escala distinta), no coseno. Mezclarlos sería un bug de escala.
- **Fallback por recencia tras degradar:** descartado por ahora — agrega ruido no pedido; el objetivo mínimo es no abortar. Se puede sumar en un cambio posterior si se mide necesario.
