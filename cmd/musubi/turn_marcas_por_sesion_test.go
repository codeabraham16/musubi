package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// HALLAZGO (revisión ronda 2, lente producción): la alerta de presupuesto promete «una sola vez por
// sesión», pero la marca de «ya avisado» es UNA casilla (`loop_budget_alerted`) con UN session_id.
// Con la casilla única del ledger viejo el total de cada sesión se reiniciaba al alternar, así que
// dos terminales rara vez estaban pasadas a la vez. Con la cuenta por sesión el total ya no baja
// nunca: una vez que A y B cruzaron el techo (8000 por defecto) quedan pasadas para siempre, y cada
// turno de una después de un turno de la otra encuentra la casilla con el id ajeno y vuelve a avisar.
// Lo mismo le pasa a `loop_brevity_injected` con brevity_mode=auto.
func TestAlertaDePresupuestoUnaVezPorSesionConDosTerminales(t *testing.T) {
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	eng.LedgerAdd("A", "turn_recall", 9000)
	eng.LedgerAdd("B", "turn_recall", 9000)

	if msg := buildBudgetAlert(eng, "A", 8000); msg == "" {
		t.Fatal("A cruzó el techo y tenía que recibir la alerta la primera vez")
	}
	if msg := buildBudgetAlert(eng, "B", 8000); msg == "" {
		t.Fatal("B cruzó el techo y tenía que recibir la alerta la primera vez")
	}
	repetidas := 0
	for turno := 0; turno < 3; turno++ {
		if buildBudgetAlert(eng, "A", 8000) != "" {
			repetidas++
		}
		if buildBudgetAlert(eng, "B", 8000) != "" {
			repetidas++
		}
	}
	if repetidas != 0 {
		t.Errorf("la alerta «una vez por sesión» se repitió %d veces en 6 turnos alternados", repetidas)
	}

	brevedades := 0
	for turno := 0; turno < 3; turno++ {
		if buildBrevityNudge(eng, "A", "auto", 8000) != "" {
			brevedades++
		}
		if buildBrevityNudge(eng, "B", "auto", 8000) != "" {
			brevedades++
		}
	}
	if brevedades != 2 {
		t.Errorf("la brevedad automática se inyectó %d veces en 6 turnos alternados; una por sesión son 2", brevedades)
	}
}

// La marca por sesión está acotada: sin tope, un valor de `meta` crecería con cada sesión de la vida
// del proyecto.
func TestLasMarcasPorSesionEstanAcotadas(t *testing.T) {
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	for i := 0; i < maxMarcasPorSesion+10; i++ {
		if !marcarUnaVezPorSesion(eng, metaBudgetAlerted, fmt.Sprintf("s-%03d", i), "1") {
			t.Fatalf("la sesión s-%03d no tenía marca y la función dijo que sí", i)
		}
	}
	raw, _, _ := eng.GetMeta(metaBudgetAlerted)
	var m marcasPorSesion
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Valor) != maxMarcasPorSesion || len(m.Orden) != maxMarcasPorSesion {
		t.Fatalf("la marca guarda %d sesiones (orden %d); el tope es %d", len(m.Valor), len(m.Orden), maxMarcasPorSesion)
	}
	if _, sigue := m.Valor["s-000"]; sigue {
		t.Error("se tenía que olvidar la sesión más vieja, no otra")
	}
}
