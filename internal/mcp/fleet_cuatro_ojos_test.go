package mcp

// Pruebas de la aprobación de cuatro ojos (Ola 2). El eje que dice CUÁNTAS PERSONAS hacen falta.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func marcarCuatroOjos(t *testing.T, s *McpServer, proyecto, device string) {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_require_approval", map[string]any{
		"device": device, "project": proyecto, "requerir": true,
	}); e != nil {
		t.Fatalf("require_approval(%q): %+v", device, e)
	}
}

// otroConPantalla es el SEGUNDO par de ojos: misma capacidad, otro nombre.
func otroConPantalla(proyecto string) *Principal {
	p := conPantalla(proyecto)
	p.Name = "revisora"
	return p
}

func otroConShell(proyecto string) *Principal {
	p := conShell(proyecto)
	p.Name = "revisora"
	return p
}

// pantallaLista enrola, late y marca la máquina.
func pantallaConCuatroOjos(t *testing.T, s *McpServer) {
	t.Helper()
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	marcarCuatroOjos(t, s, "casa", "pc-gio")
}

// ── LA COMPROBACIÓN QUE ES TODO EL CONTROL ──────────────────────────────────────────────────

// NADIE APRUEBA SU PROPIA SOLICITUD. Sin esto queda una tabla, una tool, un estado «concedida»
// con nombre y hora en la bitácora — y una sola persona abriendo la sesión. El control se ve
// entero y no existe: es el único falso verde que esta feature no puede permitirse.
//
// Sabotaje: sacar `if quien == sol.Solicitante` de toolFleetApprove.
func TestNadieApruebaSuPropiaSolicitud(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo := conPantalla("casa")

	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el primer pedido tenía que devolver una solicitud, no un error: %+v", e)
	}
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if id == "" {
		t.Fatal("la máquina está marcada y el pedido no abrió ninguna solicitud")
	}

	// El mismo principal intenta aprobarse.
	_, e = callAsPrincipal(t, s, yo, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": true,
	})
	if e == nil {
		t.Fatal("una sola persona pidió y aprobó: eso son dos ojos, no cuatro")
	}
	if !strings.Contains(e.Message, "su propia") {
		t.Fatalf("el rechazo no explica la asimetría: %q", e.Message)
	}

	// Y la sesión sigue sin abrirse: el rechazo no puede haber dejado la aprobación puesta.
	res, e = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("segundo pedido: %+v", e)
	}
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("se acuñó una contraseña después de una autoaprobación rechazada")
	}
}

// ── LA PUERTA ───────────────────────────────────────────────────────────────────────────────

// EL PRIMER PEDIDO NO ACUÑA NADA. Es la misma regla que SesionEsperandoPermiso: una contraseña
// que existe es una contraseña que se puede filtrar, aunque nadie la haya usado. Si la puerta
// estuviera después de entregarPantalla, la credencial ya estaría hecha.
//
// Sabotaje: mover la llamada a puertaDeCuatroOjos después de AbrirSesionPantalla.
func TestConCuatroOjosElPrimerPedidoNoAcunaContrasena(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	m := jsonOf(t, res)
	if _, hay := m["password"]; hay {
		t.Fatal("una máquina con cuatro ojos entregó la contraseña sin que nadie aprobara")
	}
	if m["estado"] != string(fleet.AprobacionPendiente) {
		t.Fatalf("estado = %v, esperaba pendiente", m["estado"])
	}
	// Y TAMPOCO SE ABRIÓ NINGUNA FILA DE SESIÓN.
	//
	// Sin esto, la prueba quedaba VERDE bajo el sabotaje que ella misma declara: mover la puerta
	// después de AbrirSesionPantalla —pero antes de entregarPantalla— tampoco devuelve
	// contraseña, así que las dos afirmaciones de arriba se cumplían igual. La declaración del
	// sabotaje y lo que la prueba miraba no coincidían, que es la forma más silenciosa de un
	// falso verde: la encontró una revisión adversaria, no yo.
	ses, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_sessions", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_sessions: %+v", e)
	}
	if filas, _ := jsonOf(t, ses)["sesiones"].([]any); len(filas) != 0 {
		t.Fatalf("se registró %d sesión(es) de pantalla antes de que nadie aprobara", len(filas))
	}
}

