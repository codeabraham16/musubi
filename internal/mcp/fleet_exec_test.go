package mcp

// Pruebas del slice S5: ejecución remota auditada.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// enrolarConExec da de alta una máquina con metrics+exec y devuelve su token.
func enrolarConExec(t *testing.T, s *McpServer, proyecto, nombre string) string {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics", "exec"}, "project": proyecto, "os": "linux",
	})
	if e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	return tok
}

// conExec es un principal con exec sobre todo su proyecto.
func conExec(proyecto string) *Principal {
	return &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: proyecto,
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
	}
}

// F1 — LA BITÁCORA SE ESCRIBE ANTES DE EJECUTAR.
//
// El registro existe desde el encolado, no desde el resultado. Si el cerebro se cae, si el agente
// nunca responde, si la máquina se apaga: el PEDIDO queda auditado igual.
//
// Sabotaje que la hace fallar: encolar recién al recibir el resultado.
func TestElPedidoQuedaAuditadoAunqueNadieLoEjecute(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")

	// Se encola y NO hay agente que lo levante.
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"echo", "hola"}, "no_wait": true,
	})
	if e != nil {
		t.Fatalf("exec: %+v", e)
	}
	id, _ := jsonOf(t, res)["command_id"].(string)
	if id == "" {
		t.Fatal("no se devolvió command_id")
	}

	// La bitácora ya lo tiene, con quién lo pidió, aunque nunca se ejecutó.
	log, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_log", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, log)
	for _, quiero := range []string{id, `"principal":"op"`, `"estado":"pendiente"`, `"exit_code":null`} {
		if !strings.Contains(crudo, quiero) {
			t.Errorf("la bitácora no registra %q antes de ejecutar:\n%s", quiero, crudo)
		}
	}
}

// F4 — sin `CapExec` sobre ESA máquina no se encola.
// Sabotaje: quitar el PuedeSobreDevice de toolFleetExec.
func TestSinCapacidadExecNoSeEncolaNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")

	// Un admin de la MEMORIA, sin concesiones de flota. La valla del track entero.
	admin := &Principal{Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}
	_, e := callAsPrincipal(t, s, admin, "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"rm", "-rf", "/"},
	})
	if e == nil {
		t.Fatal("un admin sin concesiones de flota pudo ejecutar: el puente de privilegio quedó abierto")
	}
	if e.Code != codeUnauthorized {
		t.Errorf("esperaba unauthorized, obtuve %d", e.Code)
	}
	// Y NADA quedó encolado: el rechazo es antes de tocar la cola.
	if cs, err := s.engine.BitacoraDeComandos("casa", "", 10); err != nil || len(cs) != 0 {
		t.Fatalf("quedaron %d comandos encolados pese al rechazo (err=%v)", len(cs), err)
	}
}

// Uno con exec sobre OTRA máquina tampoco puede sobre ésta.
func TestLaCapacidadExecEsPorMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	enrolarConExec(t, s, "casa", "servidor-critico")

	acotado := &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"pc-gio"}},
	}
	if _, e := callAsPrincipal(t, s, acotado, "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"true"}, "no_wait": true,
	}); e != nil {
		t.Fatalf("debería poder sobre la máquina nombrada: %+v", e)
	}
	if _, e := callAsPrincipal(t, s, acotado, "musubi_fleet_exec", map[string]any{
		"device": "servidor-critico", "argv": []string{"true"}, "no_wait": true,
	}); e == nil {
		t.Fatal("pudo ejecutar en una máquina que NO tiene nombrada")
	}
}

