package mcp

// carga_test.go — EL BANCO DE CARGA: N agentes simulados contra un cerebro EN PROCESO.
// Plan «De Cuatro a Dos Mil», Ola 0.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE
//
// Todos los números del plan —«dos mil máquinas latiendo cada 30 s», «el cerebro aguanta»— eran
// proyecciones: nadie había puesto dos mil tokens a latir contra el handler real y la base real.
// Hasta acá lo más grande que había latido junto eran cuatro máquinas.
//
// Este banco lo hace sin red ni proceso externo: levanta EL MISMO servidor que usan las pruebas
// de flota (servidorConFlota), enrola N dispositivos por la tool real (musubi_fleet_enroll) y los
// pone a latir por /fleet/heartbeat con el mismo cuerpo que manda el agente de verdad —versión,
// dirección, `puede_preguntar`, una muestra válida y, con la cadencia real, el inventario de
// servicios. Lo que se mide es lo que el cerebro hace por latido, no un fixture recortado.
//
// CÓMO CORRERLO
//
//	MUSUBI_CARGA=1 go test ./internal/mcp/ -run 'TestBancoDeCarga$' -v -timeout 15m
//
// No corre por default y tampoco con -short: 60 s y dos mil tokens no tienen lugar en
// `go test ./...`, y una prueba lenta que se cuela en la suite es una prueba que alguien apaga.
// Perillas, todas por variable de entorno:
//
//	MUSUBI_CARGA_N            dispositivos enrolados                         (default 2000)
//	MUSUBI_CARGA_T            segundos de carga                              (default 60)
//	MUSUBI_CARGA_INTERVALO    segundos entre latidos de UN agente            (default 30, la
//	                          cadencia real del agente; 0 = sin pausa, para medir el TECHO)
//	MUSUBI_CARGA_INVENTARIO   fracción de latidos que llevan 60 servicios    (default
//	                          intervalo/InventarioCada, o sea 0,1: el agente real reenvía el
//	                          inventario completo cada fleet.InventarioCada = 5 min)
//	MUSUBI_CARGA_CONEXIONES   conexiones TCP simultáneas del lado cliente    (default 256)
//	MUSUBI_CARGA_ASSERT=1     además de reportar, FALLA si p99 >= 200 ms
//
// CÓMO LEER LA SALIDA
//
//	banco de carga · 2000 agentes · 1m0s · latido cada 30s · inventario en 1 de cada 10 (60 servicios) · 256 conexiones
//	enrolar 2000 dispositivos: 15.3s
//	latidos: 4000 ok · 0 con error · 66,7 latidos/s (en 1m0s)
//	latencia: p50 2,9 ms · p95 8,1 ms · p99 19,6 ms · max 41,2 ms
//	errores: ninguno
//	escrituras al almacén por latido: 3,21 (12840 en 4000 latidos ok)
//	  ActualizarAutoreporte 4000 · GuardarRustdeskID 0 · FijarCapacidadDePreguntar 4000 · ReportarServicios 420 · PodarServiciosAusentes 420 · LatirDevice 4000
//	lecturas al almacén por latido: 2,00 — DevicePorToken 4000 · TomarComandos 4000
//	NO medido: sentencias SQL ni transacciones por latido (ver el encabezado de carga_test.go)
//
// Los números de arriba son la FORMA de la salida, no un resultado: los valores reales dependen
// de la máquina y hay que leerlos de la corrida propia.
//
// LO PRIMERO QUE ESTE BANCO ENCONTRÓ, y por eso vale la pena dejarlo escrito: con la perilla del
// techo (30 agentes, INTERVALO=0) las corridas de esta máquina dieron entre 60 y 110 latidos/s
// con p99 de UNO A DOS SEGUNDOS, con cero errores. O sea: el cerebro no se cae, pero la cola de
// latencia se va a más de un segundo por contención de escritura en SQLite muchísimo antes de
// las dos mil máquinas. «El cerebro aguanta» era cierto y no era la pregunta. La medición
// «escrituras al almacén por latido: 3,2» dice dónde mirar primero.
//
// Con la cadencia real (INTERVALO=30) «latidos/s» es N/30 por construcción y no dice nada del
// cerebro: ahí lo que importa son la latencia y los errores. Para saber cuántos latidos por
// segundo AGUANTA el cerebro, correr con MUSUBI_CARGA_INTERVALO=0 —cada agente vuelve a latir
// apenas recibe la respuesta anterior— y leer «latidos/s» como el techo de ESTA máquina.
//
// La línea «errores» desglosa por causa: `503 ×12` es el cerebro devolviendo «registry
// unavailable» (la base no contestó), `transporte ×3` es el cliente sin respuesta (timeout,
// conexión rechazada), `muestra no guardada ×N` es un 200 cuya nota dice que la telemetría se
// descartó, e `inventario no guardado ×N` lo mismo para los servicios. Esos dos últimos son el
// banco vigilándose a sí mismo: un 200 con la muestra descartada sería «el cerebro aguanta» por
// el motivo equivocado, porque no habría escrito nada.
//
// ESCRITURAS POR LATIDO: QUÉ SE MIDE Y QUÉ NO
//
// Es el número que más le importa a quien planifique la base, y acá se mide de verdad —no se
// estima— porque el almacén es un SEAM: `McpServer.engine` es la interfaz memory.StorageBackend,
// así que el banco la envuelve (almacenQueCuentaLlamadas) y cuenta las llamadas que el latido le
// hace, con desglose por método.
//
// LO QUE ESE NÚMERO ES: llamadas al almacén por latido. LO QUE NO ES: sentencias SQL ni
// transacciones. Una llamada puede ser varias sentencias adentro —ReportarServicios hace un
// upsert por servicio—, y eso NO se mide acá y no se inventa: `DbEngine.db` no se exporta, y
// aunque se exportara, `total_changes()` de SQLite es POR CONEXIÓN y el pool tiene varias;
// sumarlo daría un número plausible y falso. Por eso la última línea del reporte dice «no
// medido» en voz alta en vez de dejar el hueco para que alguien lo llene con un cero.
//
// El desglose es lo que hace útil el promedio: un 3,2 se lee «tres escrituras fijas por latido
// —autorreporte, capacidad de preguntar, latido— más el inventario cuando toca», y con eso se
// puede decidir si vale la pena juntarlas en una sola transacción.
//
// LA PRUEBA DE FORMA (TestElBancoDeCargaEstaGateado) SÍ CORRE SIEMPRE: verifica que el banco no
// arranca sin la variable. Sin ella, quitar el gate «para probar algo» y olvidarlo colaría un
// minuto de carga en cada `go test ./...`, y nadie lo notaría hasta que la suite tarde el doble.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// bancoDeCargaHabilitado es EL GATE, y es una función pura a propósito: la prueba de forma la
// recorre esquina por esquina sin tocar el entorno real. Las dos condiciones son AND: la variable
// pide correrlo, y -short lo veta aunque se lo pida.
func bancoDeCargaHabilitado(env func(string) string, short bool) bool {
	return env("MUSUBI_CARGA") == "1" && !short
}

