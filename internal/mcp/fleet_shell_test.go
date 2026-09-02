package mcp

// Pruebas de S5b: la shell interactiva. La mitad de este archivo existe para fijar UNA frase —
// una shell interactiva no es `exec`— y la otra mitad, para que el relay no se convierta en un
// token portador con permisos de shell.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// enrolarConShell da de alta un Tier B con metrics+exec+shell.
func enrolarConShell(t *testing.T, s *McpServer, proyecto, nombre string) fleet.Device {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "B", "caps": []string{"metrics", "exec", "shell"},
		"project": proyecto, "address": "gio@" + nombre + ".local", "os": "linux",
	}); e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	d, _, _ := s.engine.DevicePorNombre(proyecto, nombre)
	return d
}

// conShell es un principal con shell (y exec) sobre todo su proyecto.
func conShell(proyecto string) *Principal {
	p := conExec(proyecto)
	p.Fleet[fleet.CapShell] = []string{"*"}
	return p
}

// ── T1 · la decisión que sostiene el slice ──────────────────────────────────────────────────

// T1 — UNA SHELL INTERACTIVA NO ES `exec`, Y ÉSE ES TODO EL DISEÑO.
//
// S10 partió `exec` en dos permisos: poder ejecutar (la concesión) y poder ejecutar CUALQUIER
// COSA (la allowlist por comando). Una shell interactiva es el tercero, y se lleva puestos a los
// otros dos: quien obtiene un prompt corre lo que quiera, las veces que quiera, sin que nadie
// vuelva a mirar un argv.
//
// Si `musubi_fleet_shell` se gateara con `exec`, un principal acotado a `["journalctl"]` tendría,
// tecleando otra cosa, exactamente lo que la allowlist le estaba negando — y la allowlist pasaría
// a ser decoración en la que alguien confía.
//
// Sabotaje que la hace fallar: cambiar fleet.CapShell por fleet.CapExec en toolFleetShell.
func TestExecNoOtorgaShellNiSiquieraConAccesoTotal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")

	// Acceso de ejecución TOTAL sobre todo el proyecto... y ni una palabra de `shell`.
	todoExec := conExec("casa")
	if _, e := callAsPrincipal(t, s, todoExec, "musubi_fleet_shell", map[string]any{"device": "nas"}); e == nil {
		t.Fatal("FUGA: `exec: [\"*\"]` alcanzó para abrir una shell interactiva. Una shell se saltea cualquier allowlist de comandos: si `exec` la otorgara, la allowlist de S10 sería decoración")
	}

	// Y un admin de la MEMORIA tampoco, que es la valla del track entero.
	admin := &Principal{Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}
	if _, e := callAsPrincipal(t, s, admin, "musubi_fleet_shell", map[string]any{"device": "nas"}); e == nil {
		t.Fatal("FUGA: un admin de la memoria abrió una shell en una máquina")
	}
}

// El caso concreto que motivó T1, escrito entero: la allowlist tiene que seguir significando algo.
//
// Sabotaje que la hace fallar: el mismo de arriba.
func TestUnaAllowlistDeUnComandoNoSeSalteaPidiendoUnaShell(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")

	acotado := conExec("casa")
	acotado.ExecAllow = map[string][]string{"*": {"journalctl"}}
	// El exec de un Tier B sale por SSH de verdad; sin el doble, la prueba se come el timeout de
	// conexión entero (5 s) por nada.
	restaurar := fleet.SSHFalsoParaTest(t, "echo ok")
	defer restaurar()

	// Puede lo que la allowlist dice...
	if _, e := callAsPrincipal(t, s, acotado, "musubi_fleet_exec", map[string]any{
		"device": "nas", "argv": []string{"journalctl", "-n", "5"}, "no_wait": true}); e != nil {
		t.Fatalf("el comando permitido debería pasar: %+v", e)
	}
	// ...y NO puede saltearse la allowlist pidiendo un prompt.
	if _, e := callAsPrincipal(t, s, acotado, "musubi_fleet_shell", map[string]any{"device": "nas"}); e == nil {
		t.Fatal("FUGA: quien sólo puede correr `journalctl` obtuvo una shell interactiva, donde puede correr cualquier cosa")
	}
}

