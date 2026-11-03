---
artifact: tasks
schema_version: "1.0"
change: skills-medir-cargas
status: draft
---

# Tareas — Medir cuándo el agente CARGA una skill

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable. Marcá `[x]` al completar.

## Implementación
- [ ] T1 — `const UsoCargada = "loaded"` y `kindValido` lo acepta · _archivos:_ `internal/memory/skillusage.go` · _cubre:_ R2
- [ ] T2 — El reporte muestra `loaded` junto a los otros contadores, **sin** llamarlo utilidad · _archivos:_ `internal/memory/skillusage.go` (`SkillUsage`, `FormatSkillUsage`), `internal/mcp/methods*.go` (`toolSkillUsage`) · _depende de:_ T1 · _cubre:_ R9, R11
- [ ] T3 — Subcomando `skill-loaded`: `leerEventoSkillLoaded(stdin)` (decisión pura) + `runSkillLoaded()` (efecto: respeta `MUSUBI_SKILLLOAD_HOOK=0`, abre la base best-effort, `RecordSkillEvents` con `{Skill, ProjectID, Evidence:"", Kind:UsoCargada}`) · _archivos:_ `cmd/musubi/skill_loaded.go` (nuevo) · _depende de:_ T1 · _cubre:_ R1, R3, R4, R5, R6
- [ ] T4 — Registrar el subcomando `skill-loaded` en el dispatcher · _archivos:_ `cmd/musubi/main.go` · _depende de:_ T3
- [ ] T5 — Sumar `{"PostToolUse","Skill","skill-loaded --hook-mode"}` a `ganchosDelAgente`; que setup/provision lo escriban · _archivos:_ `cmd/musubi/agente_plugin.go`, `setup.go`, `provision.go` · _depende de:_ T3 · _cubre:_ R7, R8

## Pruebas
- [ ] T6 — `skillusage_test.go`: `kindValido("loaded")` true; `RecordSkillEvents` suma un `loaded`; un `kind` basura al lado de un `loaded` se saltea sin romper el lote · _cubre:_ R2, R3
- [ ] T7 — `skill_loaded_test.go` (nuevo): `leerEventoSkillLoaded` aplica sólo con `tool_name="Skill"` y `skill` no vacía; tool ajena → no; skill vacía → no; `MUSUBI_SKILLLOAD_HOOK=0` → no · _cubre:_ R4, R5, D5
- [ ] T8 — `hook_contrato_test.go`: el contrato incluye `PostToolUse(Skill)→skill-loaded`; `TestElPluginLlevaLosMismosGanchosQueSetup` sigue verde · _cubre:_ R7
- [ ] T9 — Custodio mcp: `musubi_skill_usage` muestra `loaded` acotado al proyecto de la credencial (molde de los tests A7–A9) · _cubre:_ R9, R11

## Docs / cierre
- [ ] T10 — `CHANGELOG.md` (`[Unreleased]`): señal nueva `loaded` observable en `musubi_skill_usage`; opt-in por instalación
- [ ] T11 — Verificar contra la spec: cada R# cubierto; **sin** cambio en `migrations.go` (R10); `go vet` + `build` + `test -race -count=1` (puerta de go-hygiene, no cacheado)

## Forecast de review
- Líneas estimadas: ~300 (subcomando ~90, tests ~150, resto chico) · ¿Chained PRs recomendado? **No** — una feature, un PR.