// cuerpoDelBancoDeCarga es el cuerpo que TestBancoDeCarga corre DESPUÉS del gate. Es una variable
// y no una llamada directa para que la prueba de forma pueda cambiarlo por uno que sólo anota si
// lo llamaron: así se prueba el gate de la PRUEBA REAL, no el de una copia.
var cuerpoDelBancoDeCarga = correrBancoDeCarga

// TestBancoDeCarga es el banco. Ver el encabezado del archivo para correrlo y leerlo.
func TestBancoDeCarga(t *testing.T) {
	if !bancoDeCargaHabilitado(os.Getenv, testing.Short()) {
		t.Skip("opt-in: MUSUBI_CARGA=1 (y sin -short) para correr el banco de carga")
	}
	cuerpoDelBancoDeCarga(t, parametrosDeCargaDesdeEnv(t))
}

// EL BANCO ESTÁ GATEADO, y esta prueba corre en cada `go test ./...` para que siga estándolo.
//
// Primero la función pura en sus tres esquinas; después la prueba REAL, corrida como subtest con
// la variable vacía y el cuerpo reemplazado por un testigo: si el testigo se ejecuta, el gate no
// está. Y el control positivo —con la variable puesta el testigo SÍ corre— existe para que la
// mitad negativa no pase vacía: un TestBancoDeCarga que nunca llamara al seam la pasaría igual.
//
// Sabotaje que la hace fallar: borrar el `if !bancoDeCargaHabilitado(...) { t.Skip(...) }` de
// TestBancoDeCarga, o hacer que bancoDeCargaHabilitado devuelva true sin mirar la variable.
func TestElBancoDeCargaEstaGateado(t *testing.T) {
	sin := func(string) string { return "" }
	con := func(k string) string {
		if k == "MUSUBI_CARGA" {
			return "1"
		}
		return ""
	}
	if bancoDeCargaHabilitado(sin, false) {
		t.Fatal("el banco correría sin MUSUBI_CARGA: un minuto de carga en cada go test ./...")
	}
	if bancoDeCargaHabilitado(con, true) {
		t.Fatal("el banco correría con -short aunque se lo pida la variable")
	}
	if !bancoDeCargaHabilitado(con, false) {
		t.Fatal("el banco no corre ni pidiéndoselo: el gate se volvió un candado sin llave")
	}

	guardado := cuerpoDelBancoDeCarga
	t.Cleanup(func() { cuerpoDelBancoDeCarga = guardado })
	corrio := false
	cuerpoDelBancoDeCarga = func(*testing.T, parametrosDeCarga) { corrio = true }

	t.Setenv("MUSUBI_CARGA", "")
	t.Run("sin la variable, TestBancoDeCarga se saltea antes de tocar nada", TestBancoDeCarga)
	if corrio {
		t.Fatal("TestBancoDeCarga corrió su cuerpo sin MUSUBI_CARGA=1: el gate no está")
	}
	// El control positivo sólo vale sin -short: con -short el gate veta a propósito.
	if !testing.Short() {
		t.Setenv("MUSUBI_CARGA", "1")
		t.Run("con la variable, TestBancoDeCarga llega a su cuerpo", TestBancoDeCarga)
		if !corrio {
			t.Fatal("con MUSUBI_CARGA=1 el cuerpo no corrió: el gate de arriba se probó contra nada")
		}
	}
}

