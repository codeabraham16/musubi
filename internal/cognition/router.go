package cognition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/logx"
)

// ErrAllEnginesDown se devuelve cuando ningún motor de la flota pudo atender la llamada. El caller
// lo trata igual que a ErrCognitionDisabled: degrada a model-free. La cognición es un acelerador,
// nunca el camino crítico.
var ErrAllEnginesDown = errors.New("ningún motor de cognición disponible: todos fallaron, están en cooldown o se negaron")

// engine es un motor de la flota con su tier y su breaker.
type engine struct {
	name    string
	tier    string
	inner   Provider // ya viene envuelto por su propio portero
	breaker *breaker
}

// router prueba los motores de la flota en orden. Es él mismo un Provider, así que nada río arriba
// se entera de que abajo hay más de un motor.
type router struct {
	engines []*engine
	// stats cuenta escaladas y agotamientos (F5). Nunca guarda prompts ni respuestas.
	stats *routerStats
}

// reportStats suma lo del router, el estado de los circuitos, y sigue hacia adentro de cada motor
// para juntar lo que cuenta el portero de cada uno (F5).
func (r *router) reportStats(st *CognitionStats) {
	r.stats.snapshot(st)
	for _, e := range r.engines {
		if e.breaker.open() {
			st.OpenCircuits = append(st.OpenCircuits, e.name)
		}
		if sr, ok := e.inner.(statsReporter); ok {
			sr.reportStats(st)
		}
	}
}

// Name nombra a la flota entera. La procedencia REAL de una respuesta la estampa el motor que la
// produjo; acá sólo se identifica el arreglo para el diagnóstico.
func (r *router) Name() string {
	parts := make([]string, 0, len(r.engines))
	for _, e := range r.engines {
		parts = append(parts, e.name)
	}
	return "fleet:" + strings.Join(parts, ",")
}

// Ask prueba los motores en orden y devuelve la primera respuesta buena.
//
// La distinción que hace toda la fase: un motor que se NIEGA (ErrSecretsBlocked) no está roto — está
// haciendo su trabajo. Eso no cuenta para el breaker y se escala al siguiente tier. Un motor que
// FALLA sí cuenta. Así la regla dura del roadmap ("un secreto no va a un tier gratis") no es un if
// que alguien pueda olvidar de escribir: es la consecuencia de que el motor barato esté en `refuse`.
func (r *router) Ask(ctx context.Context, system, user string) (string, error) {
	var seNego bool
	for _, e := range r.engines {
		if !e.breaker.allow() {
			continue
		}
		answer, err := e.inner.Ask(ctx, system, user)
		switch {
		case err == nil:
			e.breaker.success()
			return answer, nil

		case errors.Is(err, ErrSecretsBlocked):
			// Negativa por política. No es un síntoma de mala salud: si contara, tres prompts con
			// secretos seguidos apagarían un motor sano.
			e.breaker.abstain()
			seNego = true
			r.stats.escalated()
			logx.Info("router: el motor se negó por política, se escala al siguiente",
				"motor", e.name, "tier", e.tier)

		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			// Lo canceló el caller (o venció SU deadline): no dice nada sobre el motor, y seguir
			// probando la flota entera contra un contexto ya muerto sólo gasta tiempo.
			e.breaker.abstain()
			return "", err

		default:
			e.breaker.failure()
			r.stats.escalated()
			logx.Warn("router: motor caído, se prueba el siguiente",
				"motor", e.name, "error", err)
		}
	}
	r.stats.ranOut()

	// Agotada la flota. Se distingue "no te lo mando" de "no hay motor": el caller decide distinto
	// (con lo primero no tiene sentido reintentar el mismo texto).
	if seNego {
		return "", fmt.Errorf("%w: %w", ErrAllEnginesDown, ErrSecretsBlocked)
	}
	return "", ErrAllEnginesDown
}

