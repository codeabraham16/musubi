package mcp

// Pruebas del slice S4: la telemetría del host, su viaje y su compuerta.

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func muestraDePrueba() *fleet.Muestra {
	cpu := 42.5
	return &fleet.Muestra{
		Tomada: time.Now().UTC(), CPUPct: &cpu, NumCPU: 8,
		MemTotal: 8 << 30, MemUsada: 3 << 30,
		DiscoTotal: 500 << 30, DiscoUsado: 100 << 30, DiscoDisponible: 375 << 30,
		Load1: f64ptr(1.5), UptimeSeg: 3600,
	}
}

func cuerpoConMuestra(t *testing.T, m *fleet.Muestra) string {
	t.Helper()
	txt, err := m.Serializar()
	if err != nil {
		t.Fatal(err)
	}
	return `{"muestra":` + txt + `}`
}

// D5 — el cuerpo trae MEDICIONES y nunca IDENTIDAD. Un `device_id` en el cuerpo no tiene dónde
// aterrizar: la muestra se atribuye a la máquina del TOKEN.
// Sabotaje: agregar un campo de identidad a cuerpoLatido y usarlo.
func TestLaMuestraSeAtribuyeAlTokenYNoAlCuerpo(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	otroToken := enrolarDePrueba(t, s, "casa", "servidor-critico")
	otro, _, _ := s.engine.DevicePorToken(otroToken)

	cuerpo := `{"device_id":"` + otro.ID + `","name":"servidor-critico","muestra":` +
		strings.TrimPrefix(strings.TrimSuffix(cuerpoConMuestra(t, muestraDePrueba()), "}"), `{"muestra":`) + `}`
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}

	// La muestra quedó en pc-gio, no en servidor-critico.
	pcGio, _, _ := s.engine.DevicePorToken(tokenDevice)
	if pcGio.UltimaMuestra == nil {
		t.Fatal("la máquina del token no recibió la muestra")
	}
	otroDespues, _, _ := s.engine.DevicePorToken(otroToken)
	if otroDespues.UltimaMuestra != nil {
		t.Fatal("la muestra se atribuyó a la máquina que declaró el CUERPO")
	}
}

// D6 — un cuerpo que pasa el tope se RECHAZA, y el latido sigue valiendo.
func TestUnCuerpoDemasiadoGrandeSeRechazaSinTumbarElLatido(t *testing.T) {
	_, ts, tokenDevice, _ := servidorConFlota(t)

	gigante := `{"muestra":{"tomada":"2026-08-26T12:00:00Z","num_cpu":1,"basura":"` +
		strings.Repeat("A", fleet.MuestraMaxBytes+100) + `"}}`
	code, resp := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, gigante)
	// D7 — el latido SIGUE valiendo; lo que se descarta es la medición.
	if code != http.StatusOK {
		t.Fatalf("un cuerpo gigante tumbó el latido (status %d): estar viva y saber medirse son cosas distintas", code)
	}
	if !strings.Contains(resp, "demasiado grande") {
		t.Errorf("no se rechazó por tamaño: %s", resp)
	}
}

