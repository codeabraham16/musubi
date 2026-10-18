package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"musubi/internal/censo"
)

// toolcensus_test.go cubre checkToolCensus (A3): el check del doctor que confirma el CENSO DE TOOLS
// VIVAS contra el ledger de uso. Las invariantes que sella son las del diseño (R5): un cero en una
// tool que DEBE invocarse se reporta como HIPÓTESIS sin volcar el rollup, y un cero en una tool
// administrativa (cero esperado por diseño) NO se reporta.

// E3 + E4: una DebeInvocarse en cero sale como hipótesis en el Message, PERO el Status sigue `ok`
// (nunca warning/error, así buildHealthContext no lo convierte en un nag de arranque); y una
// Administrativa en cero —cero correcto por diseño— NO aparece. Además, una DebeInvocarse CON uso
// tampoco aparece: la hipótesis es sólo para las silenciosas.
func TestCheckToolCensusHipotesisSinSentencia(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	// Sólo musubi_whoami tiene actividad. El resto de las DebeInvocarse quedan en cero —entre ellas
	// save_fact—, y fleet_approve (Administrativa) también, pero por motivos opuestos.
	//
	// Se usa whoami y NO recall a propósito: el Message lista las silenciosas con Contains, y
	// "musubi_recall" es SUBCADENA de musubi_recall_code / musubi_recall_facts (que SÍ estarían en la
	// lista), así que la aserción negativa daría un falso positivo. whoami no es prefijo de ninguna.
	if err := e.RecordToolInvocations(ctx, []ToolInvocation{
		{Tool: "musubi_whoami", Outcome: OutcomeOK, Duration: 5 * time.Millisecond},
		{Tool: "musubi_whoami", Outcome: OutcomeOK, Duration: 7 * time.Millisecond},
	}); err != nil {
		t.Fatalf("sembrar ledger: %v", err)
	}

	r := checkToolCensus(e)

	if r.Status != "ok" {
		t.Errorf("E3: una hipótesis NO debe volcar el Status; esperaba ok, obtuve %q (%s)", r.Status, r.Message)
	}
	if r.Unmeasured {
		t.Errorf("E3: con historia en el ledger el check NO es Unmeasured; Message=%q", r.Message)
	}
	if !strings.Contains(r.Message, "hipótesis") {
		t.Errorf("E3: con una DebeInvocarse en cero el Message debe enmarcar una hipótesis, obtuve %q", r.Message)
	}
	if !strings.Contains(r.Message, "musubi_save_fact") {
		t.Errorf("E3: save_fact (DebeInvocarse, 0 invocaciones) debe aparecer como hipótesis, obtuve %q", r.Message)
	}
	if strings.Contains(r.Message, "musubi_fleet_approve") {
		t.Errorf("E4: fleet_approve (Administrativa, cero esperado) NO debe reportarse, obtuve %q", r.Message)
	}
	if strings.Contains(r.Message, "musubi_whoami") {
		t.Errorf("una tool CON uso no es una hipótesis; musubi_whoami no debería estar en %q", r.Message)
	}
}

// Sin una sola invocación en la ventana: no hay historia para sacar hipótesis. ok + Unmeasured, no
// un falso verde "todo se usa" ni un falso rojo "todo está muerto".
func TestCheckToolCensusSinHistoriaEsUnmeasured(t *testing.T) {
	e := newTestEngine(t) // ledger recién nacido: tool_invocations vacío

	r := checkToolCensus(e)

	if r.Status != "ok" {
		t.Errorf("sin historia el Status debe ser ok, obtuve %q", r.Status)
	}
	if !r.Unmeasured {
		t.Errorf("sin historia el check debe marcarse Unmeasured, Message=%q", r.Message)
	}
	if strings.Contains(r.Message, "hipótesis") {
		t.Errorf("sin historia no se afirma ninguna hipótesis, obtuve %q", r.Message)
	}
}

// La contracara de E3: con TODAS las DebeInvocarse ejercidas, el check reporta limpio —sin hipótesis—
// y no es Unmeasured. Guarda contra un futuro bug donde el check liste algo siempre.
func TestCheckToolCensusTodasUsadasReportaLimpio(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	lote := []ToolInvocation{}
	for _, name := range censo.DebenInvocarse() {
		lote = append(lote, ToolInvocation{Tool: name, Outcome: OutcomeOK, Duration: time.Millisecond})
	}
	if err := e.RecordToolInvocations(ctx, lote); err != nil {
		t.Fatalf("sembrar ledger: %v", err)
	}

	r := checkToolCensus(e)

	if r.Status != "ok" || r.Unmeasured {
		t.Errorf("con historia completa esperaba ok medido, obtuve status=%q unmeasured=%v", r.Status, r.Unmeasured)
	}
	if strings.Contains(r.Message, "hipótesis") {
		t.Errorf("con todas las DebeInvocarse ejercidas no debe haber hipótesis, obtuve %q", r.Message)
	}
	if !strings.Contains(r.Message, "registran uso") {
		t.Errorf("esperaba el Message de censo limpio, obtuve %q", r.Message)
	}
}
