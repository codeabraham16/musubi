---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-f3-weld-memoria
status: draft
---

# Propuesta — Soldar el grafo de código a la memoria (Track 20 · F3)

## Intención
El corazón del Track 20: cerrar el híbrido **estructura ⊕ "por qué"**. Hoy el grafo de código
(F1/F2) responde *qué llama a qué*, y la memoria responde *por qué se hizo así / qué gotcha
tiene* — pero **desconectados**. F3 los suelda: consultar un símbolo devuelve su estructura **y**
las decisiones/gotchas que lo explican. Es lo único que Graphify/CodeGraph estructuralmente no
pueden (no tienen memoria capturada del porqué). Es el análogo de `musubi_entity_context` (que
une hechos + prosa) para el mundo del código.

## Alcance
- **Incluye:** una tool MCP nueva **`musubi_code_context`** (read-only, scopeada): dado un símbolo
  (node_key), devuelve el nodo + callees/callers (como `code_graph`) **más `explained_by`**: las
  observaciones (decisiones/gotchas) que mencionan ese símbolo o su archivo, vía FTS. Es el puente
  código→memoria.
- **No incluye (explícito):**
  - Una **arista persistida** `EXPLICADO_POR` en `code_graph_edges`: sería una arista **escrita por
    el agente** (semántica, no derivable del AST), lo que **viola la línea roja de F1** ("aristas
    sólo derivadas"). Por eso el weld se **deriva al consultar** (FTS por símbolo/path), no se guarda.
  - El hook `PreToolUse` que inyecta esto solo (F2-B). TS/Py (F4). Sync central + viz CRM.

## Enfoque
**Derivar el weld, no guardarlo.** Reusa `SearchObservationsFTS` (ya scopeada por proyecto, Track
17) buscando el nombre del símbolo + su path — la misma técnica que `detect_changes.relatedMemory`,
pero orientada a "explicar" un símbolo consultado. Cero aristas nuevas, cero tokens de LLM, cero
rot (siempre refleja la memoria actual). Model-free.

## Impacto
- `internal/mcp/methods_codegraph.go`: `toolCodeContext` + helper `explainedBy` + parseo del
  node_key. `registry.go`: +1 tool (read-only) → 41. Golden + contadores (40→41) + clasificación
  read/write + barrido de aislamiento (la tool lee observaciones scopeadas).
- Compatibilidad: aditivo. No toca el esquema ni el grafo persistido.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Falsos positivos del FTS (un símbolo con nombre común matchea de más) | Buscar por nombre **y** path; devolver topic_keys (el agente juzga/expande); cap de resultados |
| Fuga cross-tenant (lee observaciones) | `SearchObservationsFTS` ya es scopeada; se agrega al barrido de aislamiento con marker |
| Scope creep hacia una arista persistida | Línea roja: el weld se DERIVA al consultar, no se persiste (respeta el invariante de F1) |

## Criterio de éxito
1. `musubi_code_context {symbol}` devuelve estructura + `explained_by` con las observaciones que
   mencionan el símbolo/archivo — cubierto por test.
2. Scopeada: un tenant no ve las decisiones de otro (barrido de aislamiento verde).
3. Model-free, Go puro, build + tests verdes sin cgo; golden consistente (41 tools).

## Rollback
Aditivo (una tool read-only). Quitar su `toolEntry` revierte sin tocar datos.
