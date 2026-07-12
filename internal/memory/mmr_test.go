package memory

import "testing"

// mmrEngine: engine con procedencia de vectores (sin model_id no hay vectores legibles).
func mmrEngine(t *testing.T) *DbEngine {
	t.Helper()
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@mmr")
	return e
}

func mmrIDs(scored []scoredCandidate) []string {
	out := make([]string, len(scored))
	for i, s := range scored {
		out[i] = s.id
	}
	return out
}

// escenario: A y B son CASI IDÉNTICOS (coseno 0.98); C es distinto y algo menos relevante que B.
// Sin diversidad el orden es A, B, C. Con diversidad, C le gana el 2º puesto al clon.
func setupClones(t *testing.T, e *DbEngine) []scoredCandidate {
	t.Helper()
	saveWithVec(t, e, "A", "t/a", "aaa", vecAt(1.0))  // el más relevante
	saveWithVec(t, e, "B", "t/b", "bbb", vecAt(0.98)) // clon de A
	saveWithVec(t, e, "C", "t/c", "ccc", vecAt(0.30)) // ajeno a A
	return []scoredCandidate{
		{candidate: candidate{id: "A"}, score: 0.100},
		{candidate: candidate{id: "B"}, score: 0.090},
		{candidate: candidate{id: "C"}, score: 0.080},
	}
}

func TestMMRElClonCedeSuLugar(t *testing.T) { // S.a
	e := mmrEngine(t)
	scored := setupClones(t, e)

	got := mmrIDs(e.diversify(scored, 0.75))
	want := []string{"A", "C", "B"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("el clon de A debía ceder el 2º puesto a C, que aporta info nueva.\n"+
				"  quería %v\n  obtuve %v", want, got)
		}
	}
}

func TestMMRNoDescartaNada(t *testing.T) { // S.b — el invariante
	e := mmrEngine(t)
	scored := setupClones(t, e)

	got := e.diversify(scored, 0.75)
	if len(got) != len(scored) {
		t.Fatalf("MMR REORDENA, NO DESCARTA: entraron %d y salieron %d", len(scored), len(got))
	}
	visto := map[string]bool{}
	for _, s := range got {
		visto[s.id] = true
	}
	for _, id := range []string{"A", "B", "C"} {
		if !visto[id] {
			t.Errorf("%s desapareció: MMR baja de posición, nunca descarta", id)
		}
	}
}

func TestMMRApagadoEsBitIdentico(t *testing.T) { // S.c
	e := mmrEngine(t)
	scored := setupClones(t, e)
	orig := mmrIDs(scored)

	for _, lambda := range []float64{1.0, 1.5, 0, -1} {
		got := mmrIDs(e.diversify(scored, lambda))
		for i := range orig {
			if got[i] != orig[i] {
				t.Fatalf("λ=%v debe APAGAR MMR (el 0 es el cero de Go: un caller distraído no puede"+
					" recibir 'diversidad pura').\n  quería %v\n  obtuve %v", lambda, orig, got)
			}
		}
	}
}

func TestMMRSinVectorNoSeCastiga(t *testing.T) { // S.d
	e := mmrEngine(t)
	saveWithVec(t, e, "A", "t/a", "aaa", vecAt(1.0))
	saveWithVec(t, e, "B", "t/b", "bbb", vecAt(0.98)) // clon de A ⇒ debe bajar
	// C se guarda SIN vector: no se lo puede castigar por una razón ajena a su contenido.
	if err := e.SaveObservation("C", "t/c", "ccc", nil); err != nil {
		t.Fatal(err)
	}
	scored := []scoredCandidate{
		{candidate: candidate{id: "A"}, score: 0.100},
		{candidate: candidate{id: "B"}, score: 0.090},
		{candidate: candidate{id: "C"}, score: 0.080},
	}

	got := mmrIDs(e.diversify(scored, 0.75))
	if got[1] != "C" {
		t.Fatalf("C no tiene vector ⇒ penalización 0 ⇒ le gana el 2º puesto al clon B."+
			" Castigarlo sería enterrarlo por no tener embedding. Obtuve %v", got)
	}
}

func TestRedundanciaNoCastigaLaLineaDeBase(t *testing.T) { // S.e
	for _, tc := range []struct {
		cos  float64
		want float64
		nota string
	}{
		{0.60, 0, "la MEDIANA del corpus: dos memorias cualesquiera. No es redundancia, es el idioma"},
		{0.40, 0, "por debajo de la base: menos parecidas que el promedio"},
		{1.00, 1, "duplicado exacto"},
		{0.80, 0.5, "a mitad de camino entre la base y el duplicado"},
	} {
		if got := redundancy(tc.cos); got < tc.want-0.001 || got > tc.want+0.001 {
			t.Errorf("redundancy(%.2f) = %.3f, quería %.3f (%s)", tc.cos, got, tc.want, tc.nota)
		}
	}
}

func TestMMRElPrimeroEsElMasRelevante(t *testing.T) { // S.f
	e := mmrEngine(t)
	scored := setupClones(t, e)

	got := e.diversify(scored, 0.5) // incluso con diversidad AGRESIVA
	if got[0].id != "A" {
		t.Fatalf("el primero elegido debe ser SIEMPRE el más relevante (no hay nada elegido contra"+
			" qué penalizar). Obtuve %q", got[0].id)
	}
}

// TestMMRCorreSinModelID pinea el bug que dejó al primer barrido de λ midiendo NADA: sin
// SetVectorModelID (model_id = ""), vectorsFor cortaba temprano y diversify quedaba INERTE EN
// SILENCIO. El filtro por model_id ya garantiza la procedencia: si es "", TODOS los embeddings lo
// tienen y compararlos es legítimo.
func TestMMRCorreSinModelID(t *testing.T) {
	e := newTestEngine(t) // <-- SIN SetVectorModelID, como el harness de evaluación
	scored := setupClones(t, e)

	got := mmrIDs(e.diversify(scored, 0.75))
	if got[1] != "C" {
		t.Fatalf("MMR debe funcionar aunque el model_id sea \"\" (el filtro ya garantiza la"+
			" procedencia). Si no, queda INERTE EN SILENCIO y cualquier medición miente. Obtuve %v", got)
	}
}
