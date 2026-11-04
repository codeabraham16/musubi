---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f4-archive
status: draft
---

# Tareas — destino del `archive` del SDD (spec-graph · F4b)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (idealmente ≤ ~400 líneas de
diff) y nombra los requisitos (`R#`) y decisiones (`D#`) que realiza. Un `1 feature = 1 PR` (musubi-rules
#7). En el plan **original** de F4 (antes del split) esto era **T5–T6 + T13**; acá se renumera
self-contained. El estado real (`[x]`/`[ ]`) refleja el worktree `C:\Proyectos\musubi-wt-f4archive`
(rama `feat/spec-graph-f4-archive`, **standalone sobre `origin/main` `8e2aa13e`** — NO apilada sobre la
federación): **todo el cambio está hecho y verificado**. Este PR es independiente de #789: no comparte una
línea con la federación, así que no espera su merge. Lo único owner-gated es el `push` y abrir el PR.

## Implementación — núcleo (HECHO)
- [x] **T1** — `MergeSpecInto` + `SectionConflict` en `internal/specgraph/archive.go`: promoción (delta
  verbatim, exige id+type) / merge (frontmatter determinista — escalar gana-el-delta salvo `id`, listas
  unión, `anchors` por-ref via `MergeAnchorEntries`; cuerpo por secciones aditivo, surface-not-auto) /
  lossless via `UpdateFrontmatterText`. · _(R13/R14/R15/R16/D9)_ · _archivos:_ `internal/specgraph/archive.go`
  · _validado en la ronda 1 de la review adversarial de F4._
- [x] **T2** — Proyección `projectSDDProcessDelta` + `var sddProcessKeys` en `internal/mcp/spec_archive.go`:
  poda las claves de proceso (H1), quita `status` del delta (H7), en promoción exige+inyecta id/type y
  valida el status contra `specgraph.SpecStatuses`; en merge no inyecta nada; reserializa con
  `SerializeFrontmatter` (D11). Puro, model-free. · _(R19/D8/D10/D11)_ · _archivos:_ `internal/mcp/spec_archive.go`

## Implementación — cableado de la tool (HECHO)
- [x] **T3** — Tool `musubi_spec_archive`: handler + `specArchiveOut` en
  `internal/mcp/methods_spec_archive.go` y la 10ª `toolEntry` en `internal/mcp/methods_spec.go`. Lee el
  delta `specs/<change>/spec.md` y la `SPEC.md` canónica del módulo del FS (vía `os.OpenRoot` +
  `ResolveSpecPath`), llama `projectSDDProcessDelta`→`MergeSpecInto` y aplica la **política de escritura**:
  SIN conflictos **escribe** la canónica (`written:true`; archivar es *persistir* la fusión) y OMITE
  `merged` del payload; CON conflictos NO escribe y surfacea `conflicts`+`merged` (surface-not-auto,
  R15/R21). Clasificada **`write`** (sin `readOnly` en la `toolEntry` ⇒ el dispatch toma el candado
  exclusivo: fundir es un read-modify-write sobre la canónica). · _(R21/D8/D9)_ · _depende de:_ T1, T2
- [x] **T4** — Directiva de la fase `archive` en `internal/memory/sdd.go:106`: además de «guardá el
  aprendizaje», **invocar** `musubi_spec_archive`. `toolSDD` **no se toca**. · _(D8)_ · _depende de:_ T3

## Pruebas
_(cada invariante con su sabotaje dirigido al invariante que el test declara — musubi-rules #8; `-count=1`)_
- [x] **T5** — Proyección (`internal/mcp/spec_archive_test.go`), 5 tests saboteables con anotaciones
  `// arnes` verbatim contra el fuente gofmt-eado: promoción poda+inyecta (H1/R13), merge status
  canónico-autoritativo (H7), merge no contamina (H1), promoción exige identidad (R13), promoción status
  válido (R13). _Sabotajes confirmados **no-vacuos y precisos**:_ neutralizar la poda ⇒ SÓLO los 2 tests
  H1 rojos; quitar `delete(…FieldStatus)` ⇒ SÓLO el test H7 rojo. · _(R19/R20)_
- [x] **T6** — Core (`internal/specgraph/archive_test.go`): crea/fusiona/surfacea + identidad + lossless
  (R13/R14/R15/R16/E8/E9/E12/E13/E14), con un sabotaje al surface. · _(R13–R16/R20)_
- [x] **T7** — Tests a **nivel tool** (`internal/mcp/methods_spec_archive_test.go`): 3 tests que
  despachan por `handleToolsCall` (el mismo camino que un cliente MCP, con la clase `write` y el candado) y
  afirman sobre la salida serializada Y sobre el archivo en disco — (1) merge limpio: canónica sin claves
  de proceso (H1) y sin degradar `status:active` (H7), aditivo, `written:true`, sin `merged`; (2) promoción
  sin status nace `active`; (3) conflicto: `written:false`, surfacea `conflicts`+`merged`, el FS queda
  intacto. Los 3 sabotajes viven en el handler y atacan el invariante que su test declara (fundir el delta
  crudo / default `draft` / aflojar la compuerta `len(conflicts)==0`), verificados **no-vacuos**. · _(R20)_
  · _depende de:_ T3

## Golden / docs / cierre (HECHO)
- [x] **T8** — Regenerados el golden de `tools/list` **y el contrato** (`go test ./internal/mcp -run
  TestToolsListGolden -update` y `-run TestContratoPorToolFijado -update`, con `GOWORK=off -tags
  treesitter`): hay **tool nueva** y la **clase** cambia (el contrato registra `musubi_spec_archive`
  `read_only:false`/`actua_afuera:false`), no sólo params. **Censada** en `internal/censo` como
  `DebeInvocarse` (igual que sus nueve hermanas `musubi_spec_*`): el invariante «toda tool viva está
  censada» (`TestCadaToolVivaEstaCensada`) rebota una tool nueva sin clase. Actualizado el **conteo de
  tools** del README (visible 89→90, registradas 94→95) en los dos idiomas; el bloque `novedades` **NO**
  se toca (refleja el release `0.148.0`; esta tool es `[Unreleased]`). · _(R21)_ · _depende de:_ T3
- [x] **T9** — `CHANGELOG.md` `[Unreleased] → ### Added` con la entrada de `musubi_spec_archive` (MINOR
  aditivo; **`VERSION` NO se toca** — sólo cambia en `chore(release): cortar X.Y.Z`). SKILL `spec-graph`
  actualizada en su FUENTE `cmd/musubi/cognitive.go` (9→10 tools + paso «ARCHIVAR»); el `.claude/skills/…`
  renderizado es gitignored y lo refresca `musubi setup`. · _depende de:_ todo
- [x] **T10** — Verify contra la spec: cada `R13…R22` con cobertura; **`gofmt -l` limpio** (rebotó
  `methods_spec_archive.go` por CRLF → normalizado a LF; las 3 anclas `// arnes` intactas, 1 ocurrencia
  c/u); `GOWORK=off go vet -tags treesitter ./...` (0) **&&** `GOWORK=off go build -tags treesitter ./...`
  (0) **&&** `GOWORK=off go test -tags treesitter -count=1 ./...` en **verde local** el **2026-11-04** (el
  census de arnes cuenta las 3 anotaciones nuevas; **`-race` lo corre el CI en Linux**, no acá: no-cgo por
  diseño). · _depende de:_ todo

## Forecast de review
- Líneas estimadas (este PR, mitad `archive`): ~**750** (≈330 core `archive.go` ya escrito · ~100 proyección
  `spec_archive.go` ya escrita · ~120 cableado de tool T3 · ~30 directiva T4 · golden/contrato generados ·
  ~250 tests — de los cuales el core (T6) y la proyección (T5) ya están · ~60 docs). El grueso ya vive en
  el worktree, verificado; lo que falta es mecánico y merge-gated.
- **El split ya se decidió** (el dueño eligió «federación ya, archive aparte»): esta mitad es
  **independiente** —no comparte una línea con la federación (#789)— y sale en su propio PR, completa y
  verde por sí sola. Cierra el **bloqueante 2** de la review adversarial de F4.
- **Owner-gated (lo único que falta):** el `push` (con un `musubi receipt emit` fresco desde el worktree) y
  abrir el PR, ambos con un OK explícito del dueño. **No** depende del merge de #789 (base standalone sobre
  `main`); el merge de #789 es una decisión aparte del dueño. Este PR no toca código existente salvo la 10ª
  `toolEntry` en `methods_spec.go` y la directiva en `sdd.go`; todo lo demás son archivos nuevos.
