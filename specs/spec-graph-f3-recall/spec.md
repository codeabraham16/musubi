---
artifact: spec
schema_version: "1.0"
change: spec-graph-f3-recall
status: draft
---

# Especificación — spec-graph F3-A: recall semántico sobre specs

## Requisitos
Cada requisito es verificable y atómico. `musubi_spec_recall` es la tool nueva.

### Contrato de la tool
- **R1** — `musubi_spec_recall` DEBE aceptar `query` (string, obligatorio) y, opcionales, `limit`
  (int, default 20), `offset` (int, default 0) y `type` (string, filtro por tipo de spec). Un
  `query` vacío o sólo-espacios DEBE devolver un error, no una lista vacía silenciosa.
- **R2** — La respuesta DEBE ser una lista (nunca `null`: slice con `make`) de specs rankeadas, cada
  una con `id`, `type`, `title`, `path`, `score` (float) y `snippet` (fragmento del body). El orden
  DEBE ser de `score` descendente.
- **R3** — La tool DEBE clasificarse **read-only pura** en `registry.go` (los tres ejes:
  autorización, lockClass, roClass) y NO DEBE modificar ningún `SPEC.md` ni estado observable fuera
  de un cache interno derivable del FS. Agregarla DEBE regenerar el golden de `tools/list`.

### Ranking: semántico con fallback model-free
- **R4** — Cuando hay un embebedor configurado y habilitado, los resultados DEBEN ordenarse por
  similitud semántica (coseno) entre el `query` y el body de cada spec.
- **R5** — Cuando NO hay embebedor (ausente, deshabilitado, o su llamada falla), la tool DEBE
  degradar a un ranking por **keyword determinista** y DEBE devolver resultados **sin error**. La
  tool NUNCA DEBE exigir el embebedor para funcionar. *(Invariante central model-free.)*
- **R6** — Para un mismo corpus, `query` y estado del embebedor, el orden DEBE ser **determinista**:
  los empates de `score` se rompen por una clave estable (`id`, y `path` como desempate secundario),
  para que el ranking sea reproducible y testeable.
- **R7** — Embeber el body de una spec DEBE pasar por el **mismo portero de privacidad** del
  embebedor que filtra qué se embebe de las observaciones. Si la política veta el contenido, ese
  body NO se embebe y esa spec se rankea por la señal de keyword (R5), sin error.

### Fuente de datos: FS-nativa y fresca
- **R8** — El corpus DEBE derivarse del FS con la misma detección **is-a-spec de F1** (frontmatter
  con `id`+`type`): un archivo sin `id`+`type` —p. ej. el diario SDD `specs/<cambio>/*.md`, cuyo
  frontmatter es `artifact`+`schema_version`— NO DEBE aparecer en los resultados.
- **R9** — El contenido rankeado DEBE revalidarse por la huella `(mtime,size)` del índice de F1: si
  el body de una spec cambió en disco, su ranking DEBE reflejar el contenido nuevo y NO DEBE servirse
  un vector/índice calculado sobre el contenido viejo.
- **R10** — Cuando se pasa `type`, la tool DEBE devolver **sólo** specs de ese tipo. Cuando se pasan
  `limit`/`offset`, DEBE paginar de forma estable y señalar si quedaron más resultados
  (`truncated`/`next_offset`, espejando `GrepResult`).

### Higiene de proyecto
- **R11** — El subsistema DEBE ser model-free y sin cgo, y DEBE pasar
  `GOWORK=off go vet ./... && go build ./... && go test -race ./...` en verde.
- **R12** — Cada invariante nuevo DEBE tener un test que se ponga **en rojo bajo sabotaje** del
  código que dice cubrir —el invariante que el test declara, no una validación cualquiera—.
- **R13** — El cambio DEBE ser aditivo y opt-in: sin migración obligatoria salvo que design meta una
  tabla de cache, y en ese caso la migración DEBE ser idempotente (`IF NOT EXISTS`/`ADD COLUMN`
  guardado), con el recall funcionando aunque la tabla no exista todavía.

## Escenarios
Formato Given/When/Then (un comportamiento observable por escenario).

### Escenario: el significado gana al texto literal
- **Given** un `SPEC.md` cuyo body describe «espera exponencial entre reintentos» y NO contiene la
  palabra «backoff», y un embebedor configurado
- **When** se llama `musubi_spec_recall("política de backoff")`
- **Then** esa spec aparece en el top de los resultados — algo que `spec_grep("backoff")` no logra

### Escenario: fallback sin embebedor (invariante model-free)
- **Given** NO hay embebedor configurado
- **When** se llama `musubi_spec_recall("reintentos")`
- **Then** la tool devuelve specs rankeadas por keyword, sin error y sin exigir el embebedor

### Escenario: frescura FS-nativa
- **Given** una spec ya rankeada una vez, cuyo body luego cambia en disco (nuevo `mtime`/`size`)
- **When** se vuelve a llamar `musubi_spec_recall` con un query que pega al contenido nuevo
- **Then** el ranking refleja el body nuevo; no se sirve un resultado calculado sobre el viejo

### Escenario: el diario SDD no es un spec
- **Given** un `specs/spec-graph-f3-recall/proposal.md` (frontmatter `artifact`+`schema_version`, sin
  `id`+`type`)
- **When** se llama `musubi_spec_recall` con cualquier query
- **Then** ese archivo NO aparece en los resultados (no pasa la detección is-a-spec)

### Escenario: filtro por type
- **Given** specs de varios `type` (p. ej. `module-design` y `task-spec`)
- **When** se llama `musubi_spec_recall(query, type="module-design")`
- **Then** sólo se devuelven specs de tipo `module-design`

### Escenario: read-only
- **Given** cualquier query
- **When** se llama `musubi_spec_recall`
- **Then** ningún `SPEC.md` cambia en disco; la tool se clasifica `readOnly` con `lock=lockSelf` (design D6: el embed de red corre fuera del candado del despacho, no bajo el lock de lectura compartido)

## Fuera de alcance
- Cross-link con `musubi_code_graph`/`musubi_impact` y overlay de cobertura en `musubi_map` → F3-B.
- Fundir las specs en `musubi_recall` (recall de observaciones): descartado; superficie separada.
- Federación de gists de specs al central (F4); arbitraje spec↔observación (F5).
- Edición/escritura de specs (ya cubierta por create/update/delete de F1).

## Preguntas abiertas
- [ ] **Q1 — dónde viven los vectores:** ¿cache en memoria por proceso, o tabla DB análoga a
  `spec_anchors` (decisión 6 del roadmap) keyed por `(path, mtime, size, embedder_id)`? Tradeoff:
  persistencia/arranque-frío vs. acoplamiento a la DB. (design)
- [ ] **Q2 — granularidad:** ¿un vector por spec, o chunking del body? Un `SPEC.md` suele ser corto;
  decidir si el chunking aporta. (design)
- [ ] **Q3 — señal de keyword del fallback:** ¿reusar el FTS/keyword del recall de observaciones
  (`musubi_search_keyword`) o una señal propia sobre el corpus FS? (design)
- [ ] **Q4 — snippet:** ¿primera línea no vacía del body, o el fragmento de mayor relevancia al
  query? (design)
- [ ] **Q5 — nombre final:** `musubi_spec_recall` (consistente con `musubi_recall`) vs.
  `musubi_spec_search`. (design)
