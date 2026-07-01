package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// methods_sdd.go expone musubi_sdd: el FLUJO SDD GUIADO (O1). Es una capa fina sobre
// el motor DAG (toolWorkflow) que (1) genera el workflow canónico SDD a partir del
// nombre de un cambio —sin YAML—, (2) surface la plantilla y la directiva de cada
// fase, y (3) al cerrar una fase persiste su CONTRATO DE RESULTADO en memoria bajo
// sdd/<change>/<phase>. Eso es la fusión memoria↔orquestación: las fases siguientes
// recuperan los artefactos por referencia barata en vez de releer archivos.

func (s *McpServer) toolSDD(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Action          string   `json:"action"`
		Change          string   `json:"change"`
		Phase           string   `json:"phase"`
		Summary         string   `json:"summary"`
		Artifacts       []string `json:"artifacts"`
		Risks           []string `json:"risks"`
		NextRecommended string   `json:"next_recommended"`
		Status          string   `json:"status"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	change := strings.TrimSpace(args.Change)

	switch action := strings.TrimSpace(args.Action); action {
	case "start":
		if change == "" {
			return nil, rpcErrorf(codeInvalidParams, "start requiere 'change' (nombre del cambio)")
		}
		def := memory.SDDWorkflowDef(change)
		run, err := s.engine.StartWorkflowRun(memory.SDDRunID(change), def)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo iniciar el flujo SDD: %v", err)
		}
		ready, _ := s.engine.WorkflowReady(run.RunID)
		return jsonResult(s.sddView(change, run, ready, "Flujo SDD iniciado. Empezá por la fase activa."))

	case "next", "status":
		if change == "" {
			return nil, rpcErrorf(codeInvalidParams, "%s requiere 'change'", action)
		}
		run, ok, err := s.engine.WorkflowRunStatus(memory.SDDRunID(change))
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo leer el flujo SDD: %v", err)
		}
		if !ok {
			return nil, rpcErrorf(codeInvalidParams, "no hay un flujo SDD para %q; iniciá uno con action=start", change)
		}
		ready, err := s.engine.WorkflowReady(run.RunID)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return jsonResult(s.sddView(change, run, ready, ""))

	case "complete":
		if change == "" || strings.TrimSpace(args.Phase) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'change' y 'phase'")
		}
		if strings.TrimSpace(args.Summary) == "" {
			return nil, rpcErrorf(codeInvalidParams, "complete requiere 'summary' (el resultado de la fase)")
		}
		phase := strings.TrimSpace(args.Phase)
		runID := memory.SDDRunID(change)
		run, err := s.engine.CompleteWorkflowStep(runID, phase, args.Summary, args.Status)
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo cerrar la fase: %v", err)
		}

		// Handoff a memoria: persistir el contrato de resultado bajo sdd/<change>/<phase>.
		// Solo cuando la fase quedó 'done' (un 'failed' no produce artefacto válido).
		var memNote string
		if run.StepStatus[phase] == memory.StepDone {
			contract := memory.SDDContract{
				Summary:         args.Summary,
				Artifacts:       args.Artifacts,
				Risks:           args.Risks,
				NextRecommended: args.NextRecommended,
			}
			id, rerr := s.persistSDDArtifact(change, phase, contract)
			if rerr != nil {
				return nil, rerr
			}
			memNote = "Artefacto guardado en memoria (id: " + id + ")."
		}

		ready, _ := s.engine.WorkflowReady(runID)
		return jsonResult(s.sddView(change, run, ready, memNote))

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá start|next|complete|status)", action)
	}
}

// persistSDDArtifact guarda el contrato de una fase como observación con id y
// topic_key deterministas (sdd/<change>/<phase>), de modo que re-cerrar una fase
// haga UPSERT en vez de duplicar. Genera el embedding si hay proveedor configurado.
func (s *McpServer) persistSDDArtifact(change, phase string, c memory.SDDContract) (string, *RpcError) {
	id := memory.SDDTopicKey(change, phase)
	content := c.Memo(change, phase)

	var emb []float32
	if embedding.Enabled(s.embedder) {
		embCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		vec, err := s.embedder.Embed(embCtx, content)
		if err != nil {
			return "", rpcErrorf(codeInternalError, "error al generar embedding del artefacto SDD: %v", err)
		}
		emb = vec
	}
	if err := s.engine.SaveObservationWithImportance(id, id, content, 1.0, emb); err != nil {
		return "", rpcErrorf(codeInternalError, "error al guardar el artefacto SDD: %v", err)
	}
	return id, nil
}

// sddView arma la vista de respuesta de musubi_sdd: el estado de todas las fases en
// orden canónico, la(s) fase(s) lista(s), y —para la fase activa— su directiva y la
// ruta de su plantilla. note es un mensaje opcional para el agente.
func (s *McpServer) sddView(change string, run memory.WorkflowRun, ready []string, note string) map[string]interface{} {
	type phaseView struct {
		Phase  string `json:"phase"`
		Title  string `json:"title"`
		Status string `json:"status"`
		Result string `json:"result,omitempty"`
	}
	phases := make([]phaseView, 0, len(memory.SDDPhases))
	for _, p := range memory.SDDPhases {
		st := run.StepStatus[p]
		if st == "" {
			st = memory.StepPending
		}
		phases = append(phases, phaseView{Phase: p, Title: sddPhaseTitleOf(run, p), Status: st, Result: run.StepResults[p]})
	}

	view := map[string]interface{}{
		"change": change,
		"run_id": run.RunID,
		"status": run.Status,
		"phases": phases,
		"ready":  ready,
		"done":   run.Status == memory.RunDone,
	}
	if note != "" {
		view["note"] = note
	}
	// Fase activa = primera lista; surface su guía y plantilla.
	if len(ready) > 0 {
		active := ready[0]
		view["active"] = active
		view["directive"] = memory.SDDPhaseDirective(active, change)
		if role := memory.SDDRole(active); role != "" {
			view["role"] = role
		}
		if tpl, ok := memory.SDDTemplatePath(active); ok {
			view["template"] = tpl
		}
	} else if run.Status == memory.RunDone {
		view["active"] = ""
		view["directive"] = "Flujo SDD completo. Todas las fases están cerradas."
	}
	return view
}

// sddPhaseTitleOf devuelve el título del step en la definición del run (o el id si
// la definición no lo trae, por compat).
func sddPhaseTitleOf(run memory.WorkflowRun, phase string) string {
	for _, st := range run.Def.Steps {
		if st.ID == phase {
			if st.Title != "" {
				return st.Title
			}
			return phase
		}
	}
	return phase
}
