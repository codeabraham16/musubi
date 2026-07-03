---
artifact: spec
schema_version: "1.0"
change: otel-export-journal
status: archived
---

# Especificación — Export OTel del run journal

## Requisitos

### Derivación de la traza
- **R1** — DEBE existir `WorkflowTraceOTLP(runID) (string, error)` que devuelva la traza del
  run como **OTLP/JSON** (formato `traces` de OpenTelemetry), derivada de `WorkflowJournal`.
- **R2** — Si el run no existe (journal vacío), DEBE devolver un error claro (o un documento
  OTLP sin spans, decidir en design); NO DEBE entrar en pánico.
- **R3** — La traza DEBE contener un **span raíz** que representa el run (name = workflow_id;
  sin `parentSpanId`) y un **span hijo por step** ejecutado (name = step_id; `parentSpanId` =
  el id del span raíz).

### IDs deterministas
- **R4** — El `traceId` DEBE ser de **16 bytes** (32 caracteres hex), derivado
  deterministamente de `run_id` (p. ej. `sha256(run_id)[:16]`).
- **R5** — Cada `spanId` DEBE ser de **8 bytes** (16 caracteres hex), derivado
  deterministamente: el raíz de `(run_id, "\_\_root\_\_")`, cada step de `(run_id, step_id)`.
- **R6** — Los ids DEBEN ser **estables**: dos exports del mismo run DEBEN producir idénticos
  `traceId` y `spanId`.

### Tiempos y status
- **R7** — Los timestamps OTLP (`startTimeUnixNano`, `endTimeUnixNano`) DEBEN derivarse de los
  `created_at` (UTC, `YYYY-MM-DD HH:MM:SS`) parseados a **nanosegundos unix**.
- **R8** — El span raíz DEBE ir de `run_started.created_at` a `run_done.created_at`; si no hay
  `run_done` (run en curso), DEBE terminar en el `created_at` del último evento.
- **R9** — Un span de step DEBE terminar en el `created_at` de su `step_completed` (o
  `step_skipped`) y empezar en el `created_at` del evento **inmediatamente anterior** en el
  journal (aproximación de la ventana del step). `startTimeUnixNano <= endTimeUnixNano`.
- **R10** — El `status.code` OTLP DEBE mapear: step/run con fallo (`status:failed`) →
  `STATUS_CODE_ERROR`; éxito (`done`) → `STATUS_CODE_OK`. Un step saltado (`step_skipped`)
  DEBE marcarse (atributo `musubi.skipped=true`, status OK o UNSET).

### Atributos
- **R11** — Cada span de step DEBE incluir atributos derivados del evento: `musubi.seq`,
  `musubi.event_type`, y —si el payload lo trae— `musubi.result` y `musubi.step_status`. El
  span raíz DEBE incluir `musubi.workflow_id` y `musubi.run_id`.

### Formato OTLP
- **R12** — El documento DEBE respetar la estructura OTLP/JSON: un objeto raíz con
  `resourceSpans[]`, cada uno con `scopeSpans[]`, cada uno con `spans[]`. Cada span DEBE tener
  `traceId`, `spanId`, `name`, `kind`, `startTimeUnixNano`, `endTimeUnixNano`, `status`.
- **R13** — El resultado DEBE ser JSON válido y parseable. Los `*UnixNano` DEBEN serializarse
  como **string** (convención OTLP/JSON para enteros de 64 bits).

### Tool / integración
- **R14** — `musubi_workflow` DEBE exponer una acción `otel` (run_id) que devuelva el OTLP/JSON.
  El conteo de tools MCP NO DEBE cambiar (acción nueva, no tool nueva).
- **R15** — NO DEBE haber migración de esquema (todo derivado). Build + suite verdes; model-free
  (sin LLM), Go puro, sin dependencias nuevas (sin SDK OTel).

## Escenarios

### Escenario: traza bien formada de un run completo
- **Given** un run `a → b` arrancado y completado (journal: run_started, step_completed(a),
  step_completed(b), run_done)
- **When** se llama `WorkflowTraceOTLP(runID)`
- **Then** el JSON tiene 1 span raíz (run) + 2 spans de step (a, b); `traceId` de 32 hex; cada
  `spanId` de 16 hex; los steps tienen `parentSpanId` = spanId del raíz

### Escenario: ids deterministas
- **Given** el mismo run
- **When** se exporta dos veces
- **Then** ambos documentos tienen idénticos `traceId` y `spanId`

### Escenario: step fallido → status ERROR
- **Given** un run donde el step `x` se completó con `status:failed`
- **When** se exporta
- **Then** el span de `x` tiene `status.code = STATUS_CODE_ERROR`

### Escenario: step saltado
- **Given** un run con un step saltado (`step_skipped` en el journal)
- **When** se exporta
- **Then** existe un span para ese step con el atributo `musubi.skipped=true`

### Escenario: run inexistente
- **Given** un run_id sin eventos
- **When** se exporta
- **Then** devuelve un error claro (o un documento OTLP sin spans) sin pánico

## Fuera de alcance
- Tabla `spans` persistida (se deriva).
- Push automático a un collector / transporte de red.
- Instrumentación de la memoria o de `work_units`.
- Dependencia del SDK de OpenTelemetry.

## Preguntas abiertas
- [ ] ¿Run inexistente devuelve error o documento vacío? (design; probable: error claro, como
      las otras acciones que exigen run_id existente)
- [ ] ¿`span.kind` = INTERNAL para todos, o el raíz SERVER y los steps INTERNAL? (design;
      probable: INTERNAL para todos, es lo más neutro y correcto para trabajo no-RPC)
- [ ] ¿Se incluye un `resource` con `service.name=musubi`? (design; probable: sí, ayuda a que
      el collector agrupe las trazas)
