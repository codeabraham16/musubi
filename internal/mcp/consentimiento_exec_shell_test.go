package mcp

// Pruebas de A75: el eje de consentimiento gateando las OTRAS DOS puertas —exec y shell—, que
// hasta acá no lo consultaban.
//
// El agujero que cierran: en una máquina marcada `pide`, mirar la pantalla preguntaba y abrir una
// TERMINAL en la misma máquina no preguntaba nada. Una terminal es más invasiva que mirar, no
// menos: quien tiene un prompt lee los mismos archivos y además los escribe.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// maquinaConGrado da de alta un Tier A con metrics+exec+shell, le fija el grado de consentimiento
// y declara si su agente sabe preguntar. Devuelve el servidor, el device y el token de la máquina.
//
// LA CAPACIDAD DE PREGUNTAR SE FIJA APARTE del grado porque son dos cosas distintas: el grado es
// la POLÍTICA (la escribe quien administra) y `puede_preguntar` es la CAPACIDAD MEDIDA (la reporta
// el agente). Cruzarlas es justamente lo que hace ConsentimientoEfectivo.
func maquinaConGrado(t *testing.T, grado fleet.Consentimiento, puedePreguntar bool) (*McpServer, fleet.Device, string) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	d, tok := enrolarTierAConShell(t, s, "casa", "pc-gio")
	if !d.Permite(fleet.CapShell) || !d.Permite(fleet.CapExec) {
		t.Fatal("la máquina de prueba no admite `shell` y `exec`: el rechazo vendría de la " +
			"compuerta de capacidad y la prueba pasaría por el motivo equivocado")
	}
	if err := s.engine.FijarCapacidadDePreguntar(d.ID, puedePreguntar); err != nil {
		t.Fatal(err)
	}
	if _, err := s.engine.FijarConsentimiento(d.ID, grado); err != nil {
		t.Fatal(err)
	}
	d, _, _ = s.engine.DevicePorNombre("casa", "pc-gio")
	return s, d, tok
}

// ── (1) y (2): `prohibido` pesa más que la concesión, en las DOS puertas nuevas ──────────────

// UNA MÁQUINA CON EL ACCESO PROHIBIDO NO DA UNA SHELL, AUNQUE LA CONCESIÓN ESTÉ BIEN.
//
// Es la gemela de TestElConsentimientoProhibidoPesaMasQueLaCapacidadConcedida, que probaba lo
// mismo para la pantalla y era la ÚNICA puerta gateada. El principal tiene `shell` sobre esa
// máquina —perfectamente concedido— y no hay prompt, porque el dueño de la máquina dijo que no.
// Un permiso del administrador no puede más que el candado de quien usa el equipo.
//
// Y el mensaje tiene que mandar a mirar el lugar correcto: si dijera «sin permiso» a secas,
// alguien revisaría `principals.yaml` durante media hora buscando algo que ya está.
//
// Sabotaje que la hace fallar: sacar la llamada a aplicarConsentimiento de toolFleetShell.
func TestUnaMaquinaProhibidaNoDaShellAunqueLaConcesionEsteBien(t *testing.T) {
	s, _, _ := maquinaConGrado(t, fleet.ConsentimientoProhibido, true)

	_, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e == nil {
		t.Fatal("se abrió una shell en una máquina con el acceso PROHIBIDO: el candado del dueño " +
			"no pesó nada, y una terminal es más invasiva que la pantalla que sí estaba gateada")
	}
	if !strings.Contains(e.Message, "consentimiento") {
		t.Errorf("el error no manda a mirar el consentimiento, así que manda a revisar permisos "+
			"que ya están: %s", e.Message)
	}

	// Y NO QUEDÓ NADA ENCOLADO NI REGISTRADO: la guarda va antes de escribir la bitácora, así que
	// `prohibido` no deja ni el intento. Una fila «abriendo» sobre una máquina donde nadie puede
	// entrar es una sesión fantasma en el panel de quién está adentro.
	sesiones, err := s.engine.BitacoraDeShell("casa", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sesiones) != 0 {
		t.Errorf("quedaron %d sesiones de shell registradas sobre una máquina prohibida: %+v",
			len(sesiones), sesiones[0])
	}
}

