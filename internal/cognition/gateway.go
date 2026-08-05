package cognition

import (
	"context"
	"errors"
	"fmt"

	"musubi/internal/config"
	"musubi/internal/logx"
	"musubi/internal/privacy"
)

// Modos del portero de privacidad. Son ALIAS de los de config, no copias: los mismos tres modos
// valen para la cognición y para los embeddings, y tienen que significar lo mismo en los dos lados.
const (
	GatewayModeScrub  = config.GatewayModeScrub  // default: tapar y reponer
	GatewayModeRefuse = config.GatewayModeRefuse // si hay secreto, no se manda nada
	GatewayModeOff    = config.GatewayModeOff    // sin portero (explícito y ruidoso)
)

// ErrSecretsBlocked se devuelve en modo `refuse` cuando el texto que iba a salir contenía un
// secreto. Es un error DISTINGUIBLE a propósito: el caller puede diferenciar "no te mando esto por
// política" de "el motor falló", y degradar a model-free en vez de reintentar contra el mismo muro.
var ErrSecretsBlocked = errors.New("cognición bloqueada por el portero de privacidad: el texto contenía secretos y cognition.gateway.mode es 'refuse'")

// guarded envuelve un Provider real y garantiza que ningún secreto detectable cruce hacia él.
//
// Se aplica por DECORACIÓN, no por convención: al envolver dentro de NewProvider —el único
// constructor del pilar— todo motor nace ya protegido, incluidos los que se agreguen después. Un
// caller nuevo no puede olvidarse de pasar por el portero, porque no existe una versión sin él.
type guarded struct {
	inner Provider
	mode  string
	// newSession crea la sesión de privacidad de una llamada. nil ⇒ privacy.NewSession, que es el
	// único camino en producción.
	//
	// Existe como campo, y no como llamada directa, por una sola razón: que el test pueda inyectar
	// una sesión que entra en pánico y comprobar que el recover REALMENTE ataja. Una red de
	// seguridad que nunca se vio atajar algo no se sabe si aguanta. Es un campo y no una variable
	// de paquete para no introducir estado global mutable entre tests.
	newSession func() scrubSession
	// stats son los contadores de F5. Es un PUNTERO porque `guarded` es un tipo por valor: sin
	// él, cada copia de la struct contaría por su cuenta y los números no sumarían nada.
	// nil ⇒ no se cuenta (los tests que construyen un guarded a mano no necesitan telemetría).
	stats *gatewayStats
}

// scrubSession es lo que el portero necesita de una sesión de privacidad. En producción siempre es
// *privacy.Session; la interfaz existe para poder sustituirla en el test del recover.
type scrubSession interface {
	Scrub(text string) string
	Restore(text string) string
	Count() int
	Types() []string
}

// session devuelve la sesión de esta llamada, usando el camino real salvo que un test inyecte otro.
func (g guarded) session() scrubSession {
	if g.newSession != nil {
		return g.newSession()
	}
	return privacy.NewSession()
}

// Name delega en el motor envuelto: la procedencia que se estampa en la memoria tiene que seguir
// nombrando al modelo real, no al portero. El portero es transporte, no autor.
func (g guarded) Name() string { return g.inner.Name() }

// reportStats suma lo del portero y sigue hacia adentro (F5).
func (g guarded) reportStats(st *CognitionStats) {
	g.stats.snapshot(st)
	if r, ok := g.inner.(statsReporter); ok {
		r.reportStats(st)
	}
}

// Ask tapa los secretos de system y user, llama al motor, y repone los secretos en la respuesta.
//
// Una sesión POR LLAMADA: system y user comparten el mapeo (así un mismo secreto que aparece en los
// dos recibe el mismo marcador y el modelo puede razonar sobre "el mismo valor"), y el mapeo se
// descarta al terminar. No hay estado compartido entre llamadas ni entre goroutines.
func (g guarded) Ask(ctx context.Context, system, user string) (string, error) {
	sess, scrubbedSystem, scrubbedUser, err := g.scrubPrompt(system, user)
	if err != nil {
		g.stats.record(nil, false, true)
		return "", err
	}

	// Se anota UNA VEZ por llamada, con los TIPOS y nunca los valores (D5). Va acá y no al final
	// para que también cuente la llamada que el motor rechaza: lo que se mide es el trabajo del
	// portero, no el éxito del motor.
	g.stats.record(sess.Types(), sess.Count() > 0 && g.mode == GatewayModeRefuse, false)

	if n := sess.Count(); n > 0 && g.mode == GatewayModeRefuse {
		// Se loguean los TIPOS, jamás los valores. Un log que filtra el secreto que acabás de tapar
		// convierte la guarda en teatro. (logx escribe a stderr: no contamina el stdout del MCP.)
		logx.Warn("portero de privacidad: prompt bloqueado, no se envía al motor",
			"secretos", n, "tipos", sess.Types(), "modo", g.mode)
		return "", ErrSecretsBlocked
	}

	answer, err := g.inner.Ask(ctx, scrubbedSystem, scrubbedUser)
	if err != nil {
		// No se repone nada sobre un error: el mensaje de error del motor puede terminar en un log
		// y no tiene por qué llevar secretos rehidratados adentro.
		return "", err
	}
	return sess.Restore(answer), nil
}

// ErrGatewayFailed se devuelve si el portero no pudo hacer su trabajo. Existe para que un fallo
// del portero NUNCA se pueda confundir con "no había secretos": si algo salió mal tapando, no se
// manda nada (invariante R4).
var ErrGatewayFailed = errors.New("el portero de privacidad falló al procesar el prompt; no se envió nada al motor")

