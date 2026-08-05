package cognition

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Caché de cognición (F3): responde sin llamar al motor cuando ya se preguntó lo mismo.
//
// DÓNDE SE PARA, Y POR QUÉ NO DONDE PARECÍA MEJOR
// El decorador va POR FUERA del portero de privacidad: caller → caché → portero → motor.
//
// La alternativa elegante era ponerlo adentro, cacheando el prompt ya tapado: el caché no guardaría
// un secreto jamás y el hit rate subiría, porque dos prompts que sólo difieren en el VALOR del
// secreto colapsan al mismo texto. Se descartó porque el supuesto no se sostiene: privacy.mint
// acuña los marcadores con un contador que REINTENTA ante colisión, y la colisión se chequea contra
// el texto CRUDO. Dos sesiones con el mismo texto tapado pueden numerar distinto, y entonces la
// respuesta cacheada dice "marcador 2" queriendo decir el segundo secreto mientras la sesión que
// rehidrata pone el primero. No es una fuga —los dos secretos son de quien pregunta— pero es una
// respuesta incorrecta y silenciosa.
//
// El costo de estar afuera, dicho de frente: acá se guardan prompts y respuestas CRUDOS, en
// memoria. Es contenido que el proceso ya tiene en RAM porque salió de la base local, así que no
// agrega una clase de exposición nueva; agrega tiempo de retención, y nada toca disco.

// cacheEntry es una respuesta guardada con el instante en que se guardó (para el TTL).
type cacheEntry struct {
	key    string
	answer string
	stored time.Time
}

// cached es el decorador. Implementa Provider delegando en inner ante un miss.
type cached struct {
	inner Provider

	mu      sync.Mutex
	entries map[string]*list.Element // clave → nodo del LRU
	lru     *list.List               // frente = usado más recientemente
	max     int
	ttl     time.Duration

	// now es el reloj, inyectable para que el test del vencimiento no dependa de dormir. nil ⇒
	// time.Now. Un test que espera un TTL real es lento y, peor, intermitente.
	now func() time.Time

	// hits y misses son para la telemetría de F5. Se cuentan siempre: medir es barato y sin
	// números no hay forma de saber si el caché sirve o es decoración.
	hits, misses int64
}

// newCached envuelve un Provider con el caché. Devuelve el Provider TAL CUAL cuando no hay nada que
// cachear (motor Noop) — el camino model-free queda bit-idéntico (K3).
func newCached(inner Provider, maxEntries int, ttl time.Duration) (Provider, error) {
	if _, noop := inner.(NoopProvider); noop {
		return inner, nil
	}
	// Un caché sin cota es una fuga de memoria con nombre amable. Falla explícito en vez de
	// elegir un default en silencio: la misma regla que el modo desconocido del portero.
	if maxEntries <= 0 {
		return nil, fmt.Errorf("cognition.cache.max_entries debe ser > 0 (se recibió %d): un caché sin cota es una fuga de memoria", maxEntries)
	}
	return &cached{
		inner:   inner,
		entries: make(map[string]*list.Element, maxEntries),
		lru:     list.New(),
		max:     maxEntries,
		ttl:     ttl,
	}, nil
}

// Name delega: para el caller, el motor sigue siendo el mismo.
func (c *cached) Name() string { return c.inner.Name() }

// unwrapCache saca la capa de caché, si está, y devuelve lo que envuelve.
//
// Existe para las aserciones que verifican QUÉ construyó la fábrica —sobre todo la que comprueba
// que el motor real nunca sale sin portero (garantía de F1) y la que obliga al doctor a contar la
// misma historia que el constructor—. Sin esto habría que aflojar esas aserciones a "es cualquier
// cosa", que es justo perder lo que protegen. Con el helper, agregar una capa nueva mañana no las
// rompe ni las debilita.
func unwrapCache(p Provider) Provider {
	if c, ok := p.(*cached); ok {
		return c.inner
	}
	return p
}

func (c *cached) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// cacheKey deriva la clave de (system, user).
//
// PREFIJA CADA PARTE CON SU LARGO, y no es adorno: concatenar a secas hace colisionar
// ("ab","c") con ("a","bc"), que es el bug clásico de las claves compuestas — y acá sería una
// violación de K0, o sea servir la respuesta de otra pregunta. Con el largo delante, la
// descomposición es única.
func cacheKey(system, user string) string {
	h := sha256.New()
	h.Write([]byte(strconv.Itoa(len(system))))
	h.Write([]byte{0})
	h.Write([]byte(system))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(len(user))))
	h.Write([]byte{0})
	h.Write([]byte(user))
	return hex.EncodeToString(h.Sum(nil))
}

// get busca una entrada VIGENTE y la marca como recién usada. El bool es "hubo hit".
func (c *cached) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.entries[key]
	if !ok {
		c.misses++
		return "", false
	}
	e := el.Value.(*cacheEntry)
	if c.ttl > 0 && c.clock().Sub(e.stored) >= c.ttl {
		// Vencida: se saca acá mismo en vez de esperar al desalojo por presión. Si no, una entrada
		// muerta seguiría ocupando lugar bueno hasta que el LRU la alcance.
		c.lru.Remove(el)
		delete(c.entries, key)
		c.misses++
		return "", false
	}
	c.lru.MoveToFront(el)
	c.hits++
	return e.answer, true
}

// put guarda una respuesta y desaloja LA MÁS VIEJA si hace falta.
//
// De a UNA, no vaciando el mapa entero. El rerankCache que esto reemplaza se vaciaba completo al
// llenarse: tiraba 511 entradas buenas para hacer lugar a una.
func (c *cached) put(key, answer string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.entries[key]; ok {
		e := el.Value.(*cacheEntry)
		e.answer = answer
		e.stored = c.clock()
		c.lru.MoveToFront(el)
		return
	}
	el := c.lru.PushFront(&cacheEntry{key: key, answer: answer, stored: c.clock()})
	c.entries[key] = el

	for c.lru.Len() > c.max {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.lru.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheEntry).key)
	}
}

// Ask responde del caché si puede, y si no delega en el motor y guarda la respuesta.
func (c *cached) Ask(ctx context.Context, system, user string) (string, error) {
	key := cacheKey(system, user)
	if answer, ok := c.get(key); ok {
		return answer, nil
	}
	answer, err := c.inner.Ask(ctx, system, user)
	if err != nil {
		// LOS ERRORES NO SE CACHEAN (K2). Cachear un fallo transitorio lo vuelve permanente: un
		// rate-limit de 30 s quedaría servido durante todo el TTL. El caché guarda respuestas,
		// no resultados.
		return "", err
	}
	c.put(key, answer)
	return answer, nil
}

// Stats devuelve hits y misses acumulados. Es la superficie que consume la telemetría de F5: sin
// números no hay forma de saber si el caché sirve o es decoración.
func (c *cached) Stats() (hits, misses int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

// reportStats suma lo del caché y sigue hacia adentro (F5).
func (c *cached) reportStats(st *CognitionStats) {
	c.mu.Lock()
	st.CacheHits += c.hits
	st.CacheMisses += c.misses
	st.CacheSize += c.lru.Len()
	inner := c.inner
	c.mu.Unlock() // se suelta ANTES de bajar: la capa de adentro toma su propio lock

	if r, ok := inner.(statsReporter); ok {
		r.reportStats(st)
	}
}

// Len es cuántas entradas hay guardadas. Para los tests del desalojo.
func (c *cached) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}
