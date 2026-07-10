package recalleval

import (
	"context"
	"os"
	"testing"

	"musubi/internal/embedding"
)

// TestSemanticVsLexicalReal mide el recall LÉXICO vs SEMÁNTICO sobre el fixture dorado con
// la tabla REAL de POTION (StaticProvider). Es el gate de la Fase 2: probar con números que
// encender la señal semántica mejora sobre el baseline antes de cambiar el default. Requiere
// la tabla local (bajada con `musubi embed pull`), apuntada por MUSUBI_POTION_DIR; sin ella,
// se saltea (CI no baja los ~488MB).
func TestSemanticVsLexicalReal(t *testing.T) {
	dir := os.Getenv("MUSUBI_POTION_DIR")
	if dir == "" {
		t.Skip("MUSUBI_POTION_DIR no seteado: se saltea la medición semántica con la tabla real")
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
	scores, err := Run(context.Background(), t.TempDir(), fx, embed, []Config{lexicalConfig, hybridConfig}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("léxico vs semántico (POTION multilingüe REAL):\n%s", FormatReport(scores, ks))

	// GATE de calidad R@10 (T17.3c): antes esto sólo logueaba. Ahora asserta dos cosas para que la
	// señal semántica sea un contrato defendido en CI, no una medición de una sola vez:
	//   (a) piso ABSOLUTO de R@10 del híbrido — atrapa una regresión real (bug en el tokenizer
	//       Unigram, en el ranking híbrido o en la tabla) que degrade el recall.
	//   (b) el híbrido NO queda por debajo del léxico — el win semántico debe ser ADITIVO.
	// Medido con la tabla POTION multilingüe pinneada (SHA-256) sobre el fixture dorado versionado:
	// léxico R@10=0.750, híbrido R@10=0.833 (determinista: static embeddings + ranking fijo). El
	// piso 0.80 deja ~0.03 de margen; si sube el híbrido, subir el piso (ratchet).
	const hybridRecallAt10Floor = 0.80
	var lex, hyb Scores
	for _, s := range scores {
		switch s.Config {
		case lexicalConfig.Name:
			lex = s
		case hybridConfig.Name:
			hyb = s
		}
	}
	if hyb.RecallAtK[10] < hybridRecallAt10Floor {
		t.Errorf("GATE R@10: híbrido = %.3f < piso %.2f (regresión de la señal semántica)", hyb.RecallAtK[10], hybridRecallAt10Floor)
	}
	if hyb.RecallAtK[10] < lex.RecallAtK[10] {
		t.Errorf("GATE R@10: híbrido (%.3f) por debajo del léxico (%.3f): la semántica debe SUMAR, no restar", hyb.RecallAtK[10], lex.RecallAtK[10])
	}
}
