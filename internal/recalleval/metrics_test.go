package recalleval

import (
	"math"
	"testing"
)

func rel(ids ...string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestRecallAtK(t *testing.T) {
	ranked := []string{"a", "b", "c", "d", "e"}
	relevant := rel("a", "c") // 2 relevantes
	cases := []struct {
		k    int
		want float64
	}{
		{1, 0.5}, // top-1 = [a] ⇒ 1 de 2
		{2, 0.5}, // top-2 = [a,b] ⇒ 1 de 2
		{3, 1.0}, // top-3 = [a,b,c] ⇒ 2 de 2
		{5, 1.0},
		{99, 1.0}, // k > len(ranked): se recorta
	}
	for _, c := range cases {
		if got := RecallAtK(ranked, relevant, c.k); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("RecallAtK(k=%d) = %v, quería %v", c.k, got, c.want)
		}
	}
	if got := RecallAtK(ranked, rel(), 3); got != 0 {
		t.Errorf("sin relevantes debería ser 0, obtuve %v", got)
	}
}

func TestReciprocalRank(t *testing.T) {
	if got := ReciprocalRank([]string{"a", "b", "c"}, rel("a")); got != 1.0 {
		t.Errorf("primer relevante en pos 1 ⇒ 1.0, obtuve %v", got)
	}
	if got := ReciprocalRank([]string{"x", "a", "b"}, rel("a")); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("primer relevante en pos 2 ⇒ 0.5, obtuve %v", got)
	}
	if got := ReciprocalRank([]string{"x", "y"}, rel("a")); got != 0 {
		t.Errorf("ningún relevante ⇒ 0, obtuve %v", got)
	}
}

func TestNDCGAtK(t *testing.T) {
	// relevant={a,c}. ranked=[a,b,c]: a@pos1 gana 1/log2(2)=1; c@pos3 gana 1/log2(4)=0.5.
	// DCG=1.5. IDCG (2 relevantes)=1/log2(2)+1/log2(3)=1+0.63093=1.63093. nDCG=0.91975.
	got := NDCGAtK([]string{"a", "b", "c"}, rel("a", "c"), 3)
	if math.Abs(got-0.91975) > 1e-4 {
		t.Errorf("nDCG@3 = %v, quería ~0.91975", got)
	}
	// Ranking perfecto (relevantes arriba) ⇒ nDCG = 1.
	if got := NDCGAtK([]string{"a", "c", "b"}, rel("a", "c"), 3); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("ranking perfecto ⇒ 1.0, obtuve %v", got)
	}
	// Sin relevantes ⇒ 0 (IDCG=0).
	if got := NDCGAtK([]string{"a", "b"}, rel(), 3); got != 0 {
		t.Errorf("sin relevantes ⇒ 0, obtuve %v", got)
	}
}

func TestMean(t *testing.T) {
	if got := mean(nil); got != 0 {
		t.Errorf("mean(nil) = %v, quería 0", got)
	}
	if got := mean([]float64{1, 2, 3}); math.Abs(got-2.0) > 1e-9 {
		t.Errorf("mean = %v, quería 2", got)
	}
}
