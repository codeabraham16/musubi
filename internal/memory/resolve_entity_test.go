package memory

import "testing"

// TestResolveEntityName cubre la resolución de entidades del pilar Cognición · F4 (Jaccard de
// trigramas, model-free): el match exacto gana, un nombre similar por encima del umbral se
// canonicaliza, uno distinto por debajo se respeta, y el grafo vacío no resuelve nada.
func TestResolveEntityName(t *testing.T) {
	e := newTestEngine(t)

	// Semillar entidades reales creando aristas (upsert de entities): "potion" y "wildcat".
	if _, err := e.SaveFactFromSourced("", "Potion Roja", "usa", "wildcat", "", "agent", nil); err != nil {
		t.Fatal(err)
	}

	// R3: match exacto por norma gana y devuelve el nombre ALMACENADO (canónico).
	if got, matched, err := e.ResolveEntityName("potion roja", 0.7); err != nil {
		t.Fatal(err)
	} else if !matched || got != "Potion Roja" {
		t.Errorf("match exacto: got=(%q,%v), esperaba (\"Potion Roja\", true)", got, matched)
	}

	// R2: nombre similar por encima del umbral se canonicaliza (Similarity(wildcats,wildcat)=0.8).
	if got, matched, err := e.ResolveEntityName("wildcats", 0.7); err != nil {
		t.Fatal(err)
	} else if !matched || got != "wildcat" {
		t.Errorf("similar≥thr: got=(%q,%v), esperaba (\"wildcat\", true)", got, matched)
	}

	// R2 (negativo): nombre distinto por debajo del umbral se respeta.
	if got, matched, err := e.ResolveEntityName("elefante", 0.7); err != nil {
		t.Fatal(err)
	} else if matched || got != "elefante" {
		t.Errorf("distinto<thr: got=(%q,%v), esperaba (\"elefante\", false)", got, matched)
	}
}

// TestResolveEntityNameThresholdZeroAndEmpty: umbral<=0 es no-op aunque exista el match exacto;
// grafo vacío no resuelve.
func TestResolveEntityNameThresholdZeroAndEmpty(t *testing.T) {
	e := newTestEngine(t)

	// R4: grafo vacío ⇒ (name, false).
	if got, matched, err := e.ResolveEntityName("cualquiera", 0.7); err != nil {
		t.Fatal(err)
	} else if matched || got != "cualquiera" {
		t.Errorf("grafo vacío: got=(%q,%v), esperaba (\"cualquiera\", false)", got, matched)
	}

	if _, err := e.SaveFactFromSourced("", "potion", "usa", "mana", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	// R1: umbral 0 ⇒ no-op, incluso con el match exacto presente.
	for _, thr := range []float64{0, -1} {
		if got, matched, err := e.ResolveEntityName("potion", thr); err != nil {
			t.Fatal(err)
		} else if matched || got != "potion" {
			t.Errorf("thr=%v no-op: got=(%q,%v), esperaba (\"potion\", false)", thr, got, matched)
		}
	}
}
