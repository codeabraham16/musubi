package main

import (
	"context"
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

	ledger        map[string]int
	ledgerSession string
}

func newFakeTurnStore() *fakeTurnStore {
	return &fakeTurnStore{meta: map[string]string{}}
}

func (f *fakeTurnStore) Recall(ctx context.Context, query string, opts memory.RecallOptions) (memory.RecallResult, error) {
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

func (f *fakeTurnStore) LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error) {
	if f.ledger == nil {
		f.ledger = map[string]int{}
	}
	f.ledger[surface] += tokens
	f.ledgerSession = sessionID
	return memory.TokenLedger{SessionID: sessionID, Total: tokens, Surfaces: f.ledger}, nil
}

func (f *fakeTurnStore) LedgerStatus() (memory.TokenLedger, error) {
	total := 0
	for _, v := range f.ledger {
		total += v
	}
	return memory.TokenLedger{SessionID: f.ledgerSession, Total: total, Surfaces: f.ledger}, nil
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

	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, in)

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

// deltaLoop es defaultLoop con la inyección diferencial activada.
func deltaLoop() config.LoopConfig {
	l := defaultLoop()
	l.DeltaInjection = true
	return l
}

func TestTurnDeltaInjectsOnlyNew(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 2,
		Items: []memory.RecallItem{
			{ID: "x1", TopicKey: "t", Gist: "memoria uno", ContentHash: "h1"},
			{ID: "x2", TopicKey: "t", Gist: "memoria dos", ContentHash: "h2"},
		},
	}}
	in := `{"prompt":"qué sabemos","session_id":"s1"}`

	first := turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if !strings.Contains(first, "x1") || !strings.Contains(first, "x2") {
		t.Fatalf("el primer turno debe inyectar ambas memorias, obtuve: %q", first)
	}

	// Segundo turno, misma sesión y misma memoria: nada nuevo -> silencio.
	second := turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if second != "" {
		t.Errorf("sin memoria nueva el delta no debe inyectar nada, obtuve: %q", second)
	}
}

func TestTurnDeltaReinjectsChanged(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "versión vieja", ContentHash: "h1"}},
	}}
	in := `{"prompt":"q","session_id":"s1"}`
	turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)) // inyecta x1

	// La memoria cambió (otro hash): debe re-inyectarse marcada como actualizada.
	store.recall.Items[0] = memory.RecallItem{ID: "x1", TopicKey: "t", Gist: "versión nueva", ContentHash: "h2"}
	out := turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if !strings.Contains(out, "x1") || !strings.Contains(out, "actualizado") {
		t.Errorf("una memoria modificada debe re-inyectarse marcada 'actualizado', obtuve: %q", out)
	}
}

func TestTurnDeltaResetsOnNewSession(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "memoria", ContentHash: "h1"}},
	}}
	turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"q","session_id":"s1"}`))
	// Nueva sesión: el delta se reinicia, debe volver a inyectar.
	out := turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"q","session_id":"s2"}`))
	if !strings.Contains(out, "x1") {
		t.Errorf("una sesión nueva debe reiniciar el delta y re-inyectar, obtuve: %q", out)
	}
}

func TestTurnDeltaDisabledReinjectsEveryTurn(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "memoria", ContentHash: "h1"}},
	}}
	in := `{"prompt":"q","session_id":"s1"}`
	// defaultLoop tiene DeltaInjection=false: ambos turnos re-inyectan.
	turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if !strings.Contains(out, "x1") {
		t.Errorf("con delta apagado cada turno debe re-inyectar, obtuve: %q", out)
	}
}

func TestTurnRecallAccountsTokensInLedger(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{
		Count:      1,
		UsedTokens: 42,
		Items:      []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "algo relevante"}},
	}}
	in := strings.NewReader(`{"prompt":"qué sabemos","session_id":"sess-123"}`)

	turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, in)

	// El ledger holístico estima el bloque FINAL inyectado (header + ids incluidos),
	// no solo el contenido de los gists: debe ser > 0 (no necesariamente == UsedTokens).
	if store.ledger["turn_recall"] <= 0 {
		t.Errorf("el recall por turno debe contabilizar tokens, obtuve %d", store.ledger["turn_recall"])
	}
	if store.ledgerSession != "sess-123" {
		t.Errorf("el ledger debe usar el session_id del hook, obtuve %q", store.ledgerSession)
	}
}

