package main

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// fakeTurnStore implementa turnStore para tests deterministas de la inyección por
// turno, sin DB real. Captura la query y opciones del último recall.
type fakeTurnStore struct {
	recall    memory.RecallResult
	pending   []memory.ObsRelation
	lastQuery string
	lastOpts  memory.RecallOptions

	obsCount    int
	phase       memory.PhaseState
	phaseActive bool
	batch       memory.WorkBatch
	batchActive bool
	meta        map[string]string
}

func newFakeTurnStore() *fakeTurnStore {
	return &fakeTurnStore{meta: map[string]string{}}
}

func (f *fakeTurnStore) Recall(query string, opts memory.RecallOptions) (memory.RecallResult, error) {
	f.lastQuery = query
	f.lastOpts = opts
	return f.recall, nil
}

func (f *fakeTurnStore) PendingObsRelations() ([]memory.ObsRelation, error) {
	return f.pending, nil
}

func (f *fakeTurnStore) CountObservations() (int, error) { return f.obsCount, nil }

func (f *fakeTurnStore) PhaseStatus() (memory.PhaseState, bool, error) {
	return f.phase, f.phaseActive, nil
}

func (f *fakeTurnStore) ActiveBatch() (memory.WorkBatch, bool, error) {
	return f.batch, f.batchActive, nil
}

func (f *fakeTurnStore) GetMeta(key string) (string, bool, error) {
	if f.meta == nil {
		return "", false, nil
	}
	v, ok := f.meta[key]
	return v, ok, nil
}

func (f *fakeTurnStore) SetMeta(key, value string) error {
	if f.meta == nil {
		f.meta = map[string]string{}
	}
	f.meta[key] = value
	return nil
}

func defaultLoop() config.LoopConfig {
	return config.LoopConfig{PerTurnRecall: true, RecallBudget: 250, SurfaceConflicts: true, CaptureReminder: true, ReminderAfterTurns: 5}
}

// pipeOff devuelve un pipeline desactivado (no inyecta fase) para tests que aíslan
// otros bloques.
func pipeOff() config.PipelineConfig { return config.PipelineConfig{Enabled: false} }

func defaultPipe() config.PipelineConfig {
	return config.PipelineConfig{Enabled: true, Phases: []string{"explore", "plan", "code", "verify"}}
}

func maOff() config.MultiAgentConfig { return config.MultiAgentConfig{Enabled: false} }

func maOn() config.MultiAgentConfig { return config.MultiAgentConfig{Enabled: true, MaxBatchUnits: 50} }

// hookAdditionalContext extrae el additionalContext del envelope JSON de un hook.
func hookAdditionalContext(t *testing.T, out string) (string, string) {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		return "", ""
	}
	var env struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}
	return env.HookSpecificOutput.HookEventName, env.HookSpecificOutput.AdditionalContext
}

func TestTurnInjectsRelevantMemory(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "x1", TopicKey: "arch/db", Gist: "Usamos PostgreSQL como base"}},
	}}
	in := strings.NewReader(`{"prompt":"cómo está configurada la base de datos","hook_event_name":"UserPromptSubmit"}`)

	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), in)

	event, ctx := hookAdditionalContext(t, out)
	if event != "UserPromptSubmit" {
		t.Errorf("esperaba hookEventName UserPromptSubmit, obtuve %q", event)
	}
	if !strings.Contains(ctx, "Usamos PostgreSQL como base") {
		t.Errorf("el contexto debe incluir el gist relevante, obtuve: %q", ctx)
	}
	if !strings.Contains(ctx, "x1") {
		t.Errorf("el contexto debe incluir el id para expandir, obtuve: %q", ctx)
	}
	// El recall por turno usa el prompt como query y es read-only.
	if store.lastQuery != "cómo está configurada la base de datos" {
		t.Errorf("el recall debe usar el prompt como query, obtuve %q", store.lastQuery)
	}
	if !store.lastOpts.NoBump {
		t.Error("el recall por turno debe ser read-only (NoBump=true)")
	}
	if store.lastOpts.TokenBudget != 250 {
		t.Errorf("el recall debe respetar el budget de loop, obtuve %d", store.lastOpts.TokenBudget)
	}
}

