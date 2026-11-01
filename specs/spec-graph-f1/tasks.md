---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f1
status: draft
---

# Tareas — spec-graph · F1 (core + 7 tools + SKILL + regla)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (idealmente ≤ ~400 líneas de
diff) y nombra el/los requisito(s) de la spec que satisface. Marcá `[x]` al completar.

> Convención de port: cada archivo Go de `internal/specgraph` abre con un comentario que apunta al `.ts`
> de origen en `c:\Proyectos\thinkrail\packages\spec-graph\core\`. Comentarios en español, identificadores
> en inglés verbatim. Build `GOWORK=off`, sin cgo, model-free.

## Implementación — core `internal/specgraph` (pi-free)
- [ ] **T1** — `parse.go`: vocabulario (`Fields`, `SpecTypes`×5, `SpecStatuses`×5, `LinkKinds`×4,
  `RequiredFields`=[id,type,title], `IdentityFields`=[id,type], `FieldOrder`) + `ParseFile` (split por fence
  `---`, BOM-strip, CR-strip por línea, YAML malformado⇒nil sin panic) + `IsSpec` (id+type) + `Scalar`/`List`
  + `SerializeFrontmatter` (descarta vacíos, listas flow). · _cubre:_ R1, R2, R3 (vocab), R4 (preserva claves
  desconocidas al parsear) · _archivos:_ `internal/specgraph/parse.go` · _port de:_ `parse.ts`
- [ ] **T2** — `frontmatter_edit.go`: `FrontmatterEdit{Set,Remove,AddList,RemoveList}` + `UpdateFrontmatterText`
  vía `yaml.Node` (preserva comentarios+orden+BOM+CRLF; line-ending leído del frontmatter, no del cuerpo;
  cuerpo byte-idéntico) + las **4 guardas** (no `set` id, no `set` de list-field, no `remove` id/type, debe
  seguir siendo spec). · _cubre:_ R5, R6, R4 (claves desconocidas intactas) · _depende de:_ T1 ·
  _archivos:_ `internal/specgraph/frontmatter_edit.go` · _port de:_ la mitad `updateFrontmatterText` de `parse.ts`
- [ ] **T3** — `graph.go`: `SpecNode`/`SpecEdge`/`SpecGraph{nodes,edges,forward,reverse,duplicateIds}` +
  `BuildGraph` (identidad=id, PRIMER archivo gana, árbol `parent` + DAG `depends-on`/`references`/`implements`,
  adyacencia forward+reverse, `covers`/`tags` NO son aristas). · _cubre:_ R7, R8, R9 · _depende de:_ T1 ·
  _archivos:_ `internal/specgraph/graph.go` · _port de:_ `graph.ts`
- [ ] **T4** — `validate.go`: `ValidationReport{DanglingLinks,DuplicateIds,ParentCycles}` + `ValidateGraph`
  (3 chequeos de thinkrail) + `UnknownTypes(g)` (delta Musubi puro, `type`∉`SpecTypes`). El delta `SPEC.md`
  malformado/incompleto NO vive acá sino en el handler (T7, vía `RejectedSpecs`). · _cubre:_ R12 (parte core) ·
  _depende de:_ T3 · _archivos:_ `internal/specgraph/validate.go` · _port de:_ `validate.ts` + D9
- [ ] **T5** — `query.go`: `GrepSpecs` (substring/regex, `ignoreCase` default true, filtros
  type/tag/parent/depends-on, paginado limit(1..200)/offset/maxBytes, CR-strip, limit inválido⇒default) +
  `GraphSlice` (subtree/ancestors/neighbors, depth default 1, cada arista una vez). · _cubre:_ R12 (grep/graph) ·
  _depende de:_ T1, T3 · _archivos:_ `internal/specgraph/query.go` · _port de:_ `query.ts`
- [ ] **T6** — `store.go`: `ResolveSpecPath` (endurecimiento entero: vacío/absoluto/Windows-namespace/`.md`/
  `..`/raíz-existe/case-fold por segmento/dir-ignorado/symlink/contención; NFC con `x/text/unicode/norm`;
  `IgnoredDirs` espeja `walkSourceTree`) + `Index` **con `sync.Mutex` interno** (walk orden estable NFC-key→raw,
  revalida por `(mtimeMs,size)`, cachea contenido sólo de specs, invalida `graphCache` ante cambio) +
  `RejectedSpecs()` (archivos `SPEC.md` que fallaron isSpec, con razón). · _cubre:_ R10, R11, D3, D9 ·
  _depende de:_ T1, T3 · _archivos:_ `internal/specgraph/store.go` · _port de:_ `store.ts` + D3 + D9

## Implementación — tools MCP `internal/mcp`
- [ ] **T7** — `methods_spec.go`: los 7 handlers `musubi_spec_{grep,get,graph,validate,create,update,delete}`
  llamando al core; `getIndex(s.projectPath)` memoizado en el server; `scaffoldBody(type)` por tipo; formato
  de salida con el helper de truncado que ya usan las tools de lectura (grep puede ser grande); `create`
  exclusivo (falla si existe/id en uso/ruta no resuelve); `validate` **compone** `ValidateGraph` +
  `UnknownTypes` + `index.RejectedSpecs()`. · _cubre:_ R12, R15 · _depende de:_ T1–T6 ·
  _archivos:_ `internal/mcp/methods_spec.go`
- [ ] **T8** — `registry.go`: registrar los 7 `toolEntry` con su schema de params + clasificar
  `grep/get/graph/validate` **readOnly:true** y `create/update/delete` **readOnly:false**, las 7
  `actuaAfuera=false`; `lock`/`ro` por default desde readOnly (sin `lockSelf`). · _cubre:_ R13 ·
  _depende de:_ T7 · _archivos:_ `internal/mcp/registry.go`
- [ ] **T9** — Regenerar los **3 goldens**: `go test ./internal/mcp -run TestToolsListGolden -update`
  (toolslist + toolslist-completo) y el updater del contrato (`contrato-tools.golden.json`). · _cubre:_ R14 ·
  _depende de:_ T8 · _archivos:_ `internal/mcp/testdata/{toolslist,toolslist-completo,contrato-tools}.golden.json`

## Implementación — regla always-on + SKILL
- [ ] **T10** — `agente.go`: agregar la **regla always-on de 4 líneas** al const `instruccionesAgente`
  (surfaced por `handleInitialize` en `instructions` + `anthropic/alwaysLoad`); debe seguir entrando en el tope
  del cliente. · _cubre:_ R16 (regla) · _depende de:_ T8 (nombres de tools reales) · _archivos:_ `internal/mcp/agente.go`
- [ ] **T11** — SKILL `spec-graph`: `.claude/skills/spec-graph/SKILL.md` (cuándo usar las 7 tools, el flujo
  crear→enlazar→validar, el vocabulario de 5 tipos). Se instala con `musubi setup`. · _cubre:_ R16 (SKILL) ·
  _depende de:_ T8 · _archivos:_ `.claude/skills/spec-graph/SKILL.md`

## Pruebas (cada invariante con su sabotaje)
- [ ] **T12** — `parse_test.go` + `frontmatter_edit_test.go` (tabla): parse scalars/block-arrays/malformado/
  sin-fence/CRLF; isSpec; serialize round-trip (+coma en item); lossless (comentarios+orden+BOM+CRLF+cuerpo
  byte-idéntico; rechaza set de list-field; NUNCA des-espec-ea). · _cubre:_ R1, R2, R4, R5, R6 ·
  _depende de:_ T1, T2
- [ ] **T13** — `graph_test.go` + `validate_test.go` + `query_test.go`: forward+reverse+duplicate(primero gana);
  slice 3 direcciones+cada-arista-una-vez; grep filtros+truncado+CR-strip+limit-inválido; validate dangling+
  duplicate+parent-cycles + delta (`type` desconocido flaggeado; `SPEC.md` sin type / YAML malformado flaggeado;
  `README.md`/`proposal.md` sin id+type **NO** flaggeado). · _cubre:_ R3, R7, R8, R9, R12 · _depende de:_ T3, T4, T5
- [ ] **T14** — `store_test.go`: `ResolveSpecPath` vector por vector (canónico/afuera/`.txt`/ignorado/Windows
  drive+stream con test `//go:build windows`/symlink dir+hoja/alias case/falla-cerrado/NFC) + Index (revalida por
  mtime+size, ignora no-specs/node_modules/dot-dirs, walk estable, drops deleted, memoiza, RejectedSpecs) +
  **test de concurrencia `-race`** martillando `Graph()`/`ContentEntries()` desde N goroutines (sabotaje: quitar
  el Mutex interno ⇒ `-race` acusa escritura concurrente del map). · _cubre:_ R10, R11, R17, D3 · _depende de:_ T6
