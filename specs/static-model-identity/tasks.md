# Tasks — static-model-identity

## N1 — checksum de contenido
- [ ] T1. `staticTableChecksum(tableRaw, tokRaw []byte) string` — sha256 digest-de-digests, 12 hex (R2, R3).
- [ ] T2. `loadStaticTable` devuelve también el digest de los bytes que ya leyó (sin I/O extra, R5).
- [ ] T3. Leer `tokenizer.json` una sola vez en `NewStaticProvider`; pasar sus bytes al checksum y al `loadTokenizer` (sin doble `ReadFile`).
- [ ] T4. `modelID = "static:" + basename + "@" + checksum` (R1, R6).
- [ ] T5. Tests: N1.a (recarga ⇒ mismo id), N1.b (cambia safetensors ⇒ id distinto), N1.c (cambia tokenizer ⇒ id distinto).

## M3 — auto-backfill
- [ ] T6. `countStaleEmbeddings()` — `COUNT(*)` con el MISMO predicado que el `SELECT` de `EmbedBackfill` (una sola fuente de verdad).
- [ ] T7. `AutoEmbedBackfill(embed)` — no-op si `vectorModelID==""` / `embed==nil` / 0 pendientes (R7, R11); si >0: log inicio + `spawnBackground(EmbedBackfill)` + log fin (R8, R10).
- [ ] T8. Wiring en los 2 call-sites de `cmd/musubi/main.go` (junto a `SetVectorModelID`/`WarnOnEmbedModelSwitch`).
- [ ] T9. Tests: M3.a (re-embebe el hueco), M3.b (no-op sin hueco), M3.c (engine cerrado ⇒ no lanza).

## Cierre
- [ ] T10. `go build` + `go vet` + `go test ./...` verde.
- [ ] T11. CHANGELOG (`Unreleased` → `Fixed`): N1 + M3, con la nota de migración one-time auto-sanada.
