package memory

import "testing"

var testPhases = []string{"explore", "plan", "code", "verify"}

func TestPhaseStartAndStatus(t *testing.T) {
	e := newTestEngine(t)

	if _, ok, err := e.PhaseStatus(); err != nil || ok {
		t.Fatalf("sin tarea activa PhaseStatus debe devolver ok=false, obtuve ok=%v err=%v", ok, err)
	}

	st, err := e.StartPhase("refactor-auth", testPhases)
	if err != nil {
		t.Fatalf("StartPhase error: %v", err)
	}
	if st.Task != "refactor-auth" || st.Phase != "explore" || st.Index != 0 || st.Total != 4 {
		t.Fatalf("estado inicial incorrecto: %+v", st)
	}

	got, ok, err := e.PhaseStatus()
	if err != nil || !ok {
		t.Fatalf("tras StartPhase debe haber tarea activa, ok=%v err=%v", ok, err)
	}
	if got.Phase != "explore" || got.Task != "refactor-auth" {
		t.Errorf("PhaseStatus no coincide con StartPhase: %+v", got)
	}
}

func TestPhaseStartValidations(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.StartPhase("", testPhases); err == nil {
		t.Error("StartPhase con task vacío debe fallar")
	}
	if _, err := e.StartPhase("x", nil); err == nil {
		t.Error("StartPhase sin fases debe fallar")
	}
}

func TestPhaseAdvance(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.StartPhase("t", testPhases); err != nil {
		t.Fatal(err)
	}

	st, done, err := e.AdvancePhase(testPhases)
	if err != nil || done {
		t.Fatalf("primer advance no debe terminar, done=%v err=%v", done, err)
	}
	if st.Phase != "plan" || st.Index != 1 {
		t.Fatalf("advance debe ir a plan(1), obtuve %+v", st)
	}

	// Avanzar hasta pasar la última fase.
	e.AdvancePhase(testPhases) // code
	e.AdvancePhase(testPhases) // verify
	_, done, err = e.AdvancePhase(testPhases)
	if err != nil {
		t.Fatalf("advance final error: %v", err)
	}
	if !done {
		t.Error("avanzar más allá de la última fase debe devolver done=true")
	}
	if _, ok, _ := e.PhaseStatus(); ok {
		t.Error("al terminar el pipeline la tarea activa debe limpiarse")
	}
}

func TestPhaseAdvanceUsesStoredSequence(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.StartPhase("t", testPhases); err != nil { // [explore plan code verify]
		t.Fatal(err)
	}
	// Avanzar pasando una secuencia DISTINTA (simula config cambiada): debe usar la
	// secuencia guardada en el estado, no la nueva, para no desalinear el índice.
	st, done, err := e.AdvancePhase([]string{"otra", "cosa"})
	if err != nil || done {
		t.Fatalf("no debe terminar usando la secuencia guardada, done=%v err=%v", done, err)
	}
	if st.Phase != "plan" || st.Index != 1 || st.Total != 4 {
		t.Errorf("debe avanzar según la secuencia guardada (plan, idx 1, total 4), obtuve %+v", st)
	}
}

func TestPhaseAdvanceSinTareaActiva(t *testing.T) {
	e := newTestEngine(t)
	if _, _, err := e.AdvancePhase(testPhases); err == nil {
		t.Error("AdvancePhase sin tarea activa debe fallar")
	}
}

func TestPhaseSet(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.StartPhase("t", testPhases); err != nil {
		t.Fatal(err)
	}
	st, err := e.SetPhase("code", testPhases)
	if err != nil {
		t.Fatalf("SetPhase error: %v", err)
	}
	if st.Phase != "code" || st.Index != 2 {
		t.Errorf("SetPhase('code') debe ir a index 2, obtuve %+v", st)
	}
	if _, err := e.SetPhase("inexistente", testPhases); err == nil {
		t.Error("SetPhase con una fase fuera de la secuencia debe fallar")
	}
}

func TestPhaseClear(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.StartPhase("t", testPhases); err != nil {
		t.Fatal(err)
	}
	if err := e.ClearPhase(); err != nil {
		t.Fatalf("ClearPhase error: %v", err)
	}
	if _, ok, _ := e.PhaseStatus(); ok {
		t.Error("tras ClearPhase no debe haber tarea activa")
	}
}

func TestPhaseDirectiveConocidasYFallback(t *testing.T) {
	for _, p := range testPhases {
		if PhaseDirective(p) == "" {
			t.Errorf("la fase conocida %q debe tener directiva", p)
		}
	}
	if PhaseDirective("fase-rara") == "" {
		t.Error("una fase desconocida debe tener una directiva genérica, no vacía")
	}
}
