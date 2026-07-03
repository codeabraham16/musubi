package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// otel.go DERIVA una traza OpenTelemetry (OTLP/JSON) del run journal, sin persistir
// ninguna tabla de spans: el journal (run_events) es la fuente de verdad append-only, la
// traza es una vista calculada en el momento del export (principio "derivar, no
// guardar-y-desfasar", el mismo que rige detect_changes). Un run es un TRACE; cada step,
// un SPAN. Model-free: ids deterministas por hash, tiempos derivados de created_at; cero
// SDK de OpenTelemetry (el OTLP/JSON se emite a mano). El JSON resultante es ingeribile
// por cualquier collector OTel (Jaeger, Grafana Tempo, etc.).

// Separador de unidad (0x1f): junta run_id y step_id al derivar el span_id sin riesgo de
// colisión (ningún id de step lo contiene), evitando (run="a",step="bc") == (run="ab",step="c").
const otelIDSep = "\x1f"

// Códigos de span kind y status de OTLP.
const (
	otelKindInternal = 1 // SPAN_KIND_INTERNAL: trabajo no-RPC
	otelStatusUnset  = 0
	otelStatusOK     = 1 // STATUS_CODE_OK
	otelStatusError  = 2 // STATUS_CODE_ERROR
)

// --- Structs OTLP/JSON mínimos (subconjunto del formato traces). ---

type otlpDoc struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource     `json:"resource"`
	ScopeSpans []otlpScopeSpans `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpKV `json:"attributes"`
}

type otlpScopeSpans struct {
	Scope otlpScope  `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}

type otlpScope struct {
	Name string `json:"name"`
}

type otlpSpan struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId,omitempty"`
	Name              string     `json:"name"`
	Kind              int        `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []otlpKV   `json:"attributes,omitempty"`
	Status            otlpStatus `json:"status"`
}

type otlpStatus struct {
	Code int `json:"code"`
}

type otlpKV struct {
	Key   string  `json:"key"`
	Value otlpVal `json:"value"`
}

// otlpVal codifica un valor de atributo. Los int64 van como string (convención OTLP/JSON).
type otlpVal struct {
	StringValue *string `json:"stringValue,omitempty"`
	IntValue    *string `json:"intValue,omitempty"`
	BoolValue   *bool   `json:"boolValue,omitempty"`
}

func kvStr(key, val string) otlpKV  { return otlpKV{Key: key, Value: otlpVal{StringValue: &val}} }
func kvInt(key string, n int) otlpKV {
	s := strconv.Itoa(n)
	return otlpKV{Key: key, Value: otlpVal{IntValue: &s}}
}
func kvBool(key string, b bool) otlpKV { return otlpKV{Key: key, Value: otlpVal{BoolValue: &b}} }

// otelTraceID deriva un trace_id OTel (16 bytes → 32 hex) determinista de run_id.
func otelTraceID(runID string) string {
	sum := sha256.Sum256([]byte(runID))
	return hex.EncodeToString(sum[:16])
}

// otelSpanID deriva un span_id OTel (8 bytes → 16 hex) determinista de (run_id, stepID).
func otelSpanID(runID, stepID string) string {
	sum := sha256.Sum256([]byte(runID + otelIDSep + stepID))
	return hex.EncodeToString(sum[:8])
}

// parseJournalTimeNano convierte un created_at del journal ('YYYY-MM-DD HH:MM:SS', UTC) a
// nanosegundos unix como string (formato OTLP/JSON). Vacío/ inválido → "" (el caller degrada).
func parseJournalTimeNano(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(t.UTC().UnixNano(), 10)
}

// WorkflowTraceOTLP deriva la traza OTLP/JSON de un run a partir de su journal. Un run es un
// trace (span raíz), cada step un span hijo. Devuelve error si el run no tiene eventos.
func (e *DbEngine) WorkflowTraceOTLP(runID string) (string, error) {
	events, err := e.WorkflowJournal(runID)
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", fmt.Errorf("el run %q no tiene eventos (no existe o no se ejecutó)", runID)
	}

	traceID := otelTraceID(runID)
	rootID := otelSpanID(runID, "__run__")

	// Marcas temporales del run: inicio = primer evento; fin = run_done, o el último evento.
	rootStartNano := parseJournalTimeNano(events[0].CreatedAt)
	rootEndNano := parseJournalTimeNano(events[len(events)-1].CreatedAt)
	workflowID := ""
	for _, ev := range events {
		if ev.EventType == EventRunStarted {
			workflowID = ev.Payload // run_started lleva el workflow_id en el payload
		}
		if ev.EventType == EventRunDone {
			rootEndNano = parseJournalTimeNano(ev.CreatedAt)
		}
	}
	if workflowID == "" {
		workflowID = runID
	}

	spans := []otlpSpan{{
		TraceID:           traceID,
		SpanID:            rootID,
		Name:              workflowID,
		Kind:              otelKindInternal,
		StartTimeUnixNano: rootStartNano,
		EndTimeUnixNano:   rootEndNano,
		Attributes: []otlpKV{
			kvStr("musubi.run_id", runID),
			kvStr("musubi.workflow_id", workflowID),
		},
		Status: otlpStatus{Code: otelStatusOK},
	}}

	// Un span hijo por step (a partir de step_completed / step_skipped). El inicio del span
	// es el created_at del evento inmediatamente anterior (mejor aproximación de la ventana
	// del step, ya que el journal no registra step_started); nunca antes del inicio del run.
	for i, ev := range events {
		if ev.EventType != EventStepCompleted && ev.EventType != EventStepSkipped {
			continue // run_started/run_done → raíz; step_reopened → no genera span propio
		}
		startNano := rootStartNano
		if i > 0 {
			if prev := parseJournalTimeNano(events[i-1].CreatedAt); prev != "" {
				startNano = prev
			}
		}
		endNano := parseJournalTimeNano(ev.CreatedAt)
		if endNano == "" {
			endNano = rootEndNano
		}

		attrs := []otlpKV{
			kvInt("musubi.seq", ev.Seq),
			kvStr("musubi.event_type", ev.EventType),
		}
		status := otlpStatus{Code: otelStatusOK}
		if ev.EventType == EventStepSkipped {
			attrs = append(attrs, kvBool("musubi.skipped", true))
		} else {
			// step_completed: extraer status/result del payload JSON.
			var p struct {
				Status string `json:"status"`
				Result string `json:"result"`
			}
			if json.Unmarshal([]byte(ev.Payload), &p) == nil {
				if p.Status != "" {
					attrs = append(attrs, kvStr("musubi.step_status", p.Status))
				}
				if p.Result != "" {
					attrs = append(attrs, kvStr("musubi.result", p.Result))
				}
				if p.Status == StepFailed {
					status = otlpStatus{Code: otelStatusError}
				}
			}
		}

		spans = append(spans, otlpSpan{
			TraceID:           traceID,
			SpanID:            otelSpanID(runID, ev.StepID),
			ParentSpanID:      rootID,
			Name:              ev.StepID,
			Kind:              otelKindInternal,
			StartTimeUnixNano: startNano,
			EndTimeUnixNano:   endNano,
			Attributes:        attrs,
			Status:            status,
		})
	}

	doc := otlpDoc{ResourceSpans: []otlpResourceSpans{{
		Resource:   otlpResource{Attributes: []otlpKV{kvStr("service.name", "musubi")}},
		ScopeSpans: []otlpScopeSpans{{Scope: otlpScope{Name: "musubi/workflow"}, Spans: spans}},
	}}}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error serializando la traza OTLP: %w", err)
	}
	return string(out), nil
}
