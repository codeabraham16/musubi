package mcp

// Pruebas de A13 (S6b): la procedencia del `rustdesk_id`.
//
// Ese id lo REPORTA la propia máquina en su latido, o sea que es entrada no confiable. Todo lo de
// acá abajo existe para que un id ambiguo no mande a nadie a la pantalla equivocada.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// conPantalla es un principal con `screen` sobre todo su proyecto.
func conPantallaTotal(proyecto string) *Principal {
	return &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: proyecto,
		Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}, fleet.CapMetrics: {"*"}},
	}
}

// enrolarConPantallaViva da de alta un Tier A con `screen`, lo pone a latir y le fija su id.
func enrolarConPantallaViva(t *testing.T, s *McpServer, proyecto, nombre, rid string) fleet.Device {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics", "screen"},
		"project": proyecto, "os": "linux"}); e != nil {
		t.Fatalf("enroll(%q): %+v", nombre, e)
	}
	d, _, _ := s.engine.DevicePorNombre(proyecto, nombre)
	if _, err := s.engine.LatirDevice(d.ID, time.Now(), ""); err != nil {
		t.Fatal(err)
	}
	if rid != "" {
		if err := s.engine.GuardarRustdeskID(d.ID, rid); err != nil {
			t.Fatal(err)
		}
	}
	d, _, _ = s.engine.DevicePorNombre(proyecto, nombre)
	return d
}

// A13 — SI DOS MÁQUINAS DICEN SER LA MISMA PANTALLA, NO SE ABRE NINGUNA.
//
// Conectarse sería una moneda al aire, y se llega ahí por dos caminos: alguien comprometió una
// máquina y declaró el id de otra para desviar la sesión (la colisión es la FIRMA de ese ataque),
// o —mucho más frecuente— dos máquinas clonadas de la misma imagen, porque RustDesk deriva su id
// de la máquina y los clones nacen iguales.
//
// SE NIEGA en vez de avisar: abrir igual entrega una contraseña de sesión y manda a alguien a una
// pantalla que puede no ser la que cree, que es justo el daño a evitar.
//
// Sabotaje que la hace fallar: quitar la consulta de colisión de toolFleetScreen.
func TestNoSeAbreLaPantallaDeUnIdQueReclamanDos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConPantallaViva(t, s, "casa", "pc-buena", "123456789")

	// CONTROL POSITIVO: con el id sin colisión, la pantalla SÍ se abre. Sin esto, la aserción de
	// abajo pasaría también con una tool que niega todo.
	if _, e := callAsPrincipal(t, s, conPantallaTotal("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-buena"}); e != nil {
		t.Fatalf("con un id limpio la pantalla debería abrirse: %+v", e)
	}

	// Aparece una segunda máquina declarando el MISMO id.
	enrolarConPantallaViva(t, s, "casa", "pc-clonada", "123456789")

	for _, maquina := range []string{"pc-buena", "pc-clonada"} {
		_, e := callAsPrincipal(t, s, conPantallaTotal("casa"), "musubi_fleet_screen", map[string]any{"device": maquina})
		if e == nil {
			t.Fatalf("se abrió la pantalla de %q con un id que reclaman dos máquinas: conectarse es una moneda al aire", maquina)
		}
		// El mensaje tiene que decir QUÉ pasa, CON QUIÉN y CÓMO se arregla.
		for _, quiero := range []string{"moneda al aire", "clonada", "regenerá"} {
			if !strings.Contains(e.Message, quiero) {
				t.Errorf("el mensaje de colisión no menciona %q:\n%s", quiero, e.Message)
			}
		}
	}
}

// LA COLISIÓN SE MIRA GLOBALMENTE, PERO NO SE NOMBRA LO AJENO.
//
// Acotar la consulta al proyecto de quien pregunta dejaría pasar el caso PEOR —dos tenants con el
// mismo id, donde un operador aterriza en la máquina de otra empresa—. Pero nombrar una máquina
// ajena rompería el aislamiento. Así que se cuenta y no se nombra.
//
// Sabotaje que la hace fallar: acotar QuienMasDiceSer al projectID, o devolver los nombres de
// otros proyectos.
func TestUnaColisionEntreTenantsSeDetectaSinNombrarLaMaquinaAjena(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConPantallaViva(t, s, "casa", "pc-gio", "999888777")
	enrolarConPantallaViva(t, s, "otra-empresa", "servidor-secreto", "999888777")

	_, e := callAsPrincipal(t, s, conPantallaTotal("casa"), "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e == nil {
		t.Fatal("una colisión ENTRE PROYECTOS no se detectó: es el caso peor, un operador aterrizando en la máquina de otra empresa")
	}
	if strings.Contains(e.Message, "servidor-secreto") || strings.Contains(e.Message, "otra-empresa") {
		t.Errorf("el mensaje NOMBRA una máquina de otro proyecto: el aislamiento vale también acá.\n%s", e.Message)
	}
	if !strings.Contains(e.Message, "fuera de tu alcance") {
		t.Errorf("no se dice que hay una colisión fuera del alcance:\n%s", e.Message)
	}
}

