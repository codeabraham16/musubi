package cognition

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
)

// reloj controlable: los tests de cooldown no pueden depender del reloj real (serían lentos y
// flaky, y no probarían el borde exacto).
type reloj struct {
	mu sync.Mutex
	t  time.Time
}

func nuevoReloj() *reloj { return &reloj{t: time.Unix(1_700_000_000, 0)} }

func (r *reloj) now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *reloj) avanzar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.t = r.t.Add(d)
}

// motorFalso es un Provider scripteado que cuenta invocaciones y guarda lo que recibió.
type motorFalso struct {
	mu        sync.Mutex
	nombre    string
	recibidos []string
	llamadas  int32
	respuesta string
	err       error
}

func (m *motorFalso) Name() string { return m.nombre }

func (m *motorFalso) Ask(_ context.Context, _, user string) (string, error) {
	atomic.AddInt32(&m.llamadas, 1)
	m.mu.Lock()
	m.recibidos = append(m.recibidos, user)
	m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	return m.respuesta, nil
}

func (m *motorFalso) veces() int { return int(atomic.LoadInt32(&m.llamadas)) }

func (m *motorFalso) ultimo() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recibidos) == 0 {
		return ""
	}
	return m.recibidos[len(m.recibidos)-1]
}

// armarRouter compone una flota con motores ya envueltos por su portero real.
func armarRouter(t *testing.T, rel *reloj, fails int, cooldown time.Duration, defs ...struct {
	m    *motorFalso
	tier string
}) *router {
	t.Helper()
	r := &router{}
	for _, d := range defs {
		modo := config.DefaultGatewayModeForTier(d.tier)
		g, err := newGuarded(d.m, modo)
		if err != nil {
			t.Fatalf("newGuarded(%s): %v", d.m.nombre, err)
		}
		r.engines = append(r.engines, &engine{
			name: d.m.nombre, tier: d.tier, inner: g,
			breaker: newBreaker(fails, cooldown, rel.now),
		})
	}
	return r
}

type def = struct {
	m    *motorFalso
	tier string
}

// --- C0: un secreto nunca llega a un motor de tier free -------------------------------------

func TestC0ElSecretoNoLlegaAlTierGratis(t *testing.T) {
	gratis := &motorFalso{nombre: "groq", respuesta: "no deberías ver esto"}
	privado := &motorFalso{nombre: "max", respuesta: "listo"}
	// El gratis va PRIMERO a propósito: la protección no puede depender del orden.
	r := armarRouter(t, nuevoReloj(), 3, time.Minute,
		def{gratis, config.TierFree}, def{privado, config.TierPrivate})

	got, err := r.Ask(context.Background(), "", "la clave es "+tSecretoAWS)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gratis.veces() != 0 {
		t.Fatalf("FUGA C0: el motor de tier free fue invocado con un texto que lleva un secreto (recibió %q)", gratis.ultimo())
	}
	if privado.veces() != 1 {
		t.Fatalf("el motor privado tenía que atender, fue invocado %d veces", privado.veces())
	}
	if strings.Contains(privado.ultimo(), tSecretoAWS) {
		t.Fatalf("FUGA: el motor privado recibió el secreto sin tapar: %q", privado.ultimo())
	}
	if got != "listo" {
		t.Fatalf("respuesta inesperada: %q", got)
	}
}

func TestC0SinPrivadoElSecretoNoVaANingunLado(t *testing.T) {
	gratis := &motorFalso{nombre: "groq", respuesta: "no"}
	r := armarRouter(t, nuevoReloj(), 3, time.Minute, def{gratis, config.TierFree})

	_, err := r.Ask(context.Background(), "", "la clave es "+tSecretoAWS)
	if !errors.Is(err, ErrAllEnginesDown) || !errors.Is(err, ErrSecretsBlocked) {
		t.Fatalf("esperaba agotamiento POR NEGATIVA (ambos errores), hubo: %v", err)
	}
	if gratis.veces() != 0 {
		t.Fatalf("FUGA C0: sin motor privado el texto igual salió al gratis")
	}
}

