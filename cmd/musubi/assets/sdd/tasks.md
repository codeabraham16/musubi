---
artifact: tasks
schema_version: "1.0"
change: <nombre-del-cambio>
status: draft
---

# Tareas — <título>

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable
(idealmente ≤ ~400 líneas de diff). Marcá `[x]` al completar.

## Implementación
- [ ] T1 — <tarea concreta> · _archivos:_ `<ruta>`
- [ ] T2 — <tarea concreta> · _depende de:_ T1

## Pruebas
- [ ] T3 — <test que cubre R1/R2 de la spec>

## Docs / cierre
- [ ] T4 — Actualizar `CHANGELOG.md` (sección `[Unreleased]`) si es visible al usuario
- [ ] T5 — Verificar contra la spec (cada requisito R# tiene cobertura)

## Forecast de review
- Líneas estimadas: <n> · ¿Chained PRs recomendado? <sí/no>
