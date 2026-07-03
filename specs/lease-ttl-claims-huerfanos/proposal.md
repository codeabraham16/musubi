---
artifact: proposal
schema_version: "1.0"
change: lease-ttl-claims-huerfanos
status: archived
---

# Propuesta — Lease/TTL para claims huérfanos en la pizarra multi-agente

## Intención
La pizarra (`musubi_work` / `work_units`) tiene un **bug de liveness real**: `ClaimWorkUnit`
hace un `UPDATE...RETURNING` atómico que pasa una unidad de `open` a `claimed`, pero **no
hay forma de soltarla si el agente que la reclamó muere** (crash, timeout, sesión cerrada).
La unidad queda en `claimed` para siempre; el batch nunca llega a `done` y ningún otro
agente puede retomarla. En una orquestación multi-agente real —el pilar que Track 12 elevó
a co-pilar— esto es una cola que se cuelga en silencio.

Queremos que un claim tenga un **lease con vencimiento (TTL)**: si el dueño no renueva su
lease (heartbeat) dentro de la ventana, la unidad vuelve a estar disponible y otro agente
la puede **reclamar de nuevo automáticamente**, sin intervención manual ni un barredor que
llame a un LLM. Todo determinista, model-free, en el mismo `UPDATE...RETURNING` atómico que
ya existe.

## Alcance
- **Incluye:**
  - Columnas nuevas en `work_units` (migración versionada): `lease_expires_at`,
    `heartbeat_at`, `owner_id`, `attempts`, `fencing_token`.
  - `ClaimWorkUnit` amplía su `WHERE` a `open OR (claimed AND lease_expires_at < now)` →
    **reclamo lazy** de unidades huérfanas dentro del mismo UPDATE atómico. Setea
    `owner_id`, `lease_expires_at = now + TTL`, incrementa `attempts` y `fencing_token`.
  - `HeartbeatWorkUnit(id, owner)`: renueva el lease (`lease_expires_at = now + TTL`) solo
    si seguís siendo el dueño y la unidad sigue `claimed`. 0 filas afectadas = fuiste
    expropiado → el agente debe parar.
  - **Fencing token** monótono: defiende del "worker zombie" (un agente lento que revive
    tras ser expropiado y quiere escribir con un token viejo → su UPDATE afecta 0 filas).
  - `CompleteWorkUnit` sigue exigiendo ser el dueño (ahora `owner_id`), y valida que el
    lease no haya expirado / no hayas sido expropiado.
  - Un `action` nuevo en la tool `musubi_work` para el heartbeat, y exponer TTL/lease en el
    estado del batch para diagnóstico.
  - Dead-lettering opcional: si `attempts >= max`, la unidad pasa a `failed` en vez de
    reciclar infinitamente (evita loop de crash-reclaim-crash).
- **No incluye (explícito):**
  - Un proceso barredor de fondo (goroutine) que expire leases proactivamente. El reciclado
    es **lazy** (en el próximo claim), que es más simple, sin concurrencia nueva y suficiente:
    una unidad huérfana solo importa cuando alguien la quiere reclamar. (Un sweeper eager
    para dead-letter queda como follow-up, no en este cambio.)
  - Reintentos con backoff exponencial / jitter. El TTL fijo configurable alcanza.
  - Distribución multi-nodo con relojes desincronizados: la pizarra es una única base
    SQLite con un solo escritor; `CURRENT_TIMESTAMP` es la única fuente de tiempo.

## Enfoque
`work_units` vive en una sola base SQLite con **un único escritor** (WAL + busy_timeout),
así que no hace falta `SKIP LOCKED` ni locking distribuido: el `UPDATE...RETURNING` con
subselect ya es atómico. El patrón es el **visibility timeout de SQS** (y el lease de
Chubby): reclamar = tomar un lease con vencimiento; trabajar = renovarlo; terminar = cerrar
como dueño. El reciclado de huérfanos se hace **ampliando la condición de elegibilidad del
claim** (no con un job aparte): una unidad `claimed` cuyo lease venció es, a efectos del
claim, tan reclamable como una `open`. Semántica **at-least-once** → el trabajo debe ser
idempotente; el fencing token es la barrera que vuelve seguro el solapamiento.

## Impacto
- Áreas/archivos afectados:
  - `internal/memory/migrations.go` (migración v4: columnas + índice por lease).
  - `internal/memory/work.go` (claim con reciclado lazy, heartbeat nuevo, complete por owner).
  - `internal/mcp/methods*.go` + `registry.go` (action de heartbeat en `musubi_work`; sin
    tool nueva → el conteo de tools no cambia).
  - `internal/config` (TTL del lease y max attempts configurables, con defaults sanos).
  - Tests nuevos de work.go (expiración, reclamo lazy, fencing, heartbeat perdido).
- Compatibilidad: **aditivo y retrocompatible**. Las columnas nuevas son nullable / con
  default; una unidad sin lease (`lease_expires_at IS NULL`) se trata como no-vencida hasta
  su primer claim bajo el esquema nuevo. `claimed_by` se conserva (alias histórico de
  `owner_id`) para no romper datos existentes.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Un agente vivo pero lento pierde su lease y su trabajo se duplica | TTL con margen holgado (default conservador); heartbeat barato; fencing token vuelve inofensiva la doble escritura (el zombie afecta 0 filas) |
| Reciclado infinito de una unidad que siempre crashea | `attempts` + dead-letter a `failed` cuando supera el máximo |
| Reloj: `now` inconsistente | Única fuente `CURRENT_TIMESTAMP` de la misma base; no se compara contra relojes de cliente |
| Romper datos/tests existentes de la pizarra | Migración aditiva; `claimed_by` preservado; defaults que hacen que el comportamiento viejo (sin heartbeat) siga cerrando batches |
| Semántica at-least-once sorprende al usuario | Documentar en la directiva/tool que el trabajo debe ser idempotente; es el estándar de toda cola con visibility timeout |

## Estrategia de rollback
Cambio aditivo y contenido. Rollback = revertir el PR; las columnas nuevas quedan inertes
(ninguna otra ruta las lee). No hay migración destructiva que deshacer: `user_version`
puede quedar en v4 sin daño porque las columnas son ignoradas por el binario viejo
(SQLite no rechaza columnas de más). Ante duda, un binario previo abre la misma base sin
error porque nunca referencia las columnas nuevas.

## Criterio de éxito
1. Un claim sobre una unidad `claimed` cuyo `lease_expires_at` ya pasó la **reclama** y
   devuelve la unidad al nuevo dueño, incrementando `attempts` y `fencing_token` — cubierto por test.
2. `HeartbeatWorkUnit` renueva el lease del dueño; si la unidad fue expropiada, devuelve
   0 filas / señal de "parar" — cubierto por test.
3. Un `CompleteWorkUnit` de un dueño expropiado (token viejo) **no** cierra la unidad
   (afecta 0 filas) — cubierto por test (defensa fencing).
4. Una unidad con `attempts >= max` va a dead-letter (`failed`) en vez de reciclar — test.
5. Todo model-free, Go puro, sin deps nuevas; build + suite verdes; migración idempotente.