- [ ] **T15** — tests de la capa MCP: los 3 goldens en verde; el tripwire `contrato_tool_test.go` fija
  {readOnly, actuaAfuera} de las 7; el `read_concurrency` incluye las 4 de lectura; test de que `instructions`
  sigue entrando en el tope del cliente con las 4 líneas nuevas. · _cubre:_ R13, R14, R16, R17 · _depende de:_ T8, T9, T10

## Docs / cierre
- [ ] **T16** — `CHANGELOG.md` sección `[Unreleased]` (feat: subsistema spec-graph + 7 tools `musubi_spec_*`)
  + bump semántico (minor: tools nuevas, aditivo). · _cubre:_ R18
- [ ] **T17** — Verificar contra la spec: cada R1–R18 con cobertura; `GOWORK=off go vet ./... && go build ./...
  && go test -race -count=1 ./...` en verde; alcance del diff limpio (sólo los archivos de estas tareas). ·
  _depende de:_ T1–T16

## Forecast de review
- **Líneas estimadas:** ~1300–1700 Go (core) + ~1500–2000 (tests) + ~500 (tools+registry) + goldens generados
  (grandes pero mecánicos) + SKILL/CHANGELOG. Diff total **grande** (>3000 líneas con goldens y tests).
- **¿Chained PRs?** **No** — F1 ya es la rodaja más chica del roadmap (F1–F5 se entregan por separado), y la
  convención es 1 feature = 1 PR. Las tareas T1–T17 son commits reviewables DENTRO de ese PR, ordenados para
  revisar archivo por archivo. Partir core↔tools en PRs apilados agrega la fragilidad del stack (el hijo se
  cierra al borrar la base) sin ganar nada, porque las tools no compilan sin el core ni el golden sin las tools.
- **Nivel de revisión:** la fase `verify` corre `musubi_detect_changes` para dimensionar el panel adversarial
  con el diff real; no se fija a ojo acá.
