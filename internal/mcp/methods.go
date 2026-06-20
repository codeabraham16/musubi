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
		Action string                `json:"action"`
		Batch  string                `json:"batch"`
		Units  []memory.WorkUnitSpec `json:"units"`
		Agent  string                `json:"agent"`
		ID     string                `json:"id"`
		Result string                `json:"result"`
		Status string                `json:"status"`
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
		u, ok, err := s.engine.ClaimWorkUnit(args.Batch, args.Agent)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo reclamar: %v", err)
		}
		if !ok {
			return jsonResult(map[string]interface{}{"claimed": false})
		}
		return jsonResult(map[string]interface{}{"claimed": true, "unit": u})

	case "complete":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'id'")
		}
		if err := s.engine.CompleteWorkUnit(args.ID, args.Result, args.Status, args.Agent); err != nil {
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

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá plan|claim|complete|status|clear)", action)
	}
}

// toolWorkflow es la interfaz MCP del motor de orquestación DAG (model-free).
// Musubi NO ejecuta los steps: define el grafo, persiste el estado y devuelve los
// steps listos; el agente ejecuta y reporta con 'complete'. El estado es resumible.
func (s *McpServer) toolWorkflow(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action     string `json:"action"`
		Workflow   string `json:"workflow"`   // id → .musubi/workflows/<id>.yaml
		Definition string `json:"definition"` // YAML inline (alternativa a 'workflow')
		RunID      string `json:"run_id"`
		Step       string `json:"step"`
		Result     string `json:"result"`
		Status     string `json:"status"`
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
		run, err := s.engine.CompleteWorkflowStep(args.RunID, args.Step, args.Result, args.Status)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		return jsonResult(map[string]interface{}{"run": run, "ready": ready})

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
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá start|next|complete|status|resume|validate|list)", action)
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
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Subject) == "" || strings.TrimSpace(args.Predicate) == "" || strings.TrimSpace(args.Object) == "" {
		return nil, rpcErrorf(codeInvalidParams, "subject, predicate y object son obligatorios")
	}

	res, err := s.engine.SaveFact(args.Subject, args.Predicate, args.Object)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar hecho: %v", err)
	}
	if res.Created {
		return textResult(fmt.Sprintf("Hecho guardado: %s %s %s.", args.Subject, args.Predicate, args.Object)), nil
	}
	return textResult("El hecho ya existía, no se duplicó."), nil
}

func (s *McpServer) toolRecallFacts(raw json.RawMessage) (interface{}, *RpcError) {
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

	res, err := s.engine.RecallFacts(args.Entity, maxHops, s.graph.MaxFacts)
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
		return jsonResult(memory.TokenLedger{Surfaces: map[string]int{}})
	case "", "status":
		l, err := s.engine.LedgerStatus()
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al leer el ledger: %v", err)
		}
		return jsonResult(l)
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
	cm := memory.CodeMemory{
		Path:        key,
		Gist:        args.Gist,
		Symbols:     args.Symbols,
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

	// Validar nombre: no vacío y slug-safe (previene path traversal).
	if strings.TrimSpace(args.Name) == "" {
		return nil, rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	if !slugRegex.MatchString(args.Name) {
		return nil, rpcErrorf(codeInvalidParams, "name debe ser un slug válido (solo letras minúsculas, números y guiones, ej. 'mi-skill'): %q", args.Name)
	}

	// Validar triggers: al menos uno y cada uno debe ser un glob válido.
	if len(args.Triggers) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "triggers no puede estar vacío")
	}
	for _, t := range args.Triggers {
		if _, err := filepath.Match(t, ""); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "trigger inválido %q: %v", t, err)
		}
	}

	// Validar rules: no vacío y mínimo 20 caracteres.
	if len(strings.TrimSpace(args.Rules)) < 20 {
		return nil, rpcErrorf(codeInvalidParams, "rules debe tener al menos 20 caracteres (actual: %d)", len(strings.TrimSpace(args.Rules)))
	}

	// Construir ruta y aplicar defensa de path traversal adicional.
	skillsDir := filepath.Join(s.projectPath, config.DirName, config.SkillsDir)
	skillPath := filepath.Join(skillsDir, args.Name+".yaml")
	// Verificar que la ruta resultante está bajo el directorio de skills (cinturón y tirantes).
	if !strings.HasPrefix(filepath.Clean(skillPath), filepath.Clean(skillsDir)) {
		return nil, rpcErrorf(codeInvalidParams, "nombre de skill no permitido: %q", args.Name)
	}

	// Puerta de sobrescritura: rechazar si el archivo existe y overwrite=false.
	if _, err := os.Stat(skillPath); err == nil && !args.Overwrite {
		return nil, rpcErrorf(codeInvalidParams, "la skill %q ya existe; pasa overwrite=true para reemplazarla", args.Name)
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

	// Serializar a YAML.
	data, err := yaml.Marshal(sk)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al serializar skill: %v", err)
	}

	// Crear el directorio de skills si no existe.
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al crear directorio de skills: %v", err)
	}

	// Escribir el archivo YAML de la skill.
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al escribir skill: %v", err)
	}

	// Escribir el sentinel (best-effort: fallo no cancela el guardado de la skill).
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(sentinelPath, []byte(timestamp), 0644); err != nil {
		logx.Warn("no se pudo escribir el sentinel", "error", err)
	}

	// Actualizar la huella del stack (best-effort): marca que las skills cubren el
	// stack actual, así el hook SessionStart no vuelve a pedir generación hasta que
	// el stack realmente cambie.
	if s.engine != nil {
		stack, _ := detector.DetectStack(s.projectPath)
		if err := s.engine.SetMeta(memory.MetaStackFingerprint, detector.StackFingerprint(stack)); err != nil {
			logx.Warn("no se pudo actualizar la huella del stack", "error", err)
		}
	}

	return textResult(fmt.Sprintf("skill %q guardada en %s", args.Name, skillPath)), nil
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

	// Obtener el catálogo con timeout vía contexto (5 segundos).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cat, err := skillsource.FetchCatalog(ctx, s.sourcing.CatalogURL)
	if err != nil {
		// Degradación graciosa: catálogo inaccesible → texto explicativo, no RpcError.
		return textResult(fmt.Sprintf(
			"El catálogo de skills no está disponible en este momento (%v). "+
				"Podés buscar skills manualmente en la documentación oficial del stack detectado "+
				"y guardarlas con musubi_save_skill.", err)), nil
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

	return jsonResult(candidatos)
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

	telemetryLogs, err := s.engine.GetUnresolvedTelemetryLogs()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al obtener telemetría: %v", err)
	}

	return jsonResult(map[string]interface{}{
		"active_skills":  activeSkills,
		"telemetry_logs": telemetryLogs,
	})
}
