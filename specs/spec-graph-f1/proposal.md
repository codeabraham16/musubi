---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f1
status: draft
---

# Propuesta — SPEC.md al lado del código: el grafo de specs (spec-graph · F1)

## Intención
Hoy el flujo SDD, en su fase `archive`, manda «fusioná los delta specs en las specs principales» —
pero **esas specs principales por módulo NO EXISTEN**. Los `specs/<cambio>/` de Musubi son un diario
de trabajo por-cambio, no una descripción viva del sistema. El port del `core/` model-free de thinkrail
(`packages/spec-graph`, Apache-2.0) crea ese destino que falta: un `SPEC.md` tipado al lado de cada
módulo, en git, navegable como grafo. Es el paso 1 de 5; sólo construye los cimientos.

## Alcance
- **Incluye (F1):**
  1. Subsistema `spec-graph` **portátil y model-free** (sin pi, sin cgo): detección de «esto es una
     spec» (frontmatter con `id`+`type`), parseo/edición **lossless** del frontmatter, construcción del
     grafo (árbol `parent` + DAG `depends-on`/`references`/`implements` + aristas **inversas** + detección
     de `id` duplicados), e índice **DERIVADO del filesystem** revalidado por `(mtimeMs, size)` — nunca
     rancio. Más `resolveSpecPath` endurecido (NFC, rechazo de symlink, dos puntos/ADS de Windows).
  2. Las **7 tools** `musubi_spec_*`: `grep` · `get` · `graph` · `validate` (read-only) y `create` ·
     `update` · `delete` (write). Tocan **sólo** frontmatter/estructura; la prosa va por el `write`/`edit`
     normal.
  3. El vocabulario de 5 tipos de thinkrail **tal cual**: `goal-and-requirements` → `architecture-design`
     → `module-design` → `submodule-design` → `task-spec`.
  4. Una **SKILL** `spec-graph` y la **regla always-on de 4 líneas** inyectada como sección de system-prompt.
- **No incluye (queda para F2-F5, explícito):** el anclaje `anchors: ["ruta#símbolo"]` y la detección de
  rancedumbre (F2) · el recall semántico sobre cuerpos de spec y el cruce con `musubi_code_graph`/`musubi_impact`
  + overlay de cobertura en `musubi_map` (F3) · la federación de gists de spec al central por el outbox (F4) ·
  la arbitración spec↔observación vía `musubi_judge`/`supersedes` (F5). F1 **no toca** el diario
  `specs/<cambio>/`, que sigue siendo `task-spec`.

## Enfoque
Port directo del `core/` de thinkrail a Go, respetando su diseño: la spec vive **al lado del módulo**
(fractal), el grafo y el índice se **derivan** del disco (no hay estado paralelo que se desincronice), y
las tools son cirugía de frontmatter, no de prosa. Edición **lossless** con `gopkg.in/yaml.v3` (`yaml.Node`
preserva comentarios y orden de claves); NFC con `golang.org/x/text/unicode/norm` (ya está en
`internal/embedding`). Las tools se registran en el estilo de `internal/mcp/registry.go`. El esquema de
frontmatter es **nuevo e independiente** del `schema_version:"1.0"` de las plantillas SDD: «es una spec» =
hay `id`+`type`, así que un `proposal.md` del diario SDD **no** se confunde con una spec.

## Impacto
- Áreas/archivos afectados: nuevo `internal/specgraph/` (core portátil) · `internal/mcp/registry.go` +
  7 handlers · el golden de `tools/list` (regenerar) · nueva SKILL en `.claude/skills/` · el inyector de
  la regla always-on (mecanismo exacto = decisión de la fase design).
- Compatibilidad: **puramente aditivo**. Tools nuevas, subsistema nuevo, archivos `SPEC.md` nuevos y
  opcionales en disco. **Sin migración de datos.** Sin las tools, un `SPEC.md` es markdown inerte.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Scope creep hacia F2-F5 | F1 excluye explícito anclaje/rancedumbre/recall/federación/arbitración; la clave `anchors` se difiere pero el esquema queda forward-compatible por la edición lossless (una clave desconocida se preserva) |
| La edición lossless es sutil (comentarios, orden, anchors YAML) | `yaml.Node` + test de round-trip saboteable (mutar prosa/orden y exigir rojo) |
| `resolveSpecPath`: escape por symlink, dos puntos/ADS de Windows, NFC | portar el endurecimiento de thinkrail con un test por cada vector |
| 7 tools de golpe inflan el golden y arriesgan mal-clasificar read-only | musubi-rules: regenerar golden + clasificar cada una en `registry.go` (grep/get/graph/validate read-only; create/update/delete write) — marcar readOnly algo que muta es un lost-update que `-race` no ve |
| Chocar el esquema nuevo con el `schema_version:"1.0"` de SDD | esquema independiente; «es una spec» = `id`+`type`, el frontmatter SDD no lo satisface |

## Criterio de éxito
1. Las 7 `musubi_spec_*` existen, están clasificadas (4 read-only / 3 write) y el golden de `tools/list`
   está regenerado y en verde.
2. Un `SPEC.md` con `id`+`type` al lado de un módulo se detecta como spec; un `proposal.md` del diario SDD **no**.
3. El grafo arma árbol (`parent`) + DAG (`depends-on`/`references`/`implements`) + aristas inversas y marca
   `id` duplicados, derivado del FS y revalidado por `(mtime, size)`.
4. Round-trip lossless: `create`/`update` preservan prosa, comentarios y orden de claves (test saboteable).
5. `resolveSpecPath` rechaza escape por symlink y dos puntos/ADS de Windows, y normaliza a NFC (un test c/u).
6. SKILL `spec-graph` + regla always-on de 4 líneas presentes.
7. Model-free, sin cgo, suite en verde (build con `GOWORK=off`).

## Rollback
Aditivo y sin datos. Revertir = quitar el subsistema, los 7 handlers, las entradas del golden y de
`registry.go`, la SKILL y la regla. Los `SPEC.md` que hubieran quedado en disco son markdown inerte sin
las tools; no hay migración que deshacer.
