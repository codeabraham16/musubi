# Tasks — gate-novedad-captura

## Motor
- [ ] T1. `ConflictOptions.DetectOnly bool` (default false = comportamiento de hoy, R2).
- [ ] T2. En `DetectRelations`: si `DetectOnly`, forzar `rel = pendingRel(...)` tras `decideRelation` (R3, R4). El `markSuperseded` queda inalcanzable por construcción (R0).

## Cableado
- [ ] T3. C3 — `captureCommits(store, git, embed, detect)`: llamar `detect(id)` sólo si `!deduped` (R6).
- [ ] T4. C3 — `runCapture` arma el `detect` desde `cfg.Conflicts` con `DetectOnly: true`, tragando errores (R8, R9).
- [ ] T5. C4 — `methods.go:1588`: `DetectRelations` con `DetectOnly: true`, best-effort (R7, R8).

## Tests
- [ ] T6. M4.b / R0 — dos commits de mensaje parecido, mismo `topic_key`, coseno alto: con `DetectOnly` NINGUNO queda superseded y la relación es `pending`. **Y el gemelo:** sin `DetectOnly` ese mismo caso SÍ auto-supersede (demuestra que el flag evita un peligro REAL, no hipotético).
- [ ] T7. M4.c / R1 — el duplicado semántico SE GUARDA igual (no se descarta).
- [ ] T8. M4.a — el duplicado capturado queda con relación `pending` (hoy: sin ninguna marca).
- [ ] T9. M4.d / R2 — `DetectOnly: false` ⇒ el camino explícito auto-resuelve como siempre.
- [ ] T10. R6 — un commit dedupeado por hash exacto NO dispara `detect`.

## Cierre
- [ ] T11. MEDIR el costo de la detección por commit capturado (no asumirlo).
- [ ] T12. `go build` + `go vet` + `go test ./...` verde.
- [ ] T13. CHANGELOG.
