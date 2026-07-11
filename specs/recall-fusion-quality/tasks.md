# Tasks — recall-fusion-quality

## Q1 — Piso de coseno
- [ ] T1. `RecallOptions.VectorFloor float64` (recall.go) + doc.
- [ ] T2. `augmentWithVectorPool` recibe `floor float64`; saltea `SearchResult` con `Similarity < floor` (guarda `floor > 0`). Actualizar el call-site en `Recall` para pasar `opts.VectorFloor`.
- [ ] T3. Config: `MemoryConfig.VectorFloor float64` (yaml `vector_floor`), default `0.30` en `config.Default()`; sanitize/merge si aplica.
- [ ] T4. Wiring MCP: la capa que arma `RecallOptions` pasa `cfg.Memory.VectorFloor` a `VectorFloor` (junto a Stemming/Cooccurrence).
- [ ] T5. Tests: Q1.a (piso 0.40 filtra 0.28/0.11) y Q1.b (piso 0 = histórico) sobre `augmentWithVectorPool` con un stub de vectores.

## Q2 — Degradación elegante
- [ ] T6. Helper `isFTSCorruption(err) bool` (match "corrupt"/"malformed"/"database disk image").
- [ ] T7. En `Recall`: tratar el error de `recallCandidates` — degradar (log + cands/lexRank nil) si es corrupción, propagar si no.
- [ ] T8. Tests: Q2.a (FTS corrupto + QueryVector → items, sin error) y Q2.b (error no-corrupción → propaga).

## Cierre
- [ ] T9. `go build ./...` + `go test ./...` verde; regenerar golden si el recall híbrido se movió (`-update`).
- [ ] T10. `gofmt` (cuidado con el quirk de comillas curvas go1.26); lint 0.
