package cognition

import (
	"context"
	"errors"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/privacy"
)

// spy es un motor falso que CAPTURA lo que le llega. Es el instrumento del invariante R0: lo que
// este espía ve es, literalmente, lo que habría cruzado la red hacia el LLM.
type spy struct {
	gotSystem string
	gotUser   string
	llamado   bool
	answer    string
	err       error
}

func (s *spy) Name() string { return "spy" }

func (s *spy) Ask(_ context.Context, system, user string) (string, error) {
	s.llamado = true
	s.gotSystem, s.gotUser = system, user
	return s.answer, s.err
}

const (
	tSecretoAWS = "AKIA1234567890ABCDEF"
	tSecretoGH  = "ghp_aBcDeFgHiJkLmNoPqRsT1234"
)

// --- R0: ningún secreto cruza la frontera --------------------------------------------------

func TestR0ElSecretoNuncaLlegaAlMotor(t *testing.T) {
	sp := &spy{answer: "listo"}
	g, err := newGuarded(sp, GatewayModeScrub)
	if err != nil {
		t.Fatalf("newGuarded: %v", err)
	}

	system := "sos un asistente; la clave de infra es " + tSecretoAWS
	user := "revisá el token " + tSecretoGH + " y decime si sirve"

	if _, err := g.Ask(context.Background(), system, user); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !sp.llamado {
		t.Fatal("el motor no fue invocado")
	}
	for _, campo := range []struct{ nombre, val string }{
		{"system", sp.gotSystem}, {"user", sp.gotUser},
	} {
		for _, sec := range []string{tSecretoAWS, tSecretoGH} {
			if strings.Contains(campo.val, sec) {
				t.Fatalf("FUGA R0: el secreto %q llegó al motor en %s:\n%s", sec, campo.nombre, campo.val)
			}
		}
	}
	// Y sin embargo el motor tiene que haber recibido algo con sentido, no un texto vacío.
	if !strings.Contains(sp.gotSystem, "asistente") || !strings.Contains(sp.gotUser, "decime si sirve") {
		t.Fatalf("el portero rompió el prompt:\n system=%q\n user=%q", sp.gotSystem, sp.gotUser)
	}
}

func TestR0LaRespuestaVuelveRehidratada(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, GatewayModeScrub)

	// El motor "razona" y devuelve el marcador que recibió: el caller tiene que ver el valor real.
	sp.answer = "" // se completa abajo, cuando sepamos qué marcador recibió
	user := "la clave es " + tSecretoAWS

	// Primera pasada para descubrir el marcador que le tocó.
	sp2 := &spy{}
	g2, _ := newGuarded(sp2, GatewayModeScrub)
	_, _ = g2.Ask(context.Background(), "", user)
	marcador := sp2.gotUser[strings.Index(sp2.gotUser, "[[MSB:"):]

	sp.answer = "usá " + marcador + " en el header"
	got, err := g.Ask(context.Background(), "", user)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "usá "+tSecretoAWS+" en el header" {
		t.Fatalf("la respuesta no se rehidrató:\n got %q", got)
	}
}

// --- R4: falla cerrado (modo refuse) -------------------------------------------------------

func TestR4RefuseNoEnviaNada(t *testing.T) {
	sp := &spy{answer: "no deberías ver esto"}
	g, err := newGuarded(sp, GatewayModeRefuse)
	if err != nil {
		t.Fatalf("newGuarded: %v", err)
	}

	_, err = g.Ask(context.Background(), "", "la clave es "+tSecretoAWS)
	if !errors.Is(err, ErrSecretsBlocked) {
		t.Fatalf("esperaba ErrSecretsBlocked, hubo: %v", err)
	}
	if sp.llamado {
		t.Fatal("FUGA R4: en modo refuse el motor NO tenía que ser invocado, y lo fue")
	}
}

func TestR4RefuseDejaPasarLoLimpio(t *testing.T) {
	sp := &spy{answer: "ok"}
	g, _ := newGuarded(sp, GatewayModeRefuse)

	got, err := g.Ask(context.Background(), "", "una pregunta sin ningún secreto adentro")
	if err != nil {
		t.Fatalf("refuse bloqueó un texto limpio: %v", err)
	}
	if !sp.llamado || got != "ok" {
		t.Fatalf("el texto limpio no llegó al motor (llamado=%v got=%q)", sp.llamado, got)
	}
}

