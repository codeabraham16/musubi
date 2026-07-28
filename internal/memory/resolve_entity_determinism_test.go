package memory

import "testing"

// TestResolveEntityNameDeterministicTie: ante dos entidades con IDÉNTICA similitud de trigramas al
// nombre propuesto, ResolveEntityName elige siempre la lexicográficamente MENOR, de forma estable
// (auditoría v0.98.0: sin ORDER BY el empate se resolvía por orden físico de filas). "tests" y
// "testx" tienen la misma Similarity a "test" (0.667); se siembran en orden inverso para probar que
// gana el ORDER BY name, no el orden de inserción/rowid.
func TestResolveEntityNameDeterministicTie(t *testing.T) {
	e := newTestEngine(t)
	// Crear las entidades "testx" y "tests" (en ese orden) vía aristas; los objetos z* no interfieren.
	if _, err := e.SaveFactFromSourced("", "testx", "rel", "z2", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFromSourced("", "tests", "rel", "z1", "", "agent", nil); err != nil {
		t.Fatal(err)
	}

	got, matched, err := e.ResolveEntityName("test", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if !matched || got != "tests" {
		t.Fatalf("empate debe resolver al nombre lexicográficamente menor 'tests'; got=(%q,%v)", got, matched)
	}
	// Estable en repetición (no depende de un orden físico volátil).
	for i := 0; i < 5; i++ {
		g, _, err := e.ResolveEntityName("test", 0.5)
		if err != nil {
			t.Fatal(err)
		}
		if g != got {
			t.Errorf("resolución no determinista entre corridas: %q vs %q", g, got)
		}
	}
}
