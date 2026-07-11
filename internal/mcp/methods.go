package mcp

import (
	"context"
	"encoding/json"
	"errors"
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
	"musubi/internal/redact"
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
	// Marshal COMPACTO (sin indentación): la respuesta la parsea el cliente MCP, no la
	// lee un humano; la indentación era ~28% de whitespace puro en cada payload JSON.
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al serializar resultado: %v", err)
	}
	return textResult(string(jsonBytes)), nil
}

// toolSyncPull sirve un PULL entrante del cerebro híbrido (C5.3b): devuelve un lote de la memoria
// 'shared' del proyecto de la CREDENCIAL (aislamiento T17-19) con rowid > after_rowid, para que un
// cliente en team mode la baje e ingiera localmente y su recall la surfacee sola (sin red en el hot
// path). Read-only. next_cursor = el mayor rowid del lote (o after_rowid si vino vacío): el cliente
// lo guarda para pedir la página siguiente.
func (s *McpServer) toolSyncPull(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		AfterRowID int64 `json:"after_rowid"`
		Limit      int   `json:"limit"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	items, err := s.engine.ListSharedForPull(s.scopedCtx(ctx), args.AfterRowID, args.Limit)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al listar shared para pull: %v", err)
	}
	next := args.AfterRowID
	for _, o := range items {
		if o.RowID > next {
			next = o.RowID
		}
	}
	if items == nil {
		items = []memory.SharedObs{}
	}
	return jsonResult(map[string]interface{}{"items": items, "next_cursor": next})
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
	readOnly := s.toolReadOnly[callReq.Name]
	// Autorización por rol (Track 16 F1 16.1c): en modo serve hay un principal en el ctx
	// (lo autenticó el transporte HTTP). Un reader solo puede tools de lectura. En stdio
	// local no hay principal ⇒ acceso pleno (confianza local). Se chequea ANTES de tomar
	// el lock para no serializar denegaciones.
	p := principalFrom(ctx)
	if !p.canCall(readOnly) {
		if s.metrics != nil {
			s.metrics.authzDenied.Add(1) // rechazo por rol, visible en /metrics (T17.5)
		}
		return nil, rpcErrorf(codeUnauthorized, "principal %q (rol %s) no autorizado para %q", p.Name, p.Role, callReq.Name)
	}
	// Cuota de uso por-principal (Track 16 F3.2): tras autorizar, contar la llamada contra la
	// ventana del principal. Solo cuando hay principal (serve); en stdio local (p nil) no
	// aplica. Se chequea ANTES del lock para no serializar los rechazos.
	if p != nil && !s.quota.allow(p.Name, time.Now()) {
		if s.metrics != nil {
			s.metrics.quotaExceeded.Add(1) // rechazo por cuota, visible en /metrics (T17.5)
		}
		return nil, rpcErrorf(codeQuotaExceeded, "cuota excedida para el principal %q (máx %d llamadas/min); reintentá en unos segundos", p.Name, s.quota.max)
	}
	// Las tools de solo-lectura corren concurrentes entre sí (RLock); las que mutan
	// toman el lock exclusivo (serializadas, sin lost-updates de read-modify-write).
	if readOnly {
		s.dispatchMu.RLock()
		defer s.dispatchMu.RUnlock()
	} else {
		s.dispatchMu.Lock()
		defer s.dispatchMu.Unlock()
	}
	// Métrica de latencia/resultado de la tool (Track 16 F3.1), expuesta en /metrics.
	start := time.Now()
	result, rpcErr := handler(ctx, callReq.Arguments)
	if s.metrics != nil {
		s.metrics.recordTool(callReq.Name, time.Since(start), rpcErr == nil)
	}
	return result, rpcErr
}

func (s *McpServer) toolSaveObservation(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID         string  `json:"id"`
		TopicKey   string  `json:"topic_key"`
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
		MemType    string  `json:"mem_type"`
		Scope      string  `json:"scope"`
		ProjectID  string  `json:"project_id"` // origen (ingest del central); "" ⇒ project_id del engine
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
	// scope de la memoria híbrida: "" (default local), "local" o "shared". Cualquier otro
	// valor es un error de parámetro (no se coacciona silenciosamente).
	if !memory.ValidScopeParam(args.Scope) {
		return nil, rpcErrorf(codeInvalidParams, "scope inválido %q: usá 'local' o 'shared'", args.Scope)
	}
	scope := args.Scope
	if scope == "" {
		// C5.2: sin scope explícito, en team mode default a 'shared' (fluye al central); si no,
		// 'local' (histórico). Un scope explícito ('local'/'shared') se respeta como escape hatch.
		scope = s.defaultScope()
	}
	// Redacción forzada server-side (16.1d): en infra compartida (el central) TODO ingest se
	// trata como shared, así se redactan los secretos aunque el cliente declare scope=local.
	// Cierra el hueco por el que un secreto crudo entraba al pozo compartido.
	if s.forceRedact {
		scope = memory.ScopeShared
	}
	importance := args.Importance
	if importance <= 0 {
		importance = 1.0
	}

	// Atribución de escritura por CREDENCIAL (Track 17 — cierra el write-poisoning cross-tenant,
	// simétrico al aislamiento de lectura de T17.1a): un writer/reader acotado NO puede atribuir
	// la observación a otro proyecto (ni dejarla sin atribuir, visible para todos) — su origen lo
	// fija su credencial, se ignora el project_id que declare el cliente. El origen explícito de
	// los args solo se respeta para admin/legacy (ingest del central), para quien se diseñó *From.
	origin := args.ProjectID
	if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
		origin = p.ProjectID
	}
	// Atribución por PERSONA (C5.1): author se deriva de la credencial (nunca del cliente), para
	// que la memoria compartida de un equipo registre QUIÉN aportó cada cosa. Sellado server-side.
	author := authorFrom(principalFrom(ctx))

	// Redacción ANTES del embedding (Track 17 T17.2): con forceRedact el vector debe derivarse del
	// contenido YA redactado (antes se embebía el crudo, dejando un vector at-rest derivado del
	// secreto en infra compartida). topic_key también se redacta (antes cruzaba crudo al central en
	// su propia tool guardada). Idempotente con la redacción de la capa de memoria (scope shared).
	content := s.redactIfForced(args.Content)
	topicKey := s.redactIfForced(args.TopicKey)

	var emb []float32
	if embedding.Enabled(s.embedder) {
		embCtx, embCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer embCancel()
		vec, err := s.embedder.Embed(embCtx, content)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al generar embedding: %v", err)
		}
		emb = vec
	}

	// Sin id explícito: deduplicar por contenido y autogenerar UUID. El origen se derivó de la
	// credencial arriba (Track 17); admin/legacy conserva el project_id declarado por el caller.
	if strings.TrimSpace(args.ID) == "" {
		id, deduped, err := s.engine.SaveObservationDedupedTypedFrom(origin, author, topicKey, content, importance, args.MemType, scope, emb)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
		}
		if deduped {
			return textResult("Observación ya existente, no se duplicó (id: " + id + ")."), nil
		}
		return textResult("Observación guardada con éxito (id: " + id + ")." + s.detectAndSurface(id)), nil
	}

	// Con id explícito: upsert por id.
	if err := s.engine.SaveObservationTypedFrom(origin, author, args.ID, topicKey, content, importance, args.MemType, scope, emb); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
	}
	return textResult("Observación guardada con éxito (id: " + args.ID + ")." + s.detectAndSurface(args.ID)), nil
}

// toolPromote marca una observación como 'shared' (memoria híbrida local+central). Muta,
// así que va bajo el lock exclusivo del dispatch (readOnly=false). Idempotente: promover
// una ya compartida es OK; un id inexistente devuelve un error claro.
func (s *McpServer) toolPromote(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}
	if err := s.engine.PromoteObservation(args.ID); err != nil {
		if errors.Is(err, memory.ErrObservationNotFound) {
			return nil, rpcErrorf(codeInvalidParams, "no existe una observación con id %q", args.ID)
		}
		return nil, rpcErrorf(codeInternalError, "error al promover observación: %v", err)
	}
	return textResult("Observación promovida a 'shared' (id: " + args.ID + "); ahora es candidata a la memoria central."), nil
}

// toolSyncStatus devuelve la salud del sync saliente del cerebro híbrido (F2): observaciones
// shared pendientes/enviadas/en dead-letter, antigüedad de la más vieja pendiente y último
// error. Read-only, sin params.
func (s *McpServer) toolSyncStatus(_ json.RawMessage) (interface{}, *RpcError) {
	h, err := s.engine.OutboxHealth()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer el estado del sync: %v", err)
	}
	body, _ := json.Marshal(h)
	summary := fmt.Sprintf("Sync saliente — pendientes: %d, enviadas: %d, dead-letter: %d", h.Pending, h.Sent, h.Dead)
	if h.Pending > 0 && h.OldestPendingAgeSec > 0 {
		summary += fmt.Sprintf(" (la más vieja pendiente hace %ds)", h.OldestPendingAgeSec)
	}
	if h.Dead > 0 {
		summary += fmt.Sprintf("; %d en dead-letter — reintentá con musubi_sync_requeue", h.Dead)
	}
	if h.LastError != "" {
		summary += "\nÚltimo error: " + h.LastError
	}
	return textResult(summary + "\n" + string(body)), nil
}

// toolSyncRequeue devuelve las observaciones en dead-letter a la cola de envío (F2). Muta
// (readOnly=false). Útil tras un corte del central o de la VPN. Sin params.
func (s *McpServer) toolSyncRequeue(_ json.RawMessage) (interface{}, *RpcError) {
	n, err := s.engine.RequeueDeadOutbox()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al re-encolar el dead-letter del sync: %v", err)
	}
	if n == 0 {
		return textResult("No había observaciones en dead-letter; nada para re-encolar."), nil
	}
	return textResult(fmt.Sprintf("%d observación(es) re-encolada(s) para reenvío al cerebro central.", n)), nil
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
		Bid          float64               `json:"bid"`
		Note         string                `json:"note"`
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

	case "bid":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "bid requiere 'id' (la unidad)")
		}
		if strings.TrimSpace(args.Agent) == "" {
			return nil, rpcErrorf(codeInvalidParams, "bid requiere 'agent'")
		}
		if err := s.engine.BidWorkUnit(args.ID, args.Agent, args.Bid, args.Note); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo ofertar: %v", err)
		}
		return textResult("Oferta registrada."), nil

	case "award":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "award requiere 'id' (la unidad)")
		}
		u, winner, ok, err := s.engine.AwardWorkUnit(args.ID, s.multiagent.LeaseTTLSeconds)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo adjudicar: %v", err)
		}
		if !ok {
			return jsonResult(map[string]interface{}{"awarded": false,
				"note": "sin ofertas o la unidad ya no está open"})
		}
		// El ganador debe renovar el lease con heartbeat (id, agent, fencing_token) y cerrar
		// con complete, igual que un claim normal.
		return jsonResult(map[string]interface{}{"awarded": true, "winner": winner, "unit": u})

	case "bids":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "bids requiere 'id' (la unidad)")
		}
		bids, err := s.engine.WorkUnitBids(args.ID)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudieron leer las ofertas: %v", err)
		}
		return jsonResult(map[string]interface{}{"unit": args.ID, "bids": bids})

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
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá plan|claim|heartbeat|complete|status|savings|clear|bid|award|bids)", action)
	}
}

// toolDebate maneja el subsistema de debate multi-agente (Society of Minds) model-free:
// rondas de posturas atribuidas + tally determinista. Musubi estructura y cuenta; los agentes
// (LLM) producen posturas, críticas y votos.
func (s *McpServer) toolDebate(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action string `json:"action"`
		ID     string `json:"id"`
		Topic  string `json:"topic"`
		Rounds int    `json:"rounds"`
		Quorum int    `json:"quorum"`
		Agent  string `json:"agent"`
		Stance string `json:"stance"`
		Choice string `json:"choice"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	switch action := strings.TrimSpace(args.Action); action {
	case "open":
		d, err := s.engine.OpenDebate(args.Topic, args.Rounds, args.Quorum)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo abrir el debate: %v", err)
		}
		return jsonResult(map[string]interface{}{"debate": d,
			"note": "postea las posturas de la ronda 1 con action=post (id, agent, stance); tras N posturas, action=advance para pasar a la siguiente ronda con las posturas previas como material de crítica"})

	case "post":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "post requiere 'id' (el debate)")
		}
		if err := s.engine.PostPosture(args.ID, args.Agent, args.Stance); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo postear: %v", err)
		}
		return textResult("Postura registrada."), nil

	case "advance":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "advance requiere 'id' (el debate)")
		}
		round, prev, err := s.engine.AdvanceDebate(args.ID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo avanzar: %v", err)
		}
		return jsonResult(map[string]interface{}{"round": round, "previous_postures": prev,
			"note": "pasá 'previous_postures' a los agentes como material de crítica cruzada; cuando terminen las rondas, recogé los votos con action=vote y cerrá con action=tally"})

	case "vote":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "vote requiere 'id' (el debate)")
		}
		if err := s.engine.CastVote(args.ID, args.Agent, args.Choice); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo votar: %v", err)
		}
		return textResult("Voto registrado."), nil

	case "tally":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "tally requiere 'id' (el debate)")
		}
		res, d, err := s.engine.TallyDebate(args.ID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo hacer el recuento: %v", err)
		}
		return jsonResult(map[string]interface{}{"tally": res, "debate": d})

	case "status":
		if strings.TrimSpace(args.ID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "status requiere 'id' (el debate)")
		}
		d, postures, votes, err := s.engine.DebateStatus(args.ID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo leer el debate: %v", err)
		}
		return jsonResult(map[string]interface{}{"debate": d, "postures": postures, "votes": votes})

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá open|post|advance|vote|tally|status)", action)
	}
}