// LO MISMO PARA exec: `prohibido` no ejecuta, aunque la credencial pueda ejecutar cualquier cosa.
//
// Es la prueba central de A75 del lado de exec, y la que el sabotaje de la tarea apunta: quitar la
// consulta de consentimiento de toolFleetExec la pone roja.
//
// Sabotaje que la hace fallar: sacar la llamada a aplicarConsentimiento de toolFleetExec.
func TestUnaMaquinaProhibidaNoEjecutaAunqueLaConcesionEsteBien(t *testing.T) {
	s, _, _ := maquinaConGrado(t, fleet.ConsentimientoProhibido, true)

	_, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"uptime"}, "no_wait": true})
	if e == nil {
		t.Fatal("se ejecutó un comando en una máquina con el acceso PROHIBIDO: el candado del " +
			"dueño de la máquina no pesó nada")
	}
	if !strings.Contains(e.Message, "consentimiento") {
		t.Errorf("el error no manda a mirar el consentimiento: %s", e.Message)
	}

	// NADA SE ENCOLÓ. F1 escribe la bitácora antes de ejecutar, y esa fila ES el pedido que el
	// agente va a levantar: si la guarda quedara después del encolado, el comando correría igual
	// en la máquina donde el dueño dijo que no se entra, y el error de arriba sería decorativo.
	for _, c := range comandosEncolados(t, s) {
		t.Fatalf("quedó un comando encolado sobre una máquina prohibida: %v", c.Argv)
	}
}

// ── (3) `pide` en exec: se endurece a `prohibido`, y la pantalla de la misma máquina sí pregunta ─

// EN UN exec, `pide` SE ENDURECE A `prohibido` PORQUE NO HAY DÓNDE VOLVER A BUSCAR EL SÍ.
//
// Un `pide` se resuelve en dos pedidos: uno pregunta y otro recoge la respuesta. Eso funciona para
// una SESIÓN —algo que dura y a lo que se vuelve— y no para un comando de una sola vez: partirlo
// dejaría un argv esperando en la base con un permiso que vence antes que el comando, que es una
// política con otro nombre y las políticas ya existen (S10).
//
// La prueba fija las dos mitades: exec se niega, Y la MISMA máquina sigue preguntando por la
// pantalla. Sin la segunda, un endurecimiento global de `pide` pasaría la prueba y rompería el
// eje entero.
//
// Sabotaje que la hace fallar: en consentimientoParaExec, devolver d.ConsentimientoEfectivo() sin
// el .AplicarACapacidadDePreguntar(false).
func TestUnPideEnExecSeEndureceAProhibidoPorqueNoHayAQuienPreguntar(t *testing.T) {
	s, d, _ := maquinaConGrado(t, fleet.ConsentimientoPide, true)
	// El agente SÍ sabe preguntar: sin esto el endurecimiento vendría de ConsentimientoEfectivo y
	// la prueba pasaría por el motivo equivocado.
	if d.ConsentimientoEfectivo() != fleet.ConsentimientoPide {
		t.Fatalf("el efectivo es %q y tiene que ser `pide`: la prueba no ejercitaría el caso",
			d.ConsentimientoEfectivo())
	}

	_, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"uptime"}, "no_wait": true})
	if e == nil {
		t.Fatal("un exec sobre una máquina en `pide` corrió sin que nadie diera permiso: la " +
			"salida cómoda («nadie puede negarse, entonces adelante») es al revés de lo que " +
			"quiso decir quien escribió `pide`")
	}

	// Y EL ENDURECIMIENTO ES SÓLO DE exec: para el resto de las puertas esta máquina sigue en
	// `pide`, o sea que sigue preguntando. Sin esta mitad, endurecer `pide` en TODO el eje pasaría
	// la prueba de arriba y rompería el eje entero — que es el modo de fallo que importa.
	if consentimientoParaExec(d) != fleet.ConsentimientoProhibido {
		t.Errorf("consentimientoParaExec = %q, esperaba `prohibido`", consentimientoParaExec(d))
	}
	if d.ConsentimientoEfectivo() != fleet.ConsentimientoPide {
		t.Errorf("el grado efectivo de la máquina quedó en %q: el endurecimiento se derramó "+
			"del exec al eje entero, y la pantalla y la shell dejarían de preguntar",
			d.ConsentimientoEfectivo())
	}

	// Y LA SHELL DE LA MISMA MÁQUINA SÍ PREGUNTA, que es la prueba de que la asimetría es
	// deliberada y no un endurecimiento global disfrazado.
	res, e2 := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell",
		map[string]any{"device": "pc-gio"})
	if e2 != nil {
		t.Fatalf("la shell dejó de preguntar: %+v", e2)
	}
	if out := jsonOf(t, res); out["estado"] != string(fleet.ShellEsperandoPermiso) {
		t.Errorf("la shell no quedó esperando permiso: %v", out["estado"])
	}
}

