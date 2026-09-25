package main

import (
	"fmt"
	"strings"
	"testing"

	"musubi/internal/config"
)

// EL AVISO DE BAJAR LO DURABLE, en su casa nueva.
//
// Vivía en el hook PreCompact, que disparaba en el instante exacto —justo antes de que el modelo
// escribiera el resumen— y que NUNCA LLEGÓ: ese evento no admite additionalContext. Acá dispara por
// cantidad de turnos, que es un proxy de "está por compactarse", por el hook UserPromptSubmit, que
// sí llega. Las condiciones no negociables del aviso viajaron con él y son los tests de más abajo.

// soloElAviso apaga todas las demás superficies del turno para que lo que salga sea, sin
// ambigüedad, este bloque y no otro.
func soloElAviso(afterTurns int) config.LoopConfig {
	return config.LoopConfig{DurableNudgeAfterTurns: afterTurns}
}

func TestElAvisoDurableNoDisparaAntesDelUmbral(t *testing.T) {
	store := newFakeTurnStore()
	for i := 1; i < 5; i++ {
		if out := buildDurableNudge(store, "s1", 5); out != "" {
			t.Fatalf("turno %d de 5: todavía no debe avisar, obtuve %q", i, out)
		}
	}
}

func TestElAvisoDurableDisparaAlCruzarElUmbral(t *testing.T) {
	store := newFakeTurnStore()
	var out string
	for i := 0; i < 5; i++ {
		out = buildDurableNudge(store, "s1", 5)
	}
	if out == "" {
		t.Fatal("al quinto turno con umbral 5 el aviso tiene que salir")
	}
}

