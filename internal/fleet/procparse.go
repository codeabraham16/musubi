package fleet

// procparse.go parsea /proc y /sys A PARTIR DE SU CONTENIDO, no leyendo archivos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTA SEPARACIÓN VALE LA PENA
//
// /proc no es sólo del host local. Un dispositivo de TIER B (un NAS, un Raspberry Pi, un server
// sin agente) tiene el mismo /proc del otro lado de un `ssh cat`. Y un Android — que ES Linux —
// tiene el mismo /proc del otro lado de un `adb shell cat`.
//
// Con el parseo separado de la lectura, LAS TRES FUENTES COMPARTEN EL MISMO CÓDIGO: el colector
// local abre archivos, el remoto ejecuta un comando, y los dos entregan el mismo texto a las
// mismas funciones. Sin la separación habría tres parseos de /proc/meminfo que se irían
// desincronizando —y el bug de MemFree contra MemAvailable habría que arreglarlo tres veces.
//
// Está SIN build tag y con prueba, como cpudelta.go y sysctlparse.go: es la única forma de que
// el camino de Android, que no se puede correr desde acá, tenga su parte difícil cubierta.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"strconv"
	"strings"
	"time"
)

// LecturasProc son los textos crudos que hacen falta para armar una Muestra. Los campos vacíos
// simplemente no aportan nada: una fuente que no pudo leer /proc/loadavg deja la carga en nil, no
// invalida el resto de la muestra.
type LecturasProc struct {
	Stat    string // /proc/stat
	Meminfo string // /proc/meminfo
	Loadavg string // /proc/loadavg
	Uptime  string // /proc/uptime
	Df      string // salida de `df -B1 /` (o vacío)
	TempMil string // /sys/class/thermal/thermal_zone0/temp, en miligrados
	NumCPU  int    // cuántos procesadores; 0 = desconocido
}

// MuestraDesde arma una Muestra a partir de los textos. `cpu` lleva el estado entre llamadas para
// poder derivar el porcentaje; si es nil, la muestra sale sin CPU (que es lo honesto).
func MuestraDesde(l LecturasProc, cpu *contadorCPU) Muestra {
	m := Muestra{Tomada: time.Now().UTC(), NumCPU: l.NumCPU}

	if cpu != nil {
		if ocupado, total, ok := ParsearJiffies(l.Stat); ok {
			m.CPUPct = cpu.delta(ocupado, total)
		}
	}
	ParsearMeminfo(l.Meminfo, &m)
	ParsearLoadavg(l.Loadavg, &m)
	m.UptimeSeg = ParsearUptime(l.Uptime)
	ParsearDf(l.Df, &m)
	m.TempC = ParsearTempMiligrados(l.TempMil)
	return m
}

// ParsearJiffies devuelve (ocupado, total) de la primera línea de /proc/stat.
//
// `idle` (índice 3) e `iowait` (4) son el tiempo NO ocupado. iowait cuenta como ocioso a
// propósito: una máquina esperando disco no está usando CPU, y contarla como ocupada haría que un
// backup nocturno parezca saturación.
func ParsearJiffies(texto string) (ocupado, total uint64, ok bool) {
	for _, linea := range strings.Split(texto, "\n") {
		campos := strings.Fields(linea)
		if len(campos) < 5 || campos[0] != "cpu" {
			continue
		}
		var ocioso uint64
		for i, s := range campos[1:] {
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				continue
			}
			total += v
			if i == 3 || i == 4 { // idle, iowait
				ocioso += v
			}
		}
		if total == 0 {
			return 0, 0, false
		}
		return total - ocioso, total, true
	}
	return 0, 0, false
}

