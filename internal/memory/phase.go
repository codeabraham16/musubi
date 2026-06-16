package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

// phase.go implementa el estado del PIPELINE POR FASES del loop dirigido. Musubi
// mantiene (en meta, model-free) la fase actual de la tarea en curso —
// explorar→planear→codear→verificar— y la herramienta musubi_phase + el hook por
// turno se la recuerdan a Claude. Musubi secuencia; Claude hace el trabajo.
//
// Es un único pipeline activo por workspace (pensado para trabajo solo). Iniciar
// una tarea nueva reemplaza el estado anterior.

// metaPhaseState es la clave de meta donde se serializa el PhaseState activo.
const metaPhaseState = "loop_phase"

// PhaseState describe la tarea y la fase en curso.
type PhaseState struct {
	Task   string   `json:"task"`
	Phase  string   `json:"phase"`
	Index  int      `json:"index"`            // posición 0-based en la secuencia
	Total  int      `json:"total"`            // cantidad de fases de la secuencia
	Phases []string `json:"phases,omitempty"` // secuencia con la que se inició (fuente de verdad)
}

// seqFor devuelve la secuencia a usar: la guardada en el estado (fuente de verdad)
// o, si el estado es viejo y no la tiene, el fallback pasado por el caller.
func seqFor(st PhaseState, fallback []string) []string {
	if len(st.Phases) > 0 {
		return st.Phases
	}
	return fallback
}

// PhaseStatus devuelve el pipeline activo (ok=false si no hay ninguno).
func (e *DbEngine) PhaseStatus() (PhaseState, bool, error) {
	v, ok, err := e.GetMeta(metaPhaseState)
	if err != nil {
		return PhaseState{}, false, err
	}
	if !ok || strings.TrimSpace(v) == "" {
		return PhaseState{}, false, nil
	}
	var st PhaseState
	if err := json.Unmarshal([]byte(v), &st); err != nil {
		// Estado corrupto: tratarlo como inexistente en vez de romper el arranque.
		return PhaseState{}, false, nil
	}
	return st, true, nil
}

// StartPhase inicia (o reemplaza) un pipeline para task con la secuencia phases,
// posicionándolo en la primera fase.
func (e *DbEngine) StartPhase(task string, phases []string) (PhaseState, error) {
	if strings.TrimSpace(task) == "" {
		return PhaseState{}, fmt.Errorf("task es obligatorio")
	}
	if len(phases) == 0 {
		return PhaseState{}, fmt.Errorf("la secuencia de fases no puede estar vacía")
	}
	seq := append([]string(nil), phases...) // copia defensiva
	st := PhaseState{Task: task, Phase: seq[0], Index: 0, Total: len(seq), Phases: seq}
	if err := e.savePhase(st); err != nil {
		return PhaseState{}, err
	}
	return st, nil
}

// AdvancePhase mueve el pipeline activo a la fase siguiente. Si ya estaba en la
// última, limpia el estado y devuelve done=true.
func (e *DbEngine) AdvancePhase(phases []string) (PhaseState, bool, error) {
	st, ok, err := e.PhaseStatus()
	if err != nil {
		return PhaseState{}, false, err
	}
	if !ok {
		return PhaseState{}, false, fmt.Errorf("no hay un pipeline activo que avanzar")
	}
	seq := seqFor(st, phases)
	if len(seq) == 0 {
		return PhaseState{}, false, fmt.Errorf("la secuencia de fases no puede estar vacía")
	}
	next := st.Index + 1
	if next >= len(seq) {
		if err := e.ClearPhase(); err != nil {
			return PhaseState{}, false, err
		}
		return PhaseState{}, true, nil
	}
	st.Index = next
	st.Phase = seq[next]
	st.Total = len(seq)
	st.Phases = seq
	if err := e.savePhase(st); err != nil {
		return PhaseState{}, false, err
	}
	return st, false, nil
}

// SetPhase salta el pipeline activo a una fase concreta de la secuencia.
func (e *DbEngine) SetPhase(phase string, phases []string) (PhaseState, error) {
	st, ok, err := e.PhaseStatus()
	if err != nil {
		return PhaseState{}, err
	}
	if !ok {
		return PhaseState{}, fmt.Errorf("no hay un pipeline activo")
	}
	seq := seqFor(st, phases)
	idx := -1
	for i, p := range seq {
		if p == phase {
			idx = i
			break
		}
	}
	if idx < 0 {
		return PhaseState{}, fmt.Errorf("la fase %q no está en la secuencia %v", phase, seq)
	}
	st.Phase = phase
	st.Index = idx
	st.Total = len(seq)
	st.Phases = seq
	if err := e.savePhase(st); err != nil {
		return PhaseState{}, err
	}
	return st, nil
}

// ClearPhase cierra el pipeline activo (sin tarea en curso).
func (e *DbEngine) ClearPhase() error {
	return e.SetMeta(metaPhaseState, "")
}

// savePhase serializa y persiste el estado del pipeline.
func (e *DbEngine) savePhase(st PhaseState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("error al serializar el estado de fase: %w", err)
	}
	return e.SetMeta(metaPhaseState, string(data))
}

// PhaseDirective devuelve la guía concreta de qué hacer en una fase. Para fases
// fuera del vocabulario estándar devuelve una directiva genérica (nunca vacía).
func PhaseDirective(phase string) string {
	switch phase {
	case "explore":
		return "Explorá el código y el contexto relevante; recuperá memoria con musubi_recall / musubi_recall_facts. Todavía NO implementes: el objetivo es entender."
	case "plan":
		return "Definí el plan: qué archivos tocás y en qué orden. Confirmá el enfoque con el usuario antes de codear."
	case "code":
		return "Implementá el plan siguiendo las convenciones del proyecto (musubi_resolve_skills). Guardá decisiones no obvias con musubi_save_observation."
	case "verify":
		return "Verificá: corré tests/build y revisá contra el plan. Guardá lo aprendido y cerrá la fase con musubi_phase action=advance."
	default:
		return "Trabajá en esta fase y, cuando termines, avanzá con musubi_phase action=advance."
	}
}
