package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"musubi/internal/codeintel"
	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/embedding"
	"musubi/internal/logx"
	"musubi/internal/memory"
	"musubi/internal/skills"
	"musubi/internal/skillsource"

	"gopkg.in/yaml.v3"
)

const (
	defaultLimit = 5
	maxLimit     = 100
)

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *Property `json:"items,omitempty"`
}

type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type CallToolResponse struct {
	Content []TextContent `json:"content"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string) interface{} {
	return CallToolResponse{Content: []TextContent{{Type: "text", Text: text}}}
}

func jsonResult(v interface{}) (interface{}, *RpcError) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al serializar resultado: %v", err)
	}
	return textResult(string(jsonBytes)), nil
}

// clampLimit normaliza el límite recibido a un rango razonable.
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func (s *McpServer) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, *RpcError) {
	var callReq CallToolRequest
	if err := json.Unmarshal(params, &callReq); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid params: %v", err)
	}

	handler, ok := s.toolIndex[callReq.Name]
	if !ok {
		return nil, rpcErrorf(codeMethodNotFound, "Tool not found: %s", callReq.Name)
	}
	// Las tools de solo-lectura corren concurrentes entre sí (RLock); las que mutan
	// toman el lock exclusivo (serializadas, sin lost-updates de read-modify-write).
	if s.toolReadOnly[callReq.Name] {
		s.dispatchMu.RLock()
		defer s.dispatchMu.RUnlock()
	} else {
		s.dispatchMu.Lock()
		defer s.dispatchMu.Unlock()
	}
	return handler(ctx, callReq.Arguments)
}

func (s *McpServer) toolSaveObservation(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID         string  `json:"id"`
		TopicKey   string  `json:"topic_key"`
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.TopicKey) == "" {
		return nil, rpcErrorf(codeInvalidParams, "topic_key es obligatorio")
	}
	if strings.TrimSpace(args.Content) == "" {
		return nil, rpcErrorf(codeInvalidParams, "content es obligatorio")
	}
	importance := args.Importance
	if importance <= 0 {
		importance = 1.0
	}

	var emb []float32
	if embedding.Enabled(s.embedder) {
		embCtx, embCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer embCancel()
		vec, err := s.embedder.Embed(embCtx, args.Content)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al generar embedding: %v", err)
		}
		emb = vec
	}

	// Sin id explícito: deduplicar por contenido y autogenerar UUID.
	if strings.TrimSpace(args.ID) == "" {
		id, deduped, err := s.engine.SaveObservationDeduped(args.TopicKey, args.Content, importance, emb)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
		}
		if deduped {
			return textResult("Observación ya existente, no se duplicó (id: " + id + ")."), nil
		}
		return textResult("Observación guardada con éxito (id: " + id + ")." + s.detectAndSurface(id)), nil
	}

	// Con id explícito: upsert por id.
	if err := s.engine.SaveObservationWithImportance(args.ID, args.TopicKey, args.Content, importance, emb); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
	}
	return textResult("Observación guardada con éxito (id: " + args.ID + ")." + s.detectAndSurface(args.ID)), nil
}

// detectAndSurface corre la detección de conflictos para la observación recién
// guardada y devuelve un texto a anexar a la respuesta: anuncia los supersede
// auto-resueltos y pide veredicto (musubi_judge) para las relaciones pendientes.
// Si la detección está deshabilitada o no hay nada, devuelve "".
func (s *McpServer) detectAndSurface(obsID string) string {
	if !s.conflicts.Enabled {
		return ""
	}
	rels, err := s.engine.DetectRelations(obsID, memory.ConflictOptions{
		SimilarityFloor:      s.conflicts.SimilarityFloor,
		AutoResolveThreshold: s.conflicts.AutoResolveThreshold,
		CandidatePool:        s.conflicts.CandidatePool,
	})
	if err != nil {
		logx.Warn("detección de conflictos falló", "error", err)
		return ""
	}

	var auto, pending []memory.ObsRelation
	for _, r := range rels {
		if r.Status == memory.RelStatusResolved {
			auto = append(auto, r)
		} else {
			pending = append(pending, r)
		}
	}

	var b strings.Builder
	supersedes := 0
	for _, r := range auto {
		if r.Relation == memory.RelSupersedes {
			supersedes++
		}
	}
	if supersedes > 0 {
		fmt.Fprintf(&b, "\n[conflictos] Esta observación reemplaza (supersede) a %d anterior(es); quedaron ocultas del recall.", supersedes)
	}
	if len(pending) > 0 {
		b.WriteString("\n[conflictos] Detecté relación(es) que requieren tu veredicto (usá musubi_judge con el relation_id):")
		for _, r := range pending {
			fmt.Fprintf(&b, "\n- relation_id=%s target=%s (similitud %.2f): ¿se contradicen, una reemplaza a la otra, o son compatibles?", r.ID, r.TargetID, r.Confidence)
		}
	}
	return b.String()
}

// toolDoctor diagnostica o repara la base de memoria.
func (s *McpServer) toolDoctor(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Check  string `json:"check"`
		Repair bool   `json:"repair"`
		Mode   string `json:"mode"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	if args.Repair {
		if strings.TrimSpace(args.Check) == "" {
			return nil, rpcErrorf(codeInvalidParams, "repair requiere 'check' (qué reparar)")
		}
		mode := args.Mode
		if mode == "" {
			mode = "dry-run" // seguro por defecto: 'apply' debe ser explícito
		}
		res, err := s.engine.Repair(args.Check, mode)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo reparar: %v", err)
		}
		return jsonResult(res)
	}

	if strings.TrimSpace(args.Check) != "" {
		res, err := s.engine.RunCheck(args.Check)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(res)
	}

	rep, err := s.engine.Diagnose()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al diagnosticar: %v", err)
	}
	// Exponer last_maintenance para visibilidad del ciclo (T5.1). El struct embebido
	// promueve los campos de DiagnoseReport, así que el contrato existente no cambia;
	// solo se suma un campo extra.
	last, _, _ := s.engine.GetMeta("last_maintenance")
	return jsonResult(struct {
		memory.DiagnoseReport
		LastMaintenance string `json:"last_maintenance,omitempty"`
	}{DiagnoseReport: rep, LastMaintenance: last})
}

