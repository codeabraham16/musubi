package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// cerebroDeFlotaFalso levanta un central que responde tools/call con los textos dados por tool.
func cerebroDeFlotaFalso(t *testing.T, porTool map[string]string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var sobre struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&sobre)
		texto, hay := porTool[sobre.Params.Name]
		if !hay {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"message":"tool no disponible"}}`))
			return
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []map[string]any{{"type": "text", "text": texto}}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func pedirFlota(t *testing.T, relay *relayVivo) flotaRespuesta {
	t.Helper()
	rec := httptest.NewRecorder()
	handlerFlota(relay)(rec, httptest.NewRequest(http.MethodGet, "/api/flota", nil))
	var out flotaRespuesta
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body.String())
	}
	return out
}

// EL ESTADO ES LO PRIMERO QUE SE MIRA. Una flota vacía puede significar CINCO cosas distintas, y
// las cinco se dibujan igual si lo único que viaja es la lista.
//
// Sabotaje que la hace fallar: colapsar los estados en «devolver la lista vacía».
func TestUnaFlotaVaciaDistingueSusCincoCausas(t *testing.T) {
	// 1 · sin enlace al central.
	if got := pedirFlota(t, nil); got.Estado != "apagado" || !strings.Contains(got.Detalle, "MUSUBI_BRAIN_URL") {
		t.Errorf("sin relay: estado=%q detalle=%q", got.Estado, got.Detalle)
	}

	// 2 · el central no responde.
	muerto := cerebroDeFlotaFalso(t, nil)
	base := muerto.URL
	muerto.Close()
	if got := pedirFlota(t, &relayVivo{base: base, token: "tok"}); got.Estado != "caido" {
		t.Errorf("central caído: estado=%q", got.Estado)
	}

	// 3 · la credencial no ve ninguna máquina.
	sinPermiso := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"project_id":"casa","total":0,"devices":[],"sin_permiso":3}`})
	got := pedirFlota(t, &relayVivo{base: sinPermiso.URL, token: "tok"})
	if got.Estado != "sin_permiso" {
		t.Errorf("sin concesiones: estado=%q", got.Estado)
	}
	if got.SinPermiso != 3 || !strings.Contains(got.Detalle, "principals.yaml") {
		t.Errorf("no dice cuántas ni dónde arreglarlo: %+v", got)
	}

	// 4 · no hay máquinas enroladas — que NO es lo mismo que lo anterior.
	vacio := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"project_id":"casa","total":0,"devices":[]}`})
	got4 := pedirFlota(t, &relayVivo{base: vacio.URL, token: "tok"})
	if got4.Estado != "vacio" || !strings.Contains(got4.Detalle, "fleet_enroll") {
		t.Errorf("flota vacía: estado=%q detalle=%q", got4.Estado, got4.Detalle)
	}

	// 5 · hay flota.
	viva := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"devices":[{"name":"pc","online":true,"caps":["metrics"],"puedo":["metrics"]}]}`})
	if got := pedirFlota(t, &relayVivo{base: viva.URL, token: "tok"}); got.Estado != "vivo" {
		t.Errorf("con flota: estado=%q", got.Estado)
	}
}

// I5 — el panel NO inventa permisos: pregunta por las MISMAS tools. Lo que la compuerta no deja
// ver, el panel no lo ve.
//
// Sabotaje: agregar un endpoint «para el panel» que se saltee la compuerta.
func TestElPanelPreguntaPorLasMismasToolsYNoInventaUnaRutaAparte(t *testing.T) {
	var pedidas []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/mcp") {
			t.Errorf("el panel pidió a %q en vez de a /mcp: hay una segunda ruta de datos", r.URL.Path)
		}
		var sobre struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&sobre)
		pedidas = append(pedidas, sobre.Params.Name)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"devices\":[{\"name\":\"pc\"}]}"}]}}`))
	}))
	defer ts.Close()

	pedirFlota(t, &relayVivo{base: ts.URL, token: "tok"})
	// LA LISTA ES EXHAUSTIVA EN LAS DOS DIRECCIONES, y por eso hay que tocarla al agregar una
	// llamada: no sólo prohíbe pedir algo raro, también EXIGE que se pidan las cuatro. Agregar
	// una llamada al panel sin declararla acá lo rompe, que es lo que se busca — una ruta de
	// datos nueva no puede entrar sin que alguien la mire.
	quiero := map[string]bool{
		"musubi_fleet_list":     true,
		"musubi_fleet_metrics":  true,
		"musubi_fleet_services": true,
		// Las sesiones: quién está adentro de cada máquina. Es lo que el plano de ENTRAR
		// construyó y ninguna pantalla mostraba.
		"musubi_fleet_sessions": true,
	}
	for _, p := range pedidas {
		if !quiero[p] {
			t.Errorf("el panel llamó a %q, que no es una de las tools de flota", p)
		}
		delete(quiero, p)
	}
	if len(quiero) != 0 {
		t.Errorf("el panel no pidió %v", quiero)
	}
}

// El token NUNCA sale hacia el navegador: viaja del panel al cerebro y nada más.
//
// Sabotaje: incluir el token en flotaRespuesta «para que la página pueda refrescar sola».
func TestElTokenNoViajaAlNavegador(t *testing.T) {
	ts := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"devices":[{"name":"pc","online":true}]}`})
	rec := httptest.NewRecorder()
	handlerFlota(&relayVivo{base: ts.URL, token: "SECRETO-DEL-CEREBRO"})(rec, httptest.NewRequest(http.MethodGet, "/api/flota", nil))
	if strings.Contains(rec.Body.String(), "SECRETO-DEL-CEREBRO") {
		t.Fatalf("el token viajó al navegador:\n%s", rec.Body.String())
	}
}

