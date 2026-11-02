---
artifact: proposal
schema_version: "1.0"
change: spec-graph-f3-cross-link
status: draft
---

# Propuesta — spec-graph F3-B: cross-link spec↔código y cobertura en el mapa

## Intención
F2 dejó a cada spec capaz de anclarse a símbolos de código (`anchors: ["ruta#símbolo"]`),
pero nadie cruza ese anclaje con el grafo de código ni se ve la cobertura de specs en el
mapa del proyecto. F3-B conecta las dos memorias: desde una spec, qué código la implementa
y qué se rompe si lo tocás; desde un símbolo, qué specs lo gobiernan; y en `musubi_map`,
qué módulos tienen spec y cuáles no. Es el delta 3 del roadmap spec-graph.

## Alcance
- **Incluye:**
  - Tool nueva read-only `musubi_spec_coverage`, que cruza en las dos direcciones:
    - *spec → código:* dado un `id` de spec, resuelve sus anchors a símbolos y devuelve el
      blast radius reusando la maquinaria de `musubi_impact` (cierre transitivo de callers).
    - *código → spec:* dado un símbolo (`ruta#símbolo`) o un módulo (prefijo de ruta),
      devuelve las specs cuyos anchors lo cubren (cobertura inversa).
  - Overlay de cobertura en `musubi_map`, **opt-in por flag** (p. ej. `coverage=true`): marca
    qué módulos tienen al menos un `SPEC.md` y cuáles no. Surface-only.
  - Golden de `tools/list` regenerado; la tool clasificada **read-only pura** en los 3 ejes de
    `registry.go`; CHANGELOG `[Unreleased]`; bump **minor**; skill `spec-graph` actualizada.
- **No incluye:**
  - F4 (federación de gists de specs al central) y F5 (arbitraje spec↔observación).
  - Mutar specs o el grafo: el cruce es de **sólo lectura**; no crea anchors ni status.
  - Un grafo de código nuevo: F3-B **reusa** `musubi_code_graph`/`musubi_impact`. Si el grafo
    no está indexado, **degrada con aviso claro, no falla**.
  - Ranciedad nueva: la de los anchors ya la dio F2 (surface-only); F3-B la **reporta**, no la
    reimplementa.

## Enfoque
Reusar, no reconstruir. Los anchors de una spec ya los parsea `internal/specgraph` (F2); el
cierre transitivo de callers ya lo da el grafo de código (Track 20, `musubi_impact`). F3-B es
el puente: resuelve anchors → símbolos, consulta el grafo, y arma el cruce bidireccional. La
cobertura inversa (símbolo → specs) se deriva del índice de specs de F1 (FS-nativo, revalidado
por `(mtime,size)`), filtrando por anchors. El overlay del mapa lee lo mismo. Todo model-free y
determinista; **sin tabla nueva ni migración** (el cruce se computa on-demand desde el índice FS
y el grafo ya existente).

## Impacto
- Áreas/archivos afectados: `internal/specgraph/` (API de cruce sobre los anchors ya parseados),
  `internal/mcp/` (handler de la tool nueva + flag de overlay en el handler de `musubi_map` +
  `registry.go` + golden), CHANGELOG, skill `spec-graph`.
- Compatibilidad: **aditivo y opt-in**. La tool es nueva; el overlay del mapa está detrás de un
  flag y la salida por defecto de `musubi_map` **no cambia**. Sin migración de DB.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| El grafo de código no está indexado → el cruce spec→impacto queda vacío | Degradar con gracia: devolver los anchors resueltos + un aviso "grafo no indexado, corré `musubi_codegraph_index`", nunca error |
| Un anchor apunta a un símbolo que ya no existe (rancio) | Ya lo cubre F2 (ranciedad surface-only): el cruce marca el anchor como rancio en vez de inventar un nodo |
| El overlay infla la salida de `musubi_map` y rompe consumidores | Opt-in por flag; la salida por defecto queda **byte-idéntica** (test de regresión) |
| Scope creep hacia F4/F5 | Declarado fuera de alcance; el cruce es read-only y no toca federación ni arbitraje |

## Criterio de éxito
- `musubi_spec_coverage` devuelve, para una spec con anchors, sus símbolos y su blast radius; y
  para un símbolo/módulo, las specs que lo cubren — determinista y ordenado.
- `musubi_map` con el flag marca módulos con/sin spec; **sin** el flag, la salida no cambia.
- Degrada **sin error** cuando el grafo no está indexado o un anchor quedó rancio.
- Un test por invariante, cada uno verificado **en rojo bajo sabotaje** del invariante que declara
  (cruce bidireccional, degradación sin grafo, ranciedad propagada, overlay opt-in, determinismo,
  read-only).
- `GOWORK=off go vet ./... && go build ./... && go test -count=1 ./...` verde; golden de
  `tools/list` regenerado; `-race` como gate de CI.

## Rollback
Aditivo y opt-in: revertir el PR quita la tool `musubi_spec_coverage` y el flag del overlay sin
tocar nada preexistente. Sin migración que deshacer (no hay tabla nueva). El golden vuelve a su
estado anterior con el revert.