// SIN LA MARCA NO CAMBIA NADA. El control es opt-in y una máquina sin marcar tiene que seguir
// abriendo igual que siempre: si esta prueba se pone roja, encender la feature apagó la flota.
//
// Es el CONTROL POSITIVO de todas las de arriba: sin él, una puerta que niega siempre las
// pasaría todas.
func TestSinLaMarcaLaPantallaSigueAbriendoIgual(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen sobre una máquina SIN marcar: %+v", e)
	}
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatal("una máquina sin cuatro ojos dejó de entregar la contraseña: el control se aplicó a todos")
	}
}

// EL CIRCUITO ENTERO: pido, me aprueba otro, entro.
func TestConLaAprobacionDeOtroLaSesionAbre(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)

	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": true, "nota": "revisado",
	}); e != nil {
		t.Fatalf("la segunda persona no pudo aprobar: %+v", e)
	}
	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("con la aprobación puesta la sesión tenía que abrir: %+v", e)
	}
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatal("aprobada y sin contraseña: la puerta no consumió la aprobación")
	}
}

// LA APROBACIÓN ES DE UN SOLO USO. Un permiso reusable no es cuatro ojos: es una llave que la
// segunda persona entregó una vez y que después abre siempre.
//
// Sabotaje: que ConsumirAprobacion devuelva true sin tocar la fila.
func TestLaAprobacionSeGastaEnUnaSolaSesion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": true,
	}); e != nil {
		t.Fatalf("approve: %+v", e)
	}
	// Primera: abre.
	res, _ = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatal("la primera sesión con aprobación no abrió")
	}
	// Segunda: NO puede reusar el mismo permiso.
	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el segundo pedido tenía que abrir una solicitud nueva: %+v", e)
	}
	m := jsonOf(t, res)
	if _, hay := m["password"]; hay {
		t.Fatal("la misma aprobación abrió DOS sesiones: no es de un solo uso")
	}
	if m["solicitud"] == id {
		t.Fatal("el segundo pedido reusó la solicitud ya gastada en vez de abrir otra")
	}
}

// UN «NO» VALE HASTA QUE VENCE. Si volver a pedir en el acto funcionara, cuatro ojos se
// degradaría a «pedir hasta que alguien diga que sí», que es como el cansancio vence a este
// control en cualquier organización.
//
// Sabotaje: sacar 'negada' del IN de AprobacionVigenteDe.
func TestUnNoNoSeVuelveAPedirEnElActo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": false, "nota": "hoy no",
	}); e != nil {
		t.Fatalf("approve(false): %+v", e)
	}
	_, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e == nil {
		t.Fatal("después de un «no» el siguiente pedido no fue rechazado: el control se puede agotar insistiendo")
	}
	if !strings.Contains(e.Message, "hoy no") {
		t.Fatalf("el rechazo no devuelve el motivo que dio quien negó: %q", e.Message)
	}
}

// ── QUIÉN PUEDE APROBAR ─────────────────────────────────────────────────────────────────────

// APROBAR EXIGE LA MISMA CAPACIDAD SOBRE ESA MÁQUINA. Un principal con `exec` y sin `screen` no
// puede avalar una sesión de pantalla: la barra es «podrías haberlo hecho vos».
//
// Sabotaje: cambiar sol.Capacidad por fleet.CapMetrics en la comprobación de toolFleetApprove.
func TestSinLaMismaCapacidadNoSePuedeAprobar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo := conPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)

	soloExec := &Principal{
		Name: "operario", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
	}
	if _, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": true,
	}); e == nil {
		t.Fatal("alguien sin `screen` avaló una sesión de pantalla: la capacidad de aprobar se colapsó")
	}
	// Y la sesión sigue cerrada.
	res, _ = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("la aprobación de alguien sin la capacidad igual abrió la sesión")
	}
}