// TestTurnAccountsAllSurfaces verifica el ledger HOLÍSTICO (T9.1) por turno: fase,
// conflictos y captura —antes invisibles en el ledger— se contabilizan junto con el
// recall. Antes solo se medía turn_recall.
func TestTurnAccountsAllSurfaces(t *testing.T) {
	store := newFakeTurnStore()
	store.phaseActive = true
	store.phase = memory.PhaseState{Task: "refactor", Phase: "plan", Index: 1, Total: 4}
	store.pending = []memory.ObsRelation{{ID: "r1"}, {ID: "r2"}}
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "algo relevante"}}}
	store.obsCount = 3
	store.meta[metaLoopObsSeen+":sess-7"] = "5" // base previa alta → captura se dispara este turno
	store.meta[metaLoopTurns+":sess-7"] = "9"

	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250, SurfaceConflicts: true, CaptureReminder: true, ReminderAfterTurns: 2}
	in := strings.NewReader(`{"prompt":"seguimos con la tarea","session_id":"sess-7"}`)
	turnOutput(store, loop, defaultPipe(), maOff(), config.MemoryConfig{}, in)

	for _, surface := range []string{"turn_phase", "turn_recall", "turn_conflicts", "capture_reminder"} {
		if store.ledger[surface] <= 0 {
			t.Errorf("la superficie %q debe contabilizarse en el ledger, obtuve %d", surface, store.ledger[surface])
		}
	}
	if store.ledgerSession != "sess-7" {
		t.Errorf("el ledger debe usar el session_id del hook, obtuve %q", store.ledgerSession)
	}
}

func TestTurnBudgetAlertFiresOnceWhenOverBudget(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	store.ledger = map[string]int{"startup_cognitive": 9000} // gasto previo ya sobre el techo
	store.ledgerSession = "sess-b"
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"sess-b"}`

	// Primer turno sobre presupuesto (budget 8000): debe avisar.
	out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{SessionTokenBudget: 8000}, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "presupuesto") || !strings.Contains(ctx, "musubi_tokens") {
		t.Errorf("estando sobre el techo, el turno debe avisar del presupuesto, obtuve: %q", ctx)
	}
	// Segundo turno misma sesión: NO debe repetir el aviso (throttle por sesión).
	out2 := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{SessionTokenBudget: 8000}, strings.NewReader(in))
	_, ctx2 := hookAdditionalContext(t, out2)
	if strings.Contains(ctx2, "presupuesto") {
		t.Errorf("el aviso de presupuesto debe darse una sola vez por sesión, se repitió: %q", ctx2)
	}
}

func TestTurnBudgetAlertSilentWhenDisabledOrUnder(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	store.ledger = map[string]int{"startup_cognitive": 9000}
	store.ledgerSession = "sess-c"
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"sess-c"}`

	// budget 0 = gobernador desactivado: nunca avisa aunque el gasto sea alto.
	_, ctx := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if strings.Contains(ctx, "presupuesto") {
		t.Errorf("con budget 0 no debe avisar, obtuve: %q", ctx)
	}
	// Gasto por debajo del techo: no avisa.
	store.ledger = map[string]int{"startup_priming": 100}
	_, ctx2 := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{SessionTokenBudget: 8000}, strings.NewReader(in)))
	if strings.Contains(ctx2, "presupuesto") {
		t.Errorf("por debajo del techo no debe avisar, obtuve: %q", ctx2)
	}
}

func TestTurnSilentWithoutPrompt(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x", Gist: "algo"}}}}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"   "}`))
	if out != "" {
		t.Errorf("sin prompt útil el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentWhenDisabled(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x", Gist: "algo"}}}}
	cfg := defaultLoop()
	cfg.PerTurnRecall = false
	out := turnOutput(store, cfg, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("con per_turn_recall=false el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentNilStore(t *testing.T) {
	out := turnOutput(nil, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"hola"}`))
	if out != "" {
		t.Errorf("sin memoria disponible el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSilentNoMatch(t *testing.T) {
	store := &fakeTurnStore{recall: memory.RecallResult{Count: 0}}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"algo sin memoria relacionada"}`))
	if out != "" {
		t.Errorf("sin memoria relevante el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestTurnSurfacesPendingConflicts(t *testing.T) {
	store := &fakeTurnStore{
		recall:  memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "gist"}}},
		pending: []memory.ObsRelation{{ID: "r1"}, {ID: "r2"}},
	}
	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"seguimos"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "musubi_judge") {
		t.Errorf("con conflictos pendientes el contexto debe invitar a resolverlos, obtuve: %q", ctx)
	}
}

