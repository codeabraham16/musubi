package memory

import "testing"

func TestConsolidateSoftDeletesDuplicate(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "a", "arch/db", "Usamos PostgreSQL para la base de datos del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "arch/db", "Usamos PostgreSQL para la base de datos del sistema productivo.", "2026-01-02 10:00:00")
	// 'a' más fuerte (más accesos) → canónico; 'b' se archiva como duplicado (soft-delete).
	if _, err := e.db.Exec(`UPDATE observations SET access_count=10 WHERE id='a'`); err != nil {
		t.Fatal(err)
	}

	res, err := e.Consolidate(0.3)
	if err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}
	if res.Merged < 1 {
		t.Fatalf("esperaba al menos 1 fusión, obtuve %+v", res)
	}

	// El duplicado NO se borra: queda archivado (reversible) y apuntando al canónico.
	var archived int
	var supersededBy, archivedAt string
	if err := e.db.QueryRow(`SELECT archived, COALESCE(superseded_by,''), COALESCE(archived_at,'') FROM observations WHERE id='b'`).
		Scan(&archived, &supersededBy, &archivedAt); err != nil {
		t.Fatalf("'b' no debió borrarse físicamente (soft-delete): %v", err)
	}
	if archived != 1 {
		t.Errorf("'b' debió quedar archivado (archived=1), obtuve %d", archived)
	}
	if supersededBy != "a" {
		t.Errorf("'b' debió apuntar al canónico 'a', obtuve superseded_by=%q", supersededBy)
	}
	if archivedAt == "" {
		t.Errorf("'b' debió tener archived_at seteado (arranca la ventana de gracia)")
	}
	// El canónico sobrevive vivo.
	var canonArch int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='a'`).Scan(&canonArch); err != nil || canonArch != 0 {
		t.Errorf("el canónico 'a' debe seguir vivo (archived=0), obtuve archived=%d err=%v", canonArch, err)
	}
}

func TestConsolidateSkipsSuperseded(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "old", "arch/db", "Usamos PostgreSQL para la base de datos del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "new", "arch/db", "Usamos PostgreSQL para la base de datos del sistema productivo.", "2026-01-02 10:00:00")
	if err := e.markSuperseded("old", "new"); err != nil {
		t.Fatal(err)
	}
	// Consolidate solo opera sobre memorias vivas: no debe tocar la superseded 'old'.
	if _, err := e.Consolidate(0.3); err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='old'`).Scan(&sup); err != nil {
		t.Fatalf("'old' no debió borrarse: %v", err)
	}
	if sup != "new" {
		t.Errorf("'old' debe seguir superseded por 'new', obtuve %q", sup)
	}
}

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

	// Soft-delete: la fila del duplicado persiste pero archivada; quedan 2 VIVAS.
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE archived=0`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("esperaba 2 filas vivas tras consolidar, obtuve %d", n)
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