// D6 — EL SERVIDOR NO LEE EL CUERPO ENTERO A MEMORIA.
//
// La primera versión de esta prueba NO sabía fallar, y vale dejar por qué. Mandaba un cuerpo
// grande y esperaba el rechazo; pasaba con LimitReader y SIN él, porque el chequeo de tamaño
// (`len(crudo) > MuestraMaxBytes`) rechaza igual. O sea: el chequeo es lo que RECHAZA, y el
// LimitReader es lo que evita LEER megabytes a memoria — dos cosas distintas, y la prueba sólo
// cubría la primera.
//
// Ésta cuenta cuántos bytes logra ESCRIBIR el cliente antes de que el otro lado deje de leer,
// que es el reflejo observable de lo mismo. Medido: con el LimitReader el cliente coloca ~1,3 MiB
// (lo que entra en los buffers del socket) y ahí se corta; sin él, el servidor se traga los 20
// MiB enteros. La separación entre los dos casos es de un orden de magnitud, así que el techo
// puede ser generoso sin volverse frágil.
//
// Un agente corre en la superficie más expuesta de la flota: un cuerpo sin tope es un DoS con
// forma de telemetría, y el techo general del transporte (4 MiB) es demasiado alto para esta
// puerta.
//
// Sabotaje que la hace fallar: cambiar io.LimitReader(r.Body, ...) por r.Body.
func TestElServidorNoLeeElCuerpoEnteroAMemoria(t *testing.T) {
	_, ts, tokenDevice, _ := servidorConFlota(t)

	const tam = 20 << 20 // 20 MiB
	contador := &lectorQueCuenta{restante: tam}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+fleetHeartbeatPath, contador)
	req.Header.Set("Authorization", "Bearer "+tokenDevice)
	req.ContentLength = tam
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	escritos := contador.leidos
	// Techo con margen de sobra: medido, el caso bueno coloca ~1,3 MiB y el malo 20 MiB.
	const techo = 5 << 20
	if escritos > techo {
		t.Errorf("el cliente colocó %d bytes de %d: el servidor siguió leyendo, así que el tope sólo rechaza DESPUÉS de tragarse el cuerpo entero",
			escritos, tam)
	}
	t.Logf("de %d bytes ofrecidos, el otro lado aceptó %d", tam, escritos)
}

// lectorQueCuenta entrega bytes y lleva la cuenta de cuántos le pidieron.
type lectorQueCuenta struct {
	restante int
	leidos   int
}

func (l *lectorQueCuenta) Read(p []byte) (int, error) {
	if l.restante <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > l.restante {
		n = l.restante
	}
	for i := 0; i < n; i++ {
		p[i] = 'A'
	}
	l.restante -= n
	l.leidos += n
	return n, nil
}

// D7 — un cuerpo inválido descarta la MEDICIÓN, no el LATIDO.
// Sabotaje: devolver 400 ante un JSON roto → un agente con el colector roto desaparece del
// inventario, que es justo cuando más querés verlo.
func TestUnCuerpoInvalidoNoTumbaElLatido(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	casos := []struct{ nombre, cuerpo, espera string }{
		{"JSON roto", `{"muestra": {roto`, "JSON inválido"},
		{"cpu imposible", `{"muestra":{"tomada":"2026-08-26T12:00:00Z","cpu_pct":900,"num_cpu":1}}`, "cpu_pct fuera de rango"},
		{"memoria imposible", `{"muestra":{"tomada":"2026-08-26T12:00:00Z","mem_total":10,"mem_usada":99}}`, "mem_usada"},
		{"load negativo", `{"muestra":{"tomada":"2026-08-26T12:00:00Z","load1":-5}}`, "load1 negativo"},
	}
	for _, c := range casos {
		code, resp := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, c.cuerpo)
		if code != http.StatusOK {
			t.Errorf("%s: status %d, el latido debería seguir valiendo", c.nombre, code)
		}
		if !strings.Contains(resp, c.espera) {
			t.Errorf("%s: la respuesta no explica el descarte (%q): %s", c.nombre, c.espera, resp)
		}
	}
	// Y pese a todos los rechazos, la máquina figura viva.
	d, _, _ := s.engine.DevicePorToken(tokenDevice)
	if d.LastSeen.IsZero() {
		t.Error("tras cuatro cuerpos inválidos, la máquina no figura como viva")
	}
	if d.UltimaMuestra != nil {
		t.Error("se guardó una muestra inválida")
	}
}

// D8 — sin la capacidad `metrics`, la máquina late pero su muestra se descarta.
// Sabotaje: quitar la guarda `d.Permite(fleet.CapMetrics)` → conceder capacidades sería un gesto
// sin efecto.
func TestSinLaCapacidadMetricsLaMuestraSeDescarta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Una máquina a la que SÓLO se le concedió exec.
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "solo-exec", "tier": "A", "caps": []string{"exec"}, "project": "casa",
	})
	if e != nil {
		t.Fatal(e)
	}
	token, _ := jsonOf(t, res)["token"].(string)
	ts := servidorHTTP(t, s)

	code, resp := postCon(t, ts.URL+fleetHeartbeatPath, token, cuerpoConMuestra(t, muestraDePrueba()))
	if code != http.StatusOK {
		t.Fatalf("el latido debería valer igual: status %d", code)
	}
	if !strings.Contains(resp, "no tiene concedida la capacidad") {
		t.Errorf("la respuesta no explica por qué se descartó: %s", resp)
	}
	d, _, _ := s.engine.DevicePorToken(token)
	if d.UltimaMuestra != nil {
		t.Fatal("se guardó la muestra de una máquina sin `metrics`: la capacidad sería decorativa")
	}
	if d.LastSeen.IsZero() {
		t.Error("la máquina no figura viva: latir no depende de poder medirse")
	}
}