// --- R6: bit-identidad del camino model-free -----------------------------------------------

func TestR6NoopNoSeEnvuelve(t *testing.T) {
	p, err := NewProvider(config.CognitionConfig{}) // sin provider ⇒ Noop
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, esNoop := p.(NoopProvider); !esNoop {
		t.Fatalf("el camino model-free quedó envuelto: %T", p)
	}
	if _, err := p.Ask(context.Background(), "", ""); !errors.Is(err, ErrCognitionDisabled) {
		t.Fatalf("el Noop dejó de comportarse como antes: %v", err)
	}
}

func TestR6NoopIgnoraElModoDeGateway(t *testing.T) {
	// Incluso con un modo inválido, sin motor no hay nada que validar ni que romper.
	for _, modo := range []string{"", "scrub", "refuse", "off", "modo-que-no-existe"} {
		cfg := config.CognitionConfig{Gateway: config.GatewayConfig{Mode: modo}}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("modo %q rompió el camino model-free: %v", modo, err)
		}
		if _, esNoop := p.(NoopProvider); !esNoop {
			t.Fatalf("modo %q envolvió al Noop: %T", modo, p)
		}
	}
}

// --- R7: `off` es explícito, y un modo desconocido rompe el arranque -----------------------

func TestR7ModoOffDevuelveElMotorDesnudo(t *testing.T) {
	sp := &spy{answer: "ok"}
	g, err := newGuarded(sp, GatewayModeOff)
	if err != nil {
		t.Fatalf("newGuarded(off): %v", err)
	}
	if g != Provider(sp) {
		t.Fatalf("modo off tenía que devolver el motor tal cual, devolvió %T", g)
	}
}

func TestR7ModoDesconocidoEsErrorDeArranque(t *testing.T) {
	sp := &spy{}
	if _, err := newGuarded(sp, "scub"); err == nil { // typo de "scrub"
		t.Fatal("un modo mal escrito tenía que romper el arranque, y pasó en silencio")
	}

	cfg := config.CognitionConfig{
		Provider: "openai-compat",
		Endpoint: "http://127.0.0.1:4000/v1",
		Gateway:  config.GatewayConfig{Mode: "protegido"},
	}
	if _, err := NewProvider(cfg); err == nil {
		t.Fatal("NewProvider aceptó un gateway.mode desconocido con un motor real")
	}
}

func TestDefaultDeConfigEsScrub(t *testing.T) {
	// El default (Mode vacío) tiene que PROTEGER, no pasar de largo.
	sp := &spy{answer: "ok"}
	g, err := newGuarded(sp, "")
	if err != nil {
		t.Fatalf("newGuarded(\"\"): %v", err)
	}
	gu, ok := g.(guarded)
	if !ok {
		t.Fatalf("el default no envolvió el motor: %T", g)
	}
	if gu.mode != GatewayModeScrub {
		t.Fatalf("el default tenía que ser %q, fue %q", GatewayModeScrub, gu.mode)
	}
}

// --- Comportamiento del envoltorio ---------------------------------------------------------

func TestNameDelegaEnElMotorReal(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, GatewayModeScrub)
	if g.Name() != "spy" {
		t.Fatalf("Name() tenía que delegar en el motor (la procedencia nombra al modelo), dio %q", g.Name())
	}
}

func TestErrorDelMotorSePropagaSinRehidratar(t *testing.T) {
	fallo := errors.New("el backend explotó")
	sp := &spy{err: fallo, answer: "respuesta que no hay que usar"}
	g, _ := newGuarded(sp, GatewayModeScrub)

	got, err := g.Ask(context.Background(), "", "clave "+tSecretoAWS)
	if !errors.Is(err, fallo) {
		t.Fatalf("el error del motor no se propagó: %v", err)
	}
	if got != "" {
		t.Fatalf("ante un error tenía que devolver vacío, devolvió %q", got)
	}
}

// --- R4: el pánico se ataja, y ataja CERRADO ------------------------------------------------

