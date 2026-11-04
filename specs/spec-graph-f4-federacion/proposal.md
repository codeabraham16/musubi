---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f4-federacion
status: draft
---

# Propuesta — federar la spec compartida al central (spec-graph · F4)

## Intención
F1 dio el destino que al `archive` le faltaba (la `SPEC.md` tipada por módulo); F2 la ancló al código;
F3 la cruzó con el grafo y la hizo recuperable por significado. Pero la spec sigue siendo **FS-native y
local a un nodo**: una escrita en la laptop es invisible desde la sala de mando y desde cualquier otra
máquina. F4 cierra esa punta con el mecanismo que Musubi YA tiene: proyecta el gist de una spec **marcada
como compartida** al cerebro central por el **mismo outbox** que federa las observaciones (scope `shared`,
gateado por destino de sync). **F4 se partió en dos PRs** (decisión del dueño, 2026-11-03): este entrega la
**federación**; el **destino del `archive`** —cablear la fase `archive` del SDD para fusionar el delta spec
de un cambio en la `SPEC.md` canónica de su módulo— va en un **PR aparte**, con su diseño preservado. Paso 4 de 5.

## Alcance
- **Incluye (F4):**
  1. **Opt-in de compartición** — un campo de frontmatter `shared: true` en el vocabulario de spec-graph
     (preservado lossless, como `anchors`). Ausente/`false` = **local, NUNCA viaja**. Es el veredicto del
     dueño (espeja el gate de corroboración de las observaciones): escribir una spec NO la comparte;
     marcarla, sí. El estado "esta spec se comparte" vive EN la spec, viaja con ella en git y es visible
     al leerla — no en una tabla paralela que se desincroniza.
  2. **Federación por outbox** — al crear/actualizar una spec `shared: true`, proyectar un **gist
     derivado** (id estable derivado del id de la spec, `topic_key` `spec/<id>`, scope `shared`, con
     `project_id`) al outbox existente → central. Reusa `enqueueOutboxTx`/`BackfillOutbox`; gateado por
     `cfg.Sync.HasDestination()` igual que las observaciones (nodo terminal marca pero no encola; siembra
     al ganar destino). La spec sigue siendo la fuente de verdad en el FS; la observación es una
     **proyección reconstruible**.
  3. **Baja y tombstone** — pasar una spec a `shared: false`, borrarla (`musubi_spec_delete`) o cambiarle
     el id **retira** su proyección del central (no deja un fantasma). Espeja la disciplina de borrado de
     las observaciones.

- **No incluye (queda para F5 o fuera, explícito):**
  - La **arbitración spec↔observación** por `musubi_judge`/`supersedes` (F5): cuando una spec compartida
    y una observación se contradicen, quién gana. F4 federa; F4 no arbitra.
  - **Sync bidireccional**: las specs se **autoran en el FS del nodo** y se proyectan una vía
    (nodo→central), como las observaciones. El central no edita specs de vuelta.
  - La **huella de anclas de F2 NO viaja** con la proyección: un fingerprint sólo vale contra el checkout
    que lo calculó (invariante de F2, se mantiene con test).
  - El **destino del `archive`** (crear/fusionar la `SPEC.md` canónica por módulo): diseñado, **diferido
    a un PR aparte** — F4 se partió en dos (federación ahora, `archive` después). El material de diseño
    (requisitos, escenarios, la función pura de merge) queda preservado para ese PR.

## Enfoque
**Reusar el carril de observación+outbox, no construir un segundo sync de specs.** La federación de specs
y la de observaciones deben compartir la MISMA cañería (outbox, dead-letter, backfill, scope, redacción
forzada del central): un segundo mecanismo duplicaría el peor filo y divergiría. La proyección es
**derivada** (reconstruible desde el FS por el mismo `BackfillOutbox`), así que no hay nueva fuente de
verdad ni riesgo de divergencia: si la proyección y la spec difieren, **gana el FS** y se re-proyecta.
**Opt-in por frontmatter, no por tool aparte:** el estado de compartición viaja con la spec en git, es
lossless y es legible al leerla — no en un side-table que se desincroniza del checkout (la misma
disciplina FS-nativa que fijó F2 para la huella de anclas).

## Impacto
- Áreas/archivos afectados: `internal/specgraph` (vocabulario: `shared` en `Fields`; helper que detecta
  el opt-in) · `internal/mcp/methods_spec.go` (create/update/delete proyectan/retiran) · `internal/memory`
  (reuso de `enqueueOutboxTx`/`BackfillOutbox`/scope; la proyección entra como observación derivada).
  CHANGELOG `[Unreleased]` + bump semántico.
- Compatibilidad: **aditivo**. Una spec sin `shared` se comporta como en F1–F3 (opt-in puro). La
  proyección derivada reusa la tabla `observations` existente — **sin tabla nueva** (o una tabla de
  mapeo `spec_projection` mínima si `design` lo pide, idempotente al estilo `observation_origins`). El
  golden de `tools/list` se regenera **sólo si** cambian params (probable: `create`/`update` ganan
  `shared`) y se re-clasifica, aunque no cambien de clase read-only/write.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Fantasma en el central si la baja no se cablea | Tombstone en delete / unshare / cambio-de-id; test que lo fija. La proyección se purga por `topic_key` `spec/*` (es derivada) |
| **Eco del sync** (re-subir lo que baja — gotcha `sync-eco-backfill-outbox`) | La proyección SÓLO nace de una spec local marcada `shared`; lo que baja del central no re-genera una `SPEC.md` ni re-proyecta. Test de no-eco |
| Fuga de una spec interna al central | **Opt-in explícito** (default local); scopeada por `project_id`; la **redacción forzada del central** ya está activa. Lo que sale es una decisión deliberada marcada en el frontmatter |
| La proyección diverge de la spec | La proyección es DERIVADA; **gana el FS** y se re-proyecta en cada apertura (`BackfillOutbox`). Nunca se autora en el central |
| Scope creep hacia F5 (arbitraje) o sync bidireccional | Excluido explícito |

## Criterio de éxito
1. Una spec con `shared: true` proyecta su gist al central por el outbox existente (scope `shared`,
   `project_id`); una **sin** el campo **nunca** encola nada — test que lo fija primero, porque lo que
   importa es qué NO sale de la máquina.
2. Pasar a `shared: false`, borrar o re-identificar una spec **retira** su proyección (tombstone), sin
   fantasma en el central.
3. En un nodo terminal (sin destino) una spec `shared` marca pero **no encola**; al ganar destino,
   `BackfillOutbox` la siembra (espeja observaciones).
4. La huella de anclas (F2) **no viaja** con la proyección (test).
5. Model-free, sin cgo, suite en verde (`GOWORK=off`), con cada invariante nuevo verificado **fallando
   bajo sabotaje** dirigido al invariante que declara.
6. La federación de specs reusa **exactamente** la cañería de observaciones (outbox / dead-letter /
   backfill / redacción): sin un segundo mecanismo de sync.

## Rollback
Aditivo. Revertir = quitar el campo `shared` del vocabulario y el hook de proyección/retiro. Las
proyecciones derivadas en `observations` se purgan por `topic_key` `spec/*` (son reconstruibles, no
fuente de verdad). Sin migración de datos que deshacer si la proyección reusa la tabla `observations`.