// D9 — leer las métricas exige la capacidad, POR MÁQUINA. Primer consumidor real de la
// compuerta de S3.
// Sabotaje: quitar el PuedeSobreDevice del filtro de toolFleetMetrics.
func TestLeerMetricasExigeLaCapacidadPorMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)

	// Dos máquinas con métricas reportadas.
	for _, n := range []string{"pc-gio", "servidor"} {
		tok := enrolarDePrueba(t, s, "casa", n)
		if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, muestraDePrueba())); code != http.StatusOK {
			t.Fatalf("%s: el latido con muestra falló", n)
		}
	}

	// Un principal con metrics sobre UNA sola.
	acotado := &Principal{
		Name: "op", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"pc-gio"}},
	}
	res, e := callAsPrincipal(t, s, acotado, "musubi_fleet_metrics", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	out := jsonOf(t, res)
	devs, _ := out["devices"].([]any)
	if len(devs) != 1 {
		t.Fatalf("esperaba 1 máquina visible, obtuve %d", len(devs))
	}
	if fila, _ := devs[0].(map[string]any); fila["name"] != "pc-gio" {
		t.Fatalf("vio la máquina equivocada: %v", fila["name"])
	}
	// Y SE DICE cuántas quedaron fuera: una lista corta sin explicación se lee como «no hay más».
	if out["sin_permiso"] == nil {
		t.Error("no se informa cuántas máquinas quedaron fuera por permiso")
	}

	// Sin concesiones: lista vacía. No es un error, es la compuerta.
	sinNada := &Principal{Name: "nadie", Role: RoleReader, Read: ReadOwn, ProjectID: "casa"}
	res2, e2 := callAsPrincipal(t, s, sinNada, "musubi_fleet_metrics", map[string]any{})
	if e2 != nil {
		t.Fatal(e2)
	}
	if devs2, _ := jsonOf(t, res2)["devices"].([]any); len(devs2) != 0 {
		t.Fatalf("un principal sin concesiones vio %d máquinas", len(devs2))
	}
}

// D1/D3 — lo desconocido viaja como null, nunca como 0, hasta la respuesta de la tool.
// Sabotaje: reemplazar los nil por 0 en filaDeMetricas.
func TestLoDesconocidoViajaComoNullHastaLaRespuesta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")

	// Una muestra SIN cpu_pct ni temp_c: exactamente la primera de un agente recién arrancado.
	sinCPU := &fleet.Muestra{Tomada: time.Now().UTC(), NumCPU: 4, MemTotal: 100, MemUsada: 25}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, sinCPU)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	res, e := call(t, s, "musubi_fleet_metrics", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, res)
	if !strings.Contains(crudo, `"cpu_pct":null`) {
		t.Errorf("cpu_pct no viajó como null: %s", crudo)
	}
	if !strings.Contains(crudo, `"temp_c":null`) {
		t.Errorf("temp_c no viajó como null: %s", crudo)
	}
	// Y lo que SÍ se midió sale derivado.
	if !strings.Contains(crudo, `"mem_pct":25`) {
		t.Errorf("mem_pct no se derivó: %s", crudo)
	}
}