// ── El circuito de `pide` en la shell, calcado del de pantalla ───────────────────────────────

// UN `pide` EN LA SHELL PREGUNTA, NO BLOQUEA, Y NO ABRE NINGÚN CANAL TODAVÍA.
//
// El latido va cada 30 s y el diálogo espera 60: una respuesta tarda hasta minuto y medio. Colgar
// la llamada pondría un timeout de red en el camino de una decisión humana. Y sobre todo: NO se
// abre canal —un pty reservado o un SSH prendido ya son acceso a la máquina— cuando todavía no se
// sabe si la respuesta va a ser sí.
//
// Sabotaje que la hace fallar: seguir el camino normal cuando el consentimiento es `pide` (quitar
// el `if consent == fleet.ConsentimientoPide` de toolFleetShell).
func TestUnPideEnShellDevuelveLaEsperaYNoUnPrompt(t *testing.T) {
	s, _, _ := maquinaConGrado(t, fleet.ConsentimientoPide, true)

	res, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el pedido falló en vez de esperar: %+v", e)
	}
	out := jsonOf(t, res)
	if out["estado"] != string(fleet.ShellEsperandoPermiso) {
		t.Errorf("estado = %v, esperaba %q", out["estado"], fleet.ShellEsperandoPermiso)
	}
	sesID, _ := out["session_id"].(string)
	if sesID == "" {
		t.Fatal("no devolvió el id de la sesión: el operador no tiene con qué volver")
	}
	if _, hay := s.shells.buscar(sesID); hay {
		t.Error("se abrió el canal ANTES de que nadie diera permiso: un pty reservado del otro " +
			"lado ya es acceso a la máquina")
	}

	// LA PREGUNTA DICE QUE ES UNA TERMINAL. Quien contesta tiene que poder decidir, y mirar una
	// pantalla y tener un prompt no son el mismo permiso para quien está sentado ahí.
	var pregunta *fleet.Comando
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == comandoPreguntar {
			cc := c
			pregunta = &cc
		}
	}
	if pregunta == nil {
		t.Fatal("no se encoló ninguna pregunta: el `pide` de la shell no llegó a la máquina")
	}
	if len(pregunta.Argv) < 3 || pregunta.Argv[1] != sesID {
		t.Fatalf("la pregunta no lleva la sesión que se devolvió: %#v", pregunta.Argv)
	}
	if !strings.Contains(pregunta.Argv[2], "terminal") {
		t.Errorf("la pregunta no dice QUÉ se está pidiendo, así que no se puede contestar: %q",
			pregunta.Argv[2])
	}
	if !strings.Contains(pregunta.Argv[2], "op") {
		t.Errorf("la pregunta no nombra a quien pide: %q", pregunta.Argv[2])
	}

	// UN PEDIDO DE PERMISO NO ES UNA SESIÓN ABIERTA: el panel de quién está adentro no puede
	// dibujar a nadie en esta máquina todavía.
	viva := fleet.DesdeSesionShell(fleet.SesionShell{
		ID: sesID, Estado: fleet.ShellEsperandoPermiso,
		Vence: time.Now().Add(time.Minute)}, "pc-gio")
	if viva.Abierta(time.Now()) {
		t.Error("un `esperando_permiso` figura como sesión ABIERTA: el panel diría que alguien " +
			"tiene un prompt en una máquina donde nadie dio permiso")
	}
}