// toolWorkflow es la interfaz MCP del motor de orquestación DAG (model-free).
// Musubi NO ejecuta los steps: define el grafo, persiste el estado y devuelve los
// steps listos; el agente ejecuta y reporta con 'complete'. El estado es resumible.
// workflowRunLean es la vista del run para las acciones INCREMENTALES: espeja
// memory.WorkflowRun pero OMITE la definición (Def), que es inmutable tras start y el
// caller ya recibió. En un run de varios pasos, el DAG completo (títulos + directivas
// verify/await/compensate) es el mayor bloque repetido del payload. El snapshot completo
// —con definition— se conserva en start/status/resume. Los mismos tags/omitempty que el
// original para no divergir la forma de los campos que sí se envían.
type workflowRunLean struct {
	RunID       string            `json:"run_id"`
	WorkflowID  string            `json:"workflow_id"`
	Status      string            `json:"status"`
	StepStatus  map[string]string `json:"step_status"`
	StepResults map[string]string `json:"step_results"`
	StepIters   map[string]int    `json:"step_iters,omitempty"`
}

// leanRun proyecta un WorkflowRun a su vista sin definición (delta de las respuestas
// incrementales de musubi_workflow). No toca el estado persistido: solo la serialización.
func leanRun(r memory.WorkflowRun) workflowRunLean {
	return workflowRunLean{
		RunID:       r.RunID,
		WorkflowID:  r.WorkflowID,
		Status:      r.Status,
		StepStatus:  r.StepStatus,
		StepResults: r.StepResults,
		StepIters:   r.StepIters,
	}
}

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
		Input          string `json:"input"`
		Verdict        string `json:"verdict"`
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
		waiting, _ := s.engine.WorkflowAwaiting(args.RunID)
		return jsonResult(map[string]interface{}{"run": run, "ready": ready, "waiting": waiting})

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
		waiting, _ := s.engine.WorkflowAwaiting(args.RunID)
		return jsonResult(map[string]interface{}{"ready": ready, "waiting": waiting})

	case "complete":
		if strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Step) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'run_id' y 'step'")
		}
		run, err := s.engine.CompleteWorkflowStep(args.RunID, args.Step, args.Result, args.Status, args.IdempotencyKey)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		waiting, _ := s.engine.WorkflowAwaiting(args.RunID)
		return jsonResult(map[string]interface{}{"run": leanRun(run), "ready": ready, "waiting": waiting})

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

	case "rollback":
		// Inicia la saga: devuelve el plan de compensación LIFO (steps a deshacer, en orden
		// inverso). El agente ejecuta cada compensación y reporta con action=compensated.
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "rollback requiere 'run_id'")
		}
		plan, run, err := s.engine.WorkflowRollback(args.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(map[string]interface{}{"run": leanRun(run), "pending": plan})

	case "abort":
		// Aborta explícitamente un run atascado o no deseado: lo marca 'aborted' y deja de
		// despachar steps. La razón (opcional) va en 'result'. Idempotente; falla si el run ya
		// concluyó con éxito (done/compensated).
		if strings.TrimSpace(args.RunID) == "" {
			return nil, rpcErrorf(codeInvalidParams, "abort requiere 'run_id'")
		}
		run, err := s.engine.AbortWorkflowRun(args.RunID, args.Result)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(map[string]interface{}{"run": leanRun(run)})

	case "compensated":
		// El agente reporta que ejecutó la compensación de un step; devuelve el plan restante.
		if strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Step) == "" {
			return nil, rpcErrorf(codeInvalidParams, "compensated requiere 'run_id' y 'step'")
		}
		plan, run, err := s.engine.CompleteCompensation(args.RunID, args.Step)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(map[string]interface{}{"run": leanRun(run), "pending": plan})

	case "provide":
		// HITL: resuelve un gate humano (step en waiting_input). input = la decisión/dato;
		// status = done (aprobado) | failed (rechazado). Reanuda el run.
		if strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Step) == "" {
			return nil, rpcErrorf(codeInvalidParams, "provide requiere 'run_id' y 'step'")
		}
		run, err := s.engine.ProvideWorkflowInput(args.RunID, args.Step, args.Input, args.Status)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		waiting, _ := s.engine.WorkflowAwaiting(args.RunID)
		return jsonResult(map[string]interface{}{"run": leanRun(run), "ready": ready, "waiting": waiting})

	case "verify":
		// Gate de verificación (Reflexion): resuelve un step en `verifying`. verdict=pass
		// lo marca done; verdict=fail registra la reflexión (result) y reabre para otro
		// intento, o falla el gate al agotar el presupuesto.
		if strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Step) == "" {
			return nil, rpcErrorf(codeInvalidParams, "verify requiere 'run_id' y 'step'")
		}
		verdict := strings.TrimSpace(args.Verdict)
		if verdict != "pass" && verdict != "fail" {
			return nil, rpcErrorf(codeInvalidParams, "verify requiere 'verdict' = pass | fail")
		}
		run, reflections, err := s.engine.VerifyWorkflowStep(args.RunID, args.Step, verdict == "pass", args.Result)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		ready, _ := s.engine.WorkflowReady(args.RunID)
		return jsonResult(map[string]interface{}{"run": leanRun(run), "ready": ready, "reflections": reflections})

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
		waiting, _ := s.engine.WorkflowAwaiting(args.RunID)
		return jsonResult(map[string]interface{}{"run": run, "ready": ready, "waiting": waiting})

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá start|next|complete|status|resume|validate|list|journal|otel|rollback|abort|compensated|provide|verify)", action)
	}
}

