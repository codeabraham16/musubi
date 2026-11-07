---
artifact: design
schema_version: "1.0"
change: ab-lift-bajada
status: draft
---

# Diseño técnico — Bajada del contador del A/B del decididor

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Una **tool MCP dedicada** (`musubi_sync_pull_ab`) read-only sirve el snapshot `decididor_ab` scoped al tenant | (a) Piggyback en `musubi_sync_pull`; (b) push desde el central | El pull de observaciones es paginado y cursoreado; adosarle un snapshot lo mandaría en cada página y mezclaría dos cursores. El inbound es pull por diseño (local-first), no push. Una tool aparte es el canal más limpio y es la única forma de que el `SyncClient` lea datos del central (el único endpoint es `tools/call`). |
| D2 | **Merge con autoridad por columna**: central REPLACE sobre `oportunidades`/`exigencias`; local preserva `cargadas` | (a) REPLACE de toda la fila; (b) sumar los contadores | (a) borraría el numerador local en cada bajada. (b) no es idempotente: re-bajar duplica. El central es la ÚNICA fuente de `oportunidades` (nace de `RegistrarOportunidadDecididor`, que sólo corre con motor real) y la local la única de `cargadas` (nace de `bumpCargadasDecididorTx`): cada columna tiene un dueño, el merge lo respeta. |
| D3 | Fila nueva nace con `cargadas = 0` | heredar `cargadas` del central | En la topología actual el central puede tener su propio `cargadas` (cargas de sesiones del central). Heredarlo contaminaría el numerador local con cargas de otra máquina. `cargadas=0` dice «esta máquina aún no contó» y es coherente con el lift per-máquina. |
| D4 | El pull del contador corre en `drainInboundOnce`, tras el gate de ritmo, como helper aparte que **no toca `salioALaRed`/`sync_viajes`/cursor** | contarlo como un viaje más | El contador es diminuto y cambia lento; contarlo perturbaría el ritmo (que se calibra con filas de observación) y la contabilidad de viajes. Aislarlo mantiene intacta la maquinaria afinada de la bajada de observaciones y acota el blast radius. |
| D5 | Sólo BAJADA; cero vías de subida | subir `cargadas` al central para un lift global | Subir reintroduce el eco («re-sube lo que baja») y abre el problema de agregación cross-máquina. El usuario eligió explícitamente el lift local. Sin vía de subida, no hay eco posible (R7 es estructural). |
| D6 | El merge vive en `internal/memory` (`MergeDecididorABDesdeCentral`), al lado de `IngestShared` | en `internal/mcp` | El UPSERT es lógica de persistencia pura (SQL), como `IngestShared`. `memory` es el piso común y ya tiene el DSN `_txlock=immediate` y el scope. |

## Enfoque de implementación
Flujo de datos (sólo descendente):

```
CENTRAL                                   LOCAL (team mode, dueño del candado de bajada)
decididor_ab ──(musubi_sync_pull_ab,      drainInboundOnce
  scopedCtx: DecididorAB(ctx))──────────▶   └─ s.bajarDecididorAB(ctx)  [best-effort, tras el gate de ritmo]
                                                 ├─ rows := s.syncClient.PullDecididorAB()
                                                 └─ s.engine.MergeDecididorABDesdeCentral(rows)
                                                       (UPSERT por (project_id,arm), autoridad por columna)
```

Componentes:
- **`internal/mcp/registry.go`** — registra `musubi_sync_pull_ab`: schema sin argumentos requeridos,
  clasificada **read-only** (toma RLock en el dispatch; no muta la base del central).
- **`internal/mcp/methods.go`** — `toolSyncPullAB(ctx, raw)`: llama
  `s.engine.DecididorAB(s.scopedCtx(ctx))` y devuelve `{"rows": [...]}` vía `jsonResult`. El scope lo
  pone `scopedCtx` (igual que `toolSyncPull`): el central sólo entrega el tenant de la credencial.
- **`internal/mcp/syncclient.go`** — `PullDecididorAB() ([]memory.DecididorABRow, error)`: espeja la
  estructura de `Pull` (POST `tools/call` con `name:"musubi_sync_pull_ab"`), pero sin cursor ni
  medición gzip (payload mínimo). Parsea `content[0].text` → `{rows}`.
