# Tareas — Niveles en `musubi_resolve_skills`

| # | qué | dónde | invariantes |
|---|---|---|---|
| T1 | `ComoMatcheo`, `SkillResuelta`, `PresupuestoDeCuerpos`, `SeleccionarCuerpos` | `internal/skills/niveles.go` (nuevo) | A4, A5, A6 |
| T2 | `ResolveConDetalle`; `ResolveSkills` delega y no cambia de comportamiento | `internal/skills/resolver.go` | A1, A2, A3, A4 |
| T3 | DTO con tags JSON + `detail` + omisión declarada + hint | `internal/mcp/methods.go` | A7, A8, A9, A10, A11, A12, A13 |
| T4 | `detail` en el schema y en la descripción de la tool | `internal/mcp/registry.go` + golden | — |
| T5 | Pruebas de los 13 invariantes | `internal/skills/niveles_test.go`, `internal/mcp/skills_por_niveles_test.go` | todos |

## Orden y por qué

T1 y T2 son la lógica pura y se prueban sin MCP. T3 es donde el ahorro se vuelve real. T4 es lo que
hace que alguien pueda usar `detail` sin leer el código.

## Sabotaje (cada invariante tiene que saber fallar)

Antes de dar por buena la suite, romper a mano y verificar que la prueba correspondiente se pone en
rojo. Los que importan:

- **A3** — devolver el cuerpo también para `comodin`. Si sigue verde, la prueba no mide el ahorro.
- **A4** — clasificar por la primera rama evaluada en vez de por precedencia.
- **A6** — ordenar por el orden de `LoadSkills` (que es el del `ReadDir`, dependiente del sistema).
- **A7** — filtrar de la lista las skills sin cuerpo en vez de marcarlas.
- **A12** — hacer que un `detail` con typo caiga a `auto`.

> Lección de #274 (invariante A7 de aquel spec): una mutación **vacua** deja el test verde sin que el
> test esté mal. Antes de culpar a la prueba, verificar que la mutación cambia de verdad el
> comportamiento.

## Medición de cierre

Contra el arsenal real de este repo, con `modified_files: ["main.go"]`:

| | hoy | con niveles |
|---|---|---|
| cuerpos devueltos | 11 | los que matchean con evidencia |
| bytes de `rules` | medir | medir |

El número tiene que aparecer en el PR. Sin él, «progressive disclosure» es una palabra.
