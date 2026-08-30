# SDD tasks — renaissance-f5-reproducibilidad

- [x] **T1 · orden estable.** `estabilizarOrden` + `cuantizarSim` con `designResolucionSim` (I-REP2).
- [x] **T2 · normalizar la consulta.** Primero por caracteres — y el test demostró que NO alcanza.
- [x] **T3 · corregirlo: cortar por oraciones.** `primerasOraciones` + `designConsultaFrases`, con el
      tope de caracteres como segunda red (I-REP1).
- [x] **T4 · declarar el recorte.** `query_normalized` con original y usado (I-REP3, I-REP4).
- [x] **T5 · el eco deja de comerse el corpus.** `ask` lleva la consulta normalizada — lo encontró el
      propio test de ruido.
- [x] **T6 · invariantes.** I-REP1..4, cada uno con su sabotaje comparado contra la función real.
- [x] **T7 · entrega.** lint + vet + build + `go test ./...` verdes, CHANGELOG, bump, PR.
