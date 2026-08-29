package mcp

// Pruebas del slice S3: el TERCER EJE. Qué puede pedirle una persona a qué máquina.

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// devicePrueba arma un Tier A con todas las capacidades concedidas, en el proyecto dado.
func devicePrueba(nombre, proyecto string) fleet.Device {
	return fleet.Device{
		ID: "id-" + nombre, Name: nombre, ProjectID: proyecto, Tier: fleet.TierAgente,
		Caps: []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen},
	}
}

// C1 — LA VALLA DEL TRACK. El rol de memoria no otorga NADA sobre las máquinas.
//
// Sabotaje que la hace fallar: agregar `if p.isAdmin() { return true }` a PuedeSobreDevice —
// exactamente la "simplificación" que alguien va a proponer.
func TestElRolDeMemoriaNoOtorgaCapacidadesDeFlota(t *testing.T) {
	d := devicePrueba("pc-gio", "casa")

	// El principal MÁS poderoso que sabe expresar el modelo de memoria: admin, ve todo, escribe
	// en cualquier tenant. Y sin sección `fleet:`.
	todopoderoso := &Principal{
		Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
	}
	for _, c := range []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen} {
		if PuedeSobreDevice(todopoderoso, d, c) {
			t.Errorf("un admin write=any SIN grants de flota pudo %q: administrar la memoria se volvió root de la flota", c)
		}
	}
	if len(capsQuePuede(todopoderoso, d)) != 0 {
		t.Error("capsQuePuede devolvió capacidades para un principal sin grants")
	}
}

// C2 — la concesión es POR CAPACIDAD: tener metrics no da exec.
// Sabotaje: que tieneGrant ignore la capacidad y mire cualquier entrada del mapa.
func TestLaConcesionEsPorCapacidadYNoUnBooleano(t *testing.T) {
	d := devicePrueba("servidor", "casa")
	observador := &Principal{
		Name: "ojos", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	if !PuedeSobreDevice(observador, d, fleet.CapMetrics) {
		t.Error("no pudo metrics teniéndola concedida con comodín")
	}
	if PuedeSobreDevice(observador, d, fleet.CapExec) {
		t.Error("mirar las métricas de un servidor le dio poder ESCRIBIR en él")
	}
	if PuedeSobreDevice(observador, d, fleet.CapScreen) {
		t.Error("mirar las métricas le dio la pantalla")
	}
}

// C3 — la concesión es POR MÁQUINA.
// Sabotaje: que tieneGrant devuelva true si la lista no está vacía, sin mirar el nombre.
func TestLaConcesionEsPorMaquina(t *testing.T) {
	p := &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"pc-gio", "nas"}},
	}
	if !PuedeSobreDevice(p, devicePrueba("pc-gio", "casa"), fleet.CapExec) {
		t.Error("no pudo exec sobre una máquina que tiene nombrada")
	}
	if !PuedeSobreDevice(p, devicePrueba("nas", "casa"), fleet.CapExec) {
		t.Error("no pudo exec sobre la segunda máquina nombrada")
	}
	if PuedeSobreDevice(p, devicePrueba("servidor-critico", "casa"), fleet.CapExec) {
		t.Error("pudo exec sobre una máquina que NO tiene nombrada")
	}
}

