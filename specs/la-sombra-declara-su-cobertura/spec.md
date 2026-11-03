---
artifact: spec
schema_version: "1.0"
change: la-sombra-declara-su-cobertura
status: draft
---

# Especificación — la tabla de la sombra declara su cobertura

## Requisitos

- **R1** — `musubi conflicts shadow` DEBE declarar, además de la tasa por veredicto, cuántos pares
  del corpus son **medibles** por la sombra y cuántos quedan fuera, con el **motivo** de cada
  recorte. Un número sin su motivo se lee como una falla y no lo es.

- **R2** — La cobertura DEBE derivarse por **SQL del estado actual del corpus**. NO debe leer los
  contadores en memoria del `shadowWorker` ni persistir estado nuevo: el recorte es una propiedad
  del corpus, no de una corrida, y los contadores del worker mueren con el proceso.

- **R3** — El criterio de «re-detectable» DEBE ser el **mismo predicado** que usa la detección
  (`visibleObsPredicateDe`), aplicado a **las dos puntas** de la relación. NO debe ser una copia
  escrita a mano: el único sitio del repo que lo escribió a mano salió mal y le faltaba
  `quarantined = 0` (documentado en `internal/memory/scope.go:51`).

- **R4** — El criterio de «medible» DEBE derivarse de las **mismas constantes** que usa
  `encolarSombra` para decidir qué no descarta (`RelStatusPending`, `RelResolvedByHeuristic`), no de
  literales escritos en el SQL. Si el filtro de procedencia cambia, la cobertura DEBE cambiar con él
  o estaría declarando una cobertura que no es la que el worker aplica.

- **R5** — La cobertura DEBE distinguir los motivos de exclusión, porque son cosas distintas y se
  arreglan distinto: (a) **firma ajena** — la guarda del upsert conservó un veredicto que no es del
  detector, así que medirlo anotaría una procedencia falsa; (b) **punta no visible** — una de las
  dos observaciones está archivada, superseded o en cuarentena, así que el par no se vuelve a
  detectar nunca; y (c) **auto-resolución ya aplicada**, un SUBCONJUNTO de (b) que la cobertura DEBE
  nombrar aparte: el target está supersedido por su propio source, o sea que el detector resolvió
  solo y su veredicto surtió efecto. Sin (c), un «0 medibles» manda a buscar el problema donde no
  está: la única cosa que el detector firma solo es el auto-`supersedes`, y aplicarlo deja invisible
  a su propio target, así que esa vía se vacía **por construcción**. El repo ya lo había medido por
  otro lado (`internal/memory/relations_backfill.go:32-37`: exigir visibilidad «excluiría TODA
  auto-resolución exitosa», y en el cerebro central eso se comía 50 de 73 pares de señal).

- **R6** — Con `--json`, la cobertura DEBE emitirse como un objeto con **tags JSON explícitos** en
  minúscula, y la respuesta DEBE seguir trayendo el resumen por veredicto bajo su propia clave.

- **R7** — Con `shadow_verdicts` **vacía**, el comando DEBE declarar la cobertura **igual**. Hoy
  imprime «El libro del modo sombra está vacío» y apunta a la config y al motor de cognición, lo que
  **atribuye la causa equivocada**: medido el 2026-11-02, aun con la sombra encendida y el motor
  disponible el techo de lo medible es de apenas **4 pares sobre 901 re-detectables**, porque las
  otras 897 tienen firma ajena. El mensaje DEBE decir cuál es ese techo, y con techo cero DEBE decir
  que encenderla no escribiría una sola fila.

- **R8** — `SaveShadowVerdict` DEBE tener un custodio en **su propio paquete** (`internal/memory`)
  que verifique que `agree` se computa **adentro** y no lo pasa el caller. Hoy no tiene ninguno: los
  ocho tests de sombra viven en `internal/mcp/shadow_test.go` y prueban el worker y el filtro, no el
  cómputo de la columna de la que sale la tasa.

## Escenarios

### Escenario: el corpus es casi todo firma ajena y el reporte lo dice
- **Given** un corpus con 1 par medible (firmado `heuristic`) y 9 pares con veredicto de un agente
- **When** se corre `musubi conflicts shadow`
- **Then** la salida declara 1 medible y 9 recortados por firma ajena, y nombra el motivo

### Escenario: una punta invisible no se cuenta como firma ajena
- **Given** un par cuyo `target` está `superseded_by` otra observación, y nada más
- **When** se pide la cobertura
- **Then** ese par cuenta en «no re-detectable por punta no visible» y **no** en «firma ajena»,
  porque la razón por la que no se mide es otra