// T2 — `shell` está en la matriz del tier, y Tier C queda afuera por el mismo motivo que `exec`:
// en iOS no existe y en Android depende de que ADB esté habilitado, o sea que no se puede
// prometer al dar de alta.
//
// Sabotaje que la hace fallar: agregar CapShell a TierMovil.
func TestTierCNoAdmiteShellYElAltaLoRechaza(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "tablet", "tier": "C", "caps": []string{"metrics", "shell"}, "project": "casa", "os": "android"})
	if e == nil {
		t.Fatal("se dio de alta un Tier C con `shell`: si no se puede prometer ejecutar un comando, menos un prompt")
	}
	if !fleet.TierAdmite(fleet.TierAgente, fleet.CapShell) || !fleet.TierAdmite(fleet.TierProtocolo, fleet.CapShell) {
		t.Error("A y B tienen que admitir shell, o la prueba de arriba pasaría por la razón equivocada")
	}
}

// T3 — un device YA dado de alta no gana `shell` porque la capacidad exista. Nada de migraciones
// que otorguen permisos: la capacidad vive en la fila del dispositivo y hay que concederla.
//
// Sabotaje que la hace fallar: hacer que Permite consulte sólo la matriz del tier y no las caps
// concedidas al device.
func TestUnaMaquinaViejaNoGanaShellPorQueLaCapacidadExista(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Alta SIN shell, como todas las que existían antes de este slice.
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics", "exec"},
		"project": "casa", "address": "gio@router.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	if _, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "router"}); e == nil {
		t.Fatal("una máquina dada de alta sin `shell` abrió una shell: la capacidad se concede, no se hereda de una versión nueva")
	}
}

// ── T4 · la bitácora ────────────────────────────────────────────────────────────────────────

// T4 — LA SESIÓN QUEDA REGISTRADA ANTES DE CONECTAR, y sigue registrada si la conexión falla.
//
// Que alguien haya INTENTADO abrir una shell en un servidor es información de auditoría tanto
// como que lo haya logrado. Misma regla que F1 de S5 y G7 de S6.
//
// Sabotaje que la hace fallar: abrir el canal antes de escribir la fila.
func TestUnaShellQueNoLlegaAConectarQuedaAuditadaIgual(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")
	// Un ssh que falla siempre: la conexión no va a prender.
	restaurar := fleet.SSHFalsoParaTest(t, "exit 255")
	defer restaurar()

	_, _ = callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})

	res, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell_log", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, res)
	if !strings.Contains(crudo, `"principal":"op"`) || !strings.Contains(crudo, `"device":"nas"`) {
		t.Errorf("el intento no quedó en la bitácora:\n%s", crudo)
	}
}

