package cognition

import "sync"

// TELEMETRÍA DE LA COGNICIÓN (F5).
//
// Tres fases dejaron preguntas sin instrumento: F1 anotó que sin medición no se sabe si el portero
// actúa seguido o nunca; F2 dejó las escaladas del router sin contar; F3 dejó `Stats()` contando
// hits sin que nadie los lea. Esto las cierra.
//
// D5 — ACÁ NUNCA HAY UN SECRETO. Se cuentan operaciones y se clasifican TIPOS ('aws-access-key'),
// jamás valores, ni prompts, ni respuestas. Es una superficie de lectura nueva sobre el subsistema
// que maneja secretos: que sólo cuente y clasifique no es un detalle de implementación, es el
// invariante que la hace segura de exponer.
//
// D6 — Contar no cambia el comportamiento. Los contadores son observadores, no participantes.

// CognitionStats es la foto de todos los contadores. Se arma recorriendo la cadena de decoradores.
type CognitionStats struct {
	// Caché (F3).
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`
	CacheSize   int   `json:"cache_size"`

	// Portero de privacidad (F1).
	GatewayCalls int64 `json:"gateway_calls"`
	// GatewayScrubbed son las llamadas en las que se tapó AL MENOS un secreto. No es la cantidad
	// de secretos: es cuántas veces el portero tuvo trabajo real que hacer.
	GatewayScrubbed int64 `json:"gateway_scrubbed"`
	// GatewayBlocked son las llamadas cortadas por política (modo refuse).
	GatewayBlocked int64 `json:"gateway_blocked"`
	// GatewayFailed son los pánicos atajados por el recover. Debería ser 0 siempre; si no lo es,
	// hay un bug en la redacción y conviene que se vea.
	GatewayFailed int64 `json:"gateway_failed"`
	// GatewayTypes son los TIPOS de secreto tapados y cuántas veces. Sin valores, nunca.
	GatewayTypes map[string]int64 `json:"gateway_types,omitempty"`

	// Router de flota (F2).
	RouterEscalations int64 `json:"router_escalations"`
	RouterExhausted   int64 `json:"router_exhausted"`
	// OpenCircuits son los motores con el circuito abierto AHORA.
	OpenCircuits []string `json:"open_circuits,omitempty"`
}

// statsReporter lo implementa cada capa que tiene algo que contar. Cada una suma lo suyo y delega
// hacia adentro, así la foto se arma recorriendo la cadena sin que nadie tenga que saber el orden
// en que quedaron apilados los decoradores.
type statsReporter interface {
	reportStats(*CognitionStats)
}

// Stats arma la foto recorriendo la cadena desde p hacia adentro. Es READ-ONLY (D8): no resetea
// contadores, no vacía el caché, no cierra circuitos. Un contador que se resetea al leerlo hace que
// dos lectores se roben los datos entre sí.
func Stats(p Provider) CognitionStats {
	var st CognitionStats
	if r, ok := p.(statsReporter); ok {
		r.reportStats(&st)
	}
	return st
}

// gatewayStats son los contadores del portero. Vive detrás de un PUNTERO en `guarded` porque
// `guarded` es un tipo por VALOR: sin el puntero, cada copia de la struct contaría por su cuenta y
// los números no sumarían nada.
type gatewayStats struct {
	mu       sync.Mutex
	calls    int64
	scrubbed int64
	blocked  int64
	failed   int64
	types    map[string]int64
}

func newGatewayStats() *gatewayStats {
	return &gatewayStats{types: map[string]int64{}}
}

// record anota una llamada. tipos son los TIPOS de secreto tapados, nunca los valores.
func (g *gatewayStats) record(tipos []string, blocked, failed bool) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if len(tipos) > 0 {
		g.scrubbed++
		for _, t := range tipos {
			g.types[t]++
		}
	}
	if blocked {
		g.blocked++
	}
	if failed {
		g.failed++
	}
}

func (g *gatewayStats) snapshot(st *CognitionStats) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	st.GatewayCalls += g.calls
	st.GatewayScrubbed += g.scrubbed
	st.GatewayBlocked += g.blocked
	st.GatewayFailed += g.failed
	if len(g.types) > 0 {
		if st.GatewayTypes == nil {
			st.GatewayTypes = map[string]int64{}
		}
		for t, n := range g.types {
			st.GatewayTypes[t] += n
		}
	}
}

// routerStats son los contadores del router de flota.
type routerStats struct {
	mu          sync.Mutex
	escalations int64
	exhausted   int64
}

func (r *routerStats) escalated() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.escalations++
	r.mu.Unlock()
}

func (r *routerStats) ranOut() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.exhausted++
	r.mu.Unlock()
}

func (r *routerStats) snapshot(st *CognitionStats) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st.RouterEscalations += r.escalations
	st.RouterExhausted += r.exhausted
}
