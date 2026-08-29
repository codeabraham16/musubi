# SDD tasks — renaissance-f3-abstencion

- [x] **T1 · las constantes.** `designSimilitudMinima` (0,48, calibrado), `designEmbedTimeout` (5 s),
      y los nombres de modo y causa.
- [x] **T2 · `resultadoRecall`.** El recall devuelve material + modo + causa, en vez de un bool que
      no distinguía «no hay nada» de «hay y es malo».
- [x] **T3 · el piso.** `sobreElPiso`, aplicado sólo por el camino semántico (I-ABS1, I-ABS4).
- [x] **T4 · el brief lo declara.** Campos `retrieval` y `degraded_reason` (I-ABS2).
- [x] **T5 · «no hay» ≠ «no pude».** Causa `sin_recuperador` cuando FTS falla.
- [x] **T6 · el timeout.** 30 s → 5 s.
- [x] **T7 · el embebedor determinista** y los cuatro invariantes con sus sabotajes; el piso se
      ejercita en las DOS direcciones para que no pase por casualidad.
- [x] **T8 · la sonda mide el riesgo.** Falsa abstención en pedidos legítimos + conteo de causas.
- [x] **T9 · entrega.** `go vet` (con y sin tag) + `go build ./...` + `go test ./...` verdes,
      CHANGELOG, bump, PR.