// toolConflicts lista las relaciones pendientes de veredicto.
func (s *McpServer) toolConflicts(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	// Aislamiento por proyecto (Track 17): solo los conflictos cuya observación de origen es del
	// proyecto de la credencial; stdio local / admin ⇒ federado.
	rels, err := s.engine.PendingObsRelationsCtx(s.scopedCtx(ctx))
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
		TokenBudget:     s.memory.RecallTokenBudget,
		CandidatePool:   s.memory.CandidatePool,
		GistMaxTokens:   s.memory.GistMaxTokens,
		GraphCentrality: s.memory.RecallGraphCentrality,
		Cooccurrence:    s.memory.RecallCooccurrence,
		Stemming:        s.memory.RecallStemming,
	}
	// Enforcement del aislamiento por proyecto (Track 16 F1 16.1c-3): el scope del recall se
	// DERIVA del principal autenticado (su project_id sale de la credencial, no se auto-declara).
	opts.ProjectScope, opts.Federate = recallScopeFor(principalFrom(ctx))
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

func (s *McpServer) toolSaveFact(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Redacción de TODO ingest al central (Track 17 T17.2): en infra compartida, el triple no
	// puede llevar un secreto crudo al pozo compartido (recuperable por recall_facts).
	subject := s.redactIfForced(args.Subject)
	predicate := s.redactIfForced(args.Predicate)
	object := s.redactIfForced(args.Object)

	// Atribución por credencial (Track 17): el hecho se guarda para el proyecto del principal, no
	// en un espacio global compartido; la invalidación por cardinalidad queda acotada a ese
	// proyecto (no cierra hechos vivos de otros). admin/stdio ⇒ '' (espacio federado histórico).
	origin := ""
	if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
		origin = p.ProjectID
	}

	res, err := s.engine.SaveFactFrom(origin, subject, predicate, object, args.ValidFrom, s.graph.SingleValuedPredicates)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar hecho: %v", err)
	}
	msg := fmt.Sprintf("Hecho guardado: %s %s %s.", subject, predicate, object)
	if !res.Created {
		msg = fmt.Sprintf("Hecho re-afirmado (ya existía): %s %s %s.", subject, predicate, object)
	}
	if res.Invalidated > 0 {
		// Cardinalidad: el predicado es funcional y este hecho reemplazó a otro(s).
		msg += fmt.Sprintf(" Invalidó %d hecho(s) previo(s) contradictorio(s) (predicado single-valued).", res.Invalidated)
	}
	return textResult(msg), nil
}

