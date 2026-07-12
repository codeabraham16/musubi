# Tasks — dedup-semantico-coseno

## Config
- [ ] T1. `ConflictConfig.CosineFloor` (`cosine_floor`, default 0.85) + `CosineAutoThreshold` (`cosine_auto_threshold`, default 0.90); patrón ausente-vs-explícito en `applyConflictsDefaults`.
- [ ] T2. `ConflictOptions.CosineFloor` / `.CosineAutoThreshold` + `withDefaults`; wiring MCP.

## M2 — pool
- [ ] T3. `srcVector(obsID)` — trae el vector de la observación con la procedencia ACTUAL; ausente ⇒ nil (sin error).
- [ ] T4. `conflictCandidates` devuelve la UNIÓN: FTS (como hoy) + vecinos por coseno de `SearchObservations(srcVec, pool)`, dedup por id. Sin `srcVec` ⇒ pool léxico exacto (R2).

## M1/Q4 — veredicto
- [ ] T5. `candidateCosines(srcVec, ids)` — una query trae los vectores del pool (`model_id` actual) y computa `CosineSimilarity`; devuelve `map[id]float64` (ausente = sin vector).
- [ ] T6. `decideRelation` toma `cos *float64` e implementa la tabla de verdad: auto-resolve sólo con `lex>=auto` **Y** `cos>=cosAuto`; `lex>=auto` con `cos` bajo ⇒ **degrada a pending** (R6); `cos>=cosFloor` con `lex<floor` ⇒ **pending** (R5); ambos bajo el piso ⇒ ignorar; `cos==nil` ⇒ histórico (R7).
- [ ] T7. `DetectRelations` usa el filtro nuevo (hoy `sim < floor ⇒ continue` mataría el caso R5).

## Tests
- [ ] T8. D.a — duplicado semántico (lex bajo, cos alto) ⇒ `pending` (hoy: nada).
- [ ] T9. D.b — lex 0.9 + cos 0.5 ⇒ `pending`, NO supersedes (AND-gate degrada).
- [ ] T10. D.c — lex 0.9 + cos 0.99 + mismo topic + más nueva ⇒ auto `supersedes` (sobrevive).
- [ ] T11. D.d/R7 — sin vectores ⇒ resultado idéntico al histórico.
- [ ] T12. D.e/R0 — property test: sobre una grilla de (lex, cos), el gate nuevo NUNCA produce un `supersedes/resolved` que el léxico-puro no produjera.
- [ ] T13. R9 — `cosine_floor: 0` ⇒ comportamiento histórico.

## Cierre
- [ ] T14. `go build` + `go vet` + `go test ./...` verde.
- [ ] T15. CHANGELOG: el falso negativo que se cierra + el invariante R0 + los umbrales medidos.
