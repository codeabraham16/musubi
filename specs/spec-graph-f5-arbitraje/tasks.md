---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f5-arbitraje
status: draft
---

# Tareas — F5: arbitraje de conflictos spec↔observación

Checklist ordenada por dependencia. Cada tarea es un work-unit reviewable
(idealmente ≤ ~400 líneas de diff). Marcá `[x]` al completar.

Mapea 1:1 los 5 puntos de cambio del `design.md` y los 6 invariantes de la
`spec.md`. Regla de la casa (musubi-rules §8 / go-hygiene §5): **cada invariante
nuevo necesita su test verificado FALLANDO bajo un sabotaje que ataca
EXACTAMENTE lo que el test declara** — la defensa en profundidad (D4 sobre D2)
tapa el sabotaje del otro guard, así que los sabotajes van AISLADOS.

## Implementación
- [ ] **T1 — predicado `specProjection(r obsRow) bool`** · _archivos:_
  `internal/memory/conflicts.go`. `strings.HasPrefix(r.id, specProjectionIDPrefix)`.
  Mismo paquete `memory` que `spec_projection.go:19` → sin export nuevo. Es la
  base de T2, T3, T4. Comentario en español que explique el PORQUÉ del prefijo
  (señal reservada por F4a, no el topic — una nota humana podría usar `spec/<id>`
  por casualidad y quedar sobre-protegida).
- [ ] **T2 — `complementaryPair` protege la proyección como target (R3)** ·
  _archivos:_ `internal/memory/conflicts.go:178` · _depende de:_ T1. Añadir
  `|| specProjection(b)` a la condición de protección del target. La proyección
  se suma como "no se la tacha", junto a `historicalRecord(b.topicKey)` y
  `declaredLedger`.
- [ ] **T3 — `dominiosAjenos` exime la proyección (R4)** · _archivos:_
  `internal/memory/conflicts.go:250` · _depende de:_ T1. Extraer
  `exentoDeDominios(r) = historicalRecord(r.topicKey) || specProjection(r)` y
  cambiar la exención a `!exentoDeDominios(a) && !exentoDeDominios(b)`. Comentario:
  acoplada con T2 (D5) — eximir de dominios AUMENTA la exposición que T2 cierra;
  van juntas o se abre un agujero.
- [ ] **T4 — `markSuperseded` rechaza ocultar una proyección (R3/D4)** ·
  _archivos:_ `internal/memory/conflicts.go:673` · _depende de:_ T1. Al entrar, si
  `strings.HasPrefix(targetID, specProjectionIDPrefix)` → devolver error (NO
  no-op), sin tocar la fila. Verificar el orden marca-resuelve/oculta en
  `ResolveObsRelation`: el rechazo DEBE dejar la relación en `pending`, no
  resuelta-sin-efecto (envolver en la misma tx si hace falta). Error envuelto con
  `%w` (go-hygiene §1).
- [ ] **T5 — cablear `renderAndProject` → detección + surfacing (R1/R2)** ·
  _archivos:_ `internal/mcp/methods_spec.go:879` (+ helper en `internal/mcp`) ·
  _depende de:_ T1–T4. Tras `ProjectSpecShared` (que commitea), correr
  `s.engine.DetectRelations(projID, s.conflictOpts(true))` (DetectOnly=true fuerza
  todo a `pending`), encolar la sombra como `detectAndSurface`, y armar el texto
  de surfacing que enuncia **los dos desenlaces** (spec gana → `musubi_judge
  supersedes`; creencia tiene razón → `musubi_spec_update`, NO ocultar). Decidir
  la firma: `renderAndProject` pasa a `(string, error)` y el caller anexa el
  surfacing, o el surfacing se hace en el caller. No cruza la red
  (`DetectRelations` sólo DB+índice in-process) → respeta `candado_no_cruza_la_red`.

## Pruebas
Todo en `internal/memory/conflicts_test.go` salvo el de surfacing (`internal/mcp`).
Cada uno con su sabotaje aislado verificado en rojo ANTES de dar por buena la cobertura.
- [ ] **T6 — R1/R2 (detección spec→creencia, siempre pending)** · _cubre:_ R1, R2
  · _depende de:_ T5. Dado proyección + creencia humana con solape léxico, tras
  `DetectRelations(projID, DetectOnly)` nace relación `pending`
  source=proyección/target=creencia y la creencia sigue visible
  (`superseded_by IS NULL`). _Sabotaje:_ quitar la exención de T3 → el par se
  saltea → no nace relación → rojo.
