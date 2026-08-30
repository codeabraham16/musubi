package mcp

// Pruebas de la CRONOLOGÍA (fase 5 · S11) contra la compuerta de verdad.
//
// Lo que se custodia acá es lo que el dominio no puede: que la compuerta sea POR HECHO, que la
// ventana se aplique en la consulta y no después, y que una máquina revocada siga siendo
// auditable.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// conCaps arma un principal acotado a un proyecto con las capacidades de flota que se le den.
func conCaps(proyecto string, grants map[fleet.Cap][]string) *Principal {
	return &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: proyecto,
		Fleet: grants,
	}
}

// sembrarLosTresPlanos da de alta una máquina y le deja un hecho de cada plano.
func sembrarLosTresPlanos(t *testing.T, s *McpServer, proyecto, nombre string) fleet.Device {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": nombre, "tier": "A", "project": proyecto,
		"caps": []string{"metrics", "exec", "screen", "shell"}, "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil || !existe {
		t.Fatalf("no quedó la máquina: %v %v", existe, err)
	}
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: proyecto, Principal: "gio",
		Argv: []string{"systemctl", "restart", "MARCASCRIPT"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
		DeviceID: d.ID, ProjectID: proyecto, Principal: "MARCAMIRON",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: proyecto, Principal: "MARCAPROMPT",
	}); err != nil {
		t.Fatal(err)
	}
	return d
}

// tiposDe devuelve los tipos de hecho que trajo una respuesta, y su texto crudo.
func tiposDe(t *testing.T, res interface{}) (map[string]int, string) {
	t.Helper()
	out := jsonOf(t, res)
	tipos := map[string]int{}
	for _, h := range out["hechos"].([]any) {
		tipos[h.(map[string]any)["tipo"].(string)]++
	}
	return tipos, textOf(t, res)
}

// La cronología cruza los TRES planos en una sola lista para quien tiene las tres capacidades.
//
// Sabotaje: leer sólo `device_commands` → faltan las dos sesiones.
func TestLaCronologiaCruzaLosTresPlanos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	p := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	tipos, crudo := tiposDe(t, res)
	for _, quiero := range []string{"comando", "sesion_pantalla", "sesion_shell"} {
		if tipos[quiero] == 0 {
			t.Errorf("falta el hecho de tipo %q en la cronología: %s", quiero, crudo)
		}
	}
	out := jsonOf(t, res)
	if out["ocultos_por_permiso"] != float64(0) {
		t.Errorf("con las tres capacidades no debería ocultarse nada: %v", out["ocultos_por_permiso"])
	}
	// Los huecos declarados viajan SIEMPRE, también cuando la lista trae cosas: una respuesta
	// llena engaña igual si no dice qué no miró.
	if hs, _ := out["no_visto"].([]any); len(hs) == 0 {
		t.Error("la respuesta no declaró los huecos")
	}
}

