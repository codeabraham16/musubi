# SDD tasks — renaissance-f1f2-precedencia-y-material

- [x] **T1 · el contrato.** Consts `designPrecedence` (lex specialis) y `designMaterialNote`.
- [x] **T2 · la frontera.** `principles` pasa a ser el núcleo estático; `designMethodCards()` sirve el
      acervo como `method[]` con `topic` + `fuente`; `sanearMaterial` limpia sólo caracteres de control.
- [x] **T3 · el orden.** Reordenar la struct del brief (I-PRE1).
- [x] **T4 · el presupuesto.** `aplicarPresupuesto` + `cederUnItem` + `declararRecorte`, con tope por
      tarjeta y aviso ruidoso al recortar la marca (I-PRE2, I-PRE3).
- [x] **T5 · el emit.** Sacar la marca de Musubi de `designEmitWeb` y `designEmitPainter` (I-PRE4).
- [x] **T6 · el banco aprende los campos nuevos.** `bloquesDe` y `bloquesDeInstruccion`.
- [x] **T7 · invariantes.** I-PRE1, I-PRE2, I-PRE3, I-PRE4, I-MAT1 con sus tests.
- [x] **T8 · el banco de ataque cambia de bando.** A1 y A3 pasan de afirmar la vulnerabilidad a
      defender el arreglo — que es para lo que se escribieron.
- [x] **T9 · apretar umbrales.** M4 p50 6.600→2.500, M4 max 7.500→2.600, M5 0,04→0,13,
      M6 acervo 0,00→1,00.
- [x] **T10 · entrega.** `go vet` + `go build ./...` + `go test ./...` verdes, CHANGELOG, bump, PR.