// parametrosDeCarga son las perillas del banco. Ver el encabezado.
type parametrosDeCarga struct {
	N                  int
	Duracion           time.Duration
	Intervalo          time.Duration
	FraccionInventario float64
	Conexiones         int
	Assert             bool
}

// parametrosDeCargaDesdeEnv lee las perillas. Un valor que no se puede leer FALLA en vez de caer
// al default: un `MUSUBI_CARGA_N=2OOO` corriendo con 2000 sin avisar es un reporte que miente
// sobre sus propias condiciones, y el reporte es todo lo que este archivo produce.
func parametrosDeCargaDesdeEnv(t *testing.T) parametrosDeCarga {
	t.Helper()
	entero := func(clave string, def int) int {
		v := strings.TrimSpace(os.Getenv(clave))
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			t.Fatalf("%s=%q: quiero un entero no negativo", clave, v)
		}
		return n
	}
	p := parametrosDeCarga{
		N:          entero("MUSUBI_CARGA_N", 2000),
		Duracion:   time.Duration(entero("MUSUBI_CARGA_T", 60)) * time.Second,
		Intervalo:  time.Duration(entero("MUSUBI_CARGA_INTERVALO", 30)) * time.Second,
		Conexiones: entero("MUSUBI_CARGA_CONEXIONES", 256),
		Assert:     os.Getenv("MUSUBI_CARGA_ASSERT") == "1",
	}
	if p.N == 0 || p.Duracion == 0 || p.Conexiones == 0 {
		t.Fatal("MUSUBI_CARGA_N, MUSUBI_CARGA_T y MUSUBI_CARGA_CONEXIONES tienen que ser mayores que cero")
	}
	// La fracción por default SALE DE LA CADENCIA REAL y no de un número lindo: el agente reenvía
	// el inventario completo cada fleet.InventarioCada y late cada Intervalo. Con el techo
	// (Intervalo 0) no hay cadencia de la que derivarla y se conserva la de la cadencia real, para
	// que el techo se mida con la misma MEZCLA de latidos y no con una más liviana.
	intervaloParaLaFraccion := p.Intervalo
	if intervaloParaLaFraccion == 0 {
		intervaloParaLaFraccion = 30 * time.Second
	}
	p.FraccionInventario = float64(intervaloParaLaFraccion) / float64(fleet.InventarioCada)
	if v := strings.TrimSpace(os.Getenv("MUSUBI_CARGA_INVENTARIO")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f > 1 {
			t.Fatalf("MUSUBI_CARGA_INVENTARIO=%q: quiero una fracción entre 0 y 1", v)
		}
		p.FraccionInventario = f
	}
	return p
}

