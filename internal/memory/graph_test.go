package memory

import "testing"

func countRows(t *testing.T, e *DbEngine, table string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s error: %v", table, err)
	}
	return n
}

func TestSaveFactDedupEntitiesAndRelations(t *testing.T) {
	e := newTestEngine(t)

	r, err := e.SaveFact("Alice", "works_at", "ACME", "", nil)
	if err != nil {
		t.Fatalf("SaveFact error: %v", err)
	}
	if !r.Created {
		t.Error("la primera relación debería ser nueva")
	}
	// Misma relación otra vez -> no duplica.
	r, err = e.SaveFact("Alice", "works_at", "ACME", "", nil)
	if err != nil {
		t.Fatalf("SaveFact error: %v", err)
	}
	if r.Created {
		t.Error("relación repetida no debería crearse de nuevo")
	}
	// Nueva relación reusando 'Alice'.
	if _, err := e.SaveFact("Alice", "knows", "Bob", "", nil); err != nil {
		t.Fatalf("SaveFact error: %v", err)
	}

	if n := countRows(t, e, "entities"); n != 3 {
		t.Errorf("esperaba 3 entidades (Alice, ACME, Bob), obtuve %d", n)
	}
	if n := countRows(t, e, "relations"); n != 2 {
		t.Errorf("esperaba 2 relaciones, obtuve %d", n)
	}
}

func TestSaveFactEntityCaseInsensitive(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SaveFact("Alice", "likes", "Go", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("  alice ", "uses", "SQLite", "", nil); err != nil {
		t.Fatal(err)
	}
	// 'Alice' y 'alice' son la misma entidad: Alice, Go, SQLite = 3.
	if n := countRows(t, e, "entities"); n != 3 {
		t.Errorf("esperaba dedup case-insensitive (3 entidades), obtuve %d", n)
	}
}

func TestRecallFactsOneHop(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "Alice", "works_at", "ACME")
	mustFact(t, e, "Alice", "knows", "Bob")
	mustFact(t, e, "ACME", "located_in", "NYC")

	res, err := e.RecallFacts("Alice", 1, 50, "", "")
	if err != nil {
		t.Fatalf("RecallFacts error: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("a 1 hop esperaba 2 hechos (sin ACME->NYC), obtuve %d: %+v", res.Count, res.Facts)
	}
}

func TestRecallFactsTwoHops(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "Alice", "works_at", "ACME")
	mustFact(t, e, "Alice", "knows", "Bob")
	mustFact(t, e, "ACME", "located_in", "NYC")

	res, err := e.RecallFacts("Alice", 2, 50, "", "")
	if err != nil {
		t.Fatalf("RecallFacts error: %v", err)
	}
	if res.Count != 3 {
		t.Errorf("a 2 hops esperaba 3 hechos (incluye ACME->NYC), obtuve %d: %+v", res.Count, res.Facts)
	}
}

func TestRecallFactsUnknownEntity(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "Alice", "knows", "Bob")
	res, err := e.RecallFacts("Zzz", 2, 50, "", "")
	if err != nil {
		t.Fatalf("RecallFacts error: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("entidad inexistente debería dar 0 hechos, obtuve %d", res.Count)
	}
}

func TestRecallFactsRespectsMaxFacts(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "Alice", "knows", "Bob")
	mustFact(t, e, "Alice", "knows", "Carol")
	mustFact(t, e, "Alice", "knows", "Dave")

	res, err := e.RecallFacts("Alice", 1, 1, "", "")
	if err != nil {
		t.Fatalf("RecallFacts error: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("maxFacts=1 debería limitar a 1 hecho, obtuve %d", res.Count)
	}
}

func mustFact(t *testing.T, e *DbEngine, s, p, o string) {
	t.Helper()
	if _, err := e.SaveFact(s, p, o, "", nil); err != nil {
		t.Fatalf("SaveFact(%s,%s,%s) error: %v", s, p, o, err)
	}
}

// --- Invalidación bi-temporal (cardinalidad + point-in-time) ---

