package mcp

// servicios_oraculo_test.go custodia los tres agujeros que encontró la verificación adversarial
// y que ninguna prueba del slice había visto: dos formas de respuesta distinguibles, un secreto
// que llegaba al log, y un techo que se desactivaba solo.

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// TestElFiltroPorMaquinaNoDelataSiLaMaquinaExiste — el oráculo, cerrado.
//
// Preguntar por una máquina que existe pero no podés ver, y por una que no existe, tiene que
// devolver EXACTAMENTE lo mismo. Si difieren en un solo campo, probando nombres se mapea la
// flota del vecino sin tener permiso sobre ninguna.
//
// Sabotaje que la hace fallar: devolver `res["sin_permiso"] = sinPermiso` también cuando hay
// filtro (o sea, borrar el `if conFiltro { return res }` de respuestaDeServicios).
func TestElFiltroPorMaquinaNoDelataSiLaMaquinaExiste(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// bob ve SÓLO "nas". "pc-gio" existe en su mismo proyecto y no la puede ver.
	bob := principalDeFlota("bob", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"nas"}})
	sembrarServicios(t, s, "pc-gio", "postgres", "redis")
	sembrarServicios(t, s, "nas", "samba")

	existeYNoLaVeo := listarServicios(t, s, bob, "pc-gio")
	noExiste := listarServicios(t, s, bob, "no-existe-en-ningun-lado")

	if existeYNoLaVeo != noExiste {
		t.Errorf("las dos respuestas son distinguibles, así que el filtro dice si la máquina existe:\n  existe y no la veo: %s\n  no existe:          %s", existeYNoLaVeo, noExiste)
	}
	// Y la guarda de la guarda: si el escenario no tuviera servicios ocultos, las dos respuestas
	// coincidirían por casualidad y la prueba pasaría sin probar nada.
	sinFiltro := listarServicios(t, s, bob, "")
	if !strings.Contains(sinFiltro, "sin_permiso") {
		t.Fatal("el escenario no dejó servicios ocultos: las dos respuestas de arriba coincidirían por vacías y esta prueba sería decorativa")
	}
}

// TestElContadorSigueSaliendoCuandoNadieFiltro — la contraparte, para que el arreglo de arriba
// no se convierta en «nunca se avisa nada».
//
// Sabotaje que la hace fallar: devolver siempre `res` sin los contadores en respuestaDeServicios.
func TestElContadorSigueSaliendoCuandoNadieFiltro(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	bob := principalDeFlota("bob", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"nas"}})
	sembrarServicios(t, s, "pc-gio", "postgres", "redis")
	sembrarServicios(t, s, "nas", "samba")

	res := listarServicios(t, s, bob, "")
	if !strings.Contains(res, `"sin_permiso":2`) {
		t.Errorf("sin filtro, el listado no dice cuántos quedaron afuera: una lista corta sin explicación se lee como «no hay más servicios». Respuesta: %s", res)
	}
}

// TestElQueryStringDelDestinoNoLlegaAlLog.
//
// La guarda de userinfo cubría `https://prom:clave@host/...`. No cubría el token como parámetro,
// que es como lo piden varios receptores OTLP reales — y ése salía entero en la primera línea
// del journal del cerebro.
//
// Sabotaje que la hace fallar: devolver `crudo` tal cual en urlSinSecretos.
func TestElQueryStringDelDestinoNoLlegaAlLog(t *testing.T) {
	casos := []struct {
		crudo   string
		prohib  string
		esperar string
	}{
		{"https://otlp.ejemplo.com/v1/metrics?api-key=SECRETO", "SECRETO", "otlp.ejemplo.com"},
		{"https://otlp.ejemplo.com/v1/metrics?token=abc&x=1", "abc", "/v1/metrics"},
		{"http://127.0.0.1:9099/api/v1/otlp/v1/metrics", "", "127.0.0.1:9099"},
		{"://roto", "", ""},
	}
	for _, c := range casos {
		got := urlSinSecretos(c.crudo)
		if c.prohib != "" && strings.Contains(got, c.prohib) {
			t.Errorf("urlSinSecretos(%q) devolvió %q: el secreto sigue ahí y ese texto va derecho al log", c.crudo, got)
		}
		if c.esperar != "" && !strings.Contains(got, c.esperar) {
			t.Errorf("urlSinSecretos(%q) devolvió %q: se tapó de más y el operador ya no sabe a dónde empuja", c.crudo, got)
		}
	}
	// Una URL que no parsea no puede devolverse a medias.
	if got := urlSinSecretos("://roto"); strings.Contains(got, "roto") {
		t.Errorf("una URL ilegible se mostró igual (%q): si no se entendió, no se arriesga a publicar un pedazo", got)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────────────────────

// sembrarServicios enrola una máquina en "casa" y le declara los servicios que se le pasen.
func sembrarServicios(t *testing.T, s *McpServer, device string, servicios ...string) {
	t.Helper()
	enrolarDePrueba(t, s, "casa", device)
	for _, svc := range servicios {
		if _, e := call(t, s, "musubi_fleet_service_declare",
			map[string]any{"device": device, "nombre": svc, "project": "casa"}); e != nil {
			t.Fatalf("declare %s en %s: %+v", svc, device, e)
		}
	}
}

func listarServicios(t *testing.T, s *McpServer, p *Principal, device string) string {
	t.Helper()
	args := map[string]any{}
	if device != "" {
		args["device"] = device
	}
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_services", args)
	if e != nil {
		t.Fatalf("musubi_fleet_services device=%q falló: %+v", device, e)
	}
	return textOf(t, res)
}
