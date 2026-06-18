package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// workflow.go implementa el núcleo MODEL-FREE del motor de orquestación DAG (A1).
// Musubi NO ejecuta los steps: define el grafo, lo valida, persiste el estado del
// run en SQLite y le dice al agente qué step(s) están listos. El agente ejecuta y
// reporta el resultado. El estado vive en la tabla workflow_runs, así que un run
// es resumible entre sesiones y compactaciones (el diferencial de Musubi).

// Estados de step y de run.
const (
	StepPending = "pending"
	StepDone    = "done"
	StepFailed  = "failed"

	RunRunning = "running"
	RunDone    = "done"
	RunAborted = "aborted"
)

// WorkflowStep es un nodo del DAG: un id y sus dependencias (needs).
type WorkflowStep struct {
	ID    string   `yaml:"id" json:"id"`
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Title string   `yaml:"title,omitempty" json:"title,omitempty"`
}

// WorkflowDef es la definición declarativa de un workflow (parseada de YAML).
type WorkflowDef struct {
	ID            string         `yaml:"id" json:"id"`
	Name          string         `yaml:"name,omitempty" json:"name,omitempty"`
	Version       string         `yaml:"version,omitempty" json:"version,omitempty"`
	SchemaVersion string         `yaml:"schema_version,omitempty" json:"schema_version,omitempty"`
	Steps         []WorkflowStep `yaml:"steps" json:"steps"`
}

// WorkflowRun es el estado persistido de una ejecución.
type WorkflowRun struct {
	RunID       string            `json:"run_id"`
	WorkflowID  string            `json:"workflow_id"`
	Status      string            `json:"status"`
	StepStatus  map[string]string `json:"step_status"`
	StepResults map[string]string `json:"step_results"`
	Def         WorkflowDef       `json:"definition"`
}

// ParseWorkflowDef parsea un workflow YAML.
func ParseWorkflowDef(data []byte) (WorkflowDef, error) {
	var def WorkflowDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return WorkflowDef{}, fmt.Errorf("YAML de workflow inválido: %w", err)
	}
	return def, nil
}

// Validate chequea el DAG: id presente, ids de step únicos, needs que referencian
// steps existentes y ausencia de ciclos. Devuelve la lista de errores (vacía = OK).
func (d WorkflowDef) Validate() []error {
	var errs []error
	if d.ID == "" {
		errs = append(errs, fmt.Errorf("el workflow debe tener un id"))
	}
	if len(d.Steps) == 0 {
		errs = append(errs, fmt.Errorf("el workflow no tiene steps"))
	}
	seen := map[string]bool{}
	for _, s := range d.Steps {
		if s.ID == "" {
			errs = append(errs, fmt.Errorf("hay un step sin id"))
			continue
		}
		if seen[s.ID] {
			errs = append(errs, fmt.Errorf("step id duplicado: %q", s.ID))
		}
		seen[s.ID] = true
	}
	for _, s := range d.Steps {
		for _, n := range s.Needs {
			if !seen[n] {
				errs = append(errs, fmt.Errorf("step %q depende de %q que no existe", s.ID, n))
			}
		}
	}
	if cyc := d.findCycle(); len(cyc) > 0 {
		errs = append(errs, fmt.Errorf("ciclo de dependencias: %v", cyc))
	}
	return errs
}

// findCycle devuelve un ciclo (lista de step ids) si existe, o nil.
func (d WorkflowDef) findCycle() []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	needs := map[string][]string{}
	for _, s := range d.Steps {
		needs[s.ID] = s.Needs
	}
	color := map[string]int{}
	var stack []string
	var dfs func(string) []string
	dfs = func(u string) []string {
		color[u] = gray
		stack = append(stack, u)
		for _, v := range needs[u] {
			if color[v] == gray {
				return append([]string{}, stack...)
			}
			if color[v] == white {
				if c := dfs(v); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
		return nil
	}
	for _, s := range d.Steps {
		if color[s.ID] == white {
			if c := dfs(s.ID); c != nil {
				return c
			}
		}
	}
	return nil
}

// ReadySteps devuelve los ids de step listos para ejecutar: no completados y con
// TODAS sus dependencias en estado done. Es la decisión central del scheduler,
// model-free.
func (d WorkflowDef) ReadySteps(stepStatus map[string]string) []string {
	var ready []string
	for _, s := range d.Steps {
		if stepStatus[s.ID] == StepDone {
			continue
		}
		allDone := true
		for _, n := range s.Needs {
			if stepStatus[n] != StepDone {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, s.ID)
		}
	}
	return ready
}

// --- Persistencia en SQLite ---

// StartWorkflowRun crea (o reabre idempotente) un run a partir de una definición
// validada. Persiste la definición completa para que el run sea resumible sin el
// archivo YAML original.
func (e *DbEngine) StartWorkflowRun(runID string, def WorkflowDef) (WorkflowRun, error) {
	if errs := def.Validate(); len(errs) > 0 {
		return WorkflowRun{}, fmt.Errorf("workflow inválido: %v", errs)
	}
	defJSON, err := json.Marshal(def)
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error serializando la definición: %w", err)
	}
	status := map[string]string{}
	for _, s := range def.Steps {
		status[s.ID] = StepPending
	}
	statusJSON, _ := json.Marshal(status)
	_, err = e.db.Exec(`
		INSERT INTO workflow_runs (run_id, workflow_id, definition, status, step_status, step_results)
		VALUES (?, ?, ?, ?, ?, '{}')
		ON CONFLICT(run_id) DO NOTHING;`,
		runID, def.ID, string(defJSON), RunRunning, string(statusJSON))
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error creando el run: %w", err)
	}
	run, _, err := e.WorkflowRunStatus(runID)
	return run, err
}

