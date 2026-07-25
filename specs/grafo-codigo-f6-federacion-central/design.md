---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f6-federacion-central
status: draft
---

# Diseño — Federación del grafo de código (Track 20 · F6)

## D1 — Receptor central: full-replace scopeado por proyecto
Nueva primitiva `DbEngine.ReplaceProjectGraphFrom(originProjectID, nodes, edges)` (en `codegraph.go`):
en **una tx**, `DELETE FROM code_graph_nodes/edges WHERE project_id=origin` y re-inserta el set
empujado. **Rationale:** el push-on-index manda el grafo **entero**, así que el full-replace es la
semántica más simple que es correcta — espeja el local exacto, es **idempotente** (re-push = mismo
estado) y **aislado** (el DELETE va acotado al `project_id` del origen, nunca toca otro tenant).
**Alternativa descartada:** reusar `UpsertPackageGraphFrom` per-archivo — no borra los nodos
FANTASMA que quedaron de un push anterior (drift), y complica la limpieza de nodos de paquete.

## D2 — Tenancy: el origen sale del PRINCIPAL, no del payload
El tool central deriva el `project_id` destino con **`writeOriginFor(principalFrom(ctx), declared)`**
—la misma guarda que `save_observation`—: para un token `write=own` (p.ej. `davantis-musubi`)
**ignora** lo declarado y devuelve el `project_id` del principal; sólo un `write=any` puede declarar
otro. **Rationale:** cierra el invariante de aislamiento (R2/R3/E2) reusando la guarda ya probada,
sin inventar tenancy nueva. `ok=false` ⇒ `-32001` (no se pudo atribuir). El `declared` del payload
queda como cortesía para `write=any`; jamás decide el scope de un `write=own`.

## D3 — Tool central `musubi_codegraph_push` (WRITE)
En `registry.go` + `methods_codegraph.go` (`toolCodegraphPush`). Params: `nodes` (array), `edges`
(array), `project_id` (opcional, sólo lo usa `write=any`). **NO** se marca `readOnly` ⇒ nace WRITE:
`canCall` exige autoridad de escritura (un `reader`/cabina no puede empujar) y **no** entra al barrido
de aislamiento de lecturas (ese guard es sólo para `readOnly`). Llama `ReplaceProjectGraphFrom(origin,
nodes, edges)` y devuelve `{nodes, edges}` contados. Model-free: sólo persiste lo derivado.

## D4 — Cliente: `SyncClient.PushGraph` (espejo de Push)
En `syncclient.go`: un `tools/call` remoto de `musubi_codegraph_push` por HTTP JSON-RPC con el mismo
Bearer token, id JSON-RPC fijo (`"codegraph-push"`), y la misma clasificación transitorio/permanente
de `classifyResponse`. **Rationale:** reusa el transporte, el TLS-guard y el manejo de errores ya
existentes; cero dependencias nuevas.

## D5 — Enganche best-effort en `toolCodegraphIndex`
Al final del index (full o incremental), si `s.syncClient != nil` **y** `s.memory.TeamMode`, leer el
grafo local completo (`AllGraphNodesCtx`/`AllGraphEdgesCtx`, nuevas, exportan los `listAll*` ya
existentes) y llamar `s.syncClient.PushGraph(...)` en un helper `pushCodeGraphToCentral`. Es
**best-effort**: cualquier error se **loguea y se traga** (no rompe el index; R5/E3). **Gate por team
mode** — un proyecto federa su grafo bajo la misma condición que participa del cerebro compartido,
igual que el drain entrante. Sin syncClient / sin team mode ⇒ **no-op** total (R6/E4).

## D6 — Superficie nueva en el motor (interfaz `CodeGraphStore`)
`backend.go` suma a la interfaz: `ReplaceProjectGraphFrom(...)`, `AllGraphNodesCtx(ctx)`,
`AllGraphEdgesCtx(ctx)` — porque el dispatch opera sobre `s.engine` (interfaz), no el concreto.

## Decisiones diferidas (no en F1)
- **Batching (R8):** el push es **una sola request** con el grafo entero. Para los proyectos en
  alcance (Musubi ~3.5k nodos/~7.7k aristas ≈ 1–2 MB) entra holgado en un POST. Si un repo gigante
  excede el límite del body del central, el troceo es un follow-on (queda anotado como riesgo).
- **Delta en incremental:** F1 empuja el grafo completo aun tras un index incremental (simple y
  correcto). Empujar sólo el delta es una optimización posterior.

## Riesgos
| Riesgo | Mitigación |
|--------|------------|
| Grafo gigante > límite de body | Anotado; batching diferido; el timeout del http.Client acota el request |
| Golden de tools + contadores hardcodeados | Regenerar golden (`-update`); bumpear los counts (41→42) |
| Un `write=any` empujando con `project_id` ajeno | Es su privilegio por diseño (mantenimiento); `write=own` sigue blindado |