// scrubPrompt tapa system y user con una sola sesión compartida, blindado contra pánico.
//
// El recover no es paranoia decorativa: este código corre dentro del daemon MCP, y un pánico en la
// redacción (un offset raro, una entrada patológica) tumbaría todo el proceso. Con el recover, el
// peor caso es que la cognición devuelva un error y el caller degrade a model-free — que es
// exactamente la degradación con gracia que promete el plan. Y falla CERRADO: si hubo pánico, no
// hay prompt que mandar.
func (g guarded) scrubPrompt(system, user string) (sess scrubSession, outSystem, outUser string, err error) {
	defer func() {
		if r := recover(); r != nil {
			logx.Error("portero de privacidad: pánico al tapar el prompt; no se envía nada al motor", "panic", r)
			sess, outSystem, outUser = nil, "", ""
			err = ErrGatewayFailed
		}
	}()
	sess = g.session()
	outSystem = sess.Scrub(system)
	outUser = sess.Scrub(user)
	return sess, outSystem, outUser, nil
}

// newGuarded envuelve p según el modo pedido. Devuelve error ante un modo desconocido: una config
// mal escrita tiene que romper el arranque, no caer en silencio a "sin protección".
//
// El NoopProvider NO se envuelve: sin motor no hay frontera que cuidar, y así el camino model-free
// queda bit-idéntico por construcción y no por cuidado (invariante R6).
func newGuarded(p Provider, mode string) (Provider, error) {
	if !Enabled(p) {
		return p, nil
	}
	effective, err := normalizeGatewayMode(mode)
	if err != nil {
		return nil, err
	}
	if effective == GatewayModeOff {
		// Ruidoso a propósito (invariante R7): quedarse sin portero es una decisión, y tiene que
		// verse en el log de arranque de quien la tomó. `musubi doctor` además lo muestra en rojo,
		// porque un aviso que sólo existe en el log de un daemon no lo lee nadie.
		logx.Warn("portero de privacidad DESACTIVADO: el texto de la memoria sale SIN TAPAR hacia el motor",
			"modo", GatewayModeOff, "motor", p.Name())
		return p, nil
	}
	return guarded{inner: p, mode: effective, stats: newGatewayStats()}, nil
}

// normalizeGatewayMode valida el modo y devuelve el efectivo ("" ⇒ scrub).
//
// Delega en config.NormalizeGatewayMode, que es la ÚNICA fuente de verdad sobre qué modos existen:
// la comparten los DOS pilares con portero (cognición y embeddings), así que agregar un modo no
// puede dejar a uno desactualizado ni permitir que signifiquen cosas distintas en cada lado
// (invariante E6). Acá sólo se le pone el prefijo de la clave de config al mensaje de error.
func normalizeGatewayMode(mode string) (string, error) {
	effective, err := config.NormalizeGatewayMode(mode)
	if err != nil {
		return "", fmt.Errorf("cognition.%w", err)
	}
	return effective, nil
}

// GatewayStatus es el estado del portero para un diagnóstico legible.
type GatewayStatus struct {
	Status  string // ok | warning | error
	Message string
}

// InspectGateway describe en qué estado queda el portero con la config dada, SIN hacer ninguna
// llamada de red.
//
// Existe para que `musubi doctor` pueda mostrarlo. Una guarda de seguridad apagada que sólo se ve
// en el log de arranque de un daemon es una guarda que nadie va a notar: el aviso tiene que estar
// donde alguien mira.
//
// Reusa newBaseProvider y normalizeGatewayMode a propósito, para que el diagnóstico no pueda
// divergir de lo que el constructor realmente hace.
func InspectGateway(cfg config.CognitionConfig) GatewayStatus {
	if len(cfg.Fleet) > 0 {
		return inspectFleet(cfg)
	}
	base, err := newBaseProvider(cfg)
	if err != nil {
		return GatewayStatus{Status: "error", Message: fmt.Sprintf("el pilar de cognición no arranca: %v", err)}
	}
	effective, modeErr := normalizeGatewayMode(cfg.Gateway.Mode)

	// Pilar apagado: no hay frontera que cuidar. Un modo mal escrito no rompe nada hoy, pero va a
	// apagar el pilar el día que lo enciendan — se avisa ahora, no cuando duela.
	if !Enabled(base) {
		if modeErr != nil {
			return GatewayStatus{Status: "warning", Message: fmt.Sprintf(
				"pilar de cognición apagado (model-free): no hay salida de texto que cuidar, pero %v", modeErr)}
		}
		return GatewayStatus{Status: "ok",
			Message: "pilar de cognición apagado (model-free): no hay salida de texto que cuidar"}
	}

	if modeErr != nil {
		return GatewayStatus{Status: "error", Message: fmt.Sprintf("%v; el pilar entero queda apagado", modeErr)}
	}

	switch effective {
	case GatewayModeOff:
		return GatewayStatus{Status: "error", Message: fmt.Sprintf(
			"PORTERO DESACTIVADO (mode: off): el texto de la memoria sale SIN TAPAR hacia %s. "+
				"Quitá cognition.gateway.mode para volver al default protegido", base.Name())}
	case GatewayModeRefuse:
		return GatewayStatus{Status: "ok", Message: fmt.Sprintf(
			"portero activo (refuse) sobre %s: si el texto lleva un secreto, no se manda nada", base.Name())}
	default:
		return GatewayStatus{Status: "ok", Message: fmt.Sprintf(
			"portero activo (scrub) sobre %s: los secretos salen tapados y se reponen al volver", base.Name())}
	}
}