// phaseView es la respuesta de musubi_phase: el estado actual + su directiva.
type phaseView struct {
	Active    bool   `json:"active"`
	Task      string `json:"task,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Index     int    `json:"index,omitempty"`
	Total     int    `json:"total,omitempty"`
	Directive string `json:"directive,omitempty"`
	Message   string `json:"message,omitempty"`
}

func phaseViewFrom(st memory.PhaseState, active bool, message string) phaseView {
	v := phaseView{Active: active, Message: message}
	if active {
		v.Task = st.Task
		v.Phase = st.Phase
		v.Index = st.Index
		v.Total = st.Total
		v.Directive = memory.PhaseDirective(st.Phase)
	}
	return v
}

// toolPhase maneja el pipeline por fases (status/start/advance/set/clear).
func (s *McpServer) toolPhase(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action string `json:"action"`
		Task   string `json:"task"`
		Phase  string `json:"phase"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	phases := s.pipeline.Phases

	switch action := strings.TrimSpace(args.Action); action {
	case "", "status":
		st, ok, err := s.engine.PhaseStatus()
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al leer la fase: %v", err)
		}
		if !ok {
			return jsonResult(phaseViewFrom(memory.PhaseState{}, false, "No hay pipeline activo. Iniciá uno con action=start y task=<nombre>."))
		}
		return jsonResult(phaseViewFrom(st, true, ""))

	case "start":
		if strings.TrimSpace(args.Task) == "" {
			return nil, rpcErrorf(codeInvalidParams, "start requiere 'task'")
		}
		st, err := s.engine.StartPhase(args.Task, phases)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo iniciar el pipeline: %v", err)
		}
		return jsonResult(phaseViewFrom(st, true, "Pipeline iniciado."))

	case "advance":
		st, done, err := s.engine.AdvancePhase(phases)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo avanzar: %v", err)
		}
		if done {
			return jsonResult(phaseViewFrom(memory.PhaseState{}, false, "Pipeline completado. La tarea se cerró."))
		}
		return jsonResult(phaseViewFrom(st, true, "Avanzaste de fase."))

	case "set":
		if strings.TrimSpace(args.Phase) == "" {
			return nil, rpcErrorf(codeInvalidParams, "set requiere 'phase'")
		}
		st, err := s.engine.SetPhase(args.Phase, phases)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo fijar la fase: %v", err)
		}
		return jsonResult(phaseViewFrom(st, true, "Fase fijada."))

	case "clear":
		if err := s.engine.ClearPhase(); err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo cerrar el pipeline: %v", err)
		}
		return jsonResult(phaseViewFrom(memory.PhaseState{}, false, "Pipeline cerrado."))

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá status|start|advance|set|clear)", action)
	}
}

