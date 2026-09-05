//go:build linux

package fleet

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// D1 — LA PRIMERA MUESTRA NO TIENE PORCENTAJE DE CPU.
//
// /proc/stat da jiffies acumulados, no un porcentaje: sin lectura anterior no hay derivada. Un
// 0.0 inventado sería indistinguible de una máquina ociosa, justo cuando alguien mira el panel
// para entender una caída.
//
// Sabotaje que la hace fallar: inicializar `tienePrevio: true` con ceros, o devolver 0 cuando no
// hay previo.
func TestLaPrimeraMuestraNoInventaUnPorcentajeDeCPU(t *testing.T) {
	c := NuevoColector()
	m1, err := c.Tomar()
	if err != nil {
		t.Fatalf("Tomar: %v", err)
	}
	if m1.CPUPct != nil {
		t.Fatalf("la primera muestra reportó cpu_pct=%v: no hay contra qué restar, tiene que ser nil", *m1.CPUPct)
	}

	// La SEGUNDA sí, porque ya hay delta.
	quemarCPU(80 * time.Millisecond)
	m2, err := c.Tomar()
	if err != nil {
		t.Fatal(err)
	}
	if m2.CPUPct == nil {
		t.Fatal("la segunda muestra no trajo cpu_pct: el delta no se está calculando")
	}
	if *m2.CPUPct < 0 || *m2.CPUPct > 100 {
		t.Errorf("cpu_pct fuera de rango: %v", *m2.CPUPct)
	}
}

// El colector mide de verdad: los números coinciden con lo que dice /proc, no son constantes.
// Sabotaje: devolver una Muestra vacía → todos los contrastes fallan.
func TestElColectorMidePorDeVerdadYNoDevuelveCeros(t *testing.T) {
	m, err := NuevoColector().Tomar()
	if err != nil {
		t.Fatalf("Tomar: %v", err)
	}

	// Memoria contra /proc/meminfo.
	memTotalKB := leerUnaClaveDeMeminfo(t, "MemTotal")
	if m.MemTotal != memTotalKB*1024 {
		t.Errorf("MemTotal = %d, /proc/meminfo dice %d bytes", m.MemTotal, memTotalKB*1024)
	}
	if m.MemUsada == 0 || m.MemUsada >= m.MemTotal {
		t.Errorf("MemUsada = %d con total %d: no parece medido", m.MemUsada, m.MemTotal)
	}

	// I8 — MemLibre se mide DE VERDAD, y se contrasta POR CERCANÍA contra /proc/meminfo.
	//
	// Por cercanía y no por igualdad, por la misma razón que la prueba de MemAvailable de más
	// abajo: entre las dos lecturas la memoria se mueve, y comparar al byte dos lecturas
	// independientes de una cantidad que cambia es flaky por construcción.
	//
	// Sabotaje: dejar de asignar MemFree en ParsearMeminfo — MemLibre queda nil y la primera rama
	// lo dice.
	libreKB := leerUnaClaveDeMeminfo(t, "MemFree")
	if m.MemLibre == nil {
		t.Error("MemLibre = nil en Linux: /proc/meminfo SIEMPRE trae MemFree, así que esto es que no se parsea")
	} else if dif := relativa(*m.MemLibre, libreKB*1024); dif > 0.25 {
		t.Errorf("MemLibre = %d y /proc/meminfo dice %d (%.1f%% de diferencia): la RAM libre se mueve "+
			"entre dos lecturas, pero no tanto — esto ya no es ruido", *m.MemLibre, libreKB*1024, dif*100)
	}

	// I8 — Y los procesos, contra un conteo propio de /proc.
	//
	// Tolerancia amplia (±25 %) A PROPÓSITO: los procesos entran y salen entre las dos lecturas, y
	// la suite entera arranca y mata subprocesos. La aserción tiene que discriminar a la escala en
	// la que vive el bug —confundir procesos con HILOS da 3 a 5 veces más—, no más fina.
	//
	// Sabotaje: dejar listarProc() devolviendo "" (NumProcesos queda en 0 y la primera rama lo dice).
	propios := contarPidsDeProcAMano(t)
	if m.NumProcesos == 0 {
		t.Error("NumProcesos = 0 en Linux: /proc siempre tiene pids, así que esto es que no se listó")
	} else if dif := relativa(uint64(m.NumProcesos), uint64(propios)); dif > 0.25 {
		t.Errorf("NumProcesos = %d y /proc tiene %d entradas numéricas (%.1f%% de diferencia): "+
			"si el número es varias veces mayor, se están contando HILOS y no procesos",
			m.NumProcesos, propios, dif*100)
	}

	// Disco: el root siempre tiene tamaño.
	if m.DiscoTotal == 0 {
		t.Error("DiscoTotal = 0: statfs no midió el filesystem raíz")
	}
	if m.DiscoUsado == 0 || m.DiscoUsado > m.DiscoTotal {
		t.Errorf("DiscoUsado = %d con total %d", m.DiscoUsado, m.DiscoTotal)
	}

	// Uptime: una máquina que corre un test lleva al menos unos segundos encendida.
	if m.UptimeSeg < 1 {
		t.Errorf("UptimeSeg = %d", m.UptimeSeg)
	}
	if m.NumCPU < 1 {
		t.Errorf("NumCPU = %d", m.NumCPU)
	}
	// En Linux la carga SIEMPRE se puede leer: nil acá significa que /proc/loadavg no se parseó.
	if m.Load1 == nil || m.Load5 == nil || m.Load15 == nil {
		t.Errorf("carga no leída en Linux: %v %v %v", m.Load1, m.Load5, m.Load15)
	} else if *m.Load1 < 0 || *m.Load5 < 0 || *m.Load15 < 0 {
		t.Errorf("carga negativa: %v %v %v", *m.Load1, *m.Load5, *m.Load15)
	}
	// Lo medido tiene que pasar su propia validación.
	if err := m.Valida(); err != nil {
		t.Errorf("una muestra REAL no pasó Valida(): %v", err)
	}
	t.Logf("medido: cpu=%s num_cpu=%d procs=%d mem=%.1f%% disco=%.1f%% load1=%s uptime=%ds temp=%s",
		fmtPtr(m.CPUPct), m.NumCPU, m.NumProcesos,
		*PctUsado(m.MemUsada, m.MemTotal), *PctUsado(m.DiscoUsado, m.DiscoTotal),
		fmtPtr(m.Load1), m.UptimeSeg, fmtPtr(m.TempC))
}