// C4 — la tenencia sigue mandando: el grant no es puerta lateral al aislamiento.
// Sabotaje: quitar la llamada a alcanzaElProyecto de PuedeSobreDevice.
func TestElGrantNoEsUnaPuertaLateralALaTenencia(t *testing.T) {
	// Nombra una máquina del tenant ajeno, e incluso con comodín.
	acotado := &Principal{
		Name: "dev", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*", "server-acme"}},
	}
	ajena := devicePrueba("server-acme", "cliente-acme")
	if PuedeSobreDevice(acotado, ajena, fleet.CapExec) {
		t.Error("un grant alcanzó una máquina de OTRO proyecto: el eje nuevo sorteó la tenencia")
	}
	// Sobre lo suyo, el mismo comodín sí alcanza.
	if !PuedeSobreDevice(acotado, devicePrueba("pc-gio", "casa"), fleet.CapExec) {
		t.Error("el comodín no alcanzó una máquina de su propio proyecto")
	}
	// La sala de mando (read=all) SÍ cruza, porque su alcance lo dice.
	mando := &Principal{
		Name: "mando", Role: RoleWriter, Read: ReadAll, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	if !PuedeSobreDevice(mando, ajena, fleet.CapExec) {
		t.Error("un read=all con comodín debería alcanzar la máquina de otro proyecto")
	}
}

// C5 — el aparato también tiene que poder: un grant no le da pantalla a un router.
// Sabotaje: quitar la llamada a d.Permite de PuedeSobreDevice.
func TestUnGrantNoLeDaPantallaAUnRouter(t *testing.T) {
	todopoderoso := &Principal{
		Name: "root", Role: RoleAdmin, Read: ReadAll, ProjectID: "infra",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}, fleet.CapExec: {"*"}},
	}
	router := fleet.Device{
		ID: "id-r", Name: "switch", ProjectID: "infra", Tier: fleet.TierProtocolo,
		Caps: []fleet.Cap{fleet.CapMetrics, fleet.CapExec},
	}
	if PuedeSobreDevice(todopoderoso, router, fleet.CapScreen) {
		t.Error("un grant de screen le dio framebuffer a un Tier B")
	}
	if !PuedeSobreDevice(todopoderoso, router, fleet.CapExec) {
		t.Error("exec sobre un Tier B debería poder: SSH ejecuta")
	}
	// Y una capacidad que el device NO tiene concedida tampoco, aunque el tier la admita.
	sinExec := fleet.Device{
		ID: "id-x", Name: "solo-mirar", ProjectID: "infra", Tier: fleet.TierAgente,
		Caps: []fleet.Cap{fleet.CapMetrics},
	}
	if PuedeSobreDevice(todopoderoso, sinExec, fleet.CapExec) {
		t.Error("pudo exec sobre una máquina a la que no se le concedió exec")
	}
}

// C6 — revocar la máquina gana sobre cualquier concesión.
// Sabotaje: quitar la guarda de Revoked de fleet.Device.Permite (ya cubierto en S1, pero el
// kill-switch tiene que valer también DESDE ACÁ: es la ruta por la que va a pasar exec).
func TestRevocarLaMaquinaGanaSobreElComodin(t *testing.T) {
	root := &Principal{
		Name: "root", Role: RoleAdmin, Read: ReadAll, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{
			fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}, fleet.CapScreen: {"*"},
		},
	}
	d := devicePrueba("pc-gio", "casa")
	d.Revoked = true
	for _, c := range []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen} {
		if PuedeSobreDevice(root, d, c) {
			t.Errorf("el comodín sorteó el kill-switch para %q", c)
		}
	}
}

// C9 — stdio local conserva acceso pleno. Es la vía de arranque, y se prueba EXPLÍCITAMENTE
// para que sea una decisión visible y no un descuido heredado.
func TestStdioLocalConservaAccesoPleno(t *testing.T) {
	d := devicePrueba("pc-gio", "casa")
	for _, c := range []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen} {
		if !PuedeSobreDevice(nil, d, c) {
			t.Errorf("stdio local no pudo %q: se rompió la confianza local y no hay forma de otorgar la primera capacidad", c)
		}
		if !puedeOtorgar(nil, c) {
			t.Errorf("stdio local no pudo OTORGAR %q", c)
		}
	}
	// Pero ni siquiera el local sortea al aparato: un device revocado sigue sin admitir nada.
	d.Revoked = true
	if PuedeSobreDevice(nil, d, fleet.CapExec) {
		t.Error("stdio local sorteó el kill-switch")
	}
}

