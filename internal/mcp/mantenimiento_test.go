package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func servidorConMaquina(t *testing.T) (*McpServer, fleet.Device) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("no se pudo enrolar: %+v", e)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	return s, d
}

// LA VENTANA DE MANTENIMIENTO FRENA EL AUTO-HEAL, QUE ES LO QUE UN `silence` NO PUEDE.
//
// Ola 1 del plan empresa, y es LA razón de que esto viva en el dominio. Un `amtool silence` calla
// el aviso y no toca las políticas: no leen alertas, leen la muestra y actúan solas. Sin esta
// guarda, un reinicio planificado de postgres dispara `servicio_caido`, el auto-heal lo levanta
// EN MITAD DEL MANTENIMIENTO, y el silence sólo garantiza que nadie se entere — la automatización
// actuando con el canal que lo contaría apagado.
//
// Sabotaje que la hace fallar: sacar el `if enMantenimiento[d.ID] { continue }` de
// aplicarPoliticas.
func TestLasPoliticasNoActuanSobreUnaMaquinaEnMantenimiento(t *testing.T) {
	s, d := servidorConMaquina(t)
	ahora := time.Now().UTC()

	// El conjunto que consulta el scheduler: vacío antes, con la máquina después.
	if set, err := s.engine.DevicesEnMantenimiento(ahora); err != nil || set[d.ID] {
		t.Fatalf("la máquina figuraba en mantenimiento sin ninguna ventana declarada (err=%v)", err)
	}
	m, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
		Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migración de postgres",
	})
	if err != nil {
		t.Fatalf("no se pudo abrir la ventana: %v", err)
	}
	set, err := s.engine.DevicesEnMantenimiento(ahora)
	if err != nil {
		t.Fatal(err)
	}
	if !set[d.ID] {
		t.Fatal("la ventana está activa y la máquina no figura en mantenimiento: el scheduler va a evaluar políticas sobre ella y el auto-heal actuaría en mitad del mantenimiento")
	}

	// Cancelar la retira EN EL ACTO, sin borrar la fila.
	if hubo, err := s.engine.CancelarMantenimiento(d.ID, d.ProjectID, m.ID); err != nil || !hubo {
		t.Fatalf("no se pudo cancelar (hubo=%v err=%v)", hubo, err)
	}
	if set, _ := s.engine.DevicesEnMantenimiento(ahora); set[d.ID] {
		t.Error("una ventana cancelada sigue frenando el auto-heal")
	}
	filas, err := s.engine.MantenimientosDeDevice(d.ID, 10)
	if err != nil || len(filas) != 1 {
		t.Fatalf("cancelar borró la fila (quedaron %d, err=%v): la cronología se construye sobre tablas que no se editan", len(filas), err)
	}
	if !filas[0].Cancelada {
		t.Error("la fila no quedó marcada como cancelada")
	}
}

// LOS BORDES DE LA VENTANA: `desde` INCLUSIVE, `hasta` EXCLUSIVO.
//
// Con los dos inclusive, dos ventanas consecutivas se solapan un instante — y el solapamiento de
// algo que silencia alertas es la clase de detalle que nadie mira hasta que importa.
//
// Sabotaje: cambiar `ahora.Before(m.Hasta)` por `!ahora.After(m.Hasta)` en Activa.
func TestLaVentanaEmpiezaInclusiveYTerminaExclusive(t *testing.T) {
	desde := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	m := fleet.Mantenimiento{Desde: desde, Hasta: desde.Add(time.Hour)}
	casos := []struct {
		cuando time.Time
		activa bool
		por    string
	}{
		{desde.Add(-time.Second), false, "un segundo antes"},
		{desde, true, "el instante de inicio (inclusive)"},
		{desde.Add(30 * time.Minute), true, "en el medio"},
		{desde.Add(time.Hour), false, "el instante de fin (exclusivo)"},
		{desde.Add(time.Hour + time.Second), false, "un segundo después"},
	}
	for _, c := range casos {
		if got := m.Activa(c.cuando); got != c.activa {
			t.Errorf("%s: Activa=%v, esperaba %v", c.por, got, c.activa)
		}
	}
	// Y una cancelada no cubre nada, esté donde esté el reloj.
	m.Cancelada = true
	if m.Activa(desde.Add(30 * time.Minute)) {
		t.Error("una ventana cancelada sigue activa")
	}
}

