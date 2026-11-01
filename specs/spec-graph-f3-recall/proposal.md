---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f3-recall
status: draft
---

# Propuesta — spec-graph F3-A: recall semántico sobre specs

## Intención
Hoy una spec sólo se encuentra por **texto literal**: `musubi_spec_grep` hace substring o
regex (query.go:3 lo dice explícito: «nada de semántica, eso es F3»). Si pregunto por «la spec
de la política de reintentos» y el `SPEC.md` dice «espera exponencial entre intentos», el grep
no la encuentra. F3-A cierra ese hueco: **encontrar una spec por significado**, que es el
delta (2) del roadmap spec-graph («recall SEMÁNTICO sobre el body de los specs»).

## Alcance
F3 del roadmap bundlea dos capacidades independientes —recall (delta 2) y cross-link (delta 3)—
que tocan caminos distintos del sistema. Se parten en dos features/PR, igual que F2-A/F2-B:

- **Incluye (F3-A, esta propuesta):**
  - Una tool nueva **read-only** `musubi_spec_recall(query, limit?, type?, offset?)` que devuelve
    las specs más relevantes por significado al query, rankeadas, leyendo los bodies del FS.
  - **Model-free con degradación obligatoria:** con embedder configurado, rankea por similitud
    (coseno); **sin** embedder, cae a un ranking por keyword determinista y **nunca** falla ni
    exige el embedder. Reusa la maquinaria de embeddings + fallback que ya tiene el recall de
    observaciones, no la reinventa.
  - **Identidad FS-nativa preservada:** lo que se indexa se deriva del disco y se revalida por la
    misma huella `(mtime,size)` del índice de F1 → nunca rancio, sin sync, sin problema de
    federación. El `SPEC.md` git-trackeado no se muta.
  - Golden de `tools/list` regenerado + clasificación read-only en `registry.go`.
  - Entrada en `CHANGELOG [Unreleased]` + bump semántico.

- **No incluye (explícito):**
  - **Cross-link** con `musubi_code_graph`/`musubi_impact` y overlay de cobertura en
    `musubi_map` → es **F3-B**, feature/PR hermana, construida sobre la misma clave
    `ruta#símbolo` de las anclas de F2 (`anchors.go`).
  - **Fundir las specs en `musubi_recall`** (el recall de observaciones): se descarta a propósito.
    Mezclar los dos corpus acoplaría spec-graph al motor de memoria (las specs tendrían que
    espejarse en `memory.db`), reintroduciendo el problema de sync/ranciedad que todo el
    subsistema fue diseñado para evitar. El recall de specs es **su propia superficie**.
  - Federación de gists de specs al central (F4); arbitraje spec↔observación (F5).

## Enfoque
Un recall semántico paralelo al de observaciones, pero **contenido dentro de `internal/specgraph`**
y alimentado del FS, no de `memory.db`:

1. Reunir los bodies de las specs (ya hay `SpecContentEntry` en `query.go`, con `Path`, `Content`,
   `Frontmatter` — la misma colección que alimenta `GrepSpecs`).
2. Rankear por relevancia al query: coseno sobre vectores si hay embedder; si no, señal por keyword
   determinista. La elección del embebedor pasa por el **mismo portero de privacidad** que ya filtra
   qué se embebe de las observaciones (no reinventar la política).
3. Devolver specs rankeadas (id, type, title, ruta, score, snippet), con filtro opcional por `type`
   y paginado, espejando la forma de `GrepResult`.

Dónde y cómo se cachean los vectores (al vuelo vs. cache keyed por `(mtime,size)`+embedder, en
memoria vs. en una tabla análoga a `spec_anchors`) es **decisión de la fase design**; el invariante
que no se negocia es: FS-nativo (fresco por construcción) y model-free (fallback sin embedder).

## Impacto
- **Áreas/archivos afectados:** `internal/specgraph/` (archivo nuevo para el recall semántico +
  tests); `internal/mcp/methods_spec.go` (handler `toolSpecRecall`); `internal/mcp/registry.go`
  (registrar la tool, clasificarla read-only); golden de `tools/list`; `CHANGELOG.md`. Posible tabla
  de cache (si design la justifica), con migración idempotente.
- **Compatibilidad:** aditivo y opt-in puro. Una tool read-only nueva no cambia ninguna existente.
  Sin embedder el comportamiento degrada, no rompe. Sin DB (o sin la tabla de cache) el recall sigue
  funcionando al vuelo. Sin migración obligatoria si design no mete tabla.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| El embedder es opcional, pero el recall debe andar igual (invariante model-free) | Fallback keyword determinista; test con el embedder **apagado** que verifica que devuelve resultados y no error |
| Vectores que envejecen y mienten | Cache (si la hay) keyed por la huella `(mtime,size)` del índice F1 + id del embebedor; recomputo perezoso al cambiar el archivo. Nunca se sirve un vector de un body que ya cambió |
| Scope creep hacia el merge con `musubi_recall` o hacia el cross-link | Ambos **fuera** por escrito; F3-A es una tool standalone read-only que no toca el camino de recall de observaciones |
| Privacidad: embeber el body de una spec manda texto al embebedor | Pasa por el **mismo portero de privacidad** del embebedor que las observaciones; local-first; si la política lo veta, no se embebe y cae a keyword |
| Tool nueva mal clasificada (lost-update si quedara como write bajo RLock) | Es read-only pura (no muta nada); clasificarla read-only en `registry.go` y regenerar el golden |
| Costo/latencia de embeber muchas specs | Cache + cómputo perezoso; tope/paginado en la respuesta; el corpus de specs de un repo es chico frente al de observaciones |

## Criterio de éxito
- `musubi_spec_recall "política de reintentos"` devuelve, arriba, una spec cuyo body habla de
  «espera exponencial» **aunque no contenga las palabras del query** — algo que `spec_grep` no puede.
- Con el embebedor **deshabilitado**, la misma tool devuelve resultados sensatos por keyword,
  **sin error y sin exigir el embebedor** (invariante model-free verificado con test).
- Cada invariante nuevo tiene un test que se pone **en rojo bajo sabotaje** del código que dice
  cubrir (no una validación cualquiera — el invariante que el test declara).
- `GOWORK=off go vet ./... && go build ./... && go test -race ./...` en verde; golden de
  `tools/list` regenerado; `golangci-lint` (cruzado a linux/amd64) limpio antes de proponer push.
