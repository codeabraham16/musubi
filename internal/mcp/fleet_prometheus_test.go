package mcp

// Pruebas del export de la flota a Prometheus.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// exportar corre el render de flota para un principal y devuelve el texto.
func exportar(t *testing.T, s *McpServer, p *Principal) string {
	t.Helper()
	var b strings.Builder
	renderFlota(&b, s.engine, p, time.Now(), s.sondaIntervalo)
	return b.String()
}

// LA REGLA CENTRAL: un valor DESCONOCIDO no se exporta como 0, no se exporta y punto.
//
// En Prometheus importa más que en el JSON: una serie ausente se dibuja como un hueco y
// `absent()` la puede alertar, mientras que un 0 entra al gráfico como una medición real. Un
// cpu_percent 0 en el primer latido de cada agente pintaría una caída a cero en cada reinicio.
//
// Sabotaje que la hace fallar: devolver (0, true) en vez de (0, false) cuando CPUPct es nil.
func TestUnValorDesconocidoNoSeExportaComoCero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")

	// Muestra SIN cpu_pct ni temp_c: la primera de un agente recién arrancado.
	sinCPU := &fleet.Muestra{Tomada: time.Now().UTC(), NumCPU: 4, MemTotal: 100, MemUsada: 25, DiscoTotal: 1000, DiscoUsado: 100, DiscoDisponible: 850}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, sinCPU)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	out := exportar(t, s, nil) // stdio local: ve todo
	if strings.Contains(out, "musubi_fleet_device_cpu_percent") {
		t.Errorf("se exportó cpu_percent sin haberlo medido:\n%s", out)
	}
	if strings.Contains(out, "musubi_fleet_device_temperature_celsius") {
		t.Errorf("se exportó temperatura sin sensor:\n%s", out)
	}
	// Lo que SÍ se midió está.
	for _, quiero := range []string{
		`musubi_fleet_device_memory_used_bytes{device="pc-gio",project="casa",tier="A",os="linux"} 25`,
		`musubi_fleet_device_disk_available_bytes{device="pc-gio",project="casa",tier="A",os="linux"} 850`,
		`musubi_fleet_device_cpus{device="pc-gio",project="casa",tier="A",os="linux"} 4`,
	} {
		if !strings.Contains(out, quiero) {
			t.Errorf("falta la serie medida:\n  %s\nen:\n%s", quiero, out)
		}
	}
	// Y sin series, tampoco cabeceras: un HELP/TYPE sin líneas es ruido.
	if strings.Contains(out, "# TYPE musubi_fleet_device_cpu_percent") {
		t.Error("se emitió TYPE de una métrica sin ninguna serie")
	}
}

// La compuerta de S3 gobierna el scrape: el scraper es un principal más, no un caso especial.
//
// Sabotaje: quitar el PuedeSobreDevice del filtro → el token de Prometheus se convierte en una
// puerta trasera que sortea el eje de capacidades entero.
func TestElScrapeExportaSoloLoQueEsaCredencialPuedeVer(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	for _, n := range []string{"pc-gio", "servidor"} {
		tok := enrolarDePrueba(t, s, "casa", n)
		if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, muestraDePrueba())); code != http.StatusOK {
			t.Fatalf("%s: latido con muestra falló", n)
		}
	}

	// Un scraper con metrics sobre UNA sola máquina.
	acotado := &Principal{
		Name: "prom", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"pc-gio"}},
	}
	out := exportar(t, s, acotado)
	if !strings.Contains(out, `device="pc-gio"`) {
		t.Errorf("no exportó la máquina que sí puede ver:\n%s", out)
	}
	if strings.Contains(out, `device="servidor"`) {
		t.Errorf("exportó una máquina que esa credencial NO puede ver:\n%s", out)
	}
}

// C1 llega hasta el scrape: un admin SIN grants de flota no exporta ninguna máquina, y la salida
// lo DICE en vez de quedarse muda.
//
// Sabotaje: hacer que renderFlota trate al admin como federado con acceso pleno.
func TestUnAdminSinGrantsNoExportaNadaYLoDice(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")
	postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, muestraDePrueba()))

	admin := &Principal{Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny}
	out := exportar(t, s, admin)
	if strings.Contains(out, `device="pc-gio"`) {
		t.Errorf("un admin sin grants de flota exportó telemetría:\n%s", out)
	}
	if !strings.Contains(out, "ninguna máquina visible") || !strings.Contains(out, "principals.yaml") {
		t.Errorf("la salida vacía no explica por qué; alguien va a depurar Prometheus en vez de principals.yaml:\n%s", out)
	}
}

