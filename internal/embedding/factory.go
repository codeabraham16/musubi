package embedding

import (
	"fmt"
	"os"

	"musubi/internal/config"
)

// NewProvider construye el Provider adecuado según la configuración.
// Por defecto (provider vacío o "none") devuelve NoopProvider.
func NewProvider(cfg config.EmbeddingConfig) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return NoopProvider{}, nil
	case "static":
		// Tabla estática (model2vec/POTION): embeddings model-free at inference, sin
		// red ni cgo. La tabla la aporta el usuario en static_path (bring-your-own-table).
		return NewStaticProvider(cfg.StaticPath)
	case "ollama":
		return NewOllamaProvider(cfg.BaseURL, cfg.Model, cfg.Dimensions), nil
	case "openai", "openai-compatible":
		// La API key se lee de la env var nombrada en config (default OPENAI_API_KEY);
		// nunca del yaml, para no versionar secretos. Puede quedar vacía para
		// servidores locales compatibles que no exigen autenticación.
		envName := cfg.APIKeyEnv
		if envName == "" {
			envName = "OPENAI_API_KEY"
		}
		apiKey := os.Getenv(envName)
		// Si base_url sigue siendo el default de Ollama, el usuario solo cambió el
		// provider: lo tratamos como "sin definir" para caer al endpoint de OpenAI.
		baseURL := cfg.BaseURL
		if baseURL == defaultOllamaBaseURL {
			baseURL = ""
		}
		return NewOpenAIProvider(baseURL, cfg.Model, apiKey, cfg.Dimensions), nil
	default:
		return nil, fmt.Errorf("proveedor de embeddings desconocido: %q (usá 'none', 'static', 'ollama' u 'openai')", cfg.Provider)
	}
}
