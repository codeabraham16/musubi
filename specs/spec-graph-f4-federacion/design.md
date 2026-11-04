---
artifact: design
schema_version: "1.0"
change: spec-graph-f4-federacion
status: draft
---

# Diseño técnico — spec-graph · F4 (federación opt-in de specs)

> El principio rector es el de la propuesta: **reusar el carril de observación+outbox, no construir un
> segundo sync de specs**. La proyección de una spec `shared` es una observación **derivada y reconstruible**,
> no una fuente de verdad: si diverge, gana el FS. La decisión que el dueño delegó —*«escogé lo más óptimo,
> escalable y robusto»* para hasta dónde llega el retiro cuando se des-comparte una spec— se resuelve acá en
> **D7**: el retiro es **LOCAL y durable** (hard-delete de la proyección de origen local, invisible al recall
> y al push, durable ante el pull incremental); el retiro **mesh-wide** —que el central deje de servir la
> copia a toda la malla— se **descarta para F4** y se difiere a un primitivo de transporte de retracción
> propio (con F5), porque el carril de sync es **upsert-only en las dos direcciones** y fingir lo contrario
> sería una promesa falsa. Un hallazgo del grounding obligó a corregir la spec: el reconciliador sólo toca
> proyecciones de **origen local** (sello `espejo` = bajó del central ⇒ no se toca), o nukearía la
> federación. (El `archive` —hoy sólo consejo— se difiere a un PR aparte: F4 se partió en dos.)

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D1** | **La proyección es una fila de `observations` normal, SIN tabla nueva.** El id determinista (D2) *es* el mapeo spec↔proyección. | (b) tabla de mapeo `spec_projection` (como floteaba la propuesta); (c) tabla dedicada de specs federadas. | Reusa la tabla y el carril de observaciones (R5): outbox, dead-letter, backfill, redacción forzada del central, **una** cañería. El id determinista hace innecesaria una tabla paralela —que sería una **segunda fuente de verdad** que se desincroniza del FS, justo lo que la identidad FS-native de spec-graph evita (F2/D1)—. Rollback sin migración (proposal). |
| **D2** | **Id determinista y legible: `spec-proj:<project_id>:<spec_id>`.** | UUID aleatorio; hash del contenido; `spec/<id>` a secas. | `observations.id` es TEXT arbitrario (el sync y el JSON-RPC **no** asumen UUID, verificado). Un id estable hace que `update` **reemplace** por id (R6, idempotente); un UUID o un hash de contenido **duplicarían** en cada edición. Incluir `project_id` evita que dos repos con el mismo `spec_id` **colisionen en el central**. Cambiarle el `id` a la spec cambia la clave ⇒ el reconciliador retira la vieja y siembra la nueva **gratis** (R9). |
| **D3** | **El contenido/gist se renderiza de los campos PARSEADOS (`id`/`type`/`title`/cuerpo acotado), nunca del texto crudo del `SPEC.md`.** | Proyectar el `SPEC.md` crudo; proyectar el frontmatter entero. | F2/D1 metió el digest de anclas **DENTRO** del frontmatter (`anchors: ["ruta#S=digest"]`). Renderizar de campos parseados hace que **R8 (el fingerprint no viaja) sea una garantía ESTRUCTURAL**, no un filtro que alguien olvida al tocar el serializador. El gist sale del `Gist()` existente (R7); el cuerpo se acota al presupuesto de gist. |
| **D4** | **Procedencia `human`; `mem_type` `semantic`; scope `shared`; confidence `1.0`.** | Procedencia nueva `spec`/`derived`; procedencia `llm`/`caller`. | La proyección **no** la genera un modelo: marcarla `llm:*` la mandaría a **cuarentena** y nunca se promovería a `shared` (la puerta de corroboración). `human` reusa **exactamente** el carril de federación probado (como `PromoteObservationCtx`→outbox); el acto humano es marcar `shared: true`. F5 (arbitraje spec↔obs) distingue specs por el **namespace del id** (`spec-proj:`) y el `topic_key` `spec/<id>`, **sin** tocar el vocabulario de procedencia ni la redacción del central (menos superficie, menos riesgo). |
| **D5** | **Métodos de motor dedicados** `ProjectSpecShared` / `RetractSpecProjection`: upsert de la obs derivada + bump de `sync_seq` + `enqueueOutboxTx` en **una** transacción; el retiro hard-deletea obs + fila de outbox en una tx. Saltean las guardas de observación **humana** (p. ej. el sobre de procedencia), que no aplican a una fila derivada. | Reusar `saveObservation`/`PromoteObservationCtx` tal cual. | `saveObservation` corre validaciones pensadas para texto humano; una fila derivada tiene **contrato propio** (id determinista, procedencia fija, reconstruible desde el FS). Un método dedicado mantiene la **atomicidad** (como el borde de push) y deriva el contenido sólo de los args, sin lecturas de base. Reusa `enqueueOutboxTx` ⇒ **sin** segundo mecanismo de sync (R5). |
| **D6** | **Los hooks de proyección/retiro viven en `methods_spec.go`** (create/update/delete), **gateados por `outboxEnabled` = `cfg.Sync.HasDestination()`.** | Hook en el core de `specgraph`; hook en el loop de sync. | `specgraph` es el **core FIEL** (frontera de F1: *core fiel · política en tools*); «esta spec se federa» es **política** ⇒ capa de tools. Gatear por destino espeja a las observaciones: **nodo terminal marca pero no encola** (R11). El core de `specgraph` no escribe outbox ⇒ **R9 (F2/D9) se sostiene gratis**. |
| **D7** | **Retiro LOCAL durable, reconciliado como función pura del FS — SÓLO sobre proyecciones de origen local.** Reconciliador `ReconcileSpecProjections` al abrir y tras cada escritura. El discriminador es el **sello del outbox**: `espejo` ⇒ bajó del central ⇒ **no se toca**; de origen local (`pending`/`claimed`/`sent`/`dead`) cuya spec fuente ya no está `shared` en disco ⇒ **hard-delete**. | **(C) Retiro MESH-WIDE en F4**: un op de tombstone/visibilidad en el cable para que el central deje de servir la copia a toda la malla. · Reconciliar **todas** las proyecciones contra el FS local (lo que decía E10 antes). | **La decisión que delegó el dueño.** Medido: el carril es **upsert-only en las dos direcciones** —`OutboxItem` no lleva flag de visibilidad; `IngestShared` es un UPSERT puro que **no** toca `archived`/`superseded_by`/`quarantined`; `ListSharedForPull` documenta que un hard-delete local **deja** la copia del central—. **Ninguna** observación retracta hacia arriba hoy. (C) sería un primitivo de transporte de retracción que toca el **core de sync para TODAS las observaciones**: demasiado grande y desestabilizante para colarlo en F4, y lo necesita también F5 (supersede-transport) ⇒ se difiere (R10b). Lo **robusto/escalable** es no bifurcar el carril a medias: retiro local durable + un **límite declarado**, no un parche silencioso. Y reconciliar las `espejo` contra MI disco **rompería la federación** (la spec de otra máquina no vive en mi FS): verificado que `espejo` = «vino de afuera» (`inboundsync.go` lo sella en la misma tx del ingest; `outbox.go:25-27`). |

