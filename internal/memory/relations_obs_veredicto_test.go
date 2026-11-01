package memory

import (
	"context"
	"database/sql"
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
// explícito sí tiene que pisar al anterior, SEA DE QUIEN SEA EL ANTERIOR. Es el camino de
// RestoreDuplicate (deshacer una fusión marca el par not_duplicate en el mismo commit); si esto se
// rompe, deshacer una fusión dejaría el veredicto viejo y nada se quejaría.
//
// La firma del anterior se recorre a propósito: con el caso `heuristic` solo, la guarda podía
// endurecerse hasta congelar también los veredictos HUMANOS y este test seguía verde, porque nunca
// le ponía delante uno que no fuera del detector.
func TestUnVeredictoExplicitoSiPisaAlAnterior(t *testing.T) {
	for _, c := range []struct{ nombre, firmaPrevia, firmaNueva string }{
		{"sobre uno del detector", RelResolvedByHeuristic, "user"},
		{"sobre uno humano", "user", "agent"},
		{"sobre uno de otro agente", "afilador", "user"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			e := newTestEngine(t)
			for _, id := range []string{"a", "b"} {
				if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := e.UpsertObsRelation(ObsRelation{
				SourceID: "a", TargetID: "b", Relation: RelSupersedes,
				Status: RelStatusResolved, ResolvedBy: c.firmaPrevia, Reason: "la más nueva reemplaza",
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := e.UpsertObsRelation(ObsRelation{
				SourceID: "a", TargetID: "b", Relation: RelNotDuplicate, Status: RelStatusResolved,
				ResolvedBy: c.firmaNueva, Reason: "fusión deshecha", Confidence: 1.0,
			}); err != nil {
				t.Fatal(err)
			}
			all, _ := e.AllObsRelations()
			if len(all) != 1 {
				t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
			}
			if all[0].Relation != RelNotDuplicate || all[0].ResolvedBy != c.firmaNueva ||
				all[0].Reason != "fusión deshecha" {
				t.Errorf("un veredicto explícito debe pisar al anterior, quedó: %+v", all[0])
			}
		})
	}
}

// LA FILA SIN FIRMA TAMBIÉN ES UN VEREDICTO, y es el caso que la guarda protege con un COALESCE que
// se lee como decorativo y es PORTANTE: `resolved_by` es NULL-able, así que sin el COALESCE la
// comparación `observation_relations.resolved_by<>'heuristic'` devuelve NULL, el CASE cae al ELSE y
// el detector pisa la fila. Con él, un NULL cuenta como «no es del detector» y queda protegida.
//
// Que hoy la base viva no tenga ninguna fila así (medido el 2026-11-01: 0 de 1098) NO vuelve el
// invariante teórico: `nullable("")` escribe NULL cada vez que un caller resuelve sin firmar, y la
// guarda es fail-open por ese lado. El custodio existe para que el día que aparezca una, la
// protección no se haya caído sin que nadie lo note — antes de esto, sabotear el COALESCE dejaba
// los diez tests de conducta en verde y lo delataba sólo un golden del texto del SQL.
func TestElDetectorNoPisaUnVeredictoSinFirma(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "topic/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelRelated, Status: RelStatusResolved,
		ResolvedBy: "", Reason: "arbitrada por un caller que no firmó",
	}); err != nil {
		t.Fatal(err)
	}
	var firma sql.NullString
	if err := e.db.QueryRow(
		`SELECT resolved_by FROM observation_relations`).Scan(&firma); err != nil {
		t.Fatal(err)
	}
	if firma.Valid {
		t.Fatalf("precondición: la firma debía quedar NULL en la base, quedó %q", firma.String)
	}

	// El detector llega con su propio veredicto resuelto, que es la mitad 2 de la guarda.
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelSupersedes, Status: RelStatusResolved,
		ResolvedBy: RelResolvedByHeuristic, Reason: "la más nueva reemplaza",
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if len(all) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(all))
	}
	if all[0].Relation != RelRelated || all[0].Reason != "arbitrada por un caller que no firmó" {
		t.Errorf("el detector pisó un veredicto sin firma, quedó: %+v", all[0])
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
	viejoLex, viejoCos := 0.42, 0.11
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelRelated, Status: RelStatusResolved,
		ResolvedBy: "agent", Reason: "juzgada", Lex: &viejoLex, Cosine: &viejoCos,
	}); err != nil {
		t.Fatal(err)
	}
	nuevoLex, nuevoCos := 0.91, 0.77
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending,
		Lex: &nuevoLex, Cosine: &nuevoCos,
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := e.AllObsRelations()
	if all[0].Status != RelStatusResolved {
		t.Fatalf("el veredicto debía sobrevivir, quedó %q", all[0].Status)
	}
	// LAS DOS SEÑALES, cada una con su propio valor. Medirlas juntas con el mismo número no
	// distingue «se refrescaron las dos» de «se refrescó una y la otra copió». Y medir sólo `lex`
	// —como hacía este test— dejaba el COALESCE de `cosine_score` sin un solo custodio: congelarlo
	// a mano pasaba el paquete entero en verde.
	if all[0].Lex == nil || *all[0].Lex != nuevoLex {
		t.Errorf("lex_score debía refrescarse a %v, quedó %v", nuevoLex, all[0].Lex)
	}
	if all[0].Cosine == nil || *all[0].Cosine != nuevoCos {
		t.Errorf("cosine_score debía refrescarse a %v, quedó %v", nuevoCos, all[0].Cosine)
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

	// La fecha se CLAVA en un literal viejo para poder preguntar si la re-detección la movió. Se
	// compara DENTRO de SQL a propósito: el driver modernc devuelve los DATETIME reformateados a
	// RFC3339, así que un literal sembrado acá y leído en Go nunca coincide y la comparación daría
	// «movida» siempre. Es un falso positivo que ya costó una vuelta de revisión.
	const fechaVieja = "2020-01-01 00:00:00"
	if _, err := e.db.Exec(`UPDATE observation_relations SET updated_at=?`, fechaVieja); err != nil {
		t.Fatal(err)
	}
	fechaIntacta := func() bool {
		t.Helper()
		var n int
		if err := e.db.QueryRow(
			`SELECT COUNT(*) FROM observation_relations WHERE updated_at=?`, fechaVieja,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	// El camino EXPLÍCITO, el que sí auto-supersede: es el que vuelve a decidir supersedes. Y es el
	// de PRODUCCIÓN —detectAndSurface corre con DetectOnly=false en cada save_observation—, así que
	// las cinco columnas del veredicto se miden por acá y no sólo por el camino de sólo-detectar.
	rels, err := e.DetectRelations("c2", detectOnlyOpts(false))
	if err != nil {
		t.Fatalf("re-detección explícita: %v", err)
	}
	if len(rels) != 1 || rels[0].Relation != RelRelated || rels[0].Status != RelStatusResolved {
		t.Fatalf("el asiento debía seguir siendo related/resolved, quedó %+v", rels)
	}
	// LA FIRMA Y LA RAZÓN SON PARTE DEL VEREDICTO, y aseverar sólo relation/status las dejaba sin
	// custodio: con la guarda recortada a su primera mitad, el asiento conserva `related` pero pasa
	// a estar firmado `heuristic` con la razón del detector. Ahí la relación deja de ser de quien la
	// arbitró —y como la mitad 2 sólo protege al que NO firmó heuristic, la re-detección siguiente ya
	// la pisa libremente, que es el defecto original volviendo por la puerta de atrás.
	if rels[0].ResolvedBy != "user" || rels[0].Reason != "no se tapan" {
		t.Errorf("la firma y la razón del que arbitró debían sobrevivir, quedó %q/%q",
			rels[0].ResolvedBy, rels[0].Reason)
	}
	if !fechaIntacta() {
		t.Errorf("la re-detección por el camino de producción movió updated_at: readiness lo lee" +
			" como la fecha del veredicto y contaría un arbitraje que nadie hizo")
	}
	// LO QUE IMPORTA: la observación sigue en el recall. Un asiento que dice `related` y una
	// observación oculta son un cerebro que esconde memoria sin poder decir por qué.
	assertNotSuperseded(t, e, "c1")

	// LA SEGUNDA VUELTA ES LA QUE DUELE, y por eso va: si la primera dejó la fila firmada
	// `heuristic`, la mitad 2 ya no la reconoce como ajena y esta vuelta la pisa con supersedes —
	// c1 termina OCULTA. Con una sola re-detección el daño es invisible en relation/status.
	if _, err := e.DetectRelations("c2", detectOnlyOpts(false)); err != nil {
		t.Fatalf("segunda re-detección explícita: %v", err)
	}
	todas, _ := e.AllObsRelations()
	if len(todas) != 1 || todas[0].Relation != RelRelated || todas[0].ResolvedBy != "user" {
		t.Errorf("tras dos re-detecciones el veredicto debía seguir intacto, quedó %+v", todas)
	}
	if !fechaIntacta() {
		t.Errorf("la segunda re-detección movió updated_at")
	}
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
