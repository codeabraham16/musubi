package fleet

// shell_buffer.go es el buffer entre lo que la máquina imprime y lo que el cliente alcanza a
// consumir. Es chico y es la pieza donde se decide qué pasa cuando los dos ritmos no coinciden.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LAS TRES SALIDAS POSIBLES, Y POR QUÉ ESTA
//
// Un `cat archivo-grande` produce megabytes en milisegundos; el cliente los consume a la
// velocidad de la red. Hay tres formas de resolver esa diferencia y dos son malas:
//
//   1. BUFFER SIN LÍMITE — el cerebro se come el archivo entero en RAM. Un `cat /dev/urandom`
//      en una máquina de la flota tumba el cerebro de todos. No.
//
//   2. DESCARTAR LO VIEJO (ring buffer) — no se cae nada, pero la terminal se GARBLEA: faltan
//      bytes en el medio de una secuencia de escape y lo que se ve deja de tener sentido. Peor
//      que un error, porque parece que funciona.
//
//   3. CONTRAPRESIÓN — el escritor se BLOQUEA con el buffer lleno. `cat` corre más lento y no se
//      pierde un byte. Es exactamente lo que hace una terminal real sobre un enlace lento, y por
//      eso es la que no sorprende a nadie.
//
// Se eligió la 3. El bloqueo no es eterno: cerrar el buffer despierta al escritor, y los techos
// de vida e inactividad de la sesión cierran el buffer.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"sync"
	"time"
)

// bufferMaxBytes es cuánto se acumula antes de frenar a la máquina.
//
// 256 KiB es holgado para cualquier redibujado de pantalla (una terminal de 200×60 a todo color
// son ~100 KiB) y chico como para que mil sesiones no sean un problema de memoria.
const bufferMaxBytes = 256 * 1024

// programarDespertador arma el aviso que le pone plazo a `leer`. Es `time.AfterFunc` y es var por la
// prueba, que se dice: la guarda necesita meter una pausa ENTRE armar el aviso y lo que sigue, que es
// la ventana donde se colgaba, y desde afuera de la función no hay otra forma de abrirla.
var programarDespertador = time.AfterFunc

type bufferInteractivo struct {
	mu      sync.Mutex
	hay     *sync.Cond // despierta al lector cuando entran bytes o se cierra
	lugar   *sync.Cond // despierta al escritor cuando se drenó o se cierra
	datos   []byte
	cerrado bool
}

func nuevoBufferInteractivo() *bufferInteractivo {
	b := &bufferInteractivo{}
	b.hay = sync.NewCond(&b.mu)
	b.lugar = sync.NewCond(&b.mu)
	return b
}

// escribir agrega bytes, bloqueando mientras no haya lugar. Devuelve false si el buffer se cerró.
func (b *bufferInteractivo) escribir(p []byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.datos) >= bufferMaxBytes && !b.cerrado {
		b.lugar.Wait()
	}
	if b.cerrado {
		return false
	}
	b.datos = append(b.datos, p...)
	b.hay.Broadcast()
	return true
}

// leer devuelve TODO lo acumulado, bloqueando hasta que haya algo o hasta que venza `espera`.
//
// Devolver todo de una vez y no un tramo fijo importa: el cliente hace un request por lectura, y
// entregar de a 4 KiB convertiría un redibujado de pantalla en veinte viajes de red.
//
// (nil, nil) al vencer la espera NO es un error: es una terminal quieta, que es su estado normal.
func (b *bufferInteractivo) leer(espera time.Duration) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.datos) == 0 && !b.cerrado && espera > 0 {
		// sync.Cond no sabe de timeouts, así que el despertador es un goroutine que hace
		// Broadcast al vencer la espera. Es feo y es la forma 0-dep de tener un Wait con plazo;
		// la alternativa (canales) obliga a un modelo de buffer entero distinto y sin
		// contrapresión natural.
		//
		// EL BUCLE SALE POR LA MARCA QUE PONE EL DESPERTADOR, NO POR UN RELOJ QUE LEE ÉL MISMO. La
		// versión anterior armaba el aviso y DESPUÉS calculaba `limite := time.Now().Add(espera)`, y el
		// bucle seguía mientras `time.Now().Before(limite)`. El aviso suena UNA vez. Si entre armarlo y
		// leer el reloj pasaba más tiempo que lo que tarda el lector en despertarse, el lector se
		// despertaba con su reloj diciendo que todavía quedaba plazo, volvía a `Wait`, y ya no quedaba
		// nadie que lo despertara: una terminal quieta se leía PARA SIEMPRE.
		//
		// Pasó en CI, en Windows, el 2026-09-14: `panic: test timed out after 20m0s` con el lector en
		// `sync.Cond.Wait` hace 16 minutos dentro de `leer(50ms)`. En Linux la ventana es de
		// nanosegundos; en un runner cargado alcanza una expropiación. Medido acá con el temporizador
		// real y ese mismo plazo, forzando sólo la pausa: el bucle viejo no volvía en 3 s, éste vuelve.
		// En producción no era sólo CI: el long-poll de 25 s de una terminal quieta —Tier A en las dos
		// direcciones y Tier B por SSH— no volvía hasta que llegara salida o se cerrara la sesión.
		vencio := false
		fin := programarDespertador(espera, func() {
			b.mu.Lock()
			vencio = true
			b.hay.Broadcast()
			b.mu.Unlock()
		})
		defer fin.Stop()
		for len(b.datos) == 0 && !b.cerrado && !vencio {
			b.hay.Wait()
		}
	}
	if len(b.datos) == 0 {
		if b.cerrado {
			return nil, ErrCanalCerrado
		}
		return nil, nil
	}
	out := b.datos
	b.datos = nil
	// Se drenó: si había un escritor frenado por contrapresión, ahora tiene lugar.
	b.lugar.Broadcast()
	return out, nil
}

// cerrar despierta a todos. Idempotente.
func (b *bufferInteractivo) cerrar() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cerrado {
		return
	}
	b.cerrado = true
	b.hay.Broadcast()
	b.lugar.Broadcast()
}
