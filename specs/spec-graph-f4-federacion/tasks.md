---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f4-federacion
status: draft
---

# Tareas — spec-graph · F4 (federación opt-in de specs)

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable (idealmente ≤ ~400 líneas de
diff) y nombra los requisitos (`R#`) y decisiones (`D#`) que realiza. Un `1 feature = 1 PR` (musubi-rules #7).
**F4 se partió en dos PRs** (decisión del dueño, 2026-11-03): este entrega la **mitad federación** (acoplada
al outbox: T1–T4, T7–T12, T14–T15); la **mitad `archive`** (FS-local, sin sync: T5–T6, T13 y sus R13–R16/
E8–E9/D8–D9) va en un **PR aparte**, con su material preservado. El invariante **primario se testea primero**
(T8: «qué NO sale»).

## Implementación — mitad federación
- [ ] **T1** — Vocabulario `shared` en `internal/specgraph/parse.go`: `FieldShared="shared"` al const block,
  a `ListFields` y a `FieldOrder`; validación **booleana** (valor no booleano ⇒ error, R3); helper
  `IsShared(fm) bool`. Opt-in puro, lossless (sin `shared` ⇒ idéntico a F1–F3). · _(R1/R3)_ · _independiente_
  · _archivos:_ `internal/specgraph/parse.go`
- [ ] **T2** — Métodos de motor dedicados en `internal/memory` (archivo `spec_projection.go`):
  `ProjectSpecShared(specID, projectID, content string, emb []float32) error` —UPSERT de la obs derivada con
  id determinista `spec-proj:<project_id>:<spec_id>`, scope `shared`, `semantic`, procedencia `human`,
  `confidence 1.0`, + bump `sync_seq` + `enqueueOutboxTx` en **1 tx**, salteando las guardas de obs humana—;
  `RetractSpecProjection(specID, projectID string) error` —hard-delete de la obs + su fila de outbox—. El
  `content` lo pasa el caller (renderizado de campos parseados, D3). · _(R4/R5/R6/R9/D1/D2/D4/D5)_ ·
  _independiente (toma params; el caller T4 los arma)_ · _archivos:_ `internal/memory/spec_projection.go`
- [ ] **T3** — Reconciliador `ReconcileSpecProjections(shared []SpecProjectionInput) error` en
  `internal/memory`: enumera las proyecciones `spec-proj:%` locales con **JOIN a `outbox`** para leer el
  sello; **siembra** las `shared` sin proyección; **hard-deletea** las de **origen local** (sello ≠ `espejo`)
  huérfanas; deja **intactas** las `espejo` (bajaron del central). Se engancha en la apertura **al lado de**
  `BackfillOutbox` (ambos idempotentes). · _(R10/R10b/D7)_ · _depende de:_ T2 · _archivos:_
  `internal/memory/spec_projection.go`, el call-site de apertura (junto a `BackfillOutbox`)
  · **DESVÍO del plan (consignado):** la firma perdió el `root string` que este ítem pedía. `internal/memory`
  NO puede leer el FS de specs sin importar `internal/specgraph` (romper D6: leer specs es *policy*, va en la
  capa de tools/daemon). El enumerado del FS vive ahora en `ReconcileSpecProjectionsOnStartup` (capa mcp), que
  arma `[]SpecProjectionInput` ya renderizado y se lo pasa; el motor queda puro-DB. `root` era peso muerto.
- [ ] **T4** — Hooks en `internal/mcp/methods_spec.go`: `create`/`update` ganan el param `shared` (validado
  vía T1); **tras escribir el `SPEC.md`**, renderizan el `content` de los campos **PARSEADOS** (id/type/title/
  cuerpo acotado, vía el `Gist()` existente — **nunca** del texto crudo, D3) y llaman
  `ProjectSpecShared`/`RetractSpecProjection` **gateados por `outboxEnabled` = `HasDestination()`** (D6);
  `delete` retira. El pull (`IngestShared`) **no** pasa por acá ⇒ no-eco (R12). · _(R2/R3/R4/R7/R8/R11/R12/D3/D6)_
  · _depende de:_ T1, T2 · _archivos:_ `internal/mcp/methods_spec.go`

## Golden (federación)
- [x] **T7** — Regenerar el golden de `tools/list` (`go test ./internal/mcp -run TestToolsListGolden -update`,
  con `GOWORK=off -tags treesitter`) porque `create`/`update` ganan el param `shared`; confirmar la
  **clasificación** en `registry.go`: `create`/`update` **siguen `write`** (no hay tool nueva ni cambio de
  clase; marcar read-only algo que muta es un lost-update que `-race` no ve). · _(R18)_ · _depende de:_ T4 ·
  _archivos:_ golden de `internal/mcp`

## Diferido al PR de `archive`
- **T5–T6** (función pura de merge `MergeSpecInto` en `internal/specgraph/archive.go`; tool
  `musubi_spec_archive` + la directiva de la fase `archive` en `internal/memory/sdd.go`) y su test **T13**
  (crear/fusionar/surfacear + round-trip lossless) se **difieren a un PR aparte**. F4 se partió en dos; el
  código y el material de diseño quedan preservados para ese PR. · _(R13–R16/E8–E9/D8–D9)_

## Pruebas
_(cada invariante con su sabotaje dirigido al invariante que el test declara — musubi-rules #8; `-count=1`)_
- [ ] **T8** — **qué NO sale (R2/E1 — el primario, va primero):** una spec sin `shared` (o `false`) en un nodo
  con destino **no** encola ninguna fila de outbox y **no** crea obs `spec-proj:…`. _Sabotaje:_ proyectar sin
  chequear `IsShared` ⇒ el test ve la fila de outbox y se pone rojo. · _(R2)_
- [ ] **T9** — `shared` inválido **falla al guardar** (R3, no coacción); **proyecta + idempotencia**
  (R4/R6/E2/E3): `create shared:true` ⇒ obs con id determinista, `topic_key` `spec/<id>`, scope `shared`, fila
  `pending`; `update` del cuerpo **reemplaza** (mismo id), no duplica. _Sabotaje:_ id aleatorio ⇒ el update
  duplica y el test cuenta 2. · _(R3/R4/R6)_
- [ ] **T10** — **el fingerprint de anclas (F2) NO viaja (R8/E7):** una spec con `anchors` con digest proyecta
  un `content` que **no** contiene el digest. _Sabotaje:_ renderizar del `SPEC.md` crudo en vez de los campos
  parseados ⇒ el digest aparece y el test lo detecta. · _(R8/D3)_
- [ ] **T11** — **retiro local durable + reconciliación + el espejo sobrevive** (R9/R10/E4/E10/**E11**):
  unshare/delete/cambio-de-id ⇒ obs + fila de outbox desaparecen; una proyección **local** huérfana ⇒
  hard-delete al reconciliar; una proyección **`espejo`** cuya spec no vive en mi disco ⇒ se **CONSERVA**.
  _Sabotaje (E11):_ reconciliar sin filtrar por `espejo` ⇒ borra el mirror de otra máquina ⇒ rojo. **Es el
  test que prueba que la federación no se rompe.** · _(R9/R10/R10b)_
- [ ] **T12** — **nodo terminal + backfill (R11/E5)** (sin destino marca pero no encola; al ganar destino y
  reabrir, se siembra `pending`) y **no-eco (R12/E6)** (un pull que baja una proyección de otra máquina no
  crea `SPEC.md` ni re-encola — reusa el fixture `inboundsync_eco`). _Sabotajes:_ encolar sin gatear por
  `HasDestination` ⇒ el terminal encola; re-proyectar en el ingest ⇒ eco. · _(R11/R12)_
## Docs / cierre
- [ ] **T14** — `CHANGELOG.md` `[Unreleased]` + **bump semántico MINOR** (feature aditiva: campo `shared`,
  proyección al central); actualizar la SKILL `spec-graph` para documentar el opt-in `shared`. · _depende
  de:_ todo
- [ ] **T15** — Verificar contra la spec: cada `R1…R18` con cobertura; `GOWORK=off go vet ./... && GOWORK=off
  go build -tags treesitter ./... && GOWORK=off go test -tags treesitter -count=1 ./...` en **verde local**
  (**`-race` lo corre el CI en Linux**, no acá: no-cgo por diseño); golden verde; la matriz cross-platform
  ejercita cualquier brazo tras `runtime.GOOS` (lección de F1). · _depende de:_ todo

## Forecast de review
- Líneas estimadas (sólo la **mitad federación** de este PR): ~**1100** (≈60 vocab T1 · ~150 métodos de
  motor T2 · ~120 reconciliador T3 · ~120 hooks T4 · golden generado T7 · ~450 tests T8–T12 · ~90 docs T14).
  El grueso es tests + el reconciliador (lo más fino, por el filtro `espejo` que protege la federación).
- **El split ya se decidió** (el dueño eligió «federación ya, `archive` aparte»): la mitad **archive**
  (T5–T6 + su test T13) es **independiente** —no comparte código con la federación— y sale en su propio PR.
  Este PR es la mitad federación, completa y verde por sí sola.