// LA APROBACIÓN NO SIRVE PARA OTRA COSA. Avalar que alguien MIRE una pantalla no es avalar que
// abra una shell — si no, el permiso más barato de conseguir habilitaría el más caro.
//
// Sabotaje: sacar `AND capacidad = ?` de AprobacionVigenteDe.
func TestLaAprobacionDePantallaNoAbreUnaShell(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Tier A, que es el único tier que admite pantalla Y shell a la vez: en B no hay framebuffer.
	res0, e0 := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas", "tier": "A", "caps": []string{"metrics", "exec", "shell", "screen"},
		"project": "casa", "os": "linux",
	})
	if e0 != nil {
		t.Fatalf("enroll: %+v", e0)
	}
	tok, _ := jsonOf(t, res0)["token"].(string)
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	marcarCuatroOjos(t, s, "casa", "nas")

	yo := conShell("casa")
	yo.Fleet[fleet.CapScreen] = []string{"*"}
	otra := otroConShell("casa")
	otra.Fleet[fleet.CapScreen] = []string{"*"}

	// Se pide y se aprueba una PANTALLA.
	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if id == "" {
		t.Fatal("la pantalla no abrió solicitud sobre una máquina marcada")
	}
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{
		"solicitud": id, "aprobar": true,
	}); e != nil {
		t.Fatalf("approve: %+v", e)
	}

	// Y con eso se intenta abrir una SHELL.
	res, e = callAsPrincipal(t, s, yo, "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("el pedido de shell tenía que abrir su propia solicitud: %+v", e)
	}
	m := jsonOf(t, res)
	if m["session_id"] != nil {
		t.Fatal("una aprobación de `screen` abrió una SHELL: el permiso barato habilitó el caro")
	}
	if m["solicitud"] == id {
		t.Fatal("la shell reusó la solicitud de la pantalla")
	}
}

// ── LA MARCA ────────────────────────────────────────────────────────────────────────────────

// ENCENDER O APAGAR EL CONTROL ES DE ADMIN. Si quien entra pudiera apagarlo, no habría control.
//
// Sabotaje: sacar el `if !p.isAdmin()` de toolFleetRequireApproval.
func TestApagarLosCuatroOjosExigeAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)

	if _, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_require_approval", map[string]any{
		"device": "pc-gio", "requerir": false,
	}); e == nil {
		t.Fatal("alguien con `screen` apagó el control que le exige un segundo par de ojos")
	}
	// Y sigue encendido.
	res, _ := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("el control quedó apagado igual")
	}
}

// OMITIR `requerir` NO PUEDE SIGNIFICAR «APAGALO». Ésa es la dirección peligrosa del error: un
// campo olvidado apagaría el control por distracción.
func TestOmitirRequerirEsUnErrorYNoUnApagado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)

	if _, e := call(t, s, "musubi_fleet_require_approval", map[string]any{"device": "pc-gio"}); e == nil {
		t.Fatal("`requerir` omitido no dio error: un campo olvidado apaga el control")
	}
	res, _ := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("el campo omitido apagó el control")
	}
}

// ── LA SHELL ────────────────────────────────────────────────────────────────────────────────

// LA SHELL TAMBIÉN PASA POR LA PUERTA, y es el camino que más lo necesita: una shell interactiva
// se saltea cualquier allowlist de comandos.
//
// Sabotaje: sacar la llamada a puertaDeCuatroOjos de toolFleetShell.
func TestLaShellTambienExigeCuatroOjos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")
	marcarCuatroOjos(t, s, "casa", "nas")

	res, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("el pedido tenía que devolver una solicitud: %+v", e)
	}
	m := jsonOf(t, res)
	if m["session_id"] != nil {
		t.Fatal("una shell en una máquina con cuatro ojos abrió sin que nadie aprobara")
	}
	if m["solicitud"] == nil {
		t.Fatal("no se abrió ninguna solicitud")
	}
}

