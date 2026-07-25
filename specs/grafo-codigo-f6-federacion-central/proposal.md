---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-f6-federacion-central
status: draft
---

# Propuesta — Federar el grafo de código al cerebro central (Track 20 · F6)

## Intención
Cerrar la **topología federada** que el Track 20 declaró desde el día 1: que el grafo de código
de cada proyecto viaje al **cerebro central** para que la cabina (CRM) vea el código de **todos**
los proyectos —panorama general + drill-down por proyecto— sin que el central tenga el fuente.
Hoy el grafo se deriva y persiste **local** (scopeado por `project_id`) pero **nunca sale** del
daemon: el outbox durable es solo de observaciones. Falta únicamente el **transporte**.

## Alcance
- **Incluye:**
  - **Lado local:** tras `codegraph_index` (full/incremental), empujar el grafo del proyecto
    (nodos + aristas) al central por una **RPC nueva** (`codegraph_push`), **best-effort** (no
    rompe el index si el central no está) y **gateada** por la misma config de sync
    (`sync.enabled` / `central_url`).
  - **Lado central:** un **método MCP inbound** que recibe el grafo y lo persiste con
    `UpsertPackageGraphFrom(originProjectID, …)` (delete-by-source + reinsert **ya existe**),
    scopeado por el **PRINCIPAL del token** (tenancy Track 17) — nunca por un `project_id`
    provisto por el cliente.
  - **Batching** por si el grafo es grande (Musubi mismo: ~3454 nodos / ~7771 aristas).
- **No incluye:**
  - La **vista en la cabina CRM** (repo `crm-musubi`, separado) — es la **F7**, follow-on.
  - **Durabilidad offline-first** del push: se eligió **push-on-index** (re-indexar re-empuja);
    generalizar el outbox queda **descartado** para esta fase.
  - CALLS **cross-archivo** TS/Py (refinamiento aparte).

## Enfoque
Reusar lo que el central **ya tiene** (tablas `code_graph_nodes/edges`, `UpsertPackageGraphFrom`,
`CodeGraphViz(ctx)` scopeado por credencial) y sumar **solo el transporte**: el daemon local, al
terminar de indexar, empuja su grafo al central por una RPC autenticada con el **token writer del
proyecto** (el principal fija `origin project_id`). Idempotente (`node_key` estable + `src_fingerprint`),
**model-free** (solo mueve el grafo derivado, sin LLM). Mismo espíritu que el sync de observaciones,
pero sin el aparato de outbox: la reproducibilidad del grafo (deriva del AST) hace innecesaria la
durabilidad — si el central estaba caído, el próximo `codegraph_index` lo re-empuja.

## Impacto
- `internal/mcp` (local): tras el index, serializar y empujar el grafo al central (cliente RPC /
  reuso del canal de sync). Best-effort + gate por config.
- `internal/mcp` (central): nuevo método inbound `codegraph_push` → `UpsertPackageGraphFrom`
  scopeado por el principal.
- `internal/memory`: si hace falta, un lector de "grafo completo del proyecto" para serializar
  (ya existen `listAllGraphNodes/Edges` scopeados).
- **Aditivo:** sin `central_url`/`sync.enabled`, comportamiento **idéntico** al actual (el grafo
  sigue 100% local, cero push).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Tamaño del grafo en el cable | Batching por lotes; en `incremental` empujar **solo lo re-derivado** (reusar el fingerprint) + poda; compresión si hiciera falta |
| Cruce de tenants (un proyecto pisa el grafo de otro) | `project_id` **siempre** del principal del token, jamás del payload; `UpsertPackageGraphFrom(originProjectID)` ya scopea el delete-by-source |
| Central caído al indexar | **best-effort**: no rompe el index; re-indexar re-empuja; log claro (contabilizar en sync_status) |
| Idempotencia / doble escritura | `node_key` estable + `src_fingerprint`; upsert idempotente (mismo patrón que el poblado local) |

## Criterio de éxito
1. Tras `codegraph_index` en un proyecto con sync activo, el central tiene el grafo de **ese**
   proyecto scopeado por su `project_id`.
2. **Aislamiento:** un proyecto no ve ni pisa el grafo de otro (verificado con dos principales).
3. Sin `sync`/`central_url`, comportamiento **idéntico** al actual (grafo local, cero push).
4. Model-free, sin cgo, suite verde.

## Rollback
Aditivo y **gateado**. Apagar el push (config de sync) revierte sin tocar datos; el grafo local
queda intacto. En el central, el método inbound es aditivo: si nadie empuja, no se ejerce.
