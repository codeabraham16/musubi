# Tasks — banda-ciega-vecinos-al-guardar

## Implementación

- [ ] **T1** — `config`: `Conflicts.BandFloor` (default **0.80**, medido). Igual que `CosineFloor`, un **0 explícito** debe respetarse (usa `presentBlockKeys`, no el merge `== 0 ⇒ default`).
- [ ] **T2** — `conflicts.go`: `ConflictOptions.BandFloor` + `bandEnabled()` (`BandFloor > 0 && BandFloor < CosineFloor`).
- [ ] **T3** — `conflicts.go`: `type BandNeighbor {ID, TopicKey, Gist string; Cosine float64}`.
- [ ] **T4** — `conflicts.go`: `BandNeighbors(obsID, opts) ([]BandNeighbor, int, error)` — **solo lectura**, reusa `conflictCandidates` + `candidateCosines`, aplica `complementaryPair`, filtra `[BandFloor, CosineFloor)`, ordena por coseno desc, recorta al techo y devuelve **cuántos quedaron afuera**.
- [ ] **T5** — `mcp/methods.go`: `detectAndSurface` (camino **explícito**) llama a `BandNeighbors` y appendea el aviso. `detectOnly` **no** lo llama.
- [ ] **T6** — `mcp/methods.go`: formato del mensaje (gist, coseno, pregunta accionable, `(hay N más)`).
- [ ] **T7** — `conflictOpts()` propaga `BandFloor` desde la config.

## Tests

- [ ] **T8** — S.a: vecino en la banda ⇒ **aparece** Y **`SELECT COUNT(*) FROM observation_relations` no sube** (el invariante R0, medido en la base).
- [ ] **T9** — S.b: coseno ≥ `CosineFloor` ⇒ es `pending`, **NO** aparece además como vecino de banda.
- [ ] **T10** — S.c: coseno por debajo de `BandFloor` ⇒ sin vecinos.
- [ ] **T11** — S.d / S.e: sin coseno ⇒ sin vecinos; `BandFloor = 0` ⇒ sin vecinos (rollback).
- [ ] **T12** — S.g: más vecinos que el techo ⇒ se muestran los de **mayor** coseno y se informa **cuántos** quedaron afuera.
- [ ] **T13** — S.h: las guardas de #203 aplican (un hermano SDD en la banda **no** se muestra).

## Cierre

- [ ] **T14** — `go test ./...` verde + `golangci-lint` limpio.
- [ ] **T15** — Verificación **adversarial**: apagar la feature y confirmar que los tests que la prueban **fallan**.
- [ ] **T16** — CHANGELOG (`Added`: es capacidad nueva ⇒ **minor**).
- [ ] **T17** — PR.

## Orden

T1→T2→T3→T4 (el motor). T5→T7 (el cableado). T8-T13 en paralelo. T14→T17.

**T8 es EL test de esta rebanada.** Todo lo demás es comodidad; lo único que no se puede romper es que **mostrar no sea encolar**. Se verifica contra la **base**, no contra el valor de retorno.

**Nota de versión:** esto agrega capacidad (no arregla un bug) ⇒ `Added` ⇒ **minor** (v0.87.0), no patch.
