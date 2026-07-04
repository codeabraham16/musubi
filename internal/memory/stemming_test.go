package memory

import (
	"context"
	"testing"
)

// TestStemForPrefix: el stem es conservador y determinista.
func TestStemForPrefix(t *testing.T) {
	cases := map[string]string{
		"go":           "go",     // <5 runas: intacto
		"casa":         "casa",   // 4 runas: intacto (no stemmea)
		"casas":        "casa",   // -s, raíz 4 ✓
		"deploys":      "deploy", // -s
		"deployment":   "deploy", // -ment
		"deployments":  "deploy", // -ments
		"corriendo":    "corr",   // -iendo, raíz 4 ✓
		"aplicaciones": "aplic",  // -aciones, raíz 5 ✓
	}
	for in, want := range cases {
		if got := stemForPrefix(in); got != want {
			t.Errorf("stemForPrefix(%q) = %q, esperaba %q", in, got, want)
		}
	}
	// Determinismo.
	if stemForPrefix("deployments") != stemForPrefix("deployments") {
		t.Error("stemForPrefix no determinista")
	}
	// Nunca recorta por debajo de 4 runas (no over-stemmea 'ados' de 'todos'→'t').
	if got := stemForPrefix("todos"); len([]rune(got)) < 4 {
		t.Errorf("no debe recortar bajo 4 runas: todos → %q", got)
	}
}

func recallIDsStem(t *testing.T, e *DbEngine, query string, stem bool) map[string]bool {
	t.Helper()
	res, err := e.Recall(context.Background(), query, RecallOptions{TokenBudget: 4000, Stemming: stem, NoBump: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	ids := map[string]bool{}
	for _, it := range res.Items {
		ids[it.ID] = true
	}
	return ids
}

// Escenario (a): variantes morfológicas de sufijo. Query 'deploy' con Stemming off matchea sólo el
// token exacto 'deploy'; con on ('deploy'*) matchea 'deploys' y 'deployment' también.
func TestStemmingMatchesMorphologicalVariants(t *testing.T) {
	e := newTestEngine(t)
	saveObsC(t, e, "exact", "ops", "el deploy salio bien anoche")
	saveObsC(t, e, "plural", "ops", "los deploys fallaron en cadena")
	saveObsC(t, e, "noun", "ops", "el deployment quedo pausado sin avisar")
	saveObsC(t, e, "noise", "cocina", "receta con tomate y albahaca fresca")

	off := recallIDsStem(t, e, "deploy", false)
	if !off["exact"] {
		t.Fatal("sin stemming, 'deploy' debe matchear el token exacto 'deploy'")
	}
	if off["plural"] || off["noun"] {
		t.Errorf("sin stemming, 'deploy' NO debe matchear 'deploys'/'deployment'; obtuve %v", off)
	}
	on := recallIDsStem(t, e, "deploy", true)
	if !on["exact"] || !on["plural"] || !on["noun"] {
		t.Errorf("con stemming, 'deploy' debe matchear deploy/deploys/deployment; obtuve %v", on)
	}
	if on["noise"] {
		t.Errorf("el prefijo no debe traer ruido no relacionado ('noise')")
	}
}
