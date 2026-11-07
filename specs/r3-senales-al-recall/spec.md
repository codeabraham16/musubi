---
artifact: spec
schema_version: "1.0"
change: r3-senales-al-recall
status: draft
---

# Especificación — Despertar el corte R3: señales declaradas al recall

## Requisitos
Cada requisito es verificable y atómico.

- **S1** — `musubi_recall` DEBE aceptar tres parámetros OPCIONALES: `modified_files` (array de
  strings), `phase` (string) y `task` (string). Su ausencia DEBE ser indistinguible del contrato
  actual.
- **S2** — Cuando el decididor está habilitado y el llamador declara al menos una señal
  (`ResolveRequest.DeclaraAlgo()` es true), `toolRecall` DEBE resolver el reflejo model-free sobre el
  arsenal YA cargado y pasarlo a `exigirSkillSiCorresponde` como `reflejas`.
- **S3** — Cuando el llamador NO declara ninguna señal, `toolRecall` DEBE pasar `reflejas=nil` — el
  comportamiento actual, sin trabajo extra de resolución.
- **S4** — El reflejo DEBE resolverse sin re-leer `.musubi/skills` del disco: DEBE reutilizar el
  arsenal cargado en memoria.
- **S5** — Cuando un reflejo pina una skill con evidencia fuerte (glob que casó un archivo declarado,
  o alcance que casó la fase/tarea declarada), el gate `gatePasaDecididor` DEBE devolver `false` y el
  motor `ElegirSkill` NO DEBE llamarse.
- **S6** — `musubi_recall` DEBE seguir clasificado read-only (`roLeeConEscrituraIncidental`): el
  cambio NO DEBE introducir ninguna escritura nueva sobre las observaciones.
- **S7** — El daemon local (sin central ⇒ `NoopProvider`) NO DEBE tocar el disco de skills ni resolver
  reflejo alguno, aunque el llamador declare señales: el guard `decididorHabilitado()` DEBE cortar
  antes.
- **S8** — El golden de `tools/list` DEBE regenerarse y reflejar EXACTAMENTE los tres params nuevos
  bajo `musubi_recall`, sin otros cambios.

## Escenarios
Formato Given/When/Then (un escenario por comportamiento observable).

### Escenario: señal fuerte apaga el motor (corte vivo en producción)
- **Given** el decididor habilitado (motor real + dial on) y un arsenal con una skill cuyo trigger
  es `*.go`
- **When** se llama `musubi_recall` con `query` sustantiva y `modified_files: ["internal/foo.go"]`
- **Then** `gatePasaDecididor` devuelve false por match-fuerte y `ElegirSkill` NO se invoca (0
  llamadas al motor), y el recall model-free vuelve intacto

### Escenario: alcance declarado apaga el motor
- **Given** el decididor habilitado y una skill con `applies_to: [implement]`
- **When** se llama `musubi_recall` con `query` sustantiva y `phase: "implement"`
- **Then** el reflejo casa por alcance, el gate corta y `ElegirSkill` NO se invoca

### Escenario: sin señales, el camino de hoy
- **Given** el decididor habilitado y un arsenal no vacío
- **When** se llama `musubi_recall` con `query` sustantiva y SIN señales
- **Then** `reflejas` es nil, el gate NO corta por match-fuerte y el motor corre (como hoy)

### Escenario: señal que no pina, el motor igual corre
- **Given** el decididor habilitado y una skill cuyo trigger es `*.py`
- **When** se llama `musubi_recall` con `query` sustantiva y `modified_files: ["internal/foo.go"]`
- **Then** ningún reflejo pina fuerte (el glob no casa), el gate NO corta y `ElegirSkill` se invoca

### Escenario: daemon local ignora las señales
- **Given** el decididor NO habilitado (`NoopProvider` o dial off)
- **When** se llama `musubi_recall` con `query` y `modified_files` poblados
- **Then** no se lee el disco de skills, no se resuelve reflejo y el recall vuelve sin exigencia

### Escenario: regresión del contrato
- **Given** cualquier llamador existente que manda sólo `query` (y quizá `token_budget`/`rerank`)
- **When** se llama `musubi_recall`
- **Then** la respuesta es byte a byte la de antes de este cambio

## Fuera de alcance
- Inferir las señales (git status, fase del SDD, archivos recién tocados): Fork B, no se implementa.
- Cambiar el prompt, el juicio (`ElegirSkill`), el A/B, la caché o el freno del motor.
- Cambiar el default del dial o el comportamiento del daemon local.

## Preguntas abiertas
- [x] ¿Doble lectura de disco? Resuelta: resolver sobre el arsenal en memoria con
      `NewResolverDeCatalogo`, una sola lectura.
- [x] ¿`musubi_recall` se promueve a escritura? No: resolver el reflejo es lectura en memoria; la
      clase `roLeeConEscrituraIncidental` no cambia.
