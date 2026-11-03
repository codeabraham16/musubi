---
artifact: proposal
schema_version: "1.0"
change: skills-medir-cargas
status: draft
---

# Propuesta — Medir cuándo el agente CARGA una skill

## Intención
`skills-que-se-miden` declaró un límite **a propósito** (`specs/skills-que-se-miden/spec.md:103-104`):
> El camino de `.claude/skills/` no se mide. Ahí quien decide cargar el cuerpo es el consumidor y Musubi no ve esa decisión. Es un límite real; se declara y no se disimula.

Por eso hoy la pregunta *"¿el agente realmente cargó la skill X?"* sólo se contesta leyendo transcripts
offline (así salieron las cifras 7/7 y 0/402 que motivan este trabajo). Esta propuesta **levanta ese
límite**: convierte la carga de una skill en una señal viva y model-free, para medir adherencia de
forma continua en vez de por arqueología.

## Alcance
- **Incluye:** un hook `PostToolUse` sobre la tool `Skill` que registra qué skill cargó el agente,
  como un `Kind` nuevo (`loaded`) en la tabla `skill_usage` existente; un subcomando
  `musubi skill-loaded --hook-mode`; y el contador nuevo visible en `musubi_skill_usage`.
- **No incluye:**
  - Motor/conciencia: **sin decididor, sin nudge, sin modelo**. Eso es Fase 1, explícitamente afuera.
  - `tool_invocations`: NO se toca. La garantía de no guardar argumentos sigue intacta; el hook lee el
    argumento al vuelo y guarda sólo el **nombre** de la skill (público, del catálogo), nunca los
    `args` que el agente le pasó.
  - Utilidad: "¿sirvió?" sigue siendo juicio, fuera de alcance por la misma disciplina del §7.
  - Autocarga del handshake (`alwaysLoad`): no pasa por la tool `Skill`, no se cuenta — y está bien,
    porque medimos las cargas que **el agente decide**.

## Enfoque
Reflejo puro, en el molde que el repo ya tiene. Un hook invoca el binario (como `precheck
--hook-mode`), parsea el argumento de la tool (`tool_input.skill`, molde de `precheckInput`) y
registra un `SkillEvent` por la misma cañería `RecordSkillEvents`/`skill_usage` que ya cuenta
resoluciones. El `Kind` nuevo entra en la whitelist cerrada `kindValido`. Sin tabla nueva, sin tool
MCP nueva.

## Impacto
- **Áreas/archivos:** `internal/memory/skillusage.go` (constante + whitelist + reporte),
  `cmd/musubi/skill_loaded.go` (nuevo), `cmd/musubi/agente_plugin.go` · `setup.go` · `provision.go`
  (registrar el hook), `cmd/musubi/hook_contrato_test.go` (contrato), más los custodios.
- **Compatibilidad:** **sin migración de esquema** — `kind` es columna string acotada por `kindValido`;
  un valor nuevo es una fila nueva, no un cambio de tabla. Opt-in por instalación: sin el hook escrito
  en `settings.json`, byte-idéntico a hoy. Es el **primer** hook `PostToolUse` del setup.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Hook best-effort: un post perdido subcuenta | Para una *tasa* no sesga si las pérdidas son parejas; misma garantía best-effort que el resto de los hooks |
| La clave del argumento podría no ser `skill` en el payload de PostToolUse | Chequeo empírico de 1 vez al implementar: inspeccionar el payload real de `PostToolUse(Skill)` |
| Escritura desde un proceso corto mientras el daemon tiene la base abierta | Molde de los hooks que ya ESCRIBEN (capture); SQLite WAL + `busy_timeout` ya lo manejan |
| Scope creep hacia el decididor (Fase 1) | Declarado fuera; esta propuesta es SÓLO el instrumento |

## Criterio de éxito
Tras instalar el hook, `musubi_skill_usage` muestra —por skill y por proyecto— cuántas veces el agente
la cargó (`loaded`), con un custodio que sabe fallar. La tasa de carga con el catálogo actual queda
disponible como **línea base**, sin leer un solo transcript.

## Rollback
Quitar el hook de `settings.json` (o un `MUSUBI_SKILLLOAD_HOOK=0`) corta la señal sin tocar datos. La
fila `kind=loaded` es inerte si nadie la escribe. Revertir el código no deja esquema colgado: no hubo
migración.