// T4b — LA BITÁCORA DE SHELL LA LEE UNA CABINA `write=none`, IGUAL QUE LA DE COMANDOS.
//
// `musubi_fleet_shell_log` no muta nada (es un SELECT sobre shell_sessions; el vencimiento se
// DERIVA al leer), pero no estaba marcada readOnly mientras sus siete hermanas de flota sí lo
// estaban. Como readOnly es el eje de AUTORIZACIÓN (Principal.canCall), la consecuencia era muda
// y torcida: una cabina de sólo lectura veía QUÉ COMANDOS se corrieron y no QUIÉN tuvo un prompt
// — la mitad más grave de la misma auditoría, tapada por un olvido y no por una decisión.
//
// La segunda mitad de la prueba es la que la mantiene honesta: leer la bitácora NO es abrir una
// shell. Si alguien "arreglara" esto marcando readOnly a musubi_fleet_shell, acá se cae.
//
// Sabotaje que la hace fallar: quitarle `readOnly: true` a musubi_fleet_shell_log en el registro.
func TestLaBitacoraDeShellLaLeeUnaCabinaSinEscritura(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")
	restaurar := fleet.SSHFalsoParaTest(t, "exit 255")
	defer restaurar()
	// Alguien con permiso de escritura deja una sesión en la bitácora.
	_, _ = callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})

	// La cabina: ve la flota, no la toca. Tiene la capacidad `shell` concedida, así que lo único
	// que puede frenarla es su autoridad — que es justo lo que esta prueba mide.
	cabina := conShell("casa")
	cabina.Name = "panel"
	cabina.Role = RoleReader
	cabina.Write = WriteNone

	res, e := callAsPrincipal(t, s, cabina, "musubi_fleet_shell_log", map[string]any{})
	if e != nil {
		t.Fatalf("una cabina write=none no pudo leer la bitácora de shell: %+v", e)
	}
	if crudo := textOf(t, res); !strings.Contains(crudo, `"device":"nas"`) {
		t.Errorf("la cabina leyó la bitácora pero sin la sesión de `nas`:\n%s", crudo)
	}

	// Y LEER NO ES ENTRAR: la misma cabina no abre un prompt.
	if _, e := callAsPrincipal(t, s, cabina, "musubi_fleet_shell", map[string]any{"device": "nas"}); e == nil {
		t.Error("una cabina write=none abrió una shell interactiva: leer la bitácora y tener un prompt no son la misma autoridad")
	}
}

// ── T5 · los techos ─────────────────────────────────────────────────────────────────────────

// T5 — SON DOS TECHOS Y SON DISTINTOS: la vida máxima y la inactividad cubren casos que el otro
// no cubre. Sólo con el de vida, una terminal abierta en una pestaña que nadie mira es un prompt
// vivo durante dos horas.
//
// Sabotaje que la hace fallar: quitar cualquiera de los dos de SesionShell.Vencida.
func TestUnaSesionMuereTantoPorViejaComoPorAbandonada(t *testing.T) {
	base := time.Now()
	viva := fleet.SesionShell{
		Estado: fleet.ShellActiva, Creada: base,
		Vence: base.Add(fleet.ShellVidaMax), UltimoTrafico: base,
	}
	if vencida, _ := viva.Vencida(base.Add(time.Minute)); vencida {
		t.Fatal("una sesión de un minuto con tráfico reciente no puede estar vencida")
	}

	// ABANDONADA: dentro de su vida máxima, pero nadie la tocó en mucho rato.
	abandonada := viva
	abandonada.UltimoTrafico = base
	if vencida, motivo := abandonada.Vencida(base.Add(fleet.ShellInactividadMax + time.Minute)); !vencida {
		t.Error("una sesión abandonada dentro de su vida máxima siguió viva: sólo con el techo de vida, una pestaña olvidada es un prompt vivo durante horas")
	} else if !strings.Contains(motivo, "inactividad") {
		t.Errorf("el motivo no distingue el techo que la mató: %q", motivo)
	}

	// VIEJA: con tráfico constante, pero pasada su vida máxima. Sin este techo, un `tail -f` la
	// mantiene abierta para siempre.
	vieja := viva
	tarde := base.Add(fleet.ShellVidaMax + time.Minute)
	vieja.UltimoTrafico = tarde // tráfico ahora mismo
	if vencida, motivo := vieja.Vencida(tarde); !vencida {
		t.Error("una sesión pasada de su vida máxima siguió viva porque tenía tráfico: un `tail -f` la haría eterna")
	} else if !strings.Contains(motivo, "vida") {
		t.Errorf("el motivo no distingue el techo que la mató: %q", motivo)
	}
}

// ── T6 · el id de sesión NO es una credencial ───────────────────────────────────────────────

