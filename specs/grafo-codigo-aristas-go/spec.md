---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-aristas-go
status: draft
---

# Especificación — Grafo de código: aristas derivadas en Go (Track 20 · F1)

## Requisitos

### Emisor de aristas (`internal/codeintel`)
- **R1** — El emisor DEBE recibir `(path, content)` y devolver aristas derivadas
  **exclusivamente del `content` provisto** (nunca de datos persistidos), cada una con
  `{From, To, Kind, Confidence, Provenance}`.
- **R2** — Para archivos `.go` el emisor DEBE emitir aristas `IMPORTS` a partir de los
  import specs del AST, con `Confidence=1.0` y `Provenance=EXTRACTED`.
- **R3** — Para `.go` el emisor DEBE emitir aristas `CONTAINS` archivo→símbolo (y el
  agrupamiento por paquete), exactas (`Confidence=1.0`), reusando `ExtractSymbols`.
- **R4** — Para `.go` el emisor DEBERÍA emitir aristas `CALLS` derivadas del AST; la
  **estrategia de resolución** (exacta vía `go/types` donde cargue, o sintáctica acotada con
  `Confidence<1.0`) se decide en **design**. Ante ambigüedad DEBE **omitir** la arista, nunca
  inventar un destino.
- **R5** — Si el parseo falla, el `content` no compila (edición en curso) o la extensión no
  está soportada, el emisor DEBE devolver aristas vacías (o las parcialmente resueltas por
  parseo tolerante) **sin error y sin pánico** (degradación a granularidad de archivo).
- **R6** — En F1 el emisor NO DEBE emitir aristas para `.ts/.tsx/.js/.jsx/.py` ni otras
  extensiones (esos lenguajes siguen **solo-símbolos**; sus aristas son F4). DEBE degradar a
  vacío sin error.
- **R7** — La **identidad de nodo** DEBE ser estable y re-derivable: función pura de
  `(project_id, path, símbolo, kind)`. Dos ejecuciones sobre el mismo estado DEBEN producir el
  mismo id. Los métodos DEBEN distinguirse de funciones homónimas (p. ej. incluyendo el
  receiver o el kind en el id).

### Modelo y store de grafo (`internal/memory`)
- **R8** — El store DEBE persistir **nodos** `{project_id, path, symbol, kind, start_line,
  end_line, fingerprint}` y **aristas** `{project_id, from_id, to_id, kind, confidence,
  provenance, src_path, src_fingerprint}` en tablas nuevas (`code_graph_nodes`,
  `code_graph_edges`), creadas por DDL bootstrap como `code_memory`.
- **R9** — Todo nodo y arista DEBE estar **scopeado por `project_id`**; el UPSERT DEBE ser por
  clave estable + `project_id`. Dos proyectos con el mismo `path`/símbolo **NO DEBEN pisarse**
  (mismo patrón que `code_memory` `ON CONFLICT(path, project_id)`).
- **R10** — `kind` de arista DEBE ser uno de `{IMPORTS, CONTAINS, CALLS}`; `confidence` DEBE
  estar en `[0,1]`; en F1 `provenance` DEBE ser `EXTRACTED` (derivado del código). NO DEBE
  existir arista `INFERRED` ni emitida por el agente.
- **R11** — Cada nodo y arista DEBE registrar el `fingerprint` del **archivo origen** del que
  se derivó.
- **R12** — Un nodo/arista cuyo `src_fingerprint` almacenado difiera del fingerprint **actual**
  del archivo DEBE poder reportarse como **STALE**; NO DEBE devolverse como verdad en silencio.
- **R13** — El refresco DEBE ser **incremental**: dado un archivo cambiado (por fingerprint /
  `detect_changes`), solo se re-derivan y reemplazan los nodos/aristas de **ese** archivo; las
  filas de archivos no afectados quedan intactas (mismo fingerprint).
- **R14** — El poblado/refresco DEBE derivar del **contenido actual** del archivo
  ("derivar, no guardar-y-desfasar"). NO DEBE existir API pública ni tool que **inserte una
  arista directamente** provista por el llamador/agente.
- **R15** — Toda operación DEBE ser **model-free**, **Go puro**, **sin cgo**; `go build` y
  `go test` DEBEN quedar verdes.

### Preparado para federación (sync desactivado)
- **R16** — El esquema DEBE llevar `project_id` en cada fila desde el día 1, y el store DEBE
  soportar **lecturas scopeadas** (scoped vs. federate, como `GetCodeMemoryCtx`). F1 NO DEBE
  cablear el sync real al cerebro central, pero NO DEBE requerir migración para habilitarlo.

## Escenarios
Formato Given/When/Then.

