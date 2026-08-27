package mcp

// Pruebas de la puerta del AGENTE para sesiones de shell (S5c).
//
// Por este canal viaja TODO LO QUE LA PERSONA TECLEA, contraseñas de sudo incluidas. La mitad de
// este archivo existe para fijar una sola pregunta: ¿esta sesión es de ESTA máquina?

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// enrolarTierAConShell da de alta una máquina con agente y shell, y devuelve device + token.
func enrolarTierAConShell(t *testing.T, s *McpServer, proyecto, nombre string) (fleet.Device, string) {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics", "exec", "shell"},
		"project": proyecto, "os": "linux",
	})
	if e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	d, _, _ := s.engine.DevicePorNombre(proyecto, nombre)
	return d, tok
}

// LA GUARDA CENTRAL DE S5c: UNA MÁQUINA NO SE ENGANCHA A LA SESIÓN DE OTRA.
//
// Sin ella, cualquier máquina de la flota con un token válido —o sea, cualquiera que alguien
// comprometa— puede pedir las teclas de la sesión abierta en OTRA máquina y leer lo que se
// escribe ahí. Es la peor fuga posible de este track, porque lo que se lee son contraseñas.
//
// Sabotaje que la hace fallar: quitar el `ses.DeviceID != d.ID` de canalDelAgente.
func TestUnaMaquinaNoPuedeEngancharseALaSesionDeOtra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	victima, _ := enrolarTierAConShell(t, s, "casa", "pc-gio")
	_, tokenIntrusa := enrolarTierAConShell(t, s, "casa", "maquina-comprometida")

	// Una sesión abierta en la VÍCTIMA, con su canal de agente esperando.
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: victima.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	canal := fleet.NuevoCanalAgente()
	s.shells.guardar(ses.ID, canal)
	defer s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())

	// La persona teclea algo secreto: está esperando a que lo recoja el agente de la víctima.
	if err := canal.Escribir([]byte("contraseña-de-sudo\n")); err != nil {
		t.Fatal(err)
	}

	h := s.HTTPHandler(httpOptions{reqTimeout: 5 * time.Second})
	pedirTeclas := func(token string) (int, string) {
		r := httptest.NewRequest(http.MethodGet, shellAgenteEntradaPath+"?id="+ses.ID, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}

	codigo, cuerpo := pedirTeclas(tokenIntrusa)
	if codigo != http.StatusGone {
		t.Errorf("la intrusa recibió %d; esperaba 410 — un 500 sería un panic y no una guarda", codigo)
	}
	if codigo < 400 {
		t.Fatalf("FUGA: otra máquina de la flota recogió las teclas de una sesión ajena (HTTP %d, cuerpo %q). Lo que viaja por ahí son contraseñas", codigo, cuerpo)
	}
	if bytes.Contains([]byte(cuerpo), []byte("contraseña-de-sudo")) {
		t.Fatal("FUGA GRAVE: el cuerpo trae lo tecleado en una sesión ajena")
	}
}

// El control positivo del caso de arriba: la máquina DUEÑA sí recoge sus teclas. Sin esto, la
// prueba anterior pasaría también con una ruta que rechaza a todo el mundo.
//
// Sabotaje que la hace fallar: rechazar siempre en canalDelAgente.
func TestLaMaquinaDuenaSiRecogeSusTeclas(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d, token := enrolarTierAConShell(t, s, "casa", "pc-gio")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	canal := fleet.NuevoCanalAgente()
	s.shells.guardar(ses.ID, canal)
	defer s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())
	_ = canal.Escribir([]byte("uptime\n"))

	h := s.HTTPHandler(httpOptions{reqTimeout: 5 * time.Second})
	r := httptest.NewRequest(http.MethodGet, shellAgenteEntradaPath+"?id="+ses.ID, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("la máquina dueña recibió %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "uptime\n" {
		t.Errorf("no llegó lo tecleado: %q", w.Body.String())
	}
	// Y pedir las teclas ES la prueba de que el agente llegó: sin esto, «todavía no vino» sería
	// indistinguible de «vino y no imprime nada».
	if !canal.Enganchado() {
		t.Error("el canal no quedó enganchado tras el primer pedido del agente")
	}
}

