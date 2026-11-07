---
id: skills-decididor
type: module-design
status: active
title: El motor elige y exige una skill (decididor)
tags: [cognicion, skills, ola-2-conciencia, track-21]
anchors: [internal/mcp/methods_cognition.go#McpServer.exigirSkillSiCorresponde=4cef7d0b2a0e08a4ba5a3c2bfa219867f28f6beee67dd34c270c956a1b4a3e4c, internal/cognition/elegir_skills.go#ElegirSkill=5b13832bed21d13b30782297e9039a7f04b7dda2b50b0db332d9250cdac857f7, internal/config/effort.go#CognitionConfig.DecididorSkillsOn=9591f7e26c05b2de6d671a1972c6f0e06f1c47017222292c377099f687994cae]
---

# El motor elige y exige una skill (decididor)

## Responsibility

Musubi OFRECE un catálogo de skills y DELEGA la elección al agente. Medido, eso da **0 cargas en 402
turnos**: el catálogo ofrecido casi nunca se toma. El decididor convierte "catálogo ofrecido" en
"orden tomada": dado el texto de la tarea, el motor de conciencia elige **a lo sumo UNA** skill que la
tarea EXIJA —o ninguna— y el servidor la entrega como **exigencia imperativa** montada sobre la
respuesta del recall ("cargá X ahora, para esto"), no como un catálogo delegado. Cuando el texto exige
una skill puntual, la adherencia salta a **7/7**.

Nace **apagado**, detrás de un dial A/B (R2): hasta que el dueño lo encienda no cambia nada. El daemon
local sin central cae a un `NoopProvider` (no-op). **Costo acotado**: a lo sumo 1 llamada al motor por
tarea sustantiva, 0 en las triviales; factura la suscripción vía el proxy, nunca en dólares.

## Interface

**Seam de orquestación** — `exigirSkillSiCorresponde` (hermano de `rerankSiCorresponde`), invocado
dentro de `toolRecall`. La disciplina de *cuándo* gastar una llamada vive acá, en este orden: guard
(Enabled + dial) → gate conservador (`hayMatchFuerte`) → A/B (`BrazoAB`) → caché → freno
(`motorQuota`) → timeout → motor (`ElegirSkill`) → best-effort. Emite el campo estructurado
`ExigenciaSkill` (la línea imperativa, sin cuerpo) y **preserva el eje `loaded`** del A/B (R7): un
turno del brazo de control también se mide.

El insumo del gate `hayMatchFuerte` —los `reflejas`— llega de las **señales declaradas** en
`musubi_recall` (`modified_files`/`phase`/`task`, espejo de `skills.ResolveRequest`): `toolRecall` las
resuelve contra el arsenal YA cargado (`NewResolverDeCatalogo(arsenal).ResolveConDetalle`, sin releer
disco) y le pasa el reflejo al seam. Sin señales el reflejo es `nil` y el gate no corta por match
fuerte —el camino de texto libre de siempre—. Hasta que se cablearon esas señales, `reflejas` salía
**siempre vacío** y el corte estaba dormido en producción; ahora es alcanzable cuando el llamador
declara los archivos que toca, la fase o la tarea.

**Juicio puro** — `ElegirSkill` (en `internal/cognition`): prompt + llamada + parseo + validación
contra el catálogo que vio. **R8 — elección válida**: sólo puede exigir una skill que EXISTE en el
catálogo; una que el motor haya inventado se descarta a "ninguna" (exigir cargar un archivo inexistente
manda al agente a buscar lo que no está —peor que no exigir nada—). Deliberadamente **no cachea, no
fija timeout, no gatea**: esa disciplina es del llamador, idéntico a `Rerank`.

**Dial** — `DecididorSkillsOn` resuelve el puntero: ausente ⇒ apagado (R2). A diferencia del juez,
`ApplyEffort` NO lo toca: el decididor no cuelga del preset de potencia (ni de turbo), se prende aparte.

**A/B y medición** — brazo `On` (tratamiento: el decididor actúa) / `Off` (control: no actúa), con la
tabla `decididor_ab` (migración v59, readCompatible) y `skill_usage.ab_arm`. El **lift** se lee
model-free (`CalcularLift` / `FormatLiftDecididor`) desde `musubi_skill_usage`. El numerador nace local
y el denominador en el central: el lift cross-machine se materializa post-sync, cuando la bajada
entrante trae el denominador a la base local (`MergeDecididorABDesdeCentral`, tool `musubi_sync_pull_ab`;
spec `specs/ab-lift-bajada/`) y el `UPDATE`-only del numerador ya tiene filas sobre las que sumar.

## Dependencies

- **`internal/cognition`** — el motor (`Provider`), `PromptDecididor` y `ElegirSkill`. Calcado del juez
  (`Rerank`): mismo contrato, un solo prompt, un parseo; el molde ya está probado y medido.
- **`internal/config`** — el dial default-off (`DecididorSkillsOn`, `effort.go`).
- **`internal/memory`** — el eje `loaded` de `skill_usage`, el brazo del A/B (`brazo.go`), la tabla
  `decididor_ab` y el lift.
- **`internal/mcp` / `toolRecall`** — el punto de montaje: la exigencia viaja sobre el `RecallResult`.
- El motor se sirve desde el central (`musubi-cerebro`, litellm loopback, model=sonnet). Sin central,
  `NoopProvider`.

Specs en prosa relacionadas (no son nodos del grafo tipado): `specs/skills-que-se-miden/`,
`specs/arsenal-que-se-usa/`, `specs/dial-y-telemetria-cognicion/`, `specs/cache-cognicion/`,
`specs/motor-con-freno/`, `specs/juez-medible/`. Invariantes del decididor: R2/R7/R8/R9/R10, E8/E9.
