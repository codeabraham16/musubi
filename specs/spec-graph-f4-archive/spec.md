---
artifact: spec
schema_version: "1.0"
change: spec-graph-f4-archive
status: draft
---

# Especificación — destino del `archive` del SDD (spec-graph · F4b)

Vocabulario RFC 2119: **DEBE** / **DEBERÍA** / **PUEDE**. Cada requisito es verificable y atómico; los
escenarios son observables (Given/When/Then).

> **Numeración.** Este es el PR del `archive`, la mitad que F4 (federación, #789) difirió reservando a
> propósito **R13–R16 / E8–E9 / D8–D9**. Se ocupan esos números para el **core del merge** y el diseño;
> la **proyección** del delta de proceso y las transversales toman números frescos (R19–R22). Los huecos
> R17/R18 (transversales de federación) y E10/E11 (escenarios de federación) se dejan **vacíos a
> propósito**, en espejo de cómo federación dejó estos vacíos: son de la otra mitad, en su propio archivo.

## Requisitos

### A — El merge canónica↔canónica (`MergeSpecInto`, el core fiel)
- **R13 (DEBE).** Fusionar un delta en un módulo **sin** `SPEC.md` canónica lo **PROMUEVE**: la canónica
  **nace del delta** y DEBE ser una spec válida — `id` y `type` presentes. Un delta sin `id` o sin `type`
  NO DEBE promoverse: es **ERROR**, no una canónica inválida que el grafo no puede navegar. La promoción
  devuelve el delta **verbatim** (lossless por identidad).
- **R14 (DEBE).** Fusionar un delta en una canónica **existente** cuyo `id` **difiere** del del delta es
  **ERROR**: `archive` fusiona la **misma** spec de módulo en su canónica, no dos specs distintas. (La vía
  lossless ni siquiera permite renombrar el `id`.) La identidad y el `status` los gobierna la canónica
  existente, nunca el delta.
- **R15 (DEBE).** El cuerpo es **aditivo y surface-not-auto**: las secciones del delta cuyo heading **no**
  existe en la canónica se **AGREGAN** al final; una sección con el **mismo** heading y cuerpo **distinto**
  se **SURFACEA** como `SectionConflict{Heading, Canon, Delta}` y la prosa de la canónica queda **intacta**.
  El merge NO DEBE auto-fusionar ni pisar prosa en conflicto.
- **R16 (DEBE).** El frontmatter de la canónica se edita **lossless** (vía `UpdateFrontmatterText`, la
  misma de F2: comentarios, orden de claves, BOM y line-ending intactos); el **cuerpo** de la canónica
  queda **byte a byte**. Nada existente se reescribe ni se reordena; sólo se agregan secciones nuevas.

### B — La proyección del delta de PROCESO del SDD (`projectSDDProcessDelta`, la política)
- **R19 (DEBE).** El `spec.md` de **proceso** del flujo SDD (frontmatter `artifact`/`schema_version`/
  `change`/`status`, **sin** `id`/`type`) **NO es** una spec canónica; antes de entrar al merge DEBE
  **proyectarse** a forma canónica:
  - (a) las claves de **proceso** (`artifact`/`schema_version`/`change`) se **PODAN** — NO DEBEN
    contaminar la canónica (**H1**);
  - (b) el `status` del delta se **quita** — NO DEBE degradar el `status` de la canónica (**H7**); la
    canónica es autoritativa sobre su lifecycle;
  - (c) en **PROMOCIÓN** (canónica inexistente) se **EXIGEN** e inyectan `id`+`type` del módulo y un
    `status` del **vocabulario de ciclo de vida** (`specgraph.SpecStatuses`); id/type ausentes o status
    fuera del vocabulario ⇒ **ERROR** (alimenta R13). En **MERGE** NO se inyecta nada (la canónica pone
    identidad y status).

  La proyección es **PURA** y vive en la **capa de tools**; `MergeSpecInto` queda **agnóstico del SDD**
  (D8). Reserializar el frontmatter del delta NO viola R16: lo lossless es la **edición de la canónica**;
  el delta de proceso es material que se **transforma**, no se conserva byte a byte.

### C — Transversales
- **R20 (DEBE).** Aditivo, **model-free**, sin cgo; suite en verde con `GOWORK=off -tags treesitter
  -count=1`; **`gofmt -l` limpio** antes de empujar (el gate que rebotó la federación, no lo ven
  `vet`/`build`/`test`). Cada invariante nuevo DEBE tener un test que **falla bajo sabotaje** dirigido al
  invariante que declara (no una mutación vacua ni el sitio equivocado — musubi-rules #8).
- **R21 (DEBE).** La tool nueva `musubi_spec_archive` se **clasifica `write`** en `registry.go` (lee,
  fusiona y **escribe** la canónica; marcar read-only algo que muta es un lost-update que `-race` no ve); el
  golden de `tools/list` **y el contrato** se regeneran (cambia la **clase** y hay **tool nueva**, no sólo
  params); se **censa** en `internal/censo` con su clase de invocación (`DebeInvocarse`, igual que sus
  nueve hermanas `musubi_spec_*`) — el invariante «toda tool viva está censada» (A3, «se construye y no se
  enciende») falla si una tool nueva no declara si debe invocarse; y el **conteo** de tools del README se
  actualiza (visible 89→90, registradas 94→95). El bloque `novedades` del README **NO** se toca: refleja el
  último release (`0.148.0`), y esta tool es `[Unreleased]`.
- **R22 (DEBE).** La tool **escribe** la `SPEC.md` canónica en el FS **cuando el merge NO tiene conflictos**
  (archivar es *persistir* la fusión: una tool que sólo devuelve el texto deja la fase `archive` sin destino
  real); la respuesta marca `written:true` y **omite** `merged` (ya está en disco). **Con conflictos NO DEBE
  escribir**: marca `written:false`, surfacea `conflicts` **y** `merged` para inspección, y la canónica en
  disco queda **intacta** (surface-not-auto a nivel tool, espejo de R15). Es la razón de la clase `write` de
  R21.

## Escenarios

**E8 — Promoción: delta de proceso → canónica válida y limpia (R13/R19, H1).**
Given no existe `SPEC.md` del módulo y un delta de proceso del SDD (`artifact`/`schema_version`/`change`/
`status:draft`, sin `id`/`type`).
When se proyecta con `id`/`type`/`status` del módulo y se promueve.
Then la canónica nace con `id`/`type`/`status` **inyectados**, **sin una sola** clave de proceso, y con el
cuerpo del delta **íntegro**; `conflicts` vacío.

**E9 — Merge: el status de proceso no degrada ni contamina, y el cuerpo es aditivo (R14/R15/R19, H7/H1).**
Given una canónica `status:active` y un delta de proceso `status:draft` con una sección nueva.
When se proyecta y se fusiona.
Then el `status` de la canónica queda **`active`** (H7), **ninguna** clave de proceso aparece en el
frontmatter (H1), la prosa de la canónica se **conserva** y la sección nueva del delta se **agrega**.

**E12 — Identidad: fusionar dos specs distintas es error (R14).**
Given una canónica con `id: spec-graph` y un delta **canónico** con `id: otra-cosa`.
When se llama `MergeSpecInto`.
Then es **ERROR** (no se fusiona a ciegas); la canónica no se toca.

**E13 — Conflicto de sección surface-not-auto (R15).**
Given una canónica y un delta con una sección del **mismo** heading pero cuerpo **distinto**.
When se fusiona.
Then el `merged` conserva la prosa de la **canónica** para ese heading, y la del delta viaja en
`conflicts` (`SectionConflict{Heading, Canon, Delta}`); nunca se pisa ni se auto-fusiona.

**E14 — Lossless del frontmatter y del cuerpo canónico (R16).**
Given una canónica con comentarios en el frontmatter, un orden de claves dado y una sección de prosa.
When se fusiona un delta que agrega una clave escalar y una sección nueva.
Then los comentarios, el orden de las claves preexistentes y el cuerpo canónico quedan **intactos**; sólo
se refleja la clave nueva y se **agrega** la sección nueva.

**E15 — Promoción inválida (R13/R19).**
Given un delta de proceso sin `id`/`type` de módulo.
When se proyecta para promoción sin pasar `id`+`type` (o con un `status` fuera del vocabulario).
Then es **ERROR** con un mensaje que nombra la causa; no se produce ninguna canónica.

**E16 — La tool escribe si está limpio y surfacea sin escribir si hay conflicto (R22, borde tool).**
Given el delta de proceso de un cambio y la `SPEC.md` canónica de su módulo en el FS.
When se invoca `musubi_spec_archive` por el camino MCP (`handleToolsCall`).
Then si el merge no tiene conflictos la canónica en disco **cambia** (`written:true`, sin `merged` en la
salida); si una sección choca, el disco **no cambia** (`written:false`) y `conflicts`+`merged` viajan en la
respuesta.

## Fuera de alcance (recordatorio, de la propuesta)
La **federación** (`shared`, outbox, central) — es #789, la otra mitad de F4 · **arbitración
spec↔observación** (F5) · **auto-fusión** de prosa en conflicto (surface-not-auto por diseño) ·
**reescritura/reordenamiento** de la canónica (el merge es aditivo y lossless).
