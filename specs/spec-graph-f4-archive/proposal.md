---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f4-archive
status: draft
---

# Propuesta — cablear el destino del `archive` del SDD (spec-graph · F4b)

## Intención
F1 le dio a la fase `archive` del SDD su destino en papel (la `SPEC.md` tipada por módulo); F2 la ancló
al código; F3 la cruzó con el grafo. Pero el `archive` **sigue sin destino real**: hoy la directiva de la
fase sólo aconseja «guardá el aprendizaje» (`internal/memory/sdd.go:106`) y **ningún** código fusiona el
delta spec de un cambio (`specs/<change>/spec.md`) en la `SPEC.md` canónica de su módulo. F4 se **partió
en dos PRs** (decisión del dueño, 2026-11-03: *«federación ya, archive aparte»*): la **federación** salió
en #789; **este PR entrega el destino del `archive`**, con el material de diseño que aquel difirió
(R13–R16 / E8–E9 / D8–D9). Es la mitad **FS-local, sin sync**: no comparte una línea de código con la
federación. Paso 4 (b) de 5.

Además, este PR **cierra el bloqueante 2 de la review adversarial de F4** (`review/spec-graph-f4-federacion`,
debate `dfa7f90d`): confirmado leyendo el frontmatter real del `spec.md` del propio flujo SDD, el delta de
proceso trae `artifact`/`schema_version`/`change`/`status` y **no** `id`/`type`, así que fusionarlo crudo
**contamina** la canónica con las claves de proceso (**H1**) y **degrada** su `status` (**H7**). El camino
por defecto del SDD fallaba o ensuciaba. Y `archive` era la **única de 10 tools de spec con CERO tests**.

## Alcance
- **Incluye (F4b):**
  1. **Merge puro canónica↔canónica** — `MergeSpecInto(canonical, delta)` en `internal/specgraph`
     (`archive.go`): **model-free, sin I/O** (el caller lee y escribe el FS), **surface-not-auto** (la
     prosa en conflicto se devuelve, no se pisa), **lossless** (el frontmatter canónico se edita por la
     vía de F2; su cuerpo queda byte a byte; sólo se **agregan** secciones). Hereda de F2 «marca, nunca
     auto-escribe». _Ya implementado y validado en la ronda 1 de la review._
  2. **Proyección del delta de PROCESO del SDD** — `projectSDDProcessDelta(...)` en `internal/mcp`
     (`spec_archive.go`): adapta el `spec.md` de proceso a forma canónica **ANTES** del merge —poda las
     claves de proceso (fija **H1**), no deja que el `status` de proceso degrade la canónica (fija **H7**),
     y en promoción **exige e inyecta** `id`+`type` del módulo con un `status` del vocabulario de ciclo de
     vida—. Es la **política SDD**; vive en la capa de tools, dejando `MergeSpecInto` agnóstico del SDD.
  3. **Tool `musubi_spec_archive` + directiva de la fase** — la tool lee el delta y la canónica del FS,
     proyecta→fusiona, y devuelve el texto fusionado más las secciones en conflicto para que las
     reconcilie una persona. La directiva de la fase `archive` (`sdd.go`) pasa de sólo-aconsejar a
     invocarla. `toolSDD` **no se toca**.

- **No incluye (explícito):**
  - La **federación** (campo `shared`, proyección al central por el outbox): es la **otra mitad** de F4,
    entregada en #789. Este PR no comparte código con ella.
  - La **arbitración spec↔observación** (F5): cuando una spec y una observación se contradicen, quién
    gana. El `archive` fusiona delta↔canónica; no arbitra contra la memoria.
  - **Auto-fusión de prosa en conflicto**: donde dos secciones con el mismo heading divergen, el merge
    **surfacea** el conflicto y conserva la canónica; NO inventa un merge de 3 vías (sería texto que nadie
    escribió). La reconciliación la decide una persona.
  - **Reescritura o reordenamiento de la canónica**: el merge es aditivo y lossless; nada existente se
    reescribe.

