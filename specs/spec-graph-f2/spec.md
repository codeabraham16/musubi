---
artifact: spec
schema_version: "1.0"
change: spec-graph-f2
status: draft
---

# Especificación — spec-graph · F2 (anclaje + rancedumbre)

> Vocabulario RFC 2119: **DEBE** / **NO DEBE** / **DEBERÍA** / **PUEDE**. Cada requisito es verificable y
> atómico, y está pensado para tener su test saboteable en la fase `tasks`/`implement`. Numeración propia
> de F2 (R1…): el artefacto queda scopeado por `change: spec-graph-f2`.

## Requisitos

### Anclaje — esquema y escritura
- **R1** — El esquema de frontmatter de una spec DEBE sumar la clave `anchors`: una **lista** de strings,
  cada uno `ruta` o `ruta#símbolo`, relativos a la raíz del repo. `anchors` es **opt-in puro**: una spec
  sin `anchors` DEBE comportarse exactamente como en F1 (esto promueve la clave que F1/R4 difirió y dejó
  forward-compatible). `anchors` **NO** es una arista del grafo (como `covers`/`tags`).
- **R2** — `musubi_spec_create` y `musubi_spec_update` DEBEN aceptar `anchors` y **validar cada ancla al
  guardar**: la ruta DEBE existir en disco (archivo, no directorio) y, si el ancla lleva `#símbolo`, ese
  símbolo DEBE existir como símbolo top-level en ese archivo. Anclar a una ruta o símbolo **inexistente
  DEBE ser ERROR al guardar** —no truncado, no guardado a medias—, con un mensaje que ofrezca la salida
  (anclar al archivo entero). El número de anclas por spec DEBE tener un tope (reusar `maxOriginPaths` = 10).
