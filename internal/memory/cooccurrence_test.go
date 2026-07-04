package memory

import (
	"context"
	"strings"
	"testing"
)

// TestExtractExpansionTerms: la extracción es determinista, respeta df≥2, excluye query+stopwords
// y ordena por (docFreq desc, término asc). Función pura.
func TestExtractExpansionTerms(t *testing.T) {
	docs := []string{
		"deploy a produccion fallo en el despliegue",
		"otro deploy con despliegue en produccion",
		"deploy y despliegue del sistema",
	}
	terms := extractExpansionTerms(docs, map[string]bool{"deploy": true})
	// despliegue (df=3), produccion (df=2) califican; deploy=query, otro/fallo/sistema df=1.
	if len(terms) == 0 || terms[0] != "despliegue" {
		t.Fatalf("esperaba 'despliegue' primero, obtuve %v", terms)
	}
	got := strings.Join(terms, ",")
	if got != "despliegue,produccion" {
		t.Errorf("expansión inesperada: %q", got)
	}
	// Determinismo.
	if s2 := strings.Join(extractExpansionTerms(docs, map[string]bool{"deploy": true}), ","); s2 != got {
		t.Errorf("no determinista: %q vs %q", got, s2)
	}
	// Un corpus sin co-ocurrencia (todos términos únicos) ⇒ sin expansión.
	if e := extractExpansionTerms([]string{"alpha", "beta", "gamma"}, nil); len(e) != 0 {
		t.Errorf("sin co-ocurrencia no debe haber expansión, obtuve %v", e)
	}
}

// saveObs guarda una observación simple (sin embedding) y devuelve su id.
func saveObsC(t *testing.T, e *DbEngine, id, topic, content string) {
	t.Helper()
	if err := e.SaveObservation(id, topic, content, nil); err != nil {
		t.Fatalf("SaveObservation %s: %v", id, err)
	}
}

func recallIDs(t *testing.T, e *DbEngine, query string, cooc bool) map[string]bool {
	t.Helper()
	res, err := e.Recall(context.Background(), query, RecallOptions{TokenBudget: 4000, Cooccurrence: cooc, NoBump: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	ids := map[string]bool{}
	for _, it := range res.Items {
		ids[it.ID] = true
	}
	return ids
}

// Escenario (a) PUENTE DE VOCABULARIO: 'deploy' y 'despliegue' co-ocurren en varias obs; una obs
// dice sólo 'despliegue' (sin 'deploy'). Con Cooccurrence off no aparece para la query 'deploy';
// con Cooccurrence on, la expansión PRF la trae.
func TestCooccurrenceBridgesVocabulary(t *testing.T) {
	e := newTestEngine(t)
	saveObsC(t, e, "o1", "ops", "el deploy a produccion fallo en el despliegue nocturno")
	saveObsC(t, e, "o2", "ops", "otro deploy con problemas de despliegue en produccion")
	saveObsC(t, e, "o3", "ops", "revisamos el deploy y el despliegue del sistema")
	// Obs objetivo: dice 'despliegue' pero NO 'deploy'.
	saveObsC(t, e, "target", "ops", "el despliegue nocturno quedo pausado sin avisar")
	// Ruido no relacionado.
	saveObsC(t, e, "noise", "otros", "receta de cocina con tomate y albahaca")

	off := recallIDs(t, e, "deploy", false)
	if off["target"] {
		t.Fatalf("sin cooc, la obs que sólo dice 'despliegue' no debería aparecer para 'deploy'")
	}
	on := recallIDs(t, e, "deploy", true)
	if !on["target"] {
		t.Errorf("con cooc, la expansión PRF debería PUENTEAR 'deploy'→'despliegue' y traer 'target'; obtuve %v", on)
	}
	if on["noise"] {
		t.Errorf("la expansión no debería traer ruido no co-ocurrente ('noise')")
	}
}

// Escenario (c) EQUIVALENCIA: con Cooccurrence off el resultado es el histórico; y cuando no hay
// términos de expansión, on == off.
func TestCooccurrenceNoOpEquivalence(t *testing.T) {
	e := newTestEngine(t)
	// Términos únicos: ninguna co-ocurrencia ⇒ sin expansión ⇒ on == off.
	saveObsC(t, e, "u1", "x", "alpha unico primero")
	saveObsC(t, e, "u2", "x", "beta distinto segundo")
	saveObsC(t, e, "u3", "x", "gamma aparte tercero")
	off := recallIDs(t, e, "alpha", false)
	on := recallIDs(t, e, "alpha", true)
	if len(off) != len(on) {
		t.Errorf("sin co-ocurrentes, cooc on/off deben coincidir: off=%v on=%v", off, on)
	}
	for id := range off {
		if !on[id] {
			t.Errorf("divergencia en no-op: %s en off pero no en on", id)
		}
	}
}