// ── LA LISTA ────────────────────────────────────────────────────────────────────────────────

// LA LISTA SÓLO MUESTRA LO QUE PODRÍAS RESOLVER, y dice cuántas dejó afuera: una lista vacía
// significa «ninguna que yo pueda aprobar», no «no hay nadie esperando».
func TestLaListaDeAprobacionesDiceLoQueNoMuestra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo := conPantalla("casa")
	if _, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}

	// Alguien sin `screen` sobre esa máquina: no ve la solicitud, pero se le dice que hay una.
	soloMetrics := &Principal{
		Name: "panel", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	res, e := callAsPrincipal(t, s, soloMetrics, "musubi_fleet_approvals", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_approvals: %+v", e)
	}
	m := jsonOf(t, res)
	if n, _ := m["pendientes"].([]any); len(n) != 0 {
		t.Fatalf("mostró %d solicitudes que este principal no puede aprobar", len(n))
	}
	if m["fuera_de_tu_alcance"] == nil {
		t.Fatal("la lista salió vacía y en silencio: no distingue «ninguna que yo pueda» de «no hay»")
	}

	// Y quien la pidió la ve, con la marca de que no puede aprobarla.
	res, e = callAsPrincipal(t, s, yo, "musubi_fleet_approvals", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_approvals: %+v", e)
	}
	filas, _ := jsonOf(t, res)["pendientes"].([]any)
	if len(filas) != 1 {
		t.Fatalf("esperaba 1 solicitud visible, hay %d", len(filas))
	}
	if fila, _ := filas[0].(map[string]any); fila["podes_aprobarla"] != false {
		t.Fatal("la lista le dice al solicitante que puede aprobar la suya")
	}
}

// ── LO QUE ENCONTRÓ LA REVISIÓN ADVERSARIA (y ninguno de mis siete sabotajes) ────────────────

// LOS DOS RECHAZOS DE `approve` TIENEN QUE SER EL MISMO TEXTO.
//
// Un mensaje «único» que interpola algo dependiente del caso NO es único. La primera versión
// nombraba `sol.Capacidad`, que sale VACÍA cuando la solicitud no existe: probando ids se
// distinguía «no hay tal solicitud» de «hay una y no podés», o sea qué máquinas tienen gente
// pidiendo entrar. Y el segundo caso además revelaba QUÉ capacidad se estaba pidiendo a alguien
// que no tiene ninguna sobre esa máquina.
//
// Sabotaje: volver a interpolar sol.Capacidad en el mensaje de toolFleetApprove.
func TestElRechazoDeAprobarNoDistingueSiLaSolicitudExiste(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)

	res, _ := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if id == "" {
		t.Fatal("no se abrió la solicitud")
	}
	const inventado = "00000000-0000-0000-0000-000000000000"

	// Alguien SIN `screen` sobre esa máquina pregunta por las dos.
	ajeno := &Principal{
		Name: "operario", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
	}
	_, eReal := callAsPrincipal(t, s, ajeno, "musubi_fleet_approve", map[string]any{"solicitud": id, "aprobar": true})
	_, eFalsa := callAsPrincipal(t, s, ajeno, "musubi_fleet_approve", map[string]any{"solicitud": inventado, "aprobar": true})
	if eReal == nil || eFalsa == nil {
		t.Fatal("alguna de las dos no fue rechazada")
	}
	if eReal.Code != eFalsa.Code {
		t.Fatalf("códigos distintos: %d vs %d", eReal.Code, eFalsa.Code)
	}
	// Salvo el id, que el que pregunta ya conoce porque lo escribió él, tienen que ser idénticos.
	a := strings.Replace(eReal.Message, id, "<id>", 1)
	b := strings.Replace(eFalsa.Message, inventado, "<id>", 1)
	if a != b {
		t.Fatalf("los dos rechazos son distinguibles — es un oráculo de qué solicitudes existen:\n  existe:    %s\n  inventada: %s", a, b)
	}
}