// serviciosPorInventario es cuántas units manda una máquina cuando manda el inventario. Sesenta
// es una máquina real de la flota y queda debajo de fleet.ServiciosPorLatido (64) A PROPÓSITO: el
// banco tiene que medir el camino que GUARDA, no el que rebota por exceder el techo.
const serviciosPorInventario = 60

// almacenQueCuentaLlamadas envuelve el almacén real y cuenta lo que el latido le pide, por método.
//
// EMBEBE LA INTERFAZ (con el engine real adentro, nunca nil) y no la reimplementa: así todo lo de
// StorageBackend que este banco no toca pasa derecho, y envolver el almacén no cambia lo que se
// está midiendo.
//
// Contadores atómicos y no un mutex: con MUSUBI_CARGA_INTERVALO=0 y dos mil goroutines, un lock
// compartido por ocho métodos mediría el lock y no el cerebro.
type almacenQueCuentaLlamadas struct {
	memory.StorageBackend
	devicePorToken atomic.Int64
	latir          atomic.Int64
	autoreporte    atomic.Int64
	puedePreguntar atomic.Int64
	rustdesk       atomic.Int64
	reportar       atomic.Int64
	podar          atomic.Int64
	comandos       atomic.Int64
	latirYTomar    atomic.Int64
}

func (a *almacenQueCuentaLlamadas) DevicePorToken(token string) (fleet.Device, bool, error) {
	a.devicePorToken.Add(1)
	return a.StorageBackend.DevicePorToken(token)
}

func (a *almacenQueCuentaLlamadas) LatirDevice(id string, ahora time.Time, muestra string) (bool, error) {
	a.latir.Add(1)
	return a.StorageBackend.LatirDevice(id, ahora, muestra)
}

func (a *almacenQueCuentaLlamadas) ActualizarAutoreporte(id, version, direccion string) error {
	a.autoreporte.Add(1)
	return a.StorageBackend.ActualizarAutoreporte(id, version, direccion)
}

func (a *almacenQueCuentaLlamadas) FijarCapacidadDePreguntar(deviceID string, puede bool) error {
	a.puedePreguntar.Add(1)
	return a.StorageBackend.FijarCapacidadDePreguntar(deviceID, puede)
}

func (a *almacenQueCuentaLlamadas) GuardarRustdeskID(deviceID, rid string) error {
	a.rustdesk.Add(1)
	return a.StorageBackend.GuardarRustdeskID(deviceID, rid)
}

func (a *almacenQueCuentaLlamadas) ReportarServicios(deviceID string, ahora time.Time, reportes []fleet.ReporteServicio) (int, int, error) {
	a.reportar.Add(1)
	return a.StorageBackend.ReportarServicios(deviceID, ahora, reportes)
}

func (a *almacenQueCuentaLlamadas) PodarServiciosAusentes(deviceID string, vivos []string, vacioAfirma bool) (int64, error) {
	a.podar.Add(1)
	return a.StorageBackend.PodarServiciosAusentes(deviceID, vivos, vacioAfirma)
}

// LatirYTomarComandos es la que de verdad escribe el latido desde que las dos mitades se
// unieron en una transacción. Sin envolverla, el banco contaba `LatirDevice 0 · TomarComandos 0`
// y el promedio de escrituras salía SUBESTIMADO — el número que este banco existe para dar,
// mal, y con pinta de mejora. Lo encontró el refutador de esa rama, y la lección es del espía y
// no de esa rama: un contador por NOMBRE DE MÉTODO se queda ciego con cada método nuevo, y su
// ceguera se ve exactamente igual que un cero.
func (a *almacenQueCuentaLlamadas) LatirYTomarComandos(id string, ahora time.Time, muestra string, tope int) (bool, []fleet.Comando, error) {
	a.latirYTomar.Add(1)
	return a.StorageBackend.LatirYTomarComandos(id, ahora, muestra, tope)
}

