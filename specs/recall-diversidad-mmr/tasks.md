# Tasks — recall-diversidad-mmr

## Implementación

- [ ] **T1** — `internal/memory/mmr.go`: `redundancy(cos)` (0 en `redundancyBase=0.60`, 1 en 1.0) y `normalize(scores)` (min-max a `[0,1]`).
- [ ] **T2** — `mmr.go`: `vectorsFor(ids) map[string][]float32` — una sola consulta, filtrada por `model_id` (contrato de procedencia).
- [ ] **T3** — `mmr.go`: `diversify(scored, lambda)` — `lambda >= 1` ⇒ devuelve la entrada **sin tocarla ni consultar la base**.
- [ ] **T4** — `recall.go`: cablear `diversify` **entre** `scoreCandidates` y `packByBudget`.
- [ ] **T5** — `RecallOptions.MMRLambda` + `config.Memory.MMRLambda`, con el criterio ausente-vs-explícito (un `1` explícito debe respetarse).

## Tests

- [ ] **T6** — S.a: el clon (coseno 0.98) **cede su lugar** al item que aporta información nueva.
- [ ] **T7** — S.b: los 3 items **siguen presentes**. MMR reordena, **no descarta**.
- [ ] **T8** — S.c: `λ = 1` ⇒ orden **bit-idéntico** al RRF, item por item.
- [ ] **T9** — S.d: item **sin vector** ⇒ penalización 0, conserva su lugar por relevancia.
- [ ] **T10** — S.e: dos items con coseno **0.60** (la línea de base) ⇒ redundancia **0**.
- [ ] **T11** — S.f: el **primero** elegido es siempre el de mayor relevancia.

## Medición (no es opcional: es la vara)

- [ ] **T12** — Correr el **`recall-gate`** (R@10 sobre el fixture dorado, tabla POTION real) con **varios λ** y **anotar el número de cada uno**.
- [ ] **T13** — Fijar el default de λ **con esa medición**. Si **ningún** λ útil mantiene R@10 ≥ 0.80 ⇒ **ABANDONAR la feature y decirlo.** No bajar la vara.

## Cierre

- [ ] **T14** — `go test ./...` + `golangci-lint`.
- [ ] **T15** — Verificación adversarial (apagar MMR ⇒ los tests que lo prueban fallan).
- [ ] **T16** — Re-medir el recall REAL de la memoria (la consulta que motivó todo) y **mostrar el antes/después**.
- [ ] **T17** — CHANGELOG (`Added` ⇒ minor) + PR.

## Orden

T1→T3 (el motor), T4-T5 (cableado), T6-T11 (tests), **T12-T13 (la medición que decide si esto vive o muere)**, T14-T17.

**T13 es la tarea más importante del plan.** Todo lo demás es construir la feature; **T13 es la que decide si la feature merece existir.** La tentación va a ser tocar la vara para que pase; el compromiso, escrito acá de antemano, es **no hacerlo**.
