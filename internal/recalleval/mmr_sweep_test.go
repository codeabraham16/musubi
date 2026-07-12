package recalleval

import (
	"context"
	"fmt"
	"os"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestMMRLambdaSweep NO es un gate: es el INSTRUMENTO con el que se elige λ.
//
// La diversidad canja relevancia por cobertura. El default de λ no se estima: se MIDE contra el
// fixture dorado con la tabla POTION real, y el número queda escrito acá para que la próxima
// persona no tenga que confiar en mi palabra.
//
// Si NINGÚN λ que haga algo útil mantiene R@10 >= 0.80, la feature NO SE JUSTIFICA.
func TestMMRLambdaSweep(t *testing.T) {
	dir := os.Getenv("MUSUBI_POTION_DIR")
	if dir == "" {
		t.Skip("MUSUBI_POTION_DIR no seteado: se salta el barrido (necesita la tabla POTION real)")
	}
	prov, err := embedding.NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("NewStaticProvider: %v", err)
	}
	embed := func(text string) ([]float32, error) {
		return prov.Embed(context.Background(), text)
	}
	fx := loadGolden(t)
	ks := []int{1, 5, 10}

	var cfgs []Config
	lambdas := []float64{1.0, 0.75, 0.5, 0.2, 0.05}
	for _, l := range lambdas {
		cfgs = append(cfgs, Config{
			Name: fmt.Sprintf("mmr=%.2f", l),
			Opts: memory.RecallOptions{
				Stemming: true, Cooccurrence: true, GraphCentrality: true, MMRLambda: l,
			},
			UseVector: true,
		})
	}

	scores, err := Run(context.Background(), t.TempDir(), fx, embed, cfgs, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("BARRIDO DE λ (λ=1.00 es MMR APAGADO = la línea de base):\n%s", FormatReport(scores, ks))
	for _, s := range scores {
		t.Logf("  %-10s  R@1=%.3f  R@5=%.3f  R@10=%.3f", s.Config, s.RecallAtK[1], s.RecallAtK[5], s.RecallAtK[10])
	}
}
