---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f2
status: draft
---

# Propuesta — anclar el spec a su código y avisar cuando el código se movió (spec-graph · F2)

## Intención
F1 creó el destino que faltaba: un `SPEC.md` tipado al lado de cada módulo, navegable como grafo.
Pero ese spec **no sabe a qué código corresponde**, y por lo tanto **no se entera cuando ese código
cambia** — justo la forma en que una descripción del sistema «empieza a mentir» sin que nadie lo note.
F2 cierra ese lazo con el mismo mecanismo que Musubi YA usa para cuidar sus observaciones
(`origin_paths` + `symbolFingerprint`): el spec se ancla al código, y al leerlo se re-deriva la huella
del disco y se **marca** si el código del que habla se movió o desapareció. Es el paso 2 de 5.

## Alcance
- **Incluye (F2):**
  1. **Anclaje** — un campo de frontmatter `anchors: ["ruta#símbolo"]` en el vocabulario de spec-graph.
     La clave `ruta#símbolo` es **la misma** que da el grafo de código: un método se ancla calificado
     (`ruta#Tipo.Metodo`), no pelado, para no fundir homónimos (lección de #326). Sin `#` ancla al
     archivo entero. Preservado lossless (ya forward-compatible desde F1).
  2. **Rancedumbre surface-only** — al leer un spec, re-derivar la huella de cada ancla del disco y
     devolver el spec **marcado** si cambió (`changed`) o ya no existe (`missing`), nombrando cada ruta.
     **MARCA, NUNCA oculta, reordena ni auto-escribe `status: stale`** (decisión 3 del dueño, ya fijada
     en F1/D5: `status:stale` lo pone una persona, ninguna tool lo escribe sola). Reusa la maquinaria
     probada de `internal/memory/origins.go` (`symbolFingerprint`), model-free y tolerante a archivos
     que no compilan.
  3. **Validación al escribir** — `create`/`update` aceptan y validan `anchors`: anclar a una ruta o
     símbolo **inexistente es ERROR al guardar**, no truncado (como `origin_paths`); tope de rutas;
     clave calificada bien formada.
  4. **`validate` reporta la rancedumbre** — suma a los chequeos de F1 los specs con anclas rancias u
     huérfanas, consistente con D9 (la política Musubi vive en la capa de tools, no en el core fiel).

- **No incluye (queda para F3-F5, explícito):** el recall semántico sobre el cuerpo del spec y el
  cross-link con `musubi_code_graph`/`musubi_impact` + overlay de cobertura en `musubi_map` (F3) · la
  federación de gists de spec al central por el outbox (F4) · la arbitración spec↔observación vía
  `musubi_judge`/`supersedes` (F5). Y la **deuda no bloqueante heredada de F1** (tests de repo para la
  contención `os.Root` use-time de los 3 handlers de escritura · `t.Skipf`→`t.Fatalf` en las ramas
  Windows-only · lecturas del índice aún por ruta · promover los tests de overlay a tests de repo)
  **queda fuera del core de F2**: es endurecimiento ortogonal de F1, no la feature de anclaje. Se
  trata como limpieza separada; en `design` se decide si algún ítem es tan barato que vale subirse al
  mismo PR sin inflarlo.

## Enfoque
**Reusar, no reinventar.** El ancla de un spec y el ancla de una observación sobre la misma ruta deben
ser **comparables** — como ya lo son una observación y el índice de código (ambos pasan por
`FileFingerprint`/`symbolFingerprint`). F2 no inventa un segundo mecanismo de ranciedad: extiende el que
existe. La rancedumbre se calcula **después** de construir el grafo/resultado, como advertencia, **nunca
como filtro** (si influyera en la selección dejaría de ser aviso y sería un filtro silencioso — el mismo
invariante que fija `origins.go`).

**La huella de base vive de forma FS-nativa**, coherente con la identidad de spec-graph (índice derivado
del FS, revalidado por `(mtime,size)`, **nunca rancio y sin problema de sync**). Meter un estado paralelo
en SQLite para las anclas de spec iría contra esa identidad y heredaría el peor filo del gotcha de cambio
de rama. **Dónde exactamente queda la huella de base (en el propio frontmatter vs. derivada/efímera) es la
única decisión fina de F2 y se resuelve en `design`** leyendo `origins.go`, `store.go` y el core de
thinkrail — no se improvisa acá.

## Impacto
- Áreas/archivos afectados: `internal/specgraph` (vocabulario: `anchors` en `Fields`; cómputo de
  rancedumbre sobre el índice ya derivado del FS) · `internal/mcp/methods_spec.go` (los handlers de
  lectura surface la marca; `create`/`update` validan `anchors`) · reuso de `internal/memory/origins.go`
  y/o `internal/codeintel` para `symbolFingerprint`. CHANGELOG `[Unreleased]` + bump semántico.
- Compatibilidad: **aditivo**. Un spec sin `anchors` se comporta exactamente como en F1 (opt-in puro,
  como `origin_paths`). Si la huella de base **no** va a DB, **sin migración**. Si `design` decidiera una
  tabla `spec_anchors`, la migración sería idempotente al estilo `observation_origins` (v21). El golden de
  `tools/list` se regenera **sólo si** cambian los params de las tools (probable: `create`/`update` ganan
  `anchors`) — y entonces se re-clasifica, aunque no cambian de clase read-only/write.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Una marca de rancidez que salta siempre se aprende a ignorar, y eso mata la feature | Anclar al **símbolo calificado** (no al archivo entero) para que salte sólo cuando cambia lo que la nota describe (#326); surface-only; distinguir `changed` de `missing` |
| Falso positivo por **cambio de rama** (daa576e9): el disco muestra una rama y una spec anclada a código de otra rama vuelve «ya no existe» | No es bug del mecanismo (hace lo que promete: re-derivar del disco). Se documenta cómo leerlo (`git cat-file -e <rama>:<ruta>`); se recomienda nombrar la rama al anclar; la marca **se cura sola** al mergear a main |
| Estado paralelo que se desincroniza del git | Preferir la huella **FS-derivada** (decisión de `design`); si pese a todo fuera a DB, misma disciplina que `observation_origins`: FK `ON DELETE CASCADE`, re-derivar del disco, y **la huella NO viaja al central** |
| Anclar a lo inexistente deja una spec anclada a medias | **Error al guardar**, no truncado: la media-verdad es justo lo que este mecanismo viene a eliminar |
| Scope creep hacia F3-F5 o hacia la deuda de F1 | Excluido explícito; F2 no toca recall/cross-link/federación/arbitraje, y la deuda de F1 se trata como limpieza separada |

## Criterio de éxito
1. Un spec puede declarar `anchors: ["ruta#símbolo"]`; `create`/`update` lo validan y **anclar a lo
   inexistente falla al guardar** con un mensaje que ofrece la salida (anclar al archivo entero).
2. Al leer, un spec cuyo código anclado cambió o desapareció vuelve **marcado** (`changed` vs `missing`,
   nombrando la ruta), **servido igual** — no se oculta ni se reordena (test que lo fija, porque es lo
   primero que se erosiona).
3. La clave calificada `Tipo.Metodo` matchea receptor+nombre y **no funde homónimos**: tocar `B.Close`
   mueve su huella y **no** marca una spec anclada a `A.Close` (test estilo S5 de #326).
4. `musubi_spec_validate` reporta los specs con anclas rancias u huérfanas, además de los chequeos de F1.
5. Round-trip **lossless** con `anchors` presente: `create`/`update` preservan prosa, comentarios y orden
   (no regresiona F1).
6. Model-free, sin cgo, suite en verde (`GOWORK=off`), con cada invariante nuevo verificado **fallando
   bajo sabotaje dirigido** al invariante que declara.
7. La huella de base **no viaja al central**: un fingerprint sólo vale contra el checkout que lo calculó
   (fijado con un test, como las anclas de observación).

## Rollback
Aditivo. Revertir = quitar el campo `anchors` del vocabulario, el cómputo de rancedumbre y (si `design`
hubiera elegido DB) la tabla y su migración. Los `anchors:` que hubieran quedado en algún frontmatter son
claves inertes preservadas lossless, sin efecto sin el cómputo. Sin migración de datos que deshacer si la
huella quedó FS-derivada.
