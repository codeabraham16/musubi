---
artifact: tasks
schema_version: "1.0"
change: otel-export-journal
status: archived
---

# Tareas — Export OTel del run journal

## Núcleo (nuevo internal/memory/otel.go)
- [ ] T1 — Structs OTLP mínimos (otlpDoc/ResourceSpans/ScopeSpans/Scope/Resource/Span/Status/
  KV/Val) con tags json; kind=1, status codes 0/1/2. (R12–R13)
- [ ] T2 — Helpers: `otelTraceID(runID)` (hex sha256[:16]), `otelSpanID(runID, stepID)` (hex
  sha256[:8] con separador 0x1f; raíz = centinela `__run__`); `parseJournalTimeNano(s)` (UTC
  `2006-01-02 15:04:05` → UnixNano string, degradación sin panic). (R4–R7)
- [ ] T3 — `WorkflowTraceOTLP(runID) (string, error)`: cargar `WorkflowJournal`; error si vacío;
  fold lineal por seq (run_started→raíz+workflow_id; step_completed/skipped→span hijo con
  start=evento previo, end=propio, status, atributos; step_reopened ignorado; run_done→cierra
  raíz); armar otlpDoc con resource service.name=musubi; `json.MarshalIndent`. (R1–R11)

## Interfaz + handler
- [ ] T4 — `backend.go`: `WorkflowStore` += `WorkflowTraceOTLP(runID) (string, error)`.
- [ ] T5 — `methods.go toolWorkflow`: acción `otel` (requiere run_id) → devuelve el JSON como
  textResult. `registry.go`: agregar `otel` al enum/descripción de acciones (sin tools nuevas).
  Regenerar golden si cambia el snapshot. (R14–R15)

## Tests
- [ ] T6 — `otel_test.go`: trace bien formado (1 raíz + N step spans, traceId 32 hex, spanId 16
  hex, parentSpanId correcto); ids deterministas (2 exports iguales); step failed→ERROR; step
  skipped→atributo; run inexistente→error; JSON parseable con estructura OTLP. (todos los R)

## Cierre
- [ ] T7 — `go build/vet/test ./...` verdes; golden regenerado si cambió; smoke: exportar un
  run real y validar que el JSON parsea. (R15)
