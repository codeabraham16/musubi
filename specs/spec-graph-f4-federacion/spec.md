---
artifact: spec
schema_version: "1.0"
change: spec-graph-f4-federacion
status: draft
---

# Especificación — federación opt-in de specs (spec-graph · F4)

Vocabulario RFC 2119: **DEBE** / **DEBERÍA** / **PUEDE**. Cada requisito es verificable y atómico; los
escenarios son observables (Given/When/Then).

## Requisitos

### Opt-in y el invariante «qué NO sale»
- **R1 (DEBE).** El vocabulario de spec-graph DEBE reconocer un campo de frontmatter booleano `shared`.
  `shared: true` marca la spec para federación; ausente o `false` NO la marca. El campo se preserva
  **lossless** (prosa, comentarios y orden del frontmatter intactos), igual que `anchors` en F2.
- **R2 (DEBE).** Una spec **sin** `shared: true` NO DEBE producir ninguna fila de outbox ni ninguna
  observación derivada de scope `shared`. Es el invariante primario: lo que importa es qué **no** sale
  de la máquina.
- **R3 (DEBE).** `create`/`update` DEBEN validar `shared`: un valor no booleano es **ERROR al guardar**
  (no truncado ni coaccionado), consistente con la validación de `anchors`.

### Federación por el outbox existente
- **R4 (DEBE).** Al crear o actualizar una spec con `shared: true` en un nodo con destino de sync
  (`cfg.Sync.HasDestination()` verdadero), el sistema DEBE proyectar un **gist derivado** de la spec
  como observación de scope `shared`, con `topic_key` `spec/<id>`, el `project_id` del contexto, y
  encolarla en el outbox existente.
- **R5 (DEBE).** La proyección DEBE reusar la cañería de observaciones (`enqueueOutboxTx`, el sync loop,
  dead-letter, `BackfillOutbox`, la redacción forzada del central). NO DEBE existir un segundo mecanismo
  de sync para specs.
- **R6 (DEBE).** El id de la observación proyectada DEBE ser **determinista** a partir del id de la spec
  (y del proyecto), de modo que actualizar la misma spec **reemplace** la proyección anterior
  (idempotente por id), sin acumular duplicados.
- **R7 (DEBERÍA).** El gist proyectado DEBERÍA llevar `id`, `type`, `title` y un resumen del cuerpo
  (vía el `Gist()` existente), suficiente para el recall federado; NO el cuerpo completo si excede el
  presupuesto de gist.
- **R8 (DEBE).** La **huella de las anclas de F2 NO DEBE viajar** en la proyección: un fingerprint sólo
  vale contra el checkout que lo calculó.

### Baja: retiro local (la proyección es función pura del FS)
- **R9 (DEBE).** Pasar una spec de `shared: true` a `false`/ausente, borrarla con `musubi_spec_delete`,
  o cambiarle el `id`, DEBE **retirar la proyección LOCALMENTE**: hard-delete de la observación derivada
  (es reconstruible, no es fuente de verdad) y de su fila de outbox, de modo que deje de empujarse y
  salga del recall local. El retiro DEBE ser **durable ante el pull**: `IngestShared` no revive una
  proyección que el FS local ya no justifica (R10).
- **R10 (DEBE).** El conjunto de proyecciones de **ORIGEN LOCAL** DEBE ser **función pura del conjunto de
  specs `shared` en disco**, reconciliado al abrir y en cada escritura, alcanzando las proyecciones por el
  **namespace del id determinista** de R6 (`spec-proj:…`): una spec `shared` sin proyección se **siembra**;
  una proyección de origen local cuya spec fuente está ausente o ya no es `shared` se **hard-deletea**. Una
  proyección que **BAJÓ del central** (sello `espejo` en el outbox) NO es de origen local y **NO se
  reconcilia contra el FS**: es el espejo de la spec de otra máquina, y borrarla contra mi disco rompería
  la federación. Gana el FS para lo local (consistente con F2); lo de afuera lo gobierna su origen (su
  retiro es mesh-wide, R10b).
- **R10b (DEBE — límite declarado).** El retiro **MESH-WIDE** —que el central deje de servir la copia a
  las demás máquinas— queda **fuera de F4**: hoy NINGUNA observación retracta hacia arriba (el carril es
  upsert-only; el `OutboxItem` no lleva flag de visibilidad; defecto medido y documentado en
  `ListSharedForPull`). Es un primitivo de transporte de retracción propio, que se entrega junto a F5.
  F4 NO DEBE simular que el central olvidó ni inventar un segundo carril para lograrlo.

### Nodo terminal y backfill
- **R11 (DEBE).** En un nodo **terminal** (sin destino) una spec `shared: true` NO DEBE encolar (igual
  que las observaciones). Al ganar destino, el **backfill** de apertura DEBE sembrar las proyecciones
  pendientes de las specs `shared` existentes.

