# Tasks — conflictos-artefactos-mismo-cambio

## Implementación

- [ ] **T1** — `internal/memory`: declarar `const CommitTopicKey = "git-commit"` (D4).
- [ ] **T2** — `cmd/musubi/capture.go`: usar `memory.CommitTopicKey` en vez del literal.
- [ ] **T3** — `internal/memory/conflicts.go`: `sddChange(topicKey) string` — parsea `sdd/<cambio>/<fase>`, exige la barra de la fase, devuelve `""` si no matchea (R3).
- [ ] **T4** — `internal/memory/conflicts.go`: `complementaryPair(a, b obsRow) bool` — G1 (mismo cambio SDD) + G2 (commit ↔ sdd, **simétrico**, D3).
- [ ] **T5** — `DetectRelations`: `if complementaryPair(src, c) { continue }` en el **tope** del loop, antes de `Similarity` (D1).

## Tests (cada escenario de la spec)

- [ ] **T6** — S.a: dos fases SDD del **mismo** cambio, contenido casi idéntico ⇒ **0 relaciones**.
- [ ] **T7** — S.b: `sdd/cambio-a/design` vs `sdd/cambio-b/design` parecidos ⇒ **sí** hay relación (la guarda NO los tapa).
- [ ] **T8** — S.c: `git-commit` vs `sdd/<cambio>/proposal` parecidos ⇒ **0 relaciones**, en **ambos órdenes** de guardado (pinea D3).
- [ ] **T9** — S.d: dos `git-commit` parecidos ⇒ **sí** hay relación.
- [ ] **T10** — S.e: `git-commit` vs una memoria común parecida ⇒ **sí** hay relación.
- [ ] **T11** — S.f: en el escenario S.a, **ninguna** de las dos observaciones queda `superseded`/`archived` — siguen visibles en el recall.
- [ ] **T12** — unit de `sddChange`: `sdd/x/spec`→`x`; `sdd/x`→`""`; `sdd/`→`""`; `git-commit`→`""`; `notas/x/y`→`""`.

## Cierre

- [ ] **T13** — `go test ./...` verde + `golangci-lint run` limpio.
- [ ] **T14** — CHANGELOG (`Fixed`).
- [ ] **T15** — PR.
- [ ] **T16** — (fuera del código) resolver a mano como `related` las 14 pendientes ya existentes; juzgar las 9 genuinas.

## Orden

T1→T2 (constante primero, para que T4 la pueda usar). T3→T4→T5 (la guarda). T6-T12 en paralelo. T13→T16.

**T7, T9 y T10 son los tests que más importan:** el modo de fallo peligroso acá es el **falso negativo silencioso** (una guarda demasiado ancha que apaga detección real sin romper nada). Pinean lo que **NO** se debe tapar.
