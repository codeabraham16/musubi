package mcp

import (
	"sync"
	"time"
)

// authlimit.go implementa un lockout en memoria contra la fuerza bruta del bearer (Track 16
// F1 16.1e): tras `max` fallos de autenticación desde una misma IP, la bloquea por `window`.
// Un auth exitoso resetea su contador. Model-free, sin dependencias. Acota el adivinado
// online del token, que antes era ilimitado para cualquier peer del tailnet.

type authLimiter struct {
	mu     sync.Mutex
	fails  map[string]int
	until  map[string]time.Time
	max    int
	window time.Duration
}

func newAuthLimiter(max int, window time.Duration) *authLimiter {
	return &authLimiter{
		fails:  make(map[string]int),
		until:  make(map[string]time.Time),
		max:    max,
		window: window,
	}
}

// locked reporta si la IP está en lockout en el instante now. Limpia las entradas expiradas
// de paso (mantiene los mapas acotados en un tailnet de peers finitos).
func (l *authLimiter) locked(ip string, now time.Time) bool {
	if l == nil || ip == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	u, ok := l.until[ip]
	if !ok {
		return false
	}
	if now.Before(u) {
		return true
	}
	delete(l.until, ip)
	delete(l.fails, ip)
	return false
}

// fail registra un fallo de auth de la IP; al alcanzar max, activa el lockout por window
// (y reinicia el contador para que el próximo ciclo requiera de nuevo max fallos).
func (l *authLimiter) fail(ip string, now time.Time) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip]++
	if l.fails[ip] >= l.max {
		l.until[ip] = now.Add(l.window)
		l.fails[ip] = 0
	}
}

// reset limpia el contador de la IP tras un auth exitoso.
func (l *authLimiter) reset(ip string) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
	delete(l.until, ip)
}
