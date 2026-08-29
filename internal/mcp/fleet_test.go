package mcp

// Pruebas del slice S2 del track «Control de flota»: la administración de la flota por parte de
// las personas y LA PUERTA DEL DISPOSITIVO, que es una puerta aparte a propósito.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// enrolarDePrueba da de alta un dispositivo como admin local y devuelve su token crudo.
func enrolarDePrueba(t *testing.T, s *McpServer, proyecto, nombre string) string {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics", "exec"}, "project": proyecto,
		"os": "linux", "arch": "amd64",
	})
	if e != nil {
		t.Fatalf("fleet_enroll(%q): %+v", nombre, e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	if tok == "" {
		t.Fatalf("fleet_enroll(%q) no devolvió token", nombre)
	}
	return tok
}

// servidorConFlota levanta un HTTP real con auth de principals + un dispositivo enrolado.
func servidorConFlota(t *testing.T) (*McpServer, *httptest.Server, string, string) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	tokenDevice := enrolarDePrueba(t, s, "casa", "pc-gio")

	const tokenPersona = "token-de-una-persona"
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{
		reqTimeout: 10 * time.Second, token: tokenPersona, loopbackOnly: true,
	}))
	t.Cleanup(ts.Close)
	return s, ts, tokenDevice, tokenPersona
}

func postCon(t *testing.T, url, auth, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return resp.StatusCode, sb.String()
}

// ── H1 · Las dos puertas no se cruzan ────────────────────────────────────────────────────────

// B1 — LA PRUEBA CENTRAL DEL SLICE. Un token de DISPOSITIVO no autentica en /mcp.
//
// Si esto dejara de valer, comprometer cualquier máquina de la flota —la superficie más expuesta
// del sistema— entregaría musubi_recall sobre la memoria de toda la empresa.
//
// Sabotaje que la hace fallar: hacer que el handler de /mcp caiga a DevicePorToken cuando el
// registro de principals no resuelve (el "unifiquemos los lookups" que alguien va a proponer).
func TestTokenDeDispositivoNoAbreElMCP(t *testing.T) {
	_, ts, tokenDevice, tokenPersona := servidorConFlota(t)
	const rpc = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	code, _ := postCon(t, ts.URL+mcpHTTPPath, tokenDevice, rpc)
	if code != http.StatusUnauthorized {
		t.Fatalf("un token de DISPOSITIVO entró a /mcp con status %d: la flota abriría la memoria del equipo", code)
	}
	// Control: la puerta funciona para quien sí corresponde.
	if code, _ := postCon(t, ts.URL+mcpHTTPPath, tokenPersona, rpc); code != http.StatusOK {
		t.Fatalf("el token de la PERSONA no entró a /mcp: status %d", code)
	}
}

// B2 — la separación va en los dos sentidos: un token de PERSONA no late.
//
// Si valiera, `last_seen` sería escribible por cualquiera con una credencial de lectura y el
// panel mostraría vivas máquinas apagadas.
//
// Sabotaje: resolver también contra opt.registry en handlerLatido.
func TestTokenDePersonaNoLate(t *testing.T) {
	_, ts, tokenDevice, tokenPersona := servidorConFlota(t)

	code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenPersona, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("un token de PERSONA latió con status %d: last_seen sería escribible por cualquiera", code)
	}
	// Control: el device sí late.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, ""); code != http.StatusOK {
		t.Fatalf("el token del DISPOSITIVO no pudo latir: status %d", code)
	}
}

