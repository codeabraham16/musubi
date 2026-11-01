package memory

import (
	"context"
	"testing"
)

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
//
// La condición pregunta `<> 'pending'` y NO `= 'resolved'`: así es fail-closed —un estado futuro
// queda protegido por default— y es el MISMO predicado con el que readiness cuenta una relación como
// resuelta, así que guarda y lector no pueden divergir sin que se note.
func TestElSetDeLaGuardaConservaLoViejoSoloContraLaDeteccion(t *testing.T) {
	const cond = "CASE WHEN observation_relations.status<>'pending'" +
		" AND (excluded.status='pending' OR (excluded.resolved_by='heuristic'" +
		" AND COALESCE(observation_relations.resolved_by,'')<>'heuristic'))"
	casos := []struct{ col, entrante, want string }{
		{"status", "excluded.status",
			"status=" + cond + " THEN observation_relations.status ELSE excluded.status END"},
		// El segundo caso no es decorativo: `updated_at` es la única de las cinco cuyo valor
		// entrante NO es `excluded.<col>` sino el reloj, y es la que readiness lee como fecha del
		// veredicto. Si la plantilla dejara de admitir un entrante arbitrario, esta columna sería la
		// primera en romperse.
		{"updated_at", "CURRENT_TIMESTAMP",
			"updated_at=" + cond + " THEN observation_relations.updated_at ELSE CURRENT_TIMESTAMP END"},
	}
	for _, c := range casos {
		if got := setConservandoVeredicto(c.col, c.entrante); got != c.want {
			t.Errorf("el SET generado para %s cambió de forma:\n got %s\nwant %s", c.col, got, c.want)
		}
	}
}

// LA GUARDA NO PUEDE INVENTAR ARBITRAJE. Es el hallazgo que casi entró con el arreglo: conservar el
// veredicto y pisarle igual `updated_at` convierte cada re-detección en un arbitraje que nadie hizo.
//
// No existe `resolved_at`, así que para readiness `updated_at` ES la fecha del veredicto: cuenta como
// resuelta-en-la-ventana toda relación con status<>pending y updated_at reciente. Un cerebro que dejó
// de arbitrar hace meses seguiría dando verde en «alguien arbitra todavía» con sólo editar notas —el
// autoengaño exacto que esa dimensión existe para detectar, y encima introducido por el arreglo que
// venía a proteger el veredicto.
//
// Antes del arreglo esto era honesto por accidente: la fila degradada a `pending` quedaba fuera del
// conteo de resueltas. El arreglo tenía que conservar el veredicto SIN heredar ese accidente.
func TestUnaReDeteccionNoCuentaComoArbitrajeEnReadiness(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("primera detección: %v", err)
	}
	pend, err := e.PendingObsRelations()
	if err != nil || len(pend) != 1 {
		t.Fatalf("esperaba 1 pendiente, obtuve %d (err=%v)", len(pend), err)
	}
	if err := e.ResolveObsRelation(pend[0].ID, RelRelated, "agent", "arbitrada hace meses"); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}
	// El veredicto envejece FUERA de la ventana: es el caso donde la métrica tiene que decir «acá
	// ya nadie arbitra». Mientras esté dentro, cualquier valor pasa y el test no mide nada.
	if _, err := e.db.Exec(
		`UPDATE observation_relations SET updated_at=datetime('now','-90 days')`); err != nil {
		t.Fatal(err)
	}

	resueltasEnVentana := func(cuando string) int {
		t.Helper()
		rep, err := e.Readiness(context.Background(), 30)
		if err != nil {
			t.Fatalf("Readiness %s: %v", cuando, err)
		}
		n, ok := dimensionDe(t, rep, "coherencia").Evidence["resueltas_en_ventana"].(int)
		if !ok {
			t.Fatalf("resueltas_en_ventana no es int %s", cuando)
		}
		return n
	}
	if n := resueltasEnVentana("antes"); n != 0 {
		t.Fatalf("precondición: con el veredicto a 90 días la ventana de 30 debe ver 0, vio %d", n)
	}

	// Editar la nota: la detección vuelve a correr y la guarda conserva el veredicto.
	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("segunda detección: %v", err)
	}

	if n := resueltasEnVentana("después"); n != 0 {
		t.Errorf("una re-detección contó como %d arbitraje(s) de la ventana: readiness daría verde"+
			" en «alguien arbitra todavía» sin que nadie arbitre", n)
	}
	// La contracara: arbitrar DE VERDAD sí tiene que mover la fecha, o la guarda sería un candado
	// que congela la métrica en el pasado para siempre.
	if err := e.ResolveObsRelation(pend[0].ID, RelCompatible, "user", "re-arbitrada hoy"); err != nil {
		t.Fatalf("ResolveObsRelation (2ª): %v", err)
	}
	if n := resueltasEnVentana("tras re-arbitrar"); n != 1 {
		t.Errorf("un veredicto NUEVO debe contar en la ventana, contó %d", n)
	}
}