func (s *McpServer) toolRecallFacts(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Entity  string `json:"entity"`
		MaxHops int    `json:"max_hops"`
		AsOf    string `json:"as_of"`
		Rank    string `json:"rank"`
		To      string `json:"to"`
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

	// Aislamiento por proyecto (Track 17): el traversal se acota al proyecto de la credencial.
	scoped := s.scopedCtx(ctx)

	// Con 'to' seteado: camino más corto entity→to. Sin él: vecindad (BFS/pagerank).
	if strings.TrimSpace(args.To) != "" {
		res, err := s.engine.FactPathCtx(scoped, args.Entity, args.To, maxHops, args.AsOf)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al calcular el camino: %v", err)
		}
		return jsonResult(res)
	}

	res, err := s.engine.RecallFactsCtx(scoped, args.Entity, maxHops, s.graph.MaxFacts, args.AsOf, args.Rank)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al recuperar hechos: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolEntityContext(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Aislamiento por proyecto (Track 17): hechos y observaciones se acotan al proyecto de la credencial.
	res, err := s.engine.EntityContextCtx(s.scopedCtx(ctx), args.Entity, maxHops, s.graph.MaxFacts, s.graph.MaxObservations)
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

func (s *McpServer) toolMemoryExpand(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Aislamiento por proyecto (Track 17): la hidratación por id era una fuga total (leer el
	// contenido crudo de CUALQUIER proyecto enumerando ids). Se acota a la credencial.
	res, used, err := s.engine.GetObservationsBudgetCtx(s.scopedCtx(ctx), args.IDs, args.MaxTokens)
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

func (s *McpServer) toolSaveCode(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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
	// Redacción de TODO ingest al central (Track 17 T17.2): gist y symbols no pueden llevar un
	// secreto crudo al pozo compartido (recuperable por recall_code). El path es la clave, no se
	// redacta. En loopback local queda crudo.
	gist := s.redactIfForced(args.Gist)
	symbols = s.redactIfForced(symbols)
	cm := memory.CodeMemory{
		Path:        key,
		Gist:        gist,
		Symbols:     symbols,
		Fingerprint: fp,
		Tokens:      memory.EstimateTokens(gist),
	}
	// Atribución por credencial (Track 17): la memoria de código se guarda para el proyecto del
	// principal, no en un espacio global compartido. admin/stdio ⇒ project_id del engine.
	origin := ""
	if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
		origin = p.ProjectID
	}
	if err := s.engine.SaveCodeMemoryFrom(origin, cm); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar memoria de código: %v", err)
	}
	return jsonResult(map[string]interface{}{"ok": true, "path": cm.Path, "tokens": cm.Tokens})
}

func (s *McpServer) toolRecallCode(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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
	// Aislamiento por proyecto (Track 17): el gist se lee del proyecto de la credencial.
	cm, ok, err := s.engine.GetCodeMemoryCtx(s.scopedCtx(ctx), key)
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

// searchHit es un resultado de búsqueda en forma gist-first: el titular extractivo en
// lugar del contenido completo, para no gastar el presupuesto de tokens del caller. El
// contenido full se hidrata bajo demanda por id (musubi_recall / musubi_memory_expand).
type searchHit struct {
	ID         string  `json:"id"`
	TopicKey   string  `json:"topic_key,omitempty"`
	Gist       string  `json:"gist"`
	Similarity float32 `json:"similarity,omitempty"`  // solo búsqueda semántica
	FullTokens int     `json:"full_tokens,omitempty"` // costo de hidratar el contenido completo
}

// searchSource es la vista mínima de un hit (semántico o keyword) que necesita el
// empaquetado gist-first: identidad, contenido a resumir y, opcional, la similitud.
type searchSource struct {
	id       string
	topicKey string
	content  string
	sim      float32
}

// searchGistBudget acota el TAMAÑO total (en tokens) del payload de búsqueda gist-first.
// limit ya acota la CANTIDAD de hits; este es el tope de tamaño para que unos pocos gists
// grandes no inflen la respuesta. No se expone como parámetro (mantiene el schema liviano).
const searchGistBudget = 2000

// toSearchHits convierte los hits crudos en resultados gist-first acotados por presupuesto,
// con el mismo criterio que packByBudget del recall: se garantiza el top-1 (aunque exceda) y
// a partir de ahí se corta al llegar al budget. Determinista, sin LLM.
func toSearchHits(sources []searchSource, gistMax, budget int) []searchHit {
	if gistMax <= 0 {
		gistMax = 24
	}
	hits := make([]searchHit, 0, len(sources))
	used := 0
	for _, src := range sources {
		gist := memory.Gist(src.content, gistMax)
		cost := memory.EstimateTokens(gist)
		if len(hits) > 0 && used+cost > budget {
			continue // no entra; el siguiente puede ser más chico
		}
		hits = append(hits, searchHit{
			ID:         src.id,
			TopicKey:   src.topicKey,
			Gist:       gist,
			Similarity: src.sim,
			FullTokens: memory.EstimateTokens(src.content),
		})
		used += cost
		if used >= budget {
			break
		}
	}
	return hits
}

// scopedCtx adjunta al contexto el ProjectScope derivado de la CREDENCIAL (recallScopeFor),
// para que las lecturas del engine se acoten al proyecto del principal (Track 17 — aislamiento
// multi-tenant). En stdio local / admin / legacy ⇒ federado (sin filtro): idéntico criterio que
// el scope del recall, ahora extendido a las demás superficies de lectura.
func (s *McpServer) scopedCtx(ctx context.Context) context.Context {
	ps, fed := recallScopeFor(principalFrom(ctx))
	return memory.WithProjectScope(ctx, memory.ProjectScope{ProjectID: ps, Federate: fed})
}

// redactIfForced redacta text cuando el server FUERZA redacción (infra compartida: un bind
// no-loopback / el cerebro central). En loopback local queda crudo (el dev necesita el texto
// real). Es la GUARDA ÚNICA de "redactar TODO ingest al central" (Track 17 T17.2): antes solo
// cubría save_observation, dejando save_fact y save_code escribiendo secretos crudos al pozo
// compartido. Idempotente (redactar algo ya redactado es no-op).
func (s *McpServer) redactIfForced(text string) string {
	if !s.forceRedact {
		return text
	}
	clean, _ := redact.Redact(text)
	return clean
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

	results, err := s.engine.SearchObservations(s.scopedCtx(ctx), vec, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda semántica: %v", err)
	}
	sources := make([]searchSource, len(results))
	for i, r := range results {
		sources[i] = searchSource{id: r.ID, topicKey: r.TopicKey, content: r.Content, sim: r.Similarity}
	}
	return jsonResult(toSearchHits(sources, s.memory.GistMaxTokens, searchGistBudget))
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

	results, err := s.engine.SearchObservationsFTS(s.scopedCtx(ctx), args.QueryText, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda por palabra clave: %v", err)
	}
	sources := make([]searchSource, len(results))
	for i, r := range results {
		sources[i] = searchSource{id: r.ID, topicKey: r.TopicKey, content: r.Content}
	}
	return jsonResult(toSearchHits(sources, s.memory.GistMaxTokens, searchGistBudget))
}

func (s *McpServer) toolLogError(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Track 18: atribuir el log a la credencial (no al cliente) y redactar el ingest a infra
	// compartida. error_message/suggested_patch pueden traer secretos (tokens, rutas, stack); en
	// un bind no-loopback se redactan ANTES de persistir, como el resto del ingest (T17.2).
	origin := ""
	if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
		origin = p.ProjectID
	}
	msg := s.redactIfForced(args.ErrorMessage)
	patch := s.redactIfForced(args.SuggestedPatch)
	if err := s.engine.SaveTelemetryLogFrom(origin, args.FilePath, msg, patch); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar log de telemetría: %v", err)
	}
	return textResult("Log de telemetría guardado con éxito."), nil
}