// toolWork maneja la pizarra compartida del multi-agente (plan/claim/complete/
// status/clear).
func (s *McpServer) toolWork(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action       string                `json:"action"`
		Batch        string                `json:"batch"`
		Units        []memory.WorkUnitSpec `json:"units"`
		Agent        string                `json:"agent"`
		ID           string                `json:"id"`
		Result       string                `json:"result"`
		Status       string                `json:"status"`
		FencingToken int64                 `json:"fencing_token"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	switch action := strings.TrimSpace(args.Action); action {
	case "plan":
		if len(args.Units) == 0 {
			return nil, rpcErrorf(codeInvalidParams, "plan requiere 'units' (al menos una)")
		}
		if max := s.multiagent.MaxBatchUnits; max > 0 && len(args.Units) > max {
			return nil, rpcErrorf(codeInvalidParams, "el batch excede el tope de %d unidades", max)
		}
		b, err := s.engine.CreateWorkBatch(args.Batch, args.Units)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo crear el batch: %v", err)
		}
		return jsonResult(b)

	case "claim":
		u, ok, err := s.engine.ClaimWorkUnit(args.Batch, args.Agent,
			s.multiagent.LeaseTTLSeconds, s.multiagent.MaxAttempts)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo reclamar: %v", err)
		}
		if !ok {
			return jsonResult(map[string]interface{}{"claimed": false})
		}
		// El agente debe renovar el lease con action=heartbeat (pasando id, agent y
		// fencing_token de la unidad) mientras trabaja, o perderá el claim al vencer.
		return jsonResult(map[string]interface{}{"claimed": true, "unit": u})

	case "heartbeat":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "heartbeat requiere 'id'")
		}
		if strings.TrimSpace(args.Agent) == "" {
			return nil, rpcErrorf(codeInvalidParams, "heartbeat requiere 'agent' (el dueño del lease)")
		}
		ok, err := s.engine.HeartbeatWorkUnit(args.ID, args.Agent, args.FencingToken, s.multiagent.LeaseTTLSeconds)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo renovar el lease: %v", err)
		}
		if !ok {
			// Fuiste expropiado o ya no sos el dueño: el agente debe detener el trabajo.
			return jsonResult(map[string]interface{}{"alive": false,
				"note": "lease no renovado: fuiste expropiado o ya no sos el dueño; detené el trabajo de esta unidad"})
		}
		return jsonResult(map[string]interface{}{"alive": true})

	case "complete":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'id'")
		}
		if err := s.engine.CompleteWorkUnit(args.ID, args.Result, args.Status, args.Agent, args.FencingToken); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo completar: %v", err)
		}
		return textResult("Unidad completada."), nil

	case "status":
		if strings.TrimSpace(args.Batch) == "" {
			return nil, rpcErrorf(codeInvalidParams, "status requiere 'batch'")
		}
		b, err := s.engine.WorkBatchStatus(args.Batch)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo leer el batch: %v", err)
		}
		return jsonResult(b)

	case "clear":
		if strings.TrimSpace(args.Batch) == "" {
			return nil, rpcErrorf(codeInvalidParams, "clear requiere 'batch'")
		}
		if err := s.engine.ClearWorkBatch(args.Batch); err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo limpiar el batch: %v", err)
		}
		return textResult("Batch limpiado."), nil

	case "savings":
		if strings.TrimSpace(args.Batch) == "" {
			return nil, rpcErrorf(codeInvalidParams, "savings requiere 'batch'")
		}
		b, err := s.engine.WorkBatchStatus(args.Batch)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo leer el batch: %v", err)
		}
		ds := memory.EstimateDelegationSavings(b,
			s.multiagent.AvoidedContextTokensPerUnit, s.multiagent.DelegationOverheadTokens)
		return jsonResult(ds)

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá plan|claim|complete|status|savings|clear)", action)
	}
}

// toolWorkflow es la interfaz MCP del motor de orquestación DAG (model-free).
// Musubi NO ejecuta los steps: define el grafo, persiste el estado y devuelve los
// steps listos; el agente ejecuta y reporta con 'complete'. El estado es resumible.
func (s *McpServer) toolWorkflow(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action         string `json:"action"`
		Workflow       string `json:"workflow"`   // id → .musubi/workflows/<id>.yaml
		Definition     string `json:"definition"` // YAML inline (alternativa a 'workflow')
		RunID          string `json:"run_id"`
		Step           string `json:"step"`
		Result         string `json:"result"`
		Status         string `json:"status"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// loadDef carga la definición desde 'definition' (YAML inline) o
	// .musubi/workflows/<workflow>.yaml. Reusado por start y validate.
	loadDef := func() (memory.WorkflowDef, *RpcError) {
		var data []byte
		if strings.TrimSpace(args.Definition) != "" {
			data = []byte(args.Definition)
		} else if strings.TrimSpace(args.Workflow) != "" {
			path := filepath.Join(s.projectPath, config.DirName, "workflows", args.Workflow+".yaml")
			b, err := os.ReadFile(path)
			if err != nil {
				return memory.WorkflowDef{}, rpcErrorf(codeInvalidParams, "no se pudo leer el workflow %q: %v", args.Workflow, err)
			}
			data = b
		} else {
			return memory.WorkflowDef{}, rpcErrorf(codeInvalidParams, "se requiere 'workflow' (id en .musubi/workflows/) o 'definition' (YAML)")
		}
		def, perr := memory.ParseWorkflowDef(data)
		if perr != nil {
			return memory.WorkflowDef{}, rpcErrorf(codeInvalidParams, "%v", perr)
		}
		return def, nil
	}

	switch action := strings.TrimSpace(args.Action); action {
	case "start":
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "start requiere 'run_id'")
		}
		def, rerr := loadDef()
		if rerr != nil {
			return nil, rerr
		}
		run, err := s.engine.StartWorkflowRun(args.RunID, def)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		return jsonResult(map[string]interface{}{"run": run, "ready": ready})

	case "validate":
		def, rerr := loadDef()
		if rerr != nil {
			return nil, rerr
		}
		errs := def.Validate()
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return jsonResult(map[string]interface{}{"valid": len(errs) == 0, "errors": msgs})

	case "list":
		runs, err := s.engine.WorkflowListRuns()
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		return jsonResult(map[string]interface{}{"runs": runs})

	case "next":
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "next requiere 'run_id'")
		}
		ready, err := s.engine.WorkflowReady(args.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(map[string]interface{}{"ready": ready})

	case "complete":
		if strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Step) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'run_id' y 'step'")
		}
		run, err := s.engine.CompleteWorkflowStep(args.RunID, args.Step, args.Result, args.Status, args.IdempotencyKey)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		return jsonResult(map[string]interface{}{"run": run, "ready": ready})

	case "journal":
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "journal requiere 'run_id'")
		}
		events, err := s.engine.WorkflowJournal(args.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		return jsonResult(map[string]interface{}{"run_id": args.RunID, "events": events})

	case "otel":
		// Deriva la traza OTLP/JSON del run (trace=run, span=step) para ingestión por un
		// collector OpenTelemetry. Read-only, derivado del journal.
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "otel requiere 'run_id'")
		}
		trace, err := s.engine.WorkflowTraceOTLP(args.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return textResult(trace), nil

	case "status", "resume":
		// status: estado completo. resume: lo mismo + steps listos, para retomar un
		// run en otra sesión (el estado vive en SQLite).
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "%s requiere 'run_id'", action)
		}
		run, ok, err := s.engine.WorkflowRunStatus(args.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		if !ok {
			return nil, rpcErrorf(codeInvalidParams, "run %q no existe", args.RunID)
		}
		if action == "status" {
			return jsonResult(run)
		}
		ready, rerr := s.engine.WorkflowReady(args.RunID)
		if rerr != nil {
			return nil, rpcErrorf(codeInternalError, "%v", rerr)
		}
		return jsonResult(map[string]interface{}{"run": run, "ready": ready})

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá start|next|complete|status|resume|validate|list|journal|otel)", action)
	}
}

// toolConflicts lista las relaciones pendientes de veredicto.
func (s *McpServer) toolConflicts(_ json.RawMessage) (interface{}, *RpcError) {
	rels, err := s.engine.PendingObsRelations()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al listar conflictos: %v", err)
	}
	if rels == nil {
		rels = []memory.ObsRelation{}
	}
	return jsonResult(map[string]interface{}{
		"count":     len(rels),
		"relations": rels,
	})
}

// validJudgeRelations son los veredictos que el agente puede emitir (pending no
// es un veredicto válido).
var validJudgeRelations = map[string]bool{
	memory.RelRelated:       true,
	memory.RelCompatible:    true,
	memory.RelScoped:        true,
	memory.RelConflictsWith: true,
	memory.RelSupersedes:    true,
	memory.RelNotConflict:   true,
}

// toolJudge emite el veredicto de una relación entre observaciones.
func (s *McpServer) toolJudge(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		RelationID string `json:"relation_id"`
		Relation   string `json:"relation"`
		Reason     string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	if strings.TrimSpace(args.RelationID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "relation_id es obligatorio")
	}
	if !validJudgeRelations[args.Relation] {
		return nil, rpcErrorf(codeInvalidParams, "relation inválida %q (usá related|compatible|scoped|conflicts_with|supersedes|not_conflict)", args.Relation)
	}
	if err := s.engine.ResolveObsRelation(args.RelationID, args.Relation, "agent", args.Reason); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo juzgar la relación: %v", err)
	}
	msg := fmt.Sprintf("Veredicto registrado: %s (relación %s).", args.Relation, args.RelationID)
	if args.Relation == memory.RelSupersedes {
		msg += " La observación target quedó oculta del recall."
	}
	return textResult(msg), nil
}

// maxRecallBudget acota el presupuesto pedido por el cliente a un rango sano.
const maxRecallBudget = 8000

