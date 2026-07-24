---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f3-weld-memoria
status: draft
---

# Especificación — Soldar el grafo de código a la memoria (Track 20 · F3)

## Requisitos
- **R1** — DEBE existir una tool `musubi_code_context` (read-only) que reciba `symbol` (node_key
  `path#kind:name`) y devuelva `{symbol, found, node?, callees?, callers?, explained_by}`.
- **R2** — `explained_by` DEBE derivarse buscando en las observaciones (FTS) el **nombre** del
  símbolo y su **path** (y basename), devolviendo los `topic_key` (o `id`) deduplicados, acotados.
- **R3** — La tool DEBE ser **scopeada**: `explained_by` sólo incluye observaciones visibles al
  proyecto de la credencial (reusa `SearchObservationsFTS`, ya scopeada). Un tenant NO ve las
  decisiones de otro.
- **R4** — Si el símbolo no existe en el grafo del proyecto (`found=false`), la tool DEBE igual
  computar `explained_by` a partir del nombre/path derivados del arg (no rompe).
- **R5** — El weld NO DEBE persistirse como arista: se deriva al consultar (respeta el invariante
  de F1 "aristas sólo derivadas"). NO se agrega ninguna columna/tabla.
- **R6** — Registrar la tool DEBE actualizar el golden de `tools/list` y los contadores (40→41);
  la tool DEBE quedar clasificada en el barrido de aislamiento (lee observaciones scopeadas).
- **R7** — Model-free, Go puro, `go build`/`go test` verdes sin cgo.

## Escenarios

### Escenario: contexto de un símbolo con su decisión
- **Given** un símbolo `x.go#func:Cobrar` indexado y una observación `arq/cobros` que menciona "Cobrar"
- **When** `musubi_code_context {symbol: "x.go#func:Cobrar"}`
- **Then** la respuesta incluye el nodo + callees/callers y `explained_by` contiene `arq/cobros`

### Escenario: aislamiento
- **Given** una observación de otro proyecto que menciona el archivo del símbolo
- **When** un lector de otro tenant consulta `code_context`
- **Then** `explained_by` NO incluye esa observación (sí la ve un admin federado)

### Escenario: símbolo sin nodo
- **Given** un node_key que no está en el grafo del proyecto
- **When** `code_context`
- **Then** `found=false` y `explained_by` se computa igual desde el nombre/path del arg (sin error)

## Fuera de alcance
- Arista persistida `EXPLICADO_POR` (violaría "aristas sólo derivadas" de F1).
- Hook `PreToolUse` (F2-B). TS/Py (F4). Sync central + viz CRM.

## Preguntas abiertas
- [ ] ¿`explained_by` devuelve solo topic_key, o {id, topic_key}? (design: topic_key deduplicado,
      como `detect_changes.relatedMemory`; el agente expande con recall/memory_expand)
