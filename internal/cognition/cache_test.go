package cognition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
)

// Invariantes del CACHÉ DE COGNICIÓN (F3). Ver specs/cache-cognicion/spec.md.
// Cada test se verificó FALLANDO al sabotear la implementación (ver tasks.md).

// espia cuenta llamadas y devuelve una respuesta derivada del prompt, para poder distinguir
// "vino del motor" de "vino del caché" y detectar una respuesta CRUZADA (K0).
type espia struct {
	mu      sync.Mutex
	llamadas int
	err     error
}

func (e *espia) Name() string { return "espia" }

func (e *espia) Ask(_ context.Context, system, user string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llamadas++
	if e.err != nil {
		return "", e.err
	}
	return "R(" + system + "|" + user + ")#" + fmt.Sprint(e.llamadas), nil
}

func (e *espia) n() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.llamadas
}

func nuevoCache(t *testing.T, inner Provider, max int, ttl time.Duration) *cached {
	t.Helper()
	p, err := newCached(inner, max, ttl)
	if err != nil {
		t.Fatalf("newCached: %v", err)
	}
	c, ok := p.(*cached)
	if !ok {
		t.Fatalf("newCached devolvió %T, esperaba *cached", p)
	}
	return c
}

// --- K0: un hit devuelve lo del MISMO prompt, nunca lo de otro ---------------------------------

func TestK0ElHitDevuelveLaRespuestaDeSuPropioPrompt(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 10, 0)
	ctx := context.Background()

	a1, err := c.Ask(ctx, "sys-A", "user-A")
	if err != nil {
		t.Fatalf("Ask A: %v", err)
	}
	b1, err := c.Ask(ctx, "sys-B", "user-B")
	if err != nil {
		t.Fatalf("Ask B: %v", err)
	}
	if a1 == b1 {
		t.Fatalf("el espía devolvió lo mismo para prompts distintos: el test no está probando nada")
	}

	// Segunda vuelta: los dos deben salir del caché y cada uno con LO SUYO.
	a2, _ := c.Ask(ctx, "sys-A", "user-A")
	b2, _ := c.Ask(ctx, "sys-B", "user-B")
	if a2 != a1 {
		t.Errorf("FUGA K0: el hit de A devolvió %q, esperaba %q", a2, a1)
	}
	if b2 != b1 {
		t.Errorf("FUGA K0: el hit de B devolvió %q, esperaba %q", b2, b1)
	}
	if e.n() != 2 {
		t.Errorf("esperaba 2 llamadas al motor (una por prompt), hubo %d", e.n())
	}
}

// --- K8: la clave distingue system de user, y no colisiona por concatenación -------------------

func TestK8LaClaveNoColisionaPorConcatenacion(t *testing.T) {
	// El bug clásico: concatenar a secas hace ("ab","c") == ("a","bc"). Sería una violación de K0.
	pares := [][2]string{
		{"ab", "c"},
		{"a", "bc"},
		{"abc", ""},
		{"", "abc"},
		{"c", "ab"}, // invertido
	}
	vistas := map[string][2]string{}
	for _, p := range pares {
		k := cacheKey(p[0], p[1])
		if otra, dup := vistas[k]; dup {
			t.Errorf("FUGA K8: %v y %v comparten clave", otra, p)
		}
		vistas[k] = p
	}

	// Y de punta a punta: dos pares que concatenan igual deben pegarle al motor por separado.
	e := &espia{}
	c := nuevoCache(t, e, 10, 0)
	ctx := context.Background()
	r1, _ := c.Ask(ctx, "ab", "c")
	r2, _ := c.Ask(ctx, "a", "bc")
	if r1 == r2 {
		t.Errorf("FUGA K8: ('ab','c') y ('a','bc') devolvieron lo mismo")
	}
	if e.n() != 2 {
		t.Errorf("esperaba 2 llamadas al motor, hubo %d", e.n())
	}
}

// --- K1: sin entrada, se llama al motor --------------------------------------------------------

