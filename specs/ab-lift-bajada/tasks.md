---
artifact: tasks
schema_version: "1.0"
change: ab-lift-bajada
status: draft
---

# Tareas — Bajada del contador del A/B del decididor

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable.

## Implementación
- [ ] T1 — `MergeDecididorABDesdeCentral(rows []DecididorABRow) error`: UPSERT por `(project_id,arm)`
  con autoridad por columna (REPLACE oportunidades/exigencias, preservar cargadas, INSERT cargadas=0,
  first_at=MIN/last_at=MAX), validando `arm`. · _archivos:_ `internal/memory/decididor_ab.go`
- [ ] T2 — Handler `toolSyncPullAB`: `DecididorAB(s.scopedCtx(ctx))` → `jsonResult({"rows":...})`.
  · _archivos:_ `internal/mcp/methods.go` · _depende de:_ T1
- [ ] T3 — Registrar `musubi_sync_pull_ab` en el registry (schema sin requeridos) y clasificarla
  **read-only** (RLock). · _archivos:_ `internal/mcp/registry.go` · _depende de:_ T2
- [ ] T4 — `PullDecididorAB() ([]memory.DecididorABRow, error)` en el `SyncClient` (POST tools/call,
  sin cursor/gzip, parsea `{rows}`). · _archivos:_ `internal/mcp/syncclient.go` · _depende de:_ T3
- [ ] T5 — `bajarDecididorAB(ctx)` + cableado en `drainInboundOnce` tras el gate de ritmo, best-effort
  (sólo logea, no toca `salioALaRed`/viajes/cursor). · _archivos:_ `internal/mcp/scheduler.go`
  · _depende de:_ T4
- [ ] T6 — Regenerar el golden (`-update`): sólo `toolslist-completo.golden.json` cambia; confirmar
  que el golden del núcleo y `techoDelNucleo` NO cambian. · _depende de:_ T3

## Pruebas
- [ ] T7 — Unit del merge (`internal/memory`): I1 puebla (R1/R5), I2 idempotente (R3, sabotaje=suma),
  I3 preserva cargadas (R4, sabotaje=pisa — el clave). _Base sembrada, no magnitud auto-consumible._
- [ ] T8 — Integración (`internal/memory`): I4 (R6) — merge siembra el brazo que `BrazoAB(P,now)`
  elige, `RecordSkillEvents` sube cargadas a 1, `CalcularLift` pasa de ok=false a ok=true. Sabotaje:
  saltear el merge → ok=false. · _depende de:_ T1

## Docs / cierre
- [ ] T9 — `CHANGELOG.md` `[Unreleased] ### Added`: la bajada del contador del A/B cierra el límite
  deferido (lift medible localmente).
- [ ] T10 — Prosa del nodo spec tipado `skills-decididor`: una línea sobre que el lift cruza el sync
  por la bajada (sólo prosa a mano; frontmatter nunca a mano).
- [ ] T11 — Verificar contra la spec (R1-R9 cubiertos), `musubi_spec_validate`, y sabotaje
  adversarial de cada invariante nuevo (verify).

## Forecast de review
- Líneas estimadas: ~230 (merge ~40, handler ~20, registry ~20, client ~55, scheduler ~25, tests
  ~70). · ¿Chained PRs recomendado? No: un solo PR cohesivo (un canal de bajada end-to-end).