func TestC0ElTierDerivaElModoDelPortero(t *testing.T) {
	// La otra mitad de C0: que la config free/private se traduzca en refuse/scrub sola.
	cfg := config.CognitionConfig{Fleet: []config.FleetEngineConfig{
		{Name: "gratis", Provider: "openai-compat", Endpoint: "http://x/v1", Model: "m"},
		{Name: "propio", Provider: "openai-compat", Endpoint: "http://y/v1", Model: "m", Tier: config.TierPrivate},
	}}
	p, err := newRouter(cfg, nil)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	r := p.(*router)
	if r.engines[0].tier != config.TierFree {
		t.Fatalf("un motor sin tier tenía que quedar en %q, quedó en %q", config.TierFree, r.engines[0].tier)
	}
	for i, quiero := range []string{GatewayModeRefuse, GatewayModeScrub} {
		g, ok := r.engines[i].inner.(guarded)
		if !ok {
			t.Fatalf("el motor %d salió sin portero: %T", i, r.engines[i].inner)
		}
		if g.mode != quiero {
			t.Fatalf("el motor %d (%s) tenía que nacer en %q, nació en %q",
				i, r.engines[i].tier, quiero, g.mode)
		}
	}
}

// --- C1: orden y primera respuesta ----------------------------------------------------------

func TestC1DevuelveElPrimeroQueContesta(t *testing.T) {
	uno := &motorFalso{nombre: "uno", respuesta: "de uno"}
	dos := &motorFalso{nombre: "dos", respuesta: "de dos"}
	r := armarRouter(t, nuevoReloj(), 3, time.Minute,
		def{uno, config.TierPrivate}, def{dos, config.TierPrivate})

	got, err := r.Ask(context.Background(), "", "pregunta limpia")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "de uno" {
		t.Fatalf("tenía que ganar el primero, devolvió %q", got)
	}
	if dos.veces() != 0 {
		t.Fatal("el segundo motor fue invocado sin necesidad")
	}
}

// --- C2: negarse no es fallar ---------------------------------------------------------------

func TestC2NegarsePorPoliticaNoAbreElCircuito(t *testing.T) {
	gratis := &motorFalso{nombre: "groq", respuesta: "ok"}
	privado := &motorFalso{nombre: "max", respuesta: "ok"}
	// failures=2: si negarse contara, dos prompts con secretos lo apagarían.
	r := armarRouter(t, nuevoReloj(), 2, time.Minute,
		def{gratis, config.TierFree}, def{privado, config.TierPrivate})

	for i := 0; i < 5; i++ {
		if _, err := r.Ask(context.Background(), "", "clave "+tSecretoAWS); err != nil {
			t.Fatalf("Ask #%d: %v", i, err)
		}
	}
	// Ahora un texto limpio: el gratis tiene que seguir en la rotación y atenderlo.
	if _, err := r.Ask(context.Background(), "", "pregunta limpia"); err != nil {
		t.Fatalf("Ask limpio: %v", err)
	}
	if gratis.veces() != 1 {
		t.Fatalf("FUGA C2: negarse contó como falla y apagó un motor sano (el gratis fue invocado %d veces)", gratis.veces())
	}
}

// --- C3: el breaker abre y saltea -----------------------------------------------------------

func TestC3TrasNFallasElMotorSeSaltea(t *testing.T) {
	roto := &motorFalso{nombre: "roto", err: errors.New("500")}
	sano := &motorFalso{nombre: "sano", respuesta: "ok"}
	r := armarRouter(t, nuevoReloj(), 3, time.Minute,
		def{roto, config.TierPrivate}, def{sano, config.TierPrivate})

	for i := 0; i < 3; i++ {
		if _, err := r.Ask(context.Background(), "", "hola"); err != nil {
			t.Fatalf("Ask #%d: %v", i, err)
		}
	}
	if roto.veces() != 3 {
		t.Fatalf("esperaba 3 intentos antes de abrir, hubo %d", roto.veces())
	}
	for i := 0; i < 5; i++ {
		if _, err := r.Ask(context.Background(), "", "hola"); err != nil {
			t.Fatalf("Ask post-apertura #%d: %v", i, err)
		}
	}
	if roto.veces() != 3 {
		t.Fatalf("FUGA C3: el circuito no abrió, el motor roto se siguió intentando (%d veces)", roto.veces())
	}
}

