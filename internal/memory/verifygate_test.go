package memory

import "testing"

// vgRun arma a → check(verify, maxIters) → b y completa a. Devuelve el engine.
func vgRun(t *testing.T, runID string, maxIters int) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	def := WorkflowDef{ID: "vg", Steps: []WorkflowStep{
		{ID: "a"},
		{ID: "check", Needs: []string{"a"}, Verify: "revisá que compile y pase los tests", MaxIterations: maxIters},
		{ID: "b", Needs: []string{"check"}},
	}}
	if _, err := engine.StartWorkflowRun(runID, def); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady(runID)
	if _, err := engine.CompleteWorkflowStep(runID, "a", "ok", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	return engine
}

func vgContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Completar un step con verify lo deja en verifying (no done) y bloquea dependientes.
func TestVerifyGateBlocksDone(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R") // check queda ready
	if _, err := engine.CompleteWorkflowStep("R", "check", "hecho", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	run, _, _ := engine.WorkflowRunStatus("R")
	if run.StepStatus["check"] != StepVerifying {
		t.Errorf("un step con verify completado debe quedar verifying, está %q", run.StepStatus["check"])
	}
	ready, _ := engine.WorkflowReady("R")
	if vgContains(ready, "b") {
		t.Error("'b' está bloqueado por el gate en verificación, no debe estar ready")
	}
}

// verify(pass) marca el step done y destraba dependientes.
func TestVerifyGatePass(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "hecho", StepDone, "")

	run, reflections, err := engine.VerifyWorkflowStep("R", "check", true, "", "")
	if err != nil {
		t.Fatalf("verify pass: %v", err)
	}
	if run.StepStatus["check"] != StepDone {
		t.Errorf("verify pass debe dejar el step done, está %q", run.StepStatus["check"])
	}
	if len(reflections) != 0 {
		t.Errorf("un pass no debe traer reflexiones, obtuve %v", reflections)
	}
	// step_completed debe existir (uniforme).
	ev, _ := engine.WorkflowJournal("R")
	if !func() bool {
		for _, e := range ev {
			if e.EventType == EventStepCompleted && e.StepID == "check" {
				return true
			}
		}
		return false
	}() {
		t.Error("verify pass debe journalear step_completed (uniforme)")
	}
	ready, _ := engine.WorkflowReady("R")
	if !vgContains(ready, "b") {
		t.Errorf("tras el pass del gate, 'b' debe estar ready, obtuve %v", ready)
	}
}

// verify(fail) con presupuesto registra la reflexión y reabre el step.
func TestVerifyGateFailReopens(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "intento 1", StepDone, "")

	run, reflections, err := engine.VerifyWorkflowStep("R", "check", false, "faltó cubrir el caso X", "")
	if err != nil {
		t.Fatalf("verify fail: %v", err)
	}
	if run.StepStatus["check"] != StepPending {
		t.Errorf("verify fail con presupuesto debe reabrir el step (pending), está %q", run.StepStatus["check"])
	}
	if len(reflections) != 1 || reflections[0] != "faltó cubrir el caso X" {
		t.Errorf("debe devolver la reflexión acumulada, obtuve %v", reflections)
	}
	// 'b' sigue bloqueado; 'check' vuelve a estar ready para el reintento.
	ready, _ := engine.WorkflowReady("R")
	if vgContains(ready, "b") {
		t.Error("'b' no debe destrabarse tras un fail")
	}
	if !vgContains(ready, "check") {
		t.Error("'check' debe volver a estar ready para el reintento informado")
	}
}

// Agotar el presupuesto de intentos deja el step failed.
func TestVerifyGateExhaustsToFailed(t *testing.T) {
	engine := vgRun(t, "R", 2) // 2 intentos totales
	defer engine.Close()

	// Intento 1: complete → verify fail → reopen.
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "i1", StepDone, "")
	run, _, _ := engine.VerifyWorkflowStep("R", "check", false, "mal 1", "")
	if run.StepStatus["check"] != StepPending {
		t.Fatalf("tras el 1er fail (max=2) debe reabrir, está %q", run.StepStatus["check"])
	}
	// Intento 2: complete → verify fail → agotado → failed.
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "i2", StepDone, "")
	run, reflections, _ := engine.VerifyWorkflowStep("R", "check", false, "mal 2", "")
	if run.StepStatus["check"] != StepFailed {
		t.Errorf("agotado el presupuesto el step debe quedar failed, está %q", run.StepStatus["check"])
	}
	if len(reflections) != 2 {
		t.Errorf("deben acumularse 2 reflexiones, obtuve %v", reflections)
	}
}