// B3 — el 401 no es un oráculo: desconocido, revocado y basura dicen lo MISMO.
// Sabotaje: devolver un motivo distinto según el caso.
func TestElRechazoNoDiceCualExistio(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	// Un token que nunca existió.
	_, cuerpoDesconocido := postCon(t, ts.URL+fleetHeartbeatPath, "token-que-jamas-existio", "")
	// Uno que existió y fue revocado.
	if _, e := call(t, s, "musubi_fleet_revoke", map[string]any{"name": "pc-gio", "project": "casa"}); e != nil {
		t.Fatalf("fleet_revoke: %+v", e)
	}
	codeRevocado, cuerpoRevocado := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, "")
	// Uno con formato raro.
	_, cuerpoBasura := postCon(t, ts.URL+fleetHeartbeatPath, "@@@no-es-un-token@@@", "")

	if codeRevocado != http.StatusUnauthorized {
		t.Fatalf("el device revocado latió igual: status %d", codeRevocado)
	}
	if cuerpoDesconocido != cuerpoRevocado || cuerpoRevocado != cuerpoBasura {
		t.Errorf("el rechazo distingue casos y funciona como oráculo:\n desconocido=%s revocado=%s basura=%s",
			cuerpoDesconocido, cuerpoRevocado, cuerpoBasura)
	}
}

// ── H2 · El latido dice la verdad ────────────────────────────────────────────────────────────

// B4 — el cuerpo del POST no puede cambiar quién es el dispositivo.
// Sabotaje: leer un `device_id` del cuerpo y usarlo en vez del token.
func TestElCuerpoDelLatidoNoPuedeSuplantar(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	otroToken := enrolarDePrueba(t, s, "casa", "servidor-critico")

	otro, _, err := s.engine.DevicePorToken(otroToken)
	if err != nil {
		t.Fatal(err)
	}

	// pc-gio late diciendo ser el servidor crítico.
	cuerpo := `{"device_id":"` + otro.ID + `","name":"servidor-critico","project":"casa"}`
	code, resp := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if strings.Contains(resp, "servidor-critico") {
		t.Fatalf("el latido se atribuyó a la máquina que declaró el CUERPO: %s", resp)
	}
	if !strings.Contains(resp, "pc-gio") {
		t.Fatalf("el latido no se atribuyó a la máquina del TOKEN: %s", resp)
	}
	// Y el servidor crítico sigue sin latir nunca.
	otroDespues, _, _ := s.engine.DevicePorToken(otroToken)
	if !otroDespues.LastSeen.IsZero() {
		t.Error("el latido de pc-gio estampó last_seen en la fila de otra máquina")
	}
}

// B5 — el latido estampa last_seen de verdad, y el listado lo ve.
func TestElLatidoSeVeEnElInventario(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	antes := listarFlota(t, s, "casa")
	if antes[0]["online"] != false || antes[0]["nunca_latio"] != true {
		t.Fatalf("una máquina recién enrolada no debería figurar en línea: %+v", antes[0])
	}

	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, ""); code != http.StatusOK {
		t.Fatalf("el latido falló: status %d", code)
	}

	despues := listarFlota(t, s, "casa")
	if despues[0]["online"] != true {
		t.Fatalf("tras latir, la máquina debería figurar en línea: %+v", despues[0])
	}
	if _, hay := despues[0]["nunca_latio"]; hay {
		t.Error("tras latir sigue marcada como que nunca latió")
	}
}

// B6 — el lockout anti fuerza-bruta cubre la puerta nueva.
// Sabotaje: quitar el limiter de handlerLatido → la tabla de dispositivos queda como oráculo de
// fuerza bruta sin costo.
func TestLaPuertaDelDispositivoTieneLockout(t *testing.T) {
	_, ts, _, _ := servidorConFlota(t)

	visto429 := false
	for i := 0; i < 10; i++ {
		code, _ := postCon(t, ts.URL+fleetHeartbeatPath, "token-malo", "")
		if code == http.StatusTooManyRequests {
			visto429 = true
			break
		}
	}
	if !visto429 {
		t.Fatal("10 intentos fallidos seguidos no dispararon el lockout: la puerta es un oráculo de fuerza bruta")
	}
}

// El latido rechaza métodos que no son POST.
func TestElLatidoSoloAceptaPost(t *testing.T) {
	_, ts, tokenDevice, _ := servidorConFlota(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+fleetHeartbeatPath, nil)
	req.Header.Set("Authorization", "Bearer "+tokenDevice)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET al latido: status %d, esperaba 405", resp.StatusCode)
	}
}

// ── H3 · Administración y tenencia ───────────────────────────────────────────────────────────