// Una vez por TRAMO: en el turno 5, el 10, el 15 y el 20 con umbral 5. Repetirlo en cada turno lo
// convierte en ruido, y el ruido se aprende a saltear; darlo una sola vez por sesión perdía lo que
// más se usaba de él (ver buildDurableNudge).
//
// Los dos sabotajes tocan la MISMA línea a propósito, y el arnés lo marca como colisión: son los
// dos errores opuestos de esa regla, y sus motivos se leyeron y son distintos (con el primero sale
// en [5]; con el segundo, en todos los turnos desde el 5).
//
// Sabotaje que la hace fallar: volver a «una sola vez por sesión».
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif (turns-afterTurns)%afterTurns != 0 {"
// arnes: a="\tif turns != afterTurns {"
// arnes: colision_ok="TestElAvisoDurableSaleUnaVezPorTramo"
//
// Sabotaje que la hace fallar: avisar en cada turno pasado el umbral.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif (turns-afterTurns)%afterTurns != 0 {"
// arnes: a="\tif false {"
// arnes: colision_ok="TestElAvisoDurableSaleUnaVezPorTramo"
func TestElAvisoDurableSaleUnaVezPorTramo(t *testing.T) {
	store := newFakeTurnStore()
	var turnos []int
	for i := 1; i <= 20; i++ {
		if buildDurableNudge(store, "s1", 5) != "" {
			turnos = append(turnos, i)
		}
	}
	if fmt.Sprint(turnos) != "[5 10 15 20]" {
		t.Errorf("con umbral 5, en 20 turnos el aviso tiene que salir en [5 10 15 20]; salió en %v", turnos)
	}
}

// El contador va con prefijo de sesión. Sin eso SANGRA: una sesión nueva hereda los turnos de la
// anterior y dispara el aviso sin actividad propia. Es el mismo bug que ya se corrigió en el
// recordatorio de captura, y se repite solo si nadie lo afirma.
func TestElAvisoDurableNoSangraEntreSesiones(t *testing.T) {
	store := newFakeTurnStore()
	for i := 0; i < 4; i++ {
		buildDurableNudge(store, "vieja", 5)
	}
	// La sesión nueva arranca de cero: cuatro turnos suyos no alcanzan el umbral de 5.
	for i := 1; i <= 4; i++ {
		if out := buildDurableNudge(store, "nueva", 5); out != "" {
			t.Fatalf("la sesión nueva avisó en su turno %d: heredó el conteo de la anterior (%q)", i, out)
		}
	}
}

func TestElAvisoDurableSeApagaConUmbralNoPositivo(t *testing.T) {
	for _, umbral := range []int{0, -1} {
		store := newFakeTurnStore()
		for i := 0; i < 50; i++ {
			if out := buildDurableNudge(store, "s1", umbral); out != "" {
				t.Fatalf("con umbral %d el aviso debe estar apagado, obtuve %q", umbral, out)
			}
		}
	}
}

// LA CONDICIÓN NO NEGOCIABLE, heredada del hook viejo.
//
// Lo que el agente escribe acá es una síntesis SUYA, no algo que la persona dijo.
// musubi_save_observation sella procedencia `human`: usarlo guardaría una invención del modelo
// como si fuera testimonio. Tiene que ir por musubi_propose_observation, que la deja en cuarentena.
//
// Si alguien "simplifica" el texto y lo manda a save_observation, este test es lo único que avisa.
func TestElAvisoDurableMandaACuarentenaYNoAlLibroMayor(t *testing.T) {
	ctx := durableNudgeText()

	if !strings.Contains(ctx, "musubi_propose_observation") {
		t.Error("el aviso no nombra musubi_propose_observation: sin eso el agente usa el guardado normal")
	}
	if !strings.Contains(ctx, "cuarentena") {
		t.Error("el aviso no explica que va a cuarentena; sin el porqué el agente lo saltea")
	}
	// Nombrar save_observation está bien —y hace falta— sólo si es para PROHIBIRLO.
	if i := strings.Index(ctx, "musubi_save_observation"); i >= 0 {
		alrededor := ctx[max0(i-40):min0(i+40, len(ctx))]
		if !strings.Contains(alrededor, "NO") {
			t.Errorf("se nombra musubi_save_observation sin prohibirlo explícitamente: %q", alrededor)
		}
	}
}

// El freno anti-ruido. Sin esta instrucción el aviso fabrica una síntesis por sesión, y cada una
// entra a la cola de conflictos que alguien tiene que arbitrar a mano.
func TestElAvisoDurableFrenaLaSintesisVacia(t *testing.T) {
	if !strings.Contains(durableNudgeText(), "NO guardes nada") {
		t.Error("falta el freno: si no pasó nada, el aviso no debe empujar a guardar igual")
	}
}

// EL TEST QUE IMPORTA, y el que el hook viejo nunca tuvo: que el aviso llegue al ENVELOPE de
// verdad, no que una función devuelva texto. El hook PreCompact tenía texto perfecto y no llegaba
// a ningún lado.
func TestElAvisoDurableLlegaAlEnvelopeDelTurno(t *testing.T) {
	store := newFakeTurnStore()
	stdin := `{"prompt":"seguimos con lo de antes","session_id":"s1"}`

	var out string
	for i := 0; i < 3; i++ {
		out = turnOutput(store, soloElAviso(3), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(stdin))
	}
	if out == "" {
		t.Fatal("el hook de turno no emitió nada en el turno del umbral")
	}

	evento, ctx := hookAdditionalContext(t, out)
	if evento != "UserPromptSubmit" {
		t.Errorf("el aviso tiene que viajar por UserPromptSubmit (que llega), obtuve %q", evento)
	}
	if !strings.Contains(ctx, "musubi_propose_observation") {
		t.Errorf("el aviso no llegó al additionalContext del turno: %q", ctx)
	}
}

// Se imputa al ledger con su propia superficie, para que el gasto sea rastreable por
// `musubi_tokens` como cualquier otra superficie inyectada.
func TestElAvisoDurableSeContabilizaEnElLedger(t *testing.T) {
	store := newFakeTurnStore()
	stdin := `{"prompt":"seguimos","session_id":"s-42"}`
	for i := 0; i < 3; i++ {
		turnOutput(store, soloElAviso(3), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(stdin))
	}
	if store.ledger[surfaceDurableNudge] <= 0 {
		t.Errorf("esperaba tokens imputados a la superficie %q, el ledger tiene %v",
			surfaceDurableNudge, store.ledger)
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