// «Usada» se calcula con MemAvailable, no con MemTotal-MemFree: un Linux sano usa toda la RAM
// libre como page cache, y con MemFree aparecería al ~95 % permanentemente hasta que nadie
// vuelva a mirar la métrica.
//
// Sabotaje que la hace fallar: usar MemFree en leerMemoria.
func TestLaMemoriaUsadaSaleDeMemAvailableYNoDeMemFree(t *testing.T) {
	total := leerUnaClaveDeMeminfo(t, "MemTotal")
	disponible := leerUnaClaveDeMeminfo(t, "MemAvailable")
	libre := leerUnaClaveDeMeminfo(t, "MemFree")
	if disponible <= libre {
		t.Skip("en esta máquina MemAvailable no supera a MemFree: el test no distingue")
	}

	m, err := NuevoColector().Tomar()
	if err != nil {
		t.Fatal(err)
	}
	conAvailable := (total - disponible) * 1024
	conFree := (total - libre) * 1024

	// SE COMPARA POR CERCANÍA, NO POR IGUALDAD, y la razón es que la primera versión de esta
	// prueba era FLAKY: exigía que `m.MemUsada` fuera EXACTAMENTE igual a un número derivado de
	// leer /proc/meminfo por segunda vez. Entre las dos lecturas la memoria se mueve —bajo la
	// carga de la suite completa se movía ~1,4 MB— y el test fallaba cuando el código estaba bien.
	//
	// Comparar dos lecturas independientes de una cantidad que cambia, al byte, es flaky por
	// construcción. Y no hace falta: el bug que esta prueba busca —usar MemFree en vez de
	// MemAvailable— se manifiesta a escala de GIGABYTES (medido acá: 3,87 GB contra 7,38 GB).
	// La aserción tiene que discriminar a la escala en la que el bug vive, no más fina.
	distancia := func(a, b uint64) uint64 {
		if a > b {
			return a - b
		}
		return b - a
	}
	dAvailable, dFree := distancia(m.MemUsada, conAvailable), distancia(m.MemUsada, conFree)
	if dAvailable >= dFree {
		t.Errorf("MemUsada = %d está más cerca del cálculo con MemFree (%d) que del de MemAvailable (%d): "+
			"se está usando el número equivocado, y con MemFree un Linux sano aparece al ~95%% para siempre",
			m.MemUsada, conFree, conAvailable)
	}
	// Y la diferencia con el valor esperado tiene que ser RUIDO, no otra fórmula: 1 % del total.
	if margen := total * 1024 / 100; dAvailable > margen {
		t.Errorf("MemUsada = %d se aparta %d bytes de lo esperado (%d), más que el 1%% de la RAM: "+
			"eso ya no es que la memoria se movió entre lecturas", m.MemUsada, dAvailable, conAvailable)
	}
}

