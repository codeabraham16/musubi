package memory

import "testing"

// Custodio del invariante: LA DETECCIÓN NO PISA AL JUEZ.
//
// El defecto que estos tests clavan se midió el 2026-11-01 contra la base viva. Emitir un veredicto
// y después RE-GUARDAR la observación de origen devolvía la relación a `pending` con su razón
// escrita BORRADA, sin error y sin aviso. La detección corre en CADA save, así que toda nota que se
// edite deshacía el arbitraje de sus relaciones y la cola de conflictos parecía crecer sola.
//
// La asimetría es lo que lo hacía invisible: el COALESCE de lex/cosine ya blindaba el sentido
// judge→detección («juzgar no debería borrar cómo se la detectó»); el sentido inverso no estaba
// blindado, y es el que borra lo que una persona o un agente decidieron a propósito.

// El caso medido, por el camino REAL: juzgar y volver a detectar.
func TestLaDeteccionNoPisaElVeredicto(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("primera detección: %v", err)
	}
	pend, err := e.PendingObsRelations()
	if err != nil || len(pend) != 1 {
		t.Fatalf("esperaba 1 pendiente recién detectada, obtuve %d (err=%v)", len(pend), err)
	}
	const razón = "las dos describen el mismo arreglo pero una trae el comando"
	if err := e.ResolveObsRelation(pend[0].ID, RelRelated, "agent", razón); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}

	// El re-save de la observación: la detección vuelve a correr sobre el mismo par.
	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("segunda detección: %v", err)
	}

	all, err := e.AllObsRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("el par no debe duplicarse, obtuve %d filas", len(all))
	}
	got := all[0]
	if got.Status != RelStatusResolved {
		t.Errorf("status: la detección degradó el veredicto a %q", got.Status)
	}
	if got.Relation != RelRelated {
		t.Errorf("relation: la detección pisó el veredicto con %q", got.Relation)
	}
	if got.ResolvedBy != "agent" {
		t.Errorf("resolved_by: se perdió quién juzgó (quedó %q)", got.ResolvedBy)
	}
	if got.Reason != razón {
		t.Errorf("reason: se borró la razón escrita (quedó %q)", got.Reason)
	}
	// Lo que se ve desde afuera: la cola no vuelve a llenarse sola.
	if again, _ := e.PendingObsRelations(); len(again) != 0 {
		t.Errorf("la relación ya juzgada volvió a la cola (%d pendientes)", len(again))
	}
}

// La contracara, para que la guarda no se vuelva un CANDADO: un upsert que trae un veredicto
// explícito sí tiene que pisar al anterior. Es el camino de RestoreDuplicate (deshacer una fusión
// marca el par not_duplicate en el mismo commit); si esto se rompe, deshacer una fusión dejaría el
// veredicto viejo y nada se quejaría.
func TestUnVeredictoExplicitoSiPisaAlAnterior(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelSupersedes,
		Status: RelStatusResolved, ResolvedBy: "heuristic", Reason: "la más nueva reemplaza",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelNotDuplicate,
		Status: RelStatusResolved, ResolvedBy: "user", Reason: "fusión deshecha", Confidence: 1.0,
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if len(all) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
	}
	if all[0].Relation != RelNotDuplicate || all[0].ResolvedBy != "user" || all[0].Reason != "fusión deshecha" {
		t.Errorf("un veredicto explícito debe pisar al anterior, quedó: %+v", all[0])
	}
}

// Por qué la guarda va POR CAMPO y no como un WHERE que aborte el UPDATE entero: proteger el
// veredicto no debe congelar las señales. Re-detectar una fila ya juzgada tiene que seguir
// refrescando lex/cosine, que es cómo se la detectó ESTA vez.
func TestLaDeteccionRefrescaLosPuntajesDeUnaFilaResuelta(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	viejo := 0.42
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelRelated, Status: RelStatusResolved,
		ResolvedBy: "agent", Reason: "juzgada", Lex: &viejo,
	}); err != nil {
		t.Fatal(err)
	}
	nuevo := 0.91
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending, Lex: &nuevo,
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if all[0].Status != RelStatusResolved {
		t.Fatalf("el veredicto debía sobrevivir, quedó %q", all[0].Status)
	}
	if all[0].Lex == nil || *all[0].Lex != nuevo {
		t.Errorf("lex_score debía refrescarse a %v, quedó %v", nuevo, all[0].Lex)
	}
}

// El SQL de la guarda se arma por concatenación, así que conviene poder LEERLO. Este test lo congela:
// si la condición se invierte, o si una columna termina conservando lo entrante en vez de lo viejo,
// la guarda se vuelve un no-op SILENCIOSO —nada falla y los veredictos se pisan otra vez—, y lo
// único que lo delata es la forma del SQL.
func TestElSetDeLaGuardaConservaLoViejoSoloContraLaDeteccion(t *testing.T) {
	const want = "status=CASE WHEN observation_relations.status='resolved' AND excluded.status='pending'" +
		" THEN observation_relations.status ELSE excluded.status END"
	if got := setConservandoVeredicto("status"); got != want {
		t.Errorf("el SET generado cambió de forma: got %s / want %s", got, want)
	}
}
