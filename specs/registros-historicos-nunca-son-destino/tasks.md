# Tasks — registros-historicos-nunca-son-destino

## T1 — La guarda (F1)

- [x] **T1.1** — `complementaryPair` colapsa a `return historicalRecord(b.topicKey)`; el parámetro `a` pasa a `_`. (R1, R2)
- [x] **T1.2** — Se elimina `sameKind` (queda sin usos). (D1)
- [x] **T1.3** — Se reescribe el comentario del bloque: **una** regla, con G1/G2 documentadas como **subsumidas**, no como borradas. (D1)

## T2 — Tests

Los que **quedan intactos y verdes** (prueban el colapso, y son la red anti-martillo):

- [x] **T2.1** — `TestGuardaHermanosDelMismoCambioSDD` (G1) y `TestGuardaCommitVsSuPropioSDD` (G2): siguen verdes **sin una línea de cambio**. Es la evidencia de que G1/G2 quedaron **subsumidas**, no rotas. (R4, S.c, S.d)
- [x] **T2.2** — `TestUnCommitSiPuedeVolverObsoletaUnaNota` y `TestGuardaNoTapaCommitVsNota`: **destino = nota** ⇒ la relación nace. La asimetría se conserva y la guarda no es un martillo. (R2, S.e)
- [x] **T2.3** — `TestBandaRespetaLasGuardasEstructurales`: la banda usa la misma guarda. (R3, S.g)

Los que **se reescriben** — pineaban la excepción `sameKind`, o sea el contrato que este PR cambia con evidencia:

- [x] **T2.4** — `TestGuardaNoTapaCommitVsCommit` → **`TestCommitNoEsDestinoDeOtroCommit`**: 0 relaciones. (S.a)
- [x] **T2.5** — `TestGuardaNoTapaCambiosSDDDistintos` → **`TestContratoNoEsDestinoDeOtroContrato`**: 0 relaciones. (S.b)
- [x] **T2.6** — `TestCommitVsCommitSeSigueDetectando` y `TestSDDVsSDDDeCambiosDistintosSeSigueDetectando` eran **duplicados exactos** de los dos anteriores ⇒ se reemplazan por **`TestSoloLasCreenciasSeReemplazan`**, una tabla que pinea la **matriz entera** (source × target → ¿hay relación?). Una regla, un test. (R0)

`DetectOnly` (M4) — sus tests cubrían un camino que la guarda ahora bloquea río arriba:

- [x] **T2.7** — Los tests de M4 se re-apuntan del balde `git-commit` al balde **`error-fix`**, donde `DetectOnly` sigue siendo **lo único** que impide el auto-supersede. Un test que cubre un camino ya bloqueado quedaría verde para siempre **sin custodiar nada**. (D5)
- [x] **T2.8** — El gemelo `...WouldAutoSupersede` tenía un `t.Skip` que lo dejaba **pasar en verde sin probar su premisa**: pasa a `t.Fatal`. (D5)

## T3 — La medición (R8)

- [x] **T3.1** — Correr las guardas de HOY vs las de la PROPUESTA sobre las 169 relaciones reales.
- [x] **T3.2** — Verificar que los veredictos sustantivos rotos son **0**. Si no, **la propuesta se cae**.
- [x] **T3.3** — Reportar el número en el PR, no una intuición.

## T4 — Cierre

- [x] **T4.1** — `go test ./...` verde.
- [x] **T4.2** — `golangci-lint run` sin issues.
- [x] **T4.3** — CHANGELOG.
