---
artifact: tasks
schema_version: "1.0"
change: memoria-bitemporal
status: archived
---

# Tareas — Invalidación bi-temporal del grafo de hechos

## Esquema
- [ ] T1 — `migrations.go`: migración v5 `relations_bitemporal` — `ADD COLUMN` de
  `valid_from`/`valid_to`/`invalidated_at` (DATETIME) y `superseded_by` (INTEGER);
  backfill `valid_from = created_at`; `CREATE INDEX idx_rel_live ON relations(from_id,
  predicate, invalidated_at)`. Idempotente por user_version. (R1–R4)

## Config
- [ ] T2 — `config.go`: `SingleValuedPredicates []string` en `GraphConfig` + default curado
  (ES+EN) en el builder + merge `len==0 -> default`. Test de merge. (R5)

## Núcleo (graph.go)
- [ ] T3 — `SaveFact(subject, predicate, object, validFrom string, singleValued []string)`:
  parsear `validFrom` (ISO, fallback now); UPSERT con `ON CONFLICT DO UPDATE` que revive
  (limpia invalidación) y setea `valid_from`; si el predicado ∈ singleValued (case-insensitive),
  `UPDATE ... SET valid_to=now, invalidated_at=now, superseded_by=:newID WHERE from_id=:S
  AND predicate=:P AND to_id!=:O AND invalidated_at IS NULL`; devolver `{Created, Invalidated}`.
  Todo en la tx existente. (R7–R12)
- [ ] T4 — `RecallFacts(entity, maxHops, maxFacts int, asOf string)`: construir el fragmento
  WHERE temporal (default `r.invalidated_at IS NULL`; con asOf válido el filtro point-in-time);
  pasar fragmento + args a `expandFrontier` para que cada hop lo aplique; asOf inválido →
  degradar a verdad actual. (R13–R15)

## Interfaz + handlers
- [ ] T5 — `backend.go`: actualizar firmas de `SaveFact`/`RecallFacts` en `GraphStore`.
  `methods.go`: `toolSaveFact` lee `valid_from` opcional, pasa `s.graph.SingleValuedPredicates`,
  reporta invalidados; `toolRecallFacts` lee `as_of` opcional. `registry.go`: describir
  `valid_from`/`as_of` en los input schemas (sin tools nuevas → conteo intacto). Regenerar
  golden si aplica. (R16)

## Tests
- [ ] T6 — `graph_test.go`: single-valued invalida al anterior; many-valued no invalida;
  point-in-time con `as_of`; revivir triplete invalidado; backfill deja lo previo vigente. (todos los R)

## Cierre
- [ ] T7 — `go build ./...`, `go vet ./...`, `go test ./...` verdes; golden regenerado si
  cambió; verificar retrocompatibilidad (RecallFacts sobre hechos pre-migración). (R17)
