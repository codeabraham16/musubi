---
artifact: proposal
schema_version: "1.0"
change: ab-lift-bajada
status: draft
---

# Propuesta — Bajar el contador del A/B del decididor para que el lift se mida

## Intención
El A/B del decididor de skills no puede calcular un lift: su DENOMINADOR (`oportunidades`) nace
sólo en el central y su NUMERADOR (`cargadas`) sólo en cada máquina local, y las dos magnitudes
nunca conviven en una base. Es el límite conocido y deferido que `decididor_ab.go` documenta de su
puño (líneas 27-31: *"oportunidades nace en el central y cargadas en la máquina local… el flujo
cruzando el sync es trabajo siguiente"*). Este cambio es ese trabajo siguiente: hace que las filas
`decididor_ab` del central BAJEN a la base local, para que el `UPDATE`-only de
`bumpCargadasDecididorTx` tenga filas sobre las cuales sumar y el lift se lea en la máquina.

## Alcance
- **Incluye:**
  - Un read-only por el que el central entrega sus filas `decididor_ab` acotadas al proyecto de la
    credencial (una tool MCP nueva, hermana de `musubi_sync_pull`).
  - El lado cliente: el `SyncClient` baja ese snapshot una vez por ciclo de drain entrante.
  - El merge local con AUTORIDAD POR COLUMNA: el central manda sobre `oportunidades`/`exigencias`
    (REPLACE, idempotente); la local manda sobre `cargadas` (se preserva, nunca se pisa).
  - El cableado en `drainInboundOnce`, best-effort, detrás del mismo gate que el resto de la bajada.
- **No incluye:**
  - Ninguna SUBIDA de contadores (no se toca el outbox). El flujo es sólo de bajada: así no hay eco.
  - Sincronizar `skill_usage` ni ninguna otra tabla de contadores: sólo `decididor_ab`.
  - Reconstruir cargas pasadas que ocurrieron antes de la primera bajada (el instrumento mide
    PROSPECTIVAMENTE, desde que la fila existe en la local).
  - Resolver la asimetría de población del lift (ver Impacto): es un caveat aceptado, no un objetivo.
  - Migración de esquema: la tabla `decididor_ab` ya existe (v59); no se agrega ni se altera columna.

## Enfoque
La bajada entrante del cerebro híbrido ya baja observaciones `shared` del central (pull, no recall
federado en vivo: preserva local-first). Se EXTIENDE esa misma bajada con un canal lateral mínimo
para el contador del A/B: un pull dedicado del snapshot `decididor_ab` (tabla diminuta: a lo sumo dos
filas por proyecto, `on`/`off`), que se MERGEA en la base local sin pisar el numerador local. No se
piggybackea sobre el pull de observaciones (que es paginado y cursoreado); va como una llamada única
por ciclo, idempotente, que no participa de la contabilidad de `sync_viajes` ni del cursor.

## Impacto
- Áreas/archivos afectados: `internal/mcp/registry.go` (tool nueva + clasificación read-only),
  `internal/mcp/methods.go` (handler), `internal/mcp/syncclient.go` (método cliente),
  `internal/mcp/scheduler.go` (cableado en `drainInboundOnce`), `internal/memory/decididor_ab.go`
  (el merge local), el golden `toolslist-completo.golden.json` (la tool nueva; el golden del NÚCLEO
  no cambia: la tool no es del núcleo), `CHANGELOG.md`, y el SDD `specs/ab-lift-bajada/`.
- Compatibilidad: ADITIVO y retrocompatible. Un central viejo que no expone la tool hace fallar el
  pull lateral, que es best-effort y sólo logea: el resto de la bajada sigue igual. No hay migración.
- **Caveat honesto (asimetría de población):** `oportunidades` es central-wide (agrega el tráfico de
  recall de TODAS las máquinas que pasan por el central) y `cargadas` es de ESTA máquina. El lift
  local queda = (cargas de esta máquina) / (ocasiones de todas). Es una señal DIRECCIONAL, no una
  tasa absoluta por máquina; se documenta en la spec y en el código. Cerrar esta asimetría (contar
  oportunidades por máquina) es trabajo posterior, fuera de alcance.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| El merge pisa el `cargadas` local y borra el numerador | Autoridad por columna: el `DO UPDATE` NO nombra `cargadas`. Invariante I3 con test de sabotaje. |
| Re-bajar el mismo snapshot infla `oportunidades` | REPLACE, no suma (`oportunidades = excluded…`). Idempotente. Invariante I2 con test de sabotaje. |
| El canal lateral perturba el ritmo/viajes de la bajada de observaciones | No toca `salioALaRed` ni `sync_viajes`; es un helper aparte, best-effort, que sólo logea su error. |
| Fuga cross-tenant (la local ve el A/B de otro proyecto) | El central sirve con `scopedCtx`: `DecididorAB` ya acota por la credencial (mismo scope que `SkillUsage`). |
| Un central viejo sin la tool rompe la bajada | El pull lateral es best-effort: su fallo logea y no aborta el drain de observaciones. |

## Criterio de éxito
Tras un ciclo de bajada en una máquina en team mode: `DecididorAB` local devuelve filas con las
`oportunidades`/`exigencias` del central; una carga local posterior incrementa `cargadas` en la fila
del brazo del día; y `CalcularLift` sobre la base local pasa de `ok=false` (INDEFINIDO) a `ok=true`
con un lift real. Re-correr la bajada no cambia los números (idempotencia). `go vet && build &&
test -count=1` en verde, invariantes nuevos con sabotaje que los pone en rojo, golden sólo con la
tool nueva, grafo de specs válido.
