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
//	                          intervalo/300, o sea 0,1: el agente real manda el inventario
//	                          completo cada fleet.InventarioCada = 5 min)
//	MUSUBI_CARGA_CONEXIONES   conexiones TCP simultáneas del lado cliente    (default 256)
//	MUSUBI_CARGA_ASSERT=1     además de reportar, FALLA si p99 >= 200 ms
//
// CÓMO LEER LA SALIDA
//
//	banco de carga · 2000 agentes · 60 s · latido cada 30 s · inventario en 1 de cada 10 · 256 conexiones
//	enrolar 2000 dispositivos: 1,8 s
//	latidos: 4000 ok · 0 con error · 66,7 latidos/s (en 60,0 s)
//	latencia: p50 2,9 ms · p95 8,1 ms · p99 19,6 ms · max 41,2 ms
//	errores: ninguno
//	escrituras por latido: no medido (ver el encabezado de carga_test.go)
//
// Con la cadencia real (INTERVALO=30) «latidos/s» es N/30 por construcción y no dice nada del
// cerebro: ahí lo que importa son la latencia y los errores. Para saber cuántos latidos por
// segundo AGUANTA el cerebro, correr con MUSUBI_CARGA_INTERVALO=0 — cada agente vuelve a latir
// apenas recibe la respuesta anterior— y leer «latidos/s» como el techo de esta máquina.
//
// La línea «errores» desglosa por causa: `503 ×12` es el cerebro devolviendo «registry
// unavailable» (la base no contestó a tiempo), `transporte ×3` es el cliente sin respuesta
// (timeout, conexión rechazada), `muestra no guardada ×N` es un 200 cuya nota dice que la
// telemetría se descartó, e `inventario no guardado ×N` lo mismo para los servicios. Esos dos
// últimos son el banco vigilándose a sí mismo: un 200 con la muestra descartada sería «el cerebro
// aguanta» por el motivo equivocado, porque no habría escrito nada.
//
// ESCRITURAS POR LATIDO: NO MEDIDO, Y POR QUÉ NO SE INVENTA EL NÚMERO
//
// Es el número que más querría saber quien planifique la base, y no hay forma limpia de medirlo
// desde acá: `DbEngine.db` no se exporta, y aunque se exportara, `total_changes()` de SQLite es
// POR CONEXIÓN y el pool tiene ocho — sumarlo daría un número plausible y falso. Se reporta «no
// medido» antes que un cero inventado. Lo que sí se puede decir LEYENDO el handler (no midiendo):
// un latido como el del agente real toca la fila del device hasta tres veces —
// ActualizarAutoreporte, FijarCapacidadDePreguntar, LatirDevice— y, cuando lleva inventario, un
// upsert por servicio más la poda. Convertir eso en una medición es trabajo de otra ola.
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
	"testing"
	"time"

	"musubi/internal/fleet"
)

// bancoDeCargaHabilitado es EL GATE, y es una función pura a propósito: la prueba de forma la
// recorre esquina por esquina sin tocar el entorno real. Las dos condiciones son AND: la variable
// pide correrlo, y -short lo veta aunque se lo pida.
func bancoDeCargaHabilitado(env func(string) string, short bool) bool {
	return env("MUSUBI_CARGA") == "1" && !short
}

// cuerpoDelBancoDeCarga es el cuerpo que TestBancoDeCarga corre después del gate. Es una variable
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
// sobre sus propias condiciones.
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
	// La fracción por default sale de la cadencia real: inventario completo cada InventarioCada,
	// un latido cada Intervalo. Con el techo (Intervalo 0) no hay cadencia de la que derivarla y
	// se conserva la de la cadencia real, para que el techo se mida con la misma mezcla.
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