// EL CONTADOR DE /metrics NO PUEDE CONTAR MÁQUINAS QUE QUIEN SCRAPEA NO VE.
//
// La primera versión sacaba los proyectos de `vistos` (compuertado) y después pedía las
// pendientes del PROYECTO ENTERO. Una credencial acotada a una máquina recibía el conteo de todas
// las del proyecto: el segundo recorrido sin compuerta que el comentario de al lado decía estar
// evitando.
//
// Sabotaje: contar len(pendientes) en vez de filtrar por `visibles`.
func TestElConteoDeAprobacionesRespetaLaCompuertaPorMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Dos máquinas en el MISMO proyecto. La solicitud es sobre la segunda.
	maquinaConMuestra(t, s, "casa", "visible", *muestraDePrueba(), ahora)
	tok := enrolarConPantalla(t, s, "casa", "reservada")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	marcarCuatroOjos(t, s, "casa", "reservada")
	if _, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "reservada"}); e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}

	// Un principal que SÓLO ve `visible`.
	acotado := &Principal{
		Name: "panel", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"visible"}},
	}
	var b strings.Builder
	renderFlota(&b, s.engine, acotado, ahora, s.sondaIntervalo, versionDePrueba, nil)
	salida := b.String()

	if !strings.Contains(salida, nombreAprobPendientes+`{project="casa"} 0`) {
		linea := "(no está)"
		for _, l := range strings.Split(salida, "\n") {
			if strings.HasPrefix(l, nombreAprobPendientes+"{") {
				linea = l
			}
		}
		t.Fatalf("el conteo incluye una solicitud de una máquina que esta credencial no puede ver: %s", linea)
	}
}

// EL `motivo` LLEGA HASTA QUIEN APRUEBA. Sin él, la segunda persona decide a ciegas: «alguien
// quiere una shell en producción» no es información sobre la que se pueda decir que sí o que no.
// El campo existía en el dominio, en el INSERT y en la lista, y NINGÚN camino lo escribía.
//
// Sabotaje: dejar de pasar `motivo` en la llamada a puertaDeCuatroOjos.
func TestElMotivoLlegaAQuienAprueba(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)

	if _, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{
		"device": "pc-gio", "motivo": "el disco está al 98 % y hay que mirarlo",
	}); e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	res, e := callAsPrincipal(t, s, otroConPantalla("casa"), "musubi_fleet_approvals", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_approvals: %+v", e)
	}
	filas, _ := jsonOf(t, res)["pendientes"].([]any)
	if len(filas) != 1 {
		t.Fatalf("esperaba 1 solicitud, hay %d", len(filas))
	}
	fila, _ := filas[0].(map[string]any)
	if fila["motivo"] != "el disco está al 98 % y hay que mirarlo" {
		t.Fatalf("quien aprueba no recibe el motivo: %q", fila["motivo"])
	}
}

// ── LOS DOS CONTROLES JUNTOS: EL CANDADO QUE NINGUNA PRUEBA COMBINABA ────────────────────────

