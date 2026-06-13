package embedding

import (
	"fmt"

	"musubi/internal/config"
)

// NewProvider construye el Provider adecuado según la configuración.
// Por defecto (provider vacío o "none") devuelve NoopProvider.
func NewProvider(cfg config.EmbeddingConfig) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return NoopProvider{}, nil
	case "ollama":
		return NewOllamaProvider(cfg.BaseURL, cfg.Model, cfg.Dimensions), nil
	default:
		return nil, fmt.Errorf("proveedor de embeddings desconocido: %q (usá 'none' u 'ollama')", cfg.Provider)
	}
}