func (a *almacenQueCuentaLlamadas) TomarComandos(deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {
	a.comandos.Add(1)
	return a.StorageBackend.TomarComandos(deviceID, ahora, tope)
}

// llamadaContada es una fila del desglose. `escribe` separa lo que toca la base de lo que sólo la
// lee: mezclarlas daría un «escrituras por latido» inflado, que es peor que no tenerlo.
type llamadaContada struct {
	nombre   string
	contador *atomic.Int64
	escribe  bool
}

// contadas devuelve los contadores en ORDEN DE APARICIÓN EN EL HANDLER, para que el desglose se
// lea como el camino del latido y no en el orden alfabético de sus nombres.
func (a *almacenQueCuentaLlamadas) contadas() []llamadaContada {
	return []llamadaContada{
		{"DevicePorToken", &a.devicePorToken, false},
		{"ActualizarAutoreporte", &a.autoreporte, true},
		{"GuardarRustdeskID", &a.rustdesk, true},
		{"FijarCapacidadDePreguntar", &a.puedePreguntar, true},
		{"ReportarServicios", &a.reportar, true},
		{"PodarServiciosAusentes", &a.podar, true},
		{"LatirDevice", &a.latir, true},
		// La combinada escribe siempre (sella el latido) y de paso entrega comandos: cuenta como
		// UNA escritura, que es exactamente el punto de haberlas unido.
		{"LatirYTomarComandos", &a.latirYTomar, true},
		// TomarComandos lee la cola de ESTA máquina y escribe sólo si hay algo que entregar. En
		// este banco la cola está vacía, así que es una lectura y cuenta como tal: meterla en las
		// escrituras sumaría una por latido que en este escenario no ocurre.
		{"TomarComandos", &a.comandos, false},
	}
}

// enCero borra los contadores. Se llama DESPUÉS de enrolar: dar de alta dos mil dispositivos es
// trabajo del banco y no del latido, y contarlo adentro del promedio lo ensuciaría.
func (a *almacenQueCuentaLlamadas) enCero() {
	for _, c := range a.contadas() {
		c.contador.Store(0)
	}
}

// medicionDeCarga junta lo que pasó. Un mutex y no atómicos: el lock se toma UNA vez por latido y
// dura nanosegundos frente a un round-trip HTTP, y un solo lock hace que el conteo y las
// latencias sean consistentes entre sí — con atómicos separados, el reporte podría imprimir un
// total que no coincide con la cantidad de latencias que ordenó.
type medicionDeCarga struct {
	mu                   sync.Mutex
	latencias            []time.Duration
	ok                   int
	porStatus            map[int]int
	transporte           int
	muestraNoGuardada    int
	inventarioNoGuardado int
	// primeraNota guarda la primera nota de descarte de cada clase, para que el reporte diga POR
	// QUÉ se descartó y no sólo cuántas veces. Un «muestra no guardada ×4000» sin el motivo
	// obliga a correr el banco entero de nuevo con un debugger.
	primeraNota map[string]string
}

func (m *medicionDeCarga) anotar(lat time.Duration, status int, errTransporte error, r respuestaLatido, conInventario bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencias = append(m.latencias, lat)
	if errTransporte != nil {
		m.transporte++
		if _, ya := m.primeraNota["transporte"]; !ya {
			m.primeraNota["transporte"] = errTransporte.Error()
		}
		return
	}
	if status != http.StatusOK {
		m.porStatus[status]++
		return
	}
	m.ok++
	if r.Muestra != "guardada" {
		m.muestraNoGuardada++
		if _, ya := m.primeraNota["muestra"]; !ya {
			m.primeraNota["muestra"] = r.Muestra
		}
	}
	if conInventario && !strings.HasPrefix(r.Servicios, "guardados:") {
		m.inventarioNoGuardado++
		if _, ya := m.primeraNota["inventario"]; !ya {
			m.primeraNota["inventario"] = r.Servicios
		}
	}
}

// percentilDeCarga es el percentil por RANGO MÁS CERCANO sobre la muestra ordenada: el p99 de 100
// valores es el 99.º, no una interpolación entre el 99.º y el 100.º. Es el que menos suaviza la
// cola, que es justamente lo que se quiere ver — un p99 optimista en un banco de carga es peor
// que no tener p99.
func percentilDeCarga(ordenadas []time.Duration, q float64) time.Duration {
	if len(ordenadas) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(ordenadas)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(ordenadas) {
		i = len(ordenadas) - 1
	}
	return ordenadas[i]
}

// cuerpoDeLatidoDeCarga arma el cuerpo con la MISMA forma que `latir` en cmd/musubi/agent.go:
// versión, dirección, capacidad de preguntar, muestra y —cuando toca— el inventario. Se calca la
// forma en vez de importar al agente porque `latir` es del paquete main y además lee el host real.
func cuerpoDeLatidoDeCarga(i int, conInventario bool) ([]byte, error) {
	m := muestraDePrueba()
	// Cada agente con su PROPIO perfil, para que dos mil filas no guarden el mismo JSON: una base
	// con el mismo blob repetido comprime y cachea distinto que una con dos mil distintos, y el
	// banco existe para medir la segunda.
	cpu := float64((i * 37) % 100)
	m.CPUPct = &cpu
	m.MemUsada = uint64(1+i%6) << 30
	m.Tomada = time.Now().UTC()
	// `puede_preguntar` viaja SIEMPRE, como lo manda el agente nuevo: es una escritura por latido
	// (FijarCapacidadDePreguntar) y omitirla le sacaría al promedio una de las tres fijas.
	puede := false
	carga := map[string]any{
		"version":         "banco-de-carga",
		"direccion":       fmt.Sprintf("10.%d.%d.%d", (i>>16)&255, (i>>8)&255, i&255),
		"puede_preguntar": puede,
		"muestra":         m,
	}
	if conInventario {
		reportes := make([]fleet.ReporteServicio, serviciosPorInventario)
		for k := range reportes {
			salud := saludViva(fleet.EstadoCorriendo)
			reinicios := k % 3
			salud.Reinicios = &reinicios
			reportes[k] = fleet.ReporteServicio{
				Nombre: fmt.Sprintf("svc-%02d.service", k), Clase: "systemd", Salud: salud,
			}
		}
		carga["servicios"] = reportes
	}
	return json.Marshal(carga)
}

// correrBancoDeCarga es el cuerpo del banco: enrola, late, mide y reporta.
//
// SIN t.Helper(), a propósito: con él, `go test -v` le pone a CADA línea del reporte el número de
// la línea de TestBancoDeCarga, y las diez quedan diciendo «carga_test.go:120». El reporte es el
// producto de este archivo y tiene que poder citarse por línea.
func correrBancoDeCarga(t *testing.T, p parametrosDeCarga) {
	// El inventario va en 1 de cada `periodo` latidos, DESFASADO POR AGENTE para que no caigan
	// todos juntos: con fracción 0,1 son 200 inventarios por vuelta repartidos, no 2000 en un
	// mismo segundo y ninguno en los nueve siguientes. Una ráfaga sincronizada es un escenario
	// distinto —y más fácil de defender— que el que vive una flota real.
	periodo := 0
	if p.FraccionInventario > 0 {
		periodo = int(math.Round(1 / p.FraccionInventario))
		if periodo < 1 {
			periodo = 1
		}
	}
	cadencia := "sin pausa (techo)"
	if p.Intervalo > 0 {
		cadencia = fmt.Sprintf("cada %s", p.Intervalo)
	}
	inventario := "nunca"
	if periodo > 0 {
		inventario = fmt.Sprintf("en 1 de cada %d (%d servicios)", periodo, serviciosPorInventario)
	}
	t.Logf("banco de carga · %d agentes · %s · latido %s · inventario %s · %d conexiones",
		p.N, p.Duracion, cadencia, inventario, p.Conexiones)

	s, ts, _, _ := servidorConFlota(t)
	// El contador se instala ANTES de enrolar y se pone en cero DESPUÉS. Reemplazar el campo es
	// seguro justo acá y en ningún otro momento: NewMcpServer no arranca ninguna goroutine y el
	// httptest todavía no recibió un request, así que la escritura no compite con nadie.
	contador := &almacenQueCuentaLlamadas{StorageBackend: s.engine}
	s.engine = contador

	// Se enrola por la tool real, una por una, como lo haría un admin con un guion: el costo de
	// enrolar también es un número del plan —dar de alta dos mil máquinas es un trámite real— y
	// por eso se reporta aparte en vez de esconderse en el setup.
	desde := time.Now()
	tokens := make([]string, p.N)
	for i := range tokens {
		tokens[i] = enrolarDePrueba(t, s, "carga", fmt.Sprintf("carga-%04d", i))
	}
	t.Logf("enrolar %d dispositivos: %s", p.N, time.Since(desde).Round(10*time.Millisecond))
	contador.enCero()

	// Un transporte PROPIO: el DefaultClient (el que usa postCon) guarda dos conexiones ociosas
	// por host, así que con dos mil agentes churnearía una conexión por latido y el banco mediría
	// el handshake TCP en vez del cerebro. Y postCon además hace t.Fatalf, que desde una goroutine
	// que no es la del test es ilegal.
	//
	// MaxConnsPerHost acota los descriptores abiertos; cuando se llena, el que espera lo hace
	// DENTRO de su latencia medida — que es exactamente lo que le pasaría a un agente real detrás
	// de un proxy saturado, y por eso no se descuenta.
	cliente := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxConnsPerHost: p.Conexiones, MaxIdleConnsPerHost: p.Conexiones, MaxIdleConns: p.Conexiones,
			IdleConnTimeout: 90 * time.Second,
		},
	}
	defer cliente.CloseIdleConnections()

	med := &medicionDeCarga{porStatus: map[int]int{}, primeraNota: map[string]string{}}
	fin := make(chan struct{})
	var wg sync.WaitGroup
	url := ts.URL + fleetHeartbeatPath

	latir := func(i, k int) {
		conInventario := periodo > 0 && (k+i)%periodo == 0
		cuerpo, err := cuerpoDeLatidoDeCarga(i, conInventario)
		if err != nil {
			t.Errorf("armar el cuerpo del latido: %v", err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(cuerpo))
		if err != nil {
			t.Errorf("armar el request: %v", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+tokens[i])
		req.Header.Set("Content-Type", "application/json")
		t0 := time.Now()
		resp, err := cliente.Do(req)
		lat := time.Since(t0)
		if err != nil {
			med.anotar(lat, 0, err, respuestaLatido{}, conInventario)
			return
		}
		var r respuestaLatido
		_ = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		med.anotar(lat, resp.StatusCode, nil, r, conInventario)
	}

	inicio := time.Now()
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Arranque ESCALONADO a lo largo de un intervalo, como una flota real: dos mil agentes
			// no arrancan en el mismo milisegundo, y si arrancaran así el p99 mediría la ráfaga
			// del primer segundo y no el régimen — que es el número que el plan necesita.
			if p.Intervalo > 0 {
				desfase := p.Intervalo * time.Duration(i) / time.Duration(p.N)
				select {
				case <-time.After(desfase):
				case <-fin:
					return
				}
			}
			for k := 0; ; k++ {
				latir(i, k)
				if p.Intervalo == 0 {
					select {
					case <-fin:
						return
					default:
						continue
					}
				}
				select {
				case <-time.After(p.Intervalo):
				case <-fin:
					return
				}
			}
		}(i)
	}
	time.Sleep(p.Duracion)
	close(fin)
	wg.Wait()
	transcurrido := time.Since(inicio)

	med.mu.Lock()
	defer med.mu.Unlock()
	total := len(med.latencias)
	sort.Slice(med.latencias, func(a, b int) bool { return med.latencias[a] < med.latencias[b] })
	conError := total - med.ok
	t.Logf("latidos: %d ok · %d con error · %s latidos/s (en %s)",
		med.ok, conError, conComa(float64(total)/transcurrido.Seconds(), 1),
		transcurrido.Round(100*time.Millisecond))
	p99 := percentilDeCarga(med.latencias, 0.99)
	t.Logf("latencia: p50 %s · p95 %s · p99 %s · max %s",
		msDeCarga(percentilDeCarga(med.latencias, 0.50)), msDeCarga(percentilDeCarga(med.latencias, 0.95)),
		msDeCarga(p99), msDeCarga(percentilDeCarga(med.latencias, 1)))

	var errores []string
	for status, n := range med.porStatus {
		errores = append(errores, fmt.Sprintf("%d ×%d", status, n))
	}
	if med.transporte > 0 {
		errores = append(errores, fmt.Sprintf("transporte ×%d (%s)", med.transporte, med.primeraNota["transporte"]))
	}
	if med.muestraNoGuardada > 0 {
		errores = append(errores, fmt.Sprintf("muestra no guardada ×%d (%q)", med.muestraNoGuardada, med.primeraNota["muestra"]))
	}
	if med.inventarioNoGuardado > 0 {
		errores = append(errores, fmt.Sprintf("inventario no guardado ×%d (%q)", med.inventarioNoGuardado, med.primeraNota["inventario"]))
	}
	sort.Strings(errores)
	if len(errores) == 0 {
		t.Log("errores: ninguno")
	} else {
		t.Logf("errores: %s", strings.Join(errores, " · "))
	}
	reportarLlamadasAlAlmacen(t, contador, med.ok)

	if total == 0 {
		t.Fatal("no salió ni un latido: el banco no midió nada")
	}
	// EL ASSERT ES OPT-IN APARTE, y no viene con MUSUBI_CARGA. El banco se corre a mano en
	// máquinas de todo tipo —un portátil térmicamente limitado da un p99 que no dice nada del
	// cerebro— y un umbral que falla ahí enseña a ignorar el rojo. Con MUSUBI_CARGA_ASSERT=1 se
	// afirma, y ése es el modo para una máquina de referencia.
	if p.Assert && p99 >= 200*time.Millisecond {
		t.Fatalf("p99 = %s, el plan pide < 200 ms", msDeCarga(p99))
	}
}