- **R3** — La clave de símbolo DEBE interpretarse con la **misma regla calificada que el grafo de código**
  (la de `symbolMatches`, lección de #326): `ruta#Tipo.Metodo` matchea receptor **y** nombre y por lo
  tanto **NO DEBE** fundir homónimos; `ruta#Nombre` pelado matchea el comportamiento histórico (función,
  tipo o método de cualquier receptor). La huella de un símbolo DEBE ser **model-free** (extractor
  `codeintel`, tolerante a archivos que no compilan), nunca los rangos del grafo persistido.

### Rancedumbre — lectura, surface-only
- **R4** — Al leer una spec, el sistema DEBE re-derivar del disco la huella de cada ancla y compararla con
  la huella que la spec registró para ese ancla. Si difieren, la spec DEBE volver marcada `changed`
  nombrando la ruta; si la ruta o el símbolo anclado **ya no existe**, DEBE volver marcada `missing`. La
  marca DEBE distinguir `changed` de `missing` (un borrado/renombre es la deriva más fuerte).
- **R5** — La marca de rancedumbre DEBE ser **surface-only**: **NO DEBE** ocultar la spec, **NO DEBE**
  reordenarla ni descartarla, y **NO DEBE** auto-escribir `status: stale` en el frontmatter (decisión 3 del
  dueño, ya fijada en F1/D5: `status:stale` lo pone una persona). La spec DEBE devolverse con los mismos
  campos que sin marca, más un campo estructurado de rancedumbre y, donde el lector lo necesite, una
  advertencia legible antepuesta.
- **R6** — Un error de E/S que **no** sea «no existe» (disco ocupado, permiso momentáneo) **NO DEBE**
  marcar la spec: no es evidencia de deriva, y marcar por eso entrenaría a ignorar la marca.
- **R7** — La marca DEBE ser **precisa al símbolo**: anclada a `ruta#A.Foo`, cambiar **otro** símbolo del
  mismo archivo **NO DEBE** marcar, y **desplazarle las líneas** al símbolo anclado sin cambiar su
  contenido **NO DEBE** marcar (invariante estilo S5 de #326 / line-shift de #252).

### Validación
- **R8** — `musubi_spec_validate` DEBE **sumar** a los chequeos de F1 (dangling / duplicate / parent-cycles
  / type desconocido / `SPEC.md` no-spec) el reporte de specs con anclas rancias (`changed`) u huérfanas
  (`missing`), nombrando spec y ruta. La política vive en la capa de tools (frontera D1 de F1); el core
  `ValidateGraph` queda fiel.

### No-funcionales e identidad
- **R9** — La huella de base de un ancla **NO DEBE** viajar al cerebro central: un fingerprint sólo vale
  contra el checkout que lo calculó (mismo invariante que las anclas de observación, fijado con un test).
- **R10** — El cambio DEBE ser **aditivo** y **opt-in**: una spec sin `anchors` DEBE comportarse idéntico a
  F1. Si el diseño **no** introduce tabla nueva, **NO DEBE** haber migración; si la introduce, la migración
  DEBE ser idempotente al estilo `observation_origins` (v21). El subsistema DEBE seguir **model-free** y
  **sin cgo**; el build DEBE pasar con `GOWORK=off` y la suite con `-race`.
- **R11** — El round-trip DEBE seguir siendo **lossless** con `anchors` presente: `create`/`update` con
  `anchors` DEBEN preservar prosa (byte-a-byte), comentarios y orden de claves (no regresiona F1/R5).
- **R12** — Si cambian los parámetros de `create`/`update` (ganan `anchors`), el golden de `tools/list`
  DEBE regenerarse y `TestToolsListGolden` quedar en verde. La **clasificación** read-only/write **NO DEBE**
  cambiar: `create`/`update` siguen write; `get`/`grep`/`graph`/`validate` siguen read-only.

## Escenarios

### Escenario: anclar y validar al guardar
- **Given** un archivo `internal/foo/bar.go` con una función top-level `Baz`
- **When** `musubi_spec_create` crea un `SPEC.md` con `anchors: [internal/foo/bar.go#Baz]`
- **Then** se crea la spec; y si en cambio se intenta `anchors: [internal/foo/bar.go#NoExiste]`, falla al
  guardar con un mensaje que nombra el símbolo ausente y ofrece anclar al archivo entero

### Escenario: el código anclado cambió
- **Given** una spec anclada a `internal/foo/bar.go#Baz`, válida al guardarse
- **When** alguien edita el cuerpo de `Baz` y luego se lee la spec con `musubi_spec_get`
- **Then** la spec vuelve marcada `changed` nombrando `internal/foo/bar.go#Baz`, servida igual (mismos
  campos, mismo `status`, sin reordenar)

### Escenario: el símbolo anclado desapareció
- **Given** una spec anclada a `internal/foo/bar.go#Baz`
- **When** se borra o renombra `Baz` y se lee la spec
- **Then** la spec vuelve marcada `missing` para ese ancla

### Escenario: precisión al símbolo (no salta por lo ajeno)
- **Given** una spec anclada a `internal/foo/bar.go#A.Close`, con `B.Close` en el mismo archivo
- **When** se modifica `B.Close` (y no `A.Close`), o se le desplazan las líneas a `A.Close` sin tocar su
  contenido
- **Then** la spec **no** vuelve marcada

### Escenario: surface-only, nunca filtro
- **Given** una spec marcada `changed`
- **When** se la lee
- **Then** vuelve en la misma posición y con el mismo `status` que sin marca; la rancedumbre es un campo
  adicional, no un filtro ni un cambio de estado

### Escenario: validate reporta la rancedumbre
- **Given** un repo con una spec anclada a código que cambió y otra a un archivo borrado
- **When** `musubi_spec_validate` corre
- **Then** reporta la primera como `changed` y la segunda como `missing`, además de los chequeos de F1

### Escenario: la huella no viaja al central
- **Given** una spec con `anchors` y su huella de base
- **When** se federa/sincroniza lo que corresponda al cerebro central
- **Then** la huella de base **no** se envía (sólo vale contra el checkout local)

## Fuera de alcance
- Recall semántico sobre el cuerpo del spec y cruce con `musubi_code_graph`/`musubi_impact` + overlay de
  cobertura en `musubi_map` (F3).
- Federación de gists de spec al cerebro central por outbox (F4). F2 sólo garantiza que la **huella** no
  viaja (R9); el envío de gists de spec es F4.
- Arbitraje de conflictos spec↔observación por `musubi_judge`/`supersedes` (F5).
- Auto-escritura de `status: stale` (prohibida por R5; es decisión de una persona).
- La **deuda no bloqueante heredada de F1** (tests de repo para la contención `os.Root` use-time de los 3
  handlers de escritura, `t.Skipf`→`t.Fatalf` en ramas Windows-only, lecturas del índice por ruta,
  promover overlay tests): endurecimiento ortogonal, tratado como limpieza separada.

## Preguntas abiertas
- [ ] **Dónde vive la huella de base de cada ancla** (el fork central de F2, flaggeado al dueño en el
  roadmap): (a) **en el propio frontmatter** del `SPEC.md` → FS-native, co-viaja por git, sin DB, sin
  migración, preserva la identidad «índice derivado del FS, sin sync», pero el frontmatter carga un digest
  gestionado por las tools; (b) **tabla `spec_anchors`** en `memory.db` → reusa `origins.go` casi verbatim,
  pero la base **NO co-viaja por git** (rancedumbre ciega en una máquina que no ancló) y es estado paralelo
  contra la identidad FS-derived. → **design, con recomendación y consulta al dueño**.
- [ ] **Semántica de (re)captura de la huella**: ¿se captura/re-captura en `create`/`update` del ancla
  (una re-captura = «el autor vuelve a dar fe del código a este estado»)? → design.
- [ ] **Superficie de la marca**: ¿sólo `get` y `validate` cargan la rancedumbre, o también `grep`/`graph`
  por-nodo? → design (coherente con el costo de re-derivar del disco).
- [ ] **Dónde se computa**: ¿sobre el índice ya derivado del FS (cacheado por `(mtime,size)`) o fresco por
  llamada? → design (coherencia con F1/R10).
- [ ] **Reuso de `symbolFingerprint`**: ¿se exporta/factoriza desde `internal/memory`/`internal/codeintel`
  o `specgraph` usa `codeintel` directo? → design (sin duplicar la regla de matcheo calificado).
