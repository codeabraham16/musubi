package mcp

// Pruebas del slice S6: la pantalla.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func enrolarConPantalla(t *testing.T, s *McpServer, proyecto, nombre string) string {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics", "exec", "screen"},
		"project": proyecto, "os": "linux",
	})
	if e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	return tok
}

func conPantalla(proyecto string) *Principal {
	return &Principal{
		Name: "mirador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: proyecto,
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}, fleet.CapMetrics: {"*"}},
	}
}

// abrirSesion enrola, late (para que figure en línea) y pide una sesión.
func abrirSesion(t *testing.T, s *McpServer, p *Principal) map[string]any {
	t.Helper()
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}
	return jsonOf(t, res)
}

// G4 — sin `CapScreen` sobre ESA máquina no hay sesión. Y `exec` NO alcanza: mirar y tocar son
// permisos distintos.
//
// Sabotaje: usar CapExec en vez de CapScreen en toolFleetScreen.
func TestSinCapacidadScreenNoHaySesion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	// Un principal con exec sobre TODO, pero sin screen.
	soloExec := &Principal{
		Name: "op", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	if _, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_screen", map[string]any{"device": "pc-gio"}); e == nil {
		t.Fatal("alguien con `exec` y sin `screen` abrió una pantalla: los dos permisos se colapsaron")
	}
	// Y un admin pelado tampoco.
	admin := &Principal{Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}
	if _, e := callAsPrincipal(t, s, admin, "musubi_fleet_screen", map[string]any{"device": "pc-gio"}); e == nil {
		t.Fatal("un admin sin concesiones abrió una pantalla")
	}
}

// G5 — un Tier B nunca tiene pantalla, por más comodín que tenga el principal.
func TestUnTierBNuncaTienePantalla(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "switch", "tier": "B", "caps": []string{"metrics"}, "project": "infra"}); e != nil {
		t.Fatal(e)
	}
	todopoderoso := &Principal{
		Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "infra",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}},
	}
	_, e := callAsPrincipal(t, s, todopoderoso, "musubi_fleet_screen", map[string]any{"device": "switch"})
	if e == nil {
		t.Fatal("se abrió una pantalla sobre un Tier B: un router no tiene framebuffer")
	}
	if !strings.Contains(e.Message, "Tier B") {
		t.Errorf("el error no explica por qué es imposible: %s", e.Message)
	}
}

// G1 — LA GARANTÍA CENTRAL: la contraseña se devuelve UNA vez y NO queda en ningún lado.
//
// Sabotaje: agregarle una columna a screen_sessions y guardarla ahí.
func TestLaContrasenaNoQuedaEnNingunaTabla(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	out := abrirSesion(t, s, conPantalla("casa"))
	pass, _ := out["password"].(string)
	if pass == "" {
		t.Fatal("no se devolvió contraseña")
	}

	// Se verifica por TODAS las superficies que podrían exponerla. La ausencia estructural
	// —que no exista un campo donde ponerla— la custodia TestLaSesionNoTieneDondeGuardarLaContrasena
	// en internal/fleet; acá se custodian las salidas.

	// La bitácora de sesiones no la trae.
	ses, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_sessions", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(textOf(t, ses), pass) {
		t.Error("la bitácora de SESIONES expone la contraseña")
	}

	// Y el inventario tampoco.
	inv, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(textOf(t, inv), pass) {
		t.Error("el inventario expone la contraseña de pantalla")
	}
}