func (s *McpServer) toolRecall(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query       string `json:"query"`
		TokenBudget int    `json:"token_budget"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query es obligatorio")
	}

	opts := memory.RecallOptions{
		TokenBudget:   s.memory.RecallTokenBudget,
		CandidatePool: s.memory.CandidatePool,
		GistMaxTokens: s.memory.GistMaxTokens,
	}
	if args.TokenBudget > 0 {
		opts.TokenBudget = args.TokenBudget
		if opts.TokenBudget > maxRecallBudget {
			opts.TokenBudget = maxRecallBudget
		}
	}

	// Recall híbrido (T5.7 R2): si hay embedder, embeber la query para sumar el pool
	// vectorial. Best-effort: si falla, se sigue solo con el léxico (no rompe el recall).
	if embedding.Enabled(s.embedder) {
		embCtx, embCancel := context.WithTimeout(ctx, 30*time.Second)
		vec, eerr := s.embedder.Embed(embCtx, args.Query)
		embCancel()
		if eerr != nil {
			logx.Error("recall: no se pudo embeber la query, sigo solo con léxico", "error", eerr)
		} else {
			opts.QueryVector = vec
		}
	}

	res, err := s.engine.Recall(ctx, args.Query, opts)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en recall: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolSaveFact(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Subject   string `json:"subject"`
		Predicate string `json:"predicate"`
		Object    string `json:"object"`
		ValidFrom string `json:"valid_from"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Subject) == "" || strings.TrimSpace(args.Predicate) == "" || strings.TrimSpace(args.Object) == "" {
		return nil, rpcErrorf(codeInvalidParams, "subject, predicate y object son obligatorios")
	}

	res, err := s.engine.SaveFact(args.Subject, args.Predicate, args.Object, args.ValidFrom, s.graph.SingleValuedPredicates)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar hecho: %v", err)
	}
	msg := fmt.Sprintf("Hecho guardado: %s %s %s.", args.Subject, args.Predicate, args.Object)
	if !res.Created {
		msg = fmt.Sprintf("Hecho re-afirmado (ya existía): %s %s %s.", args.Subject, args.Predicate, args.Object)
	}
	if res.Invalidated > 0 {
		// Cardinalidad: el predicado es funcional y este hecho reemplazó a otro(s).
		msg += fmt.Sprintf(" Invalidó %d hecho(s) previo(s) contradictorio(s) (predicado single-valued).", res.Invalidated)
	}
	return textResult(msg), nil
}

func (s *McpServer) toolRecallFacts(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Entity  string `json:"entity"`
		MaxHops int    `json:"max_hops"`
		AsOf    string `json:"as_of"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Entity) == "" {
		return nil, rpcErrorf(codeInvalidParams, "entity es obligatorio")
	}

	maxHops := s.graph.MaxHops
	if args.MaxHops > 0 {
		maxHops = args.MaxHops
	}

	res, err := s.engine.RecallFacts(args.Entity, maxHops, s.graph.MaxFacts, args.AsOf)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al recuperar hechos: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolEntityContext(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Entity  string `json:"entity"`
		MaxHops int    `json:"max_hops"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Entity) == "" {
		return nil, rpcErrorf(codeInvalidParams, "entity es obligatorio")
	}

	maxHops := s.graph.MaxHops
	if args.MaxHops > 0 {
		maxHops = args.MaxHops
	}

	res, err := s.engine.EntityContext(args.Entity, maxHops, s.graph.MaxFacts, s.graph.MaxObservations)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al ensamblar contexto de entidad: %v", err)
	}
	return jsonResult(res)
}

// maintainResponse es la respuesta de musubi_maintain. Cuando el throttle saltea
// el ciclo, Skipped=true y Report queda nil; cuando corre, Report trae el resumen.
// LastMaintenance refleja la marca tras la corrida (o la existente si se salteó).
type maintainResponse struct {
	Skipped         bool                      `json:"skipped"`
	Reason          string                    `json:"reason,omitempty"`
	LastMaintenance string                    `json:"last_maintenance,omitempty"`
	Report          *memory.MaintenanceReport `json:"report,omitempty"`
}

func (s *McpServer) toolMaintain(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Force bool `json:"force"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// Throttle del ciclo on-demand: sin force, no corre si no pasó el intervalo de
	// auto-mantenimiento. Protege contra disparar consolidación + VACUUM en loop.
	// (AutoIntervalHours=0 ⇒ siempre "due", sin throttle.)
	if !args.Force {
		due, err := s.engine.MaintenanceDue(s.maintenance.AutoIntervalHours)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al consultar el throttle: %v", err)
		}
		if !due {
			last, _, _ := s.engine.GetMeta("last_maintenance")
			return jsonResult(maintainResponse{
				Skipped:         true,
				Reason:          fmt.Sprintf("último mantenimiento hace menos de %.0fh; pasá force=true para correr igual", s.maintenance.AutoIntervalHours),
				LastMaintenance: last,
			})
		}
	}

	rep, err := s.engine.Maintain(s.maintenanceOptions())
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en el mantenimiento: %v", err)
	}
	// Marcar la corrida para que el throttle (y el scheduler de T5.2) la respeten.
	if mErr := s.engine.MarkMaintenanceNow(); mErr != nil {
		logx.Error("no se pudo marcar last_maintenance", "error", mErr)
	}
	last, _, _ := s.engine.GetMeta("last_maintenance")
	return jsonResult(maintainResponse{Skipped: false, LastMaintenance: last, Report: &rep})
}

func (s *McpServer) toolMemoryExpand(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		IDs       []string `json:"ids"`
		MaxTokens int      `json:"max_tokens"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if len(args.IDs) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "ids no puede estar vacío")
	}

	res, used, err := s.engine.GetObservationsBudget(args.IDs, args.MaxTokens)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al expandir memorias: %v", err)
	}
	// Contabilizar la hidratación en el ledger de la sesión activa (best-effort).
	_, _ = s.engine.LedgerAdd("", "hydration", used)
	return jsonResult(res)
}