func TestTurnSilentWithoutPrompt(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x", Gist: "algo"}}}}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), strings.NewReader(`{"prompt":"   "}`))
	if out != "" {
		t.Errorf("sin prompt útil el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentWhenDisabled(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x", Gist: "algo"}}}}
	cfg := defaultLoop()
	cfg.PerTurnRecall = false
	out := turnOutput(store, cfg, pipeOff(), maOff(), strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("con per_turn_recall=false el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentNilStore(t *testing.T) {
	out := turnOutput(nil, defaultLoop(), pipeOff(), maOff(), strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("sin memoria disponible el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentNoMatch(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 0}}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), strings.NewReader(`{"prompt":"algo sin memoria relacionada"}`))
	if out != "" {
		t.Errorf("sin memoria relevante el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSurfacesPendingConflicts(t *testing.T) {
	store := &fakeTurnStore{
		recall:  memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "gist"}}},
		pending: []memory.ObsRelation{{ID: "r1"}, {ID: "r2"}},
	}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), strings.NewReader(`{"prompt":"seguimos"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "musubi_judge") {
		t.Errorf("con conflictos pendientes el contexto debe invitar a resolverlos, obtuve: %q", ctx)
	}
}

func TestTurnPhaseInjected(t *testing.T) {
	store := newFakeTurnStore()
	store.phaseActive = true
	store.phase = memory.PhaseState{Task: "refactor", Phase: "plan", Index: 1, Total: 4}
	out := turnOutput(store, defaultLoop(), defaultPipe(), maOff(), strings.NewReader(`{"prompt":"sigamos con la tarea"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "refactor") || !strings.Contains(ctx, "plan (2/4)") {
		t.Errorf("debe inyectar la fase activa con tarea y posición, obtuve: %q", ctx)
	}
}

func TestTurnBatchInjected(t *testing.T) {
	store := newFakeTurnStore()
	store.batchActive = true
	store.batch = memory.WorkBatch{BatchID: "b1", Total: 5, Done: 3, Open: 1, Claimed: 1}
	loop := config.LoopConfig{} // aislar: solo el batch
	out := turnOutput(store, loop, pipeOff(), maOn(), strings.NewReader(`{"prompt":"como va el batch"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "Batch activo") || !strings.Contains(ctx, "3/5") {
		t.Errorf("debe inyectar el estado del batch activo, obtuve: %q", ctx)
	}
}

func TestTurnBatchSilentWhenDisabled(t *testing.T) {
	store := newFakeTurnStore()
	store.batchActive = true
	store.batch = memory.WorkBatch{BatchID: "b1", Total: 2, Open: 2}
	out := turnOutput(store, config.LoopConfig{}, pipeOff(), maOff(), strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("con multiagent desactivado no debe inyectar el batch, obtuve: %q", out)
	}
}

func TestTurnPhaseSilentWhenDisabled(t *testing.T) {
	store := newFakeTurnStore()
	store.phaseActive = true
	store.phase = memory.PhaseState{Task: "x", Phase: "plan", Index: 1, Total: 4}
	// Pipeline desactivado: no debe inyectar fase (y sin otra fuente, silencio).
	loop := defaultLoop()
	loop.PerTurnRecall = false
	loop.CaptureReminder = false
	out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("con pipeline desactivado no debe inyectar fase, obtuve: %q", out)
	}
}

func TestTurnCaptureReminderAfterNTurns(t *testing.T) {
	store := newFakeTurnStore()
	store.obsCount = 3 // estable: no se guarda nada entre turnos
	loop := config.LoopConfig{CaptureReminder: true, ReminderAfterTurns: 2}
	in := `{"prompt":"seguimos trabajando"}`

	// Turno 1: fija la línea base, sin recordatorio.
	if out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in)); out != "" {
		t.Fatalf("el primer turno no debe recordar nada, obtuve: %q", out)
	}
	// Turno 2: acumula (turns=1), todavía sin recordar (umbral 2).
	if out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in)); out != "" {
		t.Fatalf("aún no debe recordar antes del umbral, obtuve: %q", out)
	}
	// Turno 3: turns alcanza el umbral → recordatorio.
	out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "captura") {
		t.Errorf("al alcanzar el umbral de turnos sin guardar debe recordar la captura, obtuve: %q", ctx)
	}
}

func TestTurnCaptureReminderResetsOnSave(t *testing.T) {
	store := newFakeTurnStore()
	store.obsCount = 1
	loop := config.LoopConfig{CaptureReminder: true, ReminderAfterTurns: 2}
	in := `{"prompt":"trabajando"}`

	turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in)) // base
	store.obsCount = 2                                         // el agente guardó algo
	if out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in)); out != "" {
		t.Fatalf("guardar algo debe reiniciar el contador (sin recordatorio), obtuve: %q", out)
	}
	// Ahora estable de nuevo: el contador arranca de cero.
	turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in))
	out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "captura") {
		t.Errorf("tras reiniciar y pasar 2 turnos sin guardar, debe recordar, obtuve: %q", ctx)
	}
}

func TestTurnConflictsWithoutRecall(t *testing.T) {
	// SurfaceConflicts es independiente de PerTurnRecall: si el recall está apagado
	// pero hay conflictos pendientes y surface está prendido, deben mostrarse.
	store := newFakeTurnStore()
	store.pending = []memory.ObsRelation{{ID: "r1"}}
	loop := config.LoopConfig{PerTurnRecall: false, SurfaceConflicts: true}
	out := turnOutput(store, loop, pipeOff(), maOff(), strings.NewReader(`{"prompt":"seguimos"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "musubi_judge") {
		t.Errorf("con recall off pero surface_conflicts on, los conflictos deben mostrarse, obtuve: %q", ctx)
	}
}

func TestTurnConflictsToggleOff(t *testing.T) {
	store := &fakeTurnStore{
		recall:  memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "gist"}}},
		pending: []memory.ObsRelation{{ID: "r1"}},
	}
	cfg := defaultLoop()
	cfg.SurfaceConflicts = false
	out := turnOutput(store, cfg, pipeOff(), maOff(), strings.NewReader(`{"prompt":"seguimos"}`))
	_, ctx := hookAdditionalContext(t, out)
	if strings.Contains(ctx, "musubi_judge") {
		t.Errorf("con surface_conflicts=false no debe mencionar conflictos, obtuve: %q", ctx)
	}
}
