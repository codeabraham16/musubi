---
artifact: design
schema_version: "1.0"
change: skills-medir-cargas
status: draft
---

# Diseño técnico — Medir cuándo el agente CARGA una skill

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Reusar `skill_usage` con un `Kind` nuevo `loaded` | Tabla nueva; columna nueva | La tabla ya tiene la forma exacta `(skill, project_id, evidence, kind, n)`. Un valor nuevo de `kind` es una fila más: **cero migración** (R10) y el reporte ya la lee |
| D2 | Hook `PostToolUse`, no `PreToolUse` | `PreToolUse(Skill)` | `loaded` es un hecho consumado; `PostToolUse` dispara **después** de correr la tool. `PreToolUse` además puede interferir/bloquear, y queremos algo puramente observacional |
| D3 | Subcomando que escribe **directo a la base** (molde `capture`), no vía daemon/MCP | Tool MCP `musubi_log_skill_load`; mandar al daemon por IPC | Un proceso corto no alcanza el ledger en memoria del daemon. Escribir directo evita depender del daemon, **no agrega golden de `tools/list`** y es un solo upsert diminuto. WAL + `busy_timeout` ya manejan la concurrencia con el daemon |
| D4 | NO tocar `tool_invocations` | Relajarlo para que guarde el arg de `Skill` | Rompe la garantía de privacidad "no se toca". Es exactamente lo que `skill_usage` se creó para evitar (skillusage.go:5-8) |
| D5 | Opt-in por instalación + apagado por `MUSUBI_SKILLLOAD_HOOK=0` | Flag en `config.yaml`; siempre encendido | Sin el hook escrito, byte-idéntico a hoy (R8). El env da un corte rápido, simétrico con `MUSUBI_CODEGRAPH_HOOK=0` |
| D6 | `evidence=""` para `loaded` | Inventar una evidencia | La carga no viene de una resolución; `""` ya es válido (`evidenciaValida`) y deja la fila `(skill,proj,"",loaded)` distinguible de las de resolución |
| D7 | `MarcarCandidata` suma `&& r.Loaded == 0` al caso "muerta" | Dejar `MarcarCandidata` intacto en Fase 0 | Una señal nueva que el reporte contradice en la misma fila es peor que no tener la señal. Con la tabla ya contando `loaded`, el caso "muerta" (`resolved==0 && body_requested==0`) empezaría a rotular muerta a una skill que el agente cargó: una mentira que aparece el instante en que el hook corre una vez. Es un defecto que ESTE cambio introduce, no un hallazgo futuro, así que se arregla acá (R12) |

## Enfoque de implementación
Flujo de datos:
```
agente → tool Skill(skill=X)
  └─ Claude Code dispara PostToolUse(matcher "Skill")
       └─ musubi skill-loaded --hook-mode   (stdin: {tool_name, tool_input.skill, session_id, tool_response})
            ├─ MUSUBI_SKILLLOAD_HOOK=0 ? → no-op, exit 0
            ├─ tool_name != "Skill" || skill == "" ? → no-op, exit 0
            ├─ abre la base (best-effort; error → stderr, exit 0)
            └─ RecordSkillEvents(ctx, []SkillEvent{{Skill:X, ProjectID:proj, Evidence:"", Kind:"loaded"}})
                 └─ upsert en skill_usage  (suma n, toca last_at)
```

Componentes:
- **`internal/memory/skillusage.go`** — `const UsoCargada = "loaded"`; `kindValido` suma `|| k == UsoCargada`. (El resto de `RecordSkillEvents` ya sirve sin cambios: valida `kind`, saltea los inválidos, upsert por lote.)
- **`cmd/musubi/skill_loaded.go`** (nuevo) — `runSkillLoaded()`, calcado de `runPrecheck`/`runCapture`. Separa la **decisión pura** (`leerEventoSkillLoaded(stdin) (evento, aplica bool)`, molde de `leerEventoPrecheck`) del efecto, para testearla sin base. Struct de stdin con `ToolName`, `ToolInput.Skill`, `SessionID`.
- **`cmd/musubi/main.go`** — registra el subcomando `skill-loaded`.
- **`cmd/musubi/agente_plugin.go`** — suma `{"PostToolUse", "Skill", "skill-loaded --hook-mode"}` a `ganchosDelAgente` (lista canónica). Los dos caminos que la materializan (plugin vía `ganchosDelPlugin`; setup repo-por-repo) quedan atados por `TestElPluginLlevaLosMismosGanchosQueSetup`.
- **`cmd/musubi/setup.go` · `provision.go`** — escriben el hook nuevo. `MergeClaudeSettings` es agnóstico al nombre del evento, así que `PostToolUse` entra sin cambios de fondo; idempotente (R7).
- **Reporte** (`FormatSkillUsage` / `toolSkillUsage`, y `SkillUsageRow.Loaded`) — incluye `loaded` junto a `resolved`/`body_sent`/`body_requested` (columna `CARGÓ`), **sin** llamarlo utilidad (R11).
- **`MarcarCandidata`** (skillusage.go) — el caso "muerta" suma `&& r.Loaded == 0` (D7, R12): una skill cargada tiene señal de uso y no puede rotularse muerta.