// medicionDeCarga junta lo que pasó. Un mutex y no atómicos: la contención de dos mil goroutines
// sobre un lock de nanosegundos es despreciable frente a un round-trip HTTP, y un solo lock hace
// que el conteo y las latencias sean consistentes entre sí.
type medicionDeCarga struct {
	mu                   sync.Mutex
	latencias            []time.Duration
	ok                   int
	porStatus            map[int]int
	transporte           int
	muestraNoGuardada    int
	inventarioNoGuardado int
	// primeraNota guarda la primera nota de descarte de cada clase, para que el reporte diga POR
	// QUÉ se descartó y no sólo cuántas veces.
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

// percentil por rango más cercano sobre la muestra ordenada: p99 de 100 valores es el 99.º, no
// una interpolación entre el 99.º y el 100.º. Es el que menos suaviza la cola, que es justamente
// lo que se quiere ver.
func percentil(ordenadas []time.Duration, q float64) time.Duration {
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

// cuerpoDeLatidoDeCarga arma el cuerpo con la misma forma que `latir` en cmd/musubi/agent.go:
// versión, dirección, capacidad de preguntar, muestra y —cuando toca— el inventario. Se calca la
// forma y no se importa el agente porque `latir` es del paquete main y lee el host real.
func cuerpoDeLatidoDeCarga(i int, conInventario bool) ([]byte, error) {
	m := muestraDePrueba()
	// Cada agente con su propio perfil, para que dos mil filas no guarden el mismo JSON: una base
	// con el mismo blob repetido comprime y cachea distinto que una con dos mil distintos.
	cpu := float64((i * 37) % 100)
	m.CPUPct = &cpu
	m.MemUsada = uint64(1+i%6) << 30
	m.Tomada = time.Now().UTC()
	carga := map[string]any{
		"version":         "banco-de-carga",
		"direccion":       fmt.Sprintf("10.%d.%d.%d", (i>>16)&255, (i>>8)&255, i&255),
		"puede_preguntar": false,
		"muestra":         m,
	}
	if conInventario {
		reportes := make([]fleet.ReporteServicio, 60)
		for k := range reportes {
			pid, reinicios := 1000+k, 0
			reportes[k] = fleet.ReporteServicio{
				Nombre: fmt.Sprintf("svc-%02d.service", k), Clase: "systemd",
				Salud: fleet.SaludServicio{Tomada: m.Tomada, Estado: fleet.EstadoCorriendo, PID: &pid, Reinicios: &reinicios},
			}
		}
		carga["servicios"] = reportes
	}
	return json.Marshal(carga)
}

// correrBancoDeCarga es el cuerpo del banco: enrola, late, mide y reporta.
func correrBancoDeCarga(t *testing.T, p parametrosDeCarga) {
	t.Helper()
	// El inventario va en 1 de cada `periodo` latidos, desfasado por agente para que no caigan
	// todos juntos: con fracción 0,1 son 200 inventarios por vuelta repartidos, no 2000 en un
	// mismo segundo y ninguno en los nueve siguientes.
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
		inventario = fmt.Sprintf("en 1 de cada %d", periodo)
	}
	t.Logf("banco de carga · %d agentes · %s · latido %s · inventario %s · %d conexiones",
		p.N, p.Duracion, cadencia, inventario, p.Conexiones)

	s, ts, _, _ := servidorConFlota(t)

	// Se enrola por la tool real, una por una, como lo haría un admin con un guion: el costo de
	// enrolar también es un número del plan y se reporta.
	desde := time.Now()
	tokens := make([]string, p.N)
	for i := range tokens {
		tokens[i] = enrolarDePrueba(t, s, "carga", fmt.Sprintf("carga-%04d", i))
	}
	t.Logf("enrolar %d dispositivos: %s", p.N, time.Since(desde).Round(10*time.Millisecond))

	// Un transporte propio: el DefaultClient guarda dos conexiones ociosas por host y con dos mil
	// agentes churnearía una conexión por latido, midiendo el handshake TCP en vez del cerebro.
	// MaxConnsPerHost acota los descriptores abiertos; cuando se llena, el que espera lo hace
	// DENTRO de su latencia medida — que es lo que le pasaría a un agente real detrás de un
	// proxy saturado, y por eso no se descuenta.
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
			// Arranque escalonado a lo largo de un intervalo, como una flota real: dos mil
			// agentes no arrancan en el mismo milisegundo, y si arrancaran así el p99 mediría
			// la ráfaga del primer segundo y no el régimen.
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
	t.Logf("latidos: %d ok · %d con error · %.1f latidos/s (en %s)",
		med.ok, conError, float64(total)/transcurrido.Seconds(), transcurrido.Round(100*time.Millisecond))
	p99 := percentil(med.latencias, 0.99)
	t.Logf("latencia: p50 %s · p95 %s · p99 %s · max %s",
		ms(percentil(med.latencias, 0.50)), ms(percentil(med.latencias, 0.95)), ms(p99), ms(percentil(med.latencias, 1)))

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
	t.Log("escrituras por latido: no medido (ver el encabezado de carga_test.go)")

	if total == 0 {
		t.Fatal("no salió ni un latido: el banco no midió nada")
	}
	if p.Assert && p99 >= 200*time.Millisecond {
		t.Fatalf("p99 = %s, el plan pide < 200 ms", ms(p99))
	}
}

// ms imprime una duración en milisegundos con un decimal: «3,1 ms» se lee, «3.141592ms» no.
func ms(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
}
