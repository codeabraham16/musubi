package mcp

import (
	"sync"
	"time"
)

// quota.go implementa una CUOTA de uso por-principal (Track 16 / Producible F3.2): limita las
// llamadas a tools/call de cada principal en una ventana deslizante, en memoria y model-free
// (como authLimiter, pero contando TODAS las llamadas de un principal autenticado, no solo los
// fallos de auth por IP). Protege al cerebro central de un principal desbocado —antes, una vez
// autenticado, un principal podía hacer saves/recalls ilimitados. Solo aplica cuando hay un
// principal (modo serve con registro); en stdio local (sin principal) no hay cuota.

// quotaLimiter cuenta las llamadas por clave (nombre del principal) en una ventana deslizante.
// max<=0 (o receptor nil) ⇒ desactivado (siempre permite). Los timestamps por clave se podan a
// la ventana en cada consulta, así los slices quedan acotados a `max` y las claves a la
// cantidad finita de principals registrados.
type quotaLimiter struct {
	mu     sync.Mutex
	hits   map[string][]int64 // principal → timestamps (unix nanos) dentro de la ventana
	max    int
	window time.Duration
}

func newQuotaLimiter(max int, window time.Duration) *quotaLimiter {
	return &quotaLimiter{hits: make(map[string][]int64), max: max, window: window}
}

// allow registra una llamada de key en el instante now y devuelve true si sigue DENTRO de la
// cuota (o si la cuota está desactivada). Poda los timestamps anteriores a la ventana antes de
// contar; al alcanzar max en la ventana, rechaza sin registrar (la llamada no cuenta contra el
// futuro). key vacía ⇒ sin cuota (no hay principal identificable).
func (q *quotaLimiter) allow(key string, now time.Time) bool {
	if q == nil || q.max <= 0 || key == "" {
		return true
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	cutoff := now.Add(-q.window).UnixNano()
	kept := q.hits[key][:0]
	for _, t := range q.hits[key] {
		if t >= cutoff {
			kept = append(kept, t)
		}
	}
	if len(kept) >= q.max {
		q.hits[key] = kept // persistir la poda aunque rechacemos
		return false
	}
	q.hits[key] = append(kept, now.UnixNano())
	return true
}