func TestK1UnMissLlamaAlMotor(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 10, 0)
	for i := 0; i < 3; i++ {
		if _, err := c.Ask(context.Background(), "s", fmt.Sprintf("pregunta-%d", i)); err != nil {
			t.Fatalf("Ask: %v", err)
		}
	}
	if e.n() != 3 {
		t.Errorf("K1: 3 prompts distintos ⇒ 3 llamadas al motor, hubo %d", e.n())
	}
}

// --- K2: los errores NO se cachean -------------------------------------------------------------

func TestK2UnErrorNoSeCachea(t *testing.T) {
	fallo := errors.New("rate limit transitorio")
	e := &espia{err: fallo}
	c := nuevoCache(t, e, 10, 0)
	ctx := context.Background()

	if _, err := c.Ask(ctx, "s", "u"); !errors.Is(err, fallo) {
		t.Fatalf("esperaba el error del motor, obtuve %v", err)
	}
	// El motor se recupera: la MISMA pregunta tiene que volver a intentarse, no servir el error.
	e.mu.Lock()
	e.err = nil
	e.mu.Unlock()

	ans, err := c.Ask(ctx, "s", "u")
	if err != nil {
		t.Fatalf("FUGA K2: tras recuperarse el motor, el caché sirvió el fallo: %v", err)
	}
	if ans == "" {
		t.Errorf("K2: esperaba una respuesta real tras la recuperación")
	}
	if e.n() != 2 {
		t.Errorf("K2: el segundo intento debía llegar al motor; llamadas=%d", e.n())
	}
}

// --- K4: cota dura, y desalojo de a UNA --------------------------------------------------------

func TestK4CotaDuraYDesalojoDeAUna(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 3, 0)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := c.Ask(ctx, "s", fmt.Sprintf("p%d", i)); err != nil {
			t.Fatalf("Ask: %v", err)
		}
	}
	if c.Len() != 3 {
		t.Fatalf("esperaba 3 entradas, hay %d", c.Len())
	}
	// Tocar p0 lo vuelve el más reciente; el que debe caer es p1.
	if _, err := c.Ask(ctx, "s", "p0"); err != nil {
		t.Fatalf("Ask p0: %v", err)
	}
	if _, err := c.Ask(ctx, "s", "p3"); err != nil { // fuerza el desalojo
		t.Fatalf("Ask p3: %v", err)
	}
	if c.Len() != 3 {
		t.Errorf("FUGA K4: la cota no se respetó, hay %d entradas (max 3)", c.Len())
	}

	// p0 debe seguir cacheado (no se llamó al motor de nuevo).
	antes := e.n()
	if _, err := c.Ask(ctx, "s", "p0"); err != nil {
		t.Fatalf("Ask p0 (2): %v", err)
	}
	if e.n() != antes {
		t.Errorf("K4: p0 era el más reciente y fue desalojado ⇒ el desalojo no es LRU")
	}
	// p1 debe haber caído.
	if _, err := c.Ask(ctx, "s", "p1"); err != nil {
		t.Fatalf("Ask p1: %v", err)
	}
	if e.n() != antes+1 {
		t.Errorf("K4: p1 era el menos reciente y debía haberse desalojado")
	}
}

// K4 — el caché NO se vacía entero al llenarse (que es lo que hacía el rerankCache que reemplaza).
func TestK4NoSeVaciaEnteroAlLlenarse(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 4, 0)
	ctx := context.Background()
	for i := 0; i < 5; i++ { // uno más que la cota
		if _, err := c.Ask(ctx, "s", fmt.Sprintf("p%d", i)); err != nil {
			t.Fatalf("Ask: %v", err)
		}
	}
	if c.Len() != 4 {
		t.Errorf("FUGA K4: tras pasarse por uno quedan %d entradas; vaciarse entero tira las buenas", c.Len())
	}
}

// --- K6: una entrada vencida no se sirve -------------------------------------------------------

