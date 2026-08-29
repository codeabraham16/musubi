package mcp

// Pruebas de la mitad del AGENTE del eje de consentimiento (A57), del lado del cerebro: recibir
// la capacidad medida y encolar el aviso que `avisa` promete.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func cuerpoConCapacidad(puede *bool, motivo string) string {
	c := map[string]any{"version": "v0-prueba"}
	if puede != nil {
		c["puede_preguntar"] = *puede
	}
	if motivo != "" {
		c["motivo_no_preguntar"] = motivo
	}
	b, _ := json.Marshal(c)
	return string(b)
}

func boolPtr(b bool) *bool { return &b }

// UN AGENTE VIEJO NO OPINA, Y ESO NO ES «NO PUEDO».
//
// Es la distinción que hace al campo un PUNTERO. Con un bool pelado, un agente que no manda el
// campo sería indistinguible de uno que midió y dijo que no — y como `puede_preguntar` endurece
// un `pide` a `prohibido`, esa confusión cerraría el acceso por pantalla a máquinas que quizás sí
// pueden, sin que nada lo dijera. La primera flota con agentes mezclados se rompería callada.
//
// Sabotaje que la hace fallar: cambiar PuedePreguntar de *bool a bool en cuerpoLatido.
func TestUnAgenteViejoQueNoOpinaNoPisaLaCapacidadMedida(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	id := idDeDevice(t, s, "pc-gio")

	// Un agente NUEVO mide y dice que sí.
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(boolPtr(true), "")); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	d, _, _ := s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Fatal("la capacidad afirmativa no se guardó")
	}

	// Ahora late un agente VIEJO: sin el campo. NO tiene que pisar nada.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(nil, "")); code != http.StatusOK {
		t.Fatal("el latido del agente viejo falló")
	}
	d, _, _ = s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Error("un agente que NO manda el campo borró la capacidad medida: «no opinó» se " +
			"confundió con «no puedo», y un `pide` en esta máquina se endurecería a `prohibido` " +
			"sin que nadie lo haya medido")
	}

	// Y un `false` EXPLÍCITO sí escribe: una máquina que perdió su escritorio tiene que dejar de
	// declarar que puede.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(boolPtr(false), "el agente corre como servicio")); code != http.StatusOK {
		t.Fatal("el latido con false falló")
	}
	d, _, _ = s.engine.DevicePorID(id)
	if d.PuedePreguntar {
		t.Error("un `false` explícito no bajó la capacidad: una máquina que perdió su escritorio " +
			"seguiría prometiendo que puede preguntar")
	}
}

// EL AVISO SE ENCOLA CUANDO EL AGENTE SABE DARLO, y es lo que `avisa` prometía y no entregaba.
//
// Sabotaje que la hace fallar: sacar el `case consent.AvisaAlUsuario()` que encola.
// Sabotaje que la hace fallar: encolar el aviso DESPUÉS de crear la sesión (llega tarde).
func TestConUnAgenteQueSabeAvisarElAvisoSeEncola(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tokenDevice := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	id := idDeDevice(t, s, "pc-gio")

	// El agente declara que sabe avisar, y late con muestra para figurar en línea.
	cuerpo := map[string]any{"version": "v1", "puede_preguntar": true,
		"rustdesk_id": "123456789", "muestra": muestraSana(30, time.Now())}
	b, _ := json.Marshal(cuerpo)
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, string(b)); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	// `avisa` es el grado por defecto, así que no hace falta fijarlo.
	d, _, _ := s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Fatal("la capacidad no se guardó: el resto de la prueba no probaría nada")
	}

	antes := comandosEncolados(t, s)
	principal := conPantalla("casa")
	if _, e := callAsPrincipal(t, s, principal, "musubi_fleet_screen",
		map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}

	nuevos := comandosEncolados(t, s)
	var aviso *fleet.Comando
	for i := range nuevos {
		if len(nuevos[i].Argv) > 0 && nuevos[i].Argv[0] == comandoAviso {
			aviso = &nuevos[i]
		}
	}
	if aviso == nil {
		t.Fatalf("no se encoló ningún aviso (antes %d comandos, ahora %d): `avisa` sigue "+
			"prometiendo una notificación que no viaja", len(antes), len(nuevos))
	}
	// EL TEXTO NOMBRA A QUIEN ENTRA. «Alguien está viendo tu pantalla» no le sirve a nadie: sin
	// el nombre no hay a quién preguntarle, y el aviso se vuelve ruido que se cierra sin leer.
	if len(aviso.Argv) < 2 {
		t.Fatalf("el aviso viajó sin texto: %#v", aviso.Argv)
	}
	if !strings.Contains(aviso.Argv[1], "mirador") {
		t.Errorf("el texto del aviso no nombra a quien entra: %q", aviso.Argv[1])
	}
	if !strings.Contains(aviso.Argv[1], "pantalla") {
		t.Errorf("el texto no dice QUÉ está pasando: %q", aviso.Argv[1])
	}
}