func (s *McpServer) toolResolveTelemetry(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if args.ID <= 0 {
		return nil, rpcErrorf(codeInvalidParams, "id debe ser un entero positivo")
	}

	// Track 18: acotar la resolución/lectura al proyecto de la credencial. Un tenant no puede
	// resolver ni leer el log crudo de otro (un id de otro proyecto ⇒ found=false, igual que
	// inexistente, sin filtrar su existencia).
	log, found, err := s.engine.ResolveTelemetryLogAndGetCtx(s.scopedCtx(ctx), args.ID)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al resolver telemetría: %v", err)
	}
	if !found {
		return nil, rpcErrorf(codeInternalError, "error al resolver telemetría: no existe log de telemetría con id %d", args.ID)
	}

	// C4: capturar el par error→fix como memoria (best-effort; un fallo NO rompe el resolve). Solo
	// si hay un parche registrado (anti-ruido: un error sin fix no es una memoria útil). Track 18: el
	// contenido se redacta ANTES de derivar embedding/gist (va al pozo compartido) y se ATRIBUYE al
	// proyecto de la credencial. C5.2: usa el scope default del proyecto (local, o shared en team
	// mode → fluye al central; la redacción del borde a shared corre igual). Si la semántica está
	// encendida, se guarda CON embedding (16.2e) homogéneo con el recall.
	if strings.TrimSpace(log.SuggestedPatch) != "" {
		content := fmt.Sprintf("Error en %s: %s\n\nArreglado con: %s", log.FilePath, log.ErrorMessage, log.SuggestedPatch)
		content = s.redactIfForced(content)
		origin := ""
		if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
			origin = p.ProjectID
		}
		author := authorFrom(principalFrom(ctx))
		_, _, _ = s.engine.SaveObservationDedupedTypedFrom(origin, author, "error-fix", content, 0.7, "procedural", s.defaultScope(), s.embedIfEnabled(content))
	}
	return textResult("Log de telemetría marcado como resuelto."), nil
}