// LA COMPUERTA ES POR HECHO Y NO POR LA LISTA. Es la prueba central de este slice.
//
// Sabotaje: compuertar la lista entera con UNA capacidad. Con `exec` (la más laxa de las tres
// acá) alguien que sólo puede ejecutar vería quién tuvo un prompt y quién miró la pantalla; con
// `shell` (la más estricta) alguien que puede ejecutar no vería sus propios comandos. Las dos
// direcciones fallan acá.
func TestLaCompuertaEsPorHechoYNoPorLaLista(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	casos := []struct {
		nombre  string
		grants  map[fleet.Cap][]string
		ve      []string
		noVe    []string
		ocultos int
	}{
		{"sólo exec", map[fleet.Cap][]string{fleet.CapExec: {"*"}},
			[]string{"MARCASCRIPT"}, []string{"MARCAMIRON", "MARCAPROMPT"}, 2},
		{"sólo shell", map[fleet.Cap][]string{fleet.CapShell: {"*"}},
			[]string{"MARCAPROMPT"}, []string{"MARCASCRIPT", "MARCAMIRON"}, 2},
		{"sólo mirar pantalla", map[fleet.Cap][]string{fleet.CapScreenView: {"*"}},
			[]string{"MARCAMIRON"}, []string{"MARCASCRIPT", "MARCAPROMPT"}, 2},
	}
	for _, c := range casos {
		res, e := callAsPrincipal(t, s, conCaps("infra", c.grants), "musubi_fleet_cronologia",
			map[string]any{"device": "pc-gio"})
		if e != nil {
			t.Fatalf("%s: %+v", c.nombre, e)
		}
		crudo := textOf(t, res)
		for _, marca := range c.ve {
			if !strings.Contains(crudo, marca) {
				t.Errorf("%s: NO vio %q y debería: %s", c.nombre, marca, crudo)
			}
		}
		for _, marca := range c.noVe {
			if strings.Contains(crudo, marca) {
				t.Errorf("FUGA · %s: vio %q sin tener la capacidad de ese plano: %s", c.nombre, marca, crudo)
			}
		}
		if n := jsonOf(t, res)["ocultos_por_permiso"]; n != float64(c.ocultos) {
			t.Errorf("%s: ocultos_por_permiso = %v, esperaba %d — el contador es lo que le dice a alguien que hay más y le falta permiso", c.nombre, n, c.ocultos)
		}
	}
}

// Una fila de `device_commands` cuyo argv es una operación de pantalla se presenta como PANTALLA y
// pide `screen:view`, no `exec`. El tipo se decide por lo que el hecho REVELA, no por la tabla.
//
// Sabotaje: clasificar por tabla de origen → quien tiene sólo `exec` se entera de que alguien
// miró la pantalla de esa máquina, que es información del otro plano.
func TestUnaOperacionDePantallaNoSeLeMuestraAQuienSoloPuedeEjecutar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	const secreto = "ContraseñaAcuñada42"
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "MARCACANAL",
		Argv: []string{fleet.OpPantalla, "ses-7", secreto, "30m0s"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}

	soloExec := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	res, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	if crudo := textOf(t, res); strings.Contains(crudo, "MARCACANAL") || strings.Contains(crudo, "ses-7") {
		t.Errorf("FUGA: con sólo `exec` se vio la operación de pantalla: %s", crudo)
	}

	// Con `screen:view` sí se ve, clasificada en el plano de ENTRAR — y SIN la contraseña.
	// Es la TERCERA superficie que muestra un argv, y el saneo tiene que estar en las tres.
	soloVer := conCaps("infra", map[fleet.Cap][]string{fleet.CapScreenView: {"*"}})
	res2, e2 := callAsPrincipal(t, s, soloVer, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e2 != nil {
		t.Fatalf("cronologia: %+v", e2)
	}
	tipos, crudo := tiposDe(t, res2)
	if tipos["canal_pantalla"] == 0 {
		t.Errorf("la operación de pantalla no apareció como `canal_pantalla`: %s", crudo)
	}
	if strings.Contains(crudo, secreto) {
		t.Fatalf("LA CONTRASEÑA LLEGÓ A LA CRONOLOGÍA: %s", crudo)
	}
	for _, h := range jsonOf(t, res2)["hechos"].([]any) {
		fila := h.(map[string]any)
		if fila["tipo"] == "canal_pantalla" && fila["plano"] != "entrar" {
			t.Errorf("la operación de pantalla quedó en el plano %v, esperaba `entrar`", fila["plano"])
		}
	}
}