> **D8–D9 (diseño del merge del `archive`): diferidas al PR de `archive`** — el `archive` se cablea con
> una función pura de merge en `internal/specgraph` (crear/fusionar, model-free, surface-not-auto),
> expuesta por una tool nueva e invocada por la directiva de la fase; `toolSDD` no se toca. Material
> de diseño preservado para ese PR.

## Enfoque de implementación

**Paquetes tocados.** Tres, respetando la frontera de F1 (core fiel · política en tools):

1. **`internal/specgraph`** — el vocabulario gana `FieldShared = "shared"` (al const block, `ListFields`,
   `FieldOrder` de `parse.go`), validado booleano (R3), lossless como `anchors` (R1). Un helper `IsShared(fm)`
   lee el opt-in.
2. **`internal/memory`** — los métodos dedicados `ProjectSpecShared` / `RetractSpecProjection` (D5) y el
   reconciliador `ReconcileSpecProjections` (D7). La proyección entra como fila `observations` derivada que
   reusa `enqueueOutboxTx`; el reconciliador keyea por el namespace `spec-proj:` y **join** contra `outbox`
   para leer el sello (`espejo` ⇒ foráneo ⇒ intacto). Se engancha en la apertura, **al lado de**
   `BackfillOutbox` (ambos idempotentes; un hard-delete quita la obs ⇒ Backfill no la re-siembra).