// Si las MÉTRICAS fallan, la tabla se dibuja igual con el inventario: perder los números es
// molesto, perder la lista de máquinas es quedarse a oscuras.
//
// Sabotaje: propagar el error de fleet_metrics → un problema de permisos de métricas borra la
// flota entera de la pantalla.
func TestSiFallanLasMetricasIgualSeVeLaFlota(t *testing.T) {
	// Sólo responde fleet_list; fleet_metrics devuelve error.
	ts := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"devices":[{"name":"pc-gio","online":true,"caps":["metrics"],"puedo":[]}]}`})
	got := pedirFlota(t, &relayVivo{base: ts.URL, token: "tok"})
	if got.Estado != "vivo" || len(got.Equipos) != 1 {
		t.Fatalf("sin métricas se perdió la flota: %+v", got)
	}
	if got.Equipos[0]["con_metricas"] != false {
		t.Errorf("no se marca que la máquina va sin métricas: %+v", got.Equipos[0])
	}
	// Y NO se inventan ceros para los campos que no vinieron.
	for _, campo := range []string{"cpu_pct", "mem_pct"} {
		if v, hay := got.Equipos[0][campo]; hay && v != nil {
			t.Errorf("se inventó %s=%v sin haberlo medido", campo, v)
		}
	}
}

// Una máquina que está en el inventario y NO en las métricas se marca como tal, en vez de
// dibujarse con ceros. Es la compuerta funcionando, no un error.
func TestUnaMaquinaSinMetricasSeDistingueDeUnaEnCero(t *testing.T) {
	ts := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list":    `{"devices":[{"name":"con","online":true},{"name":"sin","online":true}]}`,
		"musubi_fleet_metrics": `{"devices":[{"name":"con","cpu_pct":42.5,"mem_pct":10}]}`})
	got := pedirFlota(t, &relayVivo{base: ts.URL, token: "tok"})
	if len(got.Equipos) != 2 {
		t.Fatalf("equipos = %d", len(got.Equipos))
	}
	por := map[string]map[string]any{}
	for _, e := range got.Equipos {
		por[e["name"].(string)] = e
	}
	if por["con"]["con_metricas"] != true || por["con"]["cpu_pct"] != 42.5 {
		t.Errorf("la máquina CON métricas no las trajo: %+v", por["con"])
	}
	if por["sin"]["con_metricas"] != false {
		t.Errorf("la máquina SIN métricas no se marcó: %+v", por["sin"])
	}
	if v, hay := por["sin"]["cpu_pct"]; hay && v != nil {
		t.Errorf("se inventó un cpu_pct para la máquina sin métricas: %v", v)
	}
}

// La página no lleva NI UNA línea de three.js, y no toca el bundle que la CI compara byte a byte.
func TestLaPaginaDeFlotaNoDependeDelBundleWebGL(t *testing.T) {
	pagina, err := dashboardAssets.ReadFile("assets/flota.html")
	if err != nil {
		t.Fatalf("la página no está embebida: %v", err)
	}
	html := string(pagina)
	for _, prohibido := range []string{"three", "dashboard.bundle.js", "webgl", "WebGL"} {
		if strings.Contains(strings.ToLower(html), strings.ToLower(prohibido)) &&
			!strings.Contains(html, "sin una línea de three.js") &&
			!strings.Contains(html, "bundle WebGL") {
			t.Errorf("la página de flota depende de %q", prohibido)
		}
	}
	// Y el principio del track llega hasta el píxel: hay un camino para «no medido».
	if !strings.Contains(html, "const ND") {
		t.Error("la página no tiene una representación para lo NO MEDIDO: dibujaría ceros")
	}
	if !strings.Contains(html, "sin_permiso") {
		t.Error("la página no distingue «no podés ver» de «no hay»")
	}
}

// ── S9b · A21 + A23: se llega, se vuelve, y se ve lo automático ─────────────────────────────

// A21 — A LA FLOTA SE TIENE QUE PODER LLEGAR SIN SABERSE LA URL.
//
// Hasta acá `/flota` existía y no había un solo enlace hacia ella: se llegaba escribiendo la
// dirección. Una pantalla a la que sólo se llega de memoria es una pantalla que nadie mira — y
// desde S10 esa pantalla es donde se ve qué máquinas tienen algo que actúa solo.
//
// EL ENLACE VA EN LA CÁSCARA (dashboard.html), NO EN EL BUNDLE, y ésa es la mitad interesante:
// la CI reconstruye dashboard.bundle.js desde src/ y exige que no cambie ni un byte, así que
// tocarlo para agregar un `<a>` habría sido un riesgo gratuito. La cáscara no entra en esa
// verificación. (El motivo que este cabo tenía anotado en ABIERTO.md —«habría que tocar el
// bundle»— era simplemente incorrecto.)
//
// Sabotaje que la hace fallar: sacar el enlace de dashboard.html.
func TestSePuedeLlegarALaFlotaYVolverSinEscribirLaURL(t *testing.T) {
	ida := string(assetsFS(t, "assets/dashboard.html"))
	if !strings.Contains(ida, `href="/flota"`) {
		t.Error("el panel del cerebro no enlaza a /flota: a la pantalla de la flota sólo se llega escribiendo la URL")
	}
	vuelta := string(assetsFS(t, "assets/flota.html"))
	if !strings.Contains(vuelta, `href="/"`) {
		t.Error("el panel de flota no vuelve al del cerebro: un enlace de ida sin vuelta deja a alguien usando el botón del navegador para algo que la página tendría que ofrecer")
	}
}

// El bundle WebGL NO se toca para nada de esto. La CI ya lo verifica reconstruyéndolo, pero esta
// prueba corre en cada `go test` y falla en el momento, no veinte minutos después en el pipeline.
//
// Sabotaje que la hace fallar: meter la navegación dentro del bundle.
func TestElBundleWebGLNoSabeNadaDeLaFlota(t *testing.T) {
	b := string(assetsFS(t, "assets/dashboard.bundle.js"))
	if strings.Contains(b, "/api/flota") || strings.Contains(b, "politicas_activas") {
		t.Error("el bundle WebGL menciona la flota: la navegación y la tabla van en HTML plano, fuera del bundle cuyos bytes compara la CI")
	}
}

// A23 — la página dibuja lo que S10 volvió necesario ver, y distingue los tres estados.
//
// Sabotaje que la hace fallar: dibujar la columna sin distinguir la política inerte.
func TestLaPaginaDeFlotaDibujaLoAutomaticoYMarcaLoInerte(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	for _, quiero := range []struct{ frag, porque string }{
		{"politicas_activas", "sin el conteo no se distingue una máquina con auto-heal de una sin él"},
		{"puede_actuar", "una política inerte se ve idéntica a una que funciona si esto no se dibuja"},
		{"inerte", "el estado inerte necesita su propia marca visual"},
		{"function esc(", "el nombre y el argv de una política salen de un archivo de configuración y se interpolan en un atributo"},
	} {
		if !strings.Contains(p, quiero.frag) {
			t.Errorf("flota.html no contiene %q: %s", quiero.frag, quiero.porque)
		}
	}
	// Una sola fuente para la columna: si el `⚙` apareciera suelto en el HTML además de en la
	// función, habría dos formas de dibujar lo mismo y una se quedaría vieja.
	if n := strings.Count(p, "function automatico("); n != 1 {
		t.Errorf("hay %d definiciones de automatico(): tiene que haber exactamente una", n)
	}
}

// assetsFS lee un asset embebido, para que estas pruebas miren EXACTAMENTE lo que se sirve y no
// una copia del disco que podría no estar embebida.
func assetsFS(t *testing.T, ruta string) []byte {
	t.Helper()
	b, err := dashboardAssets.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer el asset embebido %q: %v", ruta, err)
	}
	return b
}

// A13 — el panel dibuja si el id de pantalla es de fiar.
//
// Sabotaje que la hace fallar: dibujar el id sin distinguir el caso ambiguo.
func TestLaPaginaDeFlotaDistingueUnIdDePantallaAmbiguo(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	for _, quiero := range []struct{ frag, porque string }{
		{"rustdesk_id_ambiguo", "sin esto, una máquina con id duplicado se ve igual que una sana"},
		{"rustdesk_id_cambio", "un id que cambió merece verse: o se reinstaló la máquina, o alguien miente"},
		{"moneda al aire", "el aviso tiene que decir POR QUÉ importa, no sólo que pasa algo"},
	} {
		if !strings.Contains(p, quiero.frag) {
			t.Errorf("flota.html no contiene %q: %s", quiero.frag, quiero.porque)
		}
	}
	if n := strings.Count(p, "function pantalla("); n != 1 {
		t.Errorf("hay %d definiciones de pantalla(): tiene que haber exactamente una", n)
	}
	// El id y el nombre de la máquina que colisiona vienen del reporte de una MÁQUINA, o sea que
	// son texto ajeno interpolado en un atributo: tienen que pasar por esc().
	if !strings.Contains(p, "esc(e.rustdesk_id)") {
		t.Error("el id de RustDesk se interpola sin escapar, y ese dato lo reporta la propia máquina")
	}
}

// A18 — EL PANEL NO PUEDE DECIR OTRA COSA QUE EL INVENTARIO.
//
// La trampa concreta: un Tier C no tiene rustdesk_id, así que caía en el `—` de «sin dato» y se
// leía como un Tier A al que todavía no le llegó el id — o sea, «ya va a aparecer». Y no va a
// aparecer nunca: es otro motor. La rama tiene que ir ANTES de ese `—`.
//
// Sabotaje: quitar la rama de pantalla_sin_motor, o ponerla después del `if (!e.rustdesk_id)`.
func TestLaPaginaDeFlotaDistingueUnaPantallaSinMotor(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	if !strings.Contains(p, "pantalla_sin_motor") {
		t.Fatal("flota.html no dibuja pantalla_sin_motor: una capacidad inerte se ve igual que una viva")
	}
	if !strings.Contains(p, "scrcpy") {
		t.Error("el aviso no dice cuál es el motor que falta, que es lo único que vuelve accionable el dato")
	}
	cuerpo := p[strings.Index(p, "function pantalla("):]
	iMotor := strings.Index(cuerpo, "pantalla_sin_motor")
	iSinID := strings.Index(cuerpo, "if (!e.rustdesk_id)")
	if iMotor < 0 || iSinID < 0 || iMotor > iSinID {
		t.Error("la rama de `sin motor` va DESPUÉS del `—` de «sin id», así que un Tier C nunca la alcanza: se dibuja como «todavía no llegó el id»")
	}
}

// ── S12 · los SERVICIOS en el panel ─────────────────────────────────────────────────────────

// SI FALLA LA TOOL DE SERVICIOS, LA FLOTA SE SIGUE VIENDO.
//
// Molde de TestSiFallanLasMetricasIgualSeVeLaFlota, y por el mismo motivo: propagar este error
// haría que un problema de permisos sobre los servicios borre la FLOTA entera de la pantalla.
//
// Sabotaje que la hace fallar: propagar el error de la tercera llamada desde handlerFlota.
func TestSiFallaLaToolDeServiciosIgualSeVeLaFlota(t *testing.T) {
	// El cerebro responde list y metrics; fleet_services devuelve error.
	ts := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list":    `{"devices":[{"name":"pc-gio","online":true,"caps":["metrics"],"puedo":["metrics"]}]}`,
		"musubi_fleet_metrics": `{"devices":[{"name":"pc-gio","cpu_pct":10}]}`})
	got := pedirFlota(t, &relayVivo{base: ts.URL, token: "tok"})
	if got.Estado != "vivo" || len(got.Equipos) != 1 {
		t.Fatalf("sin la tool de servicios se perdió la flota: %+v", got)
	}
	// Y la ausencia se DECLARA: sin `con_servicios`, la página dibuja «—» y no «0 servicios».
	if v, hay := got.Equipos[0]["con_servicios"]; hay && v != false {
		t.Errorf("con_servicios = %v tras un error: la máquina se dibujaría como «0 servicios», que es un dato que nadie midió", v)
	}
	if v, hay := got.Equipos[0]["servicios"]; hay && v != nil {
		t.Errorf("se inventaron servicios sin haberlos consultado: %v", v)
	}
}

// LOS SERVICIOS SE AGRUPAN POR MÁQUINA, Y «SIN NINGUNO» NO ES «NO SABEMOS».
//
// Son los dos casos que la página dibuja distinto: una máquina que la tool contestó con cero
// servicios lleva `con_servicios: true` y una lista vacía; una que no vino en la respuesta lleva
// lo mismo (la tool contestó, esa máquina no tiene nada). Lo que NO puede pasar es que un error
// de la tool se confunda con «cero», y eso lo cubre la prueba de arriba.
//
// Sabotaje: agrupar por el `name` del servicio en vez de por su `device` — los servicios de una
// máquina aparecen colgados de la otra.
func TestLosServiciosSeAgrupanPorSuMaquina(t *testing.T) {
	ts := cerebroDeFlotaFalso(t, map[string]string{
		"musubi_fleet_list": `{"devices":[{"name":"nas","online":true},{"name":"pc-gio","online":true}]}`,
		"musubi_fleet_services": `{"services":[
			{"nombre":"postgres","device":"nas","estado":"corriendo","fresco":true},
			{"nombre":"redis","device":"nas","estado":"fallado","fresco":true}]}`})
	got := pedirFlota(t, &relayVivo{base: ts.URL, token: "tok"})
	por := map[string]map[string]any{}
	for _, e := range got.Equipos {
		por[e["name"].(string)] = e
	}
	svs, _ := por["nas"]["servicios"].([]any)
	if len(svs) != 2 {
		t.Fatalf("`nas` quedó con %d servicios de 2: %+v", len(svs), por["nas"])
	}
	if por["nas"]["con_servicios"] != true {
		t.Error("no se marcó que `nas` tiene datos de servicios")
	}
	// La otra máquina: la tool contestó y ella no tiene ninguno. Eso ES un dato, y es distinto de
	// que la tool no haya contestado.
	if por["pc-gio"]["con_servicios"] != true {
		t.Error("`pc-gio` no quedó marcada: la tool contestó, así que «no tiene ninguno» es un dato")
	}
	if svs, _ := por["pc-gio"]["servicios"].([]any); len(svs) != 0 {
		t.Errorf("`pc-gio` quedó con servicios ajenos: %+v", svs)
	}
}

// LA PÁGINA DISTINGUE A SIMPLE VISTA UN SERVICIO SANO DE UNO QUE NO REPORTA HACE RATO, Y NUNCA
// DIBUJA UN `desconocido` COMO `detenido`.
//
// Es el invariante del slice llegando hasta el píxel. Tres cosas que la columna tiene que separar:
// «no sabemos» (el guion), «cero servicios» (un dato) y el estado de cada uno — con el FRESCOR
// como eje aparte del estado, porque un «corriendo» de hace dos días no es un «corriendo».
//
// Sabotaje que la hace fallar: usar la misma marca (o la misma clase CSS) para `desconocido` y
// `detenido`; o sacar el `if (!e.con_servicios) return ND` y dibujar 0.
func TestLaPaginaDeFlotaNoDibujaUnServicioDesconocidoComoDetenido(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	for _, quiero := range []struct{ frag, porque string }{
		{"con_servicios", "sin esta llave, «la tool no contestó» y «no corre nada» se dibujan igual"},
		{"if (!e.con_servicios) return ND", "la ausencia de datos tiene que dar un guion, no un cero"},
		{"s.fresco", "un «corriendo» de hace dos días no es un «corriendo»: el frescor es un eje aparte"},
		{"rancio", "«sin noticias» necesita su propia marca visual, distinta de sana y de fallada"},
		{"esc(s.nombre)", "el nombre del servicio lo reporta la propia máquina y las filas se arman con innerHTML"},
	} {
		if !strings.Contains(p, quiero.frag) {
			t.Errorf("flota.html no contiene %q: %s", quiero.frag, quiero.porque)
		}
	}
	// UNA sola fuente para la columna: dos formas de dibujar lo mismo dejan una vieja.
	if n := strings.Count(p, "function servicios("); n != 1 {
		t.Errorf("hay %d definiciones de servicios(): tiene que haber exactamente una", n)
	}
	// LAS CUATRO MARCAS SON DISTINTAS ENTRE SÍ. Es lo que impide que `desconocido` se dibuje como
	// `detenido`: si dos estados compartieran glifo, la columna mentiría en silencio.
	i := strings.Index(p, "const marca = {")
	if i < 0 {
		t.Fatal("flota.html no declara la tabla de marcas por estado")
	}
	tabla := p[i : strings.Index(p[i:], "}")+i]
	for _, estado := range []string{"corriendo", "detenido", "fallado", "desconocido"} {
		if !strings.Contains(tabla, estado+":") {
			t.Errorf("el estado %q no tiene marca propia: %s", estado, tabla)
		}
	}
	glifos := map[string]bool{}
	for _, campo := range strings.Split(tabla[strings.Index(tabla, "{")+1:], ",") {
		partes := strings.SplitN(campo, ":", 2)
		if len(partes) != 2 {
			continue
		}
		g := strings.Trim(strings.TrimSpace(partes[1]), "'\"")
		if g == "" {
			continue
		}
		if glifos[g] {
			t.Errorf("dos estados comparten la marca %q: un `desconocido` se dibujaría como otra cosa\n%s", g, tabla)
		}
		glifos[g] = true
	}
	if len(glifos) != 4 {
		t.Errorf("se declararon %d marcas distintas, esperaba 4: %s", len(glifos), tabla)
	}
	// Y NINGÚN BOTÓN: el invariante I4 del panel sigue en pie en un slice de visualización.
	if strings.Contains(p, "<button") || strings.Contains(p, "onclick") {
		t.Error("la página de flota ganó un botón: reiniciar un servicio se hace con musubi_fleet_exec, que deja su línea en la bitácora")
	}
}

// LA PÁGINA DIBUJA EL EJE DE ACCESO, Y LO DISTINGUE DE «qué puedo hacer yo».
//
// Son dos preguntas distintas y la columna `puedo` sólo contesta la primera. Sin la de acceso, un
// operador ve que PUEDE abrir una pantalla y no ve que la máquina la tiene PROHIBIDA — y se entera
// cuando la sesión no abre.
//
// Las tres cosas que la página tiene que distinguir, y que son fáciles de colapsar en una:
//
//	declarado y efectivo iguales   → el grado, a secas
//	declarado ≠ efectivo           → el efectivo CON marca: se endureció, y hay que poder verlo antes
//	nada declarado                 → el default, marcado como heredado: no es lo mismo que decidirlo
//
// Sabotaje que la hace fallar: dibujar sólo `consentimiento` (el declarado), que es nil en la
// mayoría de las máquinas y dejaría la columna vacía justo donde el default está rigiendo.
func TestLaPaginaDeFlotaDibujaElEjeDeAcceso(t *testing.T) {
	b, err := os.ReadFile("assets/flota.html")
	if err != nil {
		t.Fatalf("falta la página de flota: %v", err)
	}
	html := string(b)

	if !strings.Contains(html, "<th>acceso</th>") {
		t.Error("la tabla no tiene columna de acceso: se ve qué podés hacer vos y no qué se le " +
			"debe a quien usa la máquina")
	}
	// El EFECTIVO es lo que rige; dibujar sólo el declarado dejaría la columna vacía en toda
	// máquina donde nadie decidió nada, que hoy son todas.
	if !strings.Contains(html, "consentimiento_efectivo") {
		t.Error("la página no mira el consentimiento EFECTIVO: dibujaría vacío donde rige el default")
	}
	// Y tiene que poder marcar la diferencia entre declarado y efectivo, que es cuando un `pide`
	// se endureció a `prohibido`.
	if !strings.Contains(html, "declarado !== ef") {
		t.Error("la página no distingue el declarado del efectivo: un `pide` endurecido a " +
			"`prohibido` se vería igual que un `prohibido` decidido, y la degradación se " +
			"descubriría el día que una sesión no abre")
	}
	// La versión del agente, que es lo que distingue «binario viejo» de «enumerador roto».
	if !strings.Contains(html, "<th>agente</th>") || !strings.Contains(html, "e.agent_version") {
		t.Error("la página no muestra la versión del agente: una máquina en línea con cero " +
			"servicios tiene dos causas opuestas y no se distinguen sin este dato")
	}
	// El pie NO puede seguir diciendo sólo «sólo lectura» sin explicar el eje nuevo: una columna
	// que aparece sin explicación se ignora.
	if !strings.Contains(html, "musubi_fleet_consent") {
		t.Error("el pie no dice cómo se cambia la política de acceso")
	}
}

// EL SUB-PANEL POR MÁQUINA: TODO LO DE UNA MÁQUINA EN UN LUGAR.
//
// Es lo que la tabla no puede mostrar sin volverse ilegible, y contesta tres preguntas que hasta
// ahora vivían en tres tools distintas: cómo está, qué corre adentro, y quién puede entrar.
//
// Las cuatro decisiones que se custodian, y las cuatro tienen un modo de fallo real:
//
//  1. Se abre DEBAJO de la fila, no en un modal. Una máquina no se mira sola, se mira contra las
//     otras; un modal esconde la tabla justo cuando alguien compara.
//  2. Se pueden tener VARIOS abiertos. Cerrar el anterior al abrir el siguiente convierte una
//     comparación en un ejercicio de memoria.
//  3. Los servicios se agrupan POR CLASE y lo roto va primero. Cincuenta y cuatro en una lista
//     plana no se leen.
//  4. «No se pudo consultar» se dibuja distinto de «no hay». Un panel mudo que se ve tranquilo es
//     el peor resultado posible.
//
// Sabotaje que la hace fallar: cerrar los otros cajones al abrir uno; dibujar los servicios sin
// agrupar; colapsar la ausencia de dato con el cero.
func TestElSubPanelPorMaquinaMuestraTodoEnUnLugar(t *testing.T) {
	b, err := os.ReadFile("assets/flota.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)

	if !strings.Contains(html, "function cajon(e)") {
		t.Fatal("no hay sub-panel por máquina")
	}
	// Las tres secciones contestan tres preguntas distintas y por eso están separadas.
	for _, fn := range []string{"seccionVitales", "seccionServicios", "seccionAcceso"} {
		if !strings.Contains(html, "function "+fn) {
			t.Errorf("falta la sección %s", fn)
		}
	}
	// SE ABRE DEBAJO DE LA FILA: la fila de detalle es un <tr> hermano, no un modal.
	if !strings.Contains(html, `<tr class="det"`) {
		t.Error("el detalle no es una fila hermana: si es un modal, esconde la tabla justo cuando " +
			"alguien está comparando dos máquinas")
	}
	// VARIOS A LA VEZ: el clic ALTERNA el suyo y no toca los demás. Un `hidden = true` sobre
	// todos los otros sería el sabotaje.
	if !strings.Contains(html, "det.hidden = !det.hidden") {
		t.Error("el detalle no alterna sólo el suyo: cerrar el anterior al abrir el siguiente " +
			"convierte una comparación en un ejercicio de memoria")
	}
	// Los servicios agrupados por clase, y lo roto primero.
	// SE MIRA LA AGRUPACIÓN, NO EL NOMBRE DE LA VARIABLE. Buscar `porClase` a secas dejaba
	// pasar un borrado parcial: la variable seguía nombrada más abajo y el sabotaje quedaba en
	// verde. Lo que hay que exigir es la línea que AGRUPA.
	if !strings.Contains(html, "porClase[s.clase") {
		t.Error("los servicios no se agrupan por su clase: 54 en una lista plana no se leen, y " +
			"agrupados por quién los corre se leen de un vistazo")
	}
	if !strings.Contains(html, "'fallado'") || !strings.Contains(html, "'detenido'") {
		t.Error("los servicios no priorizan lo roto: es la fila que uno vino a buscar")
	}
	// «No se pudo consultar» distinto de «no hay», en las tres secciones que pueden faltar.
	for _, guarda := range []string{"e.con_metricas", "e.con_servicios", "e.con_sesiones"} {
		if !strings.Contains(html, guarda) {
			t.Errorf("el sub-panel no distingue «no se pudo consultar» de «no hay» en %s: "+
				"un panel mudo se leería como uno tranquilo", guarda)
		}
	}
}

// EL PANEL PIDE LAS SESIONES, QUE ES LO QUE EL PLANO DE ENTRAR CONSTRUYÓ Y NADIE MOSTRABA.
//
// Y su error se ignora, igual que el de métricas y servicios: un problema de permisos sobre el
// plano de entrar no puede borrar la FLOTA de la pantalla. Es el mismo bug que la decisión de las
// métricas ya evitó, y que se repite con cada llamada nueva si nadie lo cuida.
//
// Sabotaje que la hace fallar: propagar el error de fleet_sessions al estado de la respuesta.
func TestElPanelPideLasSesionesYSuErrorNoBorraLaFlota(t *testing.T) {
	b, err := os.ReadFile("flota.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, `"musubi_fleet_sessions"`) {
		t.Error("el panel no pide las sesiones: quién está adentro de cada máquina no se ve en ningún lado")
	}
	// La llave se pone SIEMPRE que la tool haya contestado, incluso vacía: «nadie adentro» es un
	// dato y «no se pudo preguntar» es otro.
	if !strings.Contains(src, `e["con_sesiones"] = true`) {
		t.Error("no se marca que la consulta de sesiones respondió: una lista vacía y una consulta " +
			"fallida se dibujarían igual")
	}
	// El error NO se propaga: se pide dentro de un `if ... err == nil`.
	if !strings.Contains(src, `if ses, err := llamarToolDelCerebro(r, cli, relay, "musubi_fleet_sessions"`) {
		t.Error("el error de las sesiones no está acotado: podría borrar la flota entera de la pantalla")
	}
}

// A38 — los dos campos que U1 absorbió y el panel no dibujaba.
//
// `num_procesos` y `mem_libre` llegan a `musubi_fleet_metrics` y a Prometheus desde U1, y el panel
// —que es donde alguien los mira— no los tenía. Es el patrón que este track viene persiguiendo:
// datos guardados que ninguna interfaz muestra.
//
// Sabotaje que la hace fallar: sacar la fila de `procesos`/`RAM libre` de seccionVitales.
// Sabotaje que la hace fallar: dibujar `mem_libre` con num() en vez de bytes() (un GiB se vería
// como 1073741824).
func TestElPanelDibujaLosProcesosYLaMemoriaLibre(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	for _, quiero := range []struct{ frag, porque string }{
		{"e.num_procesos", "el conteo de procesos llega desde U1 y no se dibujaba en ningún lado"},
		{"bytes(e.mem_libre)", "la RAM libre son bytes crudos: sin formatear se lee 1073741824 y nadie la mira"},
		{"function bytes(", "el formateo de bytes tiene que existir una sola vez, no repetido en cada celda"},
	} {
		if !strings.Contains(p, quiero.frag) {
			t.Errorf("flota.html no contiene %q: %s", quiero.frag, quiero.porque)
		}
	}
	// EL NULL NO PUEDE DIBUJARSE COMO CERO, que es la regla central del track llevada al panel:
	// `mem_libre` no existe en Windows ni en macOS. Si `bytes()` no chequea null, un Windows
	// mostraría «0 B» de RAM libre — que se lee como una máquina a punto de morir.
	i := strings.Index(p, "function bytes(")
	if i < 0 {
		t.Fatal("no hay función bytes(): el chequeo de abajo no mira nada")
	}
	cuerpo := p[i:]
	if fin := strings.Index(cuerpo, "\nfunction "); fin > 0 {
		cuerpo = cuerpo[:fin]
	}
	if !strings.Contains(cuerpo, "=== null") || !strings.Contains(cuerpo, "ND") {
		t.Errorf("bytes() no devuelve el marcador de «sin dato» ante un null: un Windows sin "+
			"MemFree dibujaría 0 B, que se lee como una máquina sin RAM libre.\n%s", cuerpo)
	}
	// Y el 0 MEDIDO sí se dibuja: un disco lleno tiene 0 disponibles de verdad. Si bytes()
	// tratara el 0 como ausencia, taparía justo el número por el que suena la alarma.
	if strings.Contains(cuerpo, "if (!v)") || strings.Contains(cuerpo, "n === 0") {
		t.Error("bytes() trata el 0 como ausencia: un 0 medido es un dato, y es el que más importa")
	}
}

// Fase 4 — el panel dibuja el RENDIMIENTO, que para un bot es lo único que importa.
//
// Un bot puede tener su proceso perfectamente vivo mientras contesta mal todas las consultas: el
// estado del supervisor dice «corriendo» y no está sano. Si el panel no lo dibuja, el dato queda
// guardado y nadie lo ve — el patrón exacto que este track persigue.
//
// Sabotaje que la hace fallar: sacar rendimientoTexto del título del chip.
// Sabotaje que la hace fallar: sacar la marca de tasa alta del chip.
func TestElPanelDibujaElRendimientoDeUnServicio(t *testing.T) {
	p := string(assetsFS(t, "assets/flota.html"))
	for _, quiero := range []struct{ frag, porque string }{
		{"function rendimientoTexto(", "sin la función el rendimiento no se dibuja en ningún lado"},
		{"rendimientoTexto(s.rendimiento)", "la función tiene que estar ENGANCHADA al chip, no sólo definida"},
		{"function tasaAlta(", "un servicio que corre y falla la mitad de las veces no está sano, y el estado del supervisor no lo dice"},
		{"tasaAlta(s.rendimiento)", "la marca tiene que estar enganchada al chip"},
		{"TASA_ERROR_ALTA", "el umbral tiene que estar nombrado, no metido a mano en una comparación"},
	} {
		if !strings.Contains(p, quiero.frag) {
			t.Errorf("flota.html no contiene %q: %s", quiero.frag, quiero.porque)
		}
	}

	// EL CERO SE DIBUJA Y EL AUSENTE NO. Es al revés que el resto del panel y es lo que sostiene
	// todo el diseño del colector: `atendidas: 0` significa «miré y no pasó nada» —el latido que
	// distingue un bot callado de un colector muerto— y tiene que verse. `rendimiento` ausente
	// significa «no se midió», que es lo normal en un servicio de systemd.
	i := strings.Index(p, "function rendimientoTexto(")
	cuerpo := p[i:]
	if fin := strings.Index(cuerpo, "\nfunction "); fin > 0 {
		cuerpo = cuerpo[:fin]
	}
	if !strings.Contains(cuerpo, "if (!r) return") {
		t.Errorf("rendimientoTexto no distingue el ausente: un servicio de systemd sin rendimiento "+
			"dibujaría un texto vacío o «undefined».\n%s", cuerpo)
	}
	if strings.Contains(cuerpo, "if (!r.atendidas)") || strings.Contains(cuerpo, "r.atendidas === 0 ? ''") {
		t.Errorf("rendimientoTexto trata el 0 como ausencia: ese cero es el latido del colector, "+
			"y sin él «el bot no tuvo consultas» y «el colector murió» se ven igual.\n%s", cuerpo)
	}
	// La tasa viene del SERVIDOR y puede ser null. Recalcularla acá repetiría en un segundo lugar
	// la decisión de que sobre cero atendidas no hay tasa — y ahí es donde se olvida.
	if strings.Contains(cuerpo, "r.fallidas / r.atendidas") || strings.Contains(cuerpo, "r.fallidas/r.atendidas") {
		t.Errorf("el panel recalcula la tasa de error en vez de usar la del servidor: sobre cero "+
			"atendidas no hay tasa, y esa decisión no puede vivir en dos lados.\n%s", cuerpo)
	}
}
