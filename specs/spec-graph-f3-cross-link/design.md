---
artifact: design
schema_version: "1.0"
change: spec-graph-f3-cross-link
status: draft
---

# Diseño técnico — spec-graph F3-B: cross-link spec↔código y cobertura en el mapa

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Tool nueva `musubi_spec_coverage` (la 9ª `musubi_spec_*`), bidireccional | (a) extender `musubi_spec_get` con un campo coverage; (b) extender `musubi_impact` para aceptar `spec_id` | `spec_get` es FS-only y está clasificada `noScopedRead`; meterle lectura del grafo (tabla con `project_id`) cambia su clasificación de aislamiento. `impact` es code-graph-only; pedirle specs lo acopla al índice FS. Una tool nueva deja cada superficie existente con su clasificación intacta y nombra el cruce como su propio concepto. |
| D2 | Resolver `ruta#símbolo` → node_key por match `(path, name)` sobre `ListGraphNodesForFileCtx`, devolviendo TODOS los homónimos; blast radius = UNIÓN dedup de sus callers | (a) construir `path#kind:name` adivinando el `kind`; (b) elegir uno por orden de `kind` | El ancla NO lleva `kind` (diseño de F2); el node_key SÍ. Adivinar el kind falla para métodos y homónimos cross-lenguaje. Listar los nodos del archivo y filtrar `n.Name == anchor.Symbol` es exacto, reusa una query scopeada que ya existe, y ante >1 homónimo devuelve el conjunto — honesto y sin pick arbitrario. **(resuelve pregunta abierta 1)** |
| D3 | Degradación tri-estado ortogonal: `graph_not_indexed` (R4) · `no_resuelto` (R6) · `staleness` changed/missing (R5) | Colapsar "no indexado" y "no resuelto" en un solo "sin impacto" | Colapsarlos miente. `GraphStatsCtx` nodes==0 ⇒ grafo no indexado (aviso top-level). Grafo no vacío y el ancla no matchea ningún nodo ⇒ `no_resuelto` (archivo no indexado / lenguaje sin extractor / grafo viejo). La ranciedad viene de `StalenessOf` (disco vs baseline), independiente del grafo. Es el patrón de `toolImpact` (methods_codegraph.go:1664-1681, "cero callers" ≠ "no está en el grafo") llevado al nivel del ancla. |
| D4 | Dirección única por llamada: `spec_id` XOR (`symbol`\|`module`); cero o dos ⇒ `codeInvalidParams` (R3) | Permitir combinaciones y resolver por precedencia | Una llamada con dos mitades es ambigua; un resultado parcial enseña a no confiar en el error. |
| D5 | Inversa: match textual `(path, symbol)` contra las anclas parseadas; acepta `ruta#símbolo` y un node_key `path#kind:name` (se normaliza quitando `kind:`); `module` = prefijo de ruta | Aceptar sólo `ruta#símbolo` | Las anclas se escriben sin `kind`; normalizar el node_key a `(path, name)` deja que el usuario pegue lo que copió de `impact`/`code_graph`. **(resuelve pregunta abierta 3)** |
| D6 | Helper exportado `specgraph.Anchors(fm) []Anchor` | Reparsear las entradas en la capa MCP | `parseAnchorEntry` es no exportado y el split `=`/`#` (D2 de F2) no debe duplicarse fuera de specgraph (frontera D1). `StalenessOf` sólo devuelve las rancias; para resolver necesito TODAS las anclas parseadas. |
| D7 | **Overlay del mapa: cobertura ANCLA-based (un dir de código está cubierto si alguna ancla de alguna spec apunta a un archivo dentro de él), NO "hay un SPEC.md en el dir"** — ⚠️ **refina R13** | (a) R13 literal: dir cubierto si contiene un `SPEC.md` (modelo co-locado de thinkrail) | Musubi NO co-loca sus specs (viven en `specs/<change>/`, no en `internal/foo/`), así que "SPEC.md en el dir de código" daría ~todo descubierto y sería inútil para este repo. La cobertura ancla-based es significativa para los dos layouts, es model-free y barata (sólo lee los `Path` de las anclas). Universo = dirs de `walkSourceTree`. **Marcado para OK del dueño en el gate.** |
| D8 | `musubi_spec_coverage` va al barrido de aislamiento (`readSweepCases`), NO a `noScopedRead` | Tratarla como las otras 5 spec-tools (FS-only) | Es la 1ª spec-tool que LEE tablas scopeadas (`code_graph_nodes`/`_edges` vía `...Ctx`). El scope lo da `s.scopedCtx(ctx)`, pero el test de aislamiento tiene que PROBARLO. Es la clase de fuga que el repo arregló 3 veces (read_surface_class_test.go:17-25). `readOnly: true`, sin `lockSelf` (no hay red). |
| D9 | Overlay opt-in; salida por defecto de `musubi_map` byte-idéntica (R11) | Agregar el campo coverage siempre | `toolMap` hoy ignora el `raw`; se agrega el parseo de `coverage bool`. Sin el flag, la respuesta es exactamente la de hoy (test de regresión byte-idéntico). El schema gana la propiedad `coverage` ⇒ regen de los 3 goldens. |
| D10 | El blast radius se ORDENA por node_key asc (R8), a diferencia de `impact` que devuelve orden de descubrimiento | Heredar el orden de BFS de `GraphImpactCtx` | `callersOfCtx` (codegraph_query.go:46) no tiene `ORDER BY`, así que el orden de BFS no es determinista entre corridas de SQLite. Coverage ordena para ser testeable y reproducible. |