func (s *McpServer) toolTokens(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action string `json:"action"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
		}
	}
	switch strings.TrimSpace(args.Action) {
	case "reset":
		if err := s.engine.LedgerReset(); err != nil {
			return nil, rpcErrorf(codeInternalError, "error al reiniciar el ledger: %v", err)
		}
		return jsonResult(memory.TokenLedger{Surfaces: map[string]int{}}.Budget(s.memory.SessionTokenBudget))
	case "", "status":
		l, err := s.engine.LedgerStatus()
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al leer el ledger: %v", err)
		}
		// Reporte del gobernador: ledger contra el presupuesto blando de sesión
		// (total, restante, % usado, estado y desglose por superficie).
		return jsonResult(l.Budget(s.memory.SessionTokenBudget))
	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida: %q (status | reset)", args.Action)
	}
}

func (s *McpServer) toolSaveCode(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Path    string `json:"path"`
		Gist    string `json:"gist"`
		Symbols string `json:"symbols"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Path) == "" || strings.TrimSpace(args.Gist) == "" {
		return nil, rpcErrorf(codeInvalidParams, "path y gist son obligatorios")
	}

	// Clave normalizada (relativa a la raíz) para que el hook PreToolUse encuentre
	// lo que guarda la tool. Fingerprint best-effort del contenido actual.
	key := memory.NormalizeCodePath(s.projectPath, args.Path)
	fp, _ := memory.FileFingerprint(s.projectPath, args.Path)
	// Símbolos DERIVADOS del contenido actual (mismo snapshot que el fingerprint) cuando
	// el llamador no los pasa: evita el string manual que se desincroniza. Si el llamador
	// pasa symbols explícito, se respeta (compat hacia atrás).
	symbols := args.Symbols
	if strings.TrimSpace(symbols) == "" {
		if content, rerr := s.readProjectFile(args.Path); rerr == nil {
			symbols = codeintel.FormatSymbols(codeintel.ExtractSymbols(key, content))
		}
	}
	cm := memory.CodeMemory{
		Path:        key,
		Gist:        args.Gist,
		Symbols:     symbols,
		Fingerprint: fp,
		Tokens:      memory.EstimateTokens(args.Gist),
	}
	if err := s.engine.SaveCodeMemory(cm); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar memoria de código: %v", err)
	}
	return jsonResult(map[string]interface{}{"ok": true, "path": cm.Path, "tokens": cm.Tokens})
}

func (s *McpServer) toolRecallCode(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, rpcErrorf(codeInvalidParams, "path es obligatorio")
	}

	key := memory.NormalizeCodePath(s.projectPath, args.Path)
	cm, ok, err := s.engine.GetCodeMemory(key)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer memoria de código: %v", err)
	}
	if !ok {
		return jsonResult(map[string]interface{}{"found": false, "path": key})
	}

	// Frescura: el gist sirve si el archivo no cambió desde que se guardó.
	current, ferr := memory.FileFingerprint(s.projectPath, args.Path)
	fresh := ferr == nil && current != "" && current == cm.Fingerprint
	_, _ = s.engine.LedgerAdd("", "code_recall", cm.Tokens)
	return jsonResult(map[string]interface{}{
		"found":   true,
		"path":    cm.Path,
		"gist":    cm.Gist,
		"symbols": cm.Symbols,
		"tokens":  cm.Tokens,
		"fresh":   fresh,
	})
}

func (s *McpServer) toolSearchSemantic(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query es obligatorio")
	}
	if !embedding.Enabled(s.embedder) {
		return nil, rpcErrorf(codeInvalidParams, "búsqueda semántica no disponible: no hay proveedor de embeddings configurado. Usá musubi_search_keyword o configurá embedding.provider en .musubi/config.yaml")
	}

	embCtx, embCancel := context.WithTimeout(ctx, 30*time.Second)
	defer embCancel()
	vec, err := s.embedder.Embed(embCtx, args.Query)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al generar embedding de la consulta: %v", err)
	}

	results, err := s.engine.SearchObservations(ctx, vec, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda semántica: %v", err)
	}
	return jsonResult(results)
}

func (s *McpServer) toolSearchKeyword(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		QueryText string `json:"query_text"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.QueryText) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query_text es obligatorio")
	}

	results, err := s.engine.SearchObservationsFTS(ctx, args.QueryText, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda por palabra clave: %v", err)
	}
	return jsonResult(results)
}

func (s *McpServer) toolLogError(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		FilePath       string `json:"file_path"`
		ErrorMessage   string `json:"error_message"`
		SuggestedPatch string `json:"suggested_patch"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.FilePath) == "" {
		return nil, rpcErrorf(codeInvalidParams, "file_path es obligatorio")
	}
	if strings.TrimSpace(args.ErrorMessage) == "" {
		return nil, rpcErrorf(codeInvalidParams, "error_message es obligatorio")
	}

	if err := s.engine.SaveTelemetryLog(args.FilePath, args.ErrorMessage, args.SuggestedPatch); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar log de telemetría: %v", err)
	}
	return textResult("Log de telemetría guardado con éxito."), nil
}

func (s *McpServer) toolResolveTelemetry(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if args.ID <= 0 {
		return nil, rpcErrorf(codeInvalidParams, "id debe ser un entero positivo")
	}

	if err := s.engine.ResolveTelemetryLog(args.ID); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al resolver telemetría: %v", err)
	}
	return textResult("Log de telemetría marcado como resuelto."), nil
}

// toolInsights devuelve el resumen de observabilidad activa (Track 6 / T6.4): tamaño de la
// memoria, hotspots de errores no resueltos, decisiones de skills y salud del ciclo. Read-only.
func (s *McpServer) toolInsights(raw json.RawMessage) (interface{}, *RpcError) {
	rep, err := s.engine.Insights()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al generar insights: %v", err)
	}
	return jsonResult(rep)
}

// slugRegex valida que el nombre de una skill sea un slug seguro para usar como nombre de archivo.
var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

// toolDetectStack detecta el ecosistema del proyecto usando el projectPath del servidor.
// No requiere parámetros; devuelve el slice []StackResult serializado como JSON.
func (s *McpServer) toolDetectStack(raw json.RawMessage) (interface{}, *RpcError) {
	results, err := detector.DetectStack(s.projectPath)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al detectar stack: %v", err)
	}
	return jsonResult(results)
}

// argsGuardarSkill contiene los parámetros del tool musubi_save_skill.
type argsGuardarSkill struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Triggers     []string `json:"triggers"`
	Capabilities []string `json:"capabilities"`
	Rules        string   `json:"rules"`
	Overwrite    bool     `json:"overwrite"`
}

