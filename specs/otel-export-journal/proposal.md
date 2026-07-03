---
artifact: proposal
schema_version: "1.0"
change: otel-export-journal
status: archived
---

# Propuesta — Export OTel del run journal (traza derivada)

## Intención
El run journal (`run_events`, entregado en v0.59.0) ya registra cada transición de un run
de workflow como evento inmutable ordenado por `seq`. Pero esa historia sólo se puede leer
con `action=journal` en el formato interno de Musubi. Para que la orquestación de Musubi sea
**observable con herramientas estándar** (Jaeger, Grafana Tempo, cualquier collector OTel),
queremos exportar un run como una **traza OpenTelemetry**: el run es un *trace*, cada step es
un *span*. Esto es especialmente valioso de cara al server casero (Musubi como cerebro +
orquestador central): poder ver, en tooling estándar, qué corrió, en qué orden, cuánto tardó
y qué falló — sin acoplarse a Musubi.

Es un primer fruto directo del journal: cero motor nuevo, cero LLM, puramente **derivado** de
eventos que ya existen.

## Alcance
- **Incluye:**
  - Una función que **deriva** una traza OTLP (formato JSON de OpenTelemetry) de un run a
    partir de su journal: un span raíz para el run + un span hijo por step, con `trace_id`/
    `span_id` **deterministas** (self-generados por hash, sin SDK), timestamps, status
    (ok/error) y atributos (seq, event_type, result, workflow_id).
  - Exposición vía `musubi_workflow` con una acción nueva (`otel`/`trace`) que devuelve el
    JSON OTLP del run indicado, listo para POSTear a un collector o guardar a archivo.
  - IDs OTel válidos: `trace_id` 16 bytes (32 hex) derivado de `run_id`; `span_id` 8 bytes
    (16 hex) derivado de `(run_id, step_id)`. Deterministas → re-exportar da la misma traza.
- **No incluye (explícito):**
  - **Una tabla `spans` persistida.** Los spans se **derivan** del journal en el momento del
    export (principio "derivar, no guardar-y-desfasar", el mismo que salvó `detect_changes`):
    el journal es la fuente de verdad append-only, la traza es una vista. Cero migración,
    cero drift.
  - **Push automático a un collector / integración de red.** Musubi devuelve el JSON; el
    envío (curl/collector) lo hace quien consume. Local-first: sin clientes de red nuevos.
  - **Instrumentación de la memoria o de la pizarra** (`work_units`). Sólo el motor de
    workflows en esta iteración; los otros subsistemas son follow-up si se piden.
  - **Dependencia del SDK de OpenTelemetry.** Se emite el OTLP/JSON a mano (structs + encoding/
    json), fiel al formato, sin romper Go-puro ni agregar deps.

## Enfoque
Derivación pura: `WorkflowJournal(runID)` ya da los eventos ordenados. Se pliega esa lista en
una estructura OTLP: `run_started`/`run_done` acotan el span raíz; cada `step_completed`
produce un span hijo cuyo fin es su `created_at` y cuyo inicio es el `created_at` del evento
inmediatamente anterior (aproximación honesta de la ventana del step, ya que el journal no
registra "step_started"); `step_skipped` → span hijo marcado skipped; el status del run/step
sale del payload. Los ids se derivan por `sha256` truncado, así son estables y no requieren
estado ni azar (compatible con la restricción model-free/determinista). Todo el tiempo se
parsea de los `created_at` (UTC ISO) a unix-nano que pide OTLP.

## Impacto
- Áreas/archivos afectados:
  - Nuevo `internal/memory/otel.go` (derivación OTLP + helpers de id/tiempo; struct `RunEvent`
    ya existe).
  - `internal/memory/backend.go` (interfaz `WorkflowStore`: `WorkflowTraceOTLP`).
  - `internal/mcp/methods.go` + `registry.go` (acción `otel` en `musubi_workflow`; sin tools
    nuevas → conteo intacto).
  - Tests nuevos (`otel_test.go`): determinismo de ids, estructura del trace, status,
    aproximación de tiempos.
- Compatibilidad: **puramente aditivo**. No hay esquema nuevo, no toca el journal ni el
  snapshot; sólo agrega una vista de lectura y una acción de tool.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| El journal no tiene "step_started" → duración por step aproximada | Documentar la semántica (ventana = evento previo → completion); es honesta y suficiente para observar orden/fallo/duración relativa |
| Formato OTLP mal formado que un collector rechace | Fijar el shape a la spec OTLP/JSON (resourceSpans→scopeSpans→spans) con test que valida las claves obligatorias; ids con la longitud exacta (32/16 hex) |
| Colisión de ids por truncado de hash | sha256 truncado a 16/8 bytes es más que suficiente para unicidad por run; el trace_id encapsula el run, los span_id sólo compiten dentro del run |
| Scope creep hacia push/collector | Línea roja: Musubi sólo DERIVA y DEVUELVE el JSON; el transporte es del consumidor |

## Estrategia de rollback
Aditivo y sin estado nuevo. Rollback = revertir el PR; desaparece la acción `otel` y nada más
(no hay tabla ni migración que deshacer). Un binario viejo simplemente no ofrece la acción.

## Criterio de éxito
1. `action=otel` sobre un run real devuelve OTLP/JSON con un span raíz (el run) y un span por
   step, `trace_id` de 32 hex y `span_id` de 16 hex — cubierto por test.
2. Los ids son **deterministas**: dos exports del mismo run dan idénticos trace/span ids — test.
3. Un step `failed` mapea a status ERROR; `done` a OK; `skipped` marcado — test.
4. La traza es parseable como JSON y respeta la estructura OTLP (resourceSpans/scopeSpans/spans).
5. Todo model-free, Go puro, sin deps nuevas (sin SDK OTel); build + suite verdes; sin migración.
