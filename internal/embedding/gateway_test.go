package embedding

import (
	"context"
	"errors"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/redact"
)

// spy es un embedder falso que CAPTURA el texto que le llega. Es el instrumento del invariante E0:
// lo que este espía ve es, literalmente, lo que habría cruzado la red.
type spy struct {
	got     []string
	llamado bool
	err     error
}

func (s *spy) Embed(_ context.Context, text string) ([]float32, error) {
	s.llamado = true
	s.got = append(s.got, text)
	return []float32{0.1, 0.2}, s.err
}

func (s *spy) Dimensions() int { return 2 }
func (s *spy) Name() string    { return "spy" }

const (
	tSecretoAWS = "AKIA1234567890ABCDEF"
	tSecretoGH  = "ghp_aBcDeFgHiJkLmNoPqRsT1234"
)

func cfgConRed(modo string) config.EmbeddingConfig {
	return config.EmbeddingConfig{
		Provider: "openai",
		Model:    "text-embedding-3-small",
		BaseURL:  "https://api.openai.com/v1",
		Gateway:  config.GatewayConfig{Mode: modo},
	}
}

// --- E0: ningún secreto cruza hacia el embedder ---------------------------------------------

func TestE0ElSecretoNuncaLlegaAlEmbedder(t *testing.T) {
	sp := &spy{}
	g, err := newGuarded(sp, config.GatewayModeScrub)
	if err != nil {
		t.Fatalf("newGuarded: %v", err)
	}

	texto := "la clave de infra es " + tSecretoAWS + " y el token " + tSecretoGH + " sigue vivo"
	if _, err := g.Embed(context.Background(), texto); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if !sp.llamado {
		t.Fatal("el embedder no fue invocado")
	}
	for _, sec := range []string{tSecretoAWS, tSecretoGH} {
		if strings.Contains(sp.got[0], sec) {
			t.Fatalf("FUGA E0: el secreto %q llegó al embedder:\n%s", sec, sp.got[0])
		}
	}
	// Y el resto del texto tiene que seguir ahí: tapar no es destruir.
	if !strings.Contains(sp.got[0], "la clave de infra es") || !strings.Contains(sp.got[0], "sigue vivo") {
		t.Fatalf("el portero rompió el texto: %q", sp.got[0])
	}
}

// La ruta de QUERY es la que hoy no tiene ninguna protección en ninguna configuración: el texto de
// una consulta nunca pasa por la redacción del camino de guardado.
func TestE0LaQueryTampocoFiltra(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, config.GatewayModeScrub)

	query := "¿qué era el token " + tSecretoGH + " que usamos en el deploy?"
	if _, err := g.Embed(context.Background(), query); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if strings.Contains(sp.got[0], tSecretoGH) {
		t.Fatalf("FUGA E0: la query salió con el secreto adentro:\n%s", sp.got[0])
	}
}

func TestTextoLimpioNoSeAltera(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, config.GatewayModeScrub)

	limpio := "el recall usa RRF sobre FTS5 y el índice vectorial IVF"
	if _, err := g.Embed(context.Background(), limpio); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if sp.got[0] != limpio {
		t.Fatalf("el portero tocó un texto sin secretos:\n want %q\n got  %q", limpio, sp.got[0])
	}
}

// --- E1: determinismo -----------------------------------------------------------------------

func TestE1MismoTextoMismoTapado(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, config.GatewayModeScrub)

	texto := "clave " + tSecretoAWS + " y de nuevo " + tSecretoAWS + ", más " + tSecretoGH
	for i := 0; i < 3; i++ {
		if _, err := g.Embed(context.Background(), texto); err != nil {
			t.Fatalf("Embed #%d: %v", i, err)
		}
	}
	for i := 1; i < len(sp.got); i++ {
		if sp.got[i] != sp.got[0] {
			t.Fatalf("FUGA E1: el mismo texto dio tapados distintos, el vector no sería estable:\n #0 %q\n #%d %q",
				sp.got[0], i, sp.got[i])
		}
	}
}

// --- E2: coherencia índice↔consulta ---------------------------------------------------------

// El invariante es estructural: si NewProvider devuelve SIEMPRE un envuelto para los providers con
// red, entonces las rutas que indexan y las que consultan usan el mismo objeto y ven la misma
// transformación. Lo que hay que impedir es que exista una forma de obtener uno desnudo.
func TestE2NoHayFormaDeObtenerUnProviderConRedSinPortero(t *testing.T) {
	for _, prov := range []string{"ollama", "openai", "openai-compatible"} {
		cfg := cfgConRed("")
		cfg.Provider = prov
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("NewProvider(%q): %v", prov, err)
		}
		if _, envuelto := p.(guarded); !envuelto {
			t.Fatalf("FUGA E2: %q sale de la fábrica SIN portero (%T); el índice y la query podrían divergir", prov, p)
		}
	}
}