// UNA MÁQUINA EN `pide` Y CON CUATRO OJOS TIENE QUE PODER ABRIRSE.
//
// Son dos controles correctos por separado que juntos daban un candado: la puerta gastaba la
// aprobación y la llamada seguía hasta `pedirPermisoParaPantalla`, que devuelve «esperando
// permiso» sin abrir nada. La siguiente llamada ya no encontraba aprobación —estaba `usada`— y
// abría otra solicitud: la persona rebotaba entre dos esperas y la sesión no se abría NUNCA.
//
// Sabotaje: volver a consumir la aprobación dentro de puertaDeCuatroOjos.
func TestPideMasCuatroOjosNoSeTrabaEnUnBucle(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	marcarCuatroOjos(t, s, "casa", "pc-gio")
	// `pide` exige que la máquina sepa preguntar, si no se endurece a `prohibido`.
	if err := s.engine.FijarCapacidadDePreguntar(devicePorNombreEnPrueba(t, s, "casa", "pc-gio").ID, true); err != nil {
		t.Fatalf("puede_preguntar: %v", err)
	}
	if _, e := call(t, s, "musubi_fleet_consent", map[string]any{"device": "pc-gio", "grado": "pide", "project": "casa"}); e != nil {
		t.Fatalf("consent: %+v", e)
	}
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	// 1) Primer pedido: cuatro ojos. NO se gasta nada todavía.
	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("primer pedido: %+v", e)
	}
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if id == "" {
		t.Fatalf("no se abrió solicitud de cuatro ojos: %v", jsonOf(t, res))
	}
	// 2) La segunda persona aprueba.
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{"solicitud": id, "aprobar": true}); e != nil {
		t.Fatalf("approve: %+v", e)
	}
	// 3) Segundo pedido: ya pasa cuatro ojos y ahora le toca preguntarle a quien usa la máquina.
	res, e = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("segundo pedido: %+v", e)
	}
	m := jsonOf(t, res)
	if m["estado"] != string(fleet.SesionEsperandoPermiso) {
		t.Fatalf("con la aprobación puesta tenía que pasar a preguntarle al usuario; devolvió %v", m)
	}
	// LO QUE IMPORTA: la aprobación NO se gastó en esa vuelta. Si se hubiera gastado, el próximo
	// pedido abriría otra solicitud y la sesión no se abriría nunca.
	sesID, _ := m["session_id"].(string)
	if sesID == "" {
		t.Fatal("no se registró la sesión esperando permiso")
	}
	if _, ok, err := s.engine.AprobacionVigenteDe(
		devicePorNombreEnPrueba(t, s, "casa", "pc-gio").ID, "mirador", fleet.CapScreen, time.Now().UTC()); err != nil || !ok {
		t.Fatal("la aprobación se gastó en una llamada que sólo preguntó: la sesión no se va a abrir nunca")
	}
}

// devicePorNombreEnPrueba evita repetir el par (proyecto, nombre) -> Device.
func devicePorNombreEnPrueba(t *testing.T, s *McpServer, proyecto, nombre string) fleet.Device {
	t.Helper()
	d, hay, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil || !hay {
		t.Fatalf("no encuentro %q en %q: %v", nombre, proyecto, err)
	}
	return d
}

// LA APROBACIÓN ES DE QUIEN PIDIÓ. El filtro por solicitante es el invariante que
// AprobacionVigenteDe defiende con más párrafos, y no tenía ninguna prueba que se pusiera roja.
//
// Sabotaje: sacar `AND solicitante = ?` de AprobacionVigenteDe.
func TestLaAprobacionDeUnoNoLeSirveAOtro(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	// `yo` pide y `otra` le aprueba.
	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{"solicitud": id, "aprobar": true}); e != nil {
		t.Fatalf("approve: %+v", e)
	}

	// Y ahora un TERCERO intenta entrar aprovechando ese «sí».
	tercero := conPantalla("casa")
	tercero.Name = "colado"
	res, e := callAsPrincipal(t, s, tercero, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("el tercero tenía que abrir su propia solicitud: %+v", e)
	}
	m := jsonOf(t, res)
	if _, hay := m["password"]; hay {
		t.Fatal("un tercero usó la aprobación que le dieron a otro: el permiso dejó de ser de quien lo pidió")
	}
	if m["solicitud"] == id {
		t.Fatal("el tercero reusó la solicitud ajena")
	}

	// Y la de `yo` sigue intacta: el intento del tercero no se la gastó.
	res, e = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatal("el intento de un tercero le gastó la aprobación a quien sí la tenía")
	}
}

