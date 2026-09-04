//go:build windows

package fleet

// colector_windows.go lee el estado del host llamando a kernel32.dll.
//
// STDLIB PURA, sin cgo y sin dependencias: `syscall.NewLazyDLL` resuelve los símbolos en tiempo
// de ejecución. Es el mismo criterio que el colector de Linux — `gopsutil` daría los tres
// sistemas de una y sería la 7ª dependencia directa de un repo que tiene 6.
//
// LO QUE WINDOWS NO TIENE, y viaja como nil en vez de como cero:
//
//   - LOAD AVERAGE. No es que valga cero: el concepto no existe fuera de UNIX. Un 0.00 en cada
//     máquina Windows de la flota sería indistinguible de una máquina ociosa.
//   - TEMPERATURA. Se puede sacar por WMI (MSAcpi_ThermalZoneTemperature), pero WMI desde Go sin
//     dependencias es COM crudo, y muchos equipos no exponen el sensor igual. Queda anotado.
//   - MEMORIA LIBRE (mem_libre). Y ésta es la que hay que leer despacio, porque el atajo está a
//     un campo de distancia: MEMORYSTATUSEX.ullAvailPhys es el análogo de MemAvailable, NO de
//     MemFree — incluye la standby list, que es cache reutilizable. Reportarlo como mem_libre
//     sería cometer en Windows exactamente la confusión que el repo peleó en Linux (85 % contra
//     40 % de RAM usada), esta vez en un archivo que casi nadie puede probar localmente. Se
//     prefiere no medirla: nil, y la tabla de capacidades de colector_test.go lo custodia.
//
// LO QUE SÍ SE AGREGÓ: el conteo de PROCESOS, con K32GetPerformanceInfo. Ahí sí Windows da el
// número exacto y sin ambigüedad.
//
// La aritmética del porcentaje de CPU es la MISMA que la de Linux (contadorCPU, en cpudelta.go),
// y ahí está probada: acá sólo se leen los contadores y se pasan. Esa separación es deliberada —
// es la única forma de que la parte difícil de este archivo esté cubierta por pruebas que corren
// en cualquier plataforma.

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes     = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetTickCount64     = kernel32.NewProc("GetTickCount64")
	procGetPerformanceInfo = kernel32.NewProc("K32GetPerformanceInfo")
)

// filetime es el FILETIME de Win32: intervalos de 100 ns desde 1601. Acá sólo se usan como
// contadores acumulados, así que el epoch no importa: importan las diferencias.
type filetime struct {
	bajo, alto uint32
}

func (f filetime) uint64() uint64 { return uint64(f.alto)<<32 | uint64(f.bajo) }

// memoryStatusEx es MEMORYSTATUSEX. El primer campo DEBE llevar el tamaño de la estructura antes
// de llamar, o la API falla: es el patrón de versionado de Win32.
type memoryStatusEx struct {
	longitud            uint32
	cargaMemoria        uint32
	totalFisica         uint64
	disponibleFisica    uint64
	totalPagina         uint64
	disponiblePagina    uint64
	totalVirtual        uint64
	disponibleVirtual   uint64
	extendidaDisponible uint64
}

// performanceInformation es PERFORMANCE_INFORMATION. Como memoryStatusEx, `cb` lleva el tamaño
// de la estructura antes de llamar: es el mismo patrón de versionado de Win32.
//
// OJO CON LOS TIPOS, porque acá un error no da un fallo sino basura plausible. Los contadores del
// medio son SIZE_T —`uintptr` en Go, 8 bytes en amd64 y 4 en 386— y los TRES últimos son DWORD
// (`uint32`). Escribirlos todos del mismo ancho corre los offsets y K32GetPerformanceInfo
// devuelve números que parecen razonables y no lo son. El padding de 4 bytes que hay entre `cb`
// (DWORD) y el primer SIZE_T en amd64 lo reproduce el propio compilador de Go si los tipos están
// bien escritos: no hay que agregar un relleno a mano.
//
// La alternativa —K32EnumProcesses— exige reintentar con un búfer más grande hasta que quepan
// todos los PIDs, y sin ese reintento CORTA EL CONTEO EN SILENCIO cuando el búfer se llena.
type performanceInformation struct {
	cb                                                 uint32
	commitTotal, commitLimit, commitPeak               uintptr
	physicalTotal, physicalAvailable, systemCache      uintptr
	kernelTotal, kernelPaged, kernelNonpaged, pageSize uintptr
	handleCount, processCount, threadCount             uint32
}