// ParsearMeminfo llena RAM y swap.
//
// «Usada» sale de MemAvailable, no de MemTotal-MemFree: MemFree no cuenta el page cache, así que
// un Linux sano —que usa toda la RAM libre como caché— aparecería al 95 % permanentemente y nadie
// volvería a mirar la métrica. MemAvailable es la estimación del kernel de cuánto se puede pedir
// sin swapear, que es la pregunta real.
//
// Se respeta la REGLA DE LOS PARES: total y usado se fijan juntos o no se fija ninguno.
func ParsearMeminfo(texto string, m *Muestra) {
	vals := make(map[string]uint64, 5)
	for _, linea := range strings.Split(texto, "\n") {
		campos := strings.Fields(linea)
		if len(campos) < 2 {
			continue
		}
		clave := strings.TrimSuffix(campos[0], ":")
		switch clave {
		case "MemTotal", "MemAvailable", "MemFree", "SwapTotal", "SwapFree":
			if v, err := strconv.ParseUint(campos[1], 10, 64); err == nil {
				vals[clave] = v * 1024 // /proc/meminfo habla en kB; adentro todo es bytes
			}
		}
	}
	if total := vals["MemTotal"]; total > 0 {
		disponible, hay := vals["MemAvailable"]
		if !hay {
			disponible = vals["MemFree"] // kernels muy viejos no exponen MemAvailable
		}
		if total >= disponible && disponible > 0 {
			m.MemTotal, m.MemUsada = total, total-disponible
		}
	}
	if total, hay := vals["SwapTotal"]; hay && total > 0 {
		if libre, hayLibre := vals["SwapFree"]; hayLibre && total >= libre {
			m.SwapTotal, m.SwapUsada = total, total-libre
		}
	}
}

// ParsearLoadavg llena las tres cargas. Cada una por separado: si una no parsea, las otras siguen
// valiendo y la que falló queda nil (no medida) en vez de cero (medida y ociosa).
func ParsearLoadavg(texto string, m *Muestra) {
	campos := strings.Fields(texto)
	if len(campos) < 3 {
		return
	}
	for i, dst := range []**float64{&m.Load1, &m.Load5, &m.Load15} {
		if v, err := strconv.ParseFloat(campos[i], 64); err == nil && v >= 0 {
			*dst = f64(v)
		}
	}
}

// ParsearUptime devuelve los segundos desde el arranque.
func ParsearUptime(texto string) uint64 {
	campos := strings.Fields(texto)
	if len(campos) < 1 {
		return 0
	}
	seg, err := strconv.ParseFloat(campos[0], 64)
	if err != nil || seg < 0 {
		return 0
	}
	return uint64(seg)
}

// ParsearDf lee la salida de `df -B1 /`, que trae las MISMAS tres columnas que statfs entrega
// localmente: tamaño, usado y disponible.
//
// Son tres y no dos porque Usado + Disponible ≠ Total: el kernel reserva ~5 % del filesystem para
// root. Medido en la máquina de desarrollo: 25,6 GB sobre 502 GB.
func ParsearDf(texto string, m *Muestra) {
	lineas := strings.Split(strings.TrimSpace(texto), "\n")
	if len(lineas) < 2 {
		return
	}
	// La última línea es la del filesystem. `df` puede partir el nombre del dispositivo en dos
	// líneas cuando es largo, así que se toman los ÚLTIMOS campos numéricos y no posiciones fijas.
	campos := strings.Fields(lineas[len(lineas)-1])
	if len(campos) < 4 {
		return
	}
	// Formato: Filesystem 1B-blocks Used Available Use% Mounted
	num := func(i int) (uint64, bool) {
		v, err := strconv.ParseUint(campos[i], 10, 64)
		return v, err == nil
	}
	// Se busca la primera posición desde la que hay tres números seguidos.
	for i := 0; i+2 < len(campos); i++ {
		total, ok1 := num(i)
		usado, ok2 := num(i + 1)
		disp, ok3 := num(i + 2)
		if ok1 && ok2 && ok3 && total > 0 && usado <= total && disp <= total {
			m.DiscoTotal, m.DiscoUsado, m.DiscoDisponible = total, usado, disp
			return
		}
	}
}

// ParsearTempMiligrados convierte el contenido de una zona térmica. Devuelve nil si no hay sensor
// o si el valor es implausible: una lectura de 0 grados es casi siempre un sensor que no está
// midiendo, no una máquina congelada.
func ParsearTempMiligrados(texto string) *float64 {
	mili, err := strconv.ParseFloat(strings.TrimSpace(texto), 64)
	if err != nil {
		return nil
	}
	c := mili / 1000
	if c <= 0 || c > 150 {
		return nil
	}
	return &c
}
