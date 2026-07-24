---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f3-weld-memoria
status: draft
---

# Diseño técnico — Soldar el grafo de código a la memoria (Track 20 · F3)

## Decisiones de arquitectura
| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| D1 | **Weld DERIVADO al consultar** (FTS por nombre+path), no una arista persistida | Arista `EXPLICADO_POR` en code_graph_edges | Una arista semántica la tendría que ESCRIBIR el agente → viola "aristas sólo derivadas" de F1 y se pudre. Derivar = cero rot, cero tokens LLM, siempre fresco. |
| D2 | **Tool nueva `musubi_code_context`** (no ampliar `code_graph`) | Agregar `explained_by` a code_graph | Mantiene `code_graph` puramente estructural y `code_context` como el puente código↔memoria — espejo exacto de `recall_facts` (grafo) vs `entity_context` (grafo+prosa). |
| D3 | **`explained_by` = topic_keys deduplicados** vía `SearchObservationsFTS` (ya scopeada) | Devolver contenido completo | Compacto (la palanca de tokens); el agente expande on-demand. Reusa el patrón de `detect_changes.relatedMemory`. |

## Enfoque de implementación
- `internal/mcp/methods_codegraph.go`:
  - `symbolNameFromKey(key)` → `(path, name)` parseando `path#kind:name`.
  - `explainedBy(ctx, path, name, limit)` → FTS por `name`, `path`, `base(path)`; dedup topic_key/id.
  - `toolCodeContext`: resuelve el nodo (scoped), arma callees/callers (CALLS), y agrega
    `explained_by`. Si el nodo trae Path, usa ese path para el FTS.
- `registry.go`: `toolEntry` de `musubi_code_context` (`readOnly: true`).
- Tests: golden (41), contadores (40→41), clasificación (read_concurrency), barrido de aislamiento
  (read_surface_class con marker = topic_key de la obs de web, que aparece en `explained_by` solo
  para el admin).

## Contratos
```go
func symbolNameFromKey(key string) (path, name string)
func (s *McpServer) explainedBy(ctx context.Context, path, name string, limit int) []string
```
Salida: `{symbol, found, node?, callees?, callers?, explained_by: []topic_key}`.

## Trade-offs
- Gana: el diferencial del Track (código + porqué) en su forma honesta (derivada), sin nuevo esquema.
- Cede: el weld depende de que la memoria mencione el símbolo/path (FTS); no hay enlace explícito
  curado. Suficiente y sin rot; un enlace curado necesitaría escritura del agente (rechazado).

## Plan de pruebas
- mcp: `code_context` de un símbolo con una observación que lo menciona → `explained_by` la incluye;
  símbolo sin nodo → found=false + explained_by igual; aislamiento por el barrido de contrato.
