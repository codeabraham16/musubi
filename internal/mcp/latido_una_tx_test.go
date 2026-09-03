package mcp

// EL TECHO DE ESCRITURAS POR LATIDO (Ola 0 · «De Cuatro a Dos Mil»).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES UNA PRUEBA Y NO UN COMENTARIO
//
// Un latido hacía como PISO cuatro escrituras en transacciones separadas: el autorreporte, la
// capacidad de preguntar, la señal de vida y el vencimiento de la cola. A 2000 máquinas cada
// 30 s son ~67 latidos/s, o sea ~270 commits/s con fsync (el WAL de esta base no fija
// `synchronous`, y el default es FULL). El sistema no se cae con un error: `busy_timeout`
// empieza a comerse latidos, las máquinas figuran caídas de a tandas, y el panel dice que se
// cayó media empresa cuando lo único que pasó es que la base no da más.
//
// Ese costo es INVISIBLE en el código: cada llamada suelta parece inofensiva, y la siguiente que
// alguien agregue «porque total ya hay varias» también. La única forma de que el techo se
// sostenga en el tiempo es contarlo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ SE CUENTA, Y POR QUÉ ES HONESTO CONTAR ESTO
//
// Se cuentan las llamadas de ESCRITURA que el latido le hace al motor, envolviendo el motor real
// con el seam de memory.StorageBackend (el mismo que usa backend_seam_test.go). Cada método de
// escritura del motor es, por construcción, UNA transacción: o un Exec en autocommit, o un
// Begin/Commit explícito. Así que contar llamadas es contar transacciones.
//
// Y se cuentan sólo las que TOCARON LA BASE. No es para aflojar el número: una sentencia de
// escritura que no matchea ninguna fila no ensucia ninguna página, así que SQLite no escribe
// frame de WAL ni hace fsync — no cuesta el commit que esta prueba acota. Donde el motor no
// alcanza para saberlo (ActualizarAutoreporte y compañía devuelven sólo error), se cuenta SIEMPRE,
// que es el lado seguro: el error posible es dar por escrito algo que no lo estaba, nunca
// perdonar una escritura real.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// topeLatidoEstable es lo que un latido de una máquina que no cambió de nada puede costar: UNA
// transacción, la de la señal de vida (que se lleva adentro la muestra, el vencimiento de la cola
// y la entrega). Es el número del camino que corre 67 veces por segundo.
const topeLatidoEstable = 1

// topeLatidoConInventario suma la escritura del inventario de servicios, que esta tarea tiene
// prohibido tocar: ReportarServicios sella la última vista de cada unit, así que sobre un
// inventario estable escribe igual.
//
// SU HERMANA, PodarServiciosAusentes, NO ENTRA EN LA CUENTA y no es un olvido: abre su propia
// transacción, pero sobre un inventario que no cambió no matchea ninguna fila, así que no ensucia
// página ni cuesta fsync. Está igual bajo la lupa del espía —la prueba de abajo falla si empieza
// a podar en cada latido—, que es la forma de que ese «no cuesta nada» siga siendo cierto.
const topeLatidoConInventario = topeLatidoEstable + 1

// espiaDeEscrituras envuelve el motor REAL —no es un fake: la base de abajo es SQLite y hace todo
// lo que hace en producción— y anota qué escrituras del latido tocaron la base.
//
// Embebe la interfaz y no el *DbEngine para que cualquier método de escritura NUEVO que el latido
// empiece a llamar pase derecho SIN contarse, y la prueba lo diga con el nombre de otra: es la
// única forma en que este archivo se equivoca, y se equivoca del lado que se nota (el conteo
// queda bajo y la aserción de más abajo, la que exige ver el latido, falla).
type espiaDeEscrituras struct {
	memory.StorageBackend
	mu         sync.Mutex
	escrituras []string
}

func (x *espiaDeEscrituras) anotar(metodo string, toco bool) {
	if !toco {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.escrituras = append(x.escrituras, metodo)
}

func (x *espiaDeEscrituras) resetear() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.escrituras = nil
}

func (x *espiaDeEscrituras) vistas() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]string(nil), x.escrituras...)
}

// ── Los métodos de escritura que puede tocar un latido ──────────────────────────────────────

func (x *espiaDeEscrituras) LatirYTomarComandos(id string, ahora time.Time, muestra string, tope int) (bool, []fleet.Comando, error) {
	vivo, cs, err := x.StorageBackend.LatirYTomarComandos(id, ahora, muestra, tope)
	// `vivo` es exactamente «el UPDATE matcheó la fila», o sea «hubo página sucia».
	x.anotar("LatirYTomarComandos", vivo)
	return vivo, cs, err
}

