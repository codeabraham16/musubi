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
	// TempMil lleva UNA LÍNEA POR ZONA TÉRMICA, `<type> <miligrados>`, y no el contenido de
	// una sola. Quién de todas contesta la pregunta lo decide ElegirTemperatura, que es
	// compartido: leer `thermal_zone0` fijo medía `acpitz` —estático— en vez del paquete de
	// CPU. Se sigue aceptando un número suelto sin tipo: es el formato viejo.
	TempMil string
	// Procs es el LISTADO de /proc (un nombre por línea), no un archivo: local sale de
	// os.ReadDir, remoto de `ls -1 /proc`. Se comparte como TEXTO —y no ya contado— para que el
	// filtro difícil, «el nombre es todo dígitos», esté escrito UNA vez y lo usen las tres
	// fuentes; dos de las tres no se pueden probar desde esta máquina.
	Procs  string
	NumCPU int // cuántos procesadores; 0 = desconocido
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
	m.TempC = ElegirTemperatura(l.TempMil)
	m.NumProcesos = ContarPids(l.Procs)
	return m
}

// ContarPids cuenta los procesos de un listado de /proc.
//
// El filtro es «el nombre es todo dígitos»: ésos son los tgid, o sea los procesos. Todo lo demás
// que vive en /proc —`self`, `thread-self`, `cpuinfo`, `net`— no lo es.
//
// Y LO QUE NO SE USA, que es la parte importante: el 4º campo de /proc/loadavg («5/1181») ya está
// leído y a mano, pero ese 1181 son los HILOS del sistema, no los procesos. Da entre 3 y 5 veces
// más y no falla nunca: produce un número plausible y equivocado. Los hilos viven en
// /proc/<pid>/task y por eso no aparecen en este conteo.
func ContarPids(listado string) int {
	n := 0
	for _, linea := range strings.Split(listado, "\n") {
		nombre := strings.TrimSpace(linea)
		if nombre == "" {
			continue
		}
		todoDigitos := true
		for _, r := range nombre {
			if r < '0' || r > '9' {
				todoDigitos = false
				break
			}
		}
		if todoDigitos {
			n++
		}
	}
	return n
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
	// MemFree se guarda TAL CUAL, y no se toca la cuenta de arriba: MemUsada SIGUE saliendo de
	// MemAvailable. Tener las dos en la struct vuelve tentador el atajo —«libre es lo contrario de
	// usada»— y con el fixture real esa confusión son 3,5 GB: 85 % contra 40 %.
	//
	// Se absorbe acá, en el parseo que ya está, y no en una función nueva: duplicar el parseo de
	// meminfo es exactamente lo que el encabezado de este archivo existe para evitar.
	if libre, hay := vals["MemFree"]; hay {
		m.MemLibre = u64(libre)
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

// ── Qué zona térmica se mira ─────────────────────────────────────────────────────────────────
//
// LEER `thermal_zone0` FIJO MIDE OTRA COSA, Y MIDE ALGO PLAUSIBLE — que es lo que lo hace peligroso.
//
// Medido en `musubi-server` el 2026-09-05, con tres zonas presentes:
//
//	thermal_zone0  acpitz          27,8 °C   ← lo que se leía
//	thermal_zone1  pch_cannonlake  51,0 °C
//	thermal_zone2  x86_pkg_temp    41,0 °C   ← el paquete de CPU, el que contesta la pregunta
//
// El síntoma que lo destapó no fue un error sino una QUIETUD: 91 puntos en tres horas con el
// valor idéntico hasta la décima, en una máquina corriendo el cerebro, Prometheus y contenedores.
// Un sensor real jitterea. `acpitz` es, en muchos equipos, un valor fijo del firmware o una
// lectura de chasis — tiene las unidades correctas, el rango plausible y la forma de un dato, y
// responde a otra pregunta. La serie existía, no alertaba nunca, y nadie podía saber por qué.
//
// Importa porque A79 dice que la térmica es la ÚNICA causa de los apagones de una máquina que la
// flota podría ver sola. Una serie congelada no la ve.
//
// EL ORDEN ES POR `type` Y NO POR NÚMERO DE ZONA: el índice no significa nada —depende del orden
// en que el kernel registró los drivers— y es exactamente lo que hizo que `zone0` pareciera «la
// principal». `acpitz` va ÚLTIMO y no se descarta: en una máquina donde es lo único que hay, una
// lectura de chasis es mejor que ninguna, y el fallback deja eso dicho en vez de devolver nil.
var preferenciaDeZonaTermica = []string{
	"x86_pkg_temp", // Intel: temperatura del paquete. La respuesta canónica a «¿está caliente?»
	"coretemp",     // por núcleo, misma familia
	"k10temp",      // AMD
	"zenpower",     // AMD, driver alternativo
	"cpu_thermal",  // ARM / Raspberry Pi
	"cpu-thermal",  // la misma, con guion: los dos existen según kernel
	"soc_thermal",  // SoC embebidos
}

// ElegirTemperatura elige la zona térmica que contesta «¿se está calentando esta máquina?».
//
// Recibe una línea por zona, `<type> <miligrados>`, y devuelve la primera que exista según
// `preferenciaDeZonaTermica`. Si ninguna preferida está, devuelve **la más alta** de las que no
// son `acpitz`, y `acpitz` sólo si es lo único que quedó. Tolera además el formato VIEJO —un
// número suelto sin tipo— porque es lo que devuelve una máquina cuyo agente todavía no se
// actualizó.
//
// EL FALLBACK TOMA LA MÁS ALTA, NO LA PRIMERA, Y ESO NO ES UN DETALLE. Tomar la primera dejaba
// al arreglo dependiendo del orden de `os.ReadDir` —un orden que no significa nada, exactamente
// el mismo error que hacía que `thermal_zone0` pareciera «la principal»—, y con eso alcanzaba
// para que un sensor roto ganara. Medido el 2026-09-05 en la workstation de desarrollo, que
// tiene diez zonas y NINGUNA preferida entre las primeras:
//
//	acpitz 27800 · INT3400 Thermal 20000 · SEN1 41050 · SEN2 50 ← · SEN3 46050
//	SEN4 50 ← · SEN5 50 ← · TCPU 46050 · TCPU_PCI 46000 · x86_pkg_temp 45000
//
// Los tres `SEN` marcados dan 0,05 °C: son sensores que no están midiendo. Con «la primera que
// no sea acpitz» y sin zona preferida, ElegirTemperatura devolvía 0,05 °C — un número con las
// unidades correctas, el rango sintácticamente válido y la forma de un dato, que le ganaba
// incluso a la lectura de chasis. Es el MISMO defecto que este archivo vino a arreglar, un paso
// más adentro. Con «la más alta» ese sensor no puede ganarle a ninguna lectura real.
//
// LA DIRECCIÓN DEL ERROR ES DELIBERADA: entre equivocarse alto y equivocarse bajo, alto se
// investiga y bajo no se ve. Una serie que subestima no dispara `> 85` nunca y en un panel se
// dibuja como una máquina fresca, no como un hueco. El techo de plausibilidad de
// ParsearTempMiligrados (150 °C) es lo que impide que esa elección se vuelva una falsa alarma.
func ElegirTemperatura(texto string) *float64 {
	type zona struct {
		tipo string
		val  *float64
	}
	var zonas []zona
	for _, linea := range strings.Split(texto, "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		campos := strings.Fields(linea)
		if len(campos) == 1 {
			// Formato viejo: un número suelto, sin tipo. Se acepta tal cual.
			if v := ParsearTempMiligrados(campos[0]); v != nil {
				zonas = append(zonas, zona{tipo: "", val: v})
			}
			continue
		}
		// El TIPO es todo lo que no es el último campo: `INT3400 Thermal` existe y tiene un
		// espacio (medido en la workstation), y quedarse con `campos[0]` lo truncaba a
		// `int3400`. Hoy sería inocuo —ningún tipo preferido lleva espacio— pero es una trampa
		// puesta para el día que alguien agregue uno.
		if v := ParsearTempMiligrados(campos[len(campos)-1]); v != nil {
			tipo := strings.ToLower(strings.Join(campos[:len(campos)-1], " "))
			zonas = append(zonas, zona{tipo: tipo, val: v})
		}
	}
	if len(zonas) == 0 {
		return nil
	}
	for _, preferida := range preferenciaDeZonaTermica {
		for _, z := range zonas {
			if z.tipo == preferida {
				return z.val
			}
		}
	}
	// Ninguna preferida: la MÁS ALTA que no sea `acpitz` (ver el doc de arriba: la primera
	// dependía del orden de ReadDir y un sensor apagado le ganaba a una lectura real).
	var mejor *float64
	for _, z := range zonas {
		if z.tipo == "acpitz" {
			continue
		}
		if mejor == nil || *z.val > *mejor {
			mejor = z.val
		}
	}
	if mejor != nil {
		return mejor
	}
	// Sólo quedó `acpitz`: una lectura de chasis es mejor que ninguna, y se devuelve la más
	// alta también acá para no reintroducir la dependencia del orden por la puerta de atrás.
	for _, z := range zonas {
		if mejor == nil || *z.val > *mejor {
			mejor = z.val
		}
	}
	return mejor
}

// ParsearTempMiligrados convierte el contenido de una zona térmica. Devuelve nil si no hay sensor
// o si el valor es implausible.
//
// EL PISO NO ES 0, Y POR QUÉ: el corte era `c <= 0`, y los sensores muertos de esta workstation
// no reportan 0 sino `50` miligrados —0,05 °C—, así que pasaban el filtro entero y competían
// como si midieran. Un `type` cualquiera de /sys/class/thermal en una máquina ENCENDIDA no baja
// de esto: silicio que disipa está por encima del ambiente, y el ambiente de una máquina que
// arranca no es glaciar. Lo que se rechaza acá no es «hace frío», es «este archivo existe pero
// nadie lo está escribiendo».
//
// ES UNA HEURÍSTICA Y NO UNA GARANTÍA, dicho para que nadie confíe de más: un sensor clavado en
// un valor plausible —27,85 °C es el caso real, ver A2 en las dos Windows— pasa por acá sin que
// se lo pueda distinguir de una lectura buena. Eso NO se detecta con un sample; se detecta con
// varianza a lo largo del tiempo, que es una pregunta para Prometheus y no para un parser.
// TempMinPlausibleC y TempMaxPlausibleC son la banda, y viven acá porque las usan LOS DOS
// parsers —éste y ParsearTempDecikelvin de Windows—. Un segundo par de constantes con el mismo
// número es el defecto de este repo sembrado a mano: N lugares que deberían decir lo mismo.
const (
	TempMinPlausibleC = 5
	TempMaxPlausibleC = 150
)

func ParsearTempMiligrados(texto string) *float64 {
	mili, err := strconv.ParseFloat(strings.TrimSpace(texto), 64)
	if err != nil {
		return nil
	}
	c := mili / 1000
	if c < TempMinPlausibleC || c > TempMaxPlausibleC {
		return nil
	}
	return &c
}
