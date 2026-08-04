package embedding

import (
	"context"
	"errors"

	"musubi/internal/config"
	"musubi/internal/logx"
	"musubi/internal/redact"
)

// ErrSecretsBlocked se devuelve en modo `refuse` cuando el texto que iba a embeberse contenía un
// secreto. Distinguible a propósito de un fallo del backend: reintentar contra el mismo proveedor
// no sirve, porque es una decisión de política y no un error transitorio.
var ErrSecretsBlocked = errors.New("embedding bloqueado por el portero de privacidad: el texto contenía secretos y embedding.gateway.mode es 'refuse'")

// ErrGatewayFailed se devuelve si el portero no pudo hacer su trabajo. Existe para que un fallo del
// portero NUNCA se pueda confundir con "no había secretos": si algo salió mal tapando, no se manda
// nada (invariante E3).
var ErrGatewayFailed = errors.New("el portero de privacidad falló al procesar el texto; no se envió nada al proveedor de embeddings")

// guarded envuelve un Provider con red y garantiza que ningún secreto detectable cruce hacia él.
//
// A diferencia del portero de la cognición, acá NO hace falta el mapeo reversible de
// internal/privacy: un embedder devuelve un []float32, no texto, así que no hay respuesta que
// rehidratar. Eso vuelve el problema más simple, no imposible — alcanza con la redacción de una
// sola vía que internal/redact ya hace, que además es determinista (invariante E1).
type guarded struct {
	inner Provider
	mode  string
	// scrub tapa el texto. nil ⇒ redact.Redact, el único camino en producción. Es un seam de test
	// (para poder inyectar un pánico y comprobar que el recover ataja de verdad), y es un campo y
	// no una variable de paquete para no meter estado global mutable entre tests.
	scrub func(string) (string, []redact.Finding)
}

// Dimensions y Name delegan: el portero es transporte, no modelo. La dimensión del vector y el
// nombre que se estampa como procedencia tienen que seguir siendo los del embedder real.
func (g guarded) Dimensions() int { return g.inner.Dimensions() }
func (g guarded) Name() string    { return g.inner.Name() }

// Embed tapa los secretos del texto y se lo pasa al proveedor.
//
// No hay paso de vuelta: lo que sale del proveedor es un vector, y un vector derivado del texto
// tapado es exactamente lo que se quiere guardar. Como el portero está en el CONSTRUCTOR, las rutas
// que indexan y las que consultan usan literalmente el mismo objeto — así el invariante E2
// (coherencia índice↔consulta) es una consecuencia estructural y no una regla que haya que recordar.
func (g guarded) Embed(ctx context.Context, text string) ([]float32, error) {
	clean, n, err := g.scrubText(text)
	if err != nil {
		return nil, err
	}
	if n > 0 && g.mode == config.GatewayModeRefuse {
		// Se loguea la CANTIDAD, jamás los valores. Un log que filtra el secreto que acabás de
		// tapar convierte la guarda en teatro.
		logx.Warn("portero de privacidad: embedding bloqueado, no se envía al proveedor",
			"secretos", n, "modo", g.mode, "proveedor", g.inner.Name())
		return nil, ErrSecretsBlocked
	}
	return g.inner.Embed(ctx, clean)
}

// scrubText tapa el texto, blindado contra pánico.
//
// El recover no es paranoia decorativa: esto corre dentro del daemon MCP y un pánico en la
// redacción tumbaría el proceso entero. Atrapado, el peor caso es un error que el caller sabe
// manejar — y falla CERRADO: si hubo pánico, no hay texto que mandar.
func (g guarded) scrubText(text string) (clean string, n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			logx.Error("portero de privacidad: pánico al tapar el texto; no se envía nada al proveedor de embeddings", "panic", r)
			clean, n = "", 0
			err = ErrGatewayFailed
		}
	}()
	fn := g.scrub
	if fn == nil {
		fn = redact.Redact
	}
	out, finds := fn(text)
	return out, len(finds), nil
}

