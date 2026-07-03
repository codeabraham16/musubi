---
artifact: design
schema_version: "1.0"
change: lease-ttl-claims-huerfanos
status: archived
---

# Diseño — Lease/TTL para claims huérfanos

## Decisión 1 — Reciclado lazy vía ampliación del `WHERE`, sin goroutine barredora
El claim ya es un `UPDATE...RETURNING` con subselect que elige la próxima unidad `open`.
Cambiamos el subselect para que también elija una unidad **huérfana**:
`status='open' OR (status='claimed' AND lease_expires_at IS NOT NULL AND lease_expires_at < now)`.
Así una unidad huérfana es, a efectos del claim, tan reclamable como una `open`, y el
reciclado ocurre exactamente cuando alguien quiere trabajo — cero procesos de fondo, cero
concurrencia nueva.
**Rationale:** una unidad huérfana solo importa si hay demanda; un sweeper eager gastaría
ciclos y agregaría una goroutine con su propio locking. `work_units` vive en una única base
SQLite single-writer (WAL + busy_timeout), así que el `UPDATE...RETURNING` sigue siendo
atómico sin `SKIP LOCKED`.
**Descartado:** goroutine con `time.Ticker` que expire leases (más código, más riesgo de
carrera, sin beneficio observable — la unidad huérfana no molesta a nadie hasta que se la
quiere reclamar).

## Decisión 2 — Dead-letter como statement determinista PREVIO, no dentro del claim
`ClaimWorkUnit` ejecuta, **antes** del UPDATE de claim y en la misma llamada Go, un UPDATE
de barrido acotado:
```
UPDATE work_units
   SET status='failed',
       result = COALESCE(NULLIF(result,''), 'lease agotado: superó el máximo de reintentos'),
       updated_at = datetime('now')
 WHERE status='claimed'
   AND lease_expires_at IS NOT NULL AND lease_expires_at < datetime('now')
   AND attempts >= :maxAttempts
   [AND batch_id = :batch]      -- solo si se pasó batch
```
Recién después corre el claim (Decisión 1), cuyo subselect ya no verá esas unidades porque
pasaron a `failed`. Como la base es single-writer, no hay ventana de interleaving entre los
dos statements.
**Rationale:** meter la lógica de dead-letter dentro del subselect del claim
(`CASE`/condicional que a veces recicla y a veces mata) vuelve el SQL ilegible y difícil de
testear. Dos statements chicos y claros, cada uno atómico, son más simples y igual de
correctos aquí.
**Descartado:** un único statement mágico; y también un dead-letter perezoso "en el próximo
status" (dejaría unidades muertas invisibles ocupando el conteo `claimed`).

## Decisión 3 — Fencing token de primera clase; `owner_id` como baseline, token como defensa
El claim incrementa `fencing_token` y lo **devuelve** en la `WorkUnit`. `HeartbeatWorkUnit`
y `CompleteWorkUnit` aceptan un `fencingToken int64`:
- Verificación baseline (siempre): `status='claimed' AND owner_id = :caller`.
- Verificación de fencing (si `fencingToken > 0`): además `AND fencing_token = :fencingToken`.
`owner_id` ya defiende el caso de agentes con id distinto; el **fencing token es
imprescindible cuando dos workers comparten el mismo id** (p. ej. ambos `"worker"`): A
reclama (token=1), se cuelga, B —también `"worker"`— lo expropia (token=2, `owner_id`
sigue `"worker"`), A revive y llama `complete` con `owner="worker"` → el chequeo de owner
pasaría, pero el de token (A tiene 1, vigente es 2) afecta **0 filas**. Correcto.
`fencingToken <= 0` salta el chequeo (retrocompat para llamadores que no lo pasen todavía).
**Rationale:** los ids de agente no son garantizados únicos; el token monótono es la única
defensa robusta contra el zombie. Hacerlo opcional preserva compatibilidad sin debilitar el
default (la tool lo devuelve y la directiva recomienda pasarlo).

## Decisión 4 — Tiempo: única fuente `datetime('now')` de la base
Todos los cálculos de lease usan `datetime('now')` / `datetime('now','+'||:ttl||' seconds')`
de la propia base (UTC, formato ISO `YYYY-MM-DD HH:MM:SS`), nunca un reloj de cliente. La
comparación `lease_expires_at < datetime('now')` es correcta lexicográficamente por el
formato ISO. Coincide con el `CURRENT_TIMESTAMP` que ya usa `updated_at`.
**Rationale:** single-writer + un solo reloj elimina de raíz el problema de relojes
desincronizados; no hace falta NTP ni skew budget.

## Decisión 5 — Config y defaults
Se agregan a `MultiAgentConfig`:
- `LeaseTTLSeconds int` (default **300** = 5 min). El trabajo de un sub-agente puede tardar
  minutos (a diferencia de los ~30 s de SQS); 5 min da margen con heartbeats espaciados.
- `MaxAttempts int` (default **5**). Tras 5 reclamos fallidos, dead-letter.
Ambos con el patrón de merge existente (`== 0 -> default`). `LeaseTTLSeconds<=0` o
`MaxAttempts<=0` caen al default en el punto de uso.

## Decisión 6 — Compatibilidad de datos y `claimed_by`
`claimed_by` (columna histórica) se conserva y se sigue seteando junto con `owner_id` (mismo
valor) para no romper filas ni lecturas viejas; `owner_id` es la columna canónica nueva.
Una unidad preexistente reclamada bajo el esquema viejo tiene `lease_expires_at IS NULL` →
la condición `lease_expires_at IS NOT NULL AND lease_expires_at < now` la trata como **no
huérfana** (no se recicla hasta que se la reclame bajo el esquema nuevo, que ya setea lease).
Esto evita expropiar trabajo en curso durante el upgrade.

## Firma de las funciones (contrato Go)
```go
// devuelve la unidad con FencingToken poblado
func (e *DbEngine) ClaimWorkUnit(batchID, agent string) (WorkUnit, bool, error)
// renueva el lease; ok=false si fuiste expropiado/no sos el dueño
func (e *DbEngine) HeartbeatWorkUnit(id, owner string, fencingToken int64) (bool, error)
// cierra como dueño; error si expropiado o token viejo
func (e *DbEngine) CompleteWorkUnit(id, result, status, agent string, fencingToken int64) error
```
`WorkUnit` gana `OwnerID string`, `FencingToken int64`, `Attempts int`, `LeaseExpiresAt string`.
La firma de `CompleteWorkUnit` cambia (agrega `fencingToken`); se actualizan sus llamadores
(handler MCP + tests). El handler MCP lee `fencing_token` opcional del payload.

## Alternativas globales descartadas
- **Cola externa (Redis/SQS real):** rompe local-first y agrega un servicio; el visibility
  timeout se replica trivial en SQLite.
- **Reintento con backoff exponencial:** innecesario para un TTL fijo; se puede sumar luego
  sin cambiar el esquema.
- **Borrar la unidad huérfana y recrearla:** perdería historial (`attempts`, `seq`) y el
  fencing token; reciclar in-place es más auditable.
