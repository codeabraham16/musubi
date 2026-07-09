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
}