// UN «NO» NO LO TAPA UNA SOLICITUD MÁS NUEVA. Puede haber dos filas vivas (la puerta lee y
// después inserta, sin índice único), y con `ORDER BY creada DESC` ganaba la más nueva: una
// pendiente posterior escondía la negativa.
//
// Sabotaje: volver el ORDER BY a `creada DESC` sin la precedencia por estado.
func TestUnNoNoLoTapaUnaSolicitudPosterior(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	d := devicePorNombreEnPrueba(t, s, "casa", "pc-gio")
	ahora := time.Now().UTC()

	// Se fabrica a mano lo que una carrera produce: dos filas vivas para el mismo trío.
	negada, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
		DeviceID: d.ID, ProjectID: "casa", Solicitante: "mirador", Capacidad: fleet.CapScreen,
		Creada: ahora.Add(-2 * time.Minute), Vence: ahora.Add(20 * time.Minute),
	})
	if err != nil {
		t.Fatalf("abrir la primera: %v", err)
	}
	if ok, err := s.engine.ResolverAprobacion(negada.ID, "revisora", "hoy no", false, ahora.Add(-time.Minute)); err != nil || !ok {
		t.Fatalf("negar: %v", err)
	}
	if _, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
		DeviceID: d.ID, ProjectID: "casa", Solicitante: "mirador", Capacidad: fleet.CapScreen,
		Creada: ahora, Vence: ahora.Add(20 * time.Minute),
	}); err != nil {
		t.Fatalf("abrir la segunda: %v", err)
	}

	sol, hay, err := s.engine.AprobacionVigenteDe(d.ID, "mirador", fleet.CapScreen, ahora)
	if err != nil || !hay {
		t.Fatalf("no devolvió ninguna: %v", err)
	}
	if sol.Estado != fleet.AprobacionNegada {
		t.Fatalf("ganó la %s más nueva y tapó el «no»: una negativa se puede borrar pidiendo otra vez", sol.Estado)
	}
}

// ── CAMINOS QUE EXISTÍAN SIN NINGUNA PRUEBA (los marcó la misma revisión) ────────────────────

// UNA APROBACIÓN YA USADA NO SE REANIMA. El `WHERE ... AND estado = 'pendiente'` de
// ResolverAprobacion es lo único que lo impide, y no tenía prueba: sin él, quien aprueba podría
// volver a poner en `concedida` una solicitud ya gastada y regalar una segunda sesión.
//
// Sabotaje: sacar `AND estado = 'pendiente'` del UPDATE de ResolverAprobacion.
func TestUnaAprobacionUsadaNoSeReanima(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	id, _ := jsonOf(t, res)["solicitud"].(string)
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{"solicitud": id, "aprobar": true}); e != nil {
		t.Fatalf("approve: %+v", e)
	}
	// Se gasta abriendo la sesión.
	res, _ = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatal("la sesión no abrió")
	}
	// Y ahora se intenta volver a aprobarla.
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{"solicitud": id, "aprobar": true}); e == nil {
		t.Fatal("se reanimó una aprobación ya usada: alcanza para una segunda sesión que nadie avaló")
	}
	res, _ = callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("la aprobación reanimada abrió una segunda sesión")
	}
}

// UNA APROBACIÓN VENCIDA NO SIRVE. El `vence > ?` de AprobacionVigenteDe es lo único que lo
// sostiene: sin él, un «sí» de hace tres días seguiría abriendo sesiones.
//
// Sabotaje: sacar `AND vence > ?` de AprobacionVigenteDe.
func TestUnaAprobacionVencidaNoAbreNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	d := devicePorNombreEnPrueba(t, s, "casa", "pc-gio")
	ahora := time.Now().UTC()

	// Un «sí» que ya venció, escrito a mano: el reloj no se puede adelantar en la prueba.
	vieja, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
		DeviceID: d.ID, ProjectID: "casa", Solicitante: "mirador", Capacidad: fleet.CapScreen,
		Creada: ahora.Add(-2 * time.Hour), Vence: ahora.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	// Se concede DENTRO de su ventana, para que quede `concedida` y no `pendiente`.
	if ok, err := s.engine.ResolverAprobacion(vieja.ID, "revisora", "", true, ahora.Add(-90*time.Minute)); err != nil || !ok {
		t.Fatalf("conceder: %v (ok=%v)", err, ok)
	}

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	m := jsonOf(t, res)
	if _, hay := m["password"]; hay {
		t.Fatal("una aprobación VENCIDA abrió la sesión: el «sí» de ayer vale para siempre")
	}
	if m["solicitud"] == vieja.ID {
		t.Fatal("se reusó la solicitud vencida en vez de abrir una nueva")
	}
}