// El rechazo no distingue «no existe» de «no podés»: si no, la tool es un oráculo de qué
// máquinas hay en un proyecto que no ves.
func TestElRechazoDeExecNoRevelaSiLaMaquinaExiste(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "existe-pero-no-podes")
	sinNada := &Principal{Name: "x", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa"}

	_, e1 := callAsPrincipal(t, s, sinNada, "musubi_fleet_exec", map[string]any{
		"device": "existe-pero-no-podes", "argv": []string{"true"}})
	_, e2 := callAsPrincipal(t, s, sinNada, "musubi_fleet_exec", map[string]any{
		"device": "no-existe-en-absoluto", "argv": []string{"true"}})
	if e1 == nil || e2 == nil {
		t.Fatal("alguno de los dos no fue rechazado")
	}
	// Se compara el mensaje SIN el nombre de la máquina: ese nombre lo escribió el propio
	// llamador, así que repetírselo no le revela nada. Lo que no puede diferir es el RESTO.
	plantilla := func(m, nombre string) string { return strings.ReplaceAll(m, nombre, "<X>") }
	t1 := plantilla(e1.Message, "existe-pero-no-podes")
	t2 := plantilla(e2.Message, "no-existe-en-absoluto")
	if t1 != t2 || e1.Code != e2.Code {
		t.Errorf("el rechazo distingue existencia de permiso:\n existe: %s\n no existe: %s", t1, t2)
	}
	// Y el mensaje tiene que ofrecer las DOS lecturas, no una: si dijera sólo «no tenés permiso»,
	// confirmaría que la máquina existe.
	if !strings.Contains(e1.Message, "o no existe") {
		t.Errorf("el mensaje no ofrece la lectura «no existe», así que confirma que existe: %s", e1.Message)
	}
}

// F5 — un comando se entrega SÓLO a la máquina a la que fue dirigido.
// Sabotaje: que TomarComandos ignore el device_id.
func TestUnComandoSoloLlegaASuMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tokA := enrolarConExec(t, s, "casa", "maquina-a")
	tokB := enrolarConExec(t, s, "casa", "maquina-b")
	ts := servidorHTTP(t, s)

	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "maquina-a", "argv": []string{"echo", "SECRETO-DE-A"}, "no_wait": true,
	}); e != nil {
		t.Fatal(e)
	}

	// B late primero: no debe llevarse nada.
	_, respB := postCon(t, ts.URL+fleetHeartbeatPath, tokB, "")
	if strings.Contains(respB, "SECRETO-DE-A") {
		t.Fatalf("la máquina B recibió un comando dirigido a A:\n%s", respB)
	}
	// A late: sí se lo lleva.
	_, respA := postCon(t, ts.URL+fleetHeartbeatPath, tokA, "")
	if !strings.Contains(respA, "SECRETO-DE-A") {
		t.Fatalf("la máquina A no recibió su comando:\n%s", respA)
	}
}

// F3 — el resultado sólo lo puede reportar la máquina dueña del comando.
// Sabotaje: quitar la comparación de device_id en GuardarResultado.
func TestUnaMaquinaNoPuedeEscribirLaBitacoraDeOtra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "maquina-a")
	tokB := enrolarConExec(t, s, "casa", "maquina-b")
	ts := servidorHTTP(t, s)

	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "maquina-a", "argv": []string{"true"}, "no_wait": true})
	if e != nil {
		t.Fatal(e)
	}
	id, _ := jsonOf(t, res)["command_id"].(string)

	// B intenta reportar el resultado del comando de A.
	code, _ := postCon(t, ts.URL+fleetResultPath, tokB,
		`{"command_id":"`+id+`","exit_code":0,"stdout":"MENTIRA"}`)
	if code == http.StatusOK {
		t.Fatal("la máquina B escribió el resultado de un comando de A: la bitácora es envenenable")
	}
	c, _, _ := s.engine.ComandoPorID(id)
	if c.Stdout != "" || c.Estado == fleet.EstadoTerminado {
		t.Fatalf("el comando quedó contaminado: %+v", c)
	}
}

// F6 — revocar corta la cola. Es la ruta más peligrosa del track: el kill-switch tiene que valer.
// Sabotaje: que el handler entregue la cola antes de resolver el token.
func TestUnDeviceRevocadoNoRecibeComandos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"echo", "PELIGRO"}, "no_wait": true}); e != nil {
		t.Fatal(e)
	}
	if _, e := call(t, s, "musubi_fleet_revoke", map[string]any{"name": "pc-gio", "project": "casa"}); e != nil {
		t.Fatal(e)
	}
	code, resp := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("un device revocado latió: status %d", code)
	}
	if strings.Contains(resp, "PELIGRO") {
		t.Fatalf("un device revocado recibió un comando encolado:\n%s", resp)
	}
}