// El disco usado se calcula con Bavail (lo que puede usar una aplicación), no con Bfree, que
// incluye la reserva de root. Es el mismo número que muestra `df`.
func TestElDiscoUsadoCoincideConDf(t *testing.T) {
	m, err := NuevoColector().Tomar()
	if err != nil {
		t.Fatal(err)
	}
	// df redondea a bloques; se admite 1 % de diferencia.
	total, usado, disponible := leerDf(t)
	if total == 0 {
		t.Skip("no se pudo leer df")
	}
	for _, c := range []struct {
		campo       string
		got, quiero uint64
	}{
		{"DiscoTotal", m.DiscoTotal, total},
		{"DiscoUsado", m.DiscoUsado, usado},
		{"DiscoDisponible", m.DiscoDisponible, disponible},
	} {
		if dif := relativa(c.got, c.quiero); dif > 0.01 {
			t.Errorf("%s = %d, df dice %d (%.1f%% de diferencia)", c.campo, c.got, c.quiero, dif*100)
		}
	}

	// Y la razón de que sean TRES: la reserva de root hace que no cierren en dos.
	// Sabotaje que lo hace fallar: calcular Disponible como Total-Usado.
	if m.DiscoUsado+m.DiscoDisponible >= m.DiscoTotal {
		t.Errorf("usado (%d) + disponible (%d) >= total (%d): la reserva de root desapareció, "+
			"así que uno de los dos números está derivado del otro en vez de medido",
			m.DiscoUsado, m.DiscoDisponible, m.DiscoTotal)
	}
}

// D3 — sin sensor no hay temperatura, y eso viaja como nil.
//
// Ahora pasa por ParsearTempMiligrados, que es el mismo camino del colector remoto: una lectura
// vacía (archivo ausente, o `cat` que falló del otro lado de un ssh) tiene que dar nil.
func TestLaTemperaturaEsNilSinSensor(t *testing.T) {
	crudo := leerArchivo("/sys/class/thermal/thermal_zone0/temp")
	got := ParsearTempMiligrados(crudo)
	if crudo == "" {
		if got != nil {
			t.Errorf("no hay thermal_zone0 pero se reportó %v", *got)
		}
		return
	}
	if got != nil && (*got <= 0 || *got > 150) {
		t.Errorf("temperatura implausible: %v", *got)
	}
}

// El colector guarda estado entre llamadas, así que dos colectores distintos no se pisan.
func TestDosColectoresSonIndependientes(t *testing.T) {
	a, b := NuevoColector(), NuevoColector()
	if _, err := a.Tomar(); err != nil {
		t.Fatal(err)
	}
	quemarCPU(50 * time.Millisecond)
	ma, _ := a.Tomar()
	mb, _ := b.Tomar() // primera vez para `b`
	if ma.CPUPct == nil {
		t.Error("el colector `a` ya tenía previo y debería dar porcentaje")
	}
	if mb.CPUPct != nil {
		t.Error("el colector `b` es nuevo: no debería tener porcentaje todavía")
	}
}

// ── ayudas ──────────────────────────────────────────────────────────────────────────────────

func leerUnaClaveDeMeminfo(t *testing.T, clave string) uint64 {
	t.Helper()
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		t.Skipf("sin /proc/meminfo: %v", err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		campos := strings.Fields(l)
		if len(campos) >= 2 && strings.TrimSuffix(campos[0], ":") == clave {
			v, err := strconv.ParseUint(campos[1], 10, 64)
			if err != nil {
				t.Fatalf("%s ilegible: %v", clave, err)
			}
			return v
		}
	}
	t.Skipf("/proc/meminfo no tiene %s", clave)
	return 0
}

// contarPidsDeProcAMano cuenta las entradas numéricas de /proc sin pasar por el código bajo
// prueba: es la referencia contra la que se contrasta el colector, igual que `df` lo es para el
// disco. Si usara ContarPids, la prueba se compararía contra sí misma.
func contarPidsDeProcAMano(t *testing.T) int {
	t.Helper()
	entradas, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("sin /proc: %v", err)
	}
	n := 0
	for _, e := range entradas {
		if _, err := strconv.ParseUint(e.Name(), 10, 64); err == nil {
			n++
		}
	}
	return n
}

func relativa(a, b uint64) float64 {
	if b == 0 {
		return 1
	}
	d := float64(a) - float64(b)
	if d < 0 {
		d = -d
	}
	return d / float64(b)
}

// leerDf pregunta al `df` del sistema, que es la referencia contra la que se contrasta statfs.
func leerDf(t *testing.T) (total, usado, disponible uint64) {
	t.Helper()
	out, err := exec.Command("df", "-B1", "--output=size,used,avail", "/").Output()
	if err != nil {
		t.Skipf("sin df: %v", err)
	}
	lineas := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lineas) < 2 {
		t.Skip("df no devolvió datos")
	}
	campos := strings.Fields(lineas[len(lineas)-1])
	if len(campos) < 3 {
		t.Skip("df con formato inesperado")
	}
	total, _ = strconv.ParseUint(campos[0], 10, 64)
	usado, _ = strconv.ParseUint(campos[1], 10, 64)
	disponible, _ = strconv.ParseUint(campos[2], 10, 64)
	return total, usado, disponible
}

// fmtPtr imprime un *float64 legible (nil o el valor), no la dirección.
func fmtPtr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return strconv.FormatFloat(*p, 'f', 1, 64)
}
