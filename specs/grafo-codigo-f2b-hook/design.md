---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f2b-hook
status: draft
---

# Diseño técnico — El hook que responde antes de leer (Track 20 · F2-B)

## Decisiones
| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| D1 | Gate por **env var** `MUSUBI_CODEGRAPH_HOOK` (default OFF) | Config file; siempre-on-si-indexado | Cero plumbing en `precheck` (que hoy no carga config); opt-in explícito y no invasivo. Doble seguro: aun ON, inerte sin índice. |
| D2 | 3ª superficie en `precheck` (no un binario/hook nuevo) | Hook nuevo en settings del usuario | El hook `PreToolUse`→`musubi precheck` YA existe e inyecta gists/telemetría; sumar la superficie es no-invasivo (no toca la config del usuario). |
| D3 | Reusar las lecturas de F2-A (ctx=Background, federado) | Nuevas queries | `ListGraphNodesForFileCtx`/`GraphOutEdgesCtx`/`GraphInEdgesCtx` ya existen; el `codeStore` del hook se extiende con esas 3. |
| D4 | Salida compacta: nombres (no node_keys), ≤10 símbolos, ≤5 refs | Volcado completo | Es la palanca de tokens: tiene que costar mucho menos que leer el archivo. |

## Implementación
`cmd/musubi/precheck.go`: interfaz `codeStore` +3 métodos de grafo; `codegraphHookEnabled()` (env);
`codeGraphMessage(store,key)` (imports + símbolos con callees/callers); helpers `graphRefNames`,
`symNameFromKey`, `joinCapped`, `noneIfEmpty`. Gate en `precheckOutput` tras la telemetría, con
`LedgerAdd(..., "precheck_codegraph", ...)`. El `*DbEngine` real ya satisface la interfaz extendida.

## Trade-offs
- Gana: navegación sin leer, automática, opt-in, medible.
- Cede: N queries por archivo (acotado); el detalle fino sigue en las tools de F2-A.

## Plan de pruebas
- `precheck_test.go`: fakeCodeStore +grafo; test ON (inyecta estructura + ledger) y test OFF (no
  inyecta). Los tests existentes (sin env var) quedan intactos.
