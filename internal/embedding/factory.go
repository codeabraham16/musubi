package embedding

import (
	"fmt"

	"musubi/internal/config"
)

// modoDeConstruccion dice para qué se arma el embebedor. Cambia UNA cosa: con "static", si se
// carga la tabla entera (modoCompleto) o se arma la consulta liviana (modoConsulta). Los proveedores
// por red se construyen igual en los dos modos, porque construirlos no lee nada.
type modoDeConstruccion int

const (
	modoCompleto modoDeConstruccion = iota
	modoConsulta
)

// NewProvider construye el Provider adecuado según la configuración.
// Por defecto (provider vacío o "none") devuelve NoopProvider.
func NewProvider(cfg config.EmbeddingConfig) (Provider, error) {
	return nuevoProvider(cfg, modoCompleto)
}

// NewProviderDeConsulta construye el embebedor para quien sólo va a embeber CONSULTAS y no puede
// pagar la tabla: un proceso efímero, como el hook por turno. Con "static" devuelve la consulta
// liviana (consulta_liviana.go), que da el mismo vector que NewProvider sin cargar la tabla; si no
// hay índice del tokenizer al lado de la tabla, devuelve ErrSinAtajo o ErrIdentidadVencida y NO cae
// a cargar la tabla entera —eso es justo lo que el caller no puede pagar—.
//
// Pasa por el MISMO envoltorio que NewProvider: el vector de la consulta tiene que salir de la
// misma transformación que el del índice (invariante E2), y eso no se garantiza con un constructor
// aparte que «hoy da lo mismo».
func NewProviderDeConsulta(cfg config.EmbeddingConfig) (Provider, error) {
	return nuevoProvider(cfg, modoConsulta)
}

func nuevoProvider(cfg config.EmbeddingConfig, modo modoDeConstruccion) (Provider, error) {
	base, err := newBaseProvider(cfg, modo)
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
func newBaseProvider(cfg config.EmbeddingConfig, modo modoDeConstruccion) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return NoopProvider{}, nil
	case "static":
		// Tabla estática (model2vec/POTION): embeddings model-free at inference, sin
		// red ni cgo. La tabla la aporta el usuario en static_path (bring-your-own-table).
		if modo == modoConsulta {
			c, err := NewConsultaLiviana(cfg.StaticPath)
			if err != nil {
				return nil, err
			}
			return c, nil
		}
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