func TestK6UnaEntradaVencidaNoSeSirve(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 10, time.Minute)
	// Reloj inyectado: un test que espera un TTL real es lento y, peor, intermitente.
	ahora := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return ahora }
	ctx := context.Background()

	if _, err := c.Ask(ctx, "s", "u"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	// Justo ANTES del borde: sigue vigente.
	ahora = ahora.Add(59 * time.Second)
	if _, err := c.Ask(ctx, "s", "u"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if e.n() != 1 {
		t.Errorf("K6: a los 59 s con TTL de 60 s la entrada debía seguir vigente; llamadas=%d", e.n())
	}
	// En el borde exacto: vencida.
	ahora = ahora.Add(time.Second)
	if _, err := c.Ask(ctx, "s", "u"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if e.n() != 2 {
		t.Errorf("FUGA K6: la entrada vencida se sirvió igual; llamadas=%d", e.n())
	}
}

func TestK6TTLCeroNoVence(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 10, 0)
	ahora := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return ahora }
	ctx := context.Background()

	if _, err := c.Ask(ctx, "s", "u"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	ahora = ahora.Add(100 * 24 * time.Hour)
	if _, err := c.Ask(ctx, "s", "u"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if e.n() != 1 {
		t.Errorf("K6: con ttl=0 la entrada no debe vencer nunca; llamadas=%d", e.n())
	}
}

// --- K3: bit-identidad con el caché apagado y con el motor Noop --------------------------------

func TestK3NoopNoSeEnvuelve(t *testing.T) {
	p, err := newCached(NoopProvider{}, 10, 0)
	if err != nil {
		t.Fatalf("newCached: %v", err)
	}
	if _, esCache := p.(*cached); esCache {
		t.Errorf("K3: el NoopProvider no debe envolverse; no hay nada que cachear")
	}
}

func TestK3ApagadoDevuelveElProviderTalCual(t *testing.T) {
	no := false
	cfg := config.CognitionConfig{Cache: config.CacheConfig{Enabled: &no}}
	e := &espia{}
	p, err := withCache(e, cfg)
	if err != nil {
		t.Fatalf("withCache: %v", err)
	}
	if p != Provider(e) {
		t.Errorf("K3: con el caché apagado se debe devolver el Provider tal cual, obtuve %T", p)
	}
}

func TestCacheEncendidoPorDefecto(t *testing.T) {
	// Ausente ⇒ true. Con un bool pelado, omitir el bloque apagaría el caché (el cero de Go),
	// que es lo contrario del default buscado.
	if !(config.CacheConfig{}).CacheEnabled() {
		t.Errorf("el caché debe nacer ENCENDIDO cuando no se declara nada")
	}
	si := true
	if !(config.CacheConfig{Enabled: &si}).CacheEnabled() {
		t.Errorf("enabled: true debe encender")
	}
	no := false
	if (config.CacheConfig{Enabled: &no}).CacheEnabled() {
		t.Errorf("enabled: false debe apagar")
	}
}

// --- Cota inválida: error explícito, no default silencioso -------------------------------------

func TestCotaInvalidaEsError(t *testing.T) {
	for _, max := range []int{0, -1, -100} {
		if _, err := newCached(&espia{}, max, 0); err == nil {
			t.Errorf("max_entries=%d debía ser error: un caché sin cota es una fuga de memoria", max)
		} else if !strings.Contains(err.Error(), "max_entries") {
			t.Errorf("el error debe nombrar el campo de config; obtuve %v", err)
		}
	}
}

// --- K7: seguro bajo concurrencia (correr con -race) -------------------------------------------

func TestK7ConcurrenciaNoCorrompeElCache(t *testing.T) {
	e := &espia{}
	c := nuevoCache(t, e, 16, 0)
	ctx := context.Background()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				// Mezcla de claves repetidas (hits) y nuevas (desalojos) para ejercitar el LRU.
				if _, err := c.Ask(ctx, "s", fmt.Sprintf("p%d", i%24)); err != nil {
					t.Errorf("Ask concurrente: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if c.Len() > 16 {
		t.Errorf("K7/K4: la cota se violó bajo concurrencia: %d entradas", c.Len())
	}
	hits, misses := c.Stats()
	if hits+misses != 400 {
		t.Errorf("las estadísticas no cuadran: %d hits + %d misses, esperaba 400", hits, misses)
	}
}
