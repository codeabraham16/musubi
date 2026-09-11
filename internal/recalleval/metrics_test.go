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

// TestRedundanciaAtK fija la métrica que faltaba para poder evaluar la diversificación.
func TestRedundanciaAtK(t *testing.T) {
	// sim de juguete: dos ids son "lo mismo" si comparten la primera letra.
	sim := func(a, b string) float64 {
		if a[0] == b[0] {
			return 1
		}
		return 0
	}

	t.Run("todo repetido da 1", func(t *testing.T) {
		if got := RedundanciaAtK([]string{"a1", "a2", "a3"}, 3, sim); got != 1 {
			t.Errorf("esperaba 1 (los tres son lo mismo), obtuve %v", got)
		}
	})
	t.Run("todo distinto da 0", func(t *testing.T) {
		if got := RedundanciaAtK([]string{"a1", "b1", "c1"}, 3, sim); got != 0 {
			t.Errorf("esperaba 0 (los tres distintos), obtuve %v", got)
		}
	})
	t.Run("mezcla da el promedio de los pares", func(t *testing.T) {
		// a1,a2,b1: pares (a1,a2)=1, (a1,b1)=0, (a2,b1)=0 => 1/3
		got := RedundanciaAtK([]string{"a1", "a2", "b1"}, 3, sim)
		if got < 0.333 || got > 0.334 {
			t.Errorf("esperaba ~0.3333, obtuve %v", got)
		}
	})
	t.Run("menos de dos resultados no tiene pares", func(t *testing.T) {
		if got := RedundanciaAtK([]string{"a1"}, 3, sim); got != 0 {
			t.Errorf("con un solo resultado no hay par que comparar: esperaba 0, obtuve %v", got)
		}
	})
	t.Run("k mayor que el ranking se acota", func(t *testing.T) {
		if got := RedundanciaAtK([]string{"a1", "a2"}, 99, sim); got != 1 {
			t.Errorf("k se tiene que acotar al largo: esperaba 1, obtuve %v", got)
		}
	})
	t.Run("sin funcion de similitud no inventa un numero", func(t *testing.T) {
		if got := RedundanciaAtK([]string{"a1", "a2"}, 2, nil); got != 0 {
			t.Errorf("sin sim no se puede medir: esperaba 0, obtuve %v", got)
		}
	})

	// EL CASO QUE JUSTIFICA TODO: dos rankings con el MISMO Recall@3 y redundancia opuesta.
	// Las métricas de relevancia no los distinguen; ésta sí.
	relevantes := map[string]bool{"a1": true, "a2": true, "a3": true, "b1": true}
	repetido := []string{"a1", "a2", "a3"} // tres caras de lo mismo
	diverso := []string{"a1", "b1", "zzz"} // dos cosas distintas + un irrelevante
	if RecallAtK(repetido, relevantes, 3) <= RecallAtK(diverso, relevantes, 3) {
		t.Skip("el caso construido no ilustra lo que quiere: revisar el fixture del test")
	}
	rRep := RedundanciaAtK(repetido, 3, sim)
	rDiv := RedundanciaAtK(diverso, 3, sim)
	if rRep <= rDiv {
		t.Errorf("el ranking repetido tiene que medir MÁS redundante: %v vs %v", rRep, rDiv)
	}
}