func (x *espiaDeEscrituras) LatirDevice(id string, ahora time.Time, muestra string) (bool, error) {
	vivo, err := x.StorageBackend.LatirDevice(id, ahora, muestra)
	x.anotar("LatirDevice", vivo)
	return vivo, err
}

func (x *espiaDeEscrituras) TomarComandos(deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {
	cs, err := x.StorageBackend.TomarComandos(deviceID, ahora, tope)
	// Abre su propia transacción, así que cuenta aunque no haya entregado nada: es justo la
	// llamada que la unificación sacó del latido y la que volvería si alguien la deshace.
	x.anotar("TomarComandos", true)
	return cs, err
}

func (x *espiaDeEscrituras) ActualizarAutoreporte(id, version, direccion string) error {
	err := x.StorageBackend.ActualizarAutoreporte(id, version, direccion)
	x.anotar("ActualizarAutoreporte", true)
	return err
}

func (x *espiaDeEscrituras) FijarCapacidadDePreguntar(deviceID string, puede bool) error {
	err := x.StorageBackend.FijarCapacidadDePreguntar(deviceID, puede)
	x.anotar("FijarCapacidadDePreguntar", true)
	return err
}

func (x *espiaDeEscrituras) GuardarRustdeskID(deviceID, rid string) error {
	err := x.StorageBackend.GuardarRustdeskID(deviceID, rid)
	x.anotar("GuardarRustdeskID", true)
	return err
}

func (x *espiaDeEscrituras) ReportarServicios(deviceID string, ahora time.Time, reportes []fleet.ReporteServicio) (int, int, error) {
	nuevos, actualizados, err := x.StorageBackend.ReportarServicios(deviceID, ahora, reportes)
	x.anotar("ReportarServicios", nuevos+actualizados > 0)
	return nuevos, actualizados, err
}

func (x *espiaDeEscrituras) PodarServiciosAusentes(deviceID string, vivos []string, vacioAfirma bool) (int64, error) {
	podados, err := x.StorageBackend.PodarServiciosAusentes(deviceID, vivos, vacioAfirma)
	x.anotar("PodarServiciosAusentes", podados > 0)
	return podados, err
}

// servidorConEspiaDeEscrituras es servidorConFlota con el motor envuelto por el espía.
func servidorConEspiaDeEscrituras(t *testing.T) (*McpServer, *httptest.Server, string, *espiaDeEscrituras) {
	t.Helper()
	espia := &espiaDeEscrituras{StorageBackend: memtest.NuevoEngine(t, t.TempDir())}
	s := NewMcpServer(espia, t.TempDir(), embedding.NoopProvider{})
	tokenDevice := enrolarDePrueba(t, s, "casa", "pc-gio")
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{
		reqTimeout: 10 * time.Second, token: "token-de-una-persona", loopbackOnly: true,
	}))
	t.Cleanup(ts.Close)
	return s, ts, tokenDevice, espia
}

// cuerpoDelAgenteReal es lo que manda `musubi agent` en TODOS sus latidos: versión, dirección,
// capacidad de preguntar, id de pantalla y la muestra. Que los mande siempre es el punto — es por
// eso que las escrituras «ocasionales» ocurrían en cada ciclo.
func cuerpoDelAgenteReal(version, direccion string) string {
	return fmt.Sprintf(`{"version":%q,"direccion":%q,"rustdesk_id":"123456789",`+
		`"puede_preguntar":true,`+
		`"muestra":{"tomada":"2026-09-03T12:00:00Z","num_cpu":8,"cpu_pct":12.5,`+
		`"mem_total":16000000,"mem_usada":8000000}}`, version, direccion)
}

func latirEnLaPuerta(t *testing.T, ts *httptest.Server, token, cuerpo string) {
	t.Helper()
	code, body := postCon(t, ts.URL+fleetHeartbeatPath, token, cuerpo)
	if code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
}

