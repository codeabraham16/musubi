package fleet

// pantalla.go es el DOMINIO de una sesión de pantalla. Dominio puro: no sabe de RustDesk ni de
// HTTP, sólo de qué es una sesión, cuánto dura y qué se puede guardar de ella.
//
// LO QUE ESTE ARCHIVO NO TIENE, Y ES TODO EL DISEÑO: la estructura `SesionPantalla` **no tiene un
// campo para la contraseña**. No es un olvido — es la garantía G1. La contraseña se acuña, viaja
// dos veces (a la máquina y a quien la pidió) y se descarta. Si algún día alguien le agrega ese
// campo "para poder reconectar", convierte la base en un llavero de acceso a la flota entera.

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// Duraciones de una sesión.
const (
	// SesionDuracionDefault es cuánto vale una contraseña de pantalla.
	//
	// Corta a propósito: el acceso a una pantalla ajena es el permiso más invasivo del track, y
	// una sesión que dura toda la tarde es indistinguible de un acceso permanente. Media hora
	// alcanza para resolver algo; para más, se pide otra y queda otra línea en la bitácora.
	SesionDuracionDefault = 30 * time.Minute
	SesionDuracionMax     = 4 * time.Hour
)

// alfabetoPantalla excluye los caracteres que se confunden al DICTARLOS por teléfono: 0/O, 1/l/I,
// 5/S, 8/B. Una contraseña de sesión se lee en voz alta más seguido de lo que uno cree, y un
// carácter ambiguo se paga con un intento fallido y una llamada más larga.
const alfabetoPantalla = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRTUVWXY2346799"

// largoPassPantalla es de dónde sale la entropía. 16 caracteres del alfabeto de arriba son
// ~91 bits: de sobra contra un atacante que puede probar contra el relay, y todavía dictable.
const largoPassPantalla = 16

// EstadoSesion es dónde está una sesión.
type EstadoSesion string

const (
	SesionSolicitada EstadoSesion = "solicitada" // registrada; la contraseña todavía no llegó a la máquina
	SesionActiva     EstadoSesion = "activa"     // la máquina confirmó que la aplicó
	SesionVencida    EstadoSesion = "vencida"    // pasó su ventana
	SesionFallida    EstadoSesion = "fallida"    // la máquina no pudo aplicarla
)

// SesionPantalla es el REGISTRO de que alguien tuvo acceso a una pantalla. No es un canal ni una
// credencial: es la línea de la bitácora.
type SesionPantalla struct {
	ID        string
	DeviceID  string
	ProjectID string
	Principal string // QUIÉN pidió mirar. La columna de la que depende toda la auditoría.
	Estado    EstadoSesion

	Creada  time.Time
	Vence   time.Time
	Cerrada time.Time

	// Error explica por qué falló, si falló. Nunca contiene la contraseña.
	Error string
}

// Vencida dice si la ventana ya pasó. Se DERIVA, igual que el «en línea» de un dispositivo: una
// columna de estado que hay que ir a actualizar miente en cuanto nadie la actualiza.
func (s SesionPantalla) Vencida(ahora time.Time) bool {
	return !s.Vence.IsZero() && ahora.After(s.Vence)
}

// NuevaPassPantalla acuña una contraseña de sesión.
//
// crypto/rand y no math/rand: es una credencial de acceso a una pantalla ajena. El rechazo por
// módulo se evita usando big.Int sobre el largo exacto del alfabeto.
func NuevaPassPantalla() (string, error) {
	out := make([]byte, largoPassPantalla)
	tope := big.NewInt(int64(len(alfabetoPantalla)))
	for i := range out {
		n, err := rand.Int(rand.Reader, tope)
		if err != nil {
			return "", fmt.Errorf("no se pudo acuñar la contraseña de pantalla: %w", err)
		}
		out[i] = alfabetoPantalla[n.Int64()]
	}
	return string(out), nil
}

// NormalizarDuracion acota lo que pide el llamador.
func NormalizarDuracion(d time.Duration) time.Duration {
	if d <= 0 {
		return SesionDuracionDefault
	}
	if d > SesionDuracionMax {
		return SesionDuracionMax
	}
	return d
}
