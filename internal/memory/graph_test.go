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

	r, err := e.SaveFact("Alice", "works_at", "ACME")
	if err != nil {
		t.Fatalf("SaveFact error: %v", err)
	}
	if !r.Created {
		t.Error("la primera relación debería ser nueva")
	}
	// Misma relación otra vez -> no duplica.
	r, err = e.SaveFact("Alice", "works_at", "ACME")
	if err != nil {
		t.Fatalf("SaveFact error: %v", err)
	}
	if r.Created {
		t.Error("relación repetida no debería crearse de nuevo")
	}
	// Nueva relación reusando 'Alice'.
	if _, err := e.SaveFact("Alice", "knows", "Bob"); err != nil {
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
	if _, err := e.SaveFact("Alice", "likes", "Go"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("  alice ", "uses", "SQLite"); err != nil {
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

	res, err := e.RecallFacts("Alice", 1, 50)
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

	res, err := e.RecallFacts("Alice", 2, 50)
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
	res, err := e.RecallFacts("Zzz", 2, 50)
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

	res, err := e.RecallFacts("Alice", 1, 1)
	if err != nil {
		t.Fatalf("RecallFacts error: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("maxFacts=1 debería limitar a 1 hecho, obtuve %d", res.Count)
	}
}

func mustFact(t *testing.T, e *DbEngine, s, p, o string) {
	t.Helper()
	if _, err := e.SaveFact(s, p, o); err != nil {
		t.Fatalf("SaveFact(%s,%s,%s) error: %v", s, p, o, err)
	}
}
