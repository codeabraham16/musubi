---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f5-arbitraje
status: draft
---

# Propuesta — F5: arbitraje de conflictos spec↔observación

## Intención

La memoria se desvía contra el contrato curado. Alguien guarda una creencia
("el módulo X hace Y"); más tarde la `SPEC.md` de X se actualiza y dice otra
cosa. Hoy el recall sigue devolviendo la creencia vieja como si siguiera
vigente, y nada avisa que contradice la spec git-trackeada. F5 hace que esa
contradicción se **detecte y se arbitre**: una spec es evidencia autoritativa
que puede envejecer una creencia, exactamente como un commit `feat: migrar de X
a Y` envejece una nota que decía "usamos X" (regla ya escrita en `conflicts.go`:
*el registro histórico es el libro mayor; la nota es la creencia actual; sólo
las creencias se reemplazan*).

## Alcance

- **Incluye:**
  - Detección spec-aware en la dirección **spec cambia → creencias viejas**:
    cuando una spec `shared` se proyecta o actualiza, surfacear las
    observaciones humanas que quedaron contradichas, para que el agente las
    arbitre con `musubi_judge`.
  - Un **guard de seguridad**: la proyección de una spec queda protegida como
    *target* del efecto `supersedes` (una creencia nunca puede ocultar una
    spec). Esto cierra un bug latente actual.
  - La proyección exenta del guardia de dominios (`dominiosAjenos`): una spec le
    habla a notas de cualquier tema, igual que un commit.
  - Guía en la skill / texto de surfacing que distinga los dos desenlaces: "la
    creencia quedó vieja → arbitrá" vs "la spec quedó vieja → actualizá la spec
    (spec_update / archive), no ocultes la creencia".

- **No incluye:**
  - Specs **locales** (no `shared`): hoy no se materializan como observación, así
    que quedan fuera de este MVP. Se suman en una fase posterior si la práctica
    lo pide.
  - La dirección inversa **creencia nueva → spec vigente** (al guardar una
    observación, chequear si contradice una spec). Queda fuera; el guard
    protect-as-target igual cubre su riesgo de seguridad.
  - Cualquier **auto-resolución** spec↔creencia. La contradicción semántica no
    es deducible léxicamente (coseno alto entre "usamos X" y "usamos Z"): queda
    `pending` para el agente, como ya hace el AND-gate.
  - Migración de esquema, tabla nueva o discriminador cross-store.
  - Materializar specs por fuera de la proyección unidireccional de F4a.

## Enfoque

F4a ya construyó el puente: `ProjectSpecShared` materializa cada spec `shared`
como una observación **derivada** (`spec-proj:<tenant>:<spec_id>`, topic
`spec/<spec_id>`), y reservó ese namespace explícitamente para F5. La spec ya es
una fila en `observations` — proyección unidireccional, el FS sigue siendo la
verdad. Por eso F5 **reusa la maquinaria obs↔obs que ya existe y está probada**
(detección léxico+coseno, cola `pending`, `musubi_judge`, `markSuperseded`,
aislamiento por tenant, conservación de veredicto) en vez de construir un carril
nuevo. El trabajo es enseñarle a esa maquinaria que la proyección es un
participante **autoritativo**: protegida como target del efecto supersede,
exenta del guardia de dominios, y capaz de surfacear —como source— las creencias
que contradice cuando la spec cambia.

## Impacto

- Áreas/archivos afectados (tentativo, se precisa en design):
  `internal/memory/conflicts.go` (predicado nuevo + guard en el loop de
  `DetectRelations`), `internal/mcp/spec_projection.go` (cablear el save/update
  de la proyección por la detección), posiblemente `internal/mcp/methods.go`
  (texto de surfacing) y la skill `spec-graph`.
- Compatibilidad: **sin migración de esquema**. La proyección ya existe como
  fila. El comportamiento obs↔obs preexistente no cambia para observaciones que
  no son proyecciones de spec (el predicado nuevo sólo matchea el namespace
  reservado).

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| Ocultar una proyección es fútil (se regenera del FS) y confunde | El guard bloquea el *efecto* supersede sobre la proyección; nunca se oculta |
| Falsos positivos ensucian la cola (lección #203 / dominiosAjenos) | Sin auto-resolución: todo queda `pending`; el agente arbitra. Reusa los pisos léxico/coseno ya calibrados |
| Tratar la proyección como `historicalRecord` la volvería append-only y perdería contradicciones nuevas | Categoría propia: autoritativa pero NO append-only — la relación con ella como target SÍ nace `pending`/`conflicts_with`; sólo se bloquea el efecto de ocultarla |
| "Spec stale, creencia correcta": ocultar la creencia sería el error | El surfacing enuncia el desenlace alternativo (actualizar la spec vía spec_update/archive) |
| Cross-tenant: una spec de un proyecto tocando memoria de otro | Reusa el aislamiento por tenant ya existente en `DetectRelations` (guard `projectID`) |

## Criterio de éxito

- Cuando una spec `shared` se proyecta/actualiza y existe una creencia humana que
  la contradice, aparece una relación `pending` con la proyección como source,
  surfaceada para `musubi_judge` — verificado con un test que la detecta.
- Una creencia humana **no puede** ocultar una proyección de spec: un intento de
  `supersedes` con la proyección como target no aplica el efecto — verificado con
  un test que, saboteado, pone en rojo.
- Cero cambio de comportamiento para pares obs↔obs que no involucran una
  proyección de spec — verificado por los tests de conflictos existentes, que
  siguen verdes sin tocarse.
- `go vet ./... && go build ./... && go test -race ./...` en verde (con
  `-count=1`); golden de tools/list regenerado sólo si cambia alguna tool.
