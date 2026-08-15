package main

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

// precompactSpy captura lo que el hook imputa al ledger, sin tocar la DB.
type precompactSpy struct {
	sessionID string
	surface   string
	tokens    int
	calls     int
}

func (s *precompactSpy) LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error) {
	s.sessionID, s.surface, s.tokens = sessionID, surface, tokens
	s.calls++
	return memory.TokenLedger{}, nil
}

func TestPrecompactEmiteElEnvelopeDelHook(t *testing.T) {
	out := precompactOutput(nil, strings.NewReader(`{"session_id":"s1","trigger":"auto"}`))
	if out == "" {
		t.Fatal("el hook no emitió nada; la compactación es justo cuando tiene que hablar")
	}
	event, ctx := hookAdditionalContext(t, out)
	if event != "PreCompact" {
		t.Errorf("esperaba hookEventName PreCompact, obtuve %q", event)
	}
	if strings.TrimSpace(ctx) == "" {
		t.Error("additionalContext vacío")
	}
}

// LA CONDICIÓN NO NEGOCIABLE de este hook, y la razón por la que existe el test.
//
// Lo que el agente escribe al compactar es una síntesis SUYA, no algo que la persona dijo.
// musubi_save_observation sella procedencia `human`: usarlo acá guardaría una invención del modelo
// como si fuera testimonio, y justo en el momento de mayor pérdida de contexto, que es cuando peor
// se sintetiza. Tiene que ir por musubi_propose_observation, que la deja en cuarentena.
//
// Si alguien "simplifica" el texto y lo manda a save_observation, este test es lo único que avisa.
func TestPrecompactMandaACuarentenaYNoAlLibroMayor(t *testing.T) {
	_, ctx := hookAdditionalContext(t, precompactOutput(nil, strings.NewReader(`{"session_id":"s1"}`)))

	if !strings.Contains(ctx, "musubi_propose_observation") {
		t.Error("el aviso no nombra musubi_propose_observation: sin eso el agente usa el guardado normal")
	}
	if !strings.Contains(ctx, "cuarentena") {
		t.Error("el aviso no explica que va a cuarentena; sin el porqué el agente lo saltea")
	}
	// Nombrar save_observation está bien —y hace falta— sólo si es para PROHIBIRLO.
	if i := strings.Index(ctx, "musubi_save_observation"); i >= 0 {
		alrededor := ctx[max0(i-40) : min0(i+40, len(ctx))]
		if !strings.Contains(alrededor, "NO") {
			t.Errorf("se nombra musubi_save_observation sin prohibirlo explícitamente: %q", alrededor)
		}
	}
}

// El freno anti-ruido. Sin esta instrucción el hook fabrica una síntesis por compactación, y cada
// una entra a la cola de conflictos que alguien tiene que arbitrar a mano.
func TestPrecompactFrenaLaSintesisVacia(t *testing.T) {
	_, ctx := hookAdditionalContext(t, precompactOutput(nil, strings.NewReader(`{"session_id":"s1"}`)))
	if !strings.Contains(ctx, "NO guardes nada") {
		t.Error("falta el freno: si no pasó nada, el hook no debe empujar a guardar igual")
	}
}

func TestPrecompactSeContabilizaEnElLedger(t *testing.T) {
	spy := &precompactSpy{}
	precompactOutput(spy, strings.NewReader(`{"session_id":"s-42"}`))

	if spy.calls != 1 {
		t.Fatalf("esperaba 1 imputación al ledger, obtuve %d", spy.calls)
	}
	if spy.sessionID != "s-42" {
		t.Errorf("el ledger debe usar el session_id del hook, obtuve %q", spy.sessionID)
	}
	if spy.surface != surfacePrecompact {
		t.Errorf("superficie esperada %q, obtuve %q", surfacePrecompact, spy.surface)
	}
	if spy.tokens <= 0 {
		t.Errorf("el bloque tiene texto: debe imputar tokens > 0, obtuve %d", spy.tokens)
	}
}

// Un stdin roto no puede tumbar la sesión del usuario: se degrada a session_id vacío y sigue.
func TestPrecompactToleraStdinInvalido(t *testing.T) {
	for _, in := range []string{"", "no soy json", "{", `{"session_id":123}`} {
		out := precompactOutput(nil, strings.NewReader(in))
		if out == "" {
			t.Errorf("con stdin %q el hook debe seguir avisando igual", in)
		}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func min0(a, b int) int {
	if a < b {
		return a
	}
	return b
}