## Enfoque de implementación
Tres piezas, todas read-only y model-free:

1. **`internal/specgraph/anchors.go`** — exportar `Anchors(fm Frontmatter) []Anchor` (parsea todas las entradas de `FieldAnchors` con el `parseAnchorEntry` existente). Única adición al core; sin cambiar comportamiento previo.

2. **`internal/mcp/methods_spec_coverage.go`** (archivo nuevo) — el handler `toolSpecCoverage(ctx, raw)`:
   - **spec→código** (`spec_id`): `node, ok := s.specIndex.Graph().Nodes[spec_id]`; si no, `codeInvalidParams` (R7). Para cada `specgraph.Anchors(node.Frontmatter)`: calcular `staleness` con `StalenessOf` (por ref); resolver node_keys con `ListGraphNodesForFileCtx(scoped, anchor.Path)` filtrando `n.Name == anchor.Symbol` (símbolo "" ⇒ todos los del archivo); blast radius = unión dedup de `GraphImpactCtx(scoped, nk, maxDepth, 200)` por cada node_key, ordenada. Si `GraphStatsCtx` nodes==0 ⇒ `graph_not_indexed` + warning top-level, sin blast (R4). Si el grafo no está vacío pero no matchea ⇒ `no_resuelto` (R6).
   - **código→spec** (`symbol`|`module`): iterar `Graph().Nodes`; para cada spec, quedarse con las anclas cuyo `(path, symbol)` matchean el `symbol` normalizado, o cuyo `Path` cae bajo `module`. Devolver las specs (id/type/title/path + anclas que matchean), ordenadas por id.
   - Validación de dirección XOR al entrar (D4/R3).

3. **`internal/mcp/methods_codegraph.go`** — `toolMap` parsea `coverage bool`; si true, computa el overlay ancla-based (universo = `walkSourceTree` dirs; cubierto = dir que contiene el `Path` de alguna ancla de alguna spec) y lo agrega como campo `coverage`. Sin el flag, no cambia nada (R11/D9).

Registro: 9ª entrada en `specToolEntries()` (`readOnly: true`), golden regen, y clasificación en `readSweepCases` (D8).

## Contratos / interfaces
**Core (nuevo):**
```go
// internal/specgraph/anchors.go
func Anchors(fm Frontmatter) []Anchor   // todas las anclas parseadas (Ref, Path, Symbol, Baseline)
```

