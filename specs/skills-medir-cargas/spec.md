---
artifact: spec
schema_version: "1.0"
change: skills-medir-cargas
status: draft
---

# Especificación — Medir cuándo el agente CARGA una skill

Contrato observable. Cada requisito es verificable y atómico; cada escenario tiene un custodio que
**sabe fallar**.

## Requisitos

- **R1** — El sistema DEBE registrar un evento con `Kind="loaded"` cuando el agente carga una skill con
  la tool `Skill`, con el **nombre** de la skill y el `project_id` derivado del cwd.
- **R2** — `loaded` DEBE ser un valor válido en la whitelist cerrada `kindValido`; cualquier `Kind`
  fuera de la taxonomía DEBE seguir salteándose (invariante existente, re-verificado para el valor
  nuevo).
- **R3** — El evento `loaded` DEBE llevar `evidence` vacío: la carga NO viene de una resolución (misma
  forma que `body_requested`). La fila resultante es `(skill, project_id, "", "loaded")`.
- **R4** — El hook `skill-loaded --hook-mode` DEBE actuar SÓLO cuando `tool_name == "Skill"` y
  `tool_input.skill` no está vacío. Para cualquier otra tool, o skill vacía, DEBE ser no-op (sin
  escribir, salida 0).
- **R5** — El hook NO DEBE persistir los `args` que el agente le pasó a la skill: sólo el nombre.
  (Privacidad: el nombre es público del catálogo; los args no.)
- **R6** — Un fallo del hook (base no disponible, stdin ilegible, JSON inválido) NO DEBE romper ni
  demorar al agente: best-effort, salida 0, sin ruido en el canal del agente.
- **R7** — `musubi setup` y `musubi provision` DEBEN registrar, de forma **idempotente**, el hook
  `PostToolUse` con matcher `Skill` → `skill-loaded --hook-mode` en `.claude/settings.json`.
- **R8** — Sin el hook registrado, NO se escribe ninguna fila `loaded`: `skill_usage` no gana filas de
  carga y todo contador `loaded` queda en cero (el único escritor de `Kind="loaded"` es el hook). La
  feature es opt-in por instalación. (El reporte SÍ gana la columna `CARGÓ` —y el pie pasa a contar la
  «carga»— y el JSON gana `"loaded":0` aunque el hook no corra —son parte del reporte, no del hook—, así que el viejo
  "byte-idéntico" era literalmente falso y chocaba con R9; lo que de verdad es invariante es que sin el
  hook no hay cargas que contar.)
- **R9** — `musubi_skill_usage` DEBE reportar el contador `loaded` por skill, acotado al `project_id`
  de la credencial (igual que `resolved`/`body_sent`/`body_requested`, invariante A7 del spec hermano).
- **R10** — NO DEBE haber migración de esquema: `loaded` entra en la columna `kind` existente de
  `skill_usage`. El `CREATE TABLE` no cambia.
- **R11** — El reporte NO DEBE llamar "utilidad" a `loaded`. Es un hecho contado (el agente la cargó),
  no un juicio de si sirvió (invariante ★A9 del spec hermano).
- **R12** — Una skill con `loaded>0` NO DEBE rotularse "muerta" por `MarcarCandidata`, aunque nunca haya
  resuelto ni se le haya pedido el cuerpo. "Muerta" significa *ninguna señal de uso*, y una carga es
  señal de uso. (Coherencia: el reporte no puede llamar muerta a algo que el agente cargó. Sin esta
  guarda, la señal nueva se contradiría con el rótulo en la misma fila.)
- **R13** — Una skill con `loaded>0` NO DEBE rotularse "retiro" por `MarcarCandidata`, aunque haya
  resuelto sin que su cuerpo viaje ni se lo pidan. "Retiro" sugiere *sacar* la skill; una que el agente
  CARGA con la tool `Skill` está en uso por el eje `loaded`, independiente del cuerpo del resolver, así
  que proponer retirarla sería un consejo falso. Es la misma incoherencia que R12 cierra para "muerta",
  aplicada al otro patrón que acciona sobre la skill. (El patrón "retiro" exige además `loaded==0`.)

## Escenarios

### Escenario: una carga queda registrada
- **Given** la base abierta y el hook instalado, proyecto `musubi`
- **When** el agente llama `Skill(skill="sdd-flow")`
- **Then** `skill_usage` tiene la fila `(skill="sdd-flow", project_id="musubi", evidence="", kind="loaded", n=1)`

### Escenario: cargas repetidas suman
- **Given** la fila anterior con `n=1`
- **When** el agente vuelve a cargar `sdd-flow`
- **Then** la misma fila queda con `n=2` (upsert, no fila nueva)

### Escenario: una tool ajena es no-op
- **Given** el hook instalado
- **When** llega un evento con `tool_name="Read"`
- **Then** no se escribe nada y el hook sale 0

### Escenario: skill vacía es no-op
- **Given** el hook instalado
- **When** llega `tool_name="Skill"` con `tool_input.skill=""`
- **Then** no se escribe nada y el hook sale 0

### Escenario: la base caída no rompe al agente
- **Given** la base no se puede abrir
- **When** llega un evento `Skill` válido
- **Then** el hook sale 0 sin propagar error (best-effort)

### Escenario: un kind inválido se saltea sin romper el lote
- **Given** un lote con un `SkillEvent{Kind:"loaded"}` y otro `{Kind:"basura"}`
- **When** `RecordSkillEvents`
- **Then** se escribe el `loaded` y se saltea `basura`; el lote no falla

### Escenario: el reporte muestra el contador acotado
- **Given** filas `loaded` de dos proyectos distintos
- **When** `musubi_skill_usage` con la credencial del proyecto `musubi`
- **Then** la respuesta incluye el `loaded` de `musubi` y NO el del otro tenant

### Escenario: una skill cargada no es "muerta"
- **Given** una fila `(skill="x", loaded=5, resolved=0, body_requested=0)`
- **When** `MarcarCandidata`
- **Then** la skill NO se rotula "muerta" (tiene señal de uso: se cargó)
- **And** una fila con los tres contadores en cero SÍ sigue siendo "muerta"

### Escenario: una skill cargada no es candidata a "retiro"
- **Given** una fila `(skill="x", resolved=5, body_sent=0, body_requested=0, loaded=10)`
- **When** `MarcarCandidata`
- **Then** la skill NO se rotula "retiro" (está en uso por el eje `loaded`)
- **And** la MISMA fila con `loaded=0` SÍ se rotula "retiro"

## Fuera de alcance
- El decididor, el motor, el nudge y cualquier llamada a modelo (Fase 1).
- Medir si la skill "sirvió" (juicio, no hecho).
- Contar skills autocargadas por el handshake (`alwaysLoad`): no pasan por la tool `Skill`.
- `tool_invocations`: no se toca (la garantía de no guardar argumentos se mantiene).

## Preguntas abiertas
- [x] Confirmar la clave exacta del argumento en el payload de `PostToolUse(Skill)`: es **`skill`**.
      La tool builtin `Skill` declara dos parámetros, `skill` (nombre) y `args`; el payload de
      `PostToolUse` refleja `tool_input` tal cual, así que la clave es `tool_input.skill`. No hizo
      falta inspección empírica: sale del esquema de la tool.
- [x] `PostToolUse` vs `PreToolUse`: se eligió **`PostToolUse`** (D2). El nombre honesto es "cargada",
      un hecho consumado; `PreToolUse` observaría "va a cargarse" y además podría interferir.
