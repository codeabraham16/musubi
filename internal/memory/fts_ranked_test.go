package memory

import (
	"context"
	"strings"
	"testing"
)

// TestBuildFTSQueryRanked verifica que la variante ponderada (T5.6) descarta stopwords y
// tokens de 1 runa, pero PRESERVA entidades cortas significativas (Go, DB, API).
func TestBuildFTSQueryRanked(t *testing.T) {
	got := buildFTSQueryRanked("fixed N+1 query in UserList")
	// 'in' (stopword) y 'N','1' (1 runa) se descartan; quedan los significativos.
	for _, want := range []string{`"fixed"`, `"query"`, `"UserList"`} {
		if !strings.Contains(got, want) {
			t.Errorf("debía conservar %s, obtuve %q", want, got)
		}
	}
	for _, no := range []string{`"in"`, `"N"`, `"1"`} {
		if strings.Contains(got, no) {
			t.Errorf("debía descartar %s, obtuve %q", no, got)
		}
	}

	// Entidades cortas (>=2 runas, no stopwords) se preservan.
	ent := buildFTSQueryRanked("Go DB API")
	for _, want := range []string{`"Go"`, `"DB"`, `"API"`} {
		if !strings.Contains(ent, want) {
			t.Errorf("debía preservar la entidad %s, obtuve %q", want, ent)
		}
	}

	// Consulta toda de ruido: fallback a buildFTSQuery (no perder recall).
	noise := buildFTSQueryRanked("de la el a")
	if noise == "" {
		t.Errorf("una consulta toda de stopwords debe caer al fallback no vacío, obtuve %q", noise)
	}

	// Sin términos alfanuméricos: vacío (igual que buildFTSQuery).
	if empty := buildFTSQueryRanked("!!! ???"); empty != "" {
		t.Errorf("sin términos debe devolver vacío, obtuve %q", empty)
	}
}

// TestBuildFTSQueryRankedPrefix combina el filtrado de ruido con el prefijo de la raíz.
func TestBuildFTSQueryRankedPrefix(t *testing.T) {
	got := buildFTSQueryRankedPrefix("los deploys de la app")
	if !strings.Contains(got, `"deploy"*`) {
		t.Errorf("debía emitir el prefijo de la raíz de 'deploys', obtuve %q", got)
	}
	if !strings.Contains(got, `"app"*`) {
		t.Errorf("debía conservar 'app' como prefijo, obtuve %q", got)
	}
	for _, no := range []string{`"los"`, `"de"`, `"la"`} {
		if strings.Contains(got, no) {
			t.Errorf("debía descartar el stopword %s, obtuve %q", no, got)
		}
	}
}

// TestRecallRankedFTSFiltersStopwordNoise: con RankedFTS, una obs que solo comparte
// stopwords con la query deja de traerse (el ruido que diluía el recall por turno).
func TestRecallRankedFTSFiltersStopwordNoise(t *testing.T) {
	e := newTestEngine(t)
	saveObsC(t, e, "rel", "ops", "el despliegue del sistema fue exitoso")
	saveObsC(t, e, "noise", "cocina", "la receta de la abuela con el tomate y la sal")
	get := func(ranked bool) map[string]bool {
		res, err := e.Recall(context.Background(), "el despliegue de la app",
			RecallOptions{TokenBudget: 4000, RankedFTS: ranked, NoBump: true})
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, it := range res.Items {
			ids[it.ID] = true
		}
		return ids
	}
	if !get(false)["noise"] {
		t.Fatal("precondición: sin ranked, los stopwords 'el/de/la' deberían traer el ruido")
	}
	r := get(true)
	if !r["rel"] {
		t.Error("ranked debe traer la obs relevante ('despliegue')")
	}
	if r["noise"] {
		t.Errorf("ranked no debe traer el ruido de puros stopwords, obtuve %v", r)
	}
}
