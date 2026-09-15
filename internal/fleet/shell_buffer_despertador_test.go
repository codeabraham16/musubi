package fleet

import (
	"testing"
	"time"
)

// TestLaLecturaConPlazoVuelveAunqueElRelojDigaQueQuedaTiempo fija que `leer` salga por el despertador
// y no por un reloj que el lector lee él mismo.
//
// EL DESPERTADOR SUENA UNA VEZ. Si el bucle decide por su propio reloj y ese reloj todavía dice que
// queda plazo cuando llega el único aviso, el lector vuelve a dormir y nadie lo despierta. Pasó en la
// CI de Windows el 2026-09-14: `internal/mcp` murió por timeout a los 20 minutos con el lector en
// `sync.Cond.Wait` hace 16, adentro de `leer(50ms)`, en TestUnaShellDeTierASinAgenteSeDistingueDeUnaTerminalQuieta.
//
// LA PAUSA ES LO ÚNICO FORZADO. Temporizador real y el mismo plazo de 50 ms que esa prueba; lo que la
// costura agrega es una espera entre ARMAR el aviso y lo que sigue, que es lo que una expropiación
// hace en un runner cargado. Sin forzarla la ventana es de nanosegundos y esta guarda no mediría nada
// en Linux: sería verde sobre el único mundo que se puede montar acá.
//
// EL VIGÍA NO ES DECORACIÓN: si el defecto vuelve, `leer` no retorna, y una guarda que se cuelga
// convierte su hallazgo en otro timeout de 20 minutos sin nombre. Cerrar el buffer libera al lector
// y la prueba falla diciendo qué pasó.
//
// Sabotaje que la pone roja: volver a que el bucle decida por el reloj.
// arnes: archivo="internal/fleet/shell_buffer.go"
// arnes: de="\t\tfor len(b.datos) == 0 && !b.cerrado && !vencio {"
// arnes: a="\t\tlimite := time.Now().Add(espera)\n\t\tfor len(b.datos) == 0 && !b.cerrado && (!vencio || time.Now().Before(limite)) {"
// arnes: arreglo_de="\t\tfor len(b.datos) == 0 && !b.cerrado && !vencio {"
// arnes: arreglo_a="\t\tfor !vencio && !b.cerrado && len(b.datos) == 0 {"
func TestLaLecturaConPlazoVuelveAunqueElRelojDigaQueQuedaTiempo(t *testing.T) {
	const espera = 50 * time.Millisecond

	leerConVigia := func(b *bufferInteractivo) (time.Duration, bool) {
		listo := make(chan time.Duration, 1)
		inicio := time.Now()
		go func() { _, _ = b.leer(espera); listo <- time.Since(inicio) }()
		select {
		case d := <-listo:
			return d, true
		case <-time.After(3 * time.Second):
			b.cerrar()
			<-listo
			return 0, false
		}
	}

	t.Run("CONTROL: sin pausa forzada vuelve, y no antes del plazo", func(t *testing.T) {
		// Sin esto, un `leer` que volviera en el acto también pasaría la mitad de abajo, y la guarda no
		// distinguiría «salió por el despertador» de «no esperó nada».
		d, volvio := leerConVigia(nuevoBufferInteractivo())
		if !volvio {
			t.Fatal("leer(50ms) sobre un buffer vacío no volvió en 3 s SIN ninguna pausa forzada: el defecto no es la ventana, es otro")
		}
		if d < espera {
			t.Errorf("leer(%v) volvió en %v, antes de su plazo: no esperó", espera, d)
		}
	})

	t.Run("con una pausa entre armar el despertador y seguir, vuelve igual", func(t *testing.T) {
		orig := programarDespertador
		t.Cleanup(func() { programarDespertador = orig })
		programarDespertador = func(d time.Duration, f func()) *time.Timer {
			tm := time.AfterFunc(d, f)
			time.Sleep(2 * d)
			return tm
		}
		if _, volvio := leerConVigia(nuevoBufferInteractivo()); !volvio {
			t.Errorf("COLGADO: leer(%v) no volvió en 3 s. El único aviso del despertador llegó con el reloj "+
				"del lector diciendo que quedaba plazo, y volvió a dormir sin nadie que lo despierte.", espera)
		}
	})
}
