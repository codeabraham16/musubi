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
		var run memory.WorkflowRun
		var ready []string
		var err error
		s.withWriteLock(func() {
			run, err = s.engine.StartWorkflowRun(memory.SDDRunID(change), def)
			if err != nil {
				return
			}
			ready, _ = s.engine.WorkflowReady(run.RunID)
		})
		if err != nil {
			return nil, rpcErrorf(codeInvalidParams, "no se pudo iniciar el flujo SDD: %v", err)
		}
		return jsonResult(s.sddView(change, run, ready, "Flujo SDD iniciado. Empezá por la fase activa."))

	case "next", "status":
		if change == "" {
			return nil, rpcErrorf(codeInvalidParams, "%s requiere 'change'", action)
		}
		var run memory.WorkflowRun
		var ok bool
		var ready []string
		var err, errReady error
		// withWriteLock y no withReadLock aunque esto sólo lea: la tool no es `readOnly`, así que
		// el despachador le tomaba el candado EXCLUSIVO. Conservarlo acá deja el comportamiento
		// idéntico y cambia sólo cuánto dura — que es lo único que este arreglo viene a cambiar.
		s.withWriteLock(func() {
			run, ok, err = s.engine.WorkflowRunStatus(memory.SDDRunID(change))
			if err != nil || !ok {
				return
			}
			ready, errReady = s.engine.WorkflowReady(run.RunID)
		})
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo leer el flujo SDD: %v", err)
		}
		if !ok {
			return nil, rpcErrorf(codeInvalidParams, "no hay un flujo SDD para %q; iniciá uno con action=start", change)
		}
		if errReady != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", errReady)
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

		// Atomicidad step↔artefacto (auditoría #23): se persiste el artefacto ANTES de marcar la
		// fase 'done', no después. El orden inverso dejaba —si el guardado del artefacto fallaba— la
		// fase 'done' SIN artefacto, rompiendo en silencio el contrato "fase done ⟹ artefacto existe"
		// del que dependen las fases siguientes (que lo recuperan por referencia). Con este orden, un
		// fallo del artefacto deja la fase SIN cerrar → el reintento es limpio; y si el complete fallara
		// tras persistir, el artefacto tiene id determinista (SDDTopicKey) y el reintento re-UPSERTa sin
		// duplicar. Las fases SDD son una cadena lineal sin gate verify/repeat, así que status=done
		// siempre resulta en StepDone: persistir cuando el intent NO es 'failed' equivale exactamente a
		// la semántica previa "solo cuando quedó done".
		// EL EMBED VA PRIMERO Y SIN CANDADO, Y ACÁ ESO NO ES UNA PREFERENCIA: ES LO QUE SALVA LA
		// ATOMICIDAD QUE EL COMENTARIO DE ARRIBA EXIGE.
		//
		// El corte obvio —un tramo de candado para el artefacto y otro para el paso— dejaría que
		// otro escritor se meta en el medio, y justo acá hay un invariante documentado («fase done
		// ⟹ artefacto existe») que depende del orden de las dos escrituras. Pero el CONTENIDO del
		// artefacto se deriva sólo de los argumentos (change, phase, contract): no depende de
		// ninguna lectura de base. Así que se puede embeber ANTES de tocar nada y dejar las dos
		// escrituras en UN solo tramo serializado — la red afuera y la atomicidad intacta.
		persistir := !strings.EqualFold(strings.TrimSpace(args.Status), memory.StepFailed)
		var artID, artContenido string
		var artEmb []float32
		if persistir {
			var rerr *RpcError
			artID, artContenido, artEmb, rerr = s.prepararArtefactoSDD(change, phase, memory.SDDContract{
				Summary:         args.Summary,
				Artifacts:       args.Artifacts,
				Risks:           args.Risks,
				NextRecommended: args.NextRecommended,
			})
			if rerr != nil {
				return nil, rerr
			}
		}

		var res interface{}
		var rpcErr *RpcError
		s.withWriteLock(func() {
			var memNote string
			if persistir {
				if rerr := s.guardarArtefactoSDD(artID, artContenido, artEmb); rerr != nil {
					rpcErr = rerr
					return
				}
				memNote = "Artefacto guardado en memoria (id: " + artID + ")."
			}

			run, err := s.engine.CompleteWorkflowStep(runID, phase, args.Summary, args.Status, "")
			if err != nil {
				rpcErr = rpcErrorf(codeInvalidParams, "no se pudo cerrar la fase: %v", err)
				return
			}

			ready, _ := s.engine.WorkflowReady(runID)
			res, rpcErr = jsonResult(s.sddView(change, run, ready, memNote))
		})
		return res, rpcErr

	default:
		return nil, rpcErrorf(codeInvalidParams, "action inválida %q (usá start|next|complete|status)", action)
	}
}

// prepararArtefactoSDD deriva el artefacto de una fase y lo embebe. NO TOCA LA BASE, y por eso se
// llama AFUERA del candado: era `persistSDDArtifact`, que hacía las dos cosas juntas y arrastraba
// la llamada de red adentro del tramo serializado.
//
// El id y el topic_key son deterministas (sdd/<change>/<phase>) para que re-cerrar una fase haga
// UPSERT en vez de duplicar, y el contenido se deriva sólo del contrato: nada de esto necesita
// leer la base, que es justamente lo que permite el corte.
func (s *McpServer) prepararArtefactoSDD(change, phase string, c memory.SDDContract) (string, string, []float32, *RpcError) {
	id := memory.SDDTopicKey(change, phase)
	content := c.Memo(change, phase)

	if !embedding.Enabled(s.embedder) {
		return id, content, nil, nil
	}
	embCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	vec, err := s.embedder.Embed(embCtx, content)
	if err != nil {
		return "", "", nil, rpcErrorf(codeInternalError, "error al generar embedding del artefacto SDD: %v", err)
	}
	return id, content, vec, nil
}

// guardarArtefactoSDD escribe el artefacto ya preparado. Se llama ADENTRO del candado, y es la
// única mitad de lo que antes era `persistSDDArtifact` que toca la base.
func (s *McpServer) guardarArtefactoSDD(id, content string, emb []float32) *RpcError {
	if err := s.engine.SaveObservationWithImportance(id, id, content, 1.0, emb); err != nil {
		return rpcErrorf(codeInternalError, "error al guardar el artefacto SDD: %v", err)
	}
	return nil
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
