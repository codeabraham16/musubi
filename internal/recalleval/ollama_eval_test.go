package recalleval

import (
	"context"
	"os"
	"strconv"
	"testing"

	"musubi/internal/embedding"
)

// TestSemanticVsOllamaReal mide el recall LÉXICO vs SEMÁNTICO usando un embedder OLLAMA local
// (bge-m3 u otro) sobre el MISMO fixture dorado que TestSemanticVsLexicalReal, para el A/B HONESTO
// contra la tabla estática POTION (Track A: subir la potencia del recall con un forward pass real
// que captura orden de palabras, no sólo vocabulario). Baseline a batir: híbrido POTION R@10=0.833.
//
// Requiere una instancia Ollama accesible con el modelo bajado, apuntada por MUSUBI_OLLAMA_URL
// (ej. http://100.79.126.62:11434 sobre el tailnet, con OLLAMA_HOST=0.0.0.0 en el server). Sin la
// var se saltea: CI no corre Ollama. Es una MEDICIÓN (imprime los números para decidir), no un gate:
// el cambio de default sólo se hace si estos números superan a POTION.
func TestSemanticVsOllamaReal(t *testing.T) {
	url := os.Getenv("MUSUBI_OLLAMA_URL")
	if url == "" {
		t.Skip("MUSUBI_OLLAMA_URL no seteado: se saltea la medición semántica con Ollama")
	}
	model := os.Getenv("MUSUBI_OLLAMA_MODEL")
	if model == "" {
		model = "bge-m3"
	}
	dim := 1024 // bge-m3
	if d := os.Getenv("MUSUBI_OLLAMA_DIM"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			dim = n
		}
	}

	prov := embedding.NewOllamaProvider(url, model, dim)
	embed := func(text string) ([]float32, error) {
		return prov.Embed(context.Background(), text)
	}

	fx := loadGolden(t)
	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, embed, []Config{lexicalConfig, hybridConfig}, ks)
	if err != nil {
		t.Fatalf("Run (¿Ollama accesible en %s con el modelo %q?): %v", url, model, err)
	}
	t.Logf("léxico vs semántico (Ollama %s, dim %d):\n%s", model, dim, FormatReport(scores, ks))

	// A/B contra el baseline POTION multilingüe (híbrido R@10=0.833, medido 2026-07-28). Se imprime
	// el delta para decidir el cambio de default; NO asevera (es medición). Para volverlo gate más
	// adelante: asertar hyb.RecallAtK[10] > potionHybridRecallAt10.
	const potionHybridRecallAt10 = 0.833
	var lex, hyb Scores
	for _, s := range scores {
		switch s.Config {
		case lexicalConfig.Name:
			lex = s
		case hybridConfig.Name:
			hyb = s
		}
	}
	t.Logf("A/B: híbrido Ollama R@10=%.3f vs POTION 0.833 (Δ=%+.3f) · léxico=%.3f",
		hyb.RecallAtK[10], hyb.RecallAtK[10]-potionHybridRecallAt10, lex.RecallAtK[10])
	if hyb.RecallAtK[10] < lex.RecallAtK[10] {
		t.Logf("AVISO: el híbrido Ollama (%.3f) quedó por debajo del léxico (%.3f) — la señal no está sumando (¿dim/modelo mal?)",
			hyb.RecallAtK[10], lex.RecallAtK[10])
	}
}