type colectorWindows struct {
	cpu contadorCPU
}

// NuevoColector devuelve el colector de este sistema operativo.
func NuevoColector() Colector { return &colectorWindows{} }

func (c *colectorWindows) Tomar() (Muestra, error) {
	m := Muestra{Tomada: time.Now().UTC(), NumCPU: runtime.NumCPU()}

	// CPU. GetSystemTimes entrega tres contadores acumulados; el de kernel INCLUYE al idle, así
	// que el total es kernel+user y lo ocupado es total menos idle. Confundirlo (restar idle del
	// user, por ejemplo) da porcentajes que parecen razonables y están mal.
	var idle, kernel, user filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r != 0 {
		total := kernel.uint64() + user.uint64()
		if o := idle.uint64(); total >= o {
			m.CPUPct = c.cpu.delta(total-o, total)
		}
	}

	// Memoria. `disponibleFisica` es el análogo de MemAvailable en Linux: lo que se puede pedir.
	// Se fijan total y usada JUNTOS o ninguno (la regla de los pares en muestra.go).
	var mem memoryStatusEx
	mem.longitud = uint32(unsafe.Sizeof(mem))
	if r, _, _ := procGlobalMemoryStatus.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
		if mem.totalFisica > 0 && mem.totalFisica >= mem.disponibleFisica {
			m.MemTotal = mem.totalFisica
			m.MemUsada = mem.totalFisica - mem.disponibleFisica
		}
		// El "page file" es lo más cercano al swap. La aritmética vive en SwapDeWindows, fuera
		// del build tag, para que se pueda probar desde cualquier máquina — que es exactamente lo
		// que faltaba cuando un swap no medible se reportaba como 100 % lleno.
		if total, usada, ok := SwapDeWindows(mem.totalPagina, mem.totalFisica, mem.disponiblePagina, mem.disponibleFisica); ok {
			m.SwapTotal, m.SwapUsada = total, usada
		}
	}

	leerDiscoWindows(&m)

	// Uptime. GetTickCount64 devuelve milisegundos desde el arranque y no se desborda como su
	// versión de 32 bits (que daba la vuelta a los 49 días).
	if ms, _, _ := procGetTickCount64.Call(); ms > 0 {
		m.UptimeSeg = uint64(ms) / 1000
	}

	// PROCESOS. K32GetPerformanceInfo da el conteo exacto en una llamada, sin enumerar nada.
	// `processCount` son PROCESOS; `threadCount` está al lado y son HILOS — es el mismo par que
	// en Linux confunde el 4º campo de /proc/loadavg, y acá está literalmente en el campo de
	// abajo. Si la llamada falla, NumProcesos queda en 0, que aguas arriba significa «no medido».
	var pi performanceInformation
	pi.cb = uint32(unsafe.Sizeof(pi))
	if r, _, _ := procGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&pi)), uintptr(pi.cb)); r != 0 && pi.processCount > 0 {
		m.NumProcesos = int(pi.processCount)
	}

	// Load, temperatura y mem_libre quedan nil: ver el encabezado. La tentación concreta es
	// emitir `mem.disponibleFisica` como mem_libre, y sería el bug de MemFree al revés.
	return m, nil
}

// leerDiscoWindows mide el volumen del sistema con GetDiskFreeSpaceExW, reproduciendo las mismas
// tres columnas que el colector de Linux saca de statfs.
//
// `disponibleParaElUsuario` puede ser MENOR que el libre total cuando hay cuotas de disco
// activas — es el análogo exacto de la reserva de root en Linux, y por eso se reportan los dos
// números: uno es el que verifica un operador, el otro el que dispara la alerta.
func leerDiscoWindows(m *Muestra) {
	ruta, err := syscall.UTF16PtrFromString(`C:\`)
	if err != nil {
		return
	}
	var disponibleParaElUsuario, total, libreTotal uint64
	r, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(ruta)),
		uintptr(unsafe.Pointer(&disponibleParaElUsuario)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&libreTotal)))
	if r == 0 || total == 0 || total < libreTotal {
		return
	}
	m.DiscoTotal = total
	m.DiscoUsado = total - libreTotal
	m.DiscoDisponible = disponibleParaElUsuario
}