// F10 — un comando VIEJO no se ejecuta. El peor pie de bala de una cola: el reinicio pedido en
// una emergencia, ejecutándose una semana después sobre un estado distinto.
// Sabotaje: quitar el UPDATE de vencimiento de TomarComandos.
func TestUnComandoViejoVenceYNoSeEntrega(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorToken(tok)

	// Encolado hace una semana: se escribe directo para poder viajar en el tiempo.
	viejo, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op",
		Argv: []string{"systemctl", "restart", "todo"}, Timeout: 30 * time.Second,
		Creado: time.Now().Add(-7 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Y uno recién encolado, que SÍ tiene que llegar.
	nuevo, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op",
		Argv: []string{"echo", "RECIENTE"}, Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := servidorHTTP(t, s)
	_, resp := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	if strings.Contains(resp, "restart") {
		t.Fatalf("se entregó un comando de hace una semana:\n%s", resp)
	}
	if !strings.Contains(resp, "RECIENTE") {
		t.Fatalf("no se entregó el comando reciente:\n%s", resp)
	}
	if c, _, _ := s.engine.ComandoPorID(viejo.ID); c.Estado != fleet.EstadoExpirado {
		t.Errorf("el comando viejo quedó en %q, esperaba expirado", c.Estado)
	}
	if c, _, _ := s.engine.ComandoPorID(nuevo.ID); c.Estado != fleet.EstadoEntregado {
		t.Errorf("el comando nuevo quedó en %q, esperaba entregado", c.Estado)
	}
}

// Dos latidos concurrentes de la misma máquina no pueden llevarse el mismo comando: un script de
// migración ejecutado dos veces no es lo mismo que un `restart` duplicado.
func TestDosLatidosNoSeLlevanElMismoComando(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	for i := 0; i < 5; i++ {
		if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
			"device": "pc-gio", "argv": []string{"echo", "uno"}, "no_wait": true}); e != nil {
			t.Fatal(e)
		}
	}

	vistos := map[string]int{}
	for i := 0; i < 3; i++ {
		_, resp := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
		for _, id := range idsDeComandos(resp) {
			vistos[id]++
		}
	}
	for id, n := range vistos {
		if n > 1 {
			t.Errorf("el comando %s se entregó %d veces", id, n)
		}
	}
	if len(vistos) != 5 {
		t.Errorf("se entregaron %d comandos distintos, esperaba 5", len(vistos))
	}
}

// F2 — la bitácora es PERMANENTE, la salida CADUCA.
// Sabotaje: que la poda borre la fila en vez de vaciar las columnas.
func TestLaPodaBorraLaSalidaYConservaLaBitacora(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorToken(tok)

	c, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio",
		Argv: []string{"cat", "/etc/secretos"}, Timeout: time.Second,
		Creado: time.Now().Add(-60 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	cero := 0
	if err := s.engine.GuardarResultado(d.ID, c.ID, &cero, "CLAVE=hunter2", "", "", time.Now()); err != nil {
		t.Fatal(err)
	}

	n, err := s.engine.PodarSalidasDeComandos(30, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("poda: n=%d err=%v", n, err)
	}
	got, existe, _ := s.engine.ComandoPorID(c.ID)
	if !existe {
		t.Fatal("la poda BORRÓ la fila: se perdió quién ejecutó qué")
	}
	if got.Stdout != "" {
		t.Errorf("la salida sobrevivió a la poda: %q", got.Stdout)
	}
	if got.Principal != "gio" || len(got.Argv) == 0 || got.ExitCode == nil {
		t.Errorf("la poda se llevó datos de auditoría: %+v", got)
	}
}

// F9 — la salida se acota también EN EL CEREBRO. El agente es un cliente y sus límites son una
// cortesía, no una garantía.
func TestLaSalidaSeAcotaTambienEnElCerebro(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorToken(tok)
	c, _ := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Argv: []string{"true"}, Timeout: time.Second})

	gigante := strings.Repeat("A", fleet.SalidaMaxBytes*2)
	cero := 0
	if err := s.engine.GuardarResultado(d.ID, c.ID, &cero, gigante, "", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.engine.ComandoPorID(c.ID)
	if len(got.Stdout) > fleet.SalidaMaxBytes+len(fleet.AvisoTruncado) {
		t.Errorf("la salida no se acotó: %d bytes", len(got.Stdout))
	}
	if !strings.Contains(got.Stdout, "truncada") {
		t.Error("se truncó SIN dejar la marca: quien lea el log saca conclusiones de datos que no están")
	}
}