// EL CIRCUITO COMPLETO: se pregunta, quien usa la máquina dice que SÍ, y el operador vuelve y
// recibe el prompt — reusando la MISMA fila, no una nueva.
//
// Sabotaje que la hace fallar: en registrarRespuestaDePermiso, mandar la respuesta siempre a
// ResponderConsentimiento (pantalla) en vez de dejar que responderConsentimientoDeLaSesion elija
// la tabla por el id.
// Sabotaje que la hace fallar: que `concedida` deje la fila en `abriendo` en vez de `permitida`
// (T7 la devolvería como «ya tenías una sesión abierta» y nadie recibiría un prompt).
func TestElCircuitoCompletoDeUnPideConcedidoEnShell(t *testing.T) {
	s, _, tokDev := maquinaConGrado(t, fleet.ConsentimientoPide, true)
	ts := servidorHTTP(t, s)
	p := conShell("casa")

	res, e := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el primer pedido falló: %+v", e)
	}
	sesID, _ := jsonOf(t, res)["session_id"].(string)

	// El agente recoge la pregunta y contesta que sí.
	cmdID := idDelComando(t, s, comandoPreguntar)
	responder(t, ts, tokDev, cmdID, prefijoRespuestaPermiso+string(fleet.RespuestaConcedida))

	// El operador vuelve: AHORA sí hay prompt.
	res2, e2 := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e2 != nil {
		t.Fatalf("con el permiso concedido, la shell no se abrió: %+v", e2)
	}
	out := jsonOf(t, res2)
	if out["session_id"] != sesID {
		t.Errorf("se abrió una sesión NUEVA (%v) en vez de reusar la que tenía el permiso (%v): "+
			"la concedida queda colgada y la bitácora muestra dos filas para un solo permiso",
			out["session_id"], sesID)
	}
	if out["estado"] != string(fleet.ShellAbriendo) && out["estado"] != string(fleet.ShellActiva) {
		t.Errorf("estado = %v: la fila con el permiso tiene que quedar como una sesión de verdad", out["estado"])
	}
	if _, hay := s.shells.buscar(sesID); !hay {
		t.Error("no hay canal: dijeron que sí y no se conectó nada")
	}
	t.Cleanup(func() { s.cerrarShell(sesID, fleet.ShellCerrada, "fin de la prueba", time.Now()) })

	// LA BITÁCORA CONSERVA QUE HUBO CONSENTIMIENTO, en su columna. Es lo que separa «entré a una
	// máquina» de «entré con el permiso de quien la está usando».
	ses, hay, err := s.engine.SesionShellPorID(sesID)
	if err != nil || !hay {
		t.Fatalf("no se pudo leer la sesión: %v", err)
	}
	if ses.Consentimiento != fleet.RespuestaConcedida {
		t.Errorf("la bitácora guardó %q: la respuesta de la persona no sobrevivió", ses.Consentimiento)
	}
	if !ses.Cerrada.IsZero() {
		t.Errorf("la sesión concedida quedó CERRADA (%s): el panel la mostraría terminada "+
			"mientras alguien la está usando", ses.Cerrada)
	}

	// EL VENCIMIENTO PASA A SER EL DE UNA SHELL Y NO EL DEL PEDIDO. La fila venía con la ventana
	// de tres minutos del permiso; dejársela mataría la terminal a los dos minutos y medio por un
	// vencimiento que ya no significa nada.
	if ses.Vence.Sub(time.Now()) < fleet.VentanaDePermiso*2 {
		t.Errorf("la sesión vence en %s: se quedó con la ventana del PEDIDO en vez del techo de "+
			"vida de una shell", ses.Vence.Sub(time.Now()))
	}
}

