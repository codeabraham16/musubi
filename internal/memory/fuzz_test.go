package memory

import (
	"math"
	"testing"
)

// fuzz_test.go endurece los PARSERS/tokenizers model-free que procesan entrada NO confiable
// (queries de usuario, expresiones `when`/`repeat_while` del agente, texto arbitrario para
// similitud). El invariante universal: NUNCA panic; además propiedades básicas (rango, simetría,
// determinismo). El corpus semilla también corre como test normal en `go test`.

// FuzzSimilarity: la similitud (Jaccard de trigramas) DEBE estar en [0,1], ser simétrica y no
// producir NaN ni panic para ningún par de strings.
func FuzzSimilarity(f *testing.F) {
	f.Add("el deploy falló", "la publicación no funcionó")
	f.Add("", "")
	f.Add("aaa", "aaa")
	f.Add("Go", "G")
	f.Add("漢字 é ñ", "の \x00 \xff")
	f.Fuzz(func(t *testing.T, a, b string) {
		s := Similarity(a, b)
		if math.IsNaN(s) || s < 0 || s > 1 {
			t.Fatalf("Similarity fuera de [0,1] o NaN: %v (a=%q b=%q)", s, a, b)
		}
		if r := Similarity(b, a); r != s {
			t.Fatalf("Similarity no simétrica: S(a,b)=%v S(b,a)=%v", s, r)
		}
	})
}

// FuzzEvalCondition: el parser de expresiones (when/repeat_while) DEBE ser determinista y no
// panicar ante cualquier expresión, incluida basura. Un error de sintaxis es válido; un panic no.
func FuzzEvalCondition(f *testing.F) {
	f.Add("step.a.status == done")
	f.Add("step.a.status == done and step.b.result contains ok")
	f.Add("not (step.a.status != done)")
	f.Add("")
	f.Add("(((")
	f.Add("a == == b")
	ctx := map[string]string{"step.a.status": "done", "step.b.result": "todo ok"}
	f.Fuzz(func(t *testing.T, expr string) {
		r1, e1 := EvalCondition(expr, ctx)
		r2, e2 := EvalCondition(expr, ctx)
		if r1 != r2 || (e1 == nil) != (e2 == nil) {
			t.Fatalf("EvalCondition no determinista para %q: (%v,%v) vs (%v,%v)", expr, r1, e1, r2, e2)
		}
	})
}

// FuzzBuildFTSQuery: los constructores de query FTS DEBEN tolerar cualquier entrada del usuario
// (puntuación, unicode, bytes nulos) sin panicar. buildFTSQueryRanked cae a buildFTSQuery si todo
// era ruido — nunca debe devolver un MATCH inválido que rompa a SQLite (acá sólo chequeamos que
// no panique la construcción; la validez FTS la cubren los tests de recall).
func FuzzBuildFTSQuery(f *testing.F) {
	f.Add("kubernetes deploy N+1")
	f.Add("")
	f.Add("的 é ñ 漢字 \x00")
	f.Add("the of a on in")
	f.Fuzz(func(t *testing.T, q string) {
		_ = buildFTSQuery(q)
		_ = buildFTSQueryRanked(q)
	})
}