// B8 — enrolar y revocar son ADMIN; listar no.
// Sabotaje: quitar el gate isAdmin de toolFleetEnroll.
func TestEnrolarYRevocarSonAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "casa"}
	reader := &Principal{Name: "ojos", Role: RoleReader, ProjectID: "casa"}

	for _, tc := range []struct {
		p    *Principal
		tool string
		args map[string]any
	}{
		{writer, "musubi_fleet_enroll", map[string]any{"name": "x", "tier": "A"}},
		{writer, "musubi_fleet_revoke", map[string]any{"name": "x"}},
		{reader, "musubi_fleet_enroll", map[string]any{"name": "x", "tier": "A"}},
		{reader, "musubi_fleet_revoke", map[string]any{"name": "x"}},
	} {
		_, e := callAsPrincipal(t, s, tc.p, tc.tool, tc.args)
		if e == nil {
			t.Errorf("%s: un %s no-admin no debería poder invocarla", tc.tool, tc.p.Role)
		} else if e.Code != codeUnauthorized {
			t.Errorf("%s (%s): esperaba unauthorized, obtuve code %d", tc.tool, tc.p.Role, e.Code)
		}
	}
	// Listar el inventario NO es admin: un reader ve su flota.
	if _, e := callAsPrincipal(t, s, reader, "musubi_fleet_list", map[string]any{}); e != nil {
		t.Errorf("un reader debería poder listar su flota: %+v", e)
	}
}

// B9 — el proyecto sale de la CREDENCIAL. Un admin acotado no enrola en el tenant de otro.
// Sabotaje: usar args.Project directo en vez de writeOriginFor.
func TestEnrolarNoPuedeElegirElTenantAjeno(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Admin con write=own: administra, pero acotado a su proyecto.
	acotado := &Principal{Name: "admin-casa", Role: RoleAdmin, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa"}

	res, e := callAsPrincipal(t, s, acotado, "musubi_fleet_enroll", map[string]any{
		"name": "infiltrada", "tier": "A", "project": "cliente-acme",
	})
	if e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	if got := jsonOf(t, res)["project_id"]; got != "casa" {
		t.Fatalf("la máquina se enroló en %q pese a que la credencial es de `casa`: plantar un agente en la flota ajena", got)
	}
	// Y no aparece en el tenant que declaró.
	if ds, err := s.engine.ListarDevices("cliente-acme", true); err != nil || len(ds) != 0 {
		t.Fatalf("quedó una máquina en el tenant ajeno: %d (err=%v)", len(ds), err)
	}
}

// B10 — listar no cruza tenants: el arg `project` sólo lo respeta read=all.
// Sabotaje: devolver `declarado` sin chequear las capacidades en fleetReadScopeFor.
func TestListarNoCruzaTenants(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio")
	enrolarDePrueba(t, s, "cliente-acme", "server-acme")

	acotado := &Principal{Name: "dev", Role: RoleWriter, Read: ReadOwn, ProjectID: "casa"}
	res, e := callAsPrincipal(t, s, acotado, "musubi_fleet_list", map[string]any{"project": "cliente-acme"})
	if e != nil {
		t.Fatalf("list: %+v", e)
	}
	out := jsonOf(t, res)
	if out["project_id"] != "casa" {
		t.Fatalf("un principal acotado listó %q: cruce de tenants", out["project_id"])
	}

	// La sala de mando (read=all) SÍ puede mirar la flota de otro proyecto.
	salaDeMando := &Principal{Name: "mando", Role: RoleWriter, Read: ReadAll, Write: WriteOwn, ProjectID: "casa"}
	res2, e2 := callAsPrincipal(t, s, salaDeMando, "musubi_fleet_list", map[string]any{"project": "cliente-acme"})
	if e2 != nil {
		t.Fatalf("list (read=all): %+v", e2)
	}
	if jsonOf(t, res2)["project_id"] != "cliente-acme" {
		t.Error("un read=all debería poder listar la flota de otro proyecto")
	}
}

// B11 — `online` se calcula al servir, con el umbral que pide el llamador.
// Sabotaje: fijar el umbral e ignorar umbral_segundos.
func TestOnlineSeCalculaConElUmbralQuePideElLlamador(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, ""); code != http.StatusOK {
		t.Fatalf("latido: status %d", code)
	}

	// Con el umbral por defecto está viva.
	if listarFlota(t, s, "casa")[0]["online"] != true {
		t.Fatal("con umbral default debería estar en línea")
	}
	// Con un umbral imposible de cumplir, la MISMA máquina figura caída: el estado es derivado,
	// no un booleano guardado.
	res, e := call(t, s, "musubi_fleet_list", map[string]any{"project": "casa", "umbral_segundos": 1})
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(1100 * time.Millisecond)
	res, e = call(t, s, "musubi_fleet_list", map[string]any{"project": "casa", "umbral_segundos": 1})
	if e != nil {
		t.Fatal(e)
	}
	devs, _ := jsonOf(t, res)["devices"].([]any)
	fila, _ := devs[0].(map[string]any)
	if fila["online"] != false {
		t.Errorf("con umbral de 1 s y un latido de hace más, debería figurar caída: %+v", fila)
	}
}

