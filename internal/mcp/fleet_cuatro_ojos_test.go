package mcp

// Pruebas de la aprobación de cuatro ojos (Ola 2). El eje que dice CUÁNTAS PERSONAS hacen falta.

import (
	"strings"
	"testing"

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