// La ventana se aplica EN LA CONSULTA. Sin eso, un tope alcanzado devuelve vacío para una ventana
// vieja y ese vacío se lee como «no pasó nada».
//
// Sabotaje: traer las últimas N filas y filtrar por fecha en Go → con `limite: 3` y cinco hechos
// nuevos encima, el hecho viejo NUNCA aparece.
func TestLaVentanaSeAplicaEnLaConsultaYNoDespues(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	viejo := time.Now().UTC().Add(-72 * time.Hour)
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creado: viejo,
		Argv: []string{"echo", "MARCAVIEJA"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	// Ruido reciente, más que el tope, para que un filtro en Go no llegue nunca al viejo.
	for i := 0; i < 5; i++ {
		if _, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "infra", Principal: "gio",
			Argv: []string{"echo", "ruido"}, Timeout: 30 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})

	// La ventana default (24 h) NO lo trae: el hecho es de hace tres días.
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	if strings.Contains(textOf(t, res), "MARCAVIEJA") {
		t.Error("un hecho de hace 72 h apareció en la ventana default de 24 h")
	}

	// Pedida su ventana, aparece — aunque el tope sea chiquito y haya ruido más nuevo.
	res2, e2 := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{
		"device": "pc-gio",
		"desde":  viejo.Add(-time.Hour).Format(time.RFC3339),
		"hasta":  viejo.Add(time.Hour).Format(time.RFC3339),
		"limite": 3,
	})
	if e2 != nil {
		t.Fatalf("cronologia con ventana: %+v", e2)
	}
	if !strings.Contains(textOf(t, res2), "MARCAVIEJA") {
		t.Fatalf("el hecho viejo no apareció con su ventana pedida: %s", textOf(t, res2))
	}
	if strings.Contains(textOf(t, res2), "ruido") {
		t.Error("apareció ruido reciente en una ventana de hace tres días")
	}
}

// El tope que corta ADENTRO de la ventana se DECLARA. Un corte silencioso se lee como «esto es
// todo lo que pasó».
//
// Sabotaje: devolver `truncado: false` siempre → falla acá.
func TestElTopeQueCortaSeDeclara(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	for i := 0; i < 6; i++ {
		if _, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "infra", Principal: "gio",
			Argv: []string{"echo", "x"}, Timeout: 30 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "limite": 3})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	out := jsonOf(t, res)
	if out["truncado"] != true {
		t.Errorf("el tope cortó y no se declaró: %v", textOf(t, res))
	}
	if n := len(out["hechos"].([]any)); n > 3 {
		t.Errorf("devolvió %d hechos con limite=3", n)
	}
	// Sin tope apretado, no se declara truncado: un `true` permanente no informa nada.
	res2, _ := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if jsonOf(t, res2)["truncado"] != false {
		t.Error("`truncado` quedó en true sin que el tope cortara nada")
	}
}

// Una máquina REVOCADA conserva su cronología para quien la podía ver: la revocación es el
// kill-switch para OPERAR, no para auditar (A51).
//
// Sabotaje: usar PuedeSobreDevice en vez de PuedeVerHistorialDeDevice → la cronología de la
// máquina que se dio de baja después de un incidente se vuelve ilegible justo cuando se necesita.
func TestLaCronologiaDeUnaMaquinaRevocadaSeSigueLeyendo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if ok, err := s.engine.RevocarDevice("infra", "pc-gio"); err != nil || !ok {
		t.Fatalf("revocar: %v %v", ok, err)
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("cronologia de una máquina revocada: %+v", e)
	}
	if !strings.Contains(textOf(t, res), "MARCASCRIPT") {
		t.Fatalf("se perdió la cronología de una máquina revocada: %s", textOf(t, res))
	}
}