// UN SÍ VALE PARA UN PROMPT, NO PARA TODOS LOS QUE VENGAN DESPUÉS.
//
// El permiso se consume una vez. Sin eso, la persona que dijo «dale» una vez habilitaría terminales
// en su máquina por tiempo indefinido, que es exactamente lo que no aceptó.
//
// Sabotaje que la hace fallar: sacarle el `AND estado = 'permitida'` a
// ReanudarSesionShellPermitida.
func TestUnPermisoDeShellSeConsumeUnaSolaVez(t *testing.T) {
	s, _, tokDev := maquinaConGrado(t, fleet.ConsentimientoPide, true)
	ts := servidorHTTP(t, s)
	p := conShell("casa")

	res, _ := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	sesID, _ := jsonOf(t, res)["session_id"].(string)
	responder(t, ts, tokDev, idDelComando(t, s, comandoPreguntar),
		prefijoRespuestaPermiso+string(fleet.RespuestaConcedida))

	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("el primer uso del permiso falló: %+v", e)
	}
	// La sesión queda VIVA, así que T7 la devuelve: se la cierra para forzar el segundo pedido a
	// pasar de nuevo por el permiso, que es lo que esta prueba mide.
	s.cerrarShell(sesID, fleet.ShellCerrada, "el operador la cerró", time.Now())

	res3, e3 := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e3 != nil {
		t.Fatalf("el segundo pedido tendría que volver a PREGUNTAR, no fallar: %+v", e3)
	}
	out := jsonOf(t, res3)
	if out["estado"] != string(fleet.ShellEsperandoPermiso) {
		t.Errorf("estado = %v: el segundo pedido reusó un permiso ya gastado en vez de volver a "+
			"preguntar", out["estado"])
	}
	if out["session_id"] == sesID {
		t.Error("el segundo pedido devolvió la MISMA fila: el sí de una persona se estiró a un " +
			"segundo prompt")
	}
}

// LOS TRES «NO» CIERRAN LA SHELL Y CADA UNO DICE QUÉ HACER.
//
// «Me dijeron que no» es una decisión que hay que respetar; «nadie contestó» dice que esa máquina
// quizás no debería estar en `pide`; «no había con qué preguntar» dice que le falta software o le
// sobra aislamiento. Los tres niegan, y confundirlos manda a arreglar la cosa equivocada.
//
// Sabotaje que la hace fallar: hacer que ConcedeElAcceso de SesionShell devuelva true cuando el
// consentimiento no está vacío (o sea, `!= negada` en vez de la lista blanca de RespuestaAviso).
func TestLosTresNoDeLaShellSeDistinguenYCadaUnoDiceQueHacer(t *testing.T) {
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
			s, _, tokDev := maquinaConGrado(t, fleet.ConsentimientoPide, true)
			ts := servidorHTTP(t, s)
			p := conShell("casa")

			if _, e := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"}); e != nil {
				t.Fatalf("el pedido falló: %+v", e)
			}
			responder(t, ts, tokDev, idDelComando(t, s, comandoPreguntar),
				prefijoRespuestaPermiso+string(c.respuesta))

			_, e := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
			if e == nil {
				t.Fatalf("con la respuesta %q la shell se abrió igual", c.respuesta)
			}
			if !strings.Contains(e.Message, c.enElTexto) {
				t.Errorf("el mensaje no distingue este caso.\n  esperaba que dijera: %q\n  dijo: %s",
					c.enElTexto, e.Message)
			}
			// Y el mensaje habla de una SHELL, no de una pantalla: mandar a mirar el eje correcto
			// por el camino equivocado es la mitad de un buen error.
			if !strings.Contains(e.Message, "shell") {
				t.Errorf("el error de una shell negada habla de otra cosa: %s", e.Message)
			}
			// La distinción sobrevive en su columna, que es lo que la separa de un texto libre.
			sesiones, err := s.engine.BitacoraDeShell("casa", "", 5)
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

// ── `avisa`: la deuda con quien está en la máquina vale para las tres puertas ────────────────