// Un id que CAMBIA queda escrito, con su valor anterior. No bloquea nada —reinstalar una máquina
// es normal— pero la otra explicación posible es que alguien esté mintiendo.
//
// Sabotaje que la hace fallar: hacer que GuardarRustdeskID sólo escriba el valor nuevo.
func TestUnIdDePantallaQueCambiaQuedaEscrito(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConPantallaViva(t, s, "casa", "pc-gio", "111111111")

	// El PRIMER reporte no es un cambio, es el estreno: no puede quedar marcado.
	if !d.RustdeskIDCambiado.IsZero() || d.RustdeskIDPrevio != "" {
		t.Fatalf("el primer id figuró como un cambio (previo=%q, cambiado=%v)", d.RustdeskIDPrevio, d.RustdeskIDCambiado)
	}

	// La máquina reporta otro id.
	if err := s.engine.GuardarRustdeskID(d.ID, "222222222"); err != nil {
		t.Fatal(err)
	}
	d2, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if d2.RustdeskID != "222222222" {
		t.Fatalf("no se guardó el id nuevo: %q", d2.RustdeskID)
	}
	if d2.RustdeskIDPrevio != "111111111" {
		t.Errorf("no se guardó el id anterior: %q. Un id que se mueve solo tiene dos explicaciones y las dos ameritan quedar escritas", d2.RustdeskIDPrevio)
	}
	if d2.RustdeskIDCambiado.IsZero() {
		t.Error("no se guardó CUÁNDO cambió")
	}

	// Y re-reportar el MISMO id no cuenta como otro cambio: si contara, cada latido pisaría el
	// «previo» con el valor actual y se perdería el dato que importa.
	antes := d2.RustdeskIDCambiado
	if err := s.engine.GuardarRustdeskID(d.ID, "222222222"); err != nil {
		t.Fatal(err)
	}
	d3, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if d3.RustdeskIDPrevio != "111111111" || !d3.RustdeskIDCambiado.Equal(antes) {
		t.Errorf("re-reportar el mismo id se contó como un cambio: previo=%q cambiado=%v", d3.RustdeskIDPrevio, d3.RustdeskIDCambiado)
	}
}

// El inventario lo dice ANTES de que alguien necesite la pantalla. Descubrir el problema en el
// momento en que hace falta mirar una máquina es descubrirlo tarde.
//
// Sabotaje que la hace fallar: no agregar rustdesk_id_ambiguo al inventario.
func TestElInventarioAvisaDeUnIdAmbiguo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConPantallaViva(t, s, "casa", "pc-gio", "555")
	enrolarConPantallaViva(t, s, "casa", "pc-clon", "555")

	res, e := callAsPrincipal(t, s, conPantallaTotal("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	for _, fila := range jsonOf(t, res)["devices"].([]any) {
		f := fila.(map[string]any)
		if f["rustdesk_id_ambiguo"] != true {
			t.Errorf("%v no figura con el id ambiguo: el problema se vería recién al intentar abrir la pantalla", f["name"])
		}
		otras, _ := f["rustdesk_id_tambien_en"].([]any)
		if len(otras) != 1 {
			t.Errorf("%v no dice con quién colisiona: %v", f["name"], f["rustdesk_id_tambien_en"])
		}
	}
}

// Y una máquina con un id limpio NO lleva la marca: un aviso que aparece siempre enseña a
// ignorarlo.
func TestUnaMaquinaConIdLimpioNoLlevaMarca(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConPantallaViva(t, s, "casa", "pc-gio", "777")

	res, e := callAsPrincipal(t, s, conPantallaTotal("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	f := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	if _, hay := f["rustdesk_id_ambiguo"]; hay {
		t.Error("una máquina con id limpio lleva la marca de ambigüedad")
	}
	if f["rustdesk_id"] != "777" {
		t.Errorf("no se muestra el id: %v", f["rustdesk_id"])
	}
}