- **`internal/memory/decididor_ab.go`** — `MergeDecididorABDesdeCentral(rows []DecididorABRow) error`:
  por fila, un `INSERT … ON CONFLICT(project_id,arm) DO UPDATE` con autoridad por columna.
- **`internal/mcp/scheduler.go`** — `bajarDecididorAB(ctx)` llamado desde `drainInboundOnce` tras
  `tocaIr`, envuelto para que su error sólo logee.

## Contratos / interfaces
Tool (wire): `musubi_sync_pull_ab`, sin argumentos. Respuesta en `content[0].text`:
```json
{"rows": [{"project_id":"P","arm":"on","oportunidades":10,"exigencias":4,"cargadas":0,
           "first_at":"...","last_at":"..."}]}
```
(`DecididorABRow` ya tiene tags JSON explícitos — musubi-rules #6.)

Cliente: `func (c *SyncClient) PullDecididorAB() ([]memory.DecididorABRow, error)` — error transitorio
ante red/HTTP/RPC (best-effort: el scheduler lo trata como reintentable).

Merge (SQL, autoridad por columna):
```sql
INSERT INTO decididor_ab (project_id, arm, oportunidades, exigencias, cargadas, first_at, last_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
ON CONFLICT(project_id, arm) DO UPDATE SET
  oportunidades = excluded.oportunidades,   -- REPLACE (idempotente), central = dueño
  exigencias    = excluded.exigencias,      -- REPLACE, central = dueño
  -- cargadas NO se nombra: la local es dueña del numerador (R4)
  first_at = MIN(decididor_ab.first_at, excluded.first_at),
  last_at  = MAX(decididor_ab.last_at,  excluded.last_at)
```
`arm` se valida (`BrazoOn`/`BrazoOff`) por defensa, igual que `RegistrarOportunidadDecididor`.

## Trade-offs
- **Se gana:** el lift se vuelve medible en la máquina, cerrando el límite deferido; idempotente;
  sin eco; blast radius acotado (un helper best-effort, cero cambios a la maquinaria de observaciones).
- **Se cede:** el lift es per-máquina (oportunidades central-wide ÷ cargadas de esta máquina) — señal
  direccional, no tasa absoluta; y es prospectivo (las cargas previas a la primera bajada no se
  recuperan). Ambos aceptados y documentados.

## Plan de pruebas
- **Unit del merge (`internal/memory`)** sobre base sembrada (nunca una magnitud auto-consumible):
  - I1/R1-R5: merge de un snapshot puebla la fila con oportunidades/exigencias del central y cargadas=0.
    Sabotaje: el `DO UPDATE` no escribe `oportunidades` → la fila queda en 0 → rojo.
  - I2/R3: merge ×2 del mismo snapshot deja oportunidades sin duplicar. Sabotaje: cambiar REPLACE por
    `oportunidades + excluded.oportunidades` → segundo merge duplica → rojo.
  - I3/R4: fila local con cargadas=5; merge con cargadas=0 del central; cargadas sigue 5. Sabotaje:
    agregar `cargadas = excluded.cargadas` al `DO UPDATE` → se pisa a 0 → rojo. (El invariante clave.)
- **Integración (`internal/memory`)** I4/R6: merge siembra la fila del brazo que `BrazoAB(P, now)`
  elige (el test llama a `BrazoAB` para decidir qué sembrar), luego `RecordSkillEvents` con una carga
  de `P` → cargadas pasa a 1 y `CalcularLift` pasa de ok=false a ok=true. Sabotaje: saltear el merge →
  sin fila, el UPDATE-only no afecta nada → cargadas=0, ok=false → rojo.
- **Golden:** `go test ./internal/mcp -run TestToolsListGolden -update` → sólo
  `toolslist-completo.golden.json` cambia (la tool nueva); el golden del núcleo NO cambia (la tool no
  es del núcleo) y `techoDelNucleo` NO se toca.
- **R7/R8/R9** se verifican por construcción + revisión adversarial en verify: no hay vía de subida
  (grep del outbox), el helper sólo logea su error, y corre dentro del candado de `drainInboundOnce`.
