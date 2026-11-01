---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f2
status: draft
---

# Tareas — spec-graph · F2 (anclaje + rancedumbre)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (idealmente ≤ ~400 líneas de
diff) y nombra los requisitos (`R#`) y decisiones (`D#`) que realiza. Un `1 feature = 1 PR` (musubi-rules #7);
los commits se ordenan como abajo para que la review lea el refactor puro (T1) aparte de la feature.

## Implementación
- [x] **T1** — Factorizar las primitivas puras a `internal/codeintel/fingerprint.go` (**move + export**):
  `SymbolFingerprint`, `FileFingerprint`, `SymbolMatches`, `SplitOriginRef`, `NormalizeCodePath`,
  `ErrSymbolNotFound`. `internal/memory/origins.go` conserva **alias finos** (`symbolFingerprint =
  codeintel.SymbolFingerprint`, …) para que sus call-sites y su suite #326 queden **sin tocar**. Refactor
  SIN cambio de comportamiento. · _(D3)_ · _archivos:_ `internal/codeintel/fingerprint.go`,
  `internal/memory/origins.go`
- [x] **T2** — Sumar el vocabulario `anchors` a `internal/specgraph/parse.go`: `FieldAnchors="anchors"` al
  const block, a `ListFields` y a `FieldOrder`. Opt-in puro (sin anchors ⇒ idéntico a F1). · _(R1/R10/D2)_ ·
  _archivos:_ `internal/specgraph/parse.go` · _independiente de T1_
- [x] **T3** — `internal/specgraph/anchors.go`: tipo `Anchor{Ref,Path,Symbol,Baseline}` + parseo de una
  entrada `ruta#símbolo=digest` (split por el **último `=`**; sin `=` ⇒ baseline vacío); `const
  maxAnchors=10`; `BuildAnchorEntries(root, refs) ([]string,error)` (normaliza, valida existencia + símbolo,
  calcula digest, **error R2** si no resuelve, tope); `StaleAnchor{Path,Reason}` + `StalenessOf(root, fm)
  []StaleAnchor` (re-deriva por ancla, compara, **surface-only**, traga E/S ajena al contenido). ·
  _(R2/R3/R4/R6/R7/D2/D4/D6)_ · _depende de:_ T1, T2 · _archivos:_ `internal/specgraph/anchors.go`
- [x] **T4** — `create`/`update` aceptan y validan `anchors`: el handler toma los refs, llama
  `BuildAnchorEntries`, y escribe las entradas enriquecidas. `update` resuelve el **conjunto final** (dedup
  **por ref**, digest nuevo gana) por una vía **anchor-aware** —no el `setListField`/`AddList` genérico— que
  un `update` ajeno **no** dispara (no re-vouchea). Lossless en todo lo demás. · _(R2/R7/R11/D7)_ · _depende
  de:_ T3 · _archivos:_ `internal/mcp/methods_spec.go`, `internal/specgraph/frontmatter_edit.go`
- [x] **T5** — `get`/`validate` surface la marca: `specNodeOut` gana `stale,omitempty` +
  `staleWarning,omitempty` (prosa antepuesta que el agente lee); `specValidateOut` gana `staleAnchors`
  (`[]` nunca `null`), **sin voltear `valid`** (sigue siendo integridad estructural). · _(R4/R5/R8/D5/D8)_ ·
  _depende de:_ T3 · _archivos:_ `internal/mcp/methods_spec.go`
- [x] **T6** — Regenerar el golden de `tools/list` (`go test ./internal/mcp -run TestToolsListGolden
  -update`) porque `create`/`update` ganan `anchors`; confirmar que la **clasificación no cambió**
  (`create`/`update` write; `get`/`grep`/`graph`/`validate` read-only). · _(R12)_ · _depende de:_ T4 ·
  _archivos:_ golden de `internal/mcp`, `registry.go` (verificación)

## Pruebas
_(cada invariante con su sabotaje dirigido al invariante que el test declara — musubi-rules #8)_
- [x] **T7** — codeintel: la suite #326 de `memory` (`TestS1…`, line-shift, extractor-le-gana) queda **verde
  sin cambios** vía los alias (guarda del move T1); + test directo de `codeintel.SymbolFingerprint` /
  `SymbolMatches`. _Sabotaje:_ romper `SymbolMatches` → esos tests en rojo desde su nuevo hogar. · _(D3)_
- [x] **T8** — anclar al guardar (R2): a símbolo existente crea; a inexistente **falla** con el mensaje que
  ofrece el archivo entero; a lenguaje sin extractor falla (D4); tope `maxAnchors=10` → error, no truncado.
  _Sabotaje:_ aflojar la validación a warning deja pasar el ancla rota. · _(R2/R3/R10)_
- [x] **T9** — changed/missing + precisión al símbolo (R4/R7): editar el cuerpo del símbolo → `changed`;
  borrar/renombrar → `missing`; tocar `B.Close` **no** marca `A.Close`; desplazar líneas sin cambiar
  contenido **no** marca. _Sabotaje:_ hashear el archivo entero en vez del rango → rojo. · _(R4/R7)_
- [x] **T10** — surface-only (R5: misma posición/status, sólo campo extra); E/S ajena **no** marca (R6);
  `validate` reporta `changed`+`missing` **sin** voltear `valid` (R8/D8). _Sabotaje:_ hacer que la marca
  reordene/filtre, o que `valid` se vuelva false por un ancla rancia → rojo. · _(R5/R6/R8)_
- [x] **T11** — round-trip lossless con `anchors`+digest (R11, estilo id-roundtrip de F1: parse→serialize→
  parse conserva prosa, comentarios, orden y el digest sobrevive); + test **negativo** R9: `specgraph` no
  expone/llama escritor de outbox/sync. · _(R9/R11)_

## Docs / cierre
- [x] **T12** — `CHANGELOG.md` `[Unreleased]` + bump semántico; actualizar la SKILL `spec-graph` para
  documentar `anchors` + rancedumbre (y una línea en la regla always-on si entra en su presupuesto).
- [x] **T13** — Verificar contra la spec: cada `R1…R12` con cobertura; `GOWORK=off go vet ./... && go build
  ./... && go test ./...` en verde local (**`-race` lo corre el CI en Linux**); golden verde; la matriz
  cross-platform (test-cross macos/windows) ejercita cualquier brazo tras `runtime.GOOS` (lección de F1).

## Forecast de review
- Líneas estimadas: ~1000 (≈150 refactor move T1 · ~240 código nuevo T2–T5 · golden generado T6 · ~450
  tests T7–T11 · docs T12). El grueso es tests + un move mecánico.
- ¿Chained PRs recomendado? **No** — un solo PR de F2 (el factoring T1 no tiene sentido suelto; existe en
  servicio de F2). T1 va como **primer commit aislado** (refactor puro, sin cambio de comportamiento) para
  que la review lo lea aparte de la feature.
