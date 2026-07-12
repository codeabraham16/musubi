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
		{"áéí", 2},      // 3 no-ASCII -> ceil(3/2.0)=2 (antes subcontaba a 1)
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

func TestEstimateTokensSegmentsMarkdown(t *testing.T) {
	// Un blob mayormente prosa con UN bloque de código no debe estimarse como si TODO
	// fuera código (/3.4): la segmentación da MENOS que el blob-completo-código y no
	// menos que el blob-completo-prosa. Arregla la sobre-estimación que recortaba recall.
	md := "Esta es una explicación en prosa sobre el despliegue del sistema y su diseño.\n```\nfor i := range xs { total += xs[i] * factor }\n```\nY después seguimos con más prosa explicando el resultado y las decisiones."
	seg := EstimateTokens(md)
	allCode := estimateTokensFor(md, kindCode)
	allProse := estimateTokensFor(md, kindProse)
	if seg >= allCode {
		t.Errorf("segmentado (%d) debería estimar MENOS que todo-código (%d)", seg, allCode)
	}
	if seg < allProse {
		t.Errorf("segmentado (%d) no debería estimar menos que todo-prosa (%d)", seg, allProse)
	}
}

// El peso no-ASCII no debe romper el sesgo conservador: prosa acentuada estima >= que
// la misma cantidad de ASCII puro (nunca sub-cuenta el español).
func TestEstimateTokensAccentedNotUnderAscii(t *testing.T) {
	accented := EstimateTokens("configuración migración inyección después")
	ascii := EstimateTokens("configuracion migracion inyeccion despues")
	if accented < ascii {
		t.Errorf("prosa acentuada (%d) no debe estimar menos que la ASCII equivalente (%d)", accented, ascii)
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

// Este test ANTES exigía que el gist se quedara en la primera oración — o sea, pineaba el bug:
// un gist de 8 tokens con un techo de 24 abandonaba 16 sin intentar decir nada más, y quedaban
// gists mudos como "SDD tasks — brain-dashboard BACKEND." (110 de 461 en la memoria real).
//
// El gist existe para que el agente decida SI VALE LA PENA EXPANDIR. Uno que no deja decidir cuesta
// tokens y obliga a expandir igual: se paga DOS VECES.
func TestGistLlenaSuTecho(t *testing.T) {
	const texto = "Primera oración. Segunda oración que sobra."

	t.Run("con techo de sobra, suma oraciones", func(t *testing.T) { // S.a
		g := Gist(texto, 50)
		if g != texto {
			t.Errorf("con 50 tokens de techo entran las DOS oraciones; el gist no debe abandonar"+
				" presupuesto. Obtuve %q", g)
		}
	})

	t.Run("no agrega una oración que no entra COMPLETA", func(t *testing.T) { // S.c
		// Techo justo para la 1ª: la 2ª no entra entera ⇒ no se agrega a medias.
		g := Gist(texto, EstimateTokens("Primera oración."))
		if g != "Primera oración." {
			t.Errorf("una oración que no entra completa NO se agrega: un gist cortado al medio"+
				" tampoco deja decidir. Obtuve %q", g)
		}
	})

	t.Run("nunca excede el techo", func(t *testing.T) { // S.b
		for _, techo := range []int{4, 8, 12, 24, 50} {
			if got := EstimateTokens(Gist(texto, techo)); got > techo {
				t.Errorf("techo %d: el gist usó %d tokens", techo, got)
			}
		}
	})

	t.Run("una sola oración: no se inventa nada", func(t *testing.T) { // S.e
		if g := Gist("Única oración.", 50); g != "Única oración." {
			t.Errorf("no hay más texto que agregar; obtuve %q", g)
		}
	})
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
