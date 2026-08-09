# Tareas — El arsenal se mide

| # | qué | dónde | invariantes |
|---|---|---|---|
| T1 | Tabla `skill_usage` + migración | `internal/memory/database.go`, `migrations.go` | — |
| T2 | `SkillEvent`, `RecordSkillEvents` (upsert), `SkillUsageCtx` (acotado) | `internal/memory/skillusage.go` (nuevo) | A7 |
| T3 | El buffer: `usageLedger` gana un segundo lote | `internal/mcp/usageledger.go` | A4, A5, A6 |
| T4 | Conteo en `resolve_skills` y en `list_skills` | `internal/mcp/methods.go` | A1, A2, A3 |
| T5 | `musubi_skill_usage` + las candidatas | `internal/mcp/methods.go`, `registry.go` + golden | A8, A9, A10 |
| T6 | Pruebas de los 10 invariantes | `internal/memory/skillusage_test.go`, `internal/mcp/skills_que_se_miden_test.go` | todos |

## Orden y por qué

T1–T2 son el dato. T3 es la cañería, y va antes que T4 porque T4 sin buffer escribiría a disco bajo
`dispatchMu`. T5 es lo único que alguien ve.

## Sabotaje

- **A2** — contar `body_sent` también para los que entraron por comodín. Si sigue verde, la prueba no
  distingue las dos cosas y la lectura de retiro es humo.
- **A3** — contar `body_requested` también con `query` vacío.
- **A4** — dejar que el error del sink se propague.
- **A7** — leer sin filtrar por `project_id`.
- **A8** — devolver sólo las skills con filas.

> Verificar que **la mutación se aplicó** antes de creerle a un verde. En #277 dos sabotajes pasaron
> en verde por regexes que no matchearon, no por pruebas malas.

## Medición de cierre

Correr el arsenal real contra los tres patrones y decir en el PR **cuántas skills caen en cada
lectura**. Si ninguna cae en ninguna, el dato no sirve todavía y hay que decirlo igual.