// La matriz de tiers de S1 se propaga hasta la tool: pedir `screen` en un Tier B falla acá.
func TestEnrolarRechazaCapacidadFueraDeTier(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "switch", "tier": "B", "caps": []string{"screen"}, "project": "infra",
	})
	if e == nil {
		t.Fatal("se enroló un Tier B con `screen`: un router no tiene framebuffer")
	}
	if e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params, obtuve code %d", e.Code)
	}
}

// El token se entrega UNA vez y no hay forma de recuperarlo: el listado no lo muestra.
func TestElTokenDelDispositivoNoApareceEnElInventario(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	token := enrolarDePrueba(t, s, "casa", "pc-gio")

	res, e := call(t, s, "musubi_fleet_list", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatal(e)
	}
	if crudo := textOf(t, res); strings.Contains(crudo, token) || strings.Contains(crudo, fleet.HashToken(token)) {
		t.Error("el inventario expone la credencial del dispositivo")
	}
}

// listarFlota devuelve las filas del inventario de un proyecto (como admin local).
func listarFlota(t *testing.T, s *McpServer, proyecto string) []map[string]any {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_list", map[string]any{"project": proyecto})
	if e != nil {
		t.Fatalf("fleet_list: %+v", e)
	}
	crudo, _ := jsonOf(t, res)["devices"].([]any)
	out := make([]map[string]any, 0, len(crudo))
	for _, f := range crudo {
		m, _ := f.(map[string]any)
		out = append(out, m)
	}
	if len(out) == 0 {
		t.Fatalf("el inventario de %q vino vacío", proyecto)
	}
	return out
}

// servidorHTTP levanta un HTTP real sobre un McpServer ya construido, sin enrolar nada.
func servidorHTTP(t *testing.T, s *McpServer) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{
		reqTimeout: 10 * time.Second, token: "token-de-una-persona", loopbackOnly: true,
	}))
	t.Cleanup(ts.Close)
	return ts
}

// QUÉ BINARIO CORRE CADA MÁQUINA, VISIBLE.
//
// Nació de una auditoría contra producción, no de una idea: `kernelos-pc` figuraba en línea,
// latiendo cada 30 s, y con CERO servicios. Eso tiene dos causas opuestas —corre un binario
// anterior a la enumeración, o corre el nuevo y su enumerador falla— y no había forma de
// distinguirlas desde afuera. El dato para hacerlo estaba guardado desde el principio: el agente
// manda su versión en cada latido y `LatirDevice` la escribe en `agent_version`. **Nadie la
// mostraba.** Una columna llena que no se podía leer.
//
// (Resultó ser lo primero: v0.106.0, tres commits anteriores a la enumeración.)
//
// La segunda mitad es la que suele salir mal: la AUSENCIA tiene que verse como ausencia. Un
// `agent_version: ""` en la fila de un Tier B —que no corre nuestro binario y nunca va a tener
// versión— se lee como «no se pudo averiguar», que es otra cosa.
//
// Sabotaje que la hace fallar: sacar el campo de la fila, o escribirlo siempre incluso vacío.
func TestElInventarioDiceQueBinarioCorreCadaMaquina(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	// Antes del primer latido no hay versión, y el campo NO está.
	fila := listarFlota(t, s, "casa")[0]
	if _, hay := fila["agent_version"]; hay {
		t.Errorf("una máquina que nunca latió trae agent_version: %+v", fila["agent_version"])
	}

	// El latido la trae.
	cuerpo := `{"version":"v0.106.0-28-gdf2ec21-rustdesk"}`
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("latido: status %d", code)
	}
	fila = listarFlota(t, s, "casa")[0]
	v, hay := fila["agent_version"].(string)
	if !hay {
		t.Fatal("después de latir con versión, el inventario no la muestra: " +
			"«corre un binario viejo» y «su enumerador está roto» quedan indistinguibles")
	}
	if v != "v0.106.0-28-gdf2ec21-rustdesk" {
		t.Errorf("agent_version = %q", v)
	}
}

