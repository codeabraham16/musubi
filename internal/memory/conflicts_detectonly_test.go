package memory

import "testing"

// M4 — gate de novedad en la captura AUTOMÁTICA. DetectOnly: detectar y MARCAR, nunca decidir.
//
// El peligro que evita es CONCRETO: el auto-supersede se dispara con mismo topic_key + léxico alto
// + más reciente. La captura automática mete TODO en un BALDE — un topic_key que no es un tema sino
// un cajón. Sin DetectOnly, dos entradas parecidas del mismo balde se auto-ocultarían entre sí.
//
// POR QUÉ EL BALDE DE ESTOS TESTS ES "error-fix" Y YA NO "git-commit". El motivador original de M4
// fueron los commits, pero los commits ya NO PUEDEN llegar hasta acá: la guarda estructural
// (complementaryPair) impide que un registro histórico sea DESTINO de una relación, así que entre
// dos commits no nace ninguna — el auto-supersede es inalcanzable ANTES de que DetectOnly opine. Los
// commits quedan protegidos DOS VECES, y la garantía fuerte es la estructural (la pinea
// TestCommitNoEsDestinoDeOtroCommit).
//
// Pero DetectOnly NO quedó sin trabajo: la telemetría guarda los error→fix en el balde "error-fix",
// que NO es un registro histórico. Ahí el auto-supersede sigue siendo posible y DetectOnly es LO
// ÚNICO que impide que un arreglo nuevo tape a uno viejo por parecerse. Por eso los tests apuntan al
// caso donde el flag es de verdad lo que sostiene la garantía: un test que sólo cubre un camino ya
// bloqueado río arriba no está probando nada.

const autoBucket = "error-fix" // el balde de la telemetría: NO es un registro histórico

func detectOnlyOpts(detectOnly bool) ConflictOptions {
	o := testOpts()
	o.DetectOnly = detectOnly
	return o
}

// Dos entradas MUY parecidas en el mismo balde de captura automática, la segunda más nueva.
func twoSimilarAutoCaptured(t *testing.T, e *DbEngine) {
	t.Helper()
	e.SetVectorModelID("static:tabla@abc")
	// Vectores casi paralelos: coseno alto, como dos arreglos que hablan del mismo error.
	if err := e.SaveObservationTyped("c1", autoBucket, "fix: typo en el README del proyecto", 0.5, "episodic", ScopeLocal, []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("c2", autoBucket, "fix: typo en el README del proyecto core", 0.5, "episodic", ScopeLocal, []float32{0.999, 0.04, 0}); err != nil {
		t.Fatal(err)
	}
	// Fechas DISTINTAS: el auto-supersede exige que la nueva sea ESTRICTAMENTE más reciente, y dos
	// saves en el mismo segundo empatan. En la vida real tienen fechas distintas, así que hay que
	// reproducir eso para que el escenario peligroso sea el de verdad.
	if _, err := e.db.Exec(`UPDATE observations SET created_at='2026-01-01T00:00:00Z' WHERE id='c1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at='2026-01-02T00:00:00Z' WHERE id='c2'`); err != nil {
		t.Fatal(err)
	}
}

// M4.b / R0 — CON DetectOnly: nada queda oculto, y el duplicado queda MARCADO pending.
func TestDetectOnlyNeverSupersedesAnAutoCapture(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

	rels, err := e.DetectRelations("c2", detectOnlyOpts(true))
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("el duplicado debería quedar MARCADO (hoy se guarda sin ninguna marca)")
	}
	for _, r := range rels {
		if r.Status != RelStatusPending {
			t.Errorf("con DetectOnly todo veredicto debe ser pending, obtuve %s/%s", r.Relation, r.Status)
		}
		if r.Relation == RelSupersedes && r.Status == RelStatusResolved {
			t.Error("R0 VIOLADO: la captura automática auto-ocultó una observación")
		}
	}
	// Lo que de verdad importa: NADA quedó fuera del recall.
	assertNotSuperseded(t, e, "c1")
	assertNotSuperseded(t, e, "c2")
}

// EL GEMELO — SIN DetectOnly, ese MISMO caso SÍ auto-supersede y oculta la entrada anterior. Prueba
// que el flag evita un peligro REAL, no hipotético. Si este test dejara de auto-superseder, el de
// arriba ya no estaría demostrando nada.
func TestWithoutDetectOnlyTheSameAutoCapturesWouldAutoSupersede(t *testing.T) {
	e := newTestEngine(t)
	twoSimilarAutoCaptured(t, e)

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
		t.Fatal("precondición del gemelo: sin DetectOnly, estas dos entradas del mismo balde DEBEN" +
			" auto-superseder. Si no lo hacen, el test de arriba dejó de demostrar que DetectOnly" +
			" evita un peligro real")
	}
	// Con el comportamiento de siempre, la entrada vieja QUEDA OCULTA: eso es lo que M4 evita.
	var sup *string
	if err := e.db.QueryRow(`SELECT superseded_by FROM observations WHERE id='c1'`).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup == nil {
		t.Fatal("precondición del gemelo: sin DetectOnly, c1 debería quedar superseded")
	}
	t.Logf("confirmado: SIN DetectOnly, c1 queda OCULTA (superseded_by=%s). M4 lo evita.", *sup)
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
	twoSimilarAutoCaptured(t, e)

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
