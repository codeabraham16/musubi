package cognition

import (
	"sync"
	"time"
)

// breaker es el circuit breaker de UN motor de la flota.
//
// Mide SALUD, no política: sólo cuentan las fallas que indican que el motor no sirve ahora mismo.
// Que un motor se niegue a mandar un texto con secretos no lo vuelve un motor roto — ver el
// tratamiento de ErrSecretsBlocked en el router (invariante C2).
//
// Tres estados implícitos, sin enum:
//
//	cerrado    openUntil en el pasado           → se intenta
//	abierto    now < openUntil                  → se saltea
//	half-open  venció openUntil y !probing      → se intenta UNA vez
type breaker struct {
	mu        sync.Mutex
	fails     int
	openUntil time.Time
	// probing marca que ya hay una llamada de prueba en vuelo tras vencer el cooldown.
	//
	// Es lo que hace cierto el "exactamente una" del invariante C4: sin esto, diez goroutines que
	// llegan justo al vencer el cooldown se irían TODAS contra un motor que probablemente sigue
	// caído. La primera pone la bandera; las demás lo saltean como si siguiera abierto.
	probing bool

	failures int           // fallas consecutivas que abren el circuito
	cooldown time.Duration // cuánto queda fuera de la rotación
	now      func() time.Time
}

func newBreaker(failures int, cooldown time.Duration, now func() time.Time) *breaker {
	if now == nil {
		now = time.Now
	}
	return &breaker{failures: failures, cooldown: cooldown, now: now}
}

// allow indica si el router puede intentar este motor ahora.
//
// Tiene efecto de lado a propósito: al conceder una prueba half-open, la reserva. Por eso no se
// llama "canTry" — preguntar YA es tomar el turno, y así dos goroutines no se llevan la misma prueba.
func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openUntil.IsZero() || b.now().After(b.openUntil) {
		if b.openUntil.IsZero() {
			return true // circuito cerrado: paso libre
		}
		// Venció el cooldown: media apertura. Sólo pasa la primera.
		if b.probing {
			return false
		}
		b.probing = true
		return true
	}
	return false // abierto y todavía en cooldown
}

// success cierra el circuito y limpia el conteo. Un motor que contesta bien vuelve a cero: el
// contador mide fallas CONSECUTIVAS, no fallas históricas.
func (b *breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fails = 0
	b.openUntil = time.Time{}
	b.probing = false
}

// failure suma una falla y abre el circuito si se alcanzó el umbral.
//
// Si la falla vino de una prueba half-open, se vuelve a abrir por un cooldown COMPLETO: un motor que
// falla la prueba no merece que lo reintenten enseguida.
func (b *breaker) failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fails++
	if b.probing || b.fails >= b.failures {
		b.openUntil = b.now().Add(b.cooldown)
		b.probing = false
	}
}

// abstain libera la prueba half-open sin contarla ni como éxito ni como falla.
//
// Es lo que corresponde cuando el motor se NEGÓ por política: negarse no dice nada sobre su salud.
// Sin esto, una negativa que cae justo en una prueba dejaría `probing` en true para siempre y el
// motor quedaría fuera de la rotación por un motivo que no tiene que ver con estar caído.
func (b *breaker) abstain() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
}

// open indica si el circuito está abierto ahora (para el diagnóstico; no reserva prueba).
func (b *breaker) open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.openUntil.IsZero() && !b.now().After(b.openUntil)
}
