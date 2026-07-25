---
artifact: tasks
schema_version: "1.0"
change: grafo-codigo-f6-federacion-central
status: draft
---

# Tareas — Federación del grafo de código (Track 20 · F6)

## Motor (internal/memory)
1. `codegraph.go`: `ReplaceProjectGraphFrom(originProjectID, nodes, edges)` — full-replace en una tx
   (DELETE nodes/edges WHERE project_id=origin → bulk insert). Reusa `cgBoolToInt`.
2. `codegraph_viz.go` (o `codegraph.go`): exportar `AllGraphNodesCtx(ctx)` / `AllGraphEdgesCtx(ctx)`
   como wrappers de los `listAllGraph*` ya existentes.
3. `backend.go`: sumar las 3 a la interfaz `CodeGraphStore`.

## MCP central (recepción)
4. `methods_codegraph.go`: `toolCodegraphPush(ctx, args)` — parse `nodes/edges/project_id`;
   `origin, ok := writeOriginFor(principalFrom(ctx), declared)`; `!ok ⇒ -32001`;
   `ReplaceProjectGraphFrom(origin, …)`; devolver `{nodes, edges}`.
5. `registry.go`: registrar `musubi_codegraph_push` (WRITE, sin `readOnly`; params nodes/edges/project_id).

## MCP cliente (envío)
6. `syncclient.go`: `PushGraph(nodes, edges, projectID)` — tools/call remoto de `musubi_codegraph_push`,
   espejo de `Push` (Bearer, id fijo, `classifyResponse`).
7. `methods_codegraph.go`: helper `pushCodeGraphToCentral(ctx)` best-effort + enganche al final de
   `toolCodegraphIndex`, gateado por `s.syncClient != nil && s.memory.TeamMode`.

## Contadores / golden
8. Regenerar `internal/mcp/testdata/toolslist.golden.json` (`-update`) y bumpear los contadores
   hardcodeados de tools (41→42) donde aparezcan (http/server/dispatch tests).

## Tests
9. `codegraph_f6_test.go` (memory): full-replace idempotente + aislamiento por project_id
   (proyecto A intacto cuando B reemplaza).
10. `methods_codegraph_test.go` (mcp): push como `write=own` atribuye al principal (ignora declared);
    `write=none`/reader rechazado; best-effort (index no rompe si el push falla); no-op sin sync.