- [ ] **T7 — R3a (`complementaryPair` protege la proyección como target)** ·
  _cubre:_ R3 (nacimiento) · _depende de:_ T2. Creencia como source, proyección
  como target, mismo dominio → NO nace relación con la proyección como target.
  _Sabotaje:_ quitar `|| specProjection(b)` de T2 → nace la relación → rojo. (No
  toca `markSuperseded`: aísla T2 de T4.)
- [ ] **T8 — R3b (`markSuperseded` rechaza)** · _cubre:_ R3 (efecto) · _depende
  de:_ T4. Llamar `markSuperseded(projID, x)` directo (o `ResolveObsRelation`
  sobre una relación insertada a mano con target=proyección) →
  `projID.superseded_by` sigue NULL y la relación NO queda resuelta. _Sabotaje:_
  quitar el guard de T4 → la proyección se oculta → rojo. (Bypassa
  `complementaryPair` llamando directo: aísla T4 de T2.)
- [ ] **T9 — R4 (cross-domain)** · _cubre:_ R4 · _depende de:_ T3. Proyección
  (dominio `spec`) vs creencia de otro dominio → el par NO se saltea por
  `dominiosAjenos`. _Sabotaje:_ el mismo de T6; assert separado para que R4 falle
  solo (mismo guard, otro ángulo).
- [ ] **T10 — R6 (aislamiento por tenant)** · _cubre:_ R6 · _depende de:_ T5.
  Proyección en tenant A, creencia en tenant B → no nace relación. No-regresión
  del guard `projectID` ya presente en `DetectRelations`.
- [ ] **T11 — R7 (no-regresión obs↔obs)** · _cubre:_ R7. La suite
  `conflicts_test.go` existente sigue VERDE sin tocarse (pares sin proyección: el
  predicado nuevo de T1 no los matchea). Es un gate, no un test nuevo: confirmar
  que ningún test preexistente cambió de comportamiento.
- [ ] **T12 — surfacing (R1, texto)** · _archivos:_ `internal/mcp/*_test.go` ·
  _depende de:_ T5. El texto de surfacing enuncia los dos desenlaces. _Sabotaje:_
  borrar la mitad "creencia tiene razón" → el assert del segundo desenlace → rojo.

## Docs / cierre
- [ ] **T13 — `CHANGELOG.md` (`[Unreleased]`)** — una viñeta: arbitraje
  spec↔observación (F5), spec shared → creencias viejas, model-free, sin migración.
  ⚠️ gotcha de rama-detrás-de-release-cut: si `[Unreleased]` ya no existe porque
  hubo release-cut, va en la versión en curso, no DENTRO de una tagueada.
- [ ] **T14 — Validar el grafo de specs** (skill `spec-graph`): colgantes, ids
  duplicados, ciclos. F5 no crea un SPEC.md nuevo (es lógica de memoria, no un
  módulo con contrato nuevo) → confirmar que no quedó nada dangling.
- [ ] **T15 — Verificar contra la spec** — cada R1–R8 con cobertura:
  R1/R2→T6+T12, R3→T7+T8, R4→T9, R5→cubierto por T7 (la relación NACE; sólo el
  efecto se bloquea), R6→T10, R7→T11, R8 (shared-only)→no hay carril para specs
  locales, se confirma por ausencia. Gate final:
  `GOWORK=off go vet ./... && go build ./... && go test -race -count=1 ./...` en verde.
- [ ] **T16 — Fase verify adversarial** (skill `adversarial-review`): evidencia
  primero (correr build/vet/test y quedarse con la salida real), dimensionar el
  panel con `musubi_detect_changes`, debatir con escépticos de lentes distintos,
  veredicto por mayoría determinista. NO auto-aprobar.

## Forecast de review
- Líneas estimadas: ~120 de producción (4 puntos chicos en `conflicts.go` + 1
  cableado en `methods_spec.go`) + ~200 de tests. **Chained PRs: no** — es un
  cambio cohesivo (un predicado y sus tres usos + el cableado que lo activa);
  partirlo dejaría guards swithout su activador o viceversa. 1 feature = 1 PR.
- **CHECKPOINT antes de implementar (regla del dueño "diseño profundo primero, el
  dueño bendice la dirección"):** confirmar el MVP léxico-puro (A) vs invertir en
  coseno vía seam `read` (B) ANTES de escribir T1. El resto (commit/push/merge) son
  compuertas del dueño por separado.
