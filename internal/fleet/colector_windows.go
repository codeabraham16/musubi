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
		// El "page file" es lo más cercano al swap. totalPagina incluye la RAM en Windows, así
		// que el swap real es la diferencia; si no da positiva, no se reporta.
		if mem.totalPagina > mem.totalFisica {
			total := mem.totalPagina - mem.totalFisica
			libre := uint64(0)
			if mem.disponiblePagina > mem.disponibleFisica {
				libre = mem.disponiblePagina - mem.disponibleFisica
			}
			if total >= libre {
				m.SwapTotal, m.SwapUsada = total, total-libre
			}
		}
	}

	leerDiscoWindows(&m)

	// Uptime. GetTickCount64 devuelve milisegundos desde el arranque y no se desborda como su
	// versión de 32 bits (que daba la vuelta a los 49 días).
	if ms, _, _ := procGetTickCount64.Call(); ms > 0 {
		m.UptimeSeg = uint64(ms) / 1000
	}

	// Load y temperatura quedan nil: ver el encabezado.
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