// toolSaveSkill valida los argumentos y guarda la skill como YAML en .musubi/skills/.
// También escribe el sentinel de manera best-effort.
func (s *McpServer) toolSaveSkill(raw json.RawMessage) (interface{}, *RpcError) {
	var args argsGuardarSkill
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}

	// Validaciones estructurales (name slug, triggers, rules mínimas) — compartidas
	// con musubi_author_skill.
	if rerr := validateSkillStructural(args.Name, args.Triggers, args.Rules); rerr != nil {
		return nil, rerr
	}

	// Construir la skill con campos de procedencia.
	sk := skills.Skill{
		Name:         args.Name,
		Description:  args.Description,
		Triggers:     args.Triggers,
		Capabilities: args.Capabilities,
		Rules:        args.Rules,
		GeneratedBy:  "auto-discovery",
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// GATE DE CALIDAD (model-free): bloquea si hay errores; los warnings avisan.
	report := skills.ValidateSkillQuality(sk)
	if !report.OK() {
		return nil, rpcErrorf(codeInvalidParams, "la skill no pasa el gate de calidad:\n%s", formatIssues(report.Errors))
	}

	// Puerta de sobrescritura: rechazar si el archivo existe y overwrite=false.
	skillsDir := filepath.Join(s.projectPath, config.DirName, config.SkillsDir)
	skillPath := filepath.Join(skillsDir, args.Name+".yaml")
	if _, err := os.Stat(skillPath); err == nil && !args.Overwrite {
		return nil, rpcErrorf(codeInvalidParams, "la skill %q ya existe; pasa overwrite=true para reemplazarla", args.Name)
	}

	path, rerr := s.writeSkillFile(sk)
	if rerr != nil {
		return nil, rerr
	}
	return textResult(skillSaveMessage(args.Name, path, report)), nil
}

// validateSkillStructural aplica las validaciones estructurales de una skill
// (name slug-safe, ≥1 trigger glob válido, rules ≥20 chars). Compartida por
// toolSaveSkill y toolAuthorSkill para no duplicar ni desincronizar los checks.
func validateSkillStructural(name string, triggers []string, rules string) *RpcError {
	if strings.TrimSpace(name) == "" {
		return rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	if !slugRegex.MatchString(name) {
		return rpcErrorf(codeInvalidParams, "name debe ser un slug válido (solo letras minúsculas, números y guiones, ej. 'mi-skill'): %q", name)
	}
	if len(triggers) == 0 {
		return rpcErrorf(codeInvalidParams, "triggers no puede estar vacío")
	}
	for _, t := range triggers {
		if _, err := filepath.Match(t, ""); err != nil {
			return rpcErrorf(codeInvalidParams, "trigger inválido %q: %v", t, err)
		}
	}
	if len(strings.TrimSpace(rules)) < 20 {
		return rpcErrorf(codeInvalidParams, "rules debe tener al menos 20 caracteres (actual: %d)", len(strings.TrimSpace(rules)))
	}
	return nil
}

// writeSkillFile serializa y persiste una skill en .musubi/skills/<name>.yaml, escribe
// el sentinel y actualiza la huella del stack (best-effort). Incluye la defensa de path
// traversal (cinturón y tirantes). Compartida por toolSaveSkill y toolAuthorSkill.
func (s *McpServer) writeSkillFile(sk skills.Skill) (string, *RpcError) {
	skillsDir := filepath.Join(s.projectPath, config.DirName, config.SkillsDir)
	skillPath := filepath.Join(skillsDir, sk.Name+".yaml")
	if !strings.HasPrefix(filepath.Clean(skillPath), filepath.Clean(skillsDir)) {
		return "", rpcErrorf(codeInvalidParams, "nombre de skill no permitido: %q", sk.Name)
	}
	data, err := yaml.Marshal(sk)
	if err != nil {
		return "", rpcErrorf(codeInternalError, "error al serializar skill: %v", err)
	}
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return "", rpcErrorf(codeInternalError, "error al crear directorio de skills: %v", err)
	}
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		return "", rpcErrorf(codeInternalError, "error al escribir skill: %v", err)
	}
	// Sentinel (best-effort): marca que ya se generaron skills.
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)
	if err := os.WriteFile(sentinelPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0644); err != nil {
		logx.Warn("no se pudo escribir el sentinel", "error", err)
	}
	// Huella del stack (best-effort): el hook SessionStart no vuelve a pedir generación
	// hasta que el stack realmente cambie.
	if s.engine != nil {
		stack, _ := detector.DetectStack(s.projectPath)
		if err := s.engine.SetMeta(memory.MetaStackFingerprint, detector.StackFingerprint(stack)); err != nil {
			logx.Warn("no se pudo actualizar la huella del stack", "error", err)
		}
	}
	return skillPath, nil
}

// formatIssues renderiza una lista de hallazgos de calidad como líneas accionables.
func formatIssues(issues []skills.QualityIssue) string {
	var b strings.Builder
	for _, i := range issues {
		fmt.Fprintf(&b, "- [%s] %s → %s\n", i.Code, i.Message, i.Fix)
	}
	return b.String()
}

// skillSaveMessage arma el mensaje de éxito del guardado con el score y, si hay,
// los avisos de calidad (que no bloquearon).
func skillSaveMessage(name, path string, r skills.QualityReport) string {
	msg := fmt.Sprintf("skill %q guardada en %s (calidad %d/100)", name, path, r.Score)
	if len(r.Warnings) > 0 {
		msg += fmt.Sprintf("\nAvisos de calidad (%d, no bloquean):\n%s", len(r.Warnings), formatIssues(r.Warnings))
	}
	return msg
}

