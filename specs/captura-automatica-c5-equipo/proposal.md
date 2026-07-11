# Propuesta — Captura Automática de Equipo (C5)

> SDD · fase **proposal** · change `captura-automatica-c5-equipo`
> Rol: Proponente — intención y valor antes que código; alcance delimitado; escéptico del scope creep.

## Intención

Cerrar el **último eslabón** de la visión de captura automática: que la memoria de un
proyecto sea **central por naturaleza** y **siga al proyecto** entre máquinas y personas —
sin pedir mandatos de "guardá esto" ni de "compartí esto".

Hoy las fases C1–C4 capturan **local**; nada cruza solo. C5 hace que, para un proyecto
marcado *"de equipo"*, **todo lo capturado fluya automático al cerebro central** (con
redacción + dedup), **con atribución por persona**, y se **auto-recupere** al abrir el
proyecto en otra máquina.

Escena objetivo (verbatim del usuario): *"esta PC ahorita hablando de esto, y me voy con la
laptop a otro sitio, y tener info en la memoria de Musubi de este proyecto… para poder seguir
trabajando desde la laptop lo que se habló en la PC"* — y lo mismo para un empleado en su
propio equipo.

## Contexto — por qué ahora

C5 no inventa infraestructura: **conecta** piezas ya construidas y endurecidas.

| Pieza que C5 reusa | Estado |
|---|---|
| Recall automático (hook `turn`/SessionStart) | ✅ en producción |
| Captura local automática (C1 proactiva, C3 commits, C4 error→fix) | ✅ mergeado |
| Cerebro central + aislamiento por `project_id` | ✅ Track 17–19 (sellado por contrato) |
| Outbox offline-first + sync saliente | ✅ F2 |
| Redacción de secretos model-free | ✅ C2 |
| Roles reader/writer/admin + principals | ✅ |
| Supersesión bi-temporal (conflictos) | ✅ Track 13 |

**Decisiones del usuario (2026-07-10) que fijan el diseño:**
1. **Atribución por persona = SÍ** (saber si lo aportó Juan o Ana, no solo "del proyecto").
2. **Política "capturar todo + limpiar después"** (dedup + decay), no un gate de salencia
   restrictivo al escribir.
3. Hacerlo **con rigor** ("esto hay que hacerlo bien").

## Alcance — qué SÍ

- **Team mode por proyecto.** Una marca de configuración que hace que el *scope default* de
  la captura pase de `local` a `shared` (central) para ese proyecto.
- **Auto-escritura al central.** Las rutas de captura (C1 proactiva, C3 commits, C4 error→fix,
  y `save_observation` con scope vacío) **honran el scope default**; en team mode escriben
  `shared` → redacción (C2) → outbox → central. Cero motor nuevo: reusa el borde local→shared.
- **Atribución por persona.** Nuevo campo `author` en las memorias, **derivado de la
  credencial** (principal), nunca del cliente; se **sella en el central al ingerir**. Habilita
  "mostrame lo de Ana" y contribuciones firmadas.
- **Auto-recall federado.** Al abrir el proyecto en otra máquina, el recall automático trae la
  memoria central del `project_id` **además** de la local — continuidad PC↔laptop↔empleado.
- **Limpieza a escala.** Dedup (trigram-Jaccard / SimHash) + decay corriendo sobre el central,
  para que "capturar todo" no degenere en ruido con varias manos escribiendo.

## Fuera de alcance — qué NO (por ahora)

- **Carril privado explícito** ("privado mío que no cruza"). Con atribución + política
  capture-all queda diferido; se puede sumar después **sin reescritura**.
- **Extracción model-free de prosa.** El extractor sigue siendo el agente (C1). C5 **no toca la
  identidad model-free**.
- **UI de "quién aportó qué".** La atribución habilita el dato; su visualización es del
  brain-dashboard, no de este cambio.
- **ACLs por-memoria.** Se apoya en los roles existentes (reader/writer/admin); no inventa
  permisos por observación.
- **Federación inter-organización.** Un cerebro por organización; federar entre cerebros es
  horizonte lejano.

## Estrategia de rollback

- **Todo aditivo y gateado por config.** `team_mode` **off por default** ⇒ comportamiento
  actual bit-a-bit (captura local, sin author, recall local). Un proyecto personal no cambia.
- **`author` = columna nueva** `TEXT NOT NULL DEFAULT ''` (migración aditiva por `user_version`,
  sin rebuild). Desactivar la feature deja la columna inerte; ninguna lectura la exige.
- **Auto-recall federado fail-safe.** Si el central no responde o falla, degrada al recall
  local (offline-first). Nunca bloquea un turno.
- **Sub-fases = PRs independientes y reversibles.** El outbox / redacción / scope existentes no
  cambian su contrato; solo se les cambia el *default* de scope por config.

## Fases propuestas (para spec/design)

- **C5.1 — Atribución (`author`).** Schema + threading desde la credencial + carry en el sync +
  sello en el central. Pequeño, foundational, con valor independiente. *Arranca por acá.*
- **C5.2 — Team-mode auto-shared.** Scope default por proyecto; las rutas de captura lo honran.
- **C5.3 — Auto-recall federado.** El recall local trae el central del `project_id`.
- **C5.4 — Limpieza a escala.** Dedup + decay en el central; gestión de volumen.

## Riesgos anticipados

- **Ruido con capture-all + varias personas** → mitiga la limpieza (C5.4) y la atribución
  (poder filtrar por persona).
- **Spoofing de atribución** → se sella en el central desde la credencial de sync, no del payload.
- **Latencia del auto-recall federado** → timeout corto + degradación a local; nunca en el hot path del turno.
- **Confusión de identidad multi-máquina** (misma persona en PC y laptop) → llavear author por
  identidad del principal, no por máquina.