// EL PANEL SIN PROYECTO PROPIO TIENE QUE PODER VER, Y ESO ESTUVO ROTO TODO EL TIEMPO.
//
// El principal del panel es `read: all` con `project_id: ""` — vacío A PROPÓSITO, porque no es de
// ningún cliente. Y las tres tools de lectura resolvían el proyecto cayendo al `ProjectID` del
// principal, así que las tres contestaban «no se pudo determinar de qué proyecto». Medido contra
// producción: `fleet_list`, `fleet_metrics` y `fleet_services` fallaban las tres igual.
//
// El síntoma MENTÍA, que es lo peor: el panel lo dibujaba como `estado: caido`, o sea «el cerebro
// se murió», con el cerebro latiendo y exportando 233 series. Un panel que culpa al backend por
// su propio problema de alcance manda a depurar el lugar equivocado.
//
// La salida NO es aflojar el WHERE por proyecto —eso sería un «listar todo» y se llevaría puesto
// el aislamiento entre tenants—: es enumerar los proyectos y consultar cada uno POR SEPARADO.
//
// Sabotaje que la hace fallar: volver `proyectosParaLeer` a `fleetReadScopeFor` a secas.
func TestUnPanelSinProyectoPropioVeTodoLoQueSuCredencialConcede(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio")
	enrolarDePrueba(t, s, "cliente-acme", "server-acme")

	// El principal del panel, tal cual está en producción.
	panel := &Principal{Name: "panel-central", Role: RoleReader, Read: ReadAll, Write: WriteNone,
		ProjectID: "", Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}}}

	res, e := callAsPrincipal(t, s, panel, "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatalf("el panel no pudo listar la flota: %+v — es exactamente el «estado: caido» de producción", e)
	}
	out := jsonOf(t, res)
	devs, _ := out["devices"].([]any)
	if len(devs) != 2 {
		t.Fatalf("el panel vio %d máquinas de 2: no está barriendo todos los proyectos que su credencial concede", len(devs))
	}

	// CADA FILA DICE DE QUÉ PROYECTO ES. Con read=all la tabla mezcla tenants, y una fila que no
	// lo dice invita a actuar sobre la máquina de otro cliente.
	vistos := map[string]string{}
	for _, d := range devs {
		f, _ := d.(map[string]any)
		nombre, _ := f["name"].(string)
		proy, hay := f["project"].(string)
		if !hay || proy == "" {
			t.Errorf("la fila de %q no dice de qué proyecto es: en una tabla mezclada eso es una trampa", nombre)
		}
		vistos[nombre] = proy
	}
	if vistos["pc-gio"] != "casa" || vistos["server-acme"] != "cliente-acme" {
		t.Errorf("las filas se atribuyeron mal: %+v", vistos)
	}

	// Y un principal ACOTADO sigue viendo lo suyo y nada más: el arreglo no puede haber abierto
	// una puerta lateral. Ésta es la mitad que impide «arreglarlo» sacando el WHERE.
	acotado := &Principal{Name: "dev", Role: RoleWriter, Read: ReadOwn, ProjectID: "casa"}
	res2, e2 := callAsPrincipal(t, s, acotado, "musubi_fleet_list", map[string]any{})
	if e2 != nil {
		t.Fatal(e2)
	}
	devs2, _ := jsonOf(t, res2)["devices"].([]any)
	if len(devs2) != 1 {
		t.Fatalf("un principal acotado vio %d máquinas: el arreglo cruzó tenants", len(devs2))
	}
}

