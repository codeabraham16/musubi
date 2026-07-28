package recalleval

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

const baselinePath = "testdata/baseline_modelfree.json"

// TestModelFreeBaselineNoRegression congela el recall MODEL-FREE (100% léxico, sin vector) sobre el
// fixture dorado y falla si MRR/Recall@k/nDCG@k caen por debajo de la baseline commiteada. Es la red
// de seguridad del pilar Cognición (F0): garantiza que, cuando F1+ agregue cognición, el camino
// model-free por default NUNCA se degrade sin que un test lo grite. El recall léxico es determinista,
// así que la comparación es exacta (con un epsilon para el ruido de float). Para regenerar la
// baseline (tras un cambio intencional del ranking): correr con MUSUBI_UPDATE_BASELINE=1 y revisar el
// diff del JSON.
func TestModelFreeBaselineNoRegression(t *testing.T) {
	fx := loadGolden(t)
	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig}, ks)
	if err != nil {
		t.Fatalf("Run léxico: %v", err)
	}
	got := scores[0]

	if os.Getenv("MUSUBI_UPDATE_BASELINE") == "1" {
		writeBaseline(t, got)
		t.Skipf("baseline model-free regenerada en %s (MUSUBI_UPDATE_BASELINE=1)", baselinePath)
	}

	want := readBaseline(t)
	const eps = 1e-9
	if got.MRR < want.MRR-eps {
		t.Errorf("REGRESIÓN model-free: MRR = %.6f < baseline %.6f", got.MRR, want.MRR)
	}
	for _, k := range ks {
		if got.RecallAtK[k] < want.RecallAtK[k]-eps {
			t.Errorf("REGRESIÓN model-free: Recall@%d = %.6f < baseline %.6f", k, got.RecallAtK[k], want.RecallAtK[k])
		}
		if got.NDCGAtK[k] < want.NDCGAtK[k]-eps {
			t.Errorf("REGRESIÓN model-free: nDCG@%d = %.6f < baseline %.6f", k, got.NDCGAtK[k], want.NDCGAtK[k])
		}
	}
}

func writeBaseline(t *testing.T, s Scores) {
	t.Helper()
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselinePath, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("escribir baseline: %v", err)
	}
}

func readBaseline(t *testing.T) Scores {
	t.Helper()
	b, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("leer baseline (%s): %v — generála con MUSUBI_UPDATE_BASELINE=1", baselinePath, err)
	}
	var s Scores
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parsear baseline: %v", err)
	}
	return s
}
