package memory

import (
	"strings"
	"testing"
)

// insertRun mete un run directo en la tabla con una antigüedad dada, que es la única forma de
// simular abandono sin esperar dos semanas.
func insertRun(t *testing.T, e *DbEngine, id, status string, diasQuieto int) {
	t.Helper()
	_, err := e.db.Exec(`
		INSERT INTO workflow_runs (run_id, workflow_id, definition, status, step_status, step_results, step_iters, created_at, updated_at)
		VALUES (?, ?, '{}', ?, '{}', '{}', '{}', datetime('now','-'||?||' days'), datetime('now','-'||?||' days'))`,
		id, id, status, diasQuieto, diasQuieto)
	if err != nil {
		t.Fatalf("insertar run %s: %v", id, err)
	}
}

func TestSinRunsAbandonadosElCheckDaOK(t *testing.T) {
	e := newTestEngine(t)
	if got := checkAbandonedRuns(e).Status; got != "ok" {
		t.Errorf("una base sin runs debería dar ok, dio %q", got)
	}
}

// EL CASO QUE MOTIVÓ EL CHECK: 16 runs SDD quedaron en 'running' desde julio porque la sesión que
// los conducía terminó y nada los cerró. Eran invisibles: ningún check los miraba.
func TestRunQuietoHaceSemanasSeSeñala(t *testing.T) {
	e := newTestEngine(t)
	insertRun(t, e, "sdd-brain-dashboard", RunRunning, 42)

	r := checkAbandonedRuns(e)
	if r.Status != "warning" {
		t.Fatalf("un run quieto hace 42 días tiene que avisar, dio %q (%s)", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "1 run") {
		t.Errorf("el mensaje no dice cuántos son: %q", r.Message)
	}
}

// LA CONTRACARA, y es la que protege el diferencial del motor: Musubi NO ejecuta los steps, así que
// un run puede estar legítimamente quieto días esperando a su agente o a un gate humano. Un umbral
// corto convertiría "me fui el fin de semana" en una alarma, y una alarma que salta siempre se
// aprende a ignorar.
func TestRunQuietoPocosDiasNoEsAbandono(t *testing.T) {
	e := newTestEngine(t)
	insertRun(t, e, "sdd-en-curso", RunRunning, 3)

	if r := checkAbandonedRuns(e); r.Status != "ok" {
		t.Errorf("3 días quieto es una pausa normal, no abandono: %q (%s)", r.Status, r.Message)
	}
}

// Un run TERMINAL viejo es historia, no abandono. Señalarlo llenaría el doctor de ruido que crece
// con el uso normal: cuanto más se usa el motor, más runs cerrados hay.
func TestRunTerminalViejoNoSeSeñala(t *testing.T) {
	e := newTestEngine(t)
	for _, st := range []string{RunDone, RunFailed, RunAborted, RunCompensated} {
		insertRun(t, e, "viejo-"+st, st, 90)
	}
	if r := checkAbandonedRuns(e); r.Status != "ok" {
		t.Errorf("los runs cerrados hace 90 días son historia, no hallazgos: %q (%s)", r.Status, r.Message)
	}
}

// EL TEST QUE CUIDA LA DECISIÓN DE DISEÑO. Este check REPORTA y no actúa: cerrar un run es una
// decisión con dueño. Si alguien le agrega un `apply` y lo mete en el auto-heal, un corte largo de
// sesión pasaría a MATAR runs sin que nadie lo pida — y el motor perdería lo único que lo
// distingue, que es sobrevivir a que se apague la sesión.
func TestElCheckDeRunsAbandonadosNoSeAutoRepara(t *testing.T) {
	e := newTestEngine(t)
	if autoHealCodes["abandoned_runs"] {
		t.Fatal("abandoned_runs entró al auto-heal: un run abandonado se CIERRA por decisión, no por timer")
	}
	for _, c := range e.doctorChecks() {
		if c.code != "abandoned_runs" {
			continue
		}
		if c.apply != nil || c.count != nil {
			t.Error("abandoned_runs tiene reparación mecánica: cerrar un run no es reparar, es decidir")
		}
		return
	}
	t.Error("abandoned_runs no está registrado en doctorChecks")
}