### No-eco
- **R12 (DEBE).** Lo que **baja** del central (sync pull) NO DEBE regenerar una `SPEC.md` local ni
  re-proyectar/re-encolar una observación de spec. La proyección **sólo** nace de una spec local marcada
  `shared` (evita el eco `sync-eco-backfill-outbox`).

### Destino del `archive` → **DIFERIDO a un PR aparte**
F4 se partió en dos (decisión del dueño, 2026-11-03): esta spec cubre la **federación**. Los requisitos
del `archive` (R13–R16) y sus escenarios (E8–E9), junto con el diseño D8–D9, se difieren al **PR de
`archive`**, con su material preservado. Los números R13–R16 y E8–E9 se dejan **vacíos a propósito** para
no renumerar los invariantes de federación.

### Transversales
- **R17 (DEBE).** Aditivo, **model-free**, sin cgo; suite en verde con `GOWORK=off`. Cada invariante
  nuevo DEBE tener un test que **falla bajo sabotaje** dirigido al invariante que declara (no una
  mutación vacua ni el sitio equivocado).
- **R18 (DEBE).** El golden de `tools/list` DEBE regenerarse si cambian params (`create`/`update` ganan
  `shared`); las tools se **re-clasifican** (siguen `write`; no cambian de clase).

## Escenarios

**E1 — Una spec sin `shared` no sale.**
Given una spec `foo` sin `shared` (o `shared: false`) en un nodo con destino de sync.
When se crea o se actualiza.
Then NO se encola ninguna fila de outbox y NO existe observación `spec/foo` de scope `shared`.

**E2 — Una spec `shared` proyecta.**
Given una spec `foo` con `shared: true` en un nodo con destino.
When se crea.
Then existe una observación derivada con id determinista, `topic_key` `spec/foo`, scope `shared`,
`project_id` del contexto, y una fila de outbox `pending`.

**E3 — Update idempotente.**
Given la spec `foo` `shared` ya proyectada.
When se actualiza su cuerpo (sigue `shared`).
Then la observación `spec/foo` se **reemplaza** (mismo id), no se duplica; `sync_seq` fresco.

**E4 — Unshare/borrado = retiro local durable.**
Given la spec `foo` `shared` proyectada, con el central ya teniendo su copia.
When pasa a `shared: false`, se borra, o se le cambia el `id`.
Then la proyección local se hard-deletea (obs + fila de outbox), sale del recall local y deja de
empujarse. Un **pull incremental** posterior NO la revive: des-compartir no empuja nada hacia arriba, así
que la fila del central no avanza su `sync_seq` y queda por debajo del cursor. (Un re-onboarding completo
desde cursor 0 sí la re-ingiere, pero como **espejo** —el central todavía la tiene, R10b—, no como
proyección local reconciliable.)

**E5 — Nodo terminal + backfill.**
Given un nodo sin destino, spec `foo` `shared`.
When se crea.
Then marca pero NO encola. When luego se configura un destino y se reabre la base, Then el backfill
siembra la proyección `pending`.

**E6 — No-eco.**
Given el sync baja observaciones del central (incluida una proyección de spec de otra máquina).
When se procesa el pull.
Then NO se crea ni reescribe ninguna `SPEC.md` local y NO se re-proyecta ni re-encola nada.

**E7 — El fingerprint no viaja.**
Given una spec `foo` `shared` con `anchors` y su huella de base.
When se proyecta.
Then la proyección NO contiene la huella de las anclas (sólo el gist/estructura federable).

**E10 — Reconciliación al abrir, origen local (FS manda).**
Given una proyección `spec/foo` de **origen local** (la sembró esta máquina) cuya spec `foo` en disco ya
NO es `shared` (o no existe).
When se abre la base y corre la reconciliación.
Then la proyección `spec/foo` se hard-deletea localmente: el conjunto de proyecciones locales queda igual
al conjunto de specs `shared` en disco, ni una de más.

**E11 — El espejo de otra máquina sobrevive la reconciliación.**
Given una proyección `spec/bar` que **BAJÓ del central** (sello `espejo`), de una spec que vive en OTRA
máquina de la malla y no en mi disco.
When se abre la base y corre la reconciliación.
Then `spec/bar` se **CONSERVA** (no es de origen local): la federación sigue mostrando las specs
compartidas de la malla; su retiro sería mesh-wide (R10b), fuera de F4.

## Fuera de alcance (recordatorio, de la propuesta)
Arbitración spec↔observación (F5) · sync bidireccional (el central no edita specs) · viaje del
fingerprint de anclas · **el destino del `archive`** (R13–R16/E8–E9, diferido a su propio PR) ·
**retiro MESH-WIDE / transporte de retracción al central** (no existe hoy para ninguna observación;
primitivo propio, va con F5 — ver R10b).
