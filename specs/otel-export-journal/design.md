---
artifact: design
schema_version: "1.0"
change: otel-export-journal
status: archived
---

# Diseño — Export OTel del run journal

## Decisión 1 — Derivar la traza del journal, sin tabla `spans`
`WorkflowTraceOTLP(runID)` llama a `WorkflowJournal(runID)` y **pliega** los eventos en una
traza OTLP en memoria. No se persiste ninguna tabla de spans.
**Rationale:** el journal ya es la fuente de verdad append-only e inmutable; una tabla de
spans sería un cache que puede desincronizarse (el mismo anti-patrón que `detect_changes`
evitó). Derivar en el export = cero migración, cero drift, y la traza siempre refleja el
journal exacto. El costo (recomputar por export) es trivial: los runs son acotados.
**Descartado:** la "tabla spans" que sugería la investigación (más esquema, más escrituras,
posible drift).

## Decisión 2 — IDs deterministas por `sha256` truncado
- `traceID` = hex(`sha256(run_id)[:16]`) → 32 chars.
- `spanID(stepID)` = hex(`sha256(run_id + "\x1f" + stepID)[:8]`) → 16 chars. El span raíz usa
  `stepID = "\x1f__run__"` (un centinela que ningún step_id real puede colisionar, porque los
  ids de step no contienen `\x1f`).
**Rationale:** deterministas (re-export = mismos ids), sin estado ni azar (compatible con la
prohibición de `Math.random`/no-determinismo), y con la longitud exacta que OTel exige. El
separador `\x1f` (unit separator) evita colisiones tipo `(run="a", step="bc")` vs
`(run="ab", step="c")`.
**Descartado:** ids aleatorios (rompen el determinismo y la idempotencia del export); contador
secuencial (no da la longitud/entropía de OTel).

## Decisión 3 — Structs OTLP mínimos + JSON a mano (sin SDK)
Se define el subconjunto de OTLP/JSON necesario como structs Go con tags `json`:
```go
type otlpDoc struct{ ResourceSpans []otlpResourceSpans `json:"resourceSpans"` }
type otlpResourceSpans struct {
    Resource   otlpResource      `json:"resource"`
    ScopeSpans []otlpScopeSpans  `json:"scopeSpans"`
}
type otlpResource struct{ Attributes []otlpKV `json:"attributes"` }
type otlpScopeSpans struct {
    Scope otlpScope  `json:"scope"`
    Spans []otlpSpan `json:"spans"`
}
type otlpScope struct{ Name string `json:"name"` }
type otlpSpan struct {
    TraceID           string     `json:"traceId"`
    SpanID            string     `json:"spanId"`
    ParentSpanID      string     `json:"parentSpanId,omitempty"`
    Name              string     `json:"name"`
    Kind              int        `json:"kind"`            // 1 = INTERNAL
    StartTimeUnixNano string     `json:"startTimeUnixNano"`
    EndTimeUnixNano   string     `json:"endTimeUnixNano"`
    Attributes        []otlpKV   `json:"attributes,omitempty"`
    Status            otlpStatus `json:"status"`
}
type otlpStatus struct{ Code int `json:"code"` }        // 0 UNSET, 1 OK, 2 ERROR
type otlpKV struct{ Key string `json:"key"`; Value otlpVal `json:"value"` }
type otlpVal struct {
    StringValue *string `json:"stringValue,omitempty"`
    IntValue    *string `json:"intValue,omitempty"`     // int64 como string
    BoolValue   *bool   `json:"boolValue,omitempty"`
}
```
`*UnixNano` e `intValue` van como **string** (convención OTLP/JSON para int64). `kind = 1`
(INTERNAL) para todos: es trabajo no-RPC, INTERNAL es lo correcto y neutro. `resource` lleva
`service.name = "musubi"` para que el collector agrupe.
**Rationale:** emitir el shape exacto a mano es ~un archivo; agregar el SDK de OTel rompería
Go-puro y sumaría un árbol de deps enorme para lo que es serializar JSON.
**Descartado:** SDK OpenTelemetry-Go (deps + peso, innecesario para exportar).

## Decisión 4 — Fold del journal a spans
Un solo paso sobre los eventos ordenados por `seq`:
- `run_started` → marca el inicio del span raíz (su `created_at`) y toma `workflow_id` del payload.
- `step_completed` / `step_skipped` → un span hijo: `end` = su `created_at`; `start` = el
  `created_at` del **evento anterior** (índice i-1; si es el primero, = start del raíz);
  status del payload (`failed`→ERROR, else OK); atributos del evento.
- `step_reopened` → se ignora para spans (es una anotación de loop; opcionalmente atributo en
  el step). No genera span propio para no duplicar.
- `run_done` → marca el `end` del span raíz.
Si no hay `run_done`, el `end` del raíz = `created_at` del último evento. `start<=end` se
garantiza tomando `max(start, rootStart)` y, si el previo fuese posterior (imposible por seq),
clamp a `end`.
**Rationale:** un fold lineal O(n) fiel al orden del journal; la aproximación de start por
"evento previo" es la mejor ventana disponible sin un evento step_started (fuera de alcance).

## Decisión 5 — Run inexistente → error claro
Si `WorkflowJournal` devuelve 0 eventos, `WorkflowTraceOTLP` devuelve
`error("run %q no tiene eventos / no existe")`. Coherente con las otras acciones del tool que
exigen un run_id válido.
**Rationale:** un documento OTLP vacío se vería como éxito engañoso; el error es más honesto.

## Decisión 6 — Tiempo
`parseJournalTime(s)` parsea `"2006-01-02 15:04:05"` (UTC) → `time.Time` → `UnixNano()` como
string. Si un `created_at` viniese vacío/mal (no debería), se degrada al del evento previo o a
0, sin panic.
**Rationale:** granularidad de segundo es suficiente para observar orden y duración relativa;
OTLP pide nanos, se convierte sin inventar precisión.

## Firmas (contrato Go)
```go
func (e *DbEngine) WorkflowTraceOTLP(runID string) (string, error)
// interfaz WorkflowStore: + WorkflowTraceOTLP(runID string) (string, error)
```
Handler: `musubi_workflow action=otel` (run_id) → `textResult(json)` o `jsonResult` del raw.
Se devuelve como **texto** (el JSON ya es el payload), para que sea copy/paste a un collector.

## Alternativas globales descartadas
- **Persistir spans** (Decisión 1). 
- **SDK OTel** (Decisión 3).
- **Push a collector desde Musubi:** rompe local-first y suma un cliente HTTP saliente; el
  transporte es responsabilidad del consumidor (curl/otel-cli).
