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

func TestClassifyContent(t *testing.T) {
	cases := []struct {
		in   string
		want contentKind
	}{
		{"Esto es una oración normal en prosa, sin nada raro.", kindProse},
		{"hola mundo", kindProse},
		{`{"name":"x","age":3,"items":[1,2,3]}`, kindJSON},
		{"  [\n  {\"a\": 1},\n  {\"b\": 2}\n]", kindJSON},
		{"func add(a, b int) int { return a + b }", kindCode},
		{"if (x > 0 && y < 10) { return x * y; }", kindCode},
	}
	for _, c := range cases {
		if got := classifyContent(c.in); got != c.want {
			t.Errorf("classifyContent(%q) = %v, quiero %v", c.in, got, c.want)
		}
	}
}

func TestEstimateTokensByContentTypeIsConservative(t *testing.T) {
	// A igualdad de texto, código y JSON deben estimar MÁS tokens que prosa
	// (densidad de símbolos): nunca subcontamos los payloads densos.
	const sample = "abcdefghij klmnopqrst uvwxyzabcd efghijklmn"
	prose := estimateTokensFor(sample, kindProse)
	code := estimateTokensFor(sample, kindCode)
	json := estimateTokensFor(sample, kindJSON)
	if !(json >= code && code >= prose) {
		t.Errorf("esperaba json>=code>=prose, obtuve prose=%d code=%d json=%d", prose, code, json)
	}
	if code <= prose {
		t.Errorf("código debe estimar más tokens que prosa: code=%d prose=%d", code, prose)
	}
}

func TestEstimateTokensCodeNotUnderBudgeted(t *testing.T) {
	// Un bloque de código real no debe estimarse por debajo de runas/3.4
	// (el viejo runas/4 lo subcontaba sistemáticamente).
	code := "for (let i = 0; i < items.length; i++) { total += items[i].price * qty; }"
	got := EstimateTokens(code)
	floor := int(float64(len([]rune(code))) / 3.5)
	if got < floor {
		t.Errorf("EstimateTokens(code)=%d subcuenta; esperaba >= %d", got, floor)
	}
}

func TestEstimateTokensCJK(t *testing.T) {
	// Cada carácter CJK pesa ~1 token; runas/4 lo subcontaría 4x.
	cjk := "你好世界你好世界" // 8 ideogramas
	got := EstimateTokens(cjk)
	if got < 6 {
		t.Errorf("EstimateTokens(CJK)=%d subcuenta; esperaba ~8 (>=6)", got)
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