// EL TECHO. Un latido de régimen —la máquina reporta lo mismo que la vez anterior, que es lo que
// hace el 99,99 % de los latidos de su vida— cuesta UNA transacción de escritura.
//
// Sabotaje que la hace fallar: en internal/mcp/fleet_http.go, volver a escribir el autorreporte en
// cada latido, o sea reemplazar la guarda
//
//	if (version != "" && version != d.AgentVer) || (direccion != "" && direccion != d.Address) {
//
// por el `if cuerpo.Version != "" || cuerpo.Direccion != "" {` que había antes. También se pone
// roja sacando la guarda de FijarCapacidadDePreguntar, o volviendo a partir el latido en
// LatirDevice + TomarComandos.
func TestUnLatidoEstableEsUnaSolaTransaccionDeEscritura(t *testing.T) {
	_, ts, token, espia := servidorConEspiaDeEscrituras(t)
	cuerpo := cuerpoDelAgenteReal("0.130.0", "100.64.0.7")

	// EL PRIMER LATIDO SÍ ESCRIBE DE MÁS, y está bien: nada de lo que la máquina reporta estaba
	// guardado todavía. Es una vez en la vida del enrolamiento, no 67 por segundo.
	latirEnLaPuerta(t, ts, token, cuerpo)
	espia.resetear()

	// El segundo es el ESTABLE: mismo cuerpo, misma máquina, nada cambió.
	latirEnLaPuerta(t, ts, token, cuerpo)

	vistas := espia.vistas()
	if len(vistas) > topeLatidoEstable {
		t.Fatalf("un latido estable costó %d escrituras (%s) y el tope es %d: a 2000 máquinas "+
			"cada 30 s eso son ~%.0f commits/s con fsync",
			len(vistas), strings.Join(vistas, ", "), topeLatidoEstable, float64(len(vistas))*66.7)
	}
	// Y LA ESCRITURA QUE SÍ TIENE QUE ESTAR. Sin esta mitad, la prueba pasaría con un latido que
	// no escribe nada — o sea con la señal de vida perdida, que es peor que el problema original.
	if len(vistas) != 1 || vistas[0] != "LatirYTomarComandos" {
		t.Fatalf("las escrituras del latido fueron %v, esperaba exactamente la de la señal de "+
			"vida: si el latido no estampa vida, la flota entera figura caída", vistas)
	}
}

// UN LATIDO QUE TRAE INVENTARIO CUESTA DOS: la del latido y la del inventario. Esta tarea no
// toca el inventario de servicios —bajar SU costo es otro trabajo— pero el latido tampoco puede
// sumarle una tercera por el camino.
//
// Sabotaje que la hace fallar: el mismo de arriba — devolver el autorreporte incondicional en
// internal/mcp/fleet_http.go hace que un latido con inventario cueste tres.
func TestElInventarioNoLeAgregaMasQueSuEscrituraAlLatido(t *testing.T) {
	_, ts, token, espia := servidorConEspiaDeEscrituras(t)

	// El agente real manda el inventario ADENTRO del mismo latido, no aparte.
	cuerpo := `{"version":"0.130.0","direccion":"100.64.0.7","rustdesk_id":"123456789",` +
		`"puede_preguntar":true,"servicios":[` +
		`{"nombre":"postgresql.service","clase":"systemd","salud":{"tomada":"2026-09-03T12:00:00Z","estado":"corriendo"}},` +
		`{"nombre":"nginx.service","clase":"systemd","salud":{"tomada":"2026-09-03T12:00:00Z","estado":"corriendo"}}]}`

	latirEnLaPuerta(t, ts, token, cuerpo)
	espia.resetear()
	latirEnLaPuerta(t, ts, token, cuerpo)

	vistas := espia.vistas()
	if len(vistas) > topeLatidoConInventario {
		t.Fatalf("un latido CON inventario costó %d escrituras (%s) y el tope es %d",
			len(vistas), strings.Join(vistas, ", "), topeLatidoConInventario)
	}
	// La poda no escribe nada cuando la máquina reporta lo mismo: si apareciera acá, el
	// inventario estaría dando de baja y volviendo a crear las mismas filas en cada latido.
	for _, v := range vistas {
		if v == "PodarServiciosAusentes" {
			t.Fatalf("la poda por ausencia escribió en un latido que reportó el MISMO "+
				"inventario: %v", vistas)
		}
	}
}