**MCP — `musubi_spec_coverage(spec_id, [max_depth])` (spec→código):**
```json
{ "direction": "spec_to_code", "spec_id": "...",
  "anchors": [ { "ref": "internal/foo/bar.go#T.M", "path": "internal/foo/bar.go", "symbol": "T.M",
                 "staleness": "none|changed|missing",
                 "resolution": "resolved|no_resuelto|graph_not_indexed",
                 "node_keys": ["internal/foo/bar.go#method:T.M"],
                 "blast_radius": ["..."], "blast_count": 3 } ],
  "blast_radius": ["...unión de todas las anclas, ordenada..."], "blast_count": 7,
  "warning": "grafo no indexado; corré musubi_codegraph_index" }
```
**MCP — `musubi_spec_coverage(symbol|module)` (código→spec):**
```json
{ "direction": "code_to_spec", "symbol": "internal/foo/bar.go#T.M",
  "specs": [ { "id": "...", "type": "...", "title": "...", "path": "...",
               "anchors": ["internal/foo/bar.go#T.M"] } ] }
```
**MCP — `musubi_map(coverage=true)`:** la salida de siempre + `"coverage": { "modules_total": K, "with_spec": J, "without_spec": ["dir", ...] }`. Sin el flag: byte-idéntica.

DTOs con tags json en minúscula (musubi-rules #6); listas con `make([]T,0,n)` nunca `nil` (go-hygiene #6).

## Trade-offs
- **Se gana:** cruce bidireccional reusando grafo e índice existentes, sin tabla ni migración; degradación honesta; overlay útil para el layout real de Musubi (D7).
- **Se cede:** D7 se aparta de thinkrail (co-locado) — se asume porque el co-locado es inútil acá. El blast radius ordenado (D10) pierde la señal "más cercano primero" de `impact`, a cambio de determinismo testeable. La resolución por `(path,name)` depende de que el grafo esté fresco; si está viejo, `no_resuelto` lo dice en vez de mentir.

## Plan de pruebas
Un test por invariante, cada uno verificado **en rojo bajo sabotaje del invariante que declara** (musubi-rules #8); el sabotaje se mecaniza con `// arnes:` (no sube el techo del censo). `GOWORK=off go vet/build/test -count=1 ./...` local; `-race` es gate de CI (no corre en Windows sin cgo).

| Req | Test | Sabotaje (ataca el invariante declarado) |
|-----|------|-------------------------------------------|
| R1 | spec→código devuelve node_key resuelto + callers | resolver ignora el grafo ⇒ blast vacío ⇒ rojo |
| R3 | `spec_id`+`module` juntos ⇒ error | quitar el XOR ⇒ resultado parcial ⇒ rojo |
| R4 | grafo vacío ⇒ warning, sin blast, sin error | quitar la guarda nodes==0 ⇒ error o vacío mudo ⇒ rojo |
| R5 | ancla `changed` ⇒ marcada | omitir `StalenessOf` ⇒ sin marca ⇒ rojo |
| R6 | ancla a archivo no indexado (grafo no vacío) ⇒ `no_resuelto` | colapsar `no_resuelto` en `graph_not_indexed` ⇒ rojo |
| R8/D10 | blast ordenado por node_key (fixture ≥2 callers) | quitar el sort ⇒ orden de BFS/SQLite ⇒ rojo |
| R9/D8 | fuga cross-tenant (víctima spec+ancla+grafo en "web", coverage como "crm") | ctx no scopeado ⇒ marcador se filtra ⇒ rojo |
| R11/D9 | `map()` sin flag == bytes de hoy | emitir coverage siempre ⇒ bytes difieren ⇒ rojo |
| R13/D7 | spec en `specs/` con ancla a `internal/foo` ⇒ `internal/foo` cubierto | usar "SPEC.md en el dir" ⇒ `internal/foo` descubierto ⇒ rojo |
| R14 | `TestToolsListGolden` con la 9ª tool | — (golden) |