// embedIfEnabled genera el vector de text con el embedder activo, o nil si la semántica está
// apagada o el embedding falla. Best-effort: la captura automática (C4) no debe romperse por un
// embed. El engine estampa la procedencia (F2.2) que fijó el entrypoint, homogénea con el recall.
func (s *McpServer) embedIfEnabled(text string) []float32 {
	if !embedding.Enabled(s.embedder) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	v, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return nil
	}
	return v
}

// toolInsights devuelve el resumen de observabilidad activa (Track 6 / T6.4): tamaño de la
// memoria, hotspots de errores no resueltos, decisiones de skills y salud del ciclo. Read-only.
func (s *McpServer) toolInsights(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	// Aislamiento por proyecto (Track 17, parcial): los counts de observations se acotan al
	// proyecto de la credencial; hotspots/decisiones siguen federados (sus tablas no tienen project_id).
	rep, err := s.engine.InsightsCtx(s.scopedCtx(ctx))
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
func (s *McpServer) toolSearchSkills(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Feedback de decisiones (Track 6 / T6.1): Musubi no re-propone las skills que el usuario ya
	// rechazó. Track 19: ACOTADO al proyecto de la credencial (GetSkillDecisionsCtx) — antes leía
	// federado, así que un 'rejected' de un tenant excluía candidatos de otro (behavior-bleed).
	// Best-effort: si la lectura falla, se devuelve sin filtrar.
	if decisions, derr := s.engine.GetSkillDecisionsCtx(s.scopedCtx(ctx)); derr == nil {
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
func (s *McpServer) toolLogSkillDecision(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Track 18: atribuir la decisión al proyecto de la credencial (no al cliente) para que no se
	// cuente en los insights de otro tenant.
	origin := ""
	if p := principalFrom(ctx); p != nil && p.Role != RoleAdmin {
		origin = p.ProjectID
	}
	if err := s.engine.SaveSkillDecisionFrom(origin, args.SkillID, args.Name, args.Decision, args.Reason); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar decisión de skill: %v", err)
	}
	return textResult(fmt.Sprintf("Decisión '%s' para skill '%s' registrada con éxito.", args.Decision, args.SkillID)), nil
}

func (s *McpServer) toolResolveSkills(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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

	// Telemetría RELEVANTE (Track 6 / T6.2): solo los errores no resueltos de los archivos que el
	// agente está tocando. Track 19: ACOTADA al proyecto de la credencial — antes corría sin scope
	// y devolvía file_path+error_message+suggested_patch de otros tenants por colisión de basename.
	telemetryLogs, err := s.engine.GetUnresolvedTelemetryLogsForFilesCtx(s.scopedCtx(ctx), args.ModifiedFiles)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al obtener telemetría: %v", err)
	}

	return jsonResult(map[string]interface{}{
		"active_skills":  activeSkills,
		"telemetry_logs": telemetryLogs,
	})
}