// Escenario: predicado single-valued invalida al anterior.
func TestSaveFactSingleValuedInvalidates(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	if _, err := e.SaveFact("Ana", "works_at", "Acme", "", sv); err != nil {
		t.Fatal(err)
	}
	r, err := e.SaveFact("Ana", "works_at", "Globex", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r.Invalidated != 1 {
		t.Errorf("guardar Globex (single-valued) debe invalidar 1 hecho (Acme), obtuve %d", r.Invalidated)
	}
	res, _ := e.RecallFacts("Ana", 1, 50, "", "")
	if len(res.Facts) != 1 || res.Facts[0].Object != "Globex" {
		t.Errorf("la verdad actual debe ser sólo Globex, obtuve %+v", res.Facts)
	}
}

// Escenario: predicado many-valued NO invalida (ambos vivos).
func TestSaveFactManyValuedDoesNotInvalidate(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"} // 'knows' NO está en el set → many-valued
	if _, err := e.SaveFact("Ana", "knows", "Beto", "", sv); err != nil {
		t.Fatal(err)
	}
	r, err := e.SaveFact("Ana", "knows", "Carla", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r.Invalidated != 0 {
		t.Errorf("un predicado many-valued no debe invalidar nada, obtuve %d", r.Invalidated)
	}
	res, _ := e.RecallFacts("Ana", 1, 50, "", "")
	if len(res.Facts) != 2 {
		t.Errorf("ambos hechos 'knows' deben seguir vivos, obtuve %+v", res.Facts)
	}
}

// Escenario: consulta point-in-time con as_of.
func TestRecallFactsAsOfPointInTime(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	// Acme válido desde 2020; Globex desde 2023 (invalida Acme al guardarse).
	if _, err := e.SaveFact("Ana", "works_at", "Acme", "2020-01-01", sv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("Ana", "works_at", "Globex", "2023-01-01", sv); err != nil {
		t.Fatal(err)
	}
	// Verdad actual: sólo Globex.
	cur, _ := e.RecallFacts("Ana", 1, 50, "", "")
	if len(cur.Facts) != 1 || cur.Facts[0].Object != "Globex" {
		t.Fatalf("verdad actual debe ser sólo Globex, obtuve %+v", cur.Facts)
	}
	// Point-in-time en 2021: Acme era la verdad (Globex aún no empezaba).
	past, _ := e.RecallFacts("Ana", 1, 50, "2021-01-01", "")
	if len(past.Facts) != 1 || past.Facts[0].Object != "Acme" {
		t.Fatalf("as_of=2021 debe devolver Acme (point-in-time), obtuve %+v", past.Facts)
	}
}

// Escenario: revivir un triplete invalidado (y que invalide al que lo había reemplazado).
func TestSaveFactReviveInvalidated(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	if _, err := e.SaveFact("Ana", "works_at", "Acme", "", sv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("Ana", "works_at", "Globex", "", sv); err != nil { // invalida Acme
		t.Fatal(err)
	}
	// Re-afirmar Acme: revive (no crea fila) e invalida Globex por cardinalidad.
	r, err := e.SaveFact("Ana", "works_at", "Acme", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r.Created {
		t.Error("re-afirmar un triplete existente no debe crear una fila nueva")
	}
	if r.Invalidated != 1 {
		t.Errorf("revivir Acme debe invalidar Globex (1), obtuve %d", r.Invalidated)
	}
	res, _ := e.RecallFacts("Ana", 1, 50, "", "")
	if len(res.Facts) != 1 || res.Facts[0].Object != "Acme" {
		t.Errorf("la verdad actual debe ser Acme revivido, obtuve %+v", res.Facts)
	}
}

// as_of mal formado degrada a verdad actual (no falla).
func TestRecallFactsAsOfInvalidDegradesToCurrent(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	e.SaveFact("Ana", "works_at", "Acme", "", sv)
	e.SaveFact("Ana", "works_at", "Globex", "", sv)
	res, err := e.RecallFacts("Ana", 1, 50, "no-es-una-fecha", "")
	if err != nil {
		t.Fatalf("as_of inválido no debe fallar: %v", err)
	}
	if len(res.Facts) != 1 || res.Facts[0].Object != "Globex" {
		t.Errorf("as_of inválido debe degradar a verdad actual (Globex), obtuve %+v", res.Facts)
	}
}
