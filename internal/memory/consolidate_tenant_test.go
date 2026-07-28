package memory

import "testing"

// seedProj guarda una observación atribuida a un project_id (Track 17), para ejercitar el
// aislamiento por tenant de la consolidación.
func seedProj(t *testing.T, e *DbEngine, project, id, topic, content string) {
	t.Helper()
	if err := e.SaveObservationTypedFrom(project, "", id, topic, content, 1.0, "", "", nil); err != nil {
		t.Fatalf("seed %s (%s): %v", id, project, err)
	}
}

// TestConsolidateNoMergeAcrossTenants: dos observaciones de TEXTO IDÉNTICO en proyectos distintos
// NO se fusionan — es el escenario que fallaba antes del fix (fusión cross-tenant vía byNorm). R1.
func TestConsolidateNoMergeAcrossTenants(t *testing.T) {
	e := newTestEngine(t)
	txt := "Usamos PostgreSQL para la base de datos del sistema."
	seedProj(t, e, "alpha", "a1", "arch/db", txt)
	seedProj(t, e, "beta", "b1", "arch/db", txt) // MISMO texto, OTRO tenant

	res, err := e.Consolidate(0.3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Merged != 0 {
		t.Errorf("no debe fusionar cross-tenant (texto idéntico, distinto project_id); Merged=%d", res.Merged)
	}
	for _, id := range []string{"a1", "b1"} {
		var arch int
		if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id=?`, id).Scan(&arch); err != nil {
			t.Fatal(err)
		}
		if arch != 0 {
			t.Errorf("%s de otro tenant debería seguir viva (archived=0), got %d", id, arch)
		}
	}
}

// TestConsolidateMergesWithinTenant: mismo texto en el MISMO proyecto SÍ fusiona (R2); un tercero
// de otro proyecto queda intacto (R1). Cubre la vía exacta-por-norma y la mezcla A+A+B.
func TestConsolidateMergesWithinTenant(t *testing.T) {
	e := newTestEngine(t)
	txt := "Usamos PostgreSQL para la base de datos del sistema."
	seedProj(t, e, "alpha", "a1", "arch/db", txt)
	seedProj(t, e, "alpha", "a2", "arch/db", txt) // mismo tenant, mismo texto → fusiona
	seedProj(t, e, "beta", "b1", "arch/db", txt)  // otro tenant → intacto

	res, err := e.Consolidate(0.3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Merged != 1 {
		t.Errorf("debe fusionar exactamente los dos de 'alpha'; Merged=%d", res.Merged)
	}
	var bArch int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='b1'`).Scan(&bArch); err != nil {
		t.Fatal(err)
	}
	if bArch != 0 {
		t.Errorf("b1 de otro tenant no debe archivarse; archived=%d", bArch)
	}
	var aArch int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id IN ('a1','a2') AND archived=1`).Scan(&aArch); err != nil {
		t.Fatal(err)
	}
	if aArch != 1 {
		t.Errorf("exactamente una de las dos 'alpha' debe quedar archivada; got %d", aArch)
	}
}
