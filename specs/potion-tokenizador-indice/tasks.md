---
artifact: tasks
schema_version: "1.0"
change: potion-tokenizador-indice
status: in-progress
---

# Tareas — el embebedor completo tokeniza con el índice

Checklist ordenada por dependencia. Pruebas primero: cada prueba nueva se ve ROJA contra el código
de hoy, o contra su sabotaje, antes de que el cambio la ponga verde.

## Medición base (antes de tocar código)
- [x] M1 — Bytes de `tokenizer.idx` en disco contra los armados por este código: **iguales**
  (14.180.024 bytes). `formatoIndice` no sube.
- [x] M2 — Heap vivo, un proceso por medición:
  - mapa: **39,7 MB**, cargado en 1,2–1,3 s;
  - índice: **13,9 MB**, cargado en 0,03 s.
- [x] M3 — Ids del mapa contra los del índice sobre las 2.907 notas reales, con tiempo por franja
  de largo: **0 notas con ids distintos**; el índice es 54,6× más rápido en total (4,2× bajo 100
  runas, 69,9× entre 100 y 1k, 62,7× entre 1k y 10k, 22,4× desde 10k). Log en
  `~/.cache/musubi-medir/ids-memoria.log`.
- [x] M4 — `NewStaticProvider` en frío con el binario de hoy: tiempo, VmRSS y VmHWM, 5 procesos.
  Mediana **1,49 s** (1,45–1,59) y VmHWM **~846 MiB**. Log en `~/.cache/musubi-medir/m4.log`.

## Pruebas (rojas primero)
- [x] T1 — Ayudante `conElMapa` y `tablaDeJugueteConSidecars` devolviendo la referencia del mapa.
  `TestUnIndiceAbiertoNoRompeNada` compara contra el mapa (D6) · _archivos:_
  `internal/embedding/consulta_liviana_test.go`
- [x] T2 — `TestElCompletoConElIndiceAlDiaNoDeserializaElJSON` y
  `TestElCompletoUsaElIndiceQueAcabaDeEscribir`. Hacen falta la costura `deserializarTokenizer`
  (D5) y el archivo de pruebas nuevo `internal/embedding/completo_indice_test.go` · _depende de:_ T1
- [x] T3 — `TestUnIndiceIlegibleConIdentidadVigenteSeReescribe` (D1) · _depende de:_ T1
- [x] T4 — `TestConsultaLivianaEsBitExactaConLaTablaReal` contra el mapa, y además el completo
  contra el mapa · _depende de:_ T1
- [x] T5 — `TestLaTokenizacionDelCharsmapEstaFijada`: la misma huella por el camino del índice ·
  _archivos:_ `internal/embedding/charsmap_fijado_test.go`
- [x] T6 — `TestIndiceTokenizerBitExacto`: texto real largo (los `.go` del paquete, ~20.000 runas)
- [x] T7 — `TestElIndiceRealEstaAtadoASuFormato` (R7, D7) y `huellasDelIndiceReal`, con el control
  contra `KnownModels`

## Implementación
- [x] T8 — `indiceAlDia`. `escribirSidecarsSiHaceFalta` devuelve `*unigram` y la limpieza sale de
  ella (D1, D2, D3) · _archivos:_ `internal/embedding/consulta_liviana.go`
- [x] T9 — `cargarSidecars` sin `cabeceraVigente`; se actualizan los comentarios de
  `cabeceraVigente` y `formatoIndice`; el sabotaje de `TestUnIndiceDeOtroFormatoSeReescribe` pasa
  a `cabeceraVigente` (D4) · _depende de:_ T8
- [x] T10 — `NewStaticProvider`: limpieza, camino rápido, deserialización por la costura y cambio
  al índice recién escrito · _archivos:_ `internal/embedding/static.go` · _depende de:_ T8
- [x] T11 — Comentarios que dejan de ser ciertos:
  - cabecera de `consulta_liviana.go` (quién usa el índice);
  - `indice_tokenizer.go` («POR QUÉ EXISTE»: ahora también el completo);
  - la doc de `NewStaticProvider` y de `cargarSidecars`.
- [x] T12 — `ci.yml`: `TestElIndiceRealEstaAtadoASuFormato` en el paso del asset real y en su
  bucle `--- PASS` · _archivos:_ `.github/workflows/ci.yml`

## Verificación
- [x] V1 — `go vet` + suite de `internal/embedding` + `cmd/musubi -run Embebedor`
- [x] V2 — Pruebas reales con `MUSUBI_SPM_TESTDATA` y `MUSUBI_POTION_DIR` (rutas absolutas)
- [x] V3 — Arnés: `Validar` y `Colisiones` sobre el árbol; cada sabotaje nuevo o movido, corrido y
  visto ROJO
- [x] V4 — Medición después: M3 con el binario nuevo, ya por `StaticProvider`, y M4 con el binario
  nuevo. Comparar contra la base: mediana **0,56 s** (0,51–0,67) y VmHWM **~533 MiB**
- [ ] V5 — Revisión adversaria (skill `adversarial-review`)

## Docs / cierre
- [x] D1 — `CHANGELOG.md` `[Unreleased]`: el arranque del embebedor y la tokenización más rápidos,
  sin cambio de vectores
- [ ] D2 — Borrar `internal/embedding/zz_medir_tmp_test.go` (no se commitea)
- [ ] D3 — Preguntarle al usuario antes del push y del PR

## Forecast de review
- Líneas estimadas: ~150 de código y ~350 de pruebas, más comentarios.
- ¿Chained PRs? No: un commit de código revertible (rollback de la propuesta), con sus pruebas.