// --- E3: falla cerrado ----------------------------------------------------------------------

func TestE3RefuseNoEmbebeNada(t *testing.T) {
	sp := &spy{}
	g, err := newGuarded(sp, config.GatewayModeRefuse)
	if err != nil {
		t.Fatalf("newGuarded: %v", err)
	}
	if _, err := g.Embed(context.Background(), "clave "+tSecretoAWS); !errors.Is(err, ErrSecretsBlocked) {
		t.Fatalf("esperaba ErrSecretsBlocked, hubo: %v", err)
	}
	if sp.llamado {
		t.Fatal("FUGA E3: en modo refuse el embedder NO tenía que ser invocado, y lo fue")
	}
}

func TestE3RefuseDejaPasarLoLimpio(t *testing.T) {
	sp := &spy{}
	g, _ := newGuarded(sp, config.GatewayModeRefuse)
	if _, err := g.Embed(context.Background(), "una consulta sin ningún secreto"); err != nil {
		t.Fatalf("refuse bloqueó un texto limpio: %v", err)
	}
	if !sp.llamado {
		t.Fatal("el texto limpio no llegó al embedder")
	}
}

func TestE3PanicoNoTumbaNiFiltra(t *testing.T) {
	sp := &spy{}
	g := guarded{
		inner: sp,
		mode:  config.GatewayModeScrub,
		scrub: func(string) (string, []redact.Finding) { panic("offset patológico en la redacción") },
	}

	// Sin el recover, este Embed tumbaría el proceso de test entero: es el modo de falla que se
	// está previniendo, con el daemon MCP en lugar del test.
	vec, err := g.Embed(context.Background(), "clave "+tSecretoAWS)
	if !errors.Is(err, ErrGatewayFailed) {
		t.Fatalf("esperaba ErrGatewayFailed tras el pánico, hubo: %v", err)
	}
	if sp.llamado {
		t.Fatal("FUGA E3: tras el pánico el embedder NO tenía que ser invocado, y lo fue")
	}
	if vec != nil {
		t.Fatalf("tras el pánico tenía que devolver nil, devolvió %v", vec)
	}
	if errors.Is(err, ErrSecretsBlocked) {
		t.Fatal("un fallo técnico del portero se está reportando como bloqueo por política")
	}
}

func TestScrubRealEsElQueSeUsaPorDefecto(t *testing.T) {
	// El campo scrub es un seam de test: si se comiera el camino de producción, el portero dejaría
	// de tapar y todos los tests de arriba probarían otra cosa.
	sp := &spy{}
	g := guarded{inner: sp, mode: config.GatewayModeScrub}
	if _, err := g.Embed(context.Background(), "clave "+tSecretoAWS); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if strings.Contains(sp.got[0], tSecretoAWS) {
		t.Fatalf("FUGA E0: el camino por defecto no tapa: %q", sp.got[0])
	}
}

func TestErrorDelProveedorSePropaga(t *testing.T) {
	fallo := errors.New("el backend explotó")
	sp := &spy{err: fallo}
	g, _ := newGuarded(sp, config.GatewayModeScrub)
	if _, err := g.Embed(context.Background(), "texto"); !errors.Is(err, fallo) {
		t.Fatalf("el error del proveedor no se propagó: %v", err)
	}
}

// --- E4: bit-identidad de los proveedores sin red -------------------------------------------

func TestE4SinRedNoSeEnvuelve(t *testing.T) {
	// none ⇒ Noop, y ni siquiera un modo inválido puede romperlo: sin socket no hay nada que validar.
	for _, modo := range []string{"", "scrub", "refuse", "off", "modo-que-no-existe"} {
		p, err := NewProvider(config.EmbeddingConfig{Gateway: config.GatewayConfig{Mode: modo}})
		if err != nil {
			t.Fatalf("modo %q rompió el camino sin red: %v", modo, err)
		}
		if _, esNoop := p.(NoopProvider); !esNoop {
			t.Fatalf("modo %q envolvió al Noop: %T", modo, p)
		}
	}
}