3. **`internal/mcp/methods_spec.go`** — `create`/`update` ganan el param `shared` (validado, R3) y, tras
   escribir, llaman `ProjectSpecShared`/`RetractSpecProjection` **gateados por `outboxEnabled`** (D6);
   `delete` retira. Golden de `tools/list` regenerado; `create`/`update` **siguen `write`** (R18).

**Flujo de proyección (`create`/`update` con `shared: true`, nodo con destino).** El handler escribe el
`SPEC.md` (como F1/F2) → llama `ProjectSpecShared(root, fm)`, que (a) renderiza el contenido de los campos
**parseados** (D3), (b) deriva el id `spec-proj:<project_id>:<spec_id>` (D2), (c) hace UPSERT de la obs
`shared`/`semantic`/`human`/`1.0` (D4) + bump `sync_seq` + `enqueueOutboxTx`, todo en una tx (D5). Idempotente
por id (R6). En un **nodo terminal** `enqueueOutboxTx` no corre (no hay destino) ⇒ marca pero no encola (R11);
al ganar destino, el reconciliador + `BackfillOutbox` de apertura la siembran (E5).

**Flujo de retiro (unshare / delete / cambio de id).** `RetractSpecProjection(id)` hard-deletea la obs
derivada + su fila de outbox. Deja de empujarse y sale del recall local (R9). Durable ante el **pull
incremental** (des-compartir no avanza el `sync_seq` del central ⇒ la copia queda bajo el cursor, E4). El
cambio de id lo cubre el reconciliador: la vieja clave queda huérfana ⇒ hard-delete; la nueva ⇒ siembra.

**Reconciliador (apertura + cada escritura).** `ReconcileSpecProjections`: enumera las specs `shared` del
índice FS y las proyecciones `spec-proj:%` locales; **origen local** = fila de outbox **no** `espejo`.
`shared sin proyección` ⇒ siembra; `proyección local sin spec shared fuente` ⇒ hard-delete; `espejo` ⇒
**intacta** (E11). El conjunto de proyecciones locales queda = al conjunto de specs `shared` en disco (E10).

**No-eco (R12).** La proyección **sólo** nace de una spec local marcada `shared` (el hook de `create`/`update`).
Lo que baja por el pull entra por `IngestShared` como obs `espejo` y **no** regenera ningún `SPEC.md` ni
re-encola (es el gotcha `sync-eco-backfill-outbox`; el sello `espejo` ya frena el re-seed de Backfill).