// EL TECHO DE 24 HORAS NO ES BUROCRACIA: ES EL ANTÍDOTO CONTRA LA VENTANA ETERNA.
//
// Una ventana silencia las alertas de la máquina Y frena su auto-heal. Sin techo, un `hasta` con
// un dedo de más —2027 en vez de 2026— deja una máquina ciega para siempre, con todo en verde. Es
// la misma forma de falla que una cola sin techo (A62) y se cierra igual: el dominio no la deja
// expresar.
//
// Sabotaje: sacar la comparación contra MantenimientoMax de ValidarMantenimiento.
func TestUnaVentanaMasLargaQueElTechoSeRechaza(t *testing.T) {
	base := fleet.Mantenimiento{DeviceID: "d1", Principal: "gio", Desde: time.Now()}

	ok := base
	ok.Hasta = ok.Desde.Add(fleet.MantenimientoMax)
	if err := fleet.ValidarMantenimiento(ok); err != nil {
		t.Errorf("una ventana de exactamente el techo se rechazó: %v", err)
	}

	larga := base
	larga.Hasta = larga.Desde.Add(fleet.MantenimientoMax + time.Minute)
	err := fleet.ValidarMantenimiento(larga)
	if err == nil {
		t.Fatal("una ventana más larga que el techo se aceptó: una máquina ciega para siempre con el panel en verde")
	}
	if !strings.Contains(err.Error(), "máximo") {
		t.Errorf("el error no dice cuál es el techo: %v", err)
	}

	// Y una que termina antes de empezar tampoco.
	alReves := base
	alReves.Hasta = alReves.Desde.Add(-time.Hour)
	if fleet.ValidarMantenimiento(alReves) == nil {
		t.Error("una ventana que termina antes de empezar se aceptó")
	}
	// Sin principal tampoco: una máquina que se calla sin dueño es lo que esto viene a evitar.
	sinDueno := base
	sinDueno.Hasta = sinDueno.Desde.Add(time.Hour)
	sinDueno.Principal = ""
	if fleet.ValidarMantenimiento(sinDueno) == nil {
		t.Error("una ventana sin principal se aceptó: no habría a quién preguntarle por qué esa máquina está callada")
	}
}

