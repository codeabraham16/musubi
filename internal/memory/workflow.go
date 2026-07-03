package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
	StepSkipped = "skipped"

	RunRunning = "running"
	RunDone    = "done"
	RunAborted = "aborted"
)

// Tipos de evento del run journal (run_events). El journal es append-only e inmutable:
// registra cada transición del run para idempotencia, auditoría y el futuro
// replay/observabilidad. El snapshot workflow_runs sigue siendo la verdad corriente.
const (
	EventRunStarted    = "run_started"
	EventStepCompleted = "step_completed"
	EventStepSkipped   = "step_skipped"
	EventStepReopened  = "step_reopened"
	EventRunDone       = "run_done"
)

// RunEvent es una entrada del journal append-only de un run.
type RunEvent struct {
	Seq       int    `json:"seq"`
	StepID    string `json:"step_id,omitempty"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload,omitempty"`
	CreatedAt string `json:"created_at"`
}

// appendRunEvent agrega un evento inmutable al journal del run, dentro de la
// transacción del caller (para que journal y snapshot se muevan juntos). seq es
// monótono creciente por run (MAX(seq)+1); stepID/idempKey vacíos → NULL.
func appendRunEvent(tx *sql.Tx, runID, stepID, eventType, payload, idempKey string) error {
	var seq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq),0)+1 FROM run_events WHERE run_id=?`, runID).Scan(&seq); err != nil {
		return fmt.Errorf("error al calcular el seq del evento: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO run_events (run_id, seq, step_id, event_type, payload, idempotency_key) VALUES (?,?,?,?,?,?)`,
		runID, seq, nullable(stepID), eventType, nullable(payload), nullable(idempKey),
	); err != nil {
		return fmt.Errorf("error al registrar el evento %q: %w", eventType, err)
	}
	return nil
}

