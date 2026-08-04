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
	// Con flota declarada manda el router (F2). Sin flota no se instancia nada nuevo: el camino de
	// motor único queda bit-idéntico al de F1 (invariante C6).
	//
	// El router construye cada motor con esta misma fábrica, incluido el portero, así que la
	// garantía de F1 —no existe un motor real sin envolver— sigue valiendo dentro de la flota.
	if len(cfg.Fleet) > 0 {
		return newRouter(cfg, nil)
	}
	base, err := newBaseProvider(cfg)
	if err != nil {
		return nil, err
	}
	// EL PORTERO SE PONE ACÁ, Y NO EN EL CALLER, A PROPÓSITO: este es el único constructor del
	// pilar, así que todo motor real nace envuelto — el de hoy y el que se agregue mañana. Un
	// caller nuevo no puede olvidarse de pasar por el portero porque no existe una versión sin él.
	// newGuarded devuelve el Provider tal cual si es el NoopProvider (nada que cuidar) o si el modo
	// es "off", y falla explícito ante un modo desconocido.
	return newGuarded(base, cfg.Gateway.Mode)
}

// newBaseProvider arma el motor desnudo, sin portero. Separado de NewProvider para que quede
// imposible construir uno sin pasar por el envoltorio.
func newBaseProvider(cfg config.CognitionConfig) (Provider, error) {
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
