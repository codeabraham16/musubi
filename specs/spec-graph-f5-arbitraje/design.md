---
artifact: design
schema_version: "1.0"
change: spec-graph-f5-arbitraje
status: draft
---

# Diseño técnico — F5: arbitraje de conflictos spec↔observación

## Hallazgo que fija el diseño (resuelve las 3 preguntas abiertas de la spec)

Leyendo el código real (no el mapa del Explore) aparecieron tres hechos que
cambian el enfoque y lo vuelven más simple:

1. **La proyección NO tiene vector, y es a propósito.**
   `renderAndProject` ([methods_spec.go:879](internal/mcp/methods_spec.go#L879))
   llama `ProjectSpecShared(..., nil)`: el embedding va `nil` porque
   `spec_create/update/delete` son tools `write` y la guarda
   `candado_no_cruza_la_red` prohíbe que una `write` toque la red. El embedder de
   red (`ollama`/`openai`) queda vedado; el `static` (in-process) existe pero no
   se usa en ese borde. **Consecuencia: ningún par spec↔creencia tiene coseno** —
   ni por el lado de la proyección (sin vector) ni importa el de la creencia (el
   coseno necesita vector en AMBOS extremos con el mismo `model_id`). F5 es, por
   construcción, **léxico puro** para este tipo de par. Eso NO es una carencia:
   es la identidad del proyecto (`conflicts.go` corre entero model-free; el coseno
   sólo corrobora). El peor caso de un falso negativo léxico es *una relación de
   menos*, nunca memoria oculta de más.

2. **`ProjectSpecShared` NO pasa por `saveObservation`, así que hoy NO corre
   `DetectRelations`.** Es un método dedicado (D5 de F4a) que saltea las guardas
   de observación humana. Por eso la proyección hoy no dispara ninguna detección:
   el namespace `spec-proj:` está materializado pero **sin consumidor** (lo dice
   su propio comentario, [spec_projection.go:17-19](internal/memory/spec_projection.go#L17)).
   F5 es ese consumidor.

3. **`DetectRelations` no toca la red** (lee la DB y el índice vectorial
   in-process), así que correrlo desde el seam `write` de la spec **respeta
   `candado_no_cruza_la_red`**. De hecho `save_observation` —también `write`— ya
   corre `DetectRelations` vía `detectAndSurface`. El seam del write es legítimo.

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | **Discriminar la proyección por el prefijo de id `spec-proj:`** (`specProjection(r obsRow) bool`), no por el topic `spec/<id>`. | Por topic `spec/<id>`. | El id es la señal reservada por F4a para esto ([spec_projection.go:17-19](internal/memory/spec_projection.go#L17)); el topic lo podría usar por casualidad una nota humana, que quedaría sobre-protegida. `obsRow` ya trae `id`. |
| D2 | **R5(a)/R4 reusando los dos guardas que ya existen**: `complementaryPair` protege la proyección como *target* (saltea el par: la relación NO nace); `dominiosAjenos` la exime como `historicalRecord`. | Un carril nuevo; un discriminador cross-store. | Cero mecanismo nuevo. La proyección se comporta, en estos dos guardas, EXACTO como un commit: no nace una relación que la tome como *target* (R5a — el *efecto* de ocultar, caso de una relación nacida por otra vía, lo cubre `markSuperseded` en D4) y habla a cualquier dominio (R4). |
| D3 | **R1/R2 cableando `renderAndProject` → `DetectRelations(projID, DetectOnly=true)` tras el commit, y surfaceando los `pending`.** | Un seam en reconcile/startup; un tool `read` dedicado `musubi_spec_arbitrate`. | El trigger natural de "spec cambia → creencias viejas" es el momento del write. `DetectRelations` no cruza la red, así que cabe en el write. `DetectOnly=true` fuerza TODO a `pending` (R2) y vuelve `markSuperseded` inalcanzable por ese camino. |
| D4 | **R3 en profundidad: guard también en `markSuperseded`** (rechaza ocultar un target que es proyección). | Confiar sólo en `complementaryPair`. | Criterio de éxito #2 exige "ningún camino, ni auto ni `musubi_judge`". `markSuperseded` es el punto único por el que pasan el auto-path y `ResolveObsRelation`; el guard ahí es imposible de saltear aunque una relación con la proyección como target haya nacido por otra vía. |
| D5 | **Acoplamiento declarado D2↔R3**: eximir la proyección de `dominiosAjenos` (R4) AUMENTA la exposición a que una creencia la supersede, así que `complementaryPair` DEBE protegerla (R3). Se hacen juntas o se abre un agujero. | Hacer sólo una. | Hoy `dominiosAjenos` saltea el par creencia×proyección por cruce de dominio; al eximir la proyección, el par procede a scoring y sin D2 podría auto-superseder la proyección. |
| D6 | **Sin computar embeddings, sin migración, sin tool nueva, sin golden.** | Computar el vector de la proyección (en un seam `read`) para habilitar coseno. | MVP léxico-puro honra el guard de red y la identidad model-free. El coseno queda como mejora futura (F5.1): computar el vector en un seam `read` donde la red está permitida. Como no se agrega ni cambia ninguna tool MCP, no hay que regenerar el golden de tools/list. |

## Enfoque de implementación

Cinco puntos de cambio. Cuatro en `internal/memory/conflicts.go`, uno en
`internal/mcp/methods_spec.go`.

1. **`specProjection(r obsRow) bool`** (conflicts.go, nuevo) —
   `strings.HasPrefix(r.id, specProjectionIDPrefix)`. `specProjectionIDPrefix`
   vive en el paquete `memory` (spec_projection.go:19), mismo paquete: accesible
   sin export nuevo.

2. **`complementaryPair`** (conflicts.go:178) — añadir `|| specProjection(b)` a la
   condición de protección del target. Hoy: `historicalRecord(b.topicKey) ||
   declaredLedger(...)`. La proyección se suma como "no se tacha" (R3).

3. **`dominiosAjenos`** (conflicts.go:250) — la exención hoy es
   `!historicalRecord(a.topicKey) && !historicalRecord(b.topicKey)`. Se extiende a
   `!exentoDeDominios(a) && !exentoDeDominios(b)`, con
   `exentoDeDominios(r) = historicalRecord(r.topicKey) || specProjection(r)` (R4).

4. **`markSuperseded`** (conflicts.go:673) — al entrar, si el `targetID` es una
   proyección (`strings.HasPrefix(targetID, specProjectionIDPrefix)`), **rechazar
   con error** sin tocar la fila (R3/D4). Devolver error, no no-op: así
   `ResolveObsRelation` propaga el rechazo y la relación NO queda marcada
   resuelta (verificar en implement que el orden marca-resuelve/oculta permita que
   el rechazo deje la relación en `pending`; si no, envolver en la misma tx).

5. **`renderAndProject`** (methods_spec.go:879) — tras `ProjectSpecShared` (que
   commitea), correr la detección y surfacear. Un helper nuevo en la capa mcp,
   gemelo de `detectAndSurface` pero con `DetectOnly=true` y sin el anuncio de
   auto-supersede (con `DetectOnly` nunca hay `resolved`): corre
   `s.engine.DetectRelations(projID, s.conflictOpts(true))`, encola la sombra como
   el otro, y arma el texto de surfacing. Como `renderAndProject` hoy devuelve sólo
   `error`, el texto de surfacing se devuelve por un canal que llegue a la
   respuesta del tool (decidir en implement: o `renderAndProject` devuelve
   `(string, error)` y el caller lo anexa, o el surfacing se hace en el caller).

**Durante implement, los 5 puntos se repartieron en 6 archivos** (el plan subcontaba
la plomería, no el algoritmo): los 4 de `conflicts.go` y el de `methods_spec.go`
(firma `(string, error)` + los dos call sites), MÁS: el helper de surfacing cayó en
`methods.go` y no en `methods_spec.go` porque éste no importa `logx`; `relations_obs.go`
tomó el fix de orden que el punto 4 anticipaba como contingencia; y exponer el id
determinista a la capa mcp pidió `SpecProjectionObsID` en `spec_projection.go` más su
firma en la interfaz `SpecProjectionStore` de `backend.go`. Ninguna firma de tool MCP
cambió; el reparto es plomería, no alcance nuevo.

**Texto de surfacing (R1 + el caso "spec vieja").** El aviso DEBE enunciar los dos
desenlaces, porque ocultar una creencia correcta sería el error:

> `[spec] La spec <id> cambió y hay N creencia(s) que podrían haber quedado
> viejas. Revisá cada una: si la SPEC gana, `musubi_judge supersedes` oculta la
> creencia vieja; si la CREENCIA tiene razón (la spec quedó atrás), NO la ocultes
> — actualizá el SPEC.md con `musubi_spec_update`.`

## Contratos / interfaces

```go
// conflicts.go
func specProjection(r obsRow) bool           // id con prefijo spec-proj:
func complementaryPair(_, b obsRow, opts ConflictOptions) bool  // + specProjection(b)
func dominiosAjenos(a, b obsRow) bool         // exención extendida a specProjection
func (e *DbEngine) markSuperseded(targetID, bySourceID string) error // rechaza target proyección

// methods_spec.go (capa mcp)
func (s *McpServer) renderAndProject(specID string, fm specgraph.Frontmatter, body string) (string, error) // + surfacing
```

Ninguna firma de tool MCP cambia. `ProjectSpecShared`, `ReconcileSpecProjections`
y el esquema de la base quedan intactos.

## Trade-offs

- **Se gana:** cero migración, cero tool nueva, cero golden, reuso total de la
  maquinaria probada (detección, cola `pending`, `musubi_judge`, aislamiento por
  tenant, conservación de veredicto). R3 a prueba de balas (dos capas, cada una
  con su test aislado). Respeta el guard de red y la identidad model-free.
- **Se cede:** sin coseno, F5 sólo surfacea contradicciones con **solape léxico**
  (identificadores/vocabulario compartido entre la creencia y el SPEC.md). Una
  contradicción puramente semántica, dicha con otras palabras, no se detecta en el
  MVP → relación de menos, nunca memoria oculta de más. El coseno queda para F5.1
  (computar el vector de la proyección en un seam `read`).

## Plan de pruebas

Todo en `internal/memory/conflicts_test.go` (+ un test de surfacing en
`internal/mcp`). Cada invariante con su sabotaje AISLADO que lo pone en rojo
atacando EXACTAMENTE lo que el test declara (adversarial-review §2; defensa en
profundidad NO debe tapar el sabotaje del otro guard):

- **R1/R2 (detección spec→creencia, siempre pending)** — dado una proyección y una
  creencia humana con solape léxico, tras `DetectRelations(projID, DetectOnly)`
  nace una relación `pending` source=proyección, target=creencia; la creencia
  sigue visible. Sabotaje: quitar la exención de `dominiosAjenos` → el par se
  saltea → no nace relación → rojo.
- **R3a (complementaryPair protege la proyección como target)** — dado una creencia
  como source y la proyección como target en el mismo dominio, NO nace relación
  con la proyección como target. Sabotaje: quitar `|| specProjection(b)` → nace la
  relación → rojo. (No depende de `markSuperseded`.)
- **R3b (markSuperseded rechaza)** — llamar `markSuperseded(projID, x)` directo (o
  `ResolveObsRelation` sobre una relación insertada a mano con target=proyección) →
  `projID.superseded_by` sigue NULL. Sabotaje: quitar el guard → la proyección se
  oculta → rojo. (Bypassa complementaryPair llamando directo: aísla este guard.)
- **R4 (cross-domain)** — proyección (dominio `spec`) vs creencia de otro dominio:
  el par NO se saltea por `dominiosAjenos`. Sabotaje: el mismo de R1 (son el mismo
  guard visto desde dos ángulos); separar el assert para que cada uno falle solo.
- **R6 (tenant)** — proyección en tenant A, creencia en tenant B: no nace relación.
  Reusa el guard de `projectID`; test de no-regresión.
- **R7 (no-regresión)** — la suite `conflicts_test.go` existente sigue verde SIN
  tocarse (pares obs↔obs sin proyección: el predicado nuevo no los matchea).
- `GOWORK=off go vet ./... && go build ./... && go test -race -count=1 ./...` en
  verde antes de reportar.
