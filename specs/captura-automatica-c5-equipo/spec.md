# Especificación — Captura Automática de Equipo (C5)

> SDD · fase **spec** · change `captura-automatica-c5-equipo`
> Vocabulario RFC 2119: **DEBE** / **DEBERÍA** / **PUEDE**. Requisitos verificables y atómicos.

## Invariante rector

Con **team mode OFF** (default), el comportamiento **DEBE** ser idéntico bit-a-bit al actual:
captura `local`, `author=''`, recall solo local. C5 es aditivo y opt-in por proyecto.

---

## R1 — Team mode (configuración)

- **R1.1** El proyecto **DEBE** poder marcarse "team mode" por configuración (bloque
  `capture.team_mode: true`, o equivalente `scope.default_shared`). Default: **off**.
- **R1.2** Con team mode **off**, el scope default de toda captura **DEBE** ser `local`.
- **R1.3** El valor **DEBE** derivar de la config del server/proyecto, nunca de un parámetro
  del cliente MCP.

## R2 — Auto-escritura al central  ·  (sub-fase C5.2)

- **R2.1** Con team mode **on**, una captura cuyo `scope` **no** viene especificado **DEBE**
  persistirse con scope `shared`.
- **R2.2** Toda captura que resulte `shared` **DEBE** pasar por la redacción de secretos (C2)
  **antes** de que su contenido cruce al central (fail-closed).
- **R2.3** Las rutas de captura automática —C1 (`save_observation` proactiva del agente),
  C3 (commits, hook `Stop`), C4 (error→fix)— **DEBEN** honrar el scope default del proyecto.
- **R2.4** El encolado en el outbox de una captura `shared` **DEBE** ser idempotente por
  `obs_id` (no duplica en reintentos).
- **R2.5** Una captura con `scope` **explícito** (`local` o `shared`) **DEBE** respetarse tal
  cual, ignorando el default (escape hatch).

## R3 — Atribución por persona  ·  (sub-fase C5.1 — corte foundational)

- **R3.1** Cada observación **DEBE** tener un campo `author` (`TEXT NOT NULL DEFAULT ''`).
- **R3.2** El `author` **DEBE** derivarse de la **credencial** (`principal.Name`), nunca de un
  parámetro del cliente.
- **R3.3** En el central, al ingerir una observación `shared`, el `author` **DEBE** sellarse
  desde la credencial de sync del principal que la empuja — **autoridad server-side**; el
  `author` del payload entrante **DEBE** ignorarse.
- **R3.4** La migración que agrega `author` **DEBE** ser **aditiva** (`ADD COLUMN`, sin rebuild),
  en su propia transacción, detrás de la guarda de esquema `user_version`.
- **R3.5** El recall e `insights` **DEBERÍAN** poder atribuir/filtrar por `author` (exponer el
  autor en el resultado; filtro opcional `author=`).
- **R3.6** Con `author=''` (memoria pre-C5 o modo legacy federado), el comportamiento **DEBE**
  ser el actual (sin atribución); las filas viejas no se rompen.

### Escenarios (C5.1)

```gherkin
Escenario: atribución sellada desde la credencial
  Dado un proyecto en team mode con principals Ana(writer) y Juan(writer)
  Cuando Ana captura una observación desde su máquina y el outbox la sincroniza al central
  Entonces la observación en el central tiene author = "ana"

Escenario: el payload no puede falsificar el autor
  Dado un cliente que sincroniza con la credencial de Juan
  Y un payload que intenta declarar author = "ana"
  Cuando el central ingiere la observación
  Entonces author = "juan"  (el payload se ignora; se sella desde la credencial)

Escenario: backward-compat con team mode off
  Dado un proyecto con team mode off
  Cuando se captura una observación
  Entonces author = "" y scope = "local"  (comportamiento actual, sin cambios)

Escenario: migración aditiva no rompe filas viejas
  Dada una base pre-C5 con observaciones sin columna author
  Cuando se aplica la migración user_version que agrega author
  Entonces las filas existentes tienen author = "" y siguen siendo legibles por el recall
```

## R4 — Auto-recall federado  ·  (sub-fase C5.3)

- **R4.1** Con team mode **on**, el recall automático (hook `turn`/SessionStart) **DEBE**
  incluir la memoria central del `project_id`, además de la local.
- **R4.2** Si el central no responde dentro de un **timeout corto**, el recall **DEBE** degradar
  al resultado local (fail-safe) sin bloquear el turno ni emitir error al usuario.
- **R4.3** El recall federado **DEBE** respetar el presupuesto de tokens (fusiona local + central
  dentro de `recall_token_budget`).
- **R4.4** El recall federado **NO DEBE** exponer memoria de otro `project_id` (respeta el
  aislamiento sellado por contrato de T19).

### Escenarios (C5.3)

```gherkin
Escenario: continuidad entre máquinas
  Dado que Ana capturó la observación X en la PC (ya sincronizada al central)
  Cuando Juan abre el MISMO proyecto en su máquina
  Entonces el recall automático de Juan incluye X, atribuida a author = "ana"

Escenario: degradación offline-first
  Dado el cerebro central inalcanzable (red caída)
  Cuando se abre el proyecto y corre el recall
  Entonces devuelve solo la memoria local, sin error visible ni bloqueo del turno
```

## R5 — Limpieza a escala  ·  (sub-fase C5.4)

- **R5.1** El central **DEBE** deduplicar casi-duplicados (trigram-Jaccard ≈ 0.8; SimHash
  Hamming ≤ 3 **PUEDE** sumarse si escala) durante el mantenimiento.
- **R5.2** El decay/olvido **DEBE** aplicarse a la memoria central para gestionar volumen; las
  memorias marcadas explícitas **DEBERÍAN** quedar exentas del auto-archivado.
- **R5.3** El mantenimiento **DEBE** correr **off the hot path** (job `maintain`, no en cada
  escritura).

## Criterios de aceptación globales

- **AC1** `go test ./...` verde en las 3 plataformas (incluye tests nuevos de R3 y R4).
- **AC2** Golden test del registro intacto (C5 no cambia el catálogo de tools, salvo que se
  agregue un parámetro `author`/`team` opcional — en cuyo caso se actualiza el golden a conciencia).
- **AC3** Un test de barrido confirma que team mode **off** deja el comportamiento idéntico
  (sin author, scope local, recall local).
- **AC4** Un test de aislamiento confirma R4.4 (el recall federado no cruza `project_id`).
- **AC5** `lint` limpio; migración aditiva con test de tripwire de `latestSchemaVersion`.

## No-goals (recordatorio de la propuesta)

Carril privado explícito · extracción model-free de prosa · UI de atribución · ACLs
por-memoria · federación inter-organización. Ninguno bloquea C5; todos son sumables sin
reescritura.