// idsDeComandos extrae los `"id":"..."` de una respuesta de latido.
func idsDeComandos(resp string) []string {
	var out []string
	for _, trozo := range strings.Split(resp, `"id":"`)[1:] {
		if i := strings.Index(trozo, `"`); i > 0 {
			out = append(out, trozo[:i])
		}
	}
	return out
}

// LA ESPERA NUNCA PUEDE SUPERAR AL DEADLINE DEL TRANSPORTE.
//
// Lo destapó un sabotaje: sin la compuerta, un exec sobre una máquina inexistente esperaba 90 s
// —timeout del comando más dos márgenes— cuando el transporte corta a los 60. El caller no
// habría recibido la nota honesta «sigue corriendo»: habría recibido un timeout de HTTP, que se
// lee como «el cerebro no anda» cuando todo funciona.
//
// Sabotaje que la hace fallar: subir esperaMaxExec por encima del deadline del transporte.
func TestLaEsperaDeExecNoSuperaAlTransporte(t *testing.T) {
	const deadlineDelTransporte = 60 * time.Second // config service.request_timeout_seconds
	if esperaMaxExec >= deadlineDelTransporte {
		t.Fatalf("esperaMaxExec (%s) alcanza el deadline del transporte (%s): un comando lento daría "+
			"un timeout de HTTP en vez de la nota «sigue corriendo»", esperaMaxExec, deadlineDelTransporte)
	}
}

// Una máquina CAÍDA no hace esperar al llamador: se dice y se encola.
// Sabotaje: quitar la guarda de EnLinea → un exec sobre una máquina apagada bloquea 45 s.
func TestExecSobreUnaMaquinaCaidaVuelveEnseguida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-apagada") // nunca latió

	arranque := time.Now()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-apagada", "argv": []string{"true"},
	})
	tardo := time.Since(arranque)
	if e != nil {
		t.Fatalf("exec: %+v", e)
	}
	if tardo > 5*time.Second {
		t.Errorf("tardó %s sobre una máquina que no late: debería volver enseguida", tardo)
	}
	out := jsonOf(t, res)
	if out["estado"] != "pendiente" {
		t.Errorf("estado = %v, esperaba pendiente", out["estado"])
	}
	nota, _ := out["nota"].(string)
	if !strings.Contains(nota, "no está latiendo") {
		t.Errorf("la nota no explica que la máquina está caída: %q", nota)
	}
	// Y el comando QUEDÓ encolado: la máquina puede volver.
	if cs, _ := s.engine.BitacoraDeComandos("casa", "", 10); len(cs) != 1 {
		t.Errorf("esperaba 1 comando encolado, hay %d", len(cs))
	}
}

// Un comando con timeout largo se encola y se consulta después, en vez de bloquear más de lo que
// el transporte aguanta.
func TestUnTimeoutLargoSeEncolaEnVezDeBloquear(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "") // que figure en línea

	arranque := time.Now()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"sleep", "300"}, "timeout_seg": 300,
	})
	if e != nil {
		t.Fatalf("exec: %+v", e)
	}
	if tardo := time.Since(arranque); tardo > 5*time.Second {
		t.Errorf("tardó %s: un timeout de 300s debería encolarse, no esperarse", tardo)
	}
	nota, _ := jsonOf(t, res)["nota"].(string)
	if !strings.Contains(nota, "transporte") {
		t.Errorf("la nota no explica por qué no se esperó: %q", nota)
	}
}

