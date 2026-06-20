package memory

import (
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
