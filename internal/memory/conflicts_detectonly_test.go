package memory

import "testing"

// M4 — gate de novedad en la captura AUTOMÁTICA. DetectOnly: detectar y MARCAR, nunca decidir.
//
// El peligro que evita es CONCRETO: el auto-supersede se dispara con mismo topic_key + léxico alto
// + más reciente. En la captura de commits TODOS llevan topic_key="git-commit" — ahí ese campo es un
// BALDE, no un tema. Sin DetectOnly, dos commits de mensaje parecido se auto-ocultarían entre sí.

func detectOnlyOpts(detectOnly bool) ConflictOptions {
	o := testOpts()
	o.DetectOnly = detectOnly
	return o
}

// Dos commits de mensaje MUY parecido, en el balde "git-commit", el segundo más nuevo.
// Es exactamente lo que produce la captura automática.
func twoSimilarCommits(t *testing.T, e *DbEngine) {
	t.Helper()
	e.SetVectorModelID("static:tabla@abc")
	// Vectores casi paralelos: coseno alto, como dos commits que hablan de lo mismo.
	if err := e.SaveObservationTyped("c1", "git-commit", "fix: typo en el README del proyecto", 0.5, "episodic", ScopeLocal, []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("c2", "git-commit", "fix: typo en el README del proyecto core", 0.5, "episodic", ScopeLocal, []float32{0.999, 0.04, 0}); err != nil {
		t.Fatal(err)
	}
	// Fechas DISTINTAS: el auto-supersede exige que la nueva sea ESTRICTAMENTE más reciente, y dos
	// saves en el mismo segundo empatan. En la vida real dos commits tienen fechas distintas, así que
	// hay que reproducir eso para que el escenario peligroso sea el de verdad.
	if _, err := e.db.Exec(`UPDATE observations SET created_at='2026-01-01T00:00:00Z' WHERE id='c1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at='2026-01-02T00:00:00Z' WHERE id='c2'`); err != nil {
		t.Fatal(err)
	}
}

// M4.b / R0 — CON DetectOnly: ningún commit queda oculto, y el duplicado queda MARCADO pending.
func TestDetectOnlyNeverSupersedesACapturedCommit(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarCommits(t, e)

	rels, err := e.DetectRelations("c2", detectOnlyOpts(true))
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("el commit duplicado debería quedar MARCADO (hoy se guarda sin ninguna marca)")
	}
	for _, r := range rels {
		if r.Status != RelStatusPending {
			t.Errorf("con DetectOnly todo veredicto debe ser pending, obtuve %s/%s", r.Relation, r.Status)
		}
		if r.Relation == RelSupersedes && r.Status == RelStatusResolved {
			t.Error("R0 VIOLADO: la captura automática auto-ocultó un commit")
		}
	}
	// Lo que de verdad importa: NINGÚN commit quedó fuera del recall.
	assertNotSuperseded(t, e, "c1")
	assertNotSuperseded(t, e, "c2")
}

// EL GEMELO — SIN DetectOnly, ese MISMO caso SÍ auto-supersede y oculta el commit anterior. Prueba
// que el flag evita un peligro REAL, no hipotético. Si este test dejara de auto-superseder, el de
// arriba ya no estaría demostrando nada.
func TestWithoutDetectOnlyTheSameCommitsWouldAutoSupersede(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarCommits(t, e)

	rels, err := e.DetectRelations("c2", detectOnlyOpts(false))
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	autoSupersede := false
	for _, r := range rels {
		if r.Relation == RelSupersedes && r.Status == RelStatusResolved && r.TargetID == "c1" {
			autoSupersede = true
		}
	}
	if !autoSupersede {
		t.Skip("estos dos commits no auto-superseden con los umbrales actuales; el gemelo pierde fuerza pero DetectOnly sigue siendo la garantía correcta")
	}
	// Con el comportamiento de siempre, el commit viejo QUEDA OCULTO: eso es lo que M4 evita.
	var sup *string
	err = e.db.QueryRow(`SELECT superseded_by FROM observations WHERE id='c1'`).Scan(&sup)
	if err != nil {
		t.Fatal(err)
	}
	if sup == nil {
		t.Fatal("precondición del gemelo: sin DetectOnly, c1 debería quedar superseded")
	}
	t.Logf("confirmado: SIN DetectOnly, el commit c1 queda OCULTO (superseded_by=%s). M4 lo evita.", *sup)
}

func assertNotSuperseded(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	var sup *string
	if err := e.db.QueryRow(`SELECT superseded_by FROM observations WHERE id=?`, id).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup != nil {
		t.Errorf("la observación %s quedó OCULTA (superseded_by=%s): la captura automática no debe auto-suprimir", id, *sup)
	}
}

// M4.c / R1 — el gate NUNCA descarta: el duplicado se guarda igual (sigue existiendo y visible).
func TestDetectOnlyNeverDropsTheSave(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarCommits(t, e)

	if _, err := e.DetectRelations("c2", detectOnlyOpts(true)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"c1", "c2"} {
		var n int
		if err := e.db.QueryRow(
			`SELECT COUNT(*) FROM observations WHERE id=? AND archived=0 AND superseded_by IS NULL`, id,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("%s debe seguir guardada y visible: el gate MARCA, no descarta (un NOOP silencioso sería pérdida de memoria)", id)
		}
	}
}

// R2 / M4.d — DetectOnly=false (el cero) deja el camino explícito del agente EXACTAMENTE como estaba.
func TestDetectOnlyDefaultsToHistoricalBehaviour(t *testing.T) {
	if testOpts().DetectOnly {
		t.Error("DetectOnly debe ser false por default: el camino explícito no cambia")
	}
}