// La tenencia también gobierna el scrape.
func TestElScrapeNoCruzaTenants(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	for _, par := range [][2]string{{"casa", "pc-gio"}, {"cliente-acme", "server-acme"}} {
		tok := enrolarDePrueba(t, s, par[0], par[1])
		postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, muestraDePrueba()))
	}
	// Comodín de capacidades, pero acotado a `casa`.
	acotado := &Principal{
		Name: "prom", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	out := exportar(t, s, acotado)
	if strings.Contains(out, `device="server-acme"`) {
		t.Errorf("el scrape cruzó tenants:\n%s", out)
	}
	if !strings.Contains(out, `device="pc-gio"`) {
		t.Errorf("no exportó lo suyo:\n%s", out)
	}
	// Uno federado CON grants ve las dos.
	federado := &Principal{
		Name: "prom-global", Role: RoleReader, Read: ReadAll,
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	todo := exportar(t, s, federado)
	if !strings.Contains(todo, `device="server-acme"`) || !strings.Contains(todo, `device="pc-gio"`) {
		t.Errorf("un federado con grants debería ver las dos flotas:\n%s", todo)
	}
}

// Un nombre de máquina con comillas partiría la línea y corrompería TODO el scrape, no sólo esa
// serie. Los nombres los escribe un administrador, así que es alcanzable.
//
// Sabotaje: interpolar el nombre sin citarLabel.
func TestUnNombreConComillasNoCorrompeElScrape(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": `raro"y\malo`, "tier": "A", "caps": []string{"metrics"}, "project": "casa", "os": "linux",
	})
	if e != nil {
		t.Fatal(e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, muestraDePrueba()))

	out := exportar(t, s, nil)
	if !strings.Contains(out, `device="raro\"y\\malo"`) {
		t.Errorf("el nombre no se escapó según el exposition format:\n%s", out)
	}
	// Cada línea de serie tiene que seguir teniendo exactamente un `}` de cierre de labels.
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, "musubi_fleet_") {
			continue
		}
		if strings.Count(l, "{") != 1 || strings.Count(l, "} ") != 1 {
			t.Errorf("línea corrupta en el exposition format: %q", l)
		}
	}
}

// Los bytes de un disco grande no pueden salir en notación científica: Prometheus la acepta,
// pero un `5.024e+11` en el dump es ilegible para el humano que depura.
func TestLosBytesGrandesNoSalenEnNotacionCientifica(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")
	m := muestraDePrueba()
	m.DiscoTotal = 502392610816
	m.DiscoUsado = 85868347392
	m.DiscoDisponible = 391000000000
	postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, m))

	out := exportar(t, s, nil)
	if !strings.Contains(out, "502392610816") {
		t.Errorf("los bytes del disco no salieron enteros:\n%s", out)
	}
	if strings.Contains(out, "e+") {
		t.Errorf("hay notación científica en la salida:\n%s", out)
	}
}

// Una máquina que nunca reportó aporta `up` y nada más: no inventa series de una muestra que no
// existe.
func TestUnaMaquinaSinMuestraAportaSoloSuEstado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "muda")

	out := exportar(t, s, nil)
	if !strings.Contains(out, `musubi_fleet_device_up{device="muda"`) {
		t.Errorf("falta el `up` de una máquina enrolada:\n%s", out)
	}
	for _, noQuiero := range []string{"memory_total", "disk_total", "load1", "uptime_seconds", "cpus"} {
		if strings.Contains(out, "musubi_fleet_device_"+noQuiero) {
			t.Errorf("se exportó %q para una máquina que nunca reportó:\n%s", noQuiero, out)
		}
	}
	// Y nunca latió, así que tampoco hay last_seen.
	if strings.Contains(out, "last_seen_seconds") {
		t.Errorf("se exportó last_seen de una máquina que nunca latió:\n%s", out)
	}
}
