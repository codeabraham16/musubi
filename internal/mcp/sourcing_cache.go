package mcp

import (
	"sync"
	"time"
)

// sourcing_cache.go implementa un caché en memoria, con TTL, para las respuestas de red
// del sourcing de skills (catálogo curado y marketplace). Evita repetir el mismo GET en
// ventanas cortas: las queries de descubrimiento se repiten (sin argumentos, la query se
// deriva del stack y es estable), así que cachearlas convierte N llamadas en 1 fetch +
// (N-1) hits locales. Es la base de ingesta del cosechador del catálogo (Track 8): el
// harvest re-consulta lo mismo entre corridas y el caché le ahorra presupuesto de API.
//
// Pensado para concurrencia: las tools de sourcing son read-only y se despachan en
// paralelo bajo RLock, así que el caché se protege con su propio mutex. TTL ≤ 0 lo
// desactiva (cada fetch va a la red), útil en tests que quieren respuestas frescas.

// cacheEntry es un valor cacheado con su instante de expiración.
type cacheEntry struct {
	value   interface{}
	expires time.Time
}

// sourcingCache es un caché clave→valor con expiración por TTL, seguro para concurrencia.
type sourcingCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]cacheEntry
}

// newSourcingCache crea un caché con TTL de seconds segundos. seconds ≤ 0 => caché inerte
// (get siempre falla, set es no-op): el sourcing sigue funcionando, solo sin cachear.
func newSourcingCache(seconds int) *sourcingCache {
	return &sourcingCache{
		ttl: time.Duration(seconds) * time.Second,
		m:   make(map[string]cacheEntry),
	}
}

// get devuelve el valor cacheado para key si existe y no expiró. Una entrada expirada se
// elimina al leerla (limpieza perezosa). Devuelve ok=false si el caché está inerte.
func (c *sourcingCache) get(key string) (interface{}, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expires) {
		delete(c.m, key)
		return nil, false
	}
	return e.value, true
}

// set guarda value bajo key con la expiración del TTL. No-op si el caché está inerte.
func (c *sourcingCache) set(key string, value interface{}) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{value: value, expires: time.Now().Add(c.ttl)}
}