// LA TOOL PIDE `metrics` SOBRE ESA MÁQUINA, Y EL TECHO SE APLICA POR EL CAMINO REAL.
//
// Sabotaje: sacar el `PuedeSobreDevice` de toolFleetMaintenance, o dejar que `minutos` pase sin
// que el dominio lo valide.
func TestLaToolDeMantenimientoRespetaElTechoYDevuelveLaVentana(t *testing.T) {
	s, _ := servidorConMaquina(t)
	ctx := context.Background()

	// Más que el techo: rechazada, con el número dicho.
	_, e := s.toolFleetMaintenance(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","minutos":1500}`))
	if e == nil {
		t.Fatal("la tool aceptó una ventana de 25 horas: el techo del dominio no se está aplicando por este camino")
	}
	if !strings.Contains(e.Message, "máximo") {
		t.Errorf("el rechazo no dice el techo: %s", e.Message)
	}

	// Dentro del techo: se crea y devuelve el id, que es lo único con lo que se puede cancelar.
	res, e := s.toolFleetMaintenance(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","minutos":90,"motivo":"migración"}`))
	if e != nil {
		t.Fatalf("la tool rechazó una ventana válida: %s", e.Message)
	}
	m := jsonOf(t, res)
	if m["id"] == nil || m["id"] == "" {
		t.Fatal("la tool no devolvió el id: sin él no hay forma de cancelar la ventana antes de tiempo")
	}
	if m["hasta"] == nil {
		t.Error("la tool no dice hasta cuándo dura")
	}

	// Y sin `minutos` ni `cancelar` no se inventa una ventana por default.
	if _, e := s.toolFleetMaintenance(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`)); e == nil {
		t.Error("sin `minutos` la tool creó algo: una ventana sin largo no existe")
	}
}

// Y LA TOOL EXIGE `metrics` SOBRE ESA MÁQUINA, con el mismo mensaje que si no existiera.
//
// Este tramo lo escribió el sabotaje: sacarle el `PuedeSobreDevice` a la tool dejaba todo el resto
// de las pruebas en verde. Una ventana silencia alertas y frena el auto-heal, así que declararla
// sobre una máquina ajena es apagarle el monitoreo a otro — no es una operación de lectura.
//
// El mensaje NO distingue «no existe» de «no podés»: distinguirlos convertiría la tool en un
// oráculo de qué máquinas hay en un proyecto que no ves, igual que en exec y en shell.
//
// Sabotaje que la hace fallar: cambiar `if !existe || !PuedeSobreDevice(...)` por `if !existe`.
func TestLaToolDeMantenimientoExigeMetricsSobreEsaMaquina(t *testing.T) {
	s, _ := servidorConMaquina(t)
	// Un principal SIN ninguna concesión de flota: el rol no otorga flota, y la ausencia no
	// significa «todas» (fleet_authz.go). Es el caso que la compuerta tiene que cerrar.
	ctx := withPrincipal(context.Background(), &Principal{
		Name: "mirón", ProjectID: "casa", Role: RoleAdmin, Read: ReadAll, Write: WriteAny,
	})
	_, e := s.toolFleetMaintenance(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","minutos":30}`))
	if e == nil {
		t.Fatal("un principal sin `metrics` sobre la máquina declaró mantenimiento: le apagó el monitoreo y frenó el auto-heal de una máquina ajena")
	}
	if !strings.Contains(e.Message, "metrics") {
		t.Errorf("el rechazo no nombra la capacidad que falta: %s", e.Message)
	}
	// Y no dejó la ventana creada a medias.
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if set, _ := s.engine.DevicesEnMantenimiento(time.Now()); set[d.ID] {
		t.Error("la tool rechazó y aun así dejó la máquina en mantenimiento")
	}
}

// Y EL SCHEDULER DE VERDAD NO ACTÚA. Ésta es LA prueba de la ventana; la de arriba mira el
// almacén, y el almacén puede estar perfecto mientras `aplicarPoliticas` lo ignora.
//
// La escribió el sabotaje: con `if enMantenimiento[d.ID] && false` —o sea, el scheduler evaluando
// políticas sobre una máquina en ventana— todo el resto seguía en verde. Y ése es exactamente el
// caso que la ventana existe para evitar: el auto-heal levantando un servicio en mitad de un
// mantenimiento planificado, con el aviso silenciado.
//
// Lleva CONTROL POSITIVO: sin él, esta prueba pasaría igual con un motor de políticas que no hace
// nada, que es como se ve un `aplicarPoliticas` roto por cualquier otro motivo.
//
// Sabotaje que la hace fallar: sacar (o neutralizar) el `if enMantenimiento[d.ID] { continue }`
// de aplicarPoliticas.
func TestElSchedulerNoAplicaPoliticasSobreUnaMaquinaEnVentana(t *testing.T) {
	ahora := time.Now()

	// CONTROL POSITIVO primero: la misma política, sin ventana, SÍ actúa.
	sc, dc := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	latir(t, sc, dc.ID, muestraSana(95, ahora), ahora)
	if n := sc.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("sin ventana la política tendría que actuar; actuó %d veces. Sin este control, la aserción de abajo pasaría con un motor de políticas roto", n)
	}

	// Y ahora la misma situación, con la máquina en ventana.
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple
	if _, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
		Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migración planificada",
	}); err != nil {
		t.Fatalf("no se pudo abrir la ventana: %v", err)
	}

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatalf("la política actuó %d vez/veces sobre una máquina EN MANTENIMIENTO: el auto-heal levanta el servicio en mitad del mantenimiento y el silence sólo garantiza que nadie se entere", n)
	}
	if filas := comandosEncolados(t, s); len(filas) != 0 {
		t.Fatalf("se encoló %d comando(s) durante la ventana", len(filas))
	}
}