**Degradación con gracia** (go-hygiene #3): sin `shared`, todo es idéntico a F1–F3 (opt-in puro). Sin destino,
marca pero no encola. El reconciliador se traga errores de E/S ajenos al contenido y nunca rompe la apertura.

## Contratos / interfaces

**Vocabulario (`internal/specgraph/parse.go`):**
```
FieldShared = "shared"   # booleano; true => federar. Ausente/false => local, NUNCA viaja (R2). Lossless (R1).
```

**Métodos de motor nuevos (`internal/memory`):**
```go
// Proyecta/actualiza la observación derivada de una spec shared (UPSERT + sync_seq + enqueueOutboxTx, 1 tx).
// id determinista spec-proj:<project_id>:<spec_id>; contenido de campos PARSEADOS (D3); procedencia human (D4).
func (e *DbEngine) ProjectSpecShared(specID, projectID, content string, emb []float32) error
// Hard-delete de la obs derivada + su fila de outbox (retiro local durable; R9).
func (e *DbEngine) RetractSpecProjection(specID, projectID string) error
// Función pura del FS: siembra las shared sin proyección, hard-deletea las LOCALES huérfanas; NO toca espejo (D7).
func (e *DbEngine) ReconcileSpecProjections(root string, sharedSpecs []SpecProjectionInput) error
```

**Adiciones de MCP (`methods_spec.go`):** `create`/`update` ganan `"shared": bool` (validado, R3); tras
escribir el `SPEC.md`, proyectan/retiran la observación derivada gateados por `outboxEnabled` (D6).

## Trade-offs

- **Se gana:** una **sola** cañería de sync (specs y observaciones comparten outbox/dead-letter/backfill/
  redacción); **cero tabla nueva, cero migración** (el id determinista es el mapeo); el opt-in **co-viaja**
  en git y es legible al leer la spec; el retiro local es **durable** bajo pull incremental; la federación de
  specs de **otras máquinas** sobrevive la reconciliación (el `espejo` las protege).
- **Se cede:** el **retiro mesh-wide no existe en F4** (R10b) — des-compartir una spec deja su copia en el
  central hasta que un primitivo de transporte de retracción (F5) lo resuelva; un **re-onboarding completo**
  la re-mostraría como espejo foráneo. Es el límite **honesto** del carril upsert-only, declarado, no
  silenciado.
- **Deuda consciente (la decisión delegada):** construir el transporte de retracción mesh-wide es trabajo de
  F5, con la arbitración spec↔observación que también necesita mover supersede/visibilidad hacia arriba. F4
  **no** debe simular que el central olvidó (sería un verde que miente).

## Plan de pruebas

Cada invariante con su test **saboteable** (musubi-rules #8: el sabotaje ataca el invariante que el test
declara). Todo con `GOWORK=off` y `-tags treesitter`; `-race` en CI (Linux), no en davantis-1 (no-cgo, por
diseño). `-count=1` antes de reportar.

- **qué NO sale (R2/E1 — el invariante primario, test primero):** una spec sin `shared` (o `false`) en un
  nodo con destino **no** encola ni crea obs `spec-proj:…`. Sabotaje: proyectar sin chequear `IsShared` ⇒ el
  test ve una fila de outbox y se pone rojo.
- **`shared` inválido falla al guardar (R3):** un valor no booleano es **error**, no coacción. Sabotaje:
  aflojar a `truthy` deja pasar un `shared: "si"`.
- **proyecta + idempotencia (R4/R6/E2/E3):** `create shared:true` ⇒ obs con id determinista, `topic_key`
  `spec/<id>`, scope `shared`, fila `pending`; `update` del cuerpo **reemplaza** (mismo id), no duplica.
  Sabotaje: id aleatorio ⇒ el update duplica y el test cuenta 2.
- **el fingerprint NO viaja (R8/E7 — hereda F2/D9):** una spec con `anchors` con digest proyecta un contenido
  que **no** contiene el digest. Sabotaje: renderizar del `SPEC.md` crudo en vez de los campos parseados ⇒ el
  digest aparece y el test lo detecta.
- **retiro local durable (R9/E4):** unshare/delete/cambio-de-id ⇒ la obs y su fila de outbox desaparecen.
  Sabotaje: retirar sólo la fila de outbox (dejar la obs) ⇒ el recall local todavía la ve.
- **reconciliación, origen local (R10/E10):** proyección local huérfana (su spec dejó de ser `shared`) ⇒
  hard-delete al abrir. Sabotaje: saltear el reconciliador ⇒ queda una proyección de más.
- **el espejo sobrevive (R10/E11 — el bug que el grounding atrapó):** una proyección con sello `espejo` cuya
  spec **no** vive en mi disco ⇒ se **conserva**. Sabotaje: reconciliar sin filtrar por `espejo` ⇒ la borra y
  el test (que simula la spec de otra máquina) se pone rojo. **Es el test que prueba que la federación no se
  rompe.**
- **nodo terminal + backfill (R11/E5):** sin destino, `shared:true` marca pero no encola; al ganar destino y
  reabrir, se siembra `pending`. Sabotaje: encolar sin gatear por `HasDestination` ⇒ el nodo terminal encola.
- **no-eco (R12/E6):** un pull que baja una proyección de spec de otra máquina **no** crea `SPEC.md` ni
  re-encola. Reusa el fixture de `inboundsync_eco`. Sabotaje: re-proyectar en el ingest ⇒ eco.
- **golden + clasificación (R18):** `TestToolsListGolden` verde tras regenerar (porque `create`/`update`
  ganan el param `shared`); `create`/`update` **siguen `write`** — sin cambio de clase y sin tool nueva
  (marcar read-only algo que muta sería un lost-update que `-race` no ve).
