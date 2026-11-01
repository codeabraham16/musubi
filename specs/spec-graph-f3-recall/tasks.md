---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f3-recall
status: draft
---

# Tareas — spec-graph · F3-A (recall semántico sobre specs)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (idealmente ≤ ~400 líneas de
diff) y nombra los requisitos (`R#`) y decisiones (`D#`) que realiza. Un `1 feature = 1 PR` (musubi-rules #7);
los commits se ordenan **core → wiring → superficie MCP → cierre** para que la review lea el core puro y
testeable (T1/T5) aparte del cableado y la capa de tools.

## Implementación
- [ ] **T1** — `internal/specgraph/recall.go` (el core, FS-nativo y model-free, sin importar el motor):
  - Tipos: `Embedder interface { Embed(ctx, text) ([]float32, error) }` (el seam, satisfecho por el
    Provider ya blindado del server — **el portero R7 viaja gratis**); `SpecHit{ID,Type,Title,Path,Score,Snippet}`;
    `RecallResult{Hits,Truncated,NextOffset}` (espeja `GrepResult`); `RecallOptions{Query,Type,Limit,Offset}`;
    `Recaller{mu sync.Mutex; cache map[string]cachedVec; embName string}` + `cachedVec{hash uint64; vec []float32}`;
    `NewRecaller(embName string) *Recaller`; `const DefaultRecallLimit = 20`.
  - `Recall(ctx, entries []SpecContentEntry, opts RecallOptions, emb Embedder, embEnabled bool) (RecallResult, error)`:
    filtra el corpus por `type` (R10) → rankea (coseno si `embEnabled` y el spec embebe; keyword si no) →
    ordena `(score desc, id asc, path asc)` (R6) → pagina con `offset/limit`, setea `Truncated/NextOffset` (R10).
    Degradación en dos niveles: query falla/timeout ⇒ toda la llamada a keyword (R5); spec falla
    (`ErrSecretsBlocked` u otro) ⇒ ese spec a keyword (R7). **El embed corre FUERA del mutex del cache.**
  - Helpers puros: `cosine(a,b []float32) float64` (clamp `[0,1]`); `keywordScore(qterms, content) float64`
    (`distinctMatched/len(qterms)` ∈ `[0,1]`); `tokenize(s) []string`; `bodyLines(content) []string` (saltea
    el bloque frontmatter `---…---`); `bestSnippet(qterms, content) string` (línea con más términos distintos,
    desempate menor nº de línea; sin match literal ⇒ primera línea de prosa; recorte ~200 chars).
  - `ContentEntries()` ya devuelve SÓLO specs reales (`scan()` filtra `IsSpec`): **R8 gratis** (el diario SDD no entra).
  - · _(R1/R2/R4/R5/R6/R7/R8/R9/R10/D1/D2/D3/D4/D7/D8/D9)_ · _archivos:_ `internal/specgraph/recall.go` · _core puro, independiente de la capa MCP_

- [ ] **T2** — Wiring en `internal/mcp/server.go`: campo `specRecaller *specgraph.Recaller` en el struct,
  construido en `NewMcpServer` junto a `specIndex` (`specgraph.NewRecaller(embedder.Name())`, con el
  `embedder` ya normalizado a `NoopProvider` si venía nil). Persiste el cache entre llamadas (D1). ·
  _(D1/D6)_ · _depende de:_ T1 · _archivos:_ `internal/mcp/server.go`

- [ ] **T3** — Handler `toolSpecRecall` + DTOs en `internal/mcp/methods_spec.go`: `specRecallOut{Hits,Truncated,NextOffset}`
  y `specHitOut{ID,Type,Title,Path,Score,Snippet}` (tags minúscula, `make` para no-nil, go-hygiene #6).
  El handler valida `query` no vacío (⇒ `codeInvalidParams`, R1), recorta `limit` por un tope duro
  (como `specGrepLimitMax`), arma el corpus con `s.specIndex.ContentEntries()`, consulta
  `embedding.Enabled(s.embedder)`, envuelve en `context.WithTimeout(ctx, 30*time.Second)` y llama
  `s.specRecaller.Recall(ctx, entries, opts, s.embedder, enabled)`. **Firma con ctx** (`toolSpecRecall(ctx, raw)`,
  no `noCtx`), porque necesita el contexto para el timeout del embed. · _(R1/R2/R10)_ · _depende de:_ T1, T2 ·
  _archivos:_ `internal/mcp/methods_spec.go`

- [ ] **T4** — Registrar la tool en `internal/mcp/registry.go` (o en `specToolEntries()` de `methods_spec.go`,
  donde viven las 7 de spec): `toolEntry` de `musubi_spec_recall` con schema (`query` obligatorio;
  `limit`/`offset`/`type` opcionales) y **`readOnly: true` + `lock: lockSelf`** (D6, espejo de
  `musubi_search_semantic`). Descripción que la contraste con `musubi_spec_grep` (significado vs literal)
  y nombre el fallback model-free. · _(R3/D5/D6)_ · _depende de:_ T3 · _archivos:_ `internal/mcp/methods_spec.go` (bloque `specToolEntries`)

## Pruebas
- [ ] **T5** — `internal/specgraph/recall_test.go`: un test por invariante, **rojo bajo sabotaje del
  invariante que declara** (tabla R12 del design). `Embedder` de prueba determinista (mapea el caso estrella
  "espera exponencial" cerca de "backoff"; puede devolver error o `ErrSecretsBlocked`-like a demanda).
  Cubre R4 (significado > texto), R5 (sin embebedor y query-falla: sin error), R6 (orden determinista con
  empate), R7 (spec secret-blocked cae a keyword), R8 (diario SDD excluido), R9 (frescura: cambiar Content
  reembebe), R10 (filtro type + paginado). · _(R4/R5/R6/R7/R8/R9/R10/R12)_ · _depende de:_ T1 · _archivos:_ `internal/specgraph/recall_test.go`

- [ ] **T6** — `internal/mcp/methods_spec_test.go`: superficie MCP. `query` vacío ⇒ error; forma de salida
  (hits rankeados, campos presentes); la tool figura `readOnly`+`lockSelf` en la clasificación
  (`read_surface_class_test`/`solo_lectura_test` según corresponda) y **ninguna llamada muta un `SPEC.md`**
  (R3). · _(R1/R3/D6)_ · _depende de:_ T3, T4 · _archivos:_ `internal/mcp/methods_spec_test.go`

- [ ] **T7** — Regenerar los **3 goldens** de `tools/list`
  (`GOWORK=off go test ./internal/mcp -run TestToolsListGolden -update`) y actualizar el **conteo de tools
  del README** (`readme_toolcount_test.go` pasa a verde). · _(R3)_ · _depende de:_ T4 · _archivos:_ goldens de `internal/mcp/testdata/`, `README.md`

## Docs / cierre
- [ ] **T8** — `CHANGELOG.md` sección `[Unreleased]`: una viñeta para `musubi_spec_recall` (recall semántico
  sobre specs, fallback model-free). **Bump minor** (tool nueva aditiva). · _depende de:_ T4 · _archivos:_ `CHANGELOG.md` (+ archivo de versión si aplica)

- [ ] **T9** — Verificación contra la spec (fase verify, adversarial): `GOWORK=off go vet ./... && go build ./...
  && go test -race -count=1 ./...` en verde; `golangci-lint` cruzado a `GOOS=linux GOARCH=amd64` limpio; cada
  `R1..R13` con cobertura trazada. · _(R11)_ · _depende de:_ T1–T8

## Forecast de review
- Líneas estimadas: core ~300 (T1) · wiring ~15 (T2) · handler+DTO ~70 (T3) · registry ~40 (T4) · tests ~320 (T5/T6) · goldens+changelog ~chico. Feature (sin tests/goldens) ~425.
- ¿Chained PRs? **No.** F3-A es **un** PR (`1 feature = 1 PR`); F3-B (cross-link) es el PR hermano separado. Dentro de este PR, los commits se ordenan core→wiring→superficie→cierre para que la review lea el core puro aparte.
- **Gate del dueño:** parar y pedir OK explícito antes de cualquier `push`/PR/`merge` (regla de oro del repo).