// needsGateway indica si p manda texto por un socket, que es lo único que define si hay frontera
// que cuidar.
//
// La regla es "¿abre un socket?" y NO "¿el host es remoto?": base_url es config, y la config se
// cambia. Una regla que dependa de parsear una URL para decidir si hay riesgo va a estar mal el día
// que alguien mueva el endpoint, y va a estar mal en silencio. Por eso un ollama en localhost
// también se envuelve.
//
// NoopProvider y StaticProvider quedan afuera por lo que SON, no por cómo estén configurados: el
// primero no embebe, el segundo es una tabla en proceso. Así el camino sin red es bit-idéntico por
// construcción (invariante E4).
func needsGateway(p Provider) bool {
	switch p.(type) {
	case NoopProvider, *StaticProvider:
		return false
	default:
		return true
	}
}

// newGuarded envuelve p según el modo pedido. Devuelve error ante un modo desconocido: una config
// mal escrita tiene que apagar la semántica, no caer en silencio a "sin protección".
func newGuarded(p Provider, mode string) (Provider, error) {
	if !needsGateway(p) {
		return p, nil
	}
	effective, err := config.NormalizeGatewayMode(mode)
	if err != nil {
		return nil, err
	}
	if effective == config.GatewayModeOff {
		// Ruidoso a propósito (invariante E5): quedarse sin portero es una decisión, y tiene que
		// verse. `musubi doctor` además lo muestra en rojo, porque un aviso que sólo existe en el
		// log de arranque de un daemon no lo lee nadie.
		logx.Warn("portero de privacidad de embeddings DESACTIVADO: el texto sale SIN TAPAR hacia el proveedor",
			"modo", config.GatewayModeOff, "proveedor", p.Name())
		return p, nil
	}
	return guarded{inner: p, mode: effective}, nil
}

// GatewayStatus es el estado del portero para un diagnóstico legible.
type GatewayStatus struct {
	Status  string // ok | warning | error
	Message string
}

// InspectGateway describe en qué estado queda el portero con la config dada, SIN tocar la red.
//
// Reusa newBaseProvider y config.NormalizeGatewayMode para que el diagnóstico no pueda divergir de
// lo que el constructor realmente hace.
func InspectGateway(cfg config.EmbeddingConfig) GatewayStatus {
	base, err := newBaseProvider(cfg)
	if err != nil {
		return GatewayStatus{Status: "error", Message: "la semántica no arranca: " + err.Error()}
	}
	effective, modeErr := config.NormalizeGatewayMode(cfg.Gateway.Mode)

	// Sin socket no hay frontera. Un modo mal escrito no rompe nada hoy, pero va a apagar la
	// semántica el día que cambien a un embedder con red: se avisa ahora, no cuando duela.
	if !needsGateway(base) {
		if modeErr != nil {
			return GatewayStatus{Status: "warning",
				Message: "embedder sin red (" + base.Name() + "): no hay salida de texto que cuidar, pero " + modeErr.Error()}
		}
		return GatewayStatus{Status: "ok",
			Message: "embedder sin red (" + base.Name() + "): el texto no sale de la máquina"}
	}

	if modeErr != nil {
		return GatewayStatus{Status: "error", Message: modeErr.Error() + "; la semántica queda apagada"}
	}

	switch effective {
	case config.GatewayModeOff:
		return GatewayStatus{Status: "error",
			Message: "PORTERO DESACTIVADO (mode: off): el texto y las consultas salen SIN TAPAR hacia " +
				base.Name() + ". Quitá embedding.gateway.mode para volver al default protegido"}
	case config.GatewayModeRefuse:
		return GatewayStatus{Status: "ok",
			Message: "portero activo (refuse) sobre " + base.Name() + ": si el texto lleva un secreto, no se embebe"}
	default:
		return GatewayStatus{Status: "ok",
			Message: "portero activo (scrub) sobre " + base.Name() + ": los secretos se tapan antes de salir"}
	}
}
