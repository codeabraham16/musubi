package memory

import "testing"

// pathObjs devuelve la secuencia de objetos de los hechos del camino (para asserts compactos).
func pathTriples(facts []Fact) []string {
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = f.Subject + "-" + f.Predicate + "-" + f.Object
	}
	return out
}

// Escenario (a): cadena A-B-C-D; el camino A→D son los 3 hechos en orden.
func TestFactPathChain(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	mustFact(t, e, "B", "r", "C")
	mustFact(t, e, "C", "r", "D")

	res, err := e.FactPath("A", "D", 5, "")
	if err != nil {
		t.Fatalf("FactPath: %v", err)
	}
	got := pathTriples(res.Facts)
	want := []string{"A-r-B", "B-r-C", "C-r-D"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("camino A→D esperado %v, obtuve %v", want, got)
	}
}

// Escenario (b): grafo NO dirigido. Hechos (A,r,B) y (C,r,B): el camino A→C usa ambos aunque
// la segunda arista "apunte" a B.
func TestFactPathUndirected(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	mustFact(t, e, "C", "r", "B")

	res, err := e.FactPath("A", "C", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 2 {
		t.Fatalf("esperaba camino de 2 hechos (no dirigido), obtuve %d: %v", res.Count, pathTriples(res.Facts))
	}
	// El primer hecho toca A, el último toca C.
	if !(res.Facts[0].Subject == "A" || res.Facts[0].Object == "A") {
		t.Errorf("el primer hecho debe tocar A, obtuve %+v", res.Facts[0])
	}
	if !(res.Facts[1].Subject == "C" || res.Facts[1].Object == "C") {
		t.Errorf("el último hecho debe tocar C, obtuve %+v", res.Facts[1])
	}
}

// Escenario (c): componentes desconectadas → sin camino.
func TestFactPathNoPath(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	mustFact(t, e, "X", "r", "Y")

	res, err := e.FactPath("A", "X", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 0 {
		t.Errorf("entidades desconectadas: esperaba camino vacío, obtuve %v", pathTriples(res.Facts))
	}
}

// Escenario (d): misma entidad → camino trivial vacío.
func TestFactPathSameEntity(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	res, err := e.FactPath("A", "A", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 0 {
		t.Errorf("from==to debe dar camino vacío, obtuve %v", pathTriples(res.Facts))
	}
}

// Escenario (e): entidad destino inexistente → vacío sin error.
func TestFactPathUnknownEntity(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	res, err := e.FactPath("A", "Zzz", 5, "")
	if err != nil {
		t.Fatalf("entidad inexistente no debe fallar: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("destino inexistente debe dar camino vacío, obtuve %v", pathTriples(res.Facts))
	}
}

// Escenario (f): con un atajo directo, se devuelve el camino más corto (1 arista).
func TestFactPathShortest(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	mustFact(t, e, "B", "r", "C")
	mustFact(t, e, "A", "r", "C") // atajo directo

	res, err := e.FactPath("A", "C", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 {
		t.Errorf("con atajo directo A-C el camino más corto es 1 hecho, obtuve %d: %v", res.Count, pathTriples(res.Facts))
	}
}

// maxHops acota la longitud: una cadena más larga que maxHops no se encuentra.
func TestFactPathMaxHops(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "r", "B")
	mustFact(t, e, "B", "r", "C")
	mustFact(t, e, "C", "r", "D")

	// A→D son 3 aristas; con maxHops=2 no debe encontrarse.
	res, err := e.FactPath("A", "D", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 0 {
		t.Errorf("con maxHops=2 no debe hallarse el camino de 3 aristas, obtuve %v", pathTriples(res.Facts))
	}
}

// Escenario (g): camino point-in-time. Un hecho invalidado antes de as_of reaparece y habilita
// un camino que hoy no existe.
func TestFactPathPointInTime(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	if _, err := e.SaveFact("Ana", "works_at", "Acme", "2020-01-01", sv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("Acme", "located_in", "NYC", "2020-01-01", nil); err != nil {
		t.Fatal(err)
	}
	// Ana cambia de trabajo: invalida Ana-works_at-Acme (single-valued).
	if _, err := e.SaveFact("Ana", "works_at", "Globex", "2023-01-01", sv); err != nil {
		t.Fatal(err)
	}

	// Hoy: Ana ya no trabaja en Acme → no hay camino Ana→NYC (Globex no está en NYC).
	cur, err := e.FactPath("Ana", "NYC", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if cur.Count != 0 {
		t.Errorf("hoy no debe existir camino Ana→NYC, obtuve %v", pathTriples(cur.Facts))
	}
	// En 2021: Ana-works_at-Acme era vigente → camino Ana→Acme→NYC.
	past, err := e.FactPath("Ana", "NYC", 5, "2021-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if past.Count != 2 {
		t.Fatalf("as_of=2021 debe dar camino de 2 hechos (Ana→Acme→NYC), obtuve %d: %v", past.Count, pathTriples(past.Facts))
	}
}