// CANCELAR UNA VENTANA NO PUEDE ALCANZAR LA FILA DE OTRO TENANT.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// MEDIA COMPUERTA: MIRABA UNA MÁQUINA Y ESCRIBÍA OTRA
//
// `CancelarMantenimiento` tomaba sólo el id de la ventana y escribía `WHERE id = ? AND
// cancelada = 0`. El id es global; el permiso, en cambio, se comprobaba contra el device QUE
// NOMBRA QUIEN LLAMA. Con `metrics` sobre una máquina propia y un id ajeno —que sale del `id`
// que devuelve la propia tool al abrir, o de la cronología— se cancelaba la ventana de otro
// cliente.
//
// Y no queda en una fuga de lectura: la ventana FRENA EL AUTO-HEAL. Cancelar la ajena devuelve
// la automatización sobre una máquina que alguien puso a resguardo a propósito, en mitad de su
// mantenimiento. Del otro lado se ve como un servicio que se levanta solo sin que nadie lo haya
// pedido — y el `silence` de alertas, si lo hubiera, garantizaría que nadie se entere.
//
// Este test recorre el camino entero por la tool, que es por donde entra un llamador real.
//
// Sabotaje que lo hace fallar: sacar `device_id`/`project_id` del WHERE de
// CancelarMantenimiento, o pasarle el id sin el dueño desde methods_mantenimiento.go.
func TestCancelarMantenimientoNoAlcanzaLaVentanaDeOtroTenant(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolar := func(nombre, proyecto string) fleet.Device {
		t.Helper()
		if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
			"name": nombre, "tier": "A", "caps": []string{"metrics"},
			"project": proyecto, "os": "linux", "arch": "amd64",
		}); e != nil {
			t.Fatalf("no se pudo enrolar %q: %+v", nombre, e)
		}
		d, _, _ := s.engine.DevicePorNombre(proyecto, nombre)
		return d
	}
	victima := enrolar("pc-cliente", "acme")
	propia := enrolar("pc-mia", "casa")

	// La víctima abre su ventana por la vía normal.
	res, e := call(t, s, "musubi_fleet_maintenance", map[string]any{
		"device": "pc-cliente", "project": "acme", "minutos": 60, "motivo": "migración de postgres",
	})
	if e != nil {
		t.Fatalf("no se pudo abrir la ventana de la víctima: %+v", e)
	}
	idAjeno := idDeLaVentana(t, res)

	ahora := time.Now().UTC()
	if set, _ := s.engine.DevicesEnMantenimiento(ahora); !set[victima.ID] {
		t.Fatal("la ventana recién abierta no frena el auto-heal: el test no probaría nada")
	}

	// EL ATAQUE: nombro MI máquina —sobre la que tengo permiso de sobra— y paso el id ajeno.
	_, e = call(t, s, "musubi_fleet_maintenance", map[string]any{
		"device": "pc-mia", "project": "casa", "cancelar": idAjeno,
	})
	if e == nil {
		t.Error("la tool aceptó cancelar una ventana de OTRO proyecto nombrando una máquina propia")
	}

	// Lo que importa no es el error, es la fila: tiene que seguir en pie.
	set, err := s.engine.DevicesEnMantenimiento(ahora)
	if err != nil {
		t.Fatal(err)
	}
	if !set[victima.ID] {
		t.Fatal("FUGA ENTRE TENANTS: se canceló la ventana de otro cliente y el auto-heal vuelve a actuar sobre su máquina en mitad del mantenimiento")
	}
	filas, err := s.engine.MantenimientosDeDevice(victima.ID, 10)
	if err != nil || len(filas) != 1 {
		t.Fatalf("no se pudo releer la ventana de la víctima (%d filas, err=%v)", len(filas), err)
	}
	if filas[0].Cancelada {
		t.Error("FUGA ENTRE TENANTS: la fila de otro cliente quedó marcada como cancelada")
	}

	// Y el motor, directo: ni siquiera nombrando el proyecto correcto con el device equivocado.
	if hubo, err := s.engine.CancelarMantenimiento(propia.ID, victima.ProjectID, idAjeno); err != nil || hubo {
		t.Errorf("el UPDATE alcanzó una fila que no es de esa máquina (hubo=%v err=%v)", hubo, err)
	}
	if hubo, err := s.engine.CancelarMantenimiento(victima.ID, propia.ProjectID, idAjeno); err != nil || hubo {
		t.Errorf("el UPDATE alcanzó una fila que no es de ese proyecto (hubo=%v err=%v)", hubo, err)
	}

	// Y el dueño SÍ puede: la reparación no puede haber roto el camino legítimo.
	if _, e := call(t, s, "musubi_fleet_maintenance", map[string]any{
		"device": "pc-cliente", "project": "acme", "cancelar": idAjeno,
	}); e != nil {
		t.Fatalf("el dueño de la ventana no pudo cancelarla: %+v", e)
	}
	if set, _ := s.engine.DevicesEnMantenimiento(ahora); set[victima.ID] {
		t.Error("el dueño canceló y la ventana sigue frenando el auto-heal")
	}
}

func idDeLaVentana(t *testing.T, res interface{}) string {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(res.(CallToolResponse).Content[0].Text), &m); err != nil {
		t.Fatalf("no se pudo leer la respuesta de mantenimiento: %v", err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("la tool no devolvió el id de la ventana: %v", m)
	}
	return id
}
