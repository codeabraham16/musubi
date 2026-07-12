# Tasks — recall-importance-rrf

## Implementación
- [ ] T1. Helper `effectiveImportance(c candidate) float64` — normaliza `<=0` a `1.0` (R3).
- [ ] T2. Helper `importanceRank(cands) map[string]int` — dense rank por importancia efectiva desc (R2).
- [ ] T3. En `scoreCandidates`: precalcular `impRank := importanceRank(cands)`; sumar `1/(rrfK+impRank[c.id])` al `rrf`; quitar el bloque `imp := c.importance / imp<=0 / rrf*imp` y dejar `score: rrf` (R1). Otros términos intactos (R7).

## Tests
- [ ] T4. `TestImportanceRankDense` — `[10,5,1,1]`→`[0,1,2,2]`; incluir un `importance:0`→normaliza a 1.0 y empata (Q3.a, R2, R3).
- [ ] T5. Reescribir `TestScoreCandidatesFusion` → `TestScoreCandidatesImportanceNoOverride` — mejor relevancia (lex/vec) gana pese a menor importancia (Q3.c, R6).
- [ ] T6. `TestScoreCandidatesImportanceTiebreak` — idénticos en todos los pools salvo importancia ⇒ mayor importancia primero (Q3.b, R5).
- [ ] T7. `TestScoreCandidatesImportanceUniform` — importancia uniforme ⇒ orden por lexRank, sin alteración (Q3.d, R4).
- [ ] T8. Verificar que `TestRecallImportanceBoost` sigue pasando (desempate a igual relevancia).

## Cierre
- [ ] T9. `go build ./...` + `go test ./internal/memory/... ./internal/recalleval/... ./internal/mcp/...` verde.
- [ ] T10. `go test ./...` full verde; `gofmt -l` limpio (quirk comillas curvas go1.26).
- [ ] T11. CHANGELOG `## [Unreleased] → ### Fixed`: entrada Q3.
