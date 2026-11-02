---
artifact: spec
schema_version: "1.0"
change: spec-graph-f3-cross-link
status: archived
---

# Especificación — spec-graph F3-B: cross-link spec↔código y cobertura en el mapa

## Requisitos
Cada requisito es verificable y atómico. Vocabulario RFC 2119.

- **R1** — `musubi_spec_coverage` DEBE aceptar un `spec_id` y devolver, para esa spec, cada uno de
  sus anchors con: la ref del anchor, el símbolo resuelto al node_key del grafo (si resuelve) y el
  blast radius (callers transitivos) de ese símbolo según el grafo de código.
- **R2** — Como alternativa a `spec_id`, `musubi_spec_coverage` DEBE aceptar un `symbol`
  (`ruta#símbolo`) o un `module` (prefijo de ruta) y devolver las specs cuyos anchors cubren ese
  símbolo o caen bajo ese módulo (cobertura inversa).
- **R3** — Recibir `spec_id` Y (`symbol`|`module`) a la vez, o ninguno de los tres, DEBE ser un
  error de uso claro: exactamente una dirección por llamada.
- **R4** — Si el grafo de código NO está indexado, la dirección spec→código DEBE degradar:
  devolver los anchors resueltos con un aviso explícito ("grafo no indexado; corré
  `musubi_codegraph_index`") y SIN blast radius, nunca un error.
- **R5** — Para cada anchor, la respuesta DEBE propagar la ranciedad surface-only de F2
  (`specgraph.StalenessOf`): un anchor `changed`/`missing` se MARCA con su razón; NO se omite ni se
  reemplaza por un nodo inventado.
- **R6** — Un anchor cuyo archivo/símbolo existe en disco pero NO resuelve a un nodo del grafo
  (p. ej. el grafo quedó desactualizado respecto del archivo) DEBE reportarse como `no_resuelto` —
  estado DISTINTO de `missing` (rancio) y de "grafo no indexado" — y SIN error.
- **R7** — Un `spec_id` inexistente (dirección forward) DEBE devolver un error claro; NUNCA un
  resultado vacío silencioso que se lea como "sin cobertura". En la dirección inversa
  (`symbol`/`module`) el match es textual puro (D5) y no hay un input bien-definidamente "mal
  formado": un `symbol`/`module` que no cubre ninguna spec NO es un error, sino `specs: []` con la
  query ECHOED en la respuesta — es el eco, no un cuerpo vacío, lo que la distingue de un error. La
  única compuerta de FORMA de la llamada es el XOR de dirección de R3 (cero o dos direcciones es error).
- **R8** — La salida DEBE ser determinista y ordenada: specs por `id` asc; anchors por ref asc;
  callers por node_key asc.
- **R9** — `musubi_spec_coverage` DEBE ser read-only pura: no muta ningún `SPEC.md`, no toca el
  grafo de código ni `memory.db`; DEBE clasificarse read-only en los tres ejes de `registry.go`.
- **R10** — El corpus de specs DEBE derivarse del índice FS de F1 con la misma regla is-a-spec
  (frontmatter `id`+`type`), revalidado por `(mtime,size)`; el diario SDD (`artifact`+
  `schema_version`, sin `id`+`type`) NO DEBE aparecer.
- **R11** — `musubi_map` DEBE aceptar un flag opt-in `coverage` que agregue, por módulo, si está
  cubierto por alguna spec (ver R13). SIN el flag, la salida de `musubi_map` DEBE ser byte-idéntica
  a la actual.
- **R12** — El overlay de cobertura DEBE ser surface-only: informa qué módulos tienen o no spec; NO
  crea, mueve ni marca specs, ni muta ningún estado persistido.
- **R13** — La granularidad de "cubierto" DEBE estar definida y ser consistente: un módulo
  (directorio de código) está cubierto si alguna ancla de alguna spec apunta a un archivo dentro de
  él (cobertura **ancla-based**; refina el modelo co-locado "un `SPEC.md` en el dir", inútil acá
  porque Musubi centraliza sus specs en `specs/<change>/`). Un módulo descubierto DEBE listarse, no
  omitirse. El universo de módulos es el de `walkSourceTree` (los directorios de código del indexador).
- **R14** — Agregar la tool DEBE regenerar el golden de `tools/list`; el censo de tools y la skill
  `spec-graph` DEBEN reflejar la tool nueva; CHANGELOG `[Unreleased]` con bump minor.

## Escenarios
Formato Given/When/Then (un escenario por comportamiento observable).

### Escenario: blast radius de una spec
- **Given** una spec `S` con un anchor a `ruta#F`, y `F` tiene callers transitivos en el grafo indexado
- **When** `musubi_spec_coverage(spec_id=S)`
- **Then** devuelve el anchor `ruta#F` resuelto a su node_key y sus callers transitivos, ordenados y deterministas

### Escenario: cobertura inversa por módulo
- **Given** dos specs `S1` y `S2` con anchors bajo `internal/foo/`
- **When** `musubi_spec_coverage(module="internal/foo")`
- **Then** devuelve `S1` y `S2` ordenadas por `id`, cada una con los anchors que caen bajo el módulo

### Escenario: grafo no indexado degrada sin error
- **Given** el grafo de código sin indexar
- **When** `musubi_spec_coverage(spec_id=S)`
- **Then** devuelve los anchors resueltos con el aviso "grafo no indexado…", sin blast radius y sin error

### Escenario: anchor rancio se marca, no rompe
- **Given** una spec con un anchor a `ruta#F` cuyo símbolo `F` fue borrado del archivo, con el grafo
  reindexado en sincronía con el disco (el nodo de `F` también se fue)
- **When** `musubi_spec_coverage(spec_id=S)`
- **Then** el anchor aparece marcado `missing` y `no_resuelto`, sin blast radius, y la llamada no falla
- **Nota (ortogonalidad, D3):** la ranciedad (disco, `StalenessOf`) y la resolución (grafo,
  `ListGraphNodesForFileCtx`) son independientes y NO se colapsan. Si el grafo NO se reindexó y el nodo
  de `F` persiste, el MISMO anchor sale `missing` (disco) Y `resolved` con su blast (grafo): las dos
  verdades se informan juntas, no se mezclan. "Marcar, no omitir, no inventar nodo" (R5) se cumple en
  ambos regímenes; el "sin blast radius" vale cuando el grafo está en sincronía, NO es una propiedad
  del estado `missing`. El test `TestSpecCoverageAnclaMissingResuelveEnGrafoStale` fija el régimen stale.

### Escenario: overlay opt-in no cambia el default
- **Given** un proyecto con specs en algunos módulos y no en otros
- **When** `musubi_map()` sin flag
- **Then** la salida es byte-idéntica a la de antes de F3-B
- **And when** `musubi_map(coverage=true)` **then** además marca, por módulo de código, si alguna ancla de spec apunta dentro (cubierto) o no (descubierto)

### Escenario: dirección ambigua es error de uso
- **Given** una llamada con `spec_id` y `module` a la vez
- **When** `musubi_spec_coverage(spec_id=S, module="x")`
- **Then** error de uso claro ("exactamente una dirección por llamada"), sin resultado parcial

## Fuera de alcance
- Reindexar el grafo automáticamente: el usuario corre `musubi_codegraph_index`. Sin índice, R4.
- Resolver símbolos en lenguajes sin extractor del grafo: si no hay nodo, es `no_resuelto` (R6), no un error.
- Federación (F4) y arbitraje spec↔observación (F5).
- Mutar specs, crear anchors o modificar el grafo de código.

## Preguntas abiertas
- [x] Resolución `ruta#símbolo` → node_key: RESUELTA en `design` (D2) — se devuelven TODOS los
      homónimos del archivo cuyo nombre matchea, cada uno con su node_key y su blast radius; la
      unión dedup es el blast radius a nivel spec. Sin pick arbitrario.
- [x] Nombre y forma del flag de `musubi_map`: RESUELTA en `design` (D9) — `coverage` (bool, default
      false); agrega un objeto `coverage` con `modules_total`/`with_spec`/`without_spec`.
- [x] La inversa, ¿acepta node_key `path#kind:name`?: RESUELTA en `design` (D5) — sí; se normaliza
      a `(path, name)`, además de `ruta#símbolo` y de `module` (prefijo de ruta).