func TestC3UnExitoResetaElConteo(t *testing.T) {
	// El contador mide fallas CONSECUTIVAS: fallar, andar, fallar no puede abrir con failures=2.
	intermitente := &motorFalso{nombre: "inter"}
	r := armarRouter(t, nuevoReloj(), 2, time.Minute, def{intermitente, config.TierPrivate})

	intermitente.err = errors.New("500")
	_, _ = r.Ask(context.Background(), "", "hola")
	intermitente.err = nil
	intermitente.respuesta = "ok"
	_, _ = r.Ask(context.Background(), "", "hola")
	intermitente.err = errors.New("500")
	_, _ = r.Ask(context.Background(), "", "hola")

	antes := intermitente.veces()
	intermitente.err = nil
	if _, err := r.Ask(context.Background(), "", "hola"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if intermitente.veces() != antes+1 {
		t.Fatal("FUGA C3: el éxito intermedio no reseteó el contador y el circuito abrió de más")
	}
}

// --- C4: half-open, exactamente una prueba --------------------------------------------------

// La prueba half-open se RESERVA: preguntar ya es tomar el turno.
//
// Este test va contra el breaker directamente y no contra el router, y es a propósito. La versión
// concurrente (abajo) NO alcanza: la primera goroutine que falla vuelve a abrir el circuito antes de
// que las otras lleguen a allow(), así que la ventana de carrera es diminuta y el test pasa por
// suerte de timing. Lo comprobé sacando la reserva: el test concurrente siguió en verde.
func TestC4LaReservaHalfOpenEsExclusiva(t *testing.T) {
	rel := nuevoReloj()
	b := newBreaker(1, time.Minute, rel.now)
	b.failure() // umbral 1 ⇒ abre
	if !b.open() {
		t.Fatal("el circuito tenía que estar abierto")
	}
	rel.avanzar(2 * time.Minute)

	if !b.allow() {
		t.Fatal("vencido el cooldown, la primera prueba tenía que pasar")
	}
	if b.allow() {
		t.Fatal("FUGA C4: entró una SEGUNDA prueba antes de que la primera reportara resultado")
	}
	// Y tras reportar, el turno se libera según lo que haya pasado.
	b.success()
	if !b.allow() {
		t.Fatal("tras una prueba exitosa el motor tenía que volver a la rotación")
	}
}

func TestC4VencidoElCooldownEntraUnaSolaPrueba(t *testing.T) {
	rel := nuevoReloj()
	roto := &motorFalso{nombre: "roto", err: errors.New("500")}
	sano := &motorFalso{nombre: "sano", respuesta: "ok"}
	r := armarRouter(t, rel, 2, time.Minute,
		def{roto, config.TierPrivate}, def{sano, config.TierPrivate})

	for i := 0; i < 2; i++ {
		_, _ = r.Ask(context.Background(), "", "hola")
	}
	if !r.engines[0].breaker.open() {
		t.Fatal("el circuito tenía que estar abierto")
	}
	trasApertura := roto.veces()

	rel.avanzar(2 * time.Minute) // vence el cooldown

	// Diez llamadas concurrentes justo al vencer: sólo UNA puede probar el motor caído.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Ask(context.Background(), "", "hola")
		}()
	}
	wg.Wait()

	if pruebas := roto.veces() - trasApertura; pruebas != 1 {
		t.Fatalf("FUGA C4: se fueron %d pruebas contra un motor caído, tenía que ser exactamente 1", pruebas)
	}
}