// LO MISMO PARA SERVICIOS Y MÉTRICAS, PORQUE LAS TRES ESTABAN ROTAS.
//
// Arreglar sólo `fleet_list` dejaría el panel a medias: la tabla de máquinas se dibujaría y las
// dos secciones que el usuario pidió —los servicios y el estado— seguirían vacías, sin error.
//
// Sabotaje que la hace fallar: dejar `fleetReadScopeFor` en toolFleetServices o en toolFleetMetrics.
func TestElPanelTambienVeLosServiciosYLasMetricasDeTodosLosProyectos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// enrolarDePrueba devuelve el TOKEN, no el id: los servicios se reportan contra el device_id,
	// así que se resuelve por nombre en cada proyecto.
	enrolarDePrueba(t, s, "casa", "pc-gio")
	enrolarDePrueba(t, s, "cliente-acme", "server-acme")
	idDe := func(proyecto, nombre string) string {
		t.Helper()
		ds, err := s.engine.ListarDevices(proyecto, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			if d.Name == nombre {
				return d.ID
			}
		}
		t.Fatalf("no se encontró %q en %q", nombre, proyecto)
		return ""
	}
	for _, d := range []string{idDe("casa", "pc-gio"), idDe("cliente-acme", "server-acme")} {
		if _, _, err := s.engine.ReportarServicios(d, time.Now().UTC(), []fleet.ReporteServicio{
			{Nombre: "sshd", Clase: "systemd", Salud: fleet.SaludServicio{
				Tomada: time.Now(), Estado: fleet.EstadoCorriendo}}}); err != nil {
			t.Fatal(err)
		}
	}

	panel := &Principal{Name: "panel-central", Role: RoleReader, Read: ReadAll, Write: WriteNone,
		ProjectID: "", Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}}}

	res, e := callAsPrincipal(t, s, panel, "musubi_fleet_services", map[string]any{})
	if e != nil {
		t.Fatalf("el panel no pudo listar los servicios: %+v", e)
	}
	svs, _ := jsonOf(t, res)["services"].([]any)
	if len(svs) != 2 {
		t.Fatalf("el panel vio %d servicios de 2 (uno por proyecto)", len(svs))
	}
	for _, sv := range svs {
		f, _ := sv.(map[string]any)
		if p, hay := f["project"].(string); !hay || p == "" {
			t.Errorf("un servicio no dice de qué proyecto es: %+v", f)
		}
	}

	// Métricas: no hay muestras, así que la lista sale vacía — pero SIN ERROR, que es la
	// diferencia entre «todavía no reportaron» y «no se pudo determinar el proyecto».
	if _, e := callAsPrincipal(t, s, panel, "musubi_fleet_metrics", map[string]any{}); e != nil {
		t.Fatalf("el panel no pudo leer las métricas: %+v", e)
	}
}