// SIN AGENTE QUE SEPA AVISAR, LA PANTALLA SE ABRE IGUAL Y NO SE ENCOLA NADA.
//
// `avisa` NO bloquea —ése es el grado siguiente— y encolar un aviso que nadie va a poder mostrar
// dejaría un comando pendiente para siempre en la cola de esa máquina, que además es ruido en la
// bitácora. Se deja la constancia en el log y listo.
//
// Sabotaje que la hace fallar: encolar el aviso sin mirar PuedePreguntar.
func TestSinCapacidadDeAvisarNoSeEncolaUnAvisoQueNadieVaAMostrar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tokenDevice := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	cuerpo := map[string]any{"version": "v1", "puede_preguntar": false,
		"motivo_no_preguntar": "el agente corre como servicio de sistema",
		"rustdesk_id":         "987654321", "muestra": muestraSana(30, time.Now())}
	b, _ := json.Marshal(cuerpo)
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, string(b)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	if _, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen",
		map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("la pantalla NO se abrió, y `avisa` no bloquea: %+v", e)
	}
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
			t.Error("se encoló un aviso que esta máquina no puede mostrar: queda pendiente para " +
				"siempre en su cola y ensucia la bitácora")
		}
	}
}

// ── `pide`: el camino de ida y vuelta ────────────────────────────────────────────────────────

// maquinaQuePide arma una máquina con `screen`, agente que sabe preguntar, y consentimiento
// `pide`. Devuelve el servidor y el token del dispositivo.
func maquinaQuePide(t *testing.T) (*McpServer, *httptest.Server, string) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	cuerpo, _ := json.Marshal(map[string]any{"version": "v1", "puede_preguntar": true,
		"rustdesk_id": "111222333", "muestra": muestraSana(30, time.Now())})
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tok, string(cuerpo)); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, b)
	}
	if _, e := call(t, s, "musubi_fleet_consent",
		map[string]any{"device": "pc-gio", "grado": "pide", "project": "casa"}); e != nil {
		t.Fatalf("fleet_consent: %+v", e)
	}
	return s, ts, tok
}

// UN `pide` NO DEVUELVE CONTRASEÑA, Y NO BLOQUEA.
//
// El latido va cada 30 s y el diálogo espera 60: una respuesta tarda hasta minuto y medio. Colgar
// la llamada pondría un timeout de red en el camino de una decisión humana, donde el vencimiento
// significa otra cosa. Y sobre todo: NO SE ACUÑA CONTRASEÑA todavía — una credencial que existe
// es una credencial que se puede filtrar, aunque nadie la use, y no se sabe si van a decir que sí.
//
// Sabotaje que la hace fallar: seguir el camino normal cuando el consentimiento es `pide`.
// Sabotaje que la hace fallar: acuñar la contraseña antes de preguntar.
func TestUnPideDevuelveLaEsperaYNoUnaContrasena(t *testing.T) {
	s, _, _ := maquinaQuePide(t)

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen",
		map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el pedido falló en vez de esperar: %+v", e)
	}
	out := jsonOf(t, res)
	if out["password"] != nil {
		t.Error("se acuñó y devolvió una contraseña ANTES de que nadie diera permiso: una " +
			"credencial que existe se puede filtrar aunque no se use")
	}
	if out["estado"] != string(fleet.SesionEsperandoPermiso) {
		t.Errorf("estado = %v, esperaba %q", out["estado"], fleet.SesionEsperandoPermiso)
	}
	if out["session_id"] == nil {
		t.Error("no devolvió el id de la sesión: el operador no tiene con qué volver")
	}

	// Y se le encoló la PREGUNTA al agente, no un aviso.
	var pregunta *fleet.Comando
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == comandoPreguntar {
			cc := c
			pregunta = &cc
		}
	}
	if pregunta == nil {
		t.Fatal("no se encoló ninguna pregunta: el `pide` no llegó a la máquina")
	}
	if len(pregunta.Argv) < 3 || pregunta.Argv[1] != out["session_id"] {
		t.Errorf("la pregunta no lleva la sesión que se devolvió: %#v", pregunta.Argv)
	}
	if !strings.Contains(pregunta.Argv[2], "mirador") {
		t.Errorf("la pregunta no nombra a quien pide: %q", pregunta.Argv[2])
	}
}