## Enfoque
**El core fiel, la política en las tools (frontera de F1, D8).** `MergeSpecInto` es un merge
canónica↔canónica **puro** y reusable: no sabe nada del SDD. El conocimiento de que «el `spec.md` del SDD
es un diario de proceso, no una spec» es **política**, y vive en la capa de tools como una proyección que
transforma el delta ANTES de entrar al merge. Así el fix de H1/H7 no mete una sola rama de SDD en el core,
y `MergeSpecInto` sirve para cualquier merge de specs, no sólo el del SDD. **Aditivo**: todo es código
nuevo; no se toca una línea existente salvo la directiva de la fase y el registro de la tool nueva.

## Impacto
- Áreas/archivos: `internal/specgraph/archive.go` (+`archive_test.go`) · `internal/mcp/spec_archive.go`
  (+`spec_archive_test.go`) · `internal/mcp/methods_spec.go` (tool nueva) · `internal/mcp/registry.go`
  (clasificación `write`) · `internal/memory/sdd.go` (directiva de la fase) · golden de `tools/list` +
  **contrato** (clase nueva, tool nueva) · README (conteo de tools) · CHANGELOG `[Unreleased]` + bump MINOR.
- Compatibilidad: **aditivo**. Sin invocar la tool, el SDD se comporta como hoy. La tool nueva **sí**
  cambia el golden y la clase (es `write`): regenerar golden + contrato, no sólo params.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| **H1 — el delta de proceso contamina la canónica** (`artifact`/`schema_version`/`change`) | La proyección **poda** esas claves ANTES del merge (denylist `sddProcessKeys`); test saboteable que las detecta en el `merged` |
| **H7 — el `status:draft` del proceso degrada el `status:active` de la canónica** | La proyección **quita** `status` del delta; la canónica es autoritativa sobre su lifecycle; test que fija que `active` sobrevive un delta `draft` |
| Promover una canónica **inválida** (sin id/type, o status basura) | La proyección **exige** id+type del módulo y **valida** el status contra `specgraph.SpecStatuses`; faltante o inválido ⇒ **error**, no una canónica inválida silenciosa (R13) |
| Fusionar **dos specs distintas** por error | Guarda de identidad: un delta con `id` ≠ al de la canónica ⇒ **error** (R14) |
| El merge **pisa** prosa divergente en silencio | Surface-not-auto: la sección en conflicto va en `conflicts`, la canónica queda intacta (R15); test que lo fija |
| Una clave de proceso **nueva** del SDD no se podaría (la poda es denylist) | **Residual declarado** (D10): el vocabulario de proceso es chico, cerrado y estable; la denylist es una `var` nombrada (un único sitio), y crece con él si crece |
| `archive` era la tool de spec **sin tests** (12 sabotajes quedaban verdes) | Este PR le da cobertura saboteable: core (`archive_test.go`) + proyección (5 tests) + tool al cablear (R20) |

## Criterio de éxito
1. Promover un delta de proceso a una canónica inexistente produce una `SPEC.md` **válida** (id/type/status
   del módulo) y **sin una sola clave de proceso** — test primero, porque es el camino por defecto del SDD hoy.
2. Fusionar un delta `status:draft` en una canónica `status:active` **no degrada** el status y **no
   contamina** el frontmatter; la sección nueva del delta se agrega; una homónima divergente se **surfacea**.
3. Promover sin id/type del módulo, o con un status fuera del vocabulario, es **error**; fusionar un delta
   con id ≠ al de la canónica es **error**.
4. El frontmatter y el cuerpo de la canónica son **lossless** (comentarios/orden/BOM/EOL intactos; cuerpo
   byte a byte; sólo se agregan secciones).
5. Model-free, sin cgo, suite en verde (`GOWORK=off -tags treesitter -count=1`), `gofmt -l` **limpio**
   antes de empujar (el gate que rebotó la federación), con cada invariante nuevo verificado **fallando
   bajo sabotaje** dirigido al invariante que declara.
6. La tool nueva se clasifica `write`, el golden + **contrato** se regeneran y el conteo de tools del
   README se actualiza (no marcar read-only algo que muta: lost-update que `-race` no ve).

## Rollback
Aditivo. Revertir = quitar `archive.go`/`spec_archive.go` y sus tests, des-registrar `musubi_spec_archive`
(golden + contrato) y volver la directiva de la fase `archive` a sólo-aconsejar. Sin migración de datos:
el merge es una función del FS, no escribe tablas.