func TestTurnPhaseInjected(t *testing.T) {
	store := newFakeTurnStore()
	store.phaseActive = true
	store.phase = memory.PhaseState{Task: "refactor", Phase: "plan", Index: 1, Total: 4}
	out := turnOutput(store, defaultLoop(), defaultPipe(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"sigamos con la tarea"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "refactor") || !strings.Contains(ctx, "plan (2/4)") {
		t.Errorf("debe inyectar la fase activa con tarea y posición, obtuve: %q", ctx)
	}
}

func TestTurnPhaseDeltaSilentWhenUnchanged(t *testing.T) {
	store := newFakeTurnStore()
	store.phaseActive = true
	store.phase = memory.PhaseState{Task: "refactor", Phase: "plan", Index: 1, Total: 4}
	loop := config.LoopConfig{} // aislar la fase
	in := `{"prompt":"sigamos","session_id":"s1"}`

	// Primer turno: inyecta la fase completa (con directiva).
	_, ctx1 := hookAdditionalContext(t, turnOutput(store, loop, defaultPipe(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if !strings.Contains(ctx1, "plan (2/4)") {
		t.Fatalf("el primer turno debe inyectar la fase, obtuve: %q", ctx1)
	}
	// Segundo turno, misma fase y sesión: silencio (no re-inyectar la directiva).
	out2 := turnOutput(store, loop, defaultPipe(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if out2 != "" {
		t.Errorf("sin cambio de fase el turno no debe re-inyectarla, obtuve: %q", out2)
	}
	// La fase avanza: debe re-inyectarse.
	store.phase = memory.PhaseState{Task: "refactor", Phase: "code", Index: 2, Total: 4}
	_, ctx3 := hookAdditionalContext(t, turnOutput(store, loop, defaultPipe(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if !strings.Contains(ctx3, "code (3/4)") {
		t.Errorf("al avanzar de fase debe re-inyectarse, obtuve: %q", ctx3)
	}
}

func TestTurnConflictsDeltaSilentWhenUnchanged(t *testing.T) {
	store := newFakeTurnStore()
	store.pending = []memory.ObsRelation{{ID: "r1"}, {ID: "r2"}}
	loop := config.LoopConfig{SurfaceConflicts: true}
	in := `{"prompt":"seguimos","session_id":"s1"}`

	// Primer turno: avisa de los 2 conflictos.
	_, ctx1 := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if !strings.Contains(ctx1, "musubi_judge") {
		t.Fatalf("el primer turno debe avisar de conflictos, obtuve: %q", ctx1)
	}
	// Segundo turno, misma cantidad: silencio (ya avisado).
	out2 := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	if out2 != "" {
		t.Errorf("sin cambio en la cantidad de conflictos no debe re-avisar, obtuve: %q", out2)
	}
	// Aparece un conflicto nuevo (cambia la cantidad): vuelve a avisar.
	store.pending = []memory.ObsRelation{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}}
	_, ctx3 := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if !strings.Contains(ctx3, "3 relación") {
		t.Errorf("al aparecer un conflicto nuevo debe re-avisar con la cuenta actualizada, obtuve: %q", ctx3)
	}
}

func TestTurnBatchInjected(t *testing.T) {
	store := newFakeTurnStore()
	store.batchActive = true
	store.batch = memory.WorkBatch{BatchID: "b1", Total: 5, Done: 3, Open: 1, Claimed: 1}
	loop := config.LoopConfig{} // aislar: solo el batch
	out := turnOutput(store, loop, pipeOff(), maOn(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"como va el batch"}`))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "Batch activo") || !strings.Contains(ctx, "3/5") {
		t.Errorf("debe inyectar el estado del batch activo, obtuve: %q", ctx)
	}
}

func TestTurnBatchSilentWhenDisabled(t *testing.T) {
	store := newFakeTurnStore()
	store.batchActive = true
	store.batch = memory.WorkBatch{BatchID: "b1", Total: 2, Open: 2}
	out := turnOutput(store, config.LoopConfig{}, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"hola"}`))
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
	out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"hola"}`))
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
	if out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)); out != "" {
		t.Fatalf("el primer turno no debe recordar nada, obtuve: %q", out)
	}
	// Turno 2: acumula (turns=1), todavía sin recordar (umbral 2).
	if out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)); out != "" {
		t.Fatalf("aún no debe recordar antes del umbral, obtuve: %q", out)
	}
	// Turno 3: turns alcanza el umbral → recordatorio.
	out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
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

	turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)) // base
	store.obsCount = 2                                                        // el agente guardó algo
	if out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)); out != "" {
		t.Fatalf("guardar algo debe reiniciar el contador (sin recordatorio), obtuve: %q", out)
	}
	// Ahora estable de nuevo: el contador arranca de cero.
	turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
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
	out := turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"seguimos"}`))
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
	out := turnOutput(store, cfg, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"prompt":"seguimos"}`))
	_, ctx := hookAdditionalContext(t, out)
	if strings.Contains(ctx, "musubi_judge") {
		t.Errorf("con surface_conflicts=false no debe mencionar conflictos, obtuve: %q", ctx)
	}
}

// --- T9.5: brevedad del gobernador (directiva de SALIDA, opt-in) ---

func TestTurnBrevityManualInjectsOncePerSession(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"sess-br"}`

	// Modo "full": el primer turno inyecta la directiva de brevedad y la contabiliza.
	_, ctx := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{BrevityMode: "full"}, strings.NewReader(in)))
	if !strings.Contains(ctx, "conciso") || !strings.Contains(ctx, "brevedad") {
		t.Errorf("modo full debe inyectar la directiva de brevedad, obtuve: %q", ctx)
	}
	if store.ledger["turn_brevity"] <= 0 {
		t.Errorf("la superficie turn_brevity debe contabilizarse en el ledger, obtuve %d", store.ledger["turn_brevity"])
	}
	// Mismo modo y sesión: no se repite (throttle por sesión+modo).
	_, ctx2 := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{BrevityMode: "full"}, strings.NewReader(in)))
	if strings.Contains(ctx2, "brevedad") {
		t.Errorf("la directiva de brevedad no debe repetirse en la misma sesión, obtuve: %q", ctx2)
	}
}

