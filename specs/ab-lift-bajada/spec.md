---
artifact: spec
schema_version: "1.0"
change: ab-lift-bajada
status: draft
---

# Especificación — Bajada del contador del A/B del decididor

## Requisitos
Cada requisito es verificable y atómico. Vocabulario RFC 2119.

- **R1** — El central DEBE exponer un read-only que devuelva sus filas `decididor_ab` ACOTADAS al
  proyecto de la credencial que lo invoca (mismo aislamiento multi-tenant que `DecididorAB`/`SkillUsage`).
- **R2** — El `SyncClient` DEBE poder bajar ese snapshot del central en una sola llamada (sin cursor
  ni paginación: la tabla tiene a lo sumo dos filas por proyecto).
- **R3** — El merge local DEBE, por cada fila bajada, fijar `oportunidades` y `exigencias` al valor
  del central (REPLACE), de modo que bajar el MISMO snapshot dos veces deje los mismos números
  (idempotencia: no suma).
- **R4** — El merge local NO DEBE modificar `cargadas` de una fila que ya existe: el numerador es
  autoridad de la máquina local y se preserva aunque el central mande otro valor (incluido 0).
- **R5** — Una fila `decididor_ab` que no existía en la base local DEBE nacer con `cargadas = 0` (la
  máquina todavía no contó cargas para ese brazo), con las `oportunidades`/`exigencias` del central.
- **R6** — Tras el merge, un `bumpCargadasDecididorTx` sobre el brazo de una fila bajada DEBE
  incrementar `cargadas` (la fila ahora existe; antes el UPDATE-only no afectaba filas). En
  consecuencia `CalcularLift` sobre la base local DEBE pasar de `ok=false` a `ok=true` cuando los dos
  brazos tienen oportunidades.
- **R7** — La bajada del contador NO DEBE encolar nada en el outbox ni agregar ninguna vía de SUBIDA:
  el flujo es sólo descendente (sin eco).
- **R8** — La bajada del contador DEBE ser best-effort: su fallo (red, central viejo sin la tool,
  error RPC) DEBE logearse y NO DEBE abortar la bajada de observaciones ni perturbar su contabilidad
  (`sync_viajes`, cursor, ritmo).
- **R9** — La bajada del contador DEBE correr sólo bajo el candado de la bajada entrante (un solo
  proceso por base), heredando el gate existente (sync configurado Y team mode).

## Escenarios
Formato Given/When/Then (un escenario por comportamiento observable).

### Escenario: el snapshot del central baja y puebla la base local (R1, R2, R5)
- **Given** un central con filas `decididor_ab` para el proyecto `P` (`on`: oportunidades=10,
  exigencias=4, cargadas=0) y una base local SIN filas `decididor_ab`
- **When** la máquina local baja el snapshot y lo mergea
- **Then** `DecididorAB` local devuelve la fila `(P, on)` con oportunidades=10, exigencias=4 y
  cargadas=0

### Escenario: re-bajar el mismo snapshot no infla el denominador (R3)
- **Given** una base local que ya mergeó el snapshot `(P, on, oportunidades=10)`
- **When** se baja y mergea el MISMO snapshot otra vez
- **Then** `DecididorAB` local sigue mostrando oportunidades=10 (no 20)

### Escenario: el merge preserva el numerador local (R4)
- **Given** una base local con la fila `(P, on)` que tiene cargadas=5 (contadas localmente) y un
  snapshot del central con esa misma fila con cargadas=0 y oportunidades=12
- **When** se mergea el snapshot
- **Then** la fila local queda con oportunidades=12 (del central) y cargadas=5 (preservada)

### Escenario: tras bajar, el lift pasa de indefinido a medible (R6)
- **Given** una base local a la que se bajó el snapshot con `on` (oportunidades=10) y `off`
  (oportunidades=8), ambas cargadas=0, y `CalcularLift` devuelve `ok=false`
- **When** `RecordSkillEvents` procesa una carga del proyecto `P` cuyo brazo del día es `on`
- **Then** la fila `(P, on)` local queda con cargadas=1 y `CalcularLift` devuelve `ok=true` con un
  lift real (tasa_on − tasa_off)

### Escenario: un central viejo sin la tool no rompe la bajada (R8)
- **Given** una máquina en team mode cuyo central no expone la tool del snapshot
- **When** corre un ciclo de bajada entrante
- **Then** el pull del contador falla, se logea, y la bajada de observaciones completa su ciclo
  normal (cursor y `sync_viajes` intactos)

## Fuera de alcance
- Subir contadores al central (ninguna vía de outbox para `decididor_ab`).
- Sincronizar `skill_usage` u otras tablas de contadores.
- Reconstruir cargas ocurridas antes de la primera bajada (medición prospectiva).
- Corregir la asimetría de población del lift (oportunidades central-wide ÷ cargadas por máquina):
  caveat aceptado y documentado, no un objetivo.

## Preguntas abiertas
- [ ] (resuelta en design) ¿Tool MCP dedicada o piggyback sobre `musubi_sync_pull`? → dedicada.
- [ ] (resuelta en design) ¿Dónde se cablea en `drainInboundOnce`? → tras el gate de ritmo, como
  helper aparte que no toca `salioALaRed`.