// LA PUERTA DE AL LADO: la contraseña viaja en el argv de un comando, y la bitácora de COMANDOS
// guarda el argv tal cual. Sin ocultarlo, `musubi_fleet_log` entrega contraseñas de sesión a
// cualquiera que pueda leerla — y la garantía G1 se cae sin que nadie toque la tabla de sesiones.
//
// Sabotaje que la hace fallar: quitar `ocultarArgvDePantalla` de toolFleetLog.
func TestLaBitacoraDeComandosNoFiltraLaContrasenaDePantalla(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// El mirador necesita también `exec` para poder LEER la bitácora de comandos: es el caso
	// peor, y el que hay que probar.
	p := &Principal{
		Name: "mirador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}, fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
	}
	out := abrirSesion(t, s, p)
	pass, _ := out["password"].(string)

	log, e := callAsPrincipal(t, s, p, "musubi_fleet_log", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, log)
	if strings.Contains(crudo, pass) {
		t.Fatalf("LA BITÁCORA DE COMANDOS FILTRA LA CONTRASEÑA DE PANTALLA:\n%s", crudo)
	}
	// Pero SÍ se ve que hubo una operación de pantalla, con su id de sesión: ocultar no es borrar.
	if !strings.Contains(crudo, "musubi:pantalla") || !strings.Contains(crudo, "[oculto]") {
		t.Errorf("la operación de pantalla desapareció de la bitácora en vez de ocultarse:\n%s", crudo)
	}
	if sid, _ := out["session_id"].(string); sid != "" && !strings.Contains(crudo, sid) {
		t.Error("no se conservó el id de sesión, que es lo que permite cruzar las dos bitácoras")
	}
}

// ESCALAMIENTO CERRADO: alguien con `exec` no puede fabricar una operación de pantalla a mano y
// acuñarse una sesión sin tener `screen`.
//
// Sabotaje: quitar la guarda de prefijo `musubi:` de toolFleetExec.
func TestConExecNoSePuedeFabricarUnaSesionDePantalla(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	soloExec := &Principal{
		Name: "op", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	_, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"musubi:pantalla", "sesion-falsa", "passelegida", "30m"},
	})
	if e == nil {
		t.Fatal("alguien con `exec` encoló una operación de pantalla: se saltó la compuerta de `screen` usando la otra mitad")
	}
	if e.Code != codeUnauthorized {
		t.Errorf("esperaba unauthorized, obtuve %d", e.Code)
	}
	// Y nada llegó a la cola.
	if cs, _ := s.engine.BitacoraDeComandos("casa", "", 10); len(cs) != 0 {
		t.Fatalf("quedaron %d comandos encolados pese al rechazo", len(cs))
	}
}

// G7 — la sesión se registra ANTES de acuñar nada, con quién la pidió.
func TestLaSesionQuedaAuditadaConQuienPidioMirar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	out := abrirSesion(t, s, conPantalla("casa"))

	ses, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_sessions", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, ses)
	for _, quiero := range []string{out["session_id"].(string), `"principal":"mirador"`, `"device":"pc-gio"`, `"vence"`} {
		if !strings.Contains(crudo, quiero) {
			t.Errorf("la bitácora de sesiones no registra %q:\n%s", quiero, crudo)
		}
	}
}

// G8 — la bitácora de sesiones no la ve cualquiera.
func TestLaBitacoraDeSesionesExigeLaCapacidad(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	abrirSesion(t, s, conPantalla("casa"))

	sinNada := &Principal{Name: "curioso", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}
	res, e := callAsPrincipal(t, s, sinNada, "musubi_fleet_sessions", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	out := jsonOf(t, res)
	if out["total"] != float64(0) {
		t.Errorf("un admin sin `screen` vio %v sesiones", out["total"])
	}
	if out["sin_permiso"] == nil {
		t.Error("no se informa cuántas quedaron ocultas")
	}
}

// Una máquina CAÍDA no puede recibir la contraseña, así que no se abre la sesión: dar una
// contraseña que nadie va a aplicar sería entregar un secreto por nada.
func TestNoSeAbreSesionSobreUnaMaquinaCaida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConPantalla(t, s, "casa", "pc-apagada") // nunca latió
	_, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-apagada"})
	if e == nil {
		t.Fatal("se acuñó una contraseña para una máquina que no late")
	}
	if !strings.Contains(e.Message, "no está latiendo") {
		t.Errorf("el error no explica por qué: %s", e.Message)
	}
	if ss, _ := s.engine.SesionesDePantalla("casa", "", 10, time.Now()); len(ss) != 0 {
		t.Errorf("quedaron %d sesiones abiertas pese al rechazo", len(ss))
	}
}