### Escenario: IMPORTS exacto
- **Given** `a.go` con `import "musubi/internal/memory"`
- **When** se derivan las aristas de `a.go`
- **Then** existe una arista `{From: a.go, To: musubi/internal/memory, Kind: IMPORTS,
  Confidence: 1.0, Provenance: EXTRACTED}`

### Escenario: CONTAINS exacto
- **Given** `foo.go` con `func Parse()` (L10–20) y `type Node struct{}` (L22–24)
- **When** se derivan las aristas de `foo.go`
- **Then** existen aristas `foo.go —CONTAINS→ Parse` y `foo.go —CONTAINS→ Node`, ambas con
  `Confidence=1.0`

### Escenario: CALLS derivado
- **Given** `p.go` donde `Parse()` invoca a `validate()` definida en el mismo paquete
- **When** se derivan las aristas de `p.go` (con la estrategia de resolución elegida en design)
- **Then** existe una arista `Parse —CALLS→ validate` con `Provenance=EXTRACTED` y la
  `Confidence` que corresponda a la estrategia; y NO se emite una arista a un destino inexistente

### Escenario: aislamiento por proyecto
- **Given** los proyectos `P1` y `P2`, ambos con `x.go` que contiene `Foo`
- **When** se persiste el grafo de ambos
- **Then** el nodo `Foo` de `P1` y el de `P2` **coexisten** y no se sobrescriben (scope por
  `project_id`); una lectura scopeada a `P1` no ve el de `P2`

### Escenario: staleness por fingerprint
- **Given** un nodo de `bar.go` persistido con `src_fingerprint = fp1`
- **When** `bar.go` cambia a `fp2` y se consulta el grafo **sin** refrescar
- **Then** el nodo se reporta **STALE** (no como verdad actual)

### Escenario: refresco incremental
- **Given** un proyecto con nodos/aristas derivados de `a.go` y `b.go`
- **When** solo `a.go` cambia y se corre el refresco
- **Then** las filas de `a.go` se re-derivan (nuevo fingerprint) y las de `b.go` quedan
  **intactas** (mismo fingerprint), correctas incluso tras **corrimiento de líneas** en `a.go`

### Escenario: archivo que no compila
- **Given** `wip.go` con un error de sintaxis (edición en curso)
- **When** se derivan sus aristas
- **Then** degrada (aristas vacías o las parcialmente tolerables) **sin pánico ni error**

### Escenario: lenguaje sin aristas en F1
- **Given** `app.ts` o `styles.css` modificados
- **When** se derivan sus aristas
- **Then** no se emite ninguna arista y no hay error (los símbolos de `.ts` siguen existiendo;
  sus aristas son F4)

### Escenario: no hay puerta trasera para aristas a mano (invariante)
- **Given** el conjunto de APIs públicas del store y de las tools MCP en F1
- **When** se busca una forma de insertar una arista provista por el llamador
- **Then** **no existe** ninguna (toda arista nace derivada del contenido)

## Fuera de alcance
- Tools públicas de consulta (`musubi_code_graph` / `impact` / `map`) y hook `PreToolUse` → **F2**.
- Arista `símbolo —[EXPLICADO_POR]→ decisión/gotcha` (weld a memoria) → **F3**.
- Aristas para TS/JS/Py y tree-sitter → **F4**.
- Sync real al cerebro central y visualización en la cabina CRM → fase posterior.
- Análisis de impacto **transitivo** (callers de callers) — requiere el recorrido de F2.

## Preguntas abiertas
- [ ] **Resolución de CALLS**: ¿`go/types` (exacto, pero necesita cargar el paquete/build y
      puede fallar en repos rotos) o sintáctica AST-only (barata, imprecisa)? Resolver en
      design; criterio: exactitud vs. robustez Go-pura en repos a medio editar. Posible: híbrido
      — `go/types` cuando carga (confidence 1.0), fallback sintáctico (confidence < 1.0).
- [ ] **Nodo destino de IMPORTS**: ¿modelar el paquete importado como nodo (kind `package`,
      marcando externos: stdlib/terceros) o guardar solo el import-path en la arista? (design)
- [ ] **Representación de métodos**: `Recv.Método` en el símbolo/id para no colisionar con
      funciones homónimas. (design)
- [ ] **CALLS cross-archivo dentro del mismo paquete**: requiere tabla de símbolos a nivel
      paquete; ¿derivamos por archivo o por paquete? (design)
- [ ] **Disparo del poblado en F1** (sin tool pública): ¿método interno `RefreshFile`/
      `EnsureGraph` usado por tests + enganchado al camino de `save_code`/`detect_changes`?
      (design; la superficie pública es F2)