// El eje del APARATO también se aplica del lado del agente: una máquina sin `shell` concedida no
// engancha nada, ni aunque el cerebro le haya ofrecido la sesión por error.
//
// Sabotaje que la hace fallar: quitar el chequeo de Permite(CapShell) de deviceDeRequest.
func TestUnaMaquinaSinShellConcedidaNoEnganchaNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"}, // SIN shell
		"project": "casa", "os": "linux"})
	if e != nil {
		t.Fatal(e)
	}
	token, _ := jsonOf(t, res)["token"].(string)
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	s.shells.guardar(ses.ID, fleet.NuevoCanalAgente())
	defer s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())

	h := s.HTTPHandler(httpOptions{reqTimeout: 5 * time.Second})
	r := httptest.NewRequest(http.MethodGet, shellAgenteEntradaPath+"?id="+ses.ID, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("una máquina sin `shell` concedida recibió %d; esperaba 403", w.Code)
	}
}

// Un canal que NO es de agente no se engancha, aunque la sesión sea de esa misma máquina.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PRIMERA VERSIÓN DE ESTA PRUEBA NOMBRABA UNA GUARDA Y EJERCITABA OTRA.
//
// Ponía una sesión de un Tier B y la pedía con el token de un Tier A — pero eso lo rechaza la
// guarda de PERTENENCIA (`ses.DeviceID != d.ID`), que corre antes. O sea que la prueba pasaba con
// la aserción de tipo sacada, porque nunca llegaba a ejecutarse: estaba probando dos veces lo
// mismo y creyendo que probaba dos cosas.
//
// Para aislarla hay que construir el estado imposible a mano: la sesión ES de esta máquina, y su
// canal es un ssh. No puede pasar hoy —abrirCanalShell elige el canal por tier—, y justamente por
// eso la guarda es defensa en profundidad: lo que la haría alcanzable es un bug futuro.
// ────────────────────────────────────────────────────────────────────────────────────────────
//
// Sabotaje que la hace fallar: quitar la aserción de tipo de canalDelAgente.
func TestUnCanalQueNoEsDeAgenteNoSeEngancha(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d, _ := enrolarTierAConShell(t, s, "casa", "pc-gio")
	// La sesión es DE ESTA MÁQUINA: la guarda de pertenencia la deja pasar y la que decide es la
	// aserción de tipo.
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	restaurar := fleet.SSHFalsoParaTest(t, "sleep 5")
	defer restaurar()
	canalSSH, err := fleet.AbrirShellPorSSH("gio@nas.local", 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	s.shells.guardar(ses.ID, canalSSH)
	defer s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())

	// CONTROL POSITIVO: con un canal de agente, la MISMA sesión y el MISMO device sí enganchan.
	// Sin esto, la aserción de abajo pasaría también con una ruta que rechaza todo.
	if _, ok := s.canalDelAgente(d, ses.ID, time.Now()); ok {
		t.Fatal("un canal SSH se entregó como canal de agente: el agente hablaría con un proceso ssh")
	}
	s.shells.guardar(ses.ID, fleet.NuevoCanalAgente())
	if _, ok := s.canalDelAgente(d, ses.ID, time.Now()); !ok {
		t.Fatal("con un canal de agente de verdad tendría que engancharse; el control positivo falla")
	}
	_ = canalSSH.Cerrar()
}

// La salida que sube el agente llega a la persona, y el reloj de inactividad se mueve con ella:
// una sesión donde `tail -f` escupe líneas está viva aunque nadie teclee.
//
// Sabotaje que la hace fallar: no llamar a TocarSesionShell en handlerShellAgenteSalida.
func TestLaSalidaQueSubeElAgenteLlegaYMantieneVivaLaSesion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d, token := enrolarTierAConShell(t, s, "casa", "pc-gio")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	canal := fleet.NuevoCanalAgente()
	s.shells.guardar(ses.ID, canal)
	defer s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())

	// Se envejece el reloj de inactividad a mano, para ver si la salida lo rejuvenece.
	viejo := time.Now().Add(-10 * time.Minute)
	if err := s.engine.TocarSesionShell(ses.ID, viejo); err != nil {
		t.Fatal(err)
	}

	h := s.HTTPHandler(httpOptions{reqTimeout: 5 * time.Second})
	r := httptest.NewRequest(http.MethodPost, shellAgenteSalidaPath+"?id="+ses.ID, bytes.NewReader([]byte("remoto$ ")))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("subir la salida devolvió %d: %s", w.Code, w.Body.String())
	}

	// Llega del lado de la persona.
	datos, err := canal.Leer(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(datos) != "remoto$ " {
		t.Errorf("lo que imprimió el pty no llegó a la persona: %q", datos)
	}
	// Y la sesión se rejuveneció.
	fresca, _, err := s.engine.SesionShellPorID(ses.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !fresca.UltimoTrafico.After(viejo.Add(time.Minute)) {
		t.Error("la salida del pty no movió el reloj de inactividad: una sesión donde `tail -f` escupe líneas se cerraría por inactividad")
	}
}