func TestE4StaticNoSeEnvuelve(t *testing.T) {
	// StaticProvider es una tabla en proceso: no abre un socket, así que no hay frontera que cuidar.
	if needsGateway(&StaticProvider{}) {
		t.Fatal("el provider static quedaría envuelto: no manda texto a ningún lado")
	}
	if needsGateway(NoopProvider{}) {
		t.Fatal("el NoopProvider quedaría envuelto")
	}
	// Y lo contrario: todo lo que sí habla por un socket tiene que quedar adentro.
	for _, p := range []Provider{&OllamaProvider{}, &OpenAIProvider{}} {
		if !needsGateway(p) {
			t.Fatalf("%T manda texto por la red y quedó SIN portero", p)
		}
	}
}

// --- E5: `off` explícito, modo desconocido apaga -------------------------------------------

func TestE5OffDevuelveElProveedorDesnudo(t *testing.T) {
	sp := &spy{}
	g, err := newGuarded(sp, config.GatewayModeOff)
	if err != nil {
		t.Fatalf("newGuarded(off): %v", err)
	}
	if g != Provider(sp) {
		t.Fatalf("modo off tenía que devolver el proveedor tal cual, devolvió %T", g)
	}
}

func TestE5ModoDesconocidoApagaLaSemantica(t *testing.T) {
	if _, err := newGuarded(&spy{}, "scub"); err == nil { // typo de "scrub"
		t.Fatal("un modo mal escrito tenía que fallar, y pasó en silencio")
	}
	if _, err := NewProvider(cfgConRed("protegido")); err == nil {
		t.Fatal("NewProvider aceptó un gateway.mode desconocido con un embedder de red")
	}
}

func TestDefaultEsScrub(t *testing.T) {
	g, err := newGuarded(&spy{}, "")
	if err != nil {
		t.Fatalf("newGuarded(\"\"): %v", err)
	}
	gu, ok := g.(guarded)
	if !ok {
		t.Fatalf("el default no envolvió el proveedor: %T", g)
	}
	if gu.mode != config.GatewayModeScrub {
		t.Fatalf("el default tenía que ser %q, fue %q", config.GatewayModeScrub, gu.mode)
	}
}

// --- Diagnóstico (musubi doctor) ------------------------------------------------------------

func TestInspectGatewayEstados(t *testing.T) {
	casos := []struct {
		nombre string
		cfg    config.EmbeddingConfig
		quiero string
	}{
		{"sin embedder", config.EmbeddingConfig{}, "ok"},
		{"sin embedder, modo mal escrito", config.EmbeddingConfig{
			Gateway: config.GatewayConfig{Mode: "scub"}}, "warning"},
		{"con red, default", cfgConRed(""), "ok"},
		{"con red, scrub", cfgConRed(config.GatewayModeScrub), "ok"},
		{"con red, refuse", cfgConRed(config.GatewayModeRefuse), "ok"},
		{"con red, off", cfgConRed(config.GatewayModeOff), "error"},
		{"con red, modo desconocido", cfgConRed("protegido"), "error"},
		{"provider desconocido", config.EmbeddingConfig{Provider: "vectores-magicos"}, "error"},
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
	st := InspectGateway(cfgConRed(config.GatewayModeOff))
	for _, quiero := range []string{"DESACTIVADO", "SIN TAPAR", "embedding.gateway.mode"} {
		if !strings.Contains(st.Message, quiero) {
			t.Fatalf("el aviso de portero apagado no menciona %q: %s", quiero, st.Message)
		}
	}
}

func TestInspectGatewayNoLeMienteAlConstructor(t *testing.T) {
	for _, modo := range []string{"", config.GatewayModeScrub, config.GatewayModeRefuse, config.GatewayModeOff, "inventado"} {
		cfg := cfgConRed(modo)
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

// --- E6: una sola fuente de verdad sobre los modos ------------------------------------------

func TestE6LosModosSonLosMismosQueLosDeConfig(t *testing.T) {
	// Si estas constantes se despegaran de config, cada pilar entendería cosas distintas por la
	// misma palabra en el yaml.
	validos := []string{"", config.GatewayModeScrub, config.GatewayModeRefuse, config.GatewayModeOff}
	for _, m := range validos {
		if _, err := config.NormalizeGatewayMode(m); err != nil {
			t.Fatalf("el modo %q tenía que ser válido: %v", m, err)
		}
	}
	for _, m := range []string{"scub", "SCRUB", "ninguno", "true"} {
		if _, err := config.NormalizeGatewayMode(m); err == nil {
			t.Fatalf("el modo %q tenía que ser rechazado", m)
		}
	}
}