// UNA MÁQUINA CON EL ACCESO PROHIBIDO NO ABRE PANTALLA, AUNQUE LA CAPACIDAD ESTÉ CONCEDIDA.
//
// Es la prueba de que el consentimiento es un EJE SEPARADO del permiso y no una capacidad más.
// El principal tiene `screen` sobre esa máquina —perfectamente concedido— y la sesión igual no se
// abre, porque el dueño de la máquina dijo que no. Un permiso del administrador no puede más que
// el candado de quien usa el equipo.
//
// Y el mensaje tiene que mandar a mirar el lugar correcto: si dijera «sin permiso» a secas,
// alguien revisaría `principals.yaml` durante media hora buscando algo que ya está.
//
// Sabotaje que la hace fallar: sacar el `switch consent` de toolFleetScreen.
func TestElConsentimientoProhibidoPesaMasQueLaCapacidadConcedida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// SE ENROLA CON `screen` DE VERDAD: el punto de la prueba es que la capacidad esté
	// perfectamente concedida y la sesión igual no se abra. Con una máquina sin `screen`, el
	// rechazo vendría de la compuerta de capacidad y la prueba pasaría por el motivo equivocado
	// — que es exactamente el error que tuvo su primera versión.
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "screen"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	ds, _ := s.engine.ListarDevices("casa", false)
	d := ds[0]
	if !d.Permite(fleet.CapScreen) {
		t.Fatal("la máquina de prueba no admite `screen`: la prueba no ejercitaría el consentimiento")
	}
	if _, err := s.engine.FijarConsentimiento(d.ID, fleet.ConsentimientoProhibido); err != nil {
		t.Fatal(err)
	}

	p := &Principal{Name: "operador", Role: RoleAdmin, Read: ReadAll, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}}}
	_, e := callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e == nil {
		t.Fatal("se abrió la pantalla de una máquina con el acceso PROHIBIDO: " +
			"el candado del dueño de la máquina no pesó nada")
	}
	if !strings.Contains(e.Message, "consentimiento") {
		t.Errorf("el error no manda a mirar el consentimiento, así que manda a revisar permisos "+
			"que ya están: %s", e.Message)
	}
}

// LA POLÍTICA DE CONSENTIMIENTO ES ADMIN, Y NO LA PUEDE TOCAR QUIEN ENTRA.
//
// Si quien tiene `screen` sobre una máquina pudiera aflojar su política de consentimiento, el eje
// entero sería decoración: la persona que va a entrar se autorizaría a sí misma a no avisar. Es
// la misma razón por la que `fleet_service_declare` es admin — escribe en el plano de control.
//
// Sabotaje que la hace fallar: cambiar la guarda de `isAdmin` por PuedeSobreDevice(CapScreen).
func TestLaPoliticaDeConsentimientoNoLaAflojaQuienEntra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio")

	// Tiene `screen` sobre todo y NO es admin.
	conPantalla := &Principal{Name: "soporte", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}}}
	_, e := callAsPrincipal(t, s, conPantalla, "musubi_fleet_consent",
		map[string]any{"device": "pc-gio", "grado": "libre"})
	if e == nil {
		t.Fatal("quien tiene `screen` pudo aflojar la política de consentimiento: se autorizó a sí mismo a no avisar")
	}

	// Un admin sí puede, y la respuesta dice lo GUARDADO y lo EFECTIVO.
	admin := &Principal{Name: "jefe", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}
	res, e2 := callAsPrincipal(t, s, admin, "musubi_fleet_consent",
		map[string]any{"device": "pc-gio", "grado": "pide"})
	if e2 != nil {
		t.Fatalf("un admin no pudo fijar la política: %+v", e2)
	}
	out := jsonOf(t, res)
	if out["guardado"] != "pide" {
		t.Errorf("guardado = %v", out["guardado"])
	}
	// LA DEGRADACIÓN SE DICE AL CONFIGURAR, no cuando alguien no puede entrar. La máquina de
	// prueba no declara poder preguntar, así que `pide` queda en `prohibido` — y quien lo
	// configuró tiene que enterarse ahí mismo.
	if out["efectivo"] != "prohibido" {
		t.Errorf("efectivo = %v, se esperaba `prohibido`: una máquina que no puede preguntar "+
			"endurece `pide`, y eso tiene que decirse al configurarlo", out["efectivo"])
	}
	if _, hay := out["nota"]; !hay {
		t.Error("no se explicó por qué el efectivo difiere del guardado")
	}

	// Y un grado que no existe se rechaza en vez de guardarse: guardarlo dejaría una fila que
	// dice una cosa y significa otra, porque el dominio lo resolvería al default.
	if _, e3 := callAsPrincipal(t, s, admin, "musubi_fleet_consent",
		map[string]any{"device": "pc-gio", "grado": "Pide"}); e3 == nil {
		t.Error("se aceptó un grado ilegible: la fila diría una cosa y el sistema haría otra")
	}
}