// Un step sin verify se completa done directo (sin verifying).
func TestVerifyGateNoVerifyDirectDone(t *testing.T) {
	engine, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()
	def := WorkflowDef{ID: "vg", Steps: []WorkflowStep{{ID: "a"}}}
	engine.StartWorkflowRun("R", def)
	engine.WorkflowReady("R")
	run, err := engine.CompleteWorkflowStep("R", "a", "ok", StepDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if run.StepStatus["a"] != StepDone {
		t.Errorf("un step sin verify debe completarse done directo, está %q", run.StepStatus["a"])
	}
}

// El candidato queda CONGELADO: un step en verifying no puede re-completarse, porque eso
// pisaría el resultado y el veredicto caería sobre bytes distintos de los revisados.
func TestVerifyGateCandidateFrozen(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	if _, err := engine.CompleteWorkflowStep("R", "check", "candidato original", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CompleteWorkflowStep("R", "check", "candidato pisado", StepDone, ""); err == nil {
		t.Fatal("re-completar un step en verificación debe fallar: el candidato está congelado")
	}
	// El resultado original sobrevive intacto.
	run, _, _ := engine.WorkflowRunStatus("R")
	if run.StepResults["check"] != "candidato original" {
		t.Errorf("el candidato congelado no debe pisarse, quedó %q", run.StepResults["check"])
	}
	if run.StepStatus["check"] != StepVerifying {
		t.Errorf("el step debe seguir en verifying, está %q", run.StepStatus["check"])
	}
}

// Un veredicto que declara un candidato distinto del congelado se RECHAZA sin tocar el estado.
func TestVerifyGateDigestMismatchRejected(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "hecho", StepDone, "")

	_, _, err := engine.VerifyWorkflowStep("R", "check", true, "", candidateDigest("otra cosa"))
	if err == nil {
		t.Fatal("un veredicto sobre un candidato distinto debe rechazarse")
	}
	run, _, _ := engine.WorkflowRunStatus("R")
	if run.StepStatus["check"] != StepVerifying {
		t.Errorf("un veredicto rechazado no debe cambiar el estado, quedó %q", run.StepStatus["check"])
	}
	// Con el digest correcto, el mismo veredicto sí entra.
	if _, _, err := engine.VerifyWorkflowStep("R", "check", true, "", candidateDigest("hecho")); err != nil {
		t.Fatalf("el veredicto con el digest congelado debe pasar: %v", err)
	}
	run, _, _ = engine.WorkflowRunStatus("R")
	if run.StepStatus["check"] != StepDone {
		t.Errorf("tras el pass ligado al candidato el step debe quedar done, está %q", run.StepStatus["check"])
	}
}

// El caso que motivó el cambio: tras un fail el step se reabre y produce OTRO candidato. Un
// veredicto que todavía apunta al candidato viejo no puede darlo por bueno.
func TestVerifyGateStaleVerdictRejectedAfterReopen(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "i1", StepDone, "")
	if _, _, err := engine.VerifyWorkflowStep("R", "check", false, "faltó X", candidateDigest("i1")); err != nil {
		t.Fatalf("fail sobre el candidato vigente: %v", err)
	}
	// Reintento informado: nuevo candidato.
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "i2", StepDone, "")

	if _, _, err := engine.VerifyWorkflowStep("R", "check", true, "", candidateDigest("i1")); err == nil {
		t.Fatal("un veredicto que apunta al candidato viejo debe rechazarse tras el reintento")
	}
	if _, _, err := engine.VerifyWorkflowStep("R", "check", true, "", candidateDigest("i2")); err != nil {
		t.Fatalf("el veredicto sobre el candidato vigente debe pasar: %v", err)
	}
}

// Sin target_digest el gate se comporta como antes (compatibilidad).
func TestVerifyGateDigestOpcional(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	engine.WorkflowReady("R")
	engine.CompleteWorkflowStep("R", "check", "hecho", StepDone, "")
	if _, _, err := engine.VerifyWorkflowStep("R", "check", true, "", ""); err != nil {
		t.Fatalf("sin digest declarado el veredicto debe pasar igual que antes: %v", err)
	}
}

// verify sobre un step que no está en verifying es error.
func TestVerifyGateNonVerifyingErrors(t *testing.T) {
	engine := vgRun(t, "R", 3)
	defer engine.Close()
	// 'a' está done, no verifying.
	if _, _, err := engine.VerifyWorkflowStep("R", "a", true, "", ""); err == nil {
		t.Error("verify sobre un step que no está en verifying debe fallar")
	}
}