// EL CICLO DE VIDA DE LA SESIÓN SE CIERRA: `solicitada` → `activa` cuando la máquina confirma.
//
// El primer e2e dejó la sesión en `solicitada` para siempre: el estado `activa` existía en el
// dominio y NADA transicionaba a él. Una bitácora que no distingue «se aplicó» de «la máquina no
// pudo» es inútil justo cuando alguien dice «no me deja entrar».
//
// Sabotaje que la hace fallar: quitar marcarSesionSiEsDePantalla del handler de resultados.
func TestLaSesionPasaAActivaCuandoLaMaquinaConfirma(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatal(e)
	}
	sid, _ := jsonOf(t, res)["session_id"].(string)

	// La máquina levanta el comando y confirma.
	_, resp := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	ids := idsDeComandos(resp)
	if len(ids) == 0 {
		t.Fatal("la máquina no recibió la operación de pantalla")
	}
	if code, _ := postCon(t, ts.URL+fleetResultPath, tok,
		`{"command_id":"`+ids[0]+`","exit_code":0,"stdout":"aplicada"}`); code != 200 {
		t.Fatalf("el reporte falló: %d", code)
	}

	ss, err := s.engine.SesionesDePantalla("casa", "", 10, time.Now())
	if err != nil || len(ss) != 1 {
		t.Fatalf("sesiones: %d (err=%v)", len(ss), err)
	}
	if ss[0].ID != sid || ss[0].Estado != fleet.SesionActiva {
		t.Fatalf("la sesión quedó en %q, esperaba activa", ss[0].Estado)
	}
}

// Y si la máquina NO pudo aplicarla, la sesión queda `fallida`, no activa.
func TestLaSesionQuedaFallidaSiLaMaquinaNoPudo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})

	_, resp := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
	ids := idsDeComandos(resp)
	postCon(t, ts.URL+fleetResultPath, tok,
		`{"command_id":"`+ids[0]+`","error":"RustDesk no está instalado"}`)

	ss, _ := s.engine.SesionesDePantalla("casa", "", 10, time.Now())
	if len(ss) != 1 || ss[0].Estado != fleet.SesionFallida {
		t.Fatalf("la sesión quedó en %q, esperaba fallida", ss[0].Estado)
	}
	if !strings.Contains(ss[0].Error, "no está instalado") {
		t.Errorf("no se conservó el motivo del fallo: %q", ss[0].Error)
	}
}

// Una sesión que NUNCA se aplicó y pasó su ventana está vencida, no pendiente para siempre.
func TestUnaSesionSolicitadaQueVencioNoQuedaPendienteParaSiempre(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorToken(tok)
	ahora := time.Now().UTC()
	if _, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio",
		Creada: ahora.Add(-2 * time.Hour), Vence: ahora.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	ss, _ := s.engine.SesionesDePantalla("casa", "", 10, ahora)
	if len(ss) != 1 || ss[0].Estado != fleet.SesionVencida {
		t.Fatalf("la sesión quedó en %q, esperaba vencida", ss[0].Estado)
	}
}

// enrolarMovil da de alta un Tier C con `screen` concedido por la matriz, y lo hace latir para
// que figure en línea (la sonda le escribe last_seen igual que un latido propio).
func enrolarMovil(t *testing.T, s *McpServer, proyecto, nombre string) string {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "C", "caps": []string{"metrics", "screen"},
		"project": proyecto, "os": "android",
	})
	if e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	return tok
}

