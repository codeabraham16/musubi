package cognition

import (
	"fmt"
	"os"
	"time"

	"musubi/internal/config"
)

// NewProvider construye el Provider de cognición según la config. Por defecto (provider vacío o
// "none") devuelve NoopProvider: el pilar nace APAGADO y Musubi es bit-idéntico a un cerebro
// model-free. "openai-compat" (alias "litellm") arma el motor real contra un endpoint de chat
// OpenAI-compatible (F3.5b). Cualquier provider desconocido es un error explícito (fail-closed),
// no un Noop silencioso, para no ocultar una config equivocada.
func NewProvider(cfg config.CognitionConfig) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return NoopProvider{}, nil
	case "openai-compat", "litellm":
		// La master key del proxy se lee de la env var NOMBRADA en auth_token_env; nunca del
		// yaml, para no versionar secretos. Puede quedar vacía para backends locales sin auth.
		apiKey := ""
		if cfg.AuthTokenEnv != "" {
			apiKey = os.Getenv(cfg.AuthTokenEnv)
		}
		timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
		return NewOpenAICompatProvider(cfg.Endpoint, cfg.Model, apiKey, timeout), nil
	default:
		return nil, fmt.Errorf("motor de cognición desconocido: %q (usá 'none' o 'openai-compat')", cfg.Provider)
	}
}