// `avisa` ENCOLA EL AVISO TAMBIÉN EN SHELL Y EN exec, Y EL TEXTO DICE QUÉ ESTÁ PASANDO.
//
// `avisa` es el grado por DEFECTO de toda la flota, así que ésta es la puerta por la que pasa la
// mayoría de los accesos. Un aviso que no distingue una pantalla mirada de una terminal abierta no
// le sirve para decidir nada a quien lo lee: son riesgos distintos y piden reacciones distintas.
//
// Sabotaje que la hace fallar: sacar el `case consent.AvisaAlUsuario()` que encola en
// aplicarConsentimiento.
func TestUnAvisaLeAvisaAQuienUsaLaMaquinaTambienEnShellYEnExec(t *testing.T) {
	casos := []struct {
		tool     string
		args     map[string]any
		p        func(string) *Principal
		enTexto  string
		modalida string
	}{
		{"musubi_fleet_shell", map[string]any{"device": "pc-gio"}, conShell, "terminal", "shell"},
		{"musubi_fleet_exec", map[string]any{"device": "pc-gio", "argv": []string{"uptime"}, "no_wait": true},
			conExec, "comando", "exec"},
	}
	for _, c := range casos {
		t.Run(c.modalida, func(t *testing.T) {
			s, _, _ := maquinaConGrado(t, fleet.ConsentimientoAvisa, true)
			if _, e := callAsPrincipal(t, s, c.p("casa"), c.tool, c.args); e != nil {
				t.Fatalf("`avisa` NO bloquea, y bloqueó: %+v", e)
			}
			var aviso *fleet.Comando
			for _, cmd := range comandosEncolados(t, s) {
				if len(cmd.Argv) > 0 && cmd.Argv[0] == comandoAviso {
					cc := cmd
					aviso = &cc
				}
			}
			if aviso == nil {
				t.Fatalf("no se encoló ningún aviso: `avisa` sigue prometiendo, en %s, una "+
					"notificación que no viaja", c.modalida)
			}
			if len(aviso.Argv) < 2 {
				t.Fatalf("el aviso viajó sin texto: %#v", aviso.Argv)
			}
			if !strings.Contains(aviso.Argv[1], c.enTexto) {
				t.Errorf("el texto no dice QUÉ está pasando (esperaba %q): %q", c.enTexto, aviso.Argv[1])
			}
			if !strings.Contains(aviso.Argv[1], "op") {
				t.Errorf("el texto no nombra a quien entra: %q", aviso.Argv[1])
			}
		})
	}
}

// SIN UN AGENTE QUE SEPA AVISAR, EL ACCESO SIGUE Y NO SE ENCOLA NADA.
//
// `avisa` NO bloquea —ése es el grado siguiente— y encolar un aviso que nadie va a poder mostrar
// dejaría un comando pendiente para siempre en la cola de esa máquina. La constancia queda en el
// log del cerebro y listo. Es el caso de casi toda la flota real: un servidor sin escritorio.
//
// Sabotaje que la hace fallar: encolar el aviso sin mirar PuedePreguntar en aplicarConsentimiento.
func TestSinCapacidadDeAvisarLaShellSeAbreIgualYNoSeEncolaNada(t *testing.T) {
	s, _, _ := maquinaConGrado(t, fleet.ConsentimientoAvisa, false)

	res, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("la shell NO se abrió, y `avisa` no bloquea: %+v", e)
	}
	if sesID, _ := jsonOf(t, res)["session_id"].(string); sesID != "" {
		t.Cleanup(func() { s.cerrarShell(sesID, fleet.ShellCerrada, "fin de la prueba", time.Now()) })
	}
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
			t.Error("se encoló un aviso que esta máquina no puede mostrar: queda pendiente para " +
				"siempre en su cola y ensucia la bitácora")
		}
	}
}

// `libre` NO AVISA NADA, Y ES EL GRADO DE UN SERVIDOR.
//
// Es la otra mitad de la documentación de A75: los servidores van en `libre`, y eso tiene que
// significar silencio de verdad. Si `libre` encolara un aviso, la única forma de no molestar a
// nadie sería no configurar el eje — y un default que enseña a apagar la protección es peor que
// no tenerla.
//
// Sabotaje que la hace fallar: cambiar el `case consent.AvisaAlUsuario()` por un `default` en
// aplicarConsentimiento.
func TestUnaMaquinaLibreNoAvisaNiPreguntaEnNingunaPuerta(t *testing.T) {
	s, _, _ := maquinaConGrado(t, fleet.ConsentimientoLibre, true)

	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"uptime"}, "no_wait": true}); e != nil {
		t.Fatalf("`libre` bloqueó un exec: %+v", e)
	}
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && fleet.EsOperacionInterna(c.Argv) {
			t.Errorf("una máquina `libre` recibió la operación interna %q: el grado que dice «no "+
				"hay nadie enfrente» tiene que significar silencio", c.Argv[0])
		}
	}
}