// toolAuthorSkill es el SISTEMA DE CREACIÓN AVANZADO de skills: valida la calidad de
// una skill y devuelve un reporte scoreado con fixes accionables SIN guardar (save=false,
// default), para iterar; con save=true guarda solo si pasa el gate de calidad. Su guía
// recomienda derivar las rules de fuentes confiables (doc oficial del stack + repos
// reputados), y reporta el tier de confiabilidad de la fuente.
func (s *McpServer) toolAuthorSkill(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Triggers     []string `json:"triggers"`
		Capabilities []string `json:"capabilities"`
		Rules        string   `json:"rules"`
		SourceURL    string   `json:"source_url"`
		Save         bool     `json:"save"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}

	sk := skills.Skill{
		Name:         args.Name,
		Description:  args.Description,
		Triggers:     args.Triggers,
		Capabilities: args.Capabilities,
		Rules:        args.Rules,
		SourceURL:    args.SourceURL,
	}
	report := skills.ValidateSkillQuality(sk)
	resp := map[string]interface{}{
		"skill":        args.Name,
		"score":        report.Score,
		"ok":           report.OK(),
		"errors":       report.Errors,
		"warnings":     report.Warnings,
		"source_trust": skills.SourceTrustTier(args.SourceURL),
	}

	if !args.Save {
		resp["saved"] = false
		resp["note"] = "Reporte de calidad (no guardado). Corregí los errores, derivá las rules de fuentes confiables (doc oficial del stack + repos reputados como anthropics/skills, awesome-cursorrules) y volvé a llamar con save=true."
		return jsonResult(resp)
	}

	// save=true: estructural primero, luego el gate de calidad.
	if rerr := validateSkillStructural(args.Name, args.Triggers, args.Rules); rerr != nil {
		return nil, rerr
	}
	if !report.OK() {
		return nil, rpcErrorf(codeInvalidParams, "la skill no pasa el gate de calidad:\n%s", formatIssues(report.Errors))
	}
	sk.GeneratedBy = "author"
	sk.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	path, rerr := s.writeSkillFile(sk)
	if rerr != nil {
		return nil, rerr
	}
	resp["saved"] = true
	resp["path"] = path
	return jsonResult(resp)
}

// toolSearchSkills busca skills aplicables al proyecto desde el catálogo remoto.
// Inputs opcionales: query (string), stack (string), limit (int).
// Degradación graciosa: catálogo caído → textResult con guía, sin RpcError.
func (s *McpServer) toolSearchSkills(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query string `json:"query"`
		Stack string `json:"stack"`
		Limit int    `json:"limit"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// Si el sourcing está deshabilitado, guiar al fallback sin tocar el catálogo.
	if !s.sourcing.Enabled {
		return textResult("El sourcing de skills está deshabilitado en la configuración de Musubi. " +
			"Para buscar skills manualmente, investigá la documentación oficial del stack detectado " +
			"y usá musubi_save_skill para guardar las reglas."), nil
	}

	// Obtener el catálogo: primero del caché (TTL = CacheSeconds), si no, de la red con
	// timeout vía contexto (5 segundos). Solo se cachean fetches exitosos.
	cacheKey := "catalog:" + s.sourcing.CatalogURL
	var cat skillsource.Catalog
	if v, ok := s.sourceCache.get(cacheKey); ok {
		cat = v.(skillsource.Catalog)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fetched, err := skillsource.FetchCatalog(ctx, s.sourcing.CatalogURL)
		if err != nil {
			// Degradación graciosa: catálogo inaccesible → texto explicativo, no RpcError.
			return textResult(fmt.Sprintf(
				"El catálogo de skills no está disponible en este momento (%v). "+
					"Podés buscar skills manualmente en la documentación oficial del stack detectado "+
					"y guardarlas con musubi_save_skill.", err)), nil
		}
		cat = fetched
		s.sourceCache.set(cacheKey, cat)
	}

	// Detectar stack y dependencias del proyecto actual.
	stacks, _ := detector.DetectStack(s.projectPath)
	deps, _ := detector.ExtractDeps(s.projectPath)

	// Determinar límite efectivo.
	maxCands := s.sourcing.MaxCandidates
	if args.Limit > 0 && args.Limit < maxCands {
		maxCands = args.Limit
	}

	// Filtrar candidatos aplicables.
	candidatos := skillsource.FilterCatalog(cat, s.projectPath, deps, stacks, maxCands)

	// Filtro adicional en memoria por stack (si se especificó).
	if args.Stack != "" {
		filtrados := candidatos[:0]
		for _, c := range candidatos {
			for _, st := range c.Entry.Stacks {
				if strings.EqualFold(st, args.Stack) {
					filtrados = append(filtrados, c)
					break
				}
			}
		}
		candidatos = filtrados
	}

	// Filtro adicional por query (nombre o descripción).
	if args.Query != "" {
		q := strings.ToLower(args.Query)
		filtrados := candidatos[:0]
		for _, c := range candidatos {
			if strings.Contains(strings.ToLower(c.Entry.Name), q) ||
				strings.Contains(strings.ToLower(c.Entry.Description), q) {
				filtrados = append(filtrados, c)
			}
		}
		candidatos = filtrados
	}

	// Feedback de decisiones (Track 6 / T6.1): Musubi no re-propone las skills que el
	// usuario ya rechazó. Best-effort: si la lectura falla, se devuelve sin filtrar.
	if decisions, derr := s.engine.GetSkillDecisions(); derr == nil {
		candidatos = excludeRejectedSkills(candidatos, decisions)
	}

	return jsonResult(candidatos)
}