// WorkflowStep es un nodo del DAG: un id, sus dependencias (needs) y, opcionalmente,
// una condición `when` (expresión model-free). Un step con `when` falso se salta
// (gate/if_then/switch se expresan con `when`, sin tipos de step separados).
type WorkflowStep struct {
	ID    string   `yaml:"id" json:"id"`
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Title string   `yaml:"title,omitempty" json:"title,omitempty"`
	When  string   `yaml:"when,omitempty" json:"when,omitempty"`
	// RepeatWhile, si no está vacío, re-abre el step (lo vuelve a ofrecer) tras
	// completarlo mientras la expresión sea verdadera — un loop de un solo step.
	// MaxIterations es la cota de seguridad anti-infinito (default defaultMaxIters).
	RepeatWhile   string `yaml:"repeat_while,omitempty" json:"repeat_while,omitempty"`
	MaxIterations int    `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
}

// defaultMaxIters es el tope de iteraciones de un loop si el step no declara uno.
const defaultMaxIters = 100

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
	StepIters   map[string]int    `json:"step_iters,omitempty"`
	Def         WorkflowDef       `json:"definition"`
}

// WorkflowRunSummary es una vista liviana de un run para listados.
type WorkflowRunSummary struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
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
		if strings.TrimSpace(s.When) != "" {
			if _, err := EvalCondition(s.When, map[string]string{}); err != nil {
				errs = append(errs, fmt.Errorf("step %q: condición when inválida: %v", s.ID, err))
			}
		}
		if strings.TrimSpace(s.RepeatWhile) != "" {
			if _, err := EvalCondition(s.RepeatWhile, map[string]string{}); err != nil {
				errs = append(errs, fmt.Errorf("step %q: condición repeat_while inválida: %v", s.ID, err))
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

// terminalStep indica si un estado de step es terminal a efectos de dependencias
// (done o skipped satisfacen una dependencia; failed la bloquea).
func terminalStep(status string) bool {
	return status == StepDone || status == StepSkipped
}

// ReadySteps devuelve los ids de step candidatos a ejecutar: pendientes y con TODAS
// sus dependencias satisfechas (done o skipped). Una dependencia failed bloquea al
// step. No evalúa `when` (eso lo hace el engine al avanzar, porque persiste skips).
// Es la decisión central del scheduler, model-free.
func (d WorkflowDef) ReadySteps(stepStatus map[string]string) []string {
	var ready []string
	for _, s := range d.Steps {
		st := stepStatus[s.ID]
		if st == StepDone || st == StepSkipped {
			continue
		}
		satisfied := true
		for _, n := range s.Needs {
			if !terminalStep(stepStatus[n]) {
				satisfied = false
				break
			}
		}
		if satisfied {
			ready = append(ready, s.ID)
		}
	}
	return ready
}

// evalContext arma el contexto para las expresiones `when`: claves
// "step.<id>.status" y "step.<id>.result".
func evalContext(run WorkflowRun) map[string]string {
	ctx := map[string]string{}
	for _, s := range run.Def.Steps {
		ctx["step."+s.ID+".status"] = run.StepStatus[s.ID]
		ctx["step."+s.ID+".result"] = run.StepResults[s.ID]
	}
	return ctx
}

// whenByID devuelve el mapa stepID -> expresión when.
func (d WorkflowDef) whenByID() map[string]string {
	m := map[string]string{}
	for _, s := range d.Steps {
		m[s.ID] = s.When
	}
	return m
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
	tx, err := e.db.Begin()
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error iniciando la tx del run: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		INSERT INTO workflow_runs (run_id, workflow_id, definition, status, step_status, step_results)
		VALUES (?, ?, ?, ?, ?, '{}')
		ON CONFLICT(run_id) DO NOTHING;`,
		runID, def.ID, string(defJSON), RunRunning, string(statusJSON))
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error creando el run: %w", err)
	}
	// run_started sólo cuando el run se crea de verdad (no al reabrir uno existente).
	if n, _ := res.RowsAffected(); n > 0 {
		if err := appendRunEvent(tx, runID, "", EventRunStarted, def.ID, ""); err != nil {
			return WorkflowRun{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkflowRun{}, fmt.Errorf("error confirmando el run: %w", err)
	}
	run, _, err := e.WorkflowRunStatus(runID)
	return run, err
}

// WorkflowRunStatus carga el estado de un run. El segundo valor es false si no existe.
func (e *DbEngine) WorkflowRunStatus(runID string) (WorkflowRun, bool, error) {
	var defJSON, statusJSON, resultsJSON, itersJSON string
	run := WorkflowRun{RunID: runID}
	err := e.db.QueryRow(`
		SELECT workflow_id, definition, status, step_status, step_results, step_iters
		FROM workflow_runs WHERE run_id=?;`, runID).
		Scan(&run.WorkflowID, &defJSON, &run.Status, &statusJSON, &resultsJSON, &itersJSON)
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
	_ = json.Unmarshal([]byte(itersJSON), &run.StepIters)
	if run.StepStatus == nil {
		run.StepStatus = map[string]string{}
	}
	if run.StepResults == nil {
		run.StepResults = map[string]string{}
	}
	if run.StepIters == nil {
		run.StepIters = map[string]int{}
	}
	return run, true, nil
}

// WorkflowReady devuelve los step ids listos para ejecutar. Antes de devolver,
// "avanza" el run: los candidatos cuya condición `when` es falsa se marcan skipped
// y se persisten (lo que puede destrabar o saltar dependientes), iterando hasta
// estabilizar. Si al saltar quedan todos los steps terminales, cierra el run.
func (e *DbEngine) WorkflowReady(runID string) ([]string, error) {
	run, ok, err := e.WorkflowRunStatus(runID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("run %q no existe", runID)
	}
	whens := run.Def.whenByID()
	changed := false
	var ready []string
	var skipped []string
	for {
		candidates := run.Def.ReadySteps(run.StepStatus)
		skippedThisPass := false
		ready = ready[:0]
		for _, id := range candidates {
			expr := whens[id]
			if strings.TrimSpace(expr) == "" {
				ready = append(ready, id)
				continue
			}
			pass, eerr := EvalCondition(expr, evalContext(run))
			if eerr != nil {
				return nil, fmt.Errorf("condición when inválida en step %q: %w", id, eerr)
			}
			if pass {
				ready = append(ready, id)
			} else {
				run.StepStatus[id] = StepSkipped
				changed = true
				skippedThisPass = true
				skipped = append(skipped, id)
			}
		}
		if !skippedThisPass {
			break
		}
	}
	if changed {
		tx, err := e.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("error iniciando la tx de skips: %w", err)
		}
		defer tx.Rollback()
		_, becameDone, perr := persistRunStatusTx(tx, run)
		if perr != nil {
			return nil, perr
		}
		for _, id := range skipped {
			if err := appendRunEvent(tx, runID, id, EventStepSkipped, "", ""); err != nil {
				return nil, err
			}
		}
		if becameDone {
			if err := appendRunEvent(tx, runID, "", EventRunDone, "", ""); err != nil {
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("error confirmando los skips: %w", err)
		}
	}
	return ready, nil
}

// persistRunStatusTx guarda step_status/step_results dentro de la transacción del
// caller y recalcula si el run quedó done (todos los steps terminales: done o skipped).
// El segundo valor (becameDone) es true si el run transicionó a done EN esta llamada,
// para que el caller emita el evento run_done en la misma tx.
func persistRunStatusTx(tx *sql.Tx, run WorkflowRun) (WorkflowRun, bool, error) {
	status := run.Status
	allTerminal := true
	for _, s := range run.Def.Steps {
		if !terminalStep(run.StepStatus[s.ID]) {
			allTerminal = false
			break
		}
	}
	becameDone := allTerminal && status != RunDone
	if allTerminal {
		status = RunDone
	}
	statusJSON, _ := json.Marshal(run.StepStatus)
	resultsJSON, _ := json.Marshal(run.StepResults)
	itersJSON, _ := json.Marshal(run.StepIters)
	_, err := tx.Exec(`
		UPDATE workflow_runs SET status=?, step_status=?, step_results=?, step_iters=?, updated_at=CURRENT_TIMESTAMP
		WHERE run_id=?;`,
		status, string(statusJSON), string(resultsJSON), string(itersJSON), run.RunID)
	if err != nil {
		return WorkflowRun{}, false, fmt.Errorf("error actualizando el run: %w", err)
	}
	run.Status = status
	return run, becameDone, nil
}

// CompleteWorkflowStep marca un step como done (o failed) con su resultado y, si
// todos los steps quedaron done, marca el run como done. Devuelve el run actualizado.
// Cada llamada registra un evento step_completed en el journal (más step_reopened /
// run_done si corresponde), en la MISMA transacción que actualiza el snapshot.
// idempotencyKey (opcional): si ya existe un evento con esa clave para el run, la
// llamada es un NO-OP (reintento seguro) y devuelve el estado actual sin re-aplicar.
func (e *DbEngine) CompleteWorkflowStep(runID, stepID, result, stepStatus, idempotencyKey string) (WorkflowRun, error) {
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

	// Idempotencia: si ya se registró un evento con esta clave, es un reintento -> no-op.
	if idempotencyKey != "" {
		var seen int
		if err := e.db.QueryRow(
			`SELECT COUNT(*) FROM run_events WHERE run_id=? AND idempotency_key=?`, runID, idempotencyKey,
		).Scan(&seen); err != nil {
			return WorkflowRun{}, fmt.Errorf("error verificando idempotencia: %w", err)
		}
		if seen > 0 {
			return run, nil // no-op: devolver el estado ya aplicado
		}
	}

	run.StepStatus[stepID] = stepStatus
	run.StepResults[stepID] = result

	// Loop de un step: si quedó done y declara repeat_while verdadero (bajo la cota
	// de iteraciones), se RE-ABRE (vuelve a pending) para ejecutarse otra vez. El
	// ctx ya refleja el done + result recién seteados.
	reopened := false
	if stepStatus == StepDone {
		if step, ok := stepByID(run.Def, stepID); ok && strings.TrimSpace(step.RepeatWhile) != "" {
			max := step.MaxIterations
			if max <= 0 {
				max = defaultMaxIters
			}
			again, eerr := EvalCondition(step.RepeatWhile, evalContext(run))
			if eerr != nil {
				return WorkflowRun{}, fmt.Errorf("repeat_while inválido en %q: %w", stepID, eerr)
			}
			if again && run.StepIters[stepID] < max {
				run.StepIters[stepID]++
				run.StepStatus[stepID] = StepPending // re-abrir para otra iteración
				reopened = true
			}
		}
	}

	// El run se cierra (done) cuando todos los steps son terminales (done o skipped).
	// Un step failed lo deja running para que el agente decida. Snapshot + eventos van
	// en una sola tx: nunca divergen.
	tx, err := e.db.Begin()
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("error iniciando la tx del complete: %w", err)
	}
	defer tx.Rollback()
	updated, becameDone, perr := persistRunStatusTx(tx, run)
	if perr != nil {
		return WorkflowRun{}, perr
	}
	payload, _ := json.Marshal(map[string]string{"status": stepStatus, "result": result})
	if err := appendRunEvent(tx, runID, stepID, EventStepCompleted, string(payload), idempotencyKey); err != nil {
		return WorkflowRun{}, err
	}
	if reopened {
		if err := appendRunEvent(tx, runID, stepID, EventStepReopened, "", ""); err != nil {
			return WorkflowRun{}, err
		}
	}
	if becameDone {
		if err := appendRunEvent(tx, runID, "", EventRunDone, "", ""); err != nil {
			return WorkflowRun{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkflowRun{}, fmt.Errorf("error confirmando el complete: %w", err)
	}
	return updated, nil
}

// WorkflowJournal devuelve el journal append-only de un run: sus eventos en orden de
// seq (run_started, step_completed, step_skipped, step_reopened, run_done). Es la base
// de la auditoría/observabilidad y del futuro replay del run.
func (e *DbEngine) WorkflowJournal(runID string) ([]RunEvent, error) {
	rows, err := e.db.Query(`
		SELECT seq, COALESCE(step_id,''), event_type, COALESCE(payload,''), COALESCE(created_at,'')
		FROM run_events WHERE run_id=? ORDER BY seq`, runID)
	if err != nil {
		return nil, fmt.Errorf("error leyendo el journal del run: %w", err)
	}
	defer rows.Close()
	out := []RunEvent{}
	for rows.Next() {
		var ev RunEvent
		if err := rows.Scan(&ev.Seq, &ev.StepID, &ev.EventType, &ev.Payload, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("error escaneando evento del journal: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando el journal: %w", err)
	}
	return out, nil
}

// stepByID devuelve el step con ese id (y true) de la definición.
func stepByID(def WorkflowDef, id string) (WorkflowStep, bool) {
	for _, s := range def.Steps {
		if s.ID == id {
			return s, true
		}
	}
	return WorkflowStep{}, false
}

// WorkflowListRuns devuelve un resumen de todos los runs, del más reciente al más viejo.
func (e *DbEngine) WorkflowListRuns() ([]WorkflowRunSummary, error) {
	rows, err := e.db.Query(`
		SELECT run_id, workflow_id, status, step_status
		FROM workflow_runs ORDER BY updated_at DESC;`)
	if err != nil {
		return nil, fmt.Errorf("error listando runs: %w", err)
	}
	defer rows.Close()
	var out []WorkflowRunSummary
	for rows.Next() {
		var s WorkflowRunSummary
		var statusJSON string
		if err := rows.Scan(&s.RunID, &s.WorkflowID, &s.Status, &statusJSON); err != nil {
			return nil, fmt.Errorf("error leyendo run: %w", err)
		}
		var st map[string]string
		_ = json.Unmarshal([]byte(statusJSON), &st)
		s.Total = len(st)
		for _, v := range st {
			if v == StepDone {
				s.Done++
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando runs: %w", err)
	}
	return out, nil
}