// LO QUE SE DEVUELVE ES LO QUE QUEDÓ EN LA BASE, NO LO QUE ENTRÓ. Hasta que existió la guarda las dos
// cosas coincidían y devolver la entrante era gratis; ahora divergen, y quien lee el retorno decide
// por él. detectAndSurface clasifica por Status y, con la entrante, anunciaría «requieren tu
// veredicto» con el id canónico de una relación YA juzgada. El agente obedecería, y
// ResolveObsRelation no tiene guarda: pisaría el veredicto que el upsert acaba de proteger, y con
// razón vacía lo borraría. O sea: blindar la base y mandar a borrarla por la puerta de al lado.
func TestDetectRelationsDevuelveLoPersistidoNoLoEntrante(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("primera detección: %v", err)
	}
	pend, _ := e.PendingObsRelations()
	if len(pend) != 1 {
		t.Fatalf("esperaba 1 pendiente, obtuve %d", len(pend))
	}
	const razón = "no se tapan: una trae el comando"
	if err := e.ResolveObsRelation(pend[0].ID, RelRelated, "user", razón); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}

	rels, err := e.DetectRelations("c2", detectOnlyOpts(true))
	if err != nil {
		t.Fatalf("segunda detección: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("esperaba 1 relación devuelta, obtuve %d", len(rels))
	}
	got := rels[0]
	if got.Status != RelStatusResolved || got.Relation != RelRelated {
		t.Errorf("devolvió lo ENTRANTE (%s/%s): quien lea esto mandará a juzgar algo ya juzgado",
			got.Relation, got.Status)
	}
	if got.ResolvedBy != "user" || got.Reason != razón {
		t.Errorf("el retorno perdió el rastro del veredicto: resolved_by=%q reason=%q",
			got.ResolvedBy, got.Reason)
	}
	if got.ID != pend[0].ID {
		t.Errorf("el id devuelto (%s) no es el canónico de la fila (%s)", got.ID, pend[0].ID)
	}
}

// EL EFECTO NO PUEDE CONTRADECIR AL ASIENTO. Es la trampa de devolver lo persistido: si el
// markSuperseded pasara a colgar de la fila de la base en vez de la decisión de ahora, rompería M4;
// y si colgara SÓLO de la decisión de ahora, ocultaría memoria que el asiento dice `related`.
//
// El caso es el real y entra por el camino explícito: alguien juzgó el par `related` —«no se tapan»—
// y después se re-guarda la observación. decideRelation vuelve a decidir supersedes (lo prueba
// TestWithoutDetectOnlyTheSameAutoCapturesWouldAutoSupersede con este mismo par), la guarda conserva
// `related` en la base... y si el efecto colgara sólo de la decisión, c1 quedaría OCULTA con un
// asiento que dice `related`: el efecto aplicado sin veredicto que lo explique, justo lo que la
// guarda existe para evitar, entrando por la puerta de al lado.
func TestUnVeredictoQueDiceRelatedNoDejaOcultarPorLaPuertaDeAlLado(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("detección inicial: %v", err)
	}
	pend, _ := e.PendingObsRelations()
	if len(pend) != 1 {
		t.Fatalf("esperaba 1 pendiente, obtuve %d", len(pend))
	}
	if err := e.ResolveObsRelation(pend[0].ID, RelRelated, "user", "no se tapan"); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}
	assertNotSuperseded(t, e, "c1") // precondición: el veredicto `related` no oculta nada

	// El camino EXPLÍCITO, el que sí auto-supersede: es el que vuelve a decidir supersedes.
	rels, err := e.DetectRelations("c2", detectOnlyOpts(false))
	if err != nil {
		t.Fatalf("re-detección explícita: %v", err)
	}
	if len(rels) != 1 || rels[0].Relation != RelRelated || rels[0].Status != RelStatusResolved {
		t.Fatalf("el asiento debía seguir siendo related/resolved, quedó %+v", rels)
	}
	// LO QUE IMPORTA: la observación sigue en el recall. Un asiento que dice `related` y una
	// observación oculta son un cerebro que esconde memoria sin poder decir por qué.
	assertNotSuperseded(t, e, "c1")
}

// LA OTRA MITAD DE LA CONJUNCIÓN: con DetectOnly, el markSuperseded sigue siendo INALCANZABLE aunque
// la fila guardada ya diga supersedes/resolved (M4). Si el efecto colgara de lo persistido, cada
// save_observation re-dispararía el ocultamiento de un veredicto viejo y la captura automática
// volvería a tapar memoria sola — el peligro concreto que DetectOnly existe para cerrar.
//
// El estado se CONSTRUYE a propósito (fila supersedes/resolved con la target visible, que es lo que
// UpsertObsRelation deja porque no aplica efectos): es el único modo de aislar de qué cuelga el
// efecto sin que el ocultamiento previo tape la medición.
func TestConDetectOnlyUnVeredictoSupersedesViejoNoReOculta(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "c2", TargetID: "c1", Relation: RelSupersedes,
		Status: RelStatusResolved, ResolvedBy: "agent", Reason: "veredicto viejo",
	}); err != nil {
		t.Fatal(err)
	}
	assertNotSuperseded(t, e, "c1") // precondición: el upsert no aplica efectos

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatalf("detección automática: %v", err)
	}
	// R0: la captura automática NO oculta memoria, ni siquiera reciclando un veredicto anterior.
	assertNotSuperseded(t, e, "c1")
}

