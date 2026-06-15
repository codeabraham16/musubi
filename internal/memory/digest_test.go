package memory

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"abcd", 1},     // 4 runas -> ceil(4/4)=1
		{"abcde", 2},    // 5 runas -> ceil(5/4)=2
		{"abcdefgh", 2}, // 8 runas -> 2
		{"áéí", 1},      // 3 runas multibyte -> 1
	}
	for _, c := range cases {
		if got := EstimateTokens(c.in); got != c.want {
			t.Errorf("EstimateTokens(%q) = %d, quiero %d", c.in, got, c.want)
		}
	}

	// Monotonía: más texto nunca da menos tokens.
	if EstimateTokens("hola mundo cruel") < EstimateTokens("hola") {
		t.Error("EstimateTokens no es monótono")
	}
}

func TestContentHashStableAndNormalized(t *testing.T) {
	a := ContentHash("hola mundo")
	b := ContentHash("  hola   mundo  ") // mismo contenido tras normalizar espacios
	if a != b {
		t.Errorf("hash debería ignorar espacios: %q vs %q", a, b)
	}
	if ContentHash("hola mundo") == ContentHash("otra cosa") {
		t.Error("contenidos distintos no deberían colisionar")
	}
	if len(a) != 64 {
		t.Errorf("esperaba hex sha256 de 64 chars, obtuve %d", len(a))
	}
}

func TestGistEmpty(t *testing.T) {
	if g := Gist("", 24); g != "" {
		t.Errorf("Gist(vacío) = %q, quiero \"\"", g)
	}
	if g := Gist("   \n  ", 24); g != "" {
		t.Errorf("Gist(espacios) = %q, quiero \"\"", g)
	}
}

func TestGistFirstSentence(t *testing.T) {
	g := Gist("Primera oración. Segunda oración que sobra.", 50)
	if g != "Primera oración." {
		t.Errorf("Gist debería tomar la primera oración, obtuve %q", g)
	}
}

func TestGistStripsMarkdownLead(t *testing.T) {
	g := Gist("## Encabezado importante", 50)
	if g != "Encabezado importante" {
		t.Errorf("Gist debería sacar el markdown inicial, obtuve %q", g)
	}
}

func TestGistTruncatesToBudget(t *testing.T) {
	long := strings.Repeat("palabra ", 100) // sin puntuación de oración
	g := Gist(long, 5)
	if EstimateTokens(g) > 5 {
		t.Errorf("gist excede el presupuesto: %d tokens (%q)", EstimateTokens(g), g)
	}
	if !strings.HasSuffix(g, "…") {
		t.Errorf("gist truncado debería terminar en elipsis, obtuve %q", g)
	}
	// No debe partir runas (UTF-8 válido).
	if !utf8.ValidString(g) {
		t.Errorf("gist no es UTF-8 válido: %q", g)
	}
}