// EL UN SOLO USO LO DECIDE LA BASE, y la rama `!gastada` no tenía quien la ejercitara. Se prueba
// en el almacén, que es donde vive la garantía: dos consumos del mismo permiso, el segundo falla.
//
// Sabotaje: sacar `AND estado = 'concedida'` del UPDATE de ConsumirAprobacion.
func TestConsumirDosVecesElMismoPermisoFallaLaSegunda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	d := devicePorNombreEnPrueba(t, s, "casa", "pc-gio")
	ahora := time.Now().UTC()

	sol, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
		DeviceID: d.ID, ProjectID: "casa", Solicitante: "mirador", Capacidad: fleet.CapScreen,
		Creada: ahora, Vence: ahora.Add(fleet.VentanaDeAprobacion),
	})
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	if ok, err := s.engine.ResolverAprobacion(sol.ID, "revisora", "", true, ahora); err != nil || !ok {
		t.Fatalf("conceder: %v", err)
	}
	if ok, err := s.engine.ConsumirAprobacion(sol.ID, ahora); err != nil || !ok {
		t.Fatalf("el primer consumo tenía que funcionar: %v (ok=%v)", err, ok)
	}
	if ok, err := s.engine.ConsumirAprobacion(sol.ID, ahora); err != nil {
		t.Fatalf("segundo consumo: %v", err)
	} else if ok {
		t.Fatal("el mismo permiso se gastó DOS veces: no es de un solo uso, y esa carrera no la ve ninguna prueba secuencial")
	}
}

// LAS DOS SERIES SALEN AUNQUE NO HAYA NADA ESPERANDO. Una serie que sólo existe cuando hay
// problema no se puede graficar y no se distingue de que el exportador no corrió — la misma
// regla que musubi_fleet_export_truncated.
//
// Sabotaje: emitir las series sólo cuando len(pendientes) > 0.
func TestLasSeriesDeAprobacionSalenEnCeroCuandoNoHayNadieEsperando(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, nil)
	salida := b.String()

	for _, linea := range []string{
		nombreAprobPendientes + `{project="casa"} 0`,
		nombreAprobEspera + `{project="casa"} 0`,
	} {
		if !strings.Contains(salida, linea) {
			t.Errorf("falta %q: una serie que sólo aparece cuando hay problema no se distingue de que el exportador no corrió", linea)
		}
	}
	for _, tipo := range []string{nombreAprobPendientes, nombreAprobEspera} {
		if !strings.Contains(salida, "# TYPE "+tipo+" gauge") {
			t.Errorf("%s sale sin TYPE: Prometheus la toma como untyped", tipo)
		}
	}
}

// APAGAR EL CONTROL TIENE QUE DEVOLVER EL ACCESO. Es el camino de la urgencia —una máquina
// marcada con un solo par de ojos disponible queda encerrada— y no tenía ninguna prueba: si
// `requerir: false` no hiciera nada, la única salida sería tocar la base a mano.
//
// Sabotaje: que FijarAprobacion ignore el valor y escriba siempre 1.
func TestApagarElControlDevuelveElAcceso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pantallaConCuatroOjos(t, s)
	yo := conPantalla("casa")

	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if _, hay := jsonOf(t, res)["password"]; hay {
		t.Fatal("la máquina marcada abrió sin aprobación")
	}
	if _, e := call(t, s, "musubi_fleet_require_approval", map[string]any{
		"device": "pc-gio", "project": "casa", "requerir": false,
	}); e != nil {
		t.Fatalf("apagar: %+v", e)
	}
	res, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen tras apagar: %+v", e)
	}
	if _, hay := jsonOf(t, res)["password"]; !hay {
		t.Fatalf("con el control apagado la sesión sigue sin abrir: la máquina quedó encerrada — %v", jsonOf(t, res))
	}
}