// Un latido SIN muestra no borra la anterior: un colector que se rompe no puede hacer
// desaparecer la última medición buena.
// Sabotaje: escribir la columna siempre en LatirDevice.
func TestUnLatidoSinMuestraNoBorraLaAnterior(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoConMuestra(t, muestraDePrueba())); code != http.StatusOK {
		t.Fatal("primer latido con muestra falló")
	}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, ""); code != http.StatusOK {
		t.Fatal("segundo latido sin muestra falló")
	}
	d, _, _ := s.engine.DevicePorToken(tokenDevice)
	if d.UltimaMuestra == nil {
		t.Fatal("un latido SIN muestra borró la anterior: un colector roto haría desaparecer la última medición buena")
	}
	if d.UltimaMuestra.NumCPU != 8 {
		t.Errorf("la muestra conservada no es la que se guardó: %+v", d.UltimaMuestra)
	}
}

// CABO CERRADO — el agente reporta lo que sabe de SÍ MISMO (versión y dirección), y eso NO
// contradice B4/D5: el invariante es que no puede decir QUIÉN ES, no que no pueda decir CÓMO
// ESTÁ. La fila que toca es la del token presentado y ninguna otra.
//
// Sabotaje que la hace fallar: usar un `device_id` del cuerpo en ActualizarAutoreporte.
func TestElAutorreporteSoloTocaLaFilaDelToken(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	otroToken := enrolarDePrueba(t, s, "casa", "servidor-critico")
	otro, _, _ := s.engine.DevicePorToken(otroToken)

	// pc-gio reporta su versión declarando ser el otro.
	cuerpo := `{"device_id":"` + otro.ID + `","name":"servidor-critico","version":"v9.9.9","direccion":"100.1.2.3"}`
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	pcGio, _, _ := s.engine.DevicePorToken(tokenDevice)
	if pcGio.AgentVer != "v9.9.9" || pcGio.Address != "100.1.2.3" {
		t.Fatalf("la máquina del token no recibió su autorreporte: ver=%q dir=%q", pcGio.AgentVer, pcGio.Address)
	}
	otroDespues, _, _ := s.engine.DevicePorToken(otroToken)
	if otroDespues.AgentVer != "" || otroDespues.Address != "" {
		t.Fatalf("el autorreporte tocó la fila de OTRA máquina: ver=%q dir=%q", otroDespues.AgentVer, otroDespues.Address)
	}
}

// Un campo vacío no pisa lo que había: un agente viejo que no reporta versión no puede borrar la
// que quedó registrada.
func TestElAutorreporteVacioNoBorraLoAnterior(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, `{"version":"v1.2.3","direccion":"100.9.9.9"}`)
	postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, `{"version":"","direccion":""}`)

	d, _, _ := s.engine.DevicePorToken(tokenDevice)
	if d.AgentVer != "v1.2.3" || d.Address != "100.9.9.9" {
		t.Fatalf("un autorreporte vacío borró lo anterior: ver=%q dir=%q", d.AgentVer, d.Address)
	}
}

// El autorreporte NO depende de `metrics`: saber qué build corre una máquina es inventario, no
// telemetría. Una máquina sin esa capacidad igual se identifica.
func TestElAutorreporteNoDependeDeLaCapacidadMetrics(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "solo-exec", "tier": "A", "caps": []string{"exec"}, "project": "casa",
	})
	if e != nil {
		t.Fatal(e)
	}
	token, _ := jsonOf(t, res)["token"].(string)
	ts := servidorHTTP(t, s)

	postCon(t, ts.URL+fleetHeartbeatPath, token, `{"version":"v2.0.0"}`)
	d, _, _ := s.engine.DevicePorToken(token)
	if d.AgentVer != "v2.0.0" {
		t.Fatalf("una máquina sin `metrics` no pudo identificarse: %q", d.AgentVer)
	}
	if d.UltimaMuestra != nil {
		t.Error("y su muestra sigue descartándose, como debe")
	}
}

// Un texto absurdo del device no ensucia el inventario ni las etiquetas de Prometheus.
func TestElAutorreporteSeRecorta(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	largo := strings.Repeat("v", 500)
	postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, `{"version":"`+largo+`"}`)

	d, _, _ := s.engine.DevicePorToken(tokenDevice)
	if len(d.AgentVer) > 64 {
		t.Errorf("la versión no se recortó: %d caracteres", len(d.AgentVer))
	}
}

// f64ptr construye un *float64 para las muestras de prueba.
func f64ptr(v float64) *float64 { return &v }