// reportarLlamadasAlAlmacen imprime el desglose. `latidos` es cuántos latidos el cerebro CONTESTÓ
// con 200: dividir por los que fallaron mezclaría un 401 —que no escribe nada— con el trabajo
// real, y el promedio saldría bajo por el motivo equivocado.
func reportarLlamadasAlAlmacen(t *testing.T, c *almacenQueCuentaLlamadas, latidos int) {
	var escrituras, lecturas int64
	var desgloseEscrituras, desgloseLecturas []string
	for _, m := range c.contadas() {
		n := m.contador.Load()
		if m.escribe {
			escrituras += n
			desgloseEscrituras = append(desgloseEscrituras, fmt.Sprintf("%s %d", m.nombre, n))
			continue
		}
		lecturas += n
		desgloseLecturas = append(desgloseLecturas, fmt.Sprintf("%s %d", m.nombre, n))
	}
	if latidos == 0 {
		t.Log("llamadas al almacén por latido: no medido (ningún latido terminó en 200)")
		return
	}
	t.Logf("escrituras al almacén por latido: %s (%d en %d latidos ok)",
		conComa(float64(escrituras)/float64(latidos), 2), escrituras, latidos)
	t.Logf("  %s", strings.Join(desgloseEscrituras, " · "))
	t.Logf("lecturas al almacén por latido: %s — %s",
		conComa(float64(lecturas)/float64(latidos), 2), strings.Join(desgloseLecturas, " · "))
	t.Log("NO medido: sentencias SQL ni transacciones por latido (ver el encabezado de carga_test.go)")
}

// conComa formatea un decimal con coma. TODO número del reporte pasa por acá: un reporte que
// mezcla «p99 19,6 ms» con «3.21 escrituras» hace dudar de si el punto es un separador de miles,
// y son números que alguien va a copiar a una planilla.
func conComa(v float64, decimales int) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', decimales, 64), ".", ",", 1)
}

// msDeCarga imprime una duración en milisegundos con un decimal: «3,1 ms» se lee de un saque,
// «3.141592ms» hay que descifrarlo.
func msDeCarga(d time.Duration) string {
	return conComa(float64(d)/float64(time.Millisecond), 1) + " ms"
}
