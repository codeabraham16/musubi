package recalleval

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"testing"
	"unicode"

	"musubi/internal/memory"
)

// lexicalConfig es el baseline: recall léxico con las señales model-free que corren en
// producción (stemming + co-ocurrencia + centralidad de grafo), SIN vector.
var lexicalConfig = Config{
	Name: "lexical",
	Opts: memory.RecallOptions{Stemming: true, Cooccurrence: true, GraphCentrality: true},
}

// hybridConfig repite el baseline pero enciende la señal vectorial (QueryVector).
var hybridConfig = Config{
	Name:      "hybrid",
	Opts:      memory.RecallOptions{Stemming: true, Cooccurrence: true, GraphCentrality: true},
	UseVector: true,
}

// hashEmbed es un embedder SINTÉTICO determinista (bag-of-tokens hasheado a dim fija +
// L2-normalize). NO tiene semántica real (no cierra huecos de traducción); su único fin es
// ejercitar el camino vectorial del harness (siembra con vector, pool por coseno) para que
// F2.3 no descubra bugs de integración recién con la tabla POTION real.
func hashEmbed(text string) ([]float32, error) {
	const dim = 64
	v := make([]float32, dim)
	for _, tok := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(tok))
		v[h.Sum32()%dim] += 1
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	if norm > 0 {
		inv := 1.0 / math.Sqrt(norm)
		for i := range v {
			v[i] = float32(float64(v[i]) * inv)
		}
	}
	return v, nil
}

func loadGolden(t *testing.T) *Fixture {
	t.Helper()
	fx, err := LoadFixture("testdata/golden.json")
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	return fx
}

// El baseline léxico corre end-to-end sobre el fixture dorado y produce métricas sanas.
func TestHarnessLexicalBaseline(t *testing.T) {
	fx := loadGolden(t)
	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("esperaba 1 Scores, obtuve %d", len(scores))
	}
	s := scores[0]
	if s.Queries != len(fx.Queries) {
		t.Errorf("evaluó %d queries, el fixture tiene %d con relevantes", s.Queries, len(fx.Queries))
	}
	if s.MRR <= 0 {
		t.Errorf("MRR léxico debería ser > 0 (algo se encuentra), fue %v", s.MRR)
	}
	// recall@k es monótono no-decreciente en k.
	if !(s.RecallAtK[1] <= s.RecallAtK[5] && s.RecallAtK[5] <= s.RecallAtK[10]) {
		t.Errorf("recall@k no monótono: @1=%v @5=%v @10=%v", s.RecallAtK[1], s.RecallAtK[5], s.RecallAtK[10])
	}
	if s.RecallAtK[10] <= 0 {
		t.Errorf("recall@10 léxico debería encontrar algo, fue %v", s.RecallAtK[10])
	}
	// Toda métrica en [0,1].
	for _, k := range ks {
		if s.RecallAtK[k] < 0 || s.RecallAtK[k] > 1 || s.NDCGAtK[k] < 0 || s.NDCGAtK[k] > 1 {
			t.Errorf("métrica fuera de [0,1] en k=%d: R=%v nDCG=%v", k, s.RecallAtK[k], s.NDCGAtK[k])
		}
	}
	report := FormatReport(scores, ks)
	if !strings.Contains(report, "config") || !strings.Contains(report, "lexical") {
		t.Errorf("reporte inesperado:\n%s", report)
	}
	// Visible con `go test -v`: el baseline léxico documenta el hueco que la tabla cerrará.
	t.Logf("baseline de recall (léxico, model-free):\n%s", report)
}

// El camino híbrido (con vector) corre sin errores y produce métricas válidas. Ejercita la
// siembra con embedding, el pool por coseno y la 4ª señal RRF, con un embedder sintético.
func TestHarnessHybridPathRuns(t *testing.T) {
	fx := loadGolden(t)
	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, hashEmbed, []Config{lexicalConfig, hybridConfig}, ks)
	if err != nil {
		t.Fatalf("Run híbrido: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("esperaba 2 Scores (lexical, hybrid), obtuve %d", len(scores))
	}
	for _, s := range scores {
		if s.Queries != len(fx.Queries) {
			t.Errorf("%s: evaluó %d queries, esperaba %d", s.Config, s.Queries, len(fx.Queries))
		}
		for _, k := range ks {
			if s.RecallAtK[k] < 0 || s.RecallAtK[k] > 1 {
				t.Errorf("%s: recall@%d fuera de [0,1]: %v", s.Config, k, s.RecallAtK[k])
			}
		}
	}
	t.Logf("léxico vs híbrido (embedder sintético, sin semántica real):\n%s", FormatReport(scores, ks))
}

// LoadFixture rechaza un fixture vacío o inexistente.
func TestLoadFixtureErrors(t *testing.T) {
	if _, err := LoadFixture("testdata/no-existe.json"); err == nil {
		t.Error("un fixture inexistente debería fallar")
	}
}
