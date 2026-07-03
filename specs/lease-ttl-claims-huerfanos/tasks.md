---
artifact: tasks
schema_version: "1.0"
change: lease-ttl-claims-huerfanos
status: archived
---

# Tareas — Lease/TTL para claims huérfanos

## Esquema
- [ ] T1 — `migrations.go`: agregar migración v4 `lease_ttl_claims` con `ADD COLUMN` de
  `owner_id`, `lease_expires_at`, `heartbeat_at`, `attempts` (DEFAULT 0), `fencing_token`
  (DEFAULT 0) + `CREATE INDEX IF NOT EXISTS idx_work_lease ON work_units(status, lease_expires_at)`.
  (R1–R3)

## Config
- [ ] T2 — `config.go`: campos `LeaseTTLSeconds` y `MaxAttempts` en `MultiAgentConfig` +
  defaults (300 / 5) en el default builder + merge `== 0 -> default`. Test de merge. (R13)

## Núcleo (work.go)
- [ ] T3 — `WorkUnit`: agregar `OwnerID`, `FencingToken`, `Attempts`, `LeaseExpiresAt`;
  actualizar los `SELECT`/`Scan` de `WorkBatchStatus` y `ClaimWorkUnit` para poblarlos. (R15)
- [ ] T4 — `ClaimWorkUnit(batchID, agent, ttlSeconds, maxAttempts)`: (a) UPDATE de
  dead-letter previo (huérfanas con `attempts >= max` → `failed`); (b) claim con subselect
  ampliado a huérfanas, seteando `owner_id`, `claimed_by`, `lease_expires_at`, `heartbeat_at`,
  `attempts+1`, `fencing_token+1`, devolviendo el token. (R4–R8) — nota: firma pública
  estable `ClaimWorkUnit(batchID, agent)`; TTL/max se pasan desde el handler o via wrapper.
- [ ] T5 — `HeartbeatWorkUnit(id, owner, fencingToken, ttlSeconds) (bool, error)`: UPDATE
  con guarda `status='claimed' AND owner_id=? [AND fencing_token=?]`; `ok=false` si 0 filas. (R9–R10)
- [ ] T6 — `CompleteWorkUnit(id, result, status, agent, fencingToken)`: sumar guarda de
  `owner_id` (canónica) y `fencing_token` opcional; error claro si expropiado. (R11–R12)

## Handler MCP
- [ ] T7 — `methods.go toolWork`: leer `fencing_token` opcional del payload; nuevo
  `action="heartbeat"` → `HeartbeatWorkUnit`; pasar TTL/maxAttempts desde `s.multiagent`;
  actualizar `claim` para devolver el token y `complete`/mensaje de action inválida. (R14)
- [ ] T8 — `registry.go`: actualizar la descripción/enum de actions de `musubi_work`
  (agregar `heartbeat`); verificar que el conteo de tools NO cambia (sigue 30). Regenerar
  golden de tools/list si la descripción es parte del snapshot. (R14)

## Tests
- [ ] T9 — `work_test.go`: escenarios de la spec — reclamo lazy de huérfana (token sube),
  no robar lease vigente, heartbeat mantiene vivo, heartbeat de expropiado (ok=false),
  fencing bloquea complete de zombie, dead-letter tras max attempts. (todos los R)

## Cierre
- [ ] T10 — `go build ./...`, `go vet ./...`, `go test ./...` verdes; golden regenerado;
  smoke efímero del ciclo claim→(expira)→reclaim→complete sobre una base temporal. (R16)
