package memory

import (
	"math"
	"testing"
)

// R8 (escenario e): effectiveHalfLife nunca encoge la vida media (clamp defensivo) y es
// monótona creciente en accessCount cuando K>0.
func TestEffectiveHalfLifeClampAndMonotonic(t *testing.T) {
	const hl = 30.0
	// K=0 → vida media fija, sin importar los accesos.
	if got := effectiveHalfLife(hl, 100, 0); got != hl {
		t.Errorf("K=0 debe dar vida media fija %v, obtuve %v", hl, got)
	}
	// access<=0 → vida media fija.
	if got := effectiveHalfLife(hl, 0, 0.5); got != hl {
		t.Errorf("access=0 debe dar vida media fija %v, obtuve %v", hl, got)
	}
	// K<0 → no encoge (clamp).
	if got := effectiveHalfLife(hl, 50, -1); got != hl {
		t.Errorf("K<0 no debe encoger la vida media, obtuve %v", got)
	}
	// Monotonía: más accesos → vida media efectiva estrictamente mayor (K>0).
	prev := hl
	for _, acc := range []int{1, 5, 20, 100, 500} {
		got := effectiveHalfLife(hl, acc, 0.5)
		if got <= prev {
			t.Errorf("effHL debe crecer con accessCount: acc=%d dio %v (prev %v)", acc, got, prev)
		}
		prev = got
	}
}

// R2/R1 (escenario a): con reinforcementK=0 y typeWeight=1, salience reproduce EXACTAMENTE
// la fórmula previa importance*freq*recency (equivalencia byte-idéntica pre-B3).
func TestSalienceEquivalenceK0(t *testing.T) {
	imp, acc, age, hl := 2.0, 7, 45.0, 30.0
	freq := 1 + math.Log(1+float64(acc))
	recency := math.Pow(0.5, age/hl)
	want := imp * freq * recency // fórmula histórica (typeWeight=1, sin refuerzo)
	got := salience(imp, acc, age, hl, 1.0, 0)
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("salience(K=0,typeWeight=1) debe igualar la fórmula previa: got %v, want %v", got, want)
	}
}

// R1/R3: aislar el REFUERZO — a igual acceso (mismo freq), K>0 da mayor saliencia que K=0
// (sólo cambia la vida media efectiva → recencia más alta).
func TestSalienceReinforcementRaisesSalience(t *testing.T) {
	base := salience(1, 10, 60, 30, 1.0, 0)
	reinforced := salience(1, 10, 60, 30, 1.0, 0.5)
	if !(reinforced > base) {
		t.Errorf("con acceso>0, K=0.5 debe dar más saliencia que K=0: base=%v reforzada=%v", base, reinforced)
	}
}

// R3 (escenario b) end-to-end: con refuerzo activo, una memoria CALIENTE sobrevive el
// archivado que se lleva a una FRÍA de misma importancia/edad.
func TestDecayHotSurvivesColdWithReinforcement(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("cold", "t", "memoria fria poco usada", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("hot", "t", "memoria caliente muy usada", nil); err != nil {
		t.Fatal(err)
	}
	// Ambas a 60 días; 'hot' con muchos accesos.
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-60 days')`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET access_count = 100 WHERE id='hot'`); err != nil {
		t.Fatal(err)
	}

	// Con refuerzo (K=0.5) y MinSalience 0.5: cold (~0.25) se archiva, hot no.
	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.5, MinAgeDays: 14, ReinforcementK: 0.5})
	if err != nil {
		t.Fatalf("Decay: %v", err)
	}
	arch := func(id string) int {
		var a int
		if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id=?`, id).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	if arch("cold") != 1 {
		t.Error("la memoria fría debe archivarse")
	}
	if arch("hot") != 0 {
		t.Error("la memoria caliente (reforzada por acceso) NO debe archivarse")
	}
	if res.Archived != 1 {
		t.Errorf("esperaba archivar sólo la fría, archivé %d", res.Archived)
	}
}

// Sin refuerzo (K=0), la misma memoria caliente SÍ se archivaría a un umbral que su freq no
// alcanza a salvar: confirma que el rescate viene del refuerzo, no sólo de la frecuencia.
func TestDecayColdArchivedWithoutReinforcement(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("hot", "t", "memoria caliente", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-300 days'), access_count = 3 WHERE id='hot'`); err != nil {
		t.Fatal(err)
	}
	// A 300 días con K=0: recency=0.5^10≈9.8e-4; freq=1+ln(4)≈2.39 → salience≈2.3e-3 < 0.2.
	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14, ReinforcementK: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res.Archived != 1 {
		t.Errorf("sin refuerzo (K=0) una memoria muy vieja se archiva pese a algunos accesos, archivé %d", res.Archived)
	}
}
