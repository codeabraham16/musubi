package memory

import "testing"

func TestUpsertYListarObsRelations(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "topic/x", "contenido A", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("b", "topic/x", "contenido B", nil); err != nil {
		t.Fatal(err)
	}

	rel := ObsRelation{
		SourceID:   "a",
		TargetID:   "b",
		Relation:   RelPending,
		Confidence: 0.6,
		Status:     RelStatusPending,
	}
	if _, err := e.UpsertObsRelation(rel); err != nil {
		t.Fatalf("UpsertObsRelation error: %v", err)
	}

	pend, err := e.PendingObsRelations()
	if err != nil {
		t.Fatalf("PendingObsRelations error: %v", err)
	}
	if len(pend) != 1 {
		t.Fatalf("esperaba 1 relación pendiente, obtuve %d", len(pend))
	}
	if pend[0].SourceID != "a" || pend[0].TargetID != "b" || pend[0].Relation != RelPending {
		t.Errorf("relación inesperada: %+v", pend[0])
	}
	if pend[0].ID == "" {
		t.Error("la relación debería tener un id asignado")
	}
}

func TestUpsertObsRelationEsIdempotentePorPar(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending}); err != nil {
		t.Fatal(err)
	}
	// Mismo par, distinto veredicto: debe ACTUALIZAR, no duplicar.
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "a", TargetID: "b", Relation: RelRelated, Confidence: 0.9, Status: RelStatusResolved, ResolvedBy: "heuristic"}); err != nil {
		t.Fatal(err)
	}
	all, err := e.AllObsRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("el par (a,b) no debe duplicarse, obtuve %d filas", len(all))
	}
	if all[0].Relation != RelRelated || all[0].Status != RelStatusResolved {
		t.Errorf("el upsert debió actualizar el veredicto: %+v", all[0])
	}
}

func TestResolveObsRelation(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending}); err != nil {
		t.Fatal(err)
	}
	pend, _ := e.PendingObsRelations()
	id := pend[0].ID

	if err := e.ResolveObsRelation(id, RelConflictsWith, "agent", "el agente determinó contradicción"); err != nil {
		t.Fatalf("ResolveObsRelation error: %v", err)
	}
	if again, _ := e.PendingObsRelations(); len(again) != 0 {
		t.Errorf("tras resolver no debe quedar pendiente, quedan %d", len(again))
	}
	all, _ := e.AllObsRelations()
	if all[0].Relation != RelConflictsWith || all[0].Status != RelStatusResolved || all[0].ResolvedBy != "agent" {
		t.Errorf("resolución no persistida correctamente: %+v", all[0])
	}
}