// S7 — un Tier B se ejecuta por SSH, con la MISMA tool, la MISMA compuerta y la MISMA bitácora.
//
// Sabotaje que la hace fallar: no rutear por tier → el comando se encola esperando un agente que
// nunca va a existir.
func TestUnTierBSeEjecutaPorSSHYNoSeEncola(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics", "exec"},
		"project": "infra", "address": "gio@router.local",
	}); e != nil {
		t.Fatal(e)
	}
	restaurar := fleet.SSHFalsoParaTest(t, "echo 'uptime remoto'; exit 0")
	defer restaurar()

	p := &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "infra",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "router", "argv": []string{"uptime"},
	})
	if e != nil {
		t.Fatalf("exec en Tier B: %+v", e)
	}
	out := jsonOf(t, res)
	if out["transporte"] != "ssh" {
		t.Errorf("no se ruteó por SSH: transporte=%v", out["transporte"])
	}
	if out["estado"] != string(fleet.EstadoTerminado) {
		t.Fatalf("el comando quedó en %v: un Tier B no tiene agente que lo levante", out["estado"])
	}
	if !strings.Contains(textOf(t, res), "uptime remoto") {
		t.Errorf("no volvió la salida del comando: %s", textOf(t, res))
	}
	// Y quedó en la MISMA bitácora que los de Tier A.
	log, _ := callAsPrincipal(t, s, p, "musubi_fleet_log", map[string]any{"project": "infra"})
	if !strings.Contains(textOf(t, log), "uptime") {
		t.Error("el comando de Tier B no quedó en la bitácora")
	}
}

// H3a — el cerebro estampa `last_seen` de un Tier B SÓLO si llegó. Un fallo de canal no es
// prueba de vida, y estamparlo igual haría que una máquina inalcanzable figure viva para siempre.
//
// Sabotaje: estampar el latido sin mirar res.Error.
func TestUnTierBInalcanzableNoFiguraVivo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"exec"},
		"project": "infra", "address": "router.local",
	}); e != nil {
		t.Fatal(e)
	}
	p := &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "infra",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}

	// Inalcanzable: no se estampa nada.
	restaurar := fleet.SSHFalsoParaTest(t, "echo 'ssh: connect to host router.local port 22: Connection refused' >&2; exit 255")
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "router", "argv": []string{"uptime"}}); e != nil {
		t.Fatal(e)
	}
	restaurar()
	d, _, _ := s.engine.DevicePorNombre("infra", "router")
	if !d.LastSeen.IsZero() {
		t.Fatal("una máquina INALCANZABLE quedó marcada como vista: figuraría viva para siempre")
	}

	// Alcanzable: ahora sí.
	restaurar2 := fleet.SSHFalsoParaTest(t, "exit 0")
	defer restaurar2()
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "router", "argv": []string{"uptime"}}); e != nil {
		t.Fatal(e)
	}
	d2, _, _ := s.engine.DevicePorNombre("infra", "router")
	if d2.LastSeen.IsZero() {
		t.Fatal("se llegó a la máquina y no se estampó la señal de vida")
	}
	if !d2.EnLinea(time.Now(), umbralEnLineaDefault) {
		t.Error("tras alcanzarla, debería figurar en línea")
	}
}