// excludeRejectedSkills quita del listado los candidatos cuya decisión MÁS RECIENTE fue
// "rejected": Musubi aprende de las decisiones y no re-propone lo descartado (T6.1).
// Last-write-wins (GetSkillDecisions viene ordenado por id ASC): una skill rechazada y luego
// aceptada vuelve a proponerse. Matchea por id (slug), la misma clave que log_skill_decision.
func excludeRejectedSkills(cands []skillsource.Candidate, decisions []memory.SkillDecision) []skillsource.Candidate {
	if len(decisions) == 0 {
		return cands
	}
	latest := make(map[string]string, len(decisions))
	for _, d := range decisions {
		latest[d.SkillID] = d.Decision
	}
	out := make([]skillsource.Candidate, 0, len(cands))
	for _, c := range cands {
		if latest[c.Entry.ID] == "rejected" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// toolDiscoverSkills DESCUBRE Agent Skills (SKILL.md) en un marketplace externo, filtradas
// por el stack del proyecto. Es un canal distinto de musubi_search_skills (catálogo curado):
// el marketplace tiene escala (~1.7M skills de GitHub) pero no sabe del proyecto; Musubi
// aporta lo que falta —arma la query desde el stack detectado— y devuelve candidatos con su
// githubUrl para que el USUARIO los revise e instale. Solo descubre: nunca baja, ejecuta ni
// instala el SKILL.md (contenido no confiable). Inputs opcionales: query (string), limit (int).
// Opt-in: si el marketplace está deshabilitado → guía textual. Degradación graciosa ante red.
func (s *McpServer) toolDiscoverSkills(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// Opt-in: el descubrimiento desde el marketplace externo es contenido no confiable, así
	// que está apagado por defecto. Si está off, guiar sin tocar la red.
	if !s.sourcing.MarketplaceEnabled {
		return textResult("El descubrimiento de skills desde el marketplace está deshabilitado " +
			"(sourcing.marketplace_enabled). Activalo en la config de Musubi para descubrir Agent " +
			"Skills de la comunidad filtradas por tu stack. Recordá: Musubi solo las enlaza; revisá " +
			"siempre el código en GitHub antes de instalar."), nil
	}

	// Construir la query: si el usuario no dio una, derivarla del stack detectado (la pieza
	// que el marketplace no conoce). El endpoint exige una query no vacía (no soporta '*').
	query := strings.TrimSpace(args.Query)
	if query == "" {
		stacks, _ := detector.DetectStack(s.projectPath)
		query = marketplaceQueryFromStack(stacks)
	}
	if query == "" {
		return textResult("No pude inferir el stack del proyecto para armar la búsqueda. " +
			"Pasá un 'query' explícito a musubi_discover_skills (ej. el lenguaje o framework)."), nil
	}

	// 1) Catálogo ESTÁTICO cosechado (default): cero rate limit. Si está configurado y se
	// puede leer (cacheado), se sirve de ahí. Ante cualquier fallo, se cae al modo live.
	if s.sourcing.MarketplaceCatalogURL != "" {
		if results, ok := s.discoverFromStaticCatalog(query, args.Limit); ok {
			return jsonResult(map[string]interface{}{
				"source": "catalog", "query": query, "count": len(results),
				"skills": results, "note": discoverSkillsNote,
			})
		}
	}

	// 2) Modo LIVE (fallback): pega a la API del marketplace. API key opcional vía env var
	// (sube el rate limit); vacío => tier anónimo.
	var apiKey string
	if s.sourcing.MarketplaceAPIKeyEnv != "" {
		apiKey = os.Getenv(s.sourcing.MarketplaceAPIKeyEnv)
	}

	// Caché por (URL, query, limit): las queries de descubrimiento se repiten (la derivada
	// del stack es estable), así que cachear el resultado ahorra llamadas a la API y su
	// rate limit. Solo se cachean fetches exitosos.
	cacheKey := fmt.Sprintf("marketplace:%s|%s|%d", s.sourcing.MarketplaceURL, query, args.Limit)
	var results []skillsource.MarketplaceSkill
	if v, ok := s.sourceCache.get(cacheKey); ok {
		results = v.([]skillsource.MarketplaceSkill)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fetched, err := skillsource.FetchMarketplaceSkills(ctx, s.sourcing.MarketplaceURL, apiKey, query, args.Limit)
		if err != nil {
			// Degradación graciosa: marketplace inaccesible → texto, no RpcError.
			return textResult(fmt.Sprintf(
				"El marketplace de skills no está disponible en este momento (%v). "+
					"Volvé a intentar más tarde o buscá skills manualmente.", err)), nil
		}
		results = fetched
		s.sourceCache.set(cacheKey, results)
	}

	return jsonResult(map[string]interface{}{
		"source": "live", "query": query, "count": len(results),
		"skills": results, "note": discoverSkillsNote,
	})
}

// discoverSkillsNote es el recordatorio que acompaña todo resultado de descubrimiento:
// Musubi solo enlaza, el usuario revisa e instala.
const discoverSkillsNote = "Resultados de descubrimiento: Musubi NO instala estas skills. " +
	"Revisá el código en 'githubUrl' antes de adoptarlas e instalalas vos mismo."

// discoverFromStaticCatalog intenta servir el descubrimiento desde el catálogo estático
// cosechado (cacheado con TTL). Devuelve ok=false ante cualquier problema (URL inaccesible,
// JSON inválido) para que el caller caiga al modo live. Cero rate limit del marketplace.
func (s *McpServer) discoverFromStaticCatalog(query string, limit int) ([]skillsource.MarketplaceSkill, bool) {
	url := s.sourcing.MarketplaceCatalogURL
	cacheKey := "mpcatalog:" + url
	var cat skillsource.MarketplaceCatalog
	if v, ok := s.sourceCache.get(cacheKey); ok {
		cat = v.(skillsource.MarketplaceCatalog)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fetched, err := skillsource.FetchMarketplaceCatalog(ctx, url)
		if err != nil {
			logx.Warn("discover: catálogo estático inaccesible, fallback a live", "error", err)
			return nil, false
		}
		cat = fetched
		s.sourceCache.set(cacheKey, cat)
	}
	return skillsource.FilterMarketplaceSkills(cat.Skills, query, limit), true
}

// marketplaceQueryFromStack arma una query de búsqueda para el marketplace a partir del
// stack detectado: junta ecosistemas y frameworks (ej. "Go", "Node.js react"). Devuelve ""
// si no se detectó nada, para que el llamador pida un query explícito.
func marketplaceQueryFromStack(stacks []detector.StackResult) string {
	seen := map[string]bool{}
	var terms []string
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			return
		}
		seen[strings.ToLower(t)] = true
		terms = append(terms, t)
	}
	for _, st := range stacks {
		add(st.Ecosystem)
		for _, fw := range st.Frameworks {
			add(fw)
		}
	}
	return strings.Join(terms, " ")
}

// toolLogSkillDecision registra una decisión de skill (accepted/rejected) en SQLite.
// Inputs: skill_id (requerido), decision (requerido, "accepted"|"rejected"),
// name (opcional), reason (opcional).
func (s *McpServer) toolLogSkillDecision(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		SkillID  string `json:"skill_id"`
		Name     string `json:"name"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}

	// Validar skill_id: requerido y debe ser slug válido.
	if strings.TrimSpace(args.SkillID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "skill_id es obligatorio")
	}
	if !slugRegex.MatchString(args.SkillID) {
		return nil, rpcErrorf(codeInvalidParams, "skill_id debe ser un slug válido (solo letras minúsculas, números y guiones): %q", args.SkillID)
	}

	// Validar decision: debe ser "accepted" o "rejected".
	if args.Decision != "accepted" && args.Decision != "rejected" {
		return nil, rpcErrorf(codeInvalidParams, "decision debe ser 'accepted' o 'rejected', se recibió: %q", args.Decision)
	}

	if err := s.engine.SaveSkillDecision(args.SkillID, args.Name, args.Decision, args.Reason); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar decisión de skill: %v", err)
	}
	return textResult(fmt.Sprintf("Decisión '%s' para skill '%s' registrada con éxito.", args.Decision, args.SkillID)), nil
}

func (s *McpServer) toolResolveSkills(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ModifiedFiles []string `json:"modified_files"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if len(args.ModifiedFiles) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "modified_files no puede estar vacío")
	}

	activeSkills, err := s.resolver.ResolveSkills(args.ModifiedFiles)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al resolver skills: %v", err)
	}

	// Telemetría RELEVANTE (Track 6 / T6.2): solo los errores no resueltos de los archivos
	// que el agente está tocando, no toda la telemetría pendiente.
	telemetryLogs, err := s.engine.GetUnresolvedTelemetryLogsForFiles(args.ModifiedFiles)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al obtener telemetría: %v", err)
	}

	return jsonResult(map[string]interface{}{
		"active_skills":  activeSkills,
		"telemetry_logs": telemetryLogs,
	})
}
