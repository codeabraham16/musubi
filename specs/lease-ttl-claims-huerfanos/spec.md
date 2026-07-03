---
artifact: spec
schema_version: "1.0"
change: lease-ttl-claims-huerfanos
status: archived
---

# Especificación — Lease/TTL para claims huérfanos

## Requisitos

### Esquema (migración v4)
- **R1** — La migración DEBE agregar a `work_units` las columnas: `owner_id` (TEXT),
  `lease_expires_at` (DATETIME), `heartbeat_at` (DATETIME), `attempts` (INTEGER NOT NULL
  DEFAULT 0) y `fencing_token` (INTEGER NOT NULL DEFAULT 0). DEBE ser aditiva
  (`ADD COLUMN`) y NO DEBE tocar filas existentes salvo por los defaults.
- **R2** — La migración DEBE crear un índice que soporte la búsqueda de unidades
  reclamables por estado + vencimiento (p. ej. sobre `(status, lease_expires_at)`).
- **R3** — La migración DEBE ser idempotente vía el runner de `user_version` (correrla dos
  veces no DEBE fallar ni duplicar columnas).

### Claim con reciclado lazy (`ClaimWorkUnit`)
- **R4** — El claim DEBE seleccionar como reclamable una unidad que esté `open`, **o** que
  esté `claimed` con `lease_expires_at` **estrictamente menor** al `now` de la base
  (huérfana). Una unidad `claimed` con lease vigente NO DEBE ser reclamable.
- **R5** — Al reclamar, en el **mismo** `UPDATE...RETURNING` atómico, DEBE: fijar
  `status='claimed'`, `owner_id` al agente que reclama, `lease_expires_at = now + TTL`,
  `heartbeat_at = now`, incrementar `attempts` en 1 e incrementar `fencing_token` en 1.
- **R6** — El claim DEBE devolver el `fencing_token` resultante junto con la unidad, para
  que el dueño lo use en operaciones posteriores.
- **R7** — Con `attempts` ya en el máximo configurado, una unidad huérfana NO DEBE
  reciclarse: DEBE pasar a `failed` (dead-letter) con un `result` que indique agotamiento
  de reintentos, y NO DEBE devolverse como reclamada. (PUEDE resolverse en el claim o en un
  paso previo determinista dentro de la misma llamada.)
- **R8** — El claim DEBE preservar el orden de servicio existente (FIFO por
  `created_at, seq, rowid` para batch vacío; por `seq` dentro de un batch).

### Heartbeat (`HeartbeatWorkUnit`)
- **R9** — DEBE existir `HeartbeatWorkUnit(id, owner)` que fije
  `lease_expires_at = now + TTL` y `heartbeat_at = now` **solo si** la unidad está
  `claimed` **y** `owner_id` coincide con `owner`.
- **R10** — Si el heartbeat afecta 0 filas (la unidad fue expropiada, completada o no le
  pertenece al llamador), DEBE devolver un error/booleano que le indique al agente que
  **ya no es dueño y debe detenerse**. NO DEBE re-adquirir el lease.

### Complete con defensa de fencing (`CompleteWorkUnit`)
- **R11** — `CompleteWorkUnit` DEBE cerrar la unidad solo si sigue `claimed` y el llamador
  es el `owner_id` actual. Un dueño **expropiado** (otro agente reclamó la unidad tras
  vencer el lease) NO DEBE poder cerrarla: el UPDATE DEBE afectar 0 filas y devolver error.
- **R12** — La verificación de propiedad DEBE basarse en `owner_id` (y opcionalmente en el
  `fencing_token` que el dueño recibió en el claim): un `complete` con un `fencing_token`
  distinto del vigente NO DEBE cerrar la unidad.

### Configuración
- **R13** — El TTL del lease DEBE ser configurable (`internal/config`) con un default
  conservador. El máximo de `attempts` antes del dead-letter DEBE ser configurable con
  default sano. Valores ausentes/≤0 DEBEN caer al default.

### Tool / integración
- **R14** — `musubi_work` DEBE exponer un `action` para el heartbeat (renovar lease) que
  invoque `HeartbeatWorkUnit`. El conteo de tools MCP NO DEBE cambiar (es un action nuevo,
  no una tool nueva).
- **R15** — El estado del batch (`WorkBatchStatus`) DEBE incluir, por unidad, el `owner_id`,
  el `lease_expires_at` y `attempts`, para diagnóstico. La salida DEBE seguir siendo compacta.
- **R16** — El build y la suite completa DEBEN quedar verdes; el cambio DEBE ser model-free
  (sin LLM), Go puro, sin dependencias con cgo.

## Escenarios

### Escenario: reclamo lazy de una unidad huérfana
- **Given** una unidad en `claimed` por el agente A con `lease_expires_at` en el pasado
- **When** el agente B llama `ClaimWorkUnit`
- **Then** la unidad se le asigna a B (`owner_id=B`), con un `lease_expires_at` futuro,
  `attempts` incrementado y `fencing_token` mayor que el que tenía A

### Escenario: no se roba un lease vigente
- **Given** una unidad en `claimed` por A con `lease_expires_at` en el futuro
- **When** B llama `ClaimWorkUnit`
- **Then** B no obtiene esa unidad (recibe otra open, o `ok=false` si no hay ninguna)

### Escenario: heartbeat mantiene vivo el lease
- **Given** una unidad `claimed` por A a punto de vencer
- **When** A llama `HeartbeatWorkUnit(id, A)` antes del vencimiento
- **Then** `lease_expires_at` se extiende y B no puede reclamarla

### Escenario: heartbeat de un dueño expropiado
- **Given** una unidad que A tenía y que B ya reclamó tras vencer el lease
- **When** A (revivido) llama `HeartbeatWorkUnit(id, A)`
- **Then** afecta 0 filas y A recibe la señal de detenerse (ya no es dueño)

### Escenario: fencing bloquea al zombie en complete
- **Given** A fue expropiado por B (B es el `owner_id` vigente, con `fencing_token` mayor)
- **When** A (revivido) llama `CompleteWorkUnit` con su token/owner viejo
- **Then** el UPDATE afecta 0 filas; la unidad NO se cierra por A; B sigue siendo el dueño

### Escenario: dead-letter tras agotar reintentos
- **Given** una unidad huérfana cuyo `attempts` ya alcanzó el máximo configurado
- **When** un agente llama `ClaimWorkUnit`
- **Then** la unidad pasa a `failed` (dead-letter) y no se entrega como reclamada

## Fuera de alcance
- Goroutine barredora que expire leases de forma proactiva (reciclado es lazy).
- Backoff exponencial / jitter en los reintentos.
- Coordinación multi-nodo con relojes distribuidos (single-writer SQLite).

## Preguntas abiertas
- [ ] ¿El dead-letter (R7) se resuelve dentro del mismo statement del claim o como un paso
      determinista previo en la misma llamada Go? (resolver en design; criterio: atomicidad
      vs. legibilidad del SQL)
- [ ] ¿El `complete` valida solo `owner_id` o exige también el `fencing_token` explícito del
      llamador? (design; probable: `owner_id` obligatorio, `fencing_token` como defensa
      opcional adicional si el llamador lo pasa)
- [ ] Default de TTL: ¿del orden de los 30s de SQS, o mayor por tratarse de trabajo de
      agente que puede tardar minutos? (design; probable: minutos, configurable)
