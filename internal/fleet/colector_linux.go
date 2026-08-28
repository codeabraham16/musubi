//go:build linux

package fleet

// colector_linux.go lee el estado del host de /proc y /sys. Stdlib pura: sin cgo, sin
// dependencias. `gopsutil` daría los tres sistemas operativos de una y sería la 7ª dependencia
// directa de un repo que tiene 6 y un observability.go escrito a propósito con «cero
// dependencias nuevas»; se prefirió el seam y un colector honesto por OS.
//
// EL PUNTO NO OBVIO, y es el que decide el diseño entero: /proc/stat NO da un porcentaje, da
// JIFFIES ACUMULADOS desde el arranque. Un «uso de CPU» es la derivada entre dos lecturas, así
// que el colector tiene que RECORDAR la anterior. De ahí salen dos consecuencias que están en el
// contrato, no escondidas acá:
//
//   - La primera muestra no tiene porcentaje (nil, nunca 0).
//   - El porcentaje es el promedio del intervalo entre latidos, no un instante.

import (
	"os"
	"runtime"
	"strings"
	"syscall"
)

// colectorLinux lee /proc y /sys. La aritmética del porcentaje vive en contadorCPU (cpudelta.go),
// compartida con el colector de Windows: las fuentes son distintas, la derivada es la misma.
type colectorLinux struct {
	cpu contadorCPU
}

// NuevoColector devuelve el colector de este sistema operativo.
func NuevoColector() Colector { return &colectorLinux{} }

func (c *colectorLinux) Tomar() (Muestra, error) {
	// LEE los archivos y delega el PARSEO a procparse.go, que es el mismo código que usan el
	// colector remoto (Tier B por SSH) y el de Android (por ADB).
	//
	// Compartirlo no es elegancia: es que el bug de MemFree contra MemAvailable —el que hace que
	// un Linux sano aparezca al 95 % para siempre— habría que arreglarlo en tres lugares si cada
	// fuente parseara por su cuenta. Y dos de esos tres no se pueden probar desde acá.
	l := LecturasProc{
		Stat:    leerArchivo("/proc/stat"),
		Meminfo: leerArchivo("/proc/meminfo"),
		Loadavg: leerArchivo("/proc/loadavg"),
		Uptime:  leerArchivo("/proc/uptime"),
		TempMil: leerArchivo("/sys/class/thermal/thermal_zone0/temp"),
		Procs:   listarProc(),
		NumCPU:  runtime.NumCPU(),
	}
	m := MuestraDesde(l, &c.cpu)

	// El disco es la ÚNICA parte que no sale de un archivo: localmente se mide con statfs, que es
	// más barato y más exacto que invocar a `df`. Remotamente no hay statfs posible, así que allá
	// se usa la salida de `df` — y ParsearDf produce los mismos tres números.
	leerDisco(&m)
	return m, nil
}

// leerArchivo devuelve el contenido, o vacío si no se pudo leer. Un archivo ausente NO es un
// error de la muestra: /sys/class/thermal no existe en toda máquina, y una lectura fallida deja
// su campo en nil, que es exactamente lo que significa «no lo pude medir».
func leerArchivo(ruta string) string {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return ""
	}
	return string(b)
}

// listarProc devuelve los nombres de las entradas de /proc, uno por línea. Si no se puede leer
// —un contenedor sin /proc montado, un permiso raro— devuelve vacío, que aguas abajo significa
// «no medido» y no «cero procesos».
//
// Devuelve el TEXTO y no el conteo a propósito: el filtro difícil («el nombre es todo dígitos»,
// que es lo que separa un proceso de `self` o de `cpuinfo`) vive en ContarPids y lo comparte con
// el Tier B por SSH y con el Tier C por ADB, que no se pueden probar desde esta máquina.
func listarProc() string {
	entradas, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entradas {
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}
	return b.String()
}

// leerDisco mide el sistema de archivos RAÍZ con statfs, reproduciendo EXACTAMENTE las tres
// columnas de `df`: Size, Used y Avail.
//
// Son tres y no dos porque Used + Avail ≠ Size: entre medio está la reserva de root (~5 %). La
// primera versión de esto calculaba Usado como `Blocks - Bavail` y el comentario afirmaba que
// era «el mismo número que df». No lo era: se iba un 29,8 % —los 25,6 GB de reserva sobre los
// 502 GB de esta máquina—, y lo encontró la prueba que contrasta contra `df` de verdad.
//
//	Usado       = (Blocks - Bfree)  -> lo que ocupan los archivos; lo que un operador verifica.
//	Disponible  = Bavail            -> lo que una aplicación todavía puede escribir; la alerta.
//
// Dar sólo uno de los dos obliga a elegir entre un panel que no cuadra con df y una alerta que
// avisa tarde. Se dan los dos y cada consumidor usa el que su pregunta necesita.
func leerDisco(m *Muestra) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return
	}
	tam := uint64(st.Bsize)
	m.DiscoTotal = st.Blocks * tam
	if st.Blocks >= st.Bfree {
		m.DiscoUsado = (st.Blocks - st.Bfree) * tam
	}
	m.DiscoDisponible = st.Bavail * tam
}