// Sin NINGÚN plano visible se EXPLICA, no se devuelve una lista vacía. Una lista vacía se lee como
// «no pasó nada» justo cuando lo que pasa es que no tenés permiso para verlo.
//
// Sabotaje: sacar la guarda algunPlanoVisible → quien sólo tiene `metrics` recibe una cronología
// vacía de una máquina llena de actividad y concluye lo contrario de lo que pasó.
func TestSinNingunPlanoVisibleSeExplicaEnVezDeDevolverVacio(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	soloMetrics := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})
	res, e := callAsPrincipal(t, s, soloMetrics, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e == nil {
		t.Fatalf("con sólo `metrics` esperaba una explicación, obtuve: %s", textOf(t, res))
	}
	if e.Code != codeUnauthorized {
		t.Errorf("código = %d, esperaba unauthorized", e.Code)
	}
	// El mensaje tiene que decir QUÉ falta, no sólo que falta algo.
	for _, quiero := range []string{"exec", "screen:view", "shell"} {
		if !strings.Contains(e.Message, quiero) {
			t.Errorf("el mensaje no nombra la capacidad %q: %s", quiero, e.Message)
		}
	}
}

// Una máquina de otro tenant no existe para esta credencial, y el mensaje no confirma que exista.
func TestLaCronologiaNoEsUnOraculoDeMaquinasAjenas(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	ajeno := conCaps("otro", map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapShell: {"*"}})
	_, e := callAsPrincipal(t, s, ajeno, "musubi_fleet_cronologia", map[string]any{
		"device": "pc-gio", "project": "infra", // DECLARA el proyecto ajeno: caso hostil
	})
	if e == nil {
		t.Fatal("una credencial de otro proyecto obtuvo la cronología de una máquina ajena")
	}
	if strings.Contains(e.Message, "MARCASCRIPT") {
		t.Errorf("el mensaje de error filtró dato del tenant ajeno: %s", e.Message)
	}
}

// `horas` junto con `desde`/`hasta` es un error, no algo que se resuelva eligiendo uno.
//
// Sabotaje: que `horas` gane en silencio → quien mandó las dos cosas recibe una ventana que no
// pidió y la respuesta se ve correcta.
func TestHorasConDesdeOHastaEsUnError(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})

	for _, args := range []map[string]any{
		{"device": "pc-gio", "horas": 3, "desde": "2026-08-25T00:00:00Z"},
		{"device": "pc-gio", "horas": 3, "hasta": "2026-08-25T00:00:00Z"},
	} {
		if _, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", args); e == nil {
			t.Errorf("se aceptaron dos formas de ventana a la vez: %v", args)
		}
	}
	// Una fecha que no es RFC3339 se rechaza con un mensaje que dice el formato.
	_, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "desde": "el martes"})
	if e == nil || !strings.Contains(e.Message, "RFC3339") {
		t.Errorf("una fecha inválida tiene que explicar el formato: %+v", e)
	}
}

// La ventana que VUELVE es la que se APLICÓ. Contestar con la pedida mientras se aplica otra hace
// irreproducible una investigación.
//
// Sabotaje: devolver los argumentos crudos en vez de la ventana normalizada → alguien copia el
// `desde` de la respuesta, lo vuelve a pedir y le vuelven hechos distintos.
func TestLaVentanaQueVuelveEsLaQueSeAplico(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})

	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "horas": 6})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	v := jsonOf(t, res)["ventana"].(map[string]any)
	desde, err1 := time.Parse(time.RFC3339, v["desde"].(string))
	hasta, err2 := time.Parse(time.RFC3339, v["hasta"].(string))
	if err1 != nil || err2 != nil {
		t.Fatalf("la ventana devuelta no es RFC3339: %v %v %v", v, err1, err2)
	}
	// Reproducible: pedirla otra vez con las puntas devueltas da la MISMA ventana.
	res2, e2 := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{
		"device": "pc-gio", "desde": v["desde"], "hasta": v["hasta"],
	})
	if e2 != nil {
		t.Fatalf("cronologia repetida: %+v", e2)
	}
	v2 := jsonOf(t, res2)["ventana"].(map[string]any)
	if v2["desde"] != v["desde"] || v2["hasta"] != v["hasta"] {
		t.Errorf("la ventana no es reproducible: %v vs %v", v, v2)
	}
	if d := hasta.Sub(desde); d < 6*time.Hour || d > 6*time.Hour+2*time.Second {
		t.Errorf("`horas: 6` dio una ventana de %s", d)
	}
}

