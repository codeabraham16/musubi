package memory

import "testing"

func TestGetObservationsReturnsContentInOrder(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := e.SaveObservation(id, "t", "contenido de "+id, nil); err != nil {
			t.Fatalf("save %s error: %v", id, err)
		}
	}

	res, err := e.GetObservations([]string{"c", "a"})
	if err != nil {
		t.Fatalf("GetObservations error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("esperaba 2 observaciones, obtuve %d", len(res))
	}
	if res[0].ID != "c" || res[1].ID != "a" {
		t.Errorf("esperaba orden [c,a], obtuve [%s,%s]", res[0].ID, res[1].ID)
	}
	if res[0].Content != "contenido de c" {
		t.Errorf("esperaba contenido completo, obtuve %q", res[0].Content)
	}
}

func TestGetObservationsSkipsMissing(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "existe", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}
	res, err := e.GetObservations([]string{"a", "no-existe"})
	if err != nil {
		t.Fatalf("GetObservations error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "a" {
		t.Errorf("esperaba solo 'a', obtuve %+v", res)
	}
}

func TestGetObservationsBudgetCaps(t *testing.T) {
	e := newTestEngine(t)
	long := "contenido bastante largo numero "
	for _, id := range []string{"a", "b", "c"} {
		if err := e.SaveObservation(id, "t", long+id+" con relleno para ocupar tokens", nil); err != nil {
			t.Fatalf("save %s error: %v", id, err)
		}
	}

	// Budget chico: no deben entrar las 3.
	res, used, err := e.GetObservationsBudget([]string{"a", "b", "c"}, 20)
	if err != nil {
		t.Fatalf("GetObservationsBudget error: %v", err)
	}
	if len(res) == 0 || len(res) >= 3 {
		t.Errorf("esperaba un recorte por presupuesto (1..2), obtuve %d", len(res))
	}
	if used > 20 {
		t.Errorf("used_tokens %d excede el presupuesto 20", used)
	}

	// Budget 0 (sin límite): trae todo, como GetObservations.
	all, _, err := e.GetObservationsBudget([]string{"a", "b", "c"}, 0)
	if err != nil {
		t.Fatalf("GetObservationsBudget(0) error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("budget 0 debe traer todo (3), obtuve %d", len(all))
	}
}

func TestGetObservationsBumpsAccess(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "algo", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}
	if _, err := e.GetObservations([]string{"a"}); err != nil {
		t.Fatalf("GetObservations error: %v", err)
	}
	var count int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id=?`, "a").Scan(&count); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count < 1 {
		t.Errorf("esperaba access_count >= 1, obtuve %d", count)
	}
}