// inspectFleet describe el estado de la flota para `musubi doctor`.
//
// Con flota, el diagnóstico tiene que hablar de la flota: si sólo mirara el motor único (que ni
// siquiera está configurado) diría "pilar apagado" mientras hay tres motores atendiendo.
func inspectFleet(cfg config.CognitionConfig) GatewayStatus {
	p, err := newRouter(cfg, nil)
	if err != nil {
		return GatewayStatus{Status: "error", Message: fmt.Sprintf("la flota de cognición no arranca: %v", err)}
	}
	r := p.(*router)

	var desprotegidos, tapandoAlGratis []string
	for _, e := range r.engines {
		g, envuelto := e.inner.(guarded)
		if !envuelto {
			desprotegidos = append(desprotegidos, e.name)
			continue
		}
		// Tapar antes de mandar a un tier gratis no filtra credenciales, así que no es un error.
		// Pero el RESTO del texto sí llega a un servicio que entrena con lo que recibe, y esa es
		// una decisión que conviene ver en vez de que quede enterrada en el yaml.
		if e.tier == config.TierFree && g.mode == GatewayModeScrub {
			tapandoAlGratis = append(tapandoAlGratis, e.name)
		}
	}

	resumen := fmt.Sprintf("flota de %d motor(es): %s", len(r.engines), r.Name()[len("fleet:"):])
	switch {
	case len(desprotegidos) > 0:
		return GatewayStatus{Status: "error", Message: fmt.Sprintf(
			"PORTERO DESACTIVADO en %s: el texto sale SIN TAPAR hacia esos motores de la flota (%s)",
			strings.Join(desprotegidos, ", "), resumen)}
	case len(tapandoAlGratis) > 0:
		return GatewayStatus{Status: "warning", Message: fmt.Sprintf(
			"%s tapa los secretos pero igual manda al tier gratis (mode: scrub sobre tier: free). "+
				"No hay fuga de credenciales, pero el resto del texto llega a un servicio que "+
				"entrena con lo que recibe. %s", strings.Join(tapandoAlGratis, ", "), resumen)}
	default:
		return GatewayStatus{Status: "ok", Message: "portero activo en toda la " + resumen}
	}
}

// newRouter arma la flota. Cada entrada se construye con LA MISMA fábrica que el motor único, así
// todo lo que F1 garantiza sobre un motor —que nace envuelto, que el modo se valida— vale igual acá.
func newRouter(cfg config.CognitionConfig, now func() time.Time) (Provider, error) {
	fails := cfg.Breaker.EffectiveFailures()
	cooldown := cfg.Breaker.EffectiveCooldown()

	engines := make([]*engine, 0, len(cfg.Fleet))
	for i, fe := range cfg.Fleet {
		tier, err := config.NormalizeTier(fe.Tier)
		if err != nil {
			return nil, fmt.Errorf("cognition.fleet[%d] (%s): %w", i, fe.Name, err)
		}

		// El modo del portero se DERIVA del tier salvo que la entrada lo declare. Acá vive la
		// regla dura: un motor en el que no se confía nace rechazando, no tapando.
		mode := fe.Gateway.Mode
		if mode == "" {
			mode = config.DefaultGatewayModeForTier(tier)
		}

		base, err := newBaseProvider(config.CognitionConfig{
			Provider:              fe.Provider,
			Model:                 fe.Model,
			Endpoint:              fe.Endpoint,
			AuthTokenEnv:          fe.AuthTokenEnv,
			RequestTimeoutSeconds: fe.RequestTimeoutSeconds,
		})
		if err != nil {
			return nil, fmt.Errorf("cognition.fleet[%d] (%s): %w", i, fe.Name, err)
		}
		if !Enabled(base) {
			return nil, fmt.Errorf("cognition.fleet[%d] (%s): un motor de la flota no puede ser 'none'", i, fe.Name)
		}
		guardedEngine, err := newGuarded(base, mode)
		if err != nil {
			return nil, fmt.Errorf("cognition.fleet[%d] (%s): %w", i, fe.Name, err)
		}

		name := fe.Name
		if name == "" {
			name = base.Name()
		}
		engines = append(engines, &engine{
			name:    name,
			tier:    tier,
			inner:   guardedEngine,
			breaker: newBreaker(fails, cooldown, now),
		})
	}

	if len(engines) == 0 {
		return nil, errors.New("cognition.fleet está declarada pero vacía")
	}
	return &router{engines: engines, stats: &routerStats{}}, nil
}