// capsQuePuede devuelve la intersección, en orden canónico.
//
// LAS IMPLICADAS TAMBIÉN SALEN, y es lo correcto aunque sorprenda. `puedo` no dice «qué te
// concedieron» —eso es `caps`— dice QUÉ PODÉS EJERCER sobre esta máquina ahora mismo (C8). Quien
// tiene `screen` puede mirar, así que esconderle `screen:view` sería mentirle por omisión a un
// panel que necesita saber qué botones habilitar: el de mirar y el de controlar son dos.
//
// El orden canónico lleva `screen:view` ANTES de `screen`, y eso encoded la escalada: la lista se
// lee de menos a más poder, así que un vistazo al final de la fila dice hasta dónde llega esta
// credencial.
//
// Sabotaje que la hace fallar: sacar CapScreenView de la lista de `todas` en capsQuePuede, o
// ponerlo después de CapScreen.
func TestCapsQuePuedeEsLaInterseccionEnOrden(t *testing.T) {
	d := devicePrueba("pc-gio", "casa")
	p := &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}, fleet.CapMetrics: {"pc-gio"}},
	}
	got := capsQuePuede(p, d)
	quiero := []fleet.Cap{fleet.CapMetrics, fleet.CapScreenView, fleet.CapScreen}
	if len(got) != len(quiero) {
		t.Fatalf("esperaba %v, obtuve %v", quiero, got)
	}
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("esperaba %v en orden canónico (de menos a más poder), obtuve %v", quiero, got)
		}
	}

	// Y quien tiene SÓLO `screen:view` no ve `screen` en su `puedo`: la implicación no se
	// devuelve al revés, o el panel habilitaría el botón de controlar a alguien que no puede.
	soloMira := &Principal{
		Name: "ojos", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapScreenView: {"*"}},
	}
	for _, c := range capsQuePuede(soloMira, d) {
		if c == fleet.CapScreen {
			t.Error("quien sólo puede mirar figura pudiendo controlar: el panel habilitaría el botón equivocado")
		}
	}
}

// ── C7 · No se puede otorgar lo que no se tiene ──────────────────────────────────────────────

// C7 — el escalamiento que cierra: alguien con exec sobre dos máquinas NOMBRADAS no puede
// mintear una tercera con exec.
// Sabotaje: que puedeOtorgar acepte cualquier selector (no sólo el comodín).
func TestNoSePuedeOtorgarLoQueNoSeTiene(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// Admin de la memoria, con exec sobre DOS máquinas nombradas. No tiene el comodín.
	acotado := &Principal{
		Name: "op", Role: RoleAdmin, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"pc-gio", "nas"}},
	}
	_, e := callAsPrincipal(t, s, acotado, "musubi_fleet_enroll", map[string]any{
		"name": "tercera", "tier": "A", "caps": []string{"exec"},
	})
	if e == nil {
		t.Fatal("pudo dar de alta una máquina nueva con exec sin tener el comodín: se amplió el alcance solo")
	}
	if e.Code != codeUnauthorized {
		t.Errorf("esperaba unauthorized, obtuve code %d", e.Code)
	}
	if !strings.Contains(e.Message, "*") {
		t.Errorf("el error no dice qué haría falta: %q", e.Message)
	}

	// Con el comodín, sí puede.
	conComodin := &Principal{
		Name: "jefe", Role: RoleAdmin, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
	}
	if _, e := callAsPrincipal(t, s, conComodin, "musubi_fleet_enroll", map[string]any{
		"name": "cuarta", "tier": "A", "caps": []string{"metrics", "exec"},
	}); e != nil {
		t.Fatalf("con el comodín debería poder otorgar: %+v", e)
	}
	// Pero NO una capacidad que tampoco tiene con comodín.
	if _, e := callAsPrincipal(t, s, conComodin, "musubi_fleet_enroll", map[string]any{
		"name": "quinta", "tier": "A", "caps": []string{"screen"},
	}); e == nil {
		t.Error("otorgó `screen` sin tenerla concedida")
	}
}

