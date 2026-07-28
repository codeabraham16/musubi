package cognition

import (
	"fmt"

	"musubi/internal/config"
)

// NewProvider construye el Provider de cognición según la config. Por defecto (provider vacío o
// "none") devuelve NoopProvider: el pilar nace APAGADO y Musubi es bit-idéntico a un cerebro
// model-free. En F0 no existe ningún motor real todavía; cualquier provider desconocido es un
// error explícito (fail-closed), no un Noop silencioso, para no ocultar una config equivocada.
func NewProvider(cfg config.CognitionConfig) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return NoopProvider{}, nil
	default:
		return nil, fmt.Errorf("motor de cognición desconocido: %q (F0 sólo soporta 'none'; los motores reales llegan en F1+)", cfg.Provider)
	}
}