// EL FILTRO `device` DE LA BITÁCORA FILTRA DE VERDAD, Y HACEN FALTA DOS MÁQUINAS PARA SABERLO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, Y POR QUÉ NINGUNA PRUEBA LO VIO
//
// Las tres tools de bitácora —`musubi_fleet_log`, `musubi_fleet_shell_log` y
// `musubi_fleet_sessions`— pasaban `args.Device` (el NOMBRE que tipeó la persona) a un parámetro
// que la capa de memoria llama `deviceID` y que termina en `AND device_id = ?`, un UUID. Nunca
// matchea. `musubi_fleet_log {device:"gio"}` contestaba `total: 0` sobre una máquina que SÍ tiene
// historia.
//
// Es el peor modo de falla que puede tener una bitácora: NO FALLA, miente en el sentido
// tranquilizador — «acá no pasó nada». Y los dos parámetros son `string`, así que cruzarlos no da
// error de compilación; es el mismo defecto que A78 («el inventario vacío»), una capa que dice
// NOMBRE y otra que dice ID sin nada que las ate.
//
// LA PRUEBA NECESITA DOS MÁQUINAS CON HECHOS EN LAS DOS, y eso es todo el diseño. Con UNA sola
// máquina y UN solo comando, filtrar por su nombre y filtrar por nada devuelven lo mismo, así que
// la prueba pasa IGUAL con el filtro roto — que es exactamente por qué esto sobrevivió. Acá se
// exige que el filtro DEJE AFUERA lo de la otra, que es lo único que distingue un filtro que
// funciona de uno que se ignora.
//
// Sabotaje que la hace fallar: volver a pasar `args.Device` en vez del id resuelto.
func TestElFiltroPorMaquinaDeLaBitacoraDejaAfueraLaOtra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	enrolarConExec(t, s, "casa", "nas")

	p := conExec("casa")
	ids := map[string]string{}
	for _, maquina := range []string{"pc-gio", "nas"} {
		res, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
			"device": maquina, "argv": []string{"echo", maquina}, "no_wait": true,
		})
		if e != nil {
			t.Fatalf("exec en %s: %+v", maquina, e)
		}
		id, _ := jsonOf(t, res)["command_id"].(string)
		if id == "" {
			t.Fatalf("no se devolvió command_id para %s", maquina)
		}
		ids[maquina] = id
	}

	// CONTROL POSITIVO: sin filtro están los dos. Sin esto, un fallo del encolado dejaría las dos
	// aserciones de abajo pasando sobre una bitácora vacía.
	todo := textOf(t, llamarComo(t, s, p, "musubi_fleet_log", map[string]any{}))
	for _, maquina := range []string{"pc-gio", "nas"} {
		if !strings.Contains(todo, ids[maquina]) {
			t.Fatalf("sin filtro la bitácora no trae el comando de %s: la prueba no está mirando "+
				"lo que cree\n%s", maquina, todo)
		}
	}

	// Y AHORA EL FILTRO, en las dos direcciones: cada uno trae el suyo y NO trae el de la otra.
	for _, c := range []struct{ pido, ajeno string }{{"pc-gio", "nas"}, {"nas", "pc-gio"}} {
		t.Run(c.pido, func(t *testing.T) {
			crudo := textOf(t, llamarComo(t, s, p, "musubi_fleet_log", map[string]any{"device": c.pido}))
			if !strings.Contains(crudo, ids[c.pido]) {
				t.Errorf("filtrando por %q no aparece SU comando: la bitácora contesta «acá no pasó "+
					"nada» sobre una máquina que sí tiene historia\n%s", c.pido, crudo)
			}
			if strings.Contains(crudo, ids[c.ajeno]) {
				t.Errorf("filtrando por %q apareció el comando de %q: el filtro no filtra",
					c.pido, c.ajeno)
			}
		})
	}

	// UN NOMBRE QUE NO EXISTE ES UN ERROR, NO UNA LISTA VACÍA. Una lista vacía afirma «esa máquina
	// no tuvo comandos» sobre algo que ni se miró — y es indistinguible del defecto que esta
	// prueba cierra.
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_log", map[string]any{"device": "no-existe"}); e == nil {
		t.Error("un nombre inexistente devolvió una bitácora en vez de un error: eso se lee como " +
			"«no pasó nada» y es justo lo que no se puede afirmar")
	}
}

// llamarComo llama una tool COMO un principal y falla si la tool falla. Se llama distinto del
// `mustCall` de methods_codegraph_test.go a propósito: ése no toma principal, y dos helpers con el
// mismo nombre y distinta compuerta es la clase de confusión que este paquete no necesita.
func llamarComo(t *testing.T, s *McpServer, p *Principal, tool string, args map[string]any) interface{} {
	t.Helper()
	res, e := callAsPrincipal(t, s, p, tool, args)
	if e != nil {
		t.Fatalf("%s(%v): %+v", tool, args, e)
	}
	return res
}