// EL CIRCUITO COMPLETO: se pregunta, el usuario dice que SÍ, y el operador vuelve y recibe la
// contraseña.
//
// Sabotaje que la hace fallar: no registrar la respuesta en registrarRespuestaDePermiso.
// Sabotaje que la hace fallar: que `concedida` cierre la sesión en vez de dejarla `solicitada`.
func TestElCircuitoCompletoDeUnPideConcedido(t *testing.T) {
	s, ts, tokDev := maquinaQuePide(t)
	p := conPantalla("casa")

	res, _ := callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	sesID, _ := jsonOf(t, res)["session_id"].(string)

	// El agente recoge la pregunta y contesta que sí.
	cmdID := idDelComando(t, s, comandoPreguntar)
	responder(t, ts, tokDev, cmdID, prefijoRespuestaPermiso+string(fleet.RespuestaConcedida))

	// El operador vuelve: AHORA sí hay contraseña.
	res2, e := callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("con el permiso concedido, la pantalla no se abrió: %+v", e)
	}
	out := jsonOf(t, res2)
	if out["password"] == nil || out["password"] == "" {
		t.Fatal("dijeron que sí y no vino la contraseña")
	}
	if out["session_id"] != sesID {
		t.Errorf("se abrió una sesión NUEVA (%v) en vez de reusar la que tenía el permiso (%v): "+
			"la concedida queda colgada y la bitácora muestra dos filas para un solo permiso",
			out["session_id"], sesID)
	}
	// Y SE DICE que hubo consentimiento: es la diferencia entre entrar a una máquina y entrar con
	// el permiso de quien la está usando.
	if out["consentimiento"] != string(fleet.RespuestaConcedida) {
		t.Errorf("la respuesta no dice que se concedió el permiso: %v", out["consentimiento"])
	}

	// LA SESIÓN CONCEDIDA QUEDA ABIERTA, no cerrada. Un `cerrada` puesto al conceder haría que el
	// panel de sesiones vivas muestre como TERMINADA una pantalla que alguien está usando en ese
	// momento — y ese panel existe justamente para poder ver quién está adentro ahora.
	sesiones, err := s.engine.SesionesDePantalla("casa", "", 5, time.Now())
	if err != nil || len(sesiones) == 0 {
		t.Fatalf("no se pudo leer la bitácora: %v", err)
	}
	var concedida *fleet.SesionPantalla
	for i := range sesiones {
		if sesiones[i].ID == sesID {
			concedida = &sesiones[i]
		}
	}
	if concedida == nil {
		t.Fatalf("la sesión %q no está en la bitácora", sesID)
	}
	if !concedida.Cerrada.IsZero() {
		t.Errorf("la sesión concedida quedó marcada como CERRADA (%s): el panel de sesiones "+
			"vivas la mostraría terminada mientras alguien la está usando", concedida.Cerrada)
	}
	if concedida.Estado != fleet.SesionSolicitada {
		t.Errorf("estado = %q; una concesión deja la sesión donde estaría si nunca hubiera hecho "+
			"falta preguntar, para que el permiso y la credencial sigan siendo dos pasos",
			concedida.Estado)
	}
}