// T6 — CADA REQUEST DEL STREAM SE RE-AUTORIZA ENTERA.
//
// Es el error clásico de las APIs de sesión: autorizar al abrir, devolver un id, y que a partir
// de ahí el id SEA el permiso. Un id así es un token portador con permisos de shell, sin
// vencimiento propio, imposible de revocar — y encima viaja en una URL, que es donde los
// identificadores terminan en logs de proxy.
//
// Sabotaje que la hace fallar: quitar el chequeo `ses.Principal != nombrePrincipal(p)` de
// autorizarShell.
func TestElIdDeSesionNoAlcanzaParaHablarleAlStream(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")

	// Se fabrica la sesión directo en la base: la prueba es sobre el relay, no sobre la apertura.
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	ahora := time.Now()

	// La dueña pasa la autorización.
	if _, err := s.autorizarShell(conShell("casa"), ses.ID, ahora); err != nil {
		t.Fatalf("la dueña de la sesión no pudo autorizarse: %v", err)
	}
	// Otra persona, con el id correcto y hasta con `shell` sobre la misma máquina, NO.
	otra := conShell("casa")
	otra.Name = "intrusa"
	if _, err := s.autorizarShell(otra, ses.ID, ahora); err == nil {
		t.Fatal("FUGA: alguien con el id de una sesión AJENA pudo hablarle. El id dice CUÁL sesión, no QUIÉN sos")
	}
}

// La cuarta pregunta, que es la que se olvida siempre: REVOCAR TIENE QUE CORTAR LA SESIÓN EN
// CURSO, no sólo impedir abrir la próxima.
//
// Sabotaje que la hace fallar: quitar el PuedeSobreDevice de autorizarShell (autorizar sólo al
// abrir).
func TestRevocarLaConcesionCortaElPromptQueYaEstabaAbierto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	ahora := time.Now()
	if _, err := s.autorizarShell(conShell("casa"), ses.ID, ahora); err != nil {
		t.Fatalf("con la concesión puesta debería autorizar: %v", err)
	}

	// Le sacan `shell` (le queda `exec`, que es justamente lo que no alcanza).
	sinShell := conExec("casa")
	if _, err := s.autorizarShell(sinShell, ses.ID, ahora); err == nil {
		t.Fatal("la sesión siguió usable tras perder la concesión `shell`: se está autorizando sólo al abrir, así que revocar a alguien no le corta el prompt que ya tiene")
	}
}

// Una sesión vencida se corta A MITAD DE USO, no sólo al abrir la siguiente.
func TestUnaSesionVencidaSeCortaEnElProximoRequest(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	// Un rato largo después: la mata el techo de inactividad.
	tarde := ses.Creada.Add(fleet.ShellInactividadMax + time.Minute)
	if _, err := s.autorizarShell(conShell("casa"), ses.ID, tarde); err == nil {
		t.Fatal("una sesión abandonada siguió aceptando requests")
	}
}

// El relay exige bearer como cualquier otra puerta, y responde 401 — no un 500 ni un cuerpo
// vacío que se leería como una terminal quieta.
//
// EL CONTROL NEGATIVO NO ES CEREMONIA. La primera versión de esta prueba usaba un id inventado
// para el caso «con token bueno», y ese id daba 401 igual (una sesión inexistente y una ajena dan
// la misma respuesta, a propósito). O sea que la prueba habría pasado con la autenticación
// APAGADA. Hay que usar una sesión REAL para el control positivo.
func TestElRelayDeShellExigeCredencial(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	reg := registroDePrueba(Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapShell: {"*"}},
		hash:  hashToken("el-token-de-op"),
	})
	h := s.HTTPHandler(httpOptions{reqTimeout: 5 * time.Second, registry: reg})
	pedir := func(token string) int {
		r := httptest.NewRequest(http.MethodGet, shellOutPath+"?id="+ses.ID, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	// CONTROL POSITIVO con una sesión REAL: si esto diera 401, la aserción de abajo no probaría
	// nada sobre la credencial.
	if c := pedir("el-token-de-op"); c == http.StatusUnauthorized {
		t.Fatal("ni con el token bueno y una sesión propia pasó: la prueba no distingue «falta credencial» de «la ruta rechaza todo»")
	}
	if c := pedir(""); c != http.StatusUnauthorized {
		t.Errorf("sin bearer el relay respondió %d; esperaba 401", c)
	}
	if c := pedir("un-token-cualquiera"); c != http.StatusUnauthorized {
		t.Errorf("con un bearer inválido el relay respondió %d; esperaba 401", c)
	}
	s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())
}

