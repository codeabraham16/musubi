# flota-en-vivo — tasks

- [x] T1 · config: `FlotaVivo *bool` en SyncConfig + `FlotaVivoActivo()`
- [x] T2 · `SyncClient.PushFlota` (POST /api/flota, bearer, timeout del sync)
- [x] T3 · remitente `RunFlotaVivo` (suscripción, filtro trabajo, batch 32/2 s, descarte)
- [x] T4 · receptor `handlerFlota` (auth, decode estricto, topes, re-sellado, publish)
- [x] T5 · ruta `/api/flota` en http.go + arranque en cmd/musubi/main.go
- [x] T6 · tests I1–I5, cada uno visto ROJO bajo su sabotaje (flip manual documentado)
- [x] T7 · go vet + build + suites mcp/config verdes; CHANGELOG [Unreleased]
- [ ] T8 · PR único; deploy central; los daemons locales toman el binario nuevo en su próximo
      rebuild (PC y laptop)
- [ ] T9 · verificación con píxeles: trabajar de verdad en una terminal y ver el pulso en el
      panel del central