// sesionQueExplota simula una falla patológica adentro de la redacción (un offset imposible, una
// entrada que rompe un invariante interno). Es la única forma honesta de probar el recover: sin
// esto, la red de seguridad nunca se ve atajar nada.
type sesionQueExplota struct{}

func (sesionQueExplota) Scrub(string) string     { panic("offset patológico en la redacción") }
func (sesionQueExplota) Restore(t string) string { return t }
func (sesionQueExplota) Count() int              { return 0 }
func (sesionQueExplota) Types() []string         { return nil }

func TestR4PanicoEnElPorteroNoTumbaNiFiltra(t *testing.T) {
	sp := &spy{answer: "no deberías ver esto"}
	g := guarded{
		inner:      sp,
		mode:       GatewayModeScrub,
		newSession: func() scrubSession { return sesionQueExplota{} },
	}

	// Si el recover no estuviera, este Ask tumbaría el proceso de test entero: el modo de falla
	// que se está previniendo es exactamente ese, con el daemon MCP en lugar del test.
	got, err := g.Ask(context.Background(), "sos un asistente", "la clave es "+tSecretoAWS)

	if !errors.Is(err, ErrGatewayFailed) {
		t.Fatalf("esperaba ErrGatewayFailed tras el pánico, hubo: %v", err)
	}
	if sp.llamado {
		t.Fatal("FUGA R4: tras el pánico el motor NO tenía que ser invocado, y lo fue")
	}
	if got != "" {
		t.Fatalf("tras el pánico tenía que devolver vacío, devolvió %q", got)
	}
	// La falla TÉCNICA no puede confundirse con la de POLÍTICA: el caller decide distinto en cada
	// caso (degradar a model-free vs. no reintentar contra el mismo motor).
	if errors.Is(err, ErrSecretsBlocked) {
		t.Fatal("un fallo técnico del portero se está reportando como bloqueo por política")
	}
}

func TestSesionRealEsLaQueSeUsaPorDefecto(t *testing.T) {
	// El campo newSession es un seam de test: si por descuido quedara como el camino de producción,
	// el portero dejaría de usar privacy.Session y todos los tests de arriba probarían otra cosa.
	g := guarded{inner: &spy{}, mode: GatewayModeScrub}
	if _, esReal := g.session().(*privacy.Session); !esReal {
		t.Fatalf("el camino por defecto no usa privacy.Session: %T", g.session())
	}
}

// --- E6: los modos son los mismos que los de config -----------------------------------------

func TestE6LosModosNoSePuedenDespegarDeConfig(t *testing.T) {
	// Si estas constantes o esta validación se despegaran de config, la misma palabra en el yaml
	// significaría cosas distintas para la cognición y para los embeddings.
	if GatewayModeScrub != config.GatewayModeScrub ||
		GatewayModeRefuse != config.GatewayModeRefuse ||
		GatewayModeOff != config.GatewayModeOff {
		t.Fatal("las constantes de modo de cognition se despegaron de las de config")
	}
	// Enumerar unos pocos modos NO alcanza: una divergencia se cuela por cualquier palabra que la
	// lista no contenga (lo comprobé saboteando — con una lista corta el sabotaje pasó limpio).
	// Se prueba contra un corpus: palabras que un dev podría elegir de verdad, más ruido
	// pseudoaleatorio determinista (LCG con semilla fija, así el test es reproducible en CI).
	corpus := []string{
		"", "scrub", "refuse", "off", "on", "auto", "none", "disabled", "enabled", "silencioso",
		"quiet", "silent", "strict", "safe", "seguro", "apagado", "prendido", "true", "false", "0",
		"1", "SCRUB", "Off", "scub", "refus", "scrub ", " off", "scrub,refuse", "default",
	}
	seed := uint32(20260804)
	for i := 0; i < 400; i++ {
		seed = seed*1664525 + 1013904223
		n := int(seed>>28) + 1
		b := make([]byte, n)
		for j := range b {
			seed = seed*1664525 + 1013904223
			b[j] = byte('a' + seed%26)
		}
		corpus = append(corpus, string(b))
	}

	for _, m := range corpus {
		valCfg, errCfg := config.NormalizeGatewayMode(m)
		valCog, errCog := normalizeGatewayMode(m)
		if (errCfg == nil) != (errCog == nil) {
			t.Fatalf("el modo %q lo acepta uno y lo rechaza el otro (config=%v, cognition=%v)", m, errCfg, errCog)
		}
		if errCfg == nil && valCfg != valCog {
			t.Fatalf("el modo %q se resuelve distinto: config=%q, cognition=%q", m, valCfg, valCog)
		}
	}
}