// ENTREGAR UN COMANDO NO CUESTA UNA TRANSACCIÓN DE MÁS, que es la mitad «meter la entrega adentro
// del latido» del arreglo. Y el comando tiene que llegar igual: un techo que se cumple porque la
// cola dejó de funcionar no es un techo, es una regresión.
//
// Sabotaje que la hace fallar: volver a llamar a TomarComandos por separado desde handlerLatido
// (o sea deshacer LatirYTomarComandos) — el conteo pasa a dos.
func TestEntregarUnComandoNoLeCuestaOtraTransaccionAlLatido(t *testing.T) {
	s, ts, token, espia := servidorConEspiaDeEscrituras(t)
	cuerpo := cuerpoDelAgenteReal("0.130.0", "100.64.0.7")
	latirEnLaPuerta(t, ts, token, cuerpo) // el primero, el que se paga una vez

	d, _, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio",
		Argv: []string{"uptime"}, Timeout: 10 * time.Second, Creado: time.Now(),
	}); err != nil {
		t.Fatalf("EncolarComando: %v", err)
	}

	espia.resetear()
	code, body := postCon(t, ts.URL+fleetHeartbeatPath, token, cuerpo)
	if code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	if !strings.Contains(body, "uptime") {
		t.Fatalf("el comando no viajó de vuelta en el latido: %s", body)
	}
	if vistas := espia.vistas(); len(vistas) > topeLatidoEstable {
		t.Fatalf("un latido que entregó un comando costó %d escrituras (%s) y el tope es %d: la "+
			"entrega tiene que ir adentro de la transacción de la señal de vida",
			len(vistas), strings.Join(vistas, ", "), topeLatidoEstable)
	}
}

// LO QUE SÍ CAMBIÓ SE ESCRIBE, y esta prueba es la contracara obligatoria del techo: la forma
// fácil de cumplirlo es dejar de escribir, y entonces una máquina actualizada seguiría figurando
// con la versión vieja en el inventario para siempre — que es el bug que la comparación tiene que
// evitar CREAR mientras evita el otro.
//
// Sabotaje que la hace fallar: INVERTIR la comparación en internal/mcp/fleet_http.go — escribir
// `version == d.AgentVer` y `direccion == d.Address` en vez de `!=` — o borrar la llamada a
// ActualizarAutoreporte y quedarse con la guarda sola.
func TestUnAgenteQueSeActualizoSiEscribeSuVersionNueva(t *testing.T) {
	s, ts, token, espia := servidorConEspiaDeEscrituras(t)

	latirEnLaPuerta(t, ts, token, cuerpoDelAgenteReal("0.130.0", "100.64.0.7"))
	espia.resetear()

	// Alguien actualizó el agente y lo movió de red: las dos cosas que este campo existe para
	// contar.
	latirEnLaPuerta(t, ts, token, cuerpoDelAgenteReal("0.131.0", "100.64.0.9"))

	d, _, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil {
		t.Fatal(err)
	}
	if d.AgentVer != "0.131.0" {
		t.Errorf("agent_version = %q, esperaba 0.131.0: el inventario diría para siempre que "+
			"esta máquina corre un binario que ya no corre", d.AgentVer)
	}
	if d.Address != "100.64.0.9" {
		t.Errorf("address = %q, esperaba 100.64.0.9: nadie podría alcanzar la máquina", d.Address)
	}
	var vioAutoreporte bool
	for _, v := range espia.vistas() {
		if v == "ActualizarAutoreporte" {
			vioAutoreporte = true
		}
	}
	if !vioAutoreporte {
		t.Errorf("el autorreporte no se escribió aunque la versión cambió: %v", espia.vistas())
	}
}

// Y LA CAPACIDAD DE PREGUNTAR TAMBIÉN, que es la que más caro sale equivocarse: si un `false`
// nuevo no se guardara, una máquina que perdió su escritorio seguiría declarando que puede
// preguntarle a alguien, y un `pide` prometería un diálogo que nadie va a ver nunca.
//
// Sabotaje que la hace fallar: invertir la comparación en fleet_http.go
// (`*cuerpo.PuedePreguntar == d.PuedePreguntar`), o sacar la llamada de adentro de la guarda.
func TestUnaMaquinaQuePerdioSuEscritorioSiEscribeElFalse(t *testing.T) {
	s, ts, token, _ := servidorConEspiaDeEscrituras(t)

	latirEnLaPuerta(t, ts, token, `{"version":"0.130.0","puede_preguntar":true}`)
	d, _, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil {
		t.Fatal(err)
	}
	if !d.PuedePreguntar {
		t.Fatalf("el `true` inicial no se guardó: el escenario no es el que dice")
	}

	// El escritorio se fue (el agente pasó a correr como servicio, alguien desinstaló zenity).
	latirEnLaPuerta(t, ts, token, `{"version":"0.130.0","puede_preguntar":false,"motivo_no_preguntar":"sin zenity"}`)

	d, _, err = s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil {
		t.Fatal(err)
	}
	if d.PuedePreguntar {
		t.Error("la máquina sigue declarando que puede preguntar: un `pide` se le seguiría " +
			"tratando como si hubiera alguien del otro lado")
	}
}