// C7 — un admin SIN grants de flota puede administrar el registro pero no conceder capacidades.
// Es C1 visto desde la tool: la separación entre «administrar la flota» y «poder sobre las
// máquinas» tiene que sobrevivir al camino real, no sólo a la función pura.
func TestUnAdminSinGrantsEnrolaPeroNoConcede(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	admin := &Principal{Name: "root", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa"}

	// Dar de alta una máquina SIN capacidades: puede. Administrar el inventario es su trabajo.
	if _, e := callAsPrincipal(t, s, admin, "musubi_fleet_enroll", map[string]any{
		"name": "inventariada", "tier": "A", "project": "casa",
	}); e != nil {
		t.Fatalf("un admin debería poder registrar una máquina sin capacidades: %+v", e)
	}
	// Concederle exec: no puede.
	if _, e := callAsPrincipal(t, s, admin, "musubi_fleet_enroll", map[string]any{
		"name": "con-poder", "tier": "A", "caps": []string{"exec"}, "project": "casa",
	}); e == nil {
		t.Error("un admin sin grants de flota concedió exec: el puente de privilegio quedó abierto")
	}
}

// C8 — el inventario dice qué puede ejercer QUIEN MIRA, no sólo qué admite la máquina.
// Sabotaje: devolver `caps` también en `puedo`.
func TestElInventarioDiceQuePuedeQuienMira(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Alta como stdio local (confianza local): la máquina queda con las tres capacidades.
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec", "screen"}, "project": "casa",
	}); e != nil {
		t.Fatal(e)
	}

	// Un observador con SÓLO metrics.
	observador := &Principal{
		Name: "ojos", Role: RoleReader, Read: ReadOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	res, e := callAsPrincipal(t, s, observador, "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	devs, _ := jsonOf(t, res)["devices"].([]any)
	fila, _ := devs[0].(map[string]any)

	caps := aLista(fila["caps"])
	puedo := aLista(fila["puedo"])
	if len(caps) != 3 {
		t.Fatalf("la máquina debería admitir 3 capacidades, dice %v", caps)
	}
	if len(puedo) != 1 || puedo[0] != "metrics" {
		t.Fatalf("`puedo` debería ser sólo [metrics] para este observador, es %v", puedo)
	}
}

func aLista(v any) []string {
	crudo, _ := v.([]any)
	out := make([]string, 0, len(crudo))
	for _, x := range crudo {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// ── El parseo del YAML: fail-closed en los dos sentidos ──────────────────────────────────────

func TestElYamlDeGrantsEsFailClosed(t *testing.T) {
	tok := "un-token"
	base := "principals:\n  - name: gio\n    token_sha256: \"" + hashToken(tok) + "\"\n    project_id: casa\n    role: admin\n"

	t.Run("sin sección fleet ⇒ ninguna capacidad", func(t *testing.T) {
		reg, err := loadPrincipals(writeRegistry(t, base), "")
		if err != nil {
			t.Fatal(err)
		}
		p, _ := reg.resolve(tok)
		if len(p.Fleet) != 0 {
			t.Fatalf("esperaba ningún grant, obtuve %v", p.Fleet)
		}
		if PuedeSobreDevice(p, devicePrueba("pc-gio", "casa"), fleet.CapExec) {
			t.Error("un principal sin sección fleet pudo exec")
		}
	})

	t.Run("grants declarados se leen", func(t *testing.T) {
		body := base + "    fleet:\n      metrics: [\"*\"]\n      exec: [\"pc-gio\"]\n"
		reg, err := loadPrincipals(writeRegistry(t, body), "")
		if err != nil {
			t.Fatal(err)
		}
		p, _ := reg.resolve(tok)
		if !PuedeSobreDevice(p, devicePrueba("pc-gio", "casa"), fleet.CapExec) {
			t.Error("no pudo exec sobre la máquina nombrada")
		}
		if PuedeSobreDevice(p, devicePrueba("otra", "casa"), fleet.CapExec) {
			t.Error("pudo exec sobre una máquina no nombrada")
		}
		if !PuedeSobreDevice(p, devicePrueba("otra", "casa"), fleet.CapMetrics) {
			t.Error("el comodín de metrics no alcanzó")
		}
	})

	// Una capacidad inventada es ERROR DE ARRANQUE, no un permiso que se descarta en silencio.
	// Sabotaje: que parsearFleet ignore las claves desconocidas → alguien cree que otorgó `root`.
	t.Run("capacidad desconocida ⇒ el servidor no arranca", func(t *testing.T) {
		body := base + "    fleet:\n      root: [\"*\"]\n"
		if _, err := loadPrincipals(writeRegistry(t, body), ""); err == nil {
			t.Fatal("un `fleet: {root: [*]}` se aceptó en silencio: quien lo escribió cree que otorgó algo")
		}
	})

	// Una capacidad con lista vacía se lee como intención a medio escribir. Adivinar no es
	// tarea del parser.
	t.Run("lista vacía ⇒ error", func(t *testing.T) {
		body := base + "    fleet:\n      exec: []\n"
		if _, err := loadPrincipals(writeRegistry(t, body), ""); err == nil {
			t.Fatal("`exec: []` se aceptó: es una intención a medio escribir")
		}
	})
}
