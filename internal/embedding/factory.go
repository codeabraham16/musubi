package embedding

import (
	"fmt"

	"musubi/internal/config"
)

// NewProvider construye el Provider adecuado según la configuración.
// Por defecto (provider vacío o "none") devuelve NoopProvider.
func NewProvider(cfg config.EmbeddingConfig) (Provider, error) {
	base, err := newBaseProvider(cfg)
	if err != nil {
		return nil, err
	}
	// EL PORTERO SE PONE ACÁ, Y NO EN EL CALLER, A PROPÓSITO: este es el único constructor del
	// embedder, así que todo proveedor con red nace envuelto — el de hoy y el que se agregue
	// mañana. Además hace que el texto que se INDEXA y el que se CONSULTA pasen por el mismo
	// objeto, que es lo que garantiza la coherencia índice↔consulta (invariante E2).
	// Y el troceador va DEBAJO del portero, no encima: el texto se tapa ENTERO y recién después se
	// parte. Al revés, un secreto que cayera justo sobre el corte quedaría partido en dos mitades
	// que ninguna regla reconoce, y saldría sin tapar.
	return newGuarded(newTroceado(base), cfg.Gateway.Mode)
}

// newBaseProvider arma el embedder desnudo, sin portero. Separado de NewProvider para que quede
// imposible construir uno sin pasar por el envoltorio.
func newBaseProvider(cfg config.EmbeddingConfig) (Provider, error) {
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
		//
		// LA VARIABLE O EL ARCHIVO `<VAR>_FILE`, IGUAL QUE EL RESTO (A89/A101).
		//
		// Acá había un `os.Getenv` pelado, y la regla del `_FILE` la tenía el hermano de al lado
		// —`cognition/factory.go`, que la cita— y no ésta. La config nombra credenciales con TRES
		// campos (`auth_token_env`, `api_key_env`, `marketplace_api_key_env`) y sólo el primero
		// pasaba por `SecretoDeEnv`: diez sitios de un lado, cero del otro. Una regla que se aplica
		// en un camino y no en su gemelo no es un bug puntual, es una regla sin dueño.
		//
		// Y acá era PEOR que el A89 original: con `OPENAI_API_KEY_FILE` puesto y la variable
		// vacía, el provider se construía SIN error y con la key en blanco. El fallo llegaba
		// después, lejos y disfrazado de «el backend rechaza», en vez de «tu config nombra un
		// archivo que no leí».
		envName := cfg.APIKeyEnv
		if envName == "" {
			envName = "OPENAI_API_KEY"
		}
		// Un error acá es una ruta NOMBRADA que no se pudo leer: eso es config rota, no un backend
		// sin auth. Se dice en voz alta en vez de degradar a «sin credencial» — mismo criterio,
		// palabra por palabra, que el de cognition.
		apiKey, err := config.SecretoDeEnv(envName)
		if err != nil {
			return nil, fmt.Errorf("embedding.api_key_env: %w", err)
		}
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