func TestTurnBrevityAutoFiresOnlyWhenOverBudget(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	store.ledger = map[string]int{"startup_cognitive": 9000} // gasto previo ya sobre el techo
	store.ledgerSession = "sess-auto"
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"sess-auto"}`

	// auto + sobre el presupuesto (8000): inyecta la directiva del gobernador.
	_, ctx := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{SessionTokenBudget: 8000, BrevityMode: "auto"}, strings.NewReader(in)))
	if !strings.Contains(ctx, "conciso") || !strings.Contains(ctx, "presupuesto") {
		t.Errorf("auto sobre el techo debe inyectar la directiva del gobernador, obtuve: %q", ctx)
	}
}

func TestTurnBrevityAutoSilentUnderOrDisabled(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	store.ledger = map[string]int{"startup_priming": 100} // por debajo del techo
	store.ledgerSession = "sess-under"
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"sess-under"}`

	// Bajo presupuesto: auto no inyecta (costo cero hasta cruzar el techo).
	_, ctx := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{SessionTokenBudget: 8000, BrevityMode: "auto"}, strings.NewReader(in)))
	if strings.Contains(ctx, "conciso") {
		t.Errorf("auto bajo presupuesto no debe inyectar brevedad, obtuve: %q", ctx)
	}
	// budget 0 desactiva el gobernador: auto tampoco dispara, aunque el gasto sea alto.
	store.ledger = map[string]int{"startup_cognitive": 9000}
	_, ctx0 := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{BrevityMode: "auto"}, strings.NewReader(in)))
	if strings.Contains(ctx0, "conciso") {
		t.Errorf("con budget 0 auto no debe inyectar, obtuve: %q", ctx0)
	}
}

func TestTurnBrevityOffSilent(t *testing.T) {
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", Gist: "algo"}}}
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	in := `{"prompt":"seguimos","session_id":"s-off"}`
	for _, mode := range []string{"", "off"} {
		_, ctx := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{BrevityMode: mode}, strings.NewReader(in)))
		if strings.Contains(ctx, "brevedad") {
			t.Errorf("modo %q no debe inyectar brevedad, obtuve: %q", mode, ctx)
		}
		if store.ledger["turn_brevity"] != 0 {
			t.Errorf("modo %q no debe contabilizar turn_brevity, obtuve %d", mode, store.ledger["turn_brevity"])
		}
	}
}

func TestBrevityDirectiveLevelsDiffer(t *testing.T) {
	if brevityDirective("lite") == brevityDirective("ultra") {
		t.Error("los niveles lite y ultra deben producir directivas distintas")
	}
	if !strings.Contains(brevityDirective("ultra"), "ultra") {
		t.Error("el nivel ultra debe nombrarse en su directiva")
	}
	// Toda directiva preserva lo que no se puede comprimir sin romper precisión.
	for _, m := range []string{"lite", "full", "ultra", "auto"} {
		if !strings.Contains(brevityDirective(m), "código") {
			t.Errorf("la directiva %q debe preservar exacto el código, obtuve: %q", m, brevityDirective(m))
		}
	}
}