func TestC4LaPruebaQueAndaCierraElCircuito(t *testing.T) {
	rel := nuevoReloj()
	m := &motorFalso{nombre: "m", err: errors.New("500")}
	r := armarRouter(t, rel, 2, time.Minute, def{m, config.TierPrivate})

	for i := 0; i < 2; i++ {
		_, _ = r.Ask(context.Background(), "", "hola")
	}
	rel.avanzar(2 * time.Minute)
	m.err = nil
	m.respuesta = "ok"
	if _, err := r.Ask(context.Background(), "", "hola"); err != nil {
		t.Fatalf("la prueba half-open tenía que andar: %v", err)
	}
	if r.engines[0].breaker.open() {
		t.Fatal("FUGA C4: la prueba anduvo y el circuito quedó abierto igual")
	}
	antes := m.veces()
	if _, err := r.Ask(context.Background(), "", "hola"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if m.veces() != antes+1 {
		t.Fatal("el motor no volvió a la rotación normal tras cerrarse")
	}
}

func TestC4LaPruebaQueFallaReabrePorUnCooldownCompleto(t *testing.T) {
	rel := nuevoReloj()
	m := &motorFalso{nombre: "m", err: errors.New("500")}
	r := armarRouter(t, rel, 2, time.Minute, def{m, config.TierPrivate})

	for i := 0; i < 2; i++ {
		_, _ = r.Ask(context.Background(), "", "hola")
	}
	rel.avanzar(2 * time.Minute)
	_, _ = r.Ask(context.Background(), "", "hola") // prueba, falla
	trasPrueba := m.veces()

	rel.avanzar(30 * time.Second) // menos que el cooldown
	_, _ = r.Ask(context.Background(), "", "hola")
	if m.veces() != trasPrueba {
		t.Fatal("FUGA C4: tras fallar la prueba el motor se reintentó antes de un cooldown completo")
	}
}

// La negativa por política durante una prueba half-open no puede dejar el motor colgado.
func TestC4NegarseEnLaPruebaNoDejaElMotorColgado(t *testing.T) {
	rel := nuevoReloj()
	gratis := &motorFalso{nombre: "groq", err: errors.New("500")}
	r := armarRouter(t, rel, 2, time.Minute, def{gratis, config.TierFree})

	for i := 0; i < 2; i++ {
		_, _ = r.Ask(context.Background(), "", "hola")
	}
	rel.avanzar(2 * time.Minute)

	// La prueba cae justo en un prompt con secretos: el motor se niega, no falla.
	_, _ = r.Ask(context.Background(), "", "clave "+tSecretoAWS)

	gratis.err = nil
	gratis.respuesta = "ok"
	antes := gratis.veces()
	if _, err := r.Ask(context.Background(), "", "limpio"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gratis.veces() != antes+1 {
		t.Fatal("el motor quedó fuera de la rotación por haberse negado durante la prueba half-open")
	}
}

// --- C5: agotamiento explícito --------------------------------------------------------------

func TestC5TodosCaidosDaErrorExplicito(t *testing.T) {
	a := &motorFalso{nombre: "a", err: errors.New("500")}
	b := &motorFalso{nombre: "b", err: errors.New("503")}
	r := armarRouter(t, nuevoReloj(), 3, time.Minute,
		def{a, config.TierPrivate}, def{b, config.TierPrivate})

	_, err := r.Ask(context.Background(), "", "hola")
	if !errors.Is(err, ErrAllEnginesDown) {
		t.Fatalf("esperaba ErrAllEnginesDown, hubo: %v", err)
	}
	// Caídos NO es lo mismo que negados: el caller decide distinto.
	if errors.Is(err, ErrSecretsBlocked) {
		t.Fatal("un agotamiento por fallas se está reportando como negativa de política")
	}
}

func TestC5ElContextoCanceladoNoRecorreLaFlota(t *testing.T) {
	a := &motorFalso{nombre: "a", err: context.Canceled}
	b := &motorFalso{nombre: "b", respuesta: "ok"}
	r := armarRouter(t, nuevoReloj(), 3, time.Minute,
		def{a, config.TierPrivate}, def{b, config.TierPrivate})

	if _, err := r.Ask(context.Background(), "", "hola"); !errors.Is(err, context.Canceled) {
		t.Fatalf("esperaba context.Canceled, hubo: %v", err)
	}
	if b.veces() != 0 {
		t.Fatal("con el contexto muerto no tenía sentido seguir probando la flota")
	}
}

// --- C6: bit-identidad sin flota ------------------------------------------------------------

func TestC6SinFlotaNoHayRouter(t *testing.T) {
	p, err := NewProvider(config.CognitionConfig{})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, esRouter := p.(*router); esRouter {
		t.Fatal("sin flota no tenía que instanciarse un router")
	}
	if _, esNoop := p.(NoopProvider); !esNoop {
		t.Fatalf("el camino model-free cambió: %T", p)
	}

	// Y con motor único sigue saliendo el guarded de F1, no un router.
	p2, err := NewProvider(config.CognitionConfig{Provider: "openai-compat", Endpoint: "http://x/v1"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, esRouter := p2.(*router); esRouter {
		t.Fatal("con motor único no tenía que instanciarse un router")
	}
	// Desde F3 el motor viene además envuelto en el caché, que va POR FUERA del portero. Se
	// desenvuelve en vez de aflojar la aserción: lo que este invariante protege —que el motor
	// único NUNCA salga sin portero (garantía de F1)— sigue verificándose igual de fuerte.
	if _, esCache := p2.(*cached); !esCache {
		t.Fatalf("con el caché encendido por default, el motor único debía venir cacheado: %T", p2)
	}
	if _, esGuarded := unwrapCache(p2).(guarded); !esGuarded {
		t.Fatalf("el motor único dejó de venir envuelto por el portero: %T", unwrapCache(p2))
	}
}

func TestFlotaConMotorInvalidoFallaExplicito(t *testing.T) {
	casos := map[string]config.CognitionConfig{
		"tier desconocido": {Fleet: []config.FleetEngineConfig{
			{Provider: "openai-compat", Endpoint: "http://x/v1", Tier: "barato"}}},
		"motor none": {Fleet: []config.FleetEngineConfig{{Provider: "none"}}},
		"provider desconocido": {Fleet: []config.FleetEngineConfig{
			{Provider: "gpt-magico"}}},
		"modo de portero inválido": {Fleet: []config.FleetEngineConfig{
			{Provider: "openai-compat", Endpoint: "http://x/v1",
				Gateway: config.GatewayConfig{Mode: "scub"}}}},
	}
	for nombre, cfg := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := NewProvider(cfg); err == nil {
				t.Fatal("una flota mal configurada tenía que fallar explícito")
			}
		})
	}
}

// --- C7: seguro entre goroutines (correr con -race) -----------------------------------------

func TestC7ConcurrenciaSinCarreras(t *testing.T) {
	rel := nuevoReloj()
	roto := &motorFalso{nombre: "roto", err: errors.New("500")}
	sano := &motorFalso{nombre: "sano", respuesta: "ok"}
	r := armarRouter(t, rel, 3, time.Minute,
		def{roto, config.TierPrivate}, def{sano, config.TierPrivate})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			texto := "consulta limpia"
			if i%3 == 0 {
				texto = "clave " + tSecretoAWS
			}
			if _, err := r.Ask(context.Background(), "", texto); err != nil {
				t.Errorf("Ask concurrente: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// OJO con lo que se afirma acá. En estado CERRADO el breaker no reserva turno —serializar las
	// llamadas mataría el throughput—, así que muchas goroutines pueden entrar antes de que alguna
	// alcance a llevar el contador al umbral. Que los intentos superen `failures` NO es un bug: es
	// cómo funciona un circuit breaker, que acota los intentos NUEVOS, no los que ya están en vuelo.
	//
	// Lo que sí tiene que valer, y es lo que se prueba:
	//   1. que no haya carreras (lo verifica -race, que corre en CI);
	//   2. que el circuito EFECTIVAMENTE abra y deje de admitir intentos nuevos;
	//   3. que el conteo no se pierda por el camino.
	if !r.engines[0].breaker.open() {
		t.Fatal("tras la tormenta de fallas el circuito tenía que quedar abierto")
	}
	trasTormenta := roto.veces()
	for i := 0; i < 10; i++ {
		if _, err := r.Ask(context.Background(), "", "limpio"); err != nil {
			t.Fatalf("Ask post-tormenta: %v", err)
		}
	}
	if roto.veces() != trasTormenta {
		t.Fatalf("FUGA C7: el circuito abierto siguió admitiendo intentos (%d → %d)", trasTormenta, roto.veces())
	}
	if sano.veces() < 10 {
		t.Fatalf("el motor sano tenía que absorber las 10 llamadas, atendió %d", sano.veces())
	}
}

// --- Diagnóstico de la flota ----------------------------------------------------------------

func cfgFlota(engines ...config.FleetEngineConfig) config.CognitionConfig {
	return config.CognitionConfig{Fleet: engines}
}

func TestInspectFleetEstados(t *testing.T) {
	gratis := config.FleetEngineConfig{Name: "groq", Provider: "openai-compat", Endpoint: "http://x/v1", Model: "m"}
	privado := config.FleetEngineConfig{Name: "max", Provider: "openai-compat", Endpoint: "http://y/v1", Model: "m", Tier: config.TierPrivate}

	gratisTapando := gratis
	gratisTapando.Gateway = config.GatewayConfig{Mode: GatewayModeScrub}
	gratisApagado := gratis
	gratisApagado.Gateway = config.GatewayConfig{Mode: GatewayModeOff}

	casos := []struct {
		nombre string
		cfg    config.CognitionConfig
		quiero string
	}{
		{"flota sana", cfgFlota(gratis, privado), "ok"},
		{"scrub sobre tier gratis", cfgFlota(gratisTapando, privado), "warning"},
		{"portero apagado en un motor", cfgFlota(gratisApagado, privado), "error"},
		{"flota que no arranca", cfgFlota(config.FleetEngineConfig{Provider: "gpt-magico"}), "error"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := InspectGateway(c.cfg); got.Status != c.quiero {
				t.Fatalf("estado %q, esperaba %q — mensaje: %s", got.Status, c.quiero, got.Message)
			}
		})
	}
}

func TestInspectFleetNoDiceQueElPilarEstaApagado(t *testing.T) {
	// Con flota, el motor único NO está configurado. Si el diagnóstico mirara ese camino diría
	// "pilar apagado" mientras hay motores atendiendo.
	st := InspectGateway(cfgFlota(config.FleetEngineConfig{
		Provider: "openai-compat", Endpoint: "http://x/v1", Model: "m", Tier: config.TierPrivate}))
	if st.Status != "ok" || strings.Contains(st.Message, "apagado") {
		t.Fatalf("el diagnóstico ignoró la flota: [%s] %s", st.Status, st.Message)
	}
}
