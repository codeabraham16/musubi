package memory

import "testing"

// addPendingRel inserta una relación 'pending' directa (sin pasar por la detección), para armar
// escenarios de cola controlados.
func addPendingRel(t *testing.T, e *DbEngine, src, tgt string) {
	t.Helper()
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: src, TargetID: tgt, Relation: RelPending, Status: RelStatusPending, Confidence: 0.5,
	}); err != nil {
		t.Fatalf("upsert %s→%s: %v", src, tgt, err)
	}
}

// El cleanup 'stale_conflicts' poda el RUIDO ESTRUCTURAL de la cola —target histórico y recíprocos—
// sin tocar las relaciones nota↔nota legítimas ni las resueltas. Es la limpieza del residuo que se
// acumuló antes de que existieran las guardas (complementaryPair).
func TestStaleConflictsPodaRuidoEstructural(t *testing.T) {
	e := newTestEngine(t)

	saveAt(t, e, "nota-a", "arch/db", "Usamos Postgres como base principal.", "2026-01-01 10:00:00")
	saveAt(t, e, "nota-b", "arch/api", "El API valida tokens JWT.", "2026-01-02 10:00:00")
	saveAt(t, e, "nota-c", "arch/cache", "Cacheamos en Redis con TTL corto.", "2026-01-05 10:00:00")
	saveAt(t, e, "sdd-x", "sdd/mi-cambio/spec", "Spec del cambio mi-cambio.", "2026-01-03 10:00:00")
	saveAt(t, e, "commit-y", "git-commit", "feat: agrega algo al sistema.", "2026-01-04 10:00:00")

	// (1) target HISTÓRICO (commit / SDD) ⇒ ruido: complementaryPair ya no lo crea.
	addPendingRel(t, e, "nota-a", "sdd-x")
	addPendingRel(t, e, "nota-a", "commit-y")
	// (2) RECÍPROCO duplicado: nota-a↔nota-b en ambas direcciones.
	addPendingRel(t, e, "nota-a", "nota-b")
	addPendingRel(t, e, "nota-b", "nota-a")
	// (3) LEGÍTIMA nota→nota, sin recíproco y target no histórico ⇒ se conserva.
	addPendingRel(t, e, "nota-c", "nota-b")

	ids, err := staleConflictIDs(e)
	if err != nil {
		t.Fatal(err)
	}
	// Podar: nota-a→sdd-x, nota-a→commit-y, y el lado NO canónico del recíproco (nota-b→nota-a,
	// porque "nota-b" > "nota-a"). La dirección canónica (nota-a→nota-b) se conserva.
	if len(ids) != 3 {
		t.Fatalf("esperaba 3 relaciones a podar, obtuve %d", len(ids))
	}

	before, _ := e.AllObsRelations()
	n, err := applyDeleteStaleConflicts(e)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("borró %d, esperaba 3", n)
	}

	after, err := e.AllObsRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)-3 {
		t.Errorf("quedaron %d relaciones, esperaba %d", len(after), len(before)-3)
	}

	survives := func(src, tgt string) bool {
		for _, r := range after {
			if r.SourceID == src && r.TargetID == tgt {
				return true
			}
		}
		return false
	}
	if !survives("nota-a", "nota-b") {
		t.Error("se podó la dirección canónica del recíproco (nota-a→nota-b)")
	}
	if !survives("nota-c", "nota-b") {
		t.Error("se podó una relación legítima nota→nota (nota-c→nota-b)")
	}

	// Idempotente: una segunda corrida no encuentra nada.
	if again, err := countStaleConflicts(e); err != nil || again != 0 {
		t.Errorf("segunda corrida: esperaba 0 sin error, obtuve %d (%v)", again, err)
	}
}

// ⚠️ L6 — LA CANILLA Y EL BALDE. La guarda nueva (ledger_prefixes) impide que NAZCAN relaciones
// contra un género declarado libro mayor, pero las que YA existen sólo desaparecen si el doctor
// las reconoce. Si el doctor se quedara con `historicalRecord` a secas, el efecto neto de declarar
// un prefijo sería: la cola deja de crecer y queda llena para siempre.
//
// Por eso `doctor.go` dice que poda con «la misma función que la detección, no una aproximación
// que pueda divergir de la guarda». Este test es lo que impide que esa frase se vuelva mentira.
func TestL6ElDoctorPodaConLaMismaReglaQueLaDeteccion(t *testing.T) {
	e := newTestEngine(t)

	saveAt(t, e, "nota", "arch/db", "Usamos Postgres como base principal.", "2026-01-01 10:00:00")
	saveAt(t, e, "carta", "terminales/emisario-a-principal", "Acuse del despacho anterior.", "2026-01-02 10:00:00")

	addPendingRel(t, e, "nota", "carta")

	// CONTROL: sin declarar el prefijo, esa relación NO es ruido estructural — el doctor no la
	// toca. Sin este control el test de abajo podría estar verde porque el doctor poda de más.
	ids, err := staleConflictIDs(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("sin prefijos declarados el doctor no debe podar nada; podaría %v", ids)
	}

	// LO QUE SE PRUEBA: declarado el prefijo, la relación vieja pasa a ser podable.
	e.SetLedgerPrefixes([]string{"terminales/"})
	ids, err = staleConflictIDs(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("con `terminales/` declarado el doctor debería poder podar la relación vieja; podaría %v", ids)
	}

	// Y NO OCULTA MEMORIA: podar una relación jamás puede costar una observación.
	if _, err := applyDeleteStaleConflicts(e); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"nota", "carta"} {
		if _, ok, err := e.loadObsRow(id); err != nil || !ok {
			t.Errorf("la poda se llevó puesta la observación %q (ok=%v err=%v)", id, ok, err)
		}
	}
}