// LOS TRES «NO» SE DISTINGUEN, Y CADA UNO DICE QUÉ HACER.
//
// «Me dijeron que no» es una decisión que hay que respetar; «nadie contestó» dice que esa máquina
// quizás no debería estar en `pide`; «no había con qué preguntar» dice que le falta software o le
// sobra aislamiento. Los tres cierran, y confundirlos manda a arreglar la cosa equivocada.
//
// Sabotaje que la hace fallar: guardar la respuesta en `error` en vez de en su columna.
// Sabotaje que la hace fallar: devolver el mismo mensaje para los tres.
func TestLosTresNoSeDistinguenYCadaUnoDiceQueHacer(t *testing.T) {
	casos := []struct {
		respuesta fleet.RespuestaAviso
		enElTexto string
	}{
		{fleet.RespuestaNegada, "dijo que NO"},
		{fleet.RespuestaSinRespuesta, "nadie contestó"},
		{fleet.RespuestaNoSePudo, "no tuvo con qué preguntar"},
	}
	for _, c := range casos {
		t.Run(string(c.respuesta), func(t *testing.T) {
			s, ts, tokDev := maquinaQuePide(t)
			p := conPantalla("casa")
			callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
			cmdID := idDelComando(t, s, comandoPreguntar)
			responder(t, ts, tokDev, cmdID, prefijoRespuestaPermiso+string(c.respuesta))

			_, e := callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
			if e == nil {
				t.Fatalf("con la respuesta %q la pantalla se abrió igual", c.respuesta)
			}
			if !strings.Contains(e.Message, c.enElTexto) {
				t.Errorf("el mensaje no distingue este caso.\n  esperaba que dijera: %q\n  dijo: %s",
					c.enElTexto, e.Message)
			}
			// Y la bitácora conserva CUÁL de los tres fue, en su columna.
			sesiones, err := s.engine.SesionesDePantalla("casa", "", 5, time.Now())
			if err != nil || len(sesiones) == 0 {
				t.Fatalf("no se pudo leer la bitácora: %v", err)
			}
			if sesiones[0].Consentimiento != c.respuesta {
				t.Errorf("la bitácora guardó %q y la respuesta fue %q: la distinción entre los "+
					"tres «no» se perdió", sesiones[0].Consentimiento, c.respuesta)
			}
		})
	}
}

// UNA MÁQUINA NO PUEDE CONTESTARLE A LA PREGUNTA QUE SE LE HIZO A OTRA.
//
// Es la guarda que evita que comprometer una máquina cualquiera de la flota alcance para
// conseguir permiso de entrar a la pantalla de otra.
//
// Sabotaje que la hace fallar: sacarle el `AND device_id = ?` a ResponderConsentimiento.
func TestUnaMaquinaNoPuedeContestarPorOtra(t *testing.T) {
	s, ts, _ := maquinaQuePide(t)
	// Una segunda máquina, con su propio token.
	tokAjeno := enrolarConPantalla(t, s, "casa", "otra-maquina")

	callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	cmdID := idDelComando(t, s, comandoPreguntar)

	// La OTRA máquina intenta contestar «concedida» por la sesión de pc-gio.
	//
	// SE ESPERA UN 403 Y NO UN 200, y eso muestra que la defensa es doble: la puerta de
	// /fleet/result ya rechaza un comando que no es de esta máquina, ANTES de que la respuesta
	// llegue a tocar la sesión. La comprobación de abajo verifica la segunda capa —que aunque esa
	// puerta se abriera, el permiso tampoco se concedería—, que es la que sobrevive si alguien
	// mañana reordena el handler.
	cero := 0
	b, _ := json.Marshal(map[string]any{"command_id": cmdID, "exit_code": &cero,
		"stdout": prefijoRespuestaPermiso + string(fleet.RespuestaConcedida)})
	code, cuerpo := postCon(t, ts.URL+fleetResultPath, tokAjeno, string(b))
	if code == http.StatusOK {
		t.Errorf("la puerta de /fleet/result aceptó un comando de OTRA máquina (%s): la primera "+
			"capa de defensa se abrió", cuerpo)
	}

	sesiones, _ := s.engine.SesionesDePantalla("casa", "", 5, time.Now())
	for _, ses := range sesiones {
		if ses.Consentimiento.Concede() {
			t.Fatal("una máquina ajena concedió el permiso de otra: comprometer cualquier " +
				"máquina de la flota alcanzaría para entrar a la pantalla de todas")
		}
	}
}

// idDelComando busca el id del último comando encolado de un tipo.
func idDelComando(t *testing.T, s *McpServer, op string) string {
	t.Helper()
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == op {
			return c.ID
		}
	}
	t.Fatalf("no se encontró ningún comando %q en la bitácora", op)
	return ""
}

// responder simula al agente reportando el resultado de un comando.
func responder(t *testing.T, ts *httptest.Server, token, comandoID, stdout string) {
	t.Helper()
	cero := 0
	b, _ := json.Marshal(map[string]any{"command_id": comandoID, "exit_code": &cero, "stdout": stdout})
	if code, body := postCon(t, ts.URL+fleetResultPath, token, string(b)); code != http.StatusOK {
		t.Fatalf("el reporte del agente devolvió %d: %s", code, body)
	}
}
