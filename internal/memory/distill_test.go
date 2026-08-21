package memory

import "testing"

// TestObservationsMissingRelation valida la consulta del DESTILADOR (Musubi Renaissance): trae los blobs
// `ingested/*` de un tenant que TODAVÍA no son destino de una arista `derived_from`, ignora los de otro
// prefijo y los de otro proyecto, respeta el marcador y el límite, y el count coincide.
func TestObservationsMissingRelation(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"

	seed := func(id, topic, project string) {
		if err := e.SaveObservationTypedFrom(project, "seed", id, topic, "contenido de "+id, 1.0, "semantic", "local", nil); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	seed("b1", "ingested/youtube/aaa", proj)
	seed("b2", "ingested/web/bbb", proj)
	seed("b3", "ingested/youtube/ccc", proj)
	seed("card1", "design-corpus/algo", proj) // curada, no `ingested/` ⇒ nunca cuenta
	seed("other", "ingested/web/ddd", "crm")  // otro tenant ⇒ nunca cuenta

	// Marcar b2 como destilado: arista card1 → b2, derived_from, resuelta.
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "card1", TargetID: "b2", Relation: RelDerivedFrom, Status: RelStatusResolved}); err != nil {
		t.Fatal(err)
	}

	got, err := e.ObservationsMissingRelation(proj, "ingested/", RelDerivedFrom, 10)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, o := range got {
		ids[o.ID] = true
	}
	if len(got) != 2 || !ids["b1"] || !ids["b3"] {
		t.Fatalf("esperaba pendientes {b1,b3}, obtuve %+v", got)
	}
	if n, err := e.CountObservationsMissingRelation(proj, "ingested/", RelDerivedFrom); err != nil || n != 2 {
		t.Fatalf("count esperaba 2, obtuve %d (err %v)", n, err)
	}

	// El límite acota la lista pero NO el conteo.
	if lim, err := e.ObservationsMissingRelation(proj, "ingested/", RelDerivedFrom, 1); err != nil || len(lim) != 1 {
		t.Fatalf("con limit 1 esperaba 1 fila, obtuve %d (err %v)", len(lim), err)
	}

	// Marcar b1 y b3 también ⇒ 0 pendientes (drenado).
	for _, id := range []string{"b1", "b3"} {
		if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "card1", TargetID: id, Relation: RelDerivedFrom, Status: RelStatusResolved}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := e.CountObservationsMissingRelation(proj, "ingested/", RelDerivedFrom); err != nil || n != 0 {
		t.Fatalf("tras marcar todo esperaba 0, obtuve %d (err %v)", n, err)
	}
}
