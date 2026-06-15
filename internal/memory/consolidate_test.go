package memory

import "testing"

func TestConsolidateMergesNearDuplicates(t *testing.T) {
	e := newTestEngine(t)

	// Dos casi-duplicados (difieren solo en un punto) + uno distinto.
	if err := e.SaveObservation("a", "t", "el patron observer en go sirve para eventos", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("b", "t", "el patron observer en go sirve para eventos.", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("c", "t", "optimizacion de indices en la base de datos", nil); err != nil {
		t.Fatal(err)
	}
	// 'a' es más fuerte (más accesos) -> debe quedar como canónico.
	if _, err := e.db.Exec(`UPDATE observations SET access_count=5 WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET access_count=2 WHERE id='b'`); err != nil {
		t.Fatal(err)
	}

	res, err := e.Consolidate(0.8)
	if err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}
	if res.Merged != 1 {
		t.Errorf("esperaba 1 merge, obtuve %d", res.Merged)
	}

	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("esperaba 2 filas tras consolidar, obtuve %d", n)
	}

	// El canónico 'a' sobrevive y acumula los accesos del duplicado.
	var access int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id='a'`).Scan(&access); err != nil {
		t.Fatalf("'a' no sobrevivió: %v", err)
	}
	if access != 7 {
		t.Errorf("esperaba access_count acumulado 7, obtuve %d", access)
	}
}

func TestConsolidateNoFalseMerge(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t", "autenticacion con jwt y refresh tokens", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("y", "t", "rate limiting del login con token bucket", nil); err != nil {
		t.Fatal(err)
	}

	res, err := e.Consolidate(0.8)
	if err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}
	if res.Merged != 0 {
		t.Errorf("no debería fusionar textos distintos, mergeó %d", res.Merged)
	}
}