## Contratos / interfaces
- **`SkillEvent`** para una carga: `{Skill: "<nombre>", ProjectID: "<proyecto local>", Evidence: "", Kind: "loaded"}`.
- **stdin del hook** (subconjunto usado): `{"tool_name":"Skill","tool_input":{"skill":"<nombre>"},"session_id":"..."}`. `tool_response` y el resto se ignoran. La clave del argumento es **`skill`** (parámetro de la tool `Skill`), análogo a cómo `precheck` lee `tool_input.file_path` de `Read`.
- **Fila** en `skill_usage`: `(skill, project_id, "", "loaded", n, first_at, last_at)`.

## Trade-offs
- **Gana:** señal viva y continua; cero migración; cero golden de tools/list; best-effort (no demora al agente); opt-out byte-idéntico; reusa toda la cañería de conteo ya probada.
- **Cede:** best-effort subcuenta si se pierde un post (no sesga una *tasa* si las pérdidas son parejas); no cuenta autocargas del handshake (`alwaysLoad`) porque no pasan por la tool; queda un chequeo empírico de 1 vez del payload real.

## Plan de pruebas
- **`internal/memory/skillusage_test.go`** — `TestUnaCargaSeCuentaEnSuPropioEje` (`kindValido("loaded")` true; 3 `loaded` → `Loaded=3`, los otros ejes en 0); `TestUnaCargaSobreviveAlLadoDeUnKindInvalido` (un `kind` basura al lado de un `loaded` se saltea sin romper el lote — re-verifica el invariante existente para el valor nuevo); `TestUnaSkillCargadaNoEstaMuerta` (`Loaded=5` → NO "muerta"; fila en ceros SÍ "muerta") custodia **R12/D7**; `TestElReporteMuestraLasCargas` (FormatSkillUsage trae la columna `CARGÓ` y el conteo) custodia **R9/R11** del lado del formateo.
- **`cmd/musubi/skill_loaded_test.go`** (nuevo) — `TestLeerEventoSkillLoadedDecide` (decisión pura, sin base, molde de `leerEventoPrecheck`): aplica sólo con `tool_name="Skill"` y `skill` no vacía; tool ajena / skill vacía / sólo espacios → no aplica; `MUSUBI_SKILLLOAD_HOOK=0` → no aplica; JSON inválido → no aplica sin pánico (R4, D5, R6). `TestEscribirSkillCargadaDejaLaFila` (upsert real: `loaded=1`, `resolved=0`, acotado al proyecto del cwd — R1, R3). `TestEscribirSkillCargadaSinBaseNoRompe` (`ErrBaseAusente`, best-effort — R6).
- **`cmd/musubi/skill_loaded_setup_test.go`** (nuevo) — `TestSetupRegistraElHookDeCargaDeSkills` corre `musubi setup` de verdad y verifica que deja el hook `PostToolUse|Skill|… skill-loaded --hook-mode` en `.claude/settings.json`, idempotente a la segunda corrida (**R7**). El custodio del hook es un test de REGISTRO, no `hook_contrato_test.go`: ese contrato es sobre el **sobre** (`eventosQueLlevanContexto`) y este hook no emite sobre. La paridad plugin==setup la cubre `TestElPluginLlevaLosMismosGanchosQueSetup` (ya existente), que corre setup real y compara contra `ganchosDelPlugin`.
- **Reporte (mcp) — R9:** cubierto sin test mcp nuevo. `toolSkillUsage` es un pass-through puro (`SkillUsage→MarcarCandidata→FormatSkillUsage`) cuyo esquema de entrada no cambia; el scoping por `project_id` lo prueba el `TestA7` del spec hermano (invariante intacto, `loaded` viaja por el mismo camino) y el conteo en el reporte lo prueba `TestElReporteMuestraLasCargas`. Un test mcp sólo re-probaría el plumbing genérico de ctx (común a ~40 tools): ceremonia sin señal (musubi-rules #8).