// A18 — UN TIER SIN MOTOR NO ACUÑA NADA.
//
// Este era un hueco ACTIVO, no una tarea futura: la matriz le concede `screen` a Tier C (un móvil
// TIENE framebuffer), pero methods_pantalla sólo habla RustDesk. El camino entero pasaba: la
// autorización, «en línea» —la sonda le escribe last_seen—, la colisión de id (sin rustdesk_id no
// hay colisión posible), y entonces ABRÍA la sesión, ACUÑABA la contraseña, LA MOSTRABA la única
// vez que se muestra, y encolaba `musubi:pantalla` en una cola que en Tier C no drena nadie: el
// agente es de Tier A. El comando vencía a los 15 min y la bitácora decía que se abrió una
// pantalla.
//
// La prueba no se conforma con el error: verifica que NO quedó sesión ni comando. Un rechazo que
// igual dejó rastro acuñado sería el mismo daño con mejor cara.
//
// Sabotaje: quitar la guarda de fleet.MotorDePantalla en toolFleetScreen.
func TestUnAndroidNoAcunaContrasenaDePantalla(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarMovil(t, s, "casa", "telefono")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "") // late: pasa `en línea`

	_, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "telefono"})
	if e == nil {
		t.Fatal("se acuñó una contraseña de pantalla para un Tier C, que no tiene motor: nadie iba a levantar ese comando")
	}
	if !strings.Contains(e.Message, "motor") {
		t.Errorf("el error no dice que falta el MOTOR, que es la razón real:\n%s", e.Message)
	}
	if ss, _ := s.engine.SesionesDePantalla("casa", "", 10, time.Now()); len(ss) != 0 {
		t.Errorf("quedaron %d sesiones pese al rechazo", len(ss))
	}
	cmds, err := s.engine.BitacoraDeComandos("casa", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		if EsComandoDePantalla(c.Argv) {
			t.Fatalf("quedó encolado un comando de pantalla que nadie va a levantar: %v", c.Argv)
		}
	}
}

// EL ORDEN DE LAS DOS NEGATIVAS IMPORTA. Que no haya motor es PERMANENTE; que no esté latiendo es
// transitorio. Si la guarda de «en línea» fuera primero, un móvil apagado mandaría a alguien a
// depurar una sonda que anda perfecto, y la razón verdadera no aparecería nunca.
//
// Sabotaje: mover la guarda de MotorDePantalla debajo de la de EnLinea.
func TestLaFaltaDeMotorSeDiceAntesQueElSilencio(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarMovil(t, s, "casa", "telefono-apagado") // NUNCA late

	_, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen", map[string]any{"device": "telefono-apagado"})
	if e == nil {
		t.Fatal("se abrió la pantalla de un Tier C apagado")
	}
	if strings.Contains(e.Message, "no está latiendo") {
		t.Errorf("se culpa al silencio de un problema que es permanente; la razón es que no hay motor:\n%s", e.Message)
	}
	if !strings.Contains(e.Message, "motor") {
		t.Errorf("el error no nombra la razón real:\n%s", e.Message)
	}
}

// A18 — LA CAPACIDAD INERTE SE VE EN EL INVENTARIO, no recién al fallar la apertura.
//
// Misma lección que `puede_actuar` (A23): una capacidad que no hace nada y una que funciona no
// pueden compartir dibujo. `puedo` lista `screen` sobre el Tier C —y está bien, la concesión es
// real—, así que sin esta marca las dos filas se leen igual.
//
// Sabotaje: quitar el bloque de pantalla_sin_motor del inventario.
func TestElInventarioMarcaLaPantallaSinMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarMovil(t, s, "casa", "telefono")
	enrolarConPantalla(t, s, "casa", "pc-gio")

	res, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	filas, _ := jsonOf(t, res)["devices"].([]any)
	if len(filas) != 2 {
		t.Fatalf("el inventario trajo %d máquinas, esperaba 2", len(filas))
	}
	visto := map[string]bool{}
	for _, f := range filas {
		fila, _ := f.(map[string]any)
		nombre, _ := fila["name"].(string)
		visto[nombre] = fila["pantalla_sin_motor"] == true
	}
	if !visto["telefono"] {
		t.Error("el Tier C no se marca como pantalla sin motor: se lee igual que una que funciona")
	}
	if visto["pc-gio"] {
		t.Error("se marcó sin motor un Tier A, que SÍ tiene RustDesk: la marca perdería todo significado")
	}
}