### Escenario: el libro vacío declara su techo
- **Given** `shadow_verdicts` sin ninguna fila y un corpus sin un solo par medible
- **When** se corre `musubi conflicts shadow`
- **Then** la salida dice que no hay mediciones **y** que encenderla no escribiría una sola fila,
  en vez de atribuirlo sólo a la config o al motor

### Escenario: la auto-resolución aplicada se nombra aparte
- **Given** un par `supersedes` firmado por el detector cuyo target quedó supersedido por su propio
  source
- **When** se pide la cobertura
- **Then** el par cuenta en «punta no visible» **y además** en «auto-resolución ya aplicada», que es
  su subconjunto, para que el cero de medibles tenga su explicación estructural

### Escenario: el JSON trae las dos cosas
- **Given** cualquier estado del libro
- **When** se corre `musubi conflicts shadow --json`
- **Then** la salida es un objeto con la cobertura y el resumen por veredicto, cada campo con su tag
  en minúscula

### Escenario: el criterio de medible no se duplica
- **Given** el filtro de procedencia de `encolarSombra`
- **When** se cambia la constante de la firma del detector
- **Then** el SQL de la cobertura cambia con ella, porque se arma desde la misma constante

### Escenario: el criterio de visibilidad no se duplica
- **Given** `visibleObsPredicateDe`
- **When** se le agrega una condición nueva de visibilidad
- **Then** la cobertura la hereda sin tocar su propia consulta

## Fuera de alcance

- La pantalla model-free de `encolarSombra` (descartar el par procedencia-señal imposible). El dueño
  resolvió la escalada del panel eligiendo la opción (B), explícitamente sin ella.
- El caso S6 (el entrante sin firma).
- El contrato de `UpsertObsRelation` con un par `pending` firmado: medido **latente** el 2026-11-02
  —`pendingRel` nunca firma y los tres callers de producción escriben `resolved`—, así que su
  custodio va a su propio PR y no a éste.
- Persistir los contadores del worker, cambiar la guarda, el esquema o el camino de guardado.
- **`Medidas` no deduplica por `relation_id`**, que no tiene `UNIQUE`
  (`internal/memory/migrations.go:1003-1016`) y cuyo INSERT no lleva `ON CONFLICT`
  (`internal/mcp/shadow.go:60-66`), así que una relación re-medida cuenta dos veces. El design
  declara «filas de `shadow_verdicts`», que es exactamente lo que el código hace: **no hay defecto
  contra lo declarado**, y el arreglo —`COUNT(DISTINCT relation_id)` o deduplicar en la escritura—
  es un cambio de contrato con su propio custodio.
- **Un custodio de `COUNT(1) == COUNT(DISTINCT r.id)`**, que es el chequeo que sí detecta una
  consulta que cuente fuentes en vez de relaciones. No entra acá porque **«pares o fuentes» no está
  declarado**: R1 habla de «pares» y el design fija «`Relaciones` — todas las filas de
  `observation_relations`», así que elegir uno de los dos es semántica nueva. Medido: el corpus real
  tiene 244 fuentes con fan-out (el 78 % de las filas), y las dos identidades no lo atrapan.
- **La columna del predicado medible: `r.status` contra `r.relation`.** Medido **latente** igual que
  el contrato de `UpsertObsRelation`: `pendingRel` (`internal/memory/conflicts.go:410-412`) siempre
  escribe `RelPending` junto con `RelStatusPending`, así que hoy ninguna fila de producción
  distingue las dos columnas. La sonda que lo enrojece ya está medida y va con su PR.
- **Un custodio de R2** (que la cobertura no dependa del estado del worker). Hoy se sostiene «por
  construcción»: `internal/memory` no puede importar `internal/mcp` sin ciclo y `ShadowCoverage`
  sólo hace `SELECT`. No hay sabotaje que compile, y un custodio del grafo de imports es
  desproporcionado. Queda **declarado**, no supuesto.

## Preguntas abiertas

- [x] ¿Contadores persistidos o derivación SQL? → **derivación**, resuelto en el proposal y en D1.
- [x] ¿Existe un helper para el predicado de visibilidad con alias? → sí, `visibleObsPredicateDe`,
      y `obsrank.go:62-63` ya lo usa para las dos puntas de una arista: el mismo caso que éste.
- [x] ¿La cabecera va **siempre** o sólo cuando el recorte supera algún porcentaje? → **siempre**,
      resuelto en D5 con el mismo argumento que tenía la inclinación: un recorte chico también es
      información, y un umbral agrega una rama que habría que custodiar y que puede esconder justo
      el caso informativo.
