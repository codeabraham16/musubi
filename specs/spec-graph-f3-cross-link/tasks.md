---
artifact: tasks
schema_version: "1.0"
change: spec-graph-f3-cross-link
status: draft
---

# Plan de tareas — spec-graph F3-B: cross-link spec↔código y cobertura en el mapa

Cada tarea es un work-unit acotado (≤~400 líneas). Orden: core → handler → registro → mapa →
aislamiento → pruebas → cierre. Cada invariante lleva su prueba verificada **en rojo bajo sabotaje
del invariante que declara** (musubi-rules #8), mecanizada con `// arnes:`.

## Implementación

- [ ] **T1 — core: exportar `specgraph.Anchors(fm)`** (`internal/specgraph/anchors.go`)
  Nueva función exportada que parsea TODAS las anclas (no sólo las rancias como `StalenessOf`),
  reusando `List(fm, FieldAnchors)` + `parseAnchorEntry` + el filtro `a.Path == ""`. Sin cambiar
  comportamiento previo. (D6)

- [ ] **T2 — handler `toolSpecCoverage`** (`internal/mcp/methods_spec_coverage.go`, archivo nuevo)
  - DTOs con tags json camelCase (musubi-rules #6); listas `make([]T,0,n)` (go-hygiene #6).
  - Validación de dirección XOR al entrar: exactamente uno de `spec_id`/`symbol`/`module`; cero o
    dos ⇒ `codeInvalidParams` (R3/D4).
  - **spec→código** (`specCoverageForward`): `node, ok := s.specIndex.Graph().Nodes[spec_id]`; si no,
    `codeInvalidParams` (R7). `GraphStatsCtx(scoped)` nodes==0 ⇒ `graph_not_indexed` + warning
    top-level, sin blast (R4). `StalenessOf` por ref (R5). Por cada ancla:
    `ListGraphNodesForFileCtx(scoped, a.Path)` filtrando `n.Name == a.Symbol` (símbolo "" ⇒ todos);
    0 matches con grafo no vacío ⇒ `no_resuelto` (R6); blast = unión dedup de
    `GraphImpactCtx(scoped, nk, maxDepth, 200)` ordenada por node_key (D10).
  - **código→spec** (`specCoverageInverse`): normalizar `symbol` con `symbolNameFromKey` (acepta
    `ruta#símbolo` y node_key `path#kind:name`, D5); iterar `Graph().Nodes`, quedarse con las anclas
    cuyo `(path,symbol)` matchean o cuyo `Path` cae bajo `module` (prefijo). Specs ordenadas por id,
    anclas por ref (R8). Echo de la query ⇒ `specs: []` es "sin cobertura", no error (R7).
  - Helpers locales: `sortedStringSet`, `underModule`, `coverageOverlay` (para T4).

- [ ] **T3 — registro de la 9ª tool** (`internal/mcp/methods_spec.go`)
  9ª entrada en `specToolEntries()` con `readOnly: true`, handler `s.toolSpecCoverage`, InputSchema
  con `spec_id`/`symbol`/`module`/`max_depth` (sin `Required`: el XOR se valida en el handler).
  Actualizar el header del archivo ("son las 8 tools" → 9; la 9ª lee el grafo scopeado). (D1)

- [ ] **T4 — flag `coverage` en `musubi_map`** (`internal/mcp/methods_codegraph.go` + `registry.go`)
  `toolMap` parsea `coverage bool` del `raw` (hoy lo ignora); si true agrega el campo `coverage`
  con el overlay ancla-based (`walkSourceTree` dirs vs `path.Dir` de cada ancla). SIN el flag, salida
  byte-idéntica (R11/D9). En `registry.go`, agregar la Property `coverage` al schema de `musubi_map`.

- [ ] **T5 — clasificación de aislamiento** (`internal/mcp/read_surface_class_test.go`)
  `musubi_spec_coverage` al bloque MEDIO ("covered by dedicated isolation test"), NO a `readSweepCases`
  ni a `noScopedRead`: es la 1ª spec-tool que lee tablas scopeadas (D8). `readOnly: true`, sin `lockSelf`.

## Pruebas

- [ ] **T6 — un test por invariante** (`internal/mcp/methods_spec_coverage_test.go`, archivo nuevo)
  R1 (node_key resuelto + callers) · R3 (XOR ⇒ error) · R4 (grafo vacío ⇒ warning sin blast sin error)
  · R5 (ancla changed ⇒ marcada) · R6 (archivo no indexado, grafo no vacío ⇒ `no_resuelto`) · R8/D10
  (blast ordenado, fixture ≥2 callers) · R11/D9 (`map()` sin flag == bytes de hoy) · R13/D7 (spec en
  `specs/` con ancla a `internal/foo` ⇒ `internal/foo` cubierto). Cada uno **rojo bajo sabotaje** del
  invariante que declara, con `// arnes:` donde aplique.

- [ ] **T7 — test de aislamiento dedicado** (`internal/mcp/read_surface_class_test.go` o archivo par)
  Víctima "web": spec-on-disk anclada a `shared/auth.go#VictimCallee` + grafo `VictimCaller→VictimCallee`.
  Atacante "crm" NO ve "VictimCaller" en el blast; admin federado SÍ. ctx no scopeado ⇒ marcador se
  filtra ⇒ rojo (R9/D8).

- [ ] **T8 — regenerar los 3 goldens**
  `GOWORK=off go test ./internal/mcp -run TestToolsListGolden -update` (toolslist.golden.json,
  toolslist-completo.golden.json, contrato-tools.golden.json). La 9ª tool queda al final del golden
  (specToolEntries se appendea LAST). (R14)

- [ ] **T9 — suite verde**
  `GOWORK=off go vet ./... && go build ./... && go test -count=1 ./...` en verde. `-race` es gate de
  CI (no corre en Windows sin cgo).

## Docs-cierre

- [ ] **T10 — CHANGELOG + bump + skill**
  CHANGELOG `[Unreleased]` con la tool nueva y el flag; bump **minor**. Actualizar la skill
  `spec-graph` (censo de tools + la 9ª). (R14)

## Forecast de review

- Nivel probable: **media** (archivo nuevo + 2 handlers tocados + golden + clasificación de
  aislamiento). `musubi_detect_changes` fija el panel real en verify.
- Lentes que importan: **contrato de la spec** (los 9 requisitos ↔ sus tests), **seguridad**
  (aislamiento cross-tenant R9 — la fuga que el repo arregló 3 veces), **correctitud** (tri-estado
  R4/R5/R6 sin colapsar), **determinismo** (R8/D10 el sort del blast).
- Radio ciego posible: la resolución ancla→node_key depende de `ListGraphNodesForFileCtx`; si el
  grafo del fixture no se puebla igual que en prod, un test puede pasar por el camino equivocado.
  Mitigación: el test R1 verifica el node_key EXACTO, no sólo "no vacío".