// LA ASIMETRÍA DE LA GUARDA, QUE ES LO QUE LA SEPARA DE UN CANDADO: el detector no puede pisar un
// veredicto AJENO, pero SÍ tiene que poder refrescar uno SUYO. Un par que el heurístico marcó
// `related` por solape léxico y que hoy, con el texto cambiado, da `supersedes`, tiene que poder
// avanzar: congelar el primer veredicto automático que tocó la fila la dejaría clavada para siempre
// en una conclusión que el detector mismo ya no sostiene.
func TestElDetectorSiRefrescaUnVeredictoPropio(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelRelated, Status: RelStatusResolved,
		ResolvedBy: RelResolvedByHeuristic, Reason: "alto solape léxico; relacionadas",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelSupersedes, Status: RelStatusResolved,
		ResolvedBy: RelResolvedByHeuristic, Reason: "mismo topic_key y similitud alta",
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if len(all) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
	}
	if all[0].Relation != RelSupersedes {
		t.Errorf("el detector debe poder refrescar su PROPIO veredicto, quedó %q: la guarda se"+
			" volvió un candado", all[0].Relation)
	}
}

// Y LA CONTRACARA DE LA REGLA (1): un `pending` entrante tampoco degrada un veredicto del DETECTOR.
// La regla es «ningún pending degrada un veredicto», no «ningún pending degrada un veredicto
// HUMANO»: una fila que dice `related`/resolved describe un par que ya se mirÓ, y volverla a la cola
// la hace re-mirar para siempre.
//
// ESTA VERSIÓN DEL TEST REEMPLAZA UNA VACUA, y vale anotar por qué para no re-escribirla: la primera
// usaba el auto-SUPERSEDE y pasaba sin que la guarda hiciera nada. markSuperseded oculta la target y
// conflictCandidates excluye las superseded, así que ese par NO SE RE-DETECTA NUNCA y el upsert ni
// corría. Verde por el motivo equivocado. El auto-RELATED no oculta nada, así que la target sigue
// siendo candidata y el camino se ejerce de verdad: lo prueba el sabotaje, que ahora sí lo enrojece.
func TestUnPendingTampocoDegradaUnVeredictoDelDetector(t *testing.T) {
	e := newTestEngine(t)
	// Mismo dominio (el prefijo antes de la barra) para no caer en dominiosAjenos, pero topic_key
	// DISTINTO: es la rama de decideRelation que resuelve `related` sin ocultar nada.
	if err := e.SaveObservation("a", "topic/uno", "fix: typo en el README del proyecto", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("b", "topic/dos", "fix: typo en el README del proyecto core", nil); err != nil {
		t.Fatal(err)
	}
	rels, err := e.DetectRelations("b", detectOnlyOpts(false))
	if err != nil {
		t.Fatalf("detección explícita: %v", err)
	}
	if len(rels) != 1 || rels[0].Relation != RelRelated ||
		rels[0].Status != RelStatusResolved || rels[0].ResolvedBy != RelResolvedByHeuristic {
		t.Fatalf("precondición: el camino explícito debía dejar un related/resolved del heurístico,"+
			" dejó %+v", rels)
	}
	assertNotSuperseded(t, e, "a") // y sin ocultar nada: por eso el par sigue siendo detectable

	// Ahora la captura automática vuelve a mirar el par y propone `pending`.
	if _, err := e.DetectRelations("b", detectOnlyOpts(true)); err != nil {
		t.Fatalf("re-detección automática: %v", err)
	}
	all, _ := e.AllObsRelations()
	if len(all) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
	}
	if all[0].Status != RelStatusResolved || all[0].Relation != RelRelated {
		t.Errorf("la detección degradó un veredicto del propio detector a %s/%s: el par vuelve a la"+
			" cola y se re-mira para siempre", all[0].Relation, all[0].Status)
	}
}

// `confidence` NO es columna del veredicto, y protegerla sería peor que no hacer nada. El único
// escritor de veredictos —ResolveObsRelation— escribe relation, status, resolved_by y reason, y NO
// la toca: confidence es señal DERIVADA de la detección (en una pendiente es max(léxico, coseno) y en
// una auto-resuelta el léxico solo). Si la guarda la congelara, la fila afirmaría una similitud que
// no es la medida ahora, y braingraph/pulse la leen sin filtrar por status.
func TestLaDeteccionRefrescaLaConfianzaDeUnaFilaResuelta(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelRelated, Status: RelStatusResolved,
		ResolvedBy: "agent", Reason: "juzgada", Confidence: 0.30,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending, Confidence: 0.91,
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if len(all) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
	}
	if all[0].Status != RelStatusResolved {
		t.Fatalf("el veredicto debía sobrevivir, quedó %q", all[0].Status)
	}
	if all[0].Confidence != 0.91 {
		t.Errorf("confidence debía refrescarse a 0.91, quedó %v: la fila afirma una similitud que"+
			" no es la que se midió ahora", all[0].Confidence)
	}
}
