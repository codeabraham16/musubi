//go:build darwin

package fleet

// colector_darwin.go lee lo que macOS expone por sysctl y statfs, en stdlib pura.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ESTE COLECTOR MIDE MENOS QUE LOS OTROS DOS, Y ESO ESTÁ DECLARADO
//
// macOS guarda las métricas más interesantes detrás de MACH, no de sysctl: el uso de CPU sale de
// host_processor_info y la memoria en uso de host_statistics64. Las dos son llamadas mach que
// desde Go sin cgo exigen construir el mensaje IPC a mano — bastante código delicado para un
// número, y una superficie fea en el proceso que corre en todas las máquinas.
//
// Se prefirió medir MENOS y decirlo. Lo que este colector no puede medir viaja como nil o queda
// sin fijar, y el resto del sistema ya sabe leer eso: la tool devuelve `null`, Prometheus no
// emite la serie, y el panel muestra un hueco en vez de un cero que se creería.
//
// Lo que SÍ mide, que no es poco: DISCO (el que más alerta dispara), CARGA, uptime y CPUs.
//
// Cerrarlo del todo —CPU y memoria por mach— está anotado como su propio slice.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"runtime"
	"syscall"
	"time"
)

type colectorDarwin struct{}

// NuevoColector devuelve el colector de este sistema operativo.
func NuevoColector() Colector { return colectorDarwin{} }

func (colectorDarwin) Tomar() (Muestra, error) {
	m := Muestra{Tomada: time.Now().UTC(), NumCPU: runtime.NumCPU()}

	// Uptime desde kern.boottime. `syscall.SysctlTimeval` NO está en la stdlib (vive en
	// golang.org/x/sys), pero `syscall.Sysctl` devuelve el buffer crudo, así que se decodifica a
	// mano — el parseo vive en sysctlparse.go, sin build tag y con prueba.
	if raw, err := syscall.Sysctl("kern.boottime"); err == nil {
		if arranque, ok := parseBoottime([]byte(raw)); ok {
			if seg := time.Now().Unix() - arranque; seg > 0 {
				m.UptimeSeg = uint64(seg)
			}
		}
	}

	// Carga. vm.loadavg viene en PUNTO FIJO (ldavg[i] / fscale); leerlo crudo daría cargas de
	// varios miles. Ver parseLoadavg.
	if raw, err := syscall.Sysctl("vm.loadavg"); err == nil {
		if l1, l5, l15, ok := parseLoadavg([]byte(raw)); ok {
			m.Load1, m.Load5, m.Load15 = f64(l1), f64(l5), f64(l15)
		}
	}

	// Disco: statfs existe en darwin igual que en Linux, y las tres columnas se calculan igual.
	// Bavail excluye la reserva del sistema, Bfree no: mismos dos números, misma razón.
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil && st.Blocks > 0 {
		tam := uint64(st.Bsize)
		total := st.Blocks * tam
		if st.Blocks >= st.Bfree {
			m.DiscoTotal = total
			m.DiscoUsado = (st.Blocks - st.Bfree) * tam
			m.DiscoDisponible = st.Bavail * tam
		}
	}

	// MEMORIA: se conoce el total (hw.memsize) y NO la usada. Por la regla de los pares no se
	// fija ninguno de los dos — un total sin su usado produce un 0 % que se lee como «vacío»,
	// que es exactamente la mentira que este diseño evita. Vuelve cuando llegue el slice de mach.
	//
	// CPU y carga: nil. Ver el encabezado.
	return m, nil
}