// --- Diagnóstico: InspectGateway (musubi doctor) --------------------------------------------

func cfgMotorReal(modo string) config.CognitionConfig {
	return config.CognitionConfig{
		Provider: "openai-compat",
		Endpoint: "http://127.0.0.1:4000/v1",
		Gateway:  config.GatewayConfig{Mode: modo},
	}
}

func TestInspectGatewayEstados(t *testing.T) {
	casos := []struct {
		nombre string
		cfg    config.CognitionConfig
		quiero string
	}{
		{"pilar apagado", config.CognitionConfig{}, "ok"},
		{"pilar apagado con modo mal escrito", config.CognitionConfig{
			Gateway: config.GatewayConfig{Mode: "scub"}}, "warning"},
		{"motor real, default", cfgMotorReal(""), "ok"},
		{"motor real, scrub", cfgMotorReal(GatewayModeScrub), "ok"},
		{"motor real, refuse", cfgMotorReal(GatewayModeRefuse), "ok"},
		{"motor real, off", cfgMotorReal(GatewayModeOff), "error"},
		{"motor real, modo desconocido", cfgMotorReal("protegido"), "error"},
		{"provider desconocido", config.CognitionConfig{Provider: "gpt-magico"}, "error"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := InspectGateway(c.cfg); got.Status != c.quiero {
				t.Fatalf("estado %q, esperaba %q — mensaje: %s", got.Status, c.quiero, got.Message)
			}
		})
	}
}

func TestInspectGatewayOffGritaFuerte(t *testing.T) {
	// El mensaje es el producto: si no dice qué pasa y cómo revertirlo, el check rojo no sirve.
	st := InspectGateway(cfgMotorReal(GatewayModeOff))
	for _, quiero := range []string{"DESACTIVADO", "SIN TAPAR", "cognition.gateway.mode"} {
		if !strings.Contains(st.Message, quiero) {
			t.Fatalf("el aviso de portero apagado no menciona %q: %s", quiero, st.Message)
		}
	}
}

func TestInspectGatewayNoLeMienteAlConstructor(t *testing.T) {
	// El diagnóstico y el constructor tienen que contar la MISMA historia. Es la forma de que
	// agregar un modo nuevo no deje al doctor informando un estado que ya no existe.
	for _, modo := range []string{"", GatewayModeScrub, GatewayModeRefuse, GatewayModeOff, "modo-inventado"} {
		cfg := cfgMotorReal(modo)
		diceProtegido := strings.Contains(InspectGateway(cfg).Message, "portero activo")

		p, err := NewProvider(cfg)
		envuelto := false
		if err == nil {
			_, envuelto = p.(guarded)
		}

		if diceProtegido != envuelto {
			t.Fatalf("modo %q: el doctor dice protegido=%v pero el constructor envolvió=%v",
				modo, diceProtegido, envuelto)
		}
	}
}

func TestSesionesNoSeMezclanEntreLlamadas(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, GatewayModeScrub)

	// Llamada 1 acuña un marcador para el secreto AWS.
	_, _ = g.Ask(context.Background(), "", "clave "+tSecretoAWS)
	marcador := sp.gotUser[strings.Index(sp.gotUser, "[[MSB:"):]

	// Llamada 2 no tiene ese secreto. Si el modelo devuelve el marcador de la llamada anterior,
	// NO tiene que resolverse: el mapeo murió con la llamada 1.
	sp.answer = "eco de " + marcador
	got, err := g.Ask(context.Background(), "", "pregunta limpia")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if strings.Contains(got, tSecretoAWS) {
		t.Fatalf("FUGA: el mapeo se filtró entre llamadas, la respuesta trajo el secreto: %q", got)
	}
}