// LOS TRES RECHAZOS SON DISTINTOS, y quien está del otro lado hace cosas distintas con cada uno.
//
// El atajo es devolver 401 para todo, y manda a rotar tokens que están perfectos: una sesión que
// venció no es un problema de credencial, y una concesión revocada tampoco.
//
// Sabotaje que la hace fallar: devolver http.StatusUnauthorized fijo en los handlers.
func TestElRelayDistingueTokenMaloDeSesionMuertaYDeConcesionRevocada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op"})
	if err != nil {
		t.Fatal(err)
	}
	ahora := time.Now()

	// Sesión propia y viva ⇒ pasa la autorización.
	if _, err := s.autorizarShell(conShell("casa"), ses.ID, ahora); err != nil {
		t.Fatalf("control positivo: %v", err)
	}
	// Sesión propia, VENCIDA ⇒ 410 Gone («terminó, abrí otra»), no 401.
	tarde := ses.Creada.Add(fleet.ShellVidaMax + time.Minute)
	_, err = s.autorizarShell(conShell("casa"), ses.ID, tarde)
	if got := estadoHTTPDeShell(err); got != http.StatusGone {
		t.Errorf("una sesión propia vencida dio %d; esperaba 410: un 401 manda a revisar un token que está perfecto", got)
	}
	// Concesión REVOCADA ⇒ 403 Forbidden («tu credencial está bien y ya no te alcanza»).
	_, err = s.autorizarShell(conExec("casa"), ses.ID, ahora)
	if got := estadoHTTPDeShell(err); got != http.StatusForbidden {
		t.Errorf("una concesión revocada dio %d; esperaba 403", got)
	}
	// Sesión AJENA ⇒ 401, indistinguible de una inexistente: acá SÍ hay un oráculo que cerrar.
	otra := conShell("casa")
	otra.Name = "intrusa"
	_, err = s.autorizarShell(otra, ses.ID, ahora)
	if got := estadoHTTPDeShell(err); got != http.StatusUnauthorized {
		t.Errorf("una sesión ajena dio %d; esperaba 401", got)
	}
	_, err = s.autorizarShell(conShell("casa"), "id-que-no-existe", ahora)
	if got := estadoHTTPDeShell(err); got != http.StatusUnauthorized {
		t.Errorf("un id inexistente dio %d; tiene que ser indistinguible de una sesión ajena (401)", got)
	}
	s.cerrarShell(ses.ID, fleet.ShellCerrada, "fin de la prueba", time.Now())
}

// ── T7 · una sola sesión por par ────────────────────────────────────────────────────────────

// T7 — dos prompts simultáneos de la misma persona en la misma máquina son, casi siempre, una
// sesión olvidada más una nueva. Se devuelve la que ya está, para que quien perdió su terminal
// pueda volver a ella y cerrarla.
//
// Sabotaje que la hace fallar: quitar la consulta a SesionShellAbiertaDe.
func TestAbrirDosVecesDevuelveLaMismaSesion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConShell(t, s, "casa", "nas")
	restaurar := fleet.SSHFalsoParaTest(t, "sleep 30")
	defer restaurar()

	primera, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("primera apertura: %+v", e)
	}
	id1, _ := jsonOf(t, primera)["session_id"].(string)

	segunda, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("segunda apertura: %+v", e)
	}
	m := jsonOf(t, segunda)
	if m["session_id"] != id1 {
		t.Errorf("se abrió una SEGUNDA sesión (%v vs %v): la primera queda huérfana y es la peligrosa", m["session_id"], id1)
	}
	if nota, _ := m["nota"].(string); !strings.Contains(nota, "ya tenías") {
		t.Errorf("no se avisa que es la sesión previa: %q", nota)
	}
	s.cerrarShell(id1, fleet.ShellCerrada, "fin de la prueba", time.Now())
}
