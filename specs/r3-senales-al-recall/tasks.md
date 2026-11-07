---
artifact: tasks
schema_version: "1.0"
change: r3-senales-al-recall
status: draft
---

# Tareas — Despertar el corte R3: señales declaradas al recall

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable. Marcá `[x]` al completar.

## Implementación
- [x] T1 — Agregar `ModifiedFiles []string` (`json:"modified_files"`), `Phase`, `Task` al struct de
  args de `toolRecall`, con comentario que explique el porqué · _archivos:_ `internal/mcp/methods.go`
- [x] T2 — Reemplazar el `reflejas=nil` del TRAMO 3b: armar `skills.ResolveRequest` de los args y, si
  `DeclaraAlgo()` y arsenal no vacío, resolver con `NewResolverDeCatalogo(arsenal).ResolveConDetalle`;
  pasar el resultado a `exigirSkillSiCorresponde` · _depende de:_ T1 · _archivos:_ `internal/mcp/methods.go`
- [x] T3 — Agregar `modified_files`/`phase`/`task` a las Properties de `musubi_recall` en el schema ·
  _archivos:_ `internal/mcp/registry.go`
- [x] T4 — Regenerar el golden: `GOWORK=off go test ./internal/mcp -run TestToolsListGolden -update`;
  revisar el diff (sólo los 3 params nuevos bajo `musubi_recall`) · _depende de:_ T3

## Pruebas
- [x] T5 — Test: señal fuerte por glob apaga el motor (S5). Arsenal con skill `Triggers:["*.go"]`,
  resolver de catálogo inyectado, `toolRecall` con `modified_files:["x.go"]` ⇒ `motor.llamadas==0`.
  Sabotaje: `reflejas=nil` en el call-site ⇒ 1 ⇒ rojo · _archivos:_ `internal/mcp/decididor_skills_test.go`
- [x] T6 — Test: match por alcance apaga el motor (S5). Skill `AppliesTo:[FaseImplementar]`, recall con
  `phase:"phase:implementing"` ⇒ `motor.llamadas==0`
- [x] T7 — Test: sin señales, el motor corre (S3). Brazo on (`pidDeBrazo`), recall sin señales ⇒
  `motor.llamadas==1`. Sabotaje: poblar `reflejas` sin señales ⇒ 0 ⇒ rojo
- [x] T8 — Test: señal que no casa, el motor corre (S5 inverso). Skill `Triggers:["*.py"]`, recall con
  `modified_files:["x.go"]`, brazo on ⇒ `motor.llamadas==1`
- [x] T9 — Test de regresión del contrato (S1/S3): recall sólo con `query` ⇒ comportamiento de hoy

## Docs / cierre
- [x] T10 — `CHANGELOG.md` sección `[Unreleased]`: una viñeta (el corte R3 ahora es alcanzable por
  señales declaradas en `musubi_recall`)
- [x] T11 — Actualizar la prosa del nodo tipado `specs/skills-decididor/spec.md` (Interface): el input
  del gate `hayMatchFuerte` llega de las señales de `musubi_recall`. A mano (prosa), sin tocar frontmatter
- [x] T12 — `musubi_spec_validate` sin colgantes; gate `GOWORK=off go vet/build/test -race -count=1`
  en `./internal/mcp/... ./internal/skills/...`; sabotaje verificado a mano en T5 y T7

## Forecast de review
- Líneas estimadas: ~180 (código ~40 + tests ~120 + golden/docs ~20). ¿Chained PRs? No: un solo PR,
  un solo invariante nuevo (el corte alcanzable), dimensión acotada.