// WorkflowRunStatus carga el estado de un run. El segundo valor es false si no existe.
func (e *DbEngine) WorkflowRunStatus(runID string) (WorkflowRun, bool, error) {
	var defJSON, statusJSON, resultsJSON string
	run := WorkflowRun{RunID: runID}
	err := e.db.QueryRow(`
		SELECT workflow_id, definition, status, step_status, step_results
		FROM workflow_runs WHERE run_id=?;`, runID).
		Scan(&run.WorkflowID, &defJSON, &run.Status, &statusJSON, &resultsJSON)
	if err == sql.ErrNoRows {
		return WorkflowRun{}, false, nil
	}
	if err != nil {
		return WorkflowRun{}, false, fmt.Errorf("error leyendo el run: %w", err)
	}
	if err := json.Unmarshal([]byte(defJSON), &run.Def); err != nil {
		return WorkflowRun{}, false, fmt.Errorf("definición corrupta: %w", err)
	}
	_ = json.Unmarshal([]byte(statusJSON), &run.StepStatus)
	_ = json.Unmarshal([]byte(resultsJSON), &run.StepResults)
	if run.StepStatus == nil {
		run.StepStatus = map[string]string{}
	}
	if run.StepResults == nil {
		run.StepResults = map[string]string{}
	}
	return run, true, nil
}

// WorkflowReady devuelve los step ids listos para ejecutar en el run dado.
func (e *DbEngine) WorkflowReady(runID string) ([]string, error) {
	run, ok, err := e.WorkflowRunStatus(runID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("run %q no existe", runID)
	}
	return run.Def.ReadySteps(run.StepStatus), nil
}

// CompleteWorkflowStep marca un step como done (o failed) con su resultado y, si
// todos los steps quedaron done, marca el run como done. Devuelve el run actualizado.
func (e *DbEngine) CompleteWorkflowStep(runID, stepID, result, stepStatus string) (WorkflowRun, error) {
	if stepStatus == "" {
		stepStatus = StepDone
	}
	if stepStatus != StepDone && stepStatus != StepFailed {
		return WorkflowRun{}, fmt.Errorf("estado de step inválido: %q (usá done|failed)", stepStatus)
	}
	run, ok, err := e.WorkflowRunStatus(runID)
	if err != nil {
		return WorkflowRun{}, err
	}
	if !ok {
		return WorkflowRun{}, fmt.Errorf("run %q no existe", runID)
	}
	if _, exists := run.StepStatus[stepID]; !exists {
		return WorkflowRun{}, fmt.Errorf("el step %q no pertenece al workflow", stepID)
	}
	run.StepStatus[stepID] = stepStatus
	run.StepResults[stepID] = result

	// ¿Quedó el run completo? (todos done). Un step failed deja el run running para
	// que el agente decida; el run se cierra solo cuando todos están done.
	runStatus := run.Status
	allDone := true
	for _, s := range run.Def.Steps {
		if run.StepStatus[s.ID] != StepDone {
			allDone = false
			break
		}
	}
	if allDone {
		runStatus = RunDone
	}

	statusJSON, _ := json.Marshal(run.StepStatus)
	resultsJSON, _ := json.Marshal(run.StepResults)
	_, err = e.db.Exec(`
		UPDATE workflow_runs SET status=?, step_status=?, step_results=?, updated_at=CURRENT_TIMESTAMP
		WHERE run_id=?;`,
		runStatus, string(statusJSON), string(resultsJSON), runID)
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error actualizando el run: %w", err)
	}
	run.Status = runStatus
	return run, nil
}