// Lo que pasó RECIÉN entra en la ventana que termina ahora. Es el bug que encontró el barrido de
// aislamiento: con las fechas guardadas al segundo y el borde superior abierto, un comando
// encolado en este mismo segundo quedaba afuera.
//
// Sabotaje: sacar Ventana.Normalizada del camino → esta prueba falla, y con ella la experiencia
// más común de todas: reiniciar algo y entrar a mirar qué pasó.
func TestLoQueAcabaDePasarEntraEnLaVentana(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "gio",
		Argv: []string{"echo", "MARCARECIEN"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "horas": 1})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	if !strings.Contains(textOf(t, res), "MARCARECIEN") {
		t.Fatalf("lo que acaba de pasar no entró en la ventana que termina ahora: %s", textOf(t, res))
	}
}

// Una operación interna que este cerebro NO conoce no se le muestra a NADIE —ni al que tiene todas
// las capacidades— y se cuenta EN SU PROPIO contador.
//
// Sabotaje: sumarla a `ocultos_por_permiso` → el mensaje sería «pedile permiso a alguien» sobre
// algo que ningún permiso destraba.
func TestUnaOperacionInternaNuevaNoSeMuestraYSeCuentaAparte(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "MARCAFUTURA",
		Argv: []string{"musubi:todavia-no-existe", "algo"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	todo := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	res, e := callAsPrincipal(t, s, todo, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	out := jsonOf(t, res)
	if strings.Contains(textOf(t, res), "MARCAFUTURA") {
		t.Errorf("una operación interna sin clasificar se mostró: %s", textOf(t, res))
	}
	if out["sin_clasificar"] != float64(1) {
		t.Errorf("sin_clasificar = %v, esperaba 1", out["sin_clasificar"])
	}
	if out["ocultos_por_permiso"] != float64(0) {
		t.Errorf("ocultos_por_permiso = %v: lo sin clasificar NO es un problema de permiso", out["ocultos_por_permiso"])
	}
}

// GUARD DE EXHAUSTIVIDAD SOBRE EL CÓDIGO FUENTE: toda operación `musubi:*` que el cerebro o el
// agente nombren tiene que estar clasificada por el dominio.
//
// Sin esto, el fail-closed de la cronología es correcto Y silencioso: alguien agrega una operación
// interna nueva, la cronología la esconde de todo el mundo, y la única señal es un contador que
// nadie mira. Esta prueba convierte ese silencio en una falla de compilación de la suite.
//
// Se lee el FUENTE y no una lista declarada a propósito: una lista declarada es exactamente lo que
// alguien se olvida de actualizar — que es el problema que esto viene a resolver.
func TestTodaOperacionInternaDelCodigoEstaClasificada(t *testing.T) {
	// Sólo el literal completo entre comillas, para no confundirse con el prefijo suelto ni con
	// los ejemplos en prosa de los comentarios.
	rx := regexp.MustCompile(`"musubi:[a-z0-9:_-]+"`)
	vistas := map[string]string{}
	for _, dir := range []string{"../mcp", "../fleet", "../../cmd/musubi"} {
		entradas, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", dir, err)
		}
		for _, e := range entradas {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			// El fuente de esta prueba y el de la del dominio nombran a propósito una operación
			// inexistente para verificar el fail-closed.
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range rx.FindAllString(string(b), -1) {
				vistas[strings.Trim(m, `"`)] = filepath.Join(dir, e.Name())
			}
		}
	}
	if len(vistas) == 0 {
		t.Fatal("no se encontró ninguna operación `musubi:*` en el fuente: el barrido se rompió y quedaría verde por vacío")
	}
	for op, donde := range vistas {
		if op == fleet.PrefijoOperacionInterna {
			continue
		}
		tipo := fleet.TipoDeArgv([]string{op})
		if tipo == fleet.HechoSinClasificar {
			t.Errorf("la operación interna %q (en %s) NO está clasificada en fleet.TipoDeArgv: la cronología la escondería de todos sin decir por qué", op, donde)
		}
	}
}

// LAS DOS SUPERFICIES CUENTAN LA MISMA HISTORIA sobre la misma tabla.
//
// Un comando que nadie levantó y que pasó su vida máxima se muestra `expirado` en la bitácora Y
// en la cronología. `expirado` sólo se ESTAMPA cuando el agente viene a pedir su cola, así que
// una máquina cuyo agente no vuelve deja sus comandos en `pendiente` para siempre — y las dos
// vistas los dibujaban así. Medido en producción: 50 comandos de 10 horas, vida máxima 15 min.
//
// Es la lección de A39 aplicada al eje del tiempo: una guarda sobre UNA superficie deja la otra
// mintiendo, y la que miente es siempre la que menos se mira.
//
// Sabotaje: devolver `string(c.Estado)` en cualquiera de las dos → falla acá, en esa mitad.
func TestLasDosSuperficiesMuestranVencidoUnComandoQueNadieLevanto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	// Un comando encolado hace diez horas que nadie levantó nunca: exactamente el caso real.
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "davantis",
		Creado: time.Now().UTC().Add(-10 * time.Hour),
		Argv:   []string{"cmd", "/c", "MARCAVENCIDO"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})

	estadoDe := func(tool string, args map[string]any, filas string, marca string) string {
		t.Helper()
		res, e := callAsPrincipal(t, s, p, tool, args)
		if e != nil {
			t.Fatalf("%s: %+v", tool, e)
		}
		for _, f := range jsonOf(t, res)[filas].([]any) {
			fila := f.(map[string]any)
			argv, _ := fila["argv"].([]any)
			for _, a := range argv {
				if a == marca {
					return fila["estado"].(string)
				}
			}
		}
		t.Fatalf("%s: no apareció el comando marcado: %s", tool, textOf(t, res))
		return ""
	}

	enBitacora := estadoDe("musubi_fleet_log", map[string]any{"limite": 50}, "comandos", "MARCAVENCIDO")
	enCronologia := estadoDe("musubi_fleet_cronologia",
		map[string]any{"device": "pc-gio", "horas": 24, "limite": 100}, "hechos", "MARCAVENCIDO")

	if enBitacora != string(fleet.EstadoExpirado) {
		t.Errorf("la BITÁCORA muestra %q para un comando de 10 h que nadie levantó, esperaba %q",
			enBitacora, fleet.EstadoExpirado)
	}
	if enCronologia != string(fleet.EstadoExpirado) {
		t.Errorf("la CRONOLOGÍA muestra %q para un comando de 10 h que nadie levantó, esperaba %q",
			enCronologia, fleet.EstadoExpirado)
	}
	if enBitacora != enCronologia {
		t.Errorf("las dos superficies discrepan sobre la MISMA fila: bitácora=%q cronología=%q",
			enBitacora, enCronologia)
	}

	// CONTROL: uno recién encolado sigue `pendiente` en las dos. Sin esto, marcar todo como
	// expirado pasaría las tres aserciones de arriba.
	if got := estadoDe("musubi_fleet_log", map[string]any{"limite": 50}, "comandos", "MARCASCRIPT"); got != string(fleet.EstadoPendiente) {
		t.Errorf("un comando recién encolado se muestra %q en la bitácora", got)
	}
	if got := estadoDe("musubi_fleet_cronologia",
		map[string]any{"device": "pc-gio", "horas": 24, "limite": 100}, "hechos", "MARCASCRIPT"); got != string(fleet.EstadoPendiente) {
		t.Errorf("un comando recién encolado se muestra %q en la cronología", got)
	}
}
