# SDD tasks — renaissance-f4-seleccion

- [x] **T1 · una búsqueda, dos salidas.** `particionarPorPrefijo`; el recall devuelve también el método.
- [x] **T2 · el pool mira más.** `designPoolMax` = 300, por encima de `maxLimit` a propósito.
- [x] **T3 · el método sigue al pedido.** `designMethodCards(relevantes, modo)`, sólo por el camino
      semántico, con `method_source` declarando cuál se usó (I-SEL1).
- [x] **T4 · sin fallback en modo semántico.** Caer a importancia deshacía el piso en silencio.
- [x] **T5 · diversidad.** `diversificar` (MMR con solape léxico) + `palabrasDe` + `solapeBolsas` (I-SEL3).
- [x] **T6 · reserva para artículos completos.** `elegirCorpus` reemplaza a `preferCuratedSources` (I-SEL4).
- [x] **T7 · invariantes.** I-SEL1..4, con el sabotaje de I-SEL3 implementado y no descrito.
- [x] **T8 · el ataque A6 se reencuadra.** Cerrado por el camino semántico; por léxico sigue constante
      y ahora se DECLARA.
- [x] **T9 · el test que pasaba por coincidencia.** `TestDesignMethodExcluidoDelCorpus`, corregido.
- [x] **T10 · entrega.** lint + vet + build + `go test ./...` verdes, CHANGELOG, bump, PR.
