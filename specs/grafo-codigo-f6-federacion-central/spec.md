---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f6-federacion-central
status: draft
---

# Especificación — Federación del grafo de código (Track 20 · F6)

Vocabulario RFC 2119: **DEBE** / **DEBERÍA** / **PUEDE**.

## Requisitos

- **R1 — Push tras index.** Cuando el sync saliente está activo (`sync.enabled` **y** `central_url`
  no vacío), al terminar `musubi_codegraph_index` (full o incremental) el daemon **DEBE** empujar el
  grafo del proyecto (nodos + aristas + los `src_path` involucrados) al central por un `tools/call`
  remoto de `musubi_codegraph_push`, autenticado con el mismo Bearer token del sync.
- **R2 — Tenancy por el principal.** El central **DEBE** persistir el grafo recibido scopeado por el
  `project_id` **derivado del principal del token** (tenancy T17-19), y **NO DEBE** confiar en ningún
  `project_id` provisto en el payload para decidir el scope de escritura.
- **R3 — Aislamiento.** Un push de un proyecto A **NO DEBE** crear, pisar ni borrar nodos/aristas de
  un proyecto B. `UpsertPackageGraphFrom(originProjectID,…)` (delete-by-source + reinsert) **DEBE**
  acotar su borrado al `origin_project_id` del principal.
- **R4 — Idempotencia.** Empujar dos veces el mismo grafo sin cambios **DEBE** dejar el central en el
  mismo estado (upsert por `node_key` + `src_fingerprint`; sin filas duplicadas).
- **R5 — Best-effort.** Si el central está caído/inalcanzable o rechaza el push, `codegraph_index`
  **DEBE** completar igual y reportar el resultado local; el fallo del push **NO DEBE** romper el
  index. El error **DEBERÍA** quedar visible (log / `sync_status`).
- **R6 — No-op sin sync.** Sin `sync.enabled`/`central_url`, el comportamiento **DEBE** ser idéntico
  al actual: el grafo queda 100% local, cero tráfico de red.
- **R7 — Model-free.** El push y su recepción **DEBEN** ser model-free (mueven y persisten el grafo
  ya derivado; ningún LLM en el camino).
- **R8 — Batching.** Si el grafo excede un umbral de tamaño, el push **DEBERÍA** trocearse en lotes
  para no exceder límites de request; el central **DEBE** aplicar cada lote de forma consistente.

## Escenarios

### E1 — Push feliz tras index (Given/When/Then)
- **Given** un proyecto con `sync.enabled=true` + `central_url` válido + token writer,
- **When** corre `musubi_codegraph_index`,
- **Then** el central, consultado con la credencial de ESE proyecto, devuelve por `CodeGraphViz`
  los nodos/aristas del proyecto (mismos `node_key`), y `codegraph_index` reporta su conteo local.

### E2 — Aislamiento entre tenants (el crítico)
- **Given** el central con el grafo del proyecto A ya federado,
- **When** el proyecto B empuja su grafo (token de B),
- **Then** el grafo de A permanece intacto (mismos nodos/aristas), y `CodeGraphViz` con la credencial
  de A **no** ve nodos de B ni viceversa (aislamiento por `project_id` del principal).

### E3 — Best-effort con central caído
- **Given** `sync.enabled=true` pero el central inalcanzable,
- **When** corre `musubi_codegraph_index`,
- **Then** el index completa y reporta el grafo local; el push falla en silencio-observable (no
  aborta), y un `codegraph_index` posterior con el central arriba re-empuja (consistencia eventual).

### E4 — No-op sin sync
- **Given** `sync.enabled=false` (o `central_url` vacío),
- **When** corre `musubi_codegraph_index`,
- **Then** no hay ningún request de red y el resultado es idéntico al comportamiento actual.

### E5 — Idempotencia
- **Given** un grafo ya federado,
- **When** se re-empuja sin cambios en el fuente,
- **Then** el central queda igual (sin duplicados; `UpsertPackageGraphFrom` reemplaza por `src_path`).

## Fuera de alcance (recordatorio)
- Vista en la cabina CRM (repo `crm-musubi`) = **F7**.
- Durabilidad offline-first del push (se eligió push-on-index).
- CALLS cross-archivo TS/Py.
