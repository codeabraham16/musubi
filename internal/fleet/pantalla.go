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

	// SesionEsperandoPermiso es un `pide` en curso: se le preguntó al usuario de la máquina y
	// todavía no contestó (A57).
	//
	// ES UN ESTADO PROPIO Y NO UNA `solicitada` CON UNA MARCA, porque lo que significa es
	// distinto: en `solicitada` la contraseña YA EXISTE y está viajando; acá **no se acuñó
	// ninguna**. Confundirlas dejaría una credencial creada esperando una respuesta que puede
	// ser «no» — y una contraseña que existe es una contraseña que se puede filtrar, aunque
	// nadie la haya usado.
	SesionEsperandoPermiso EstadoSesion = "esperando_permiso"
	// SesionSinPermiso es un `pide` que no se concedió. El POR QUÉ vive en Consentimiento, no
	// acá: «me dijeron que no», «nadie contestó» y «no había con qué preguntar» terminan todas
	// en este estado y se arreglan distinto.
	SesionSinPermiso EstadoSesion = "sin_permiso"
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

	// Consentimiento es CÓMO contestó el usuario de la máquina, cuando hubo que preguntarle
	// (A57). Vacío = no hizo falta preguntar (`libre` o `avisa`).
	//
	// TIENE COLUMNA PROPIA Y NO VIAJA EN `Error` POR UNA RAZÓN QUE SE PAGA DESPUÉS: «me dijeron
	// que no» no es un error, es el sistema funcionando. Y las tres formas de no conceder
	// —negada, sin_respuesta, no_se_pudo— se arreglan distinto: la primera es una decisión que
	// hay que respetar, la segunda dice que esa máquina quizás no debería estar en `pide`, y la
	// tercera que le falta con qué preguntar. Metidas todas en un texto libre, la diferencia
	// sobrevive hasta que alguien cambia una palabra del mensaje.
	Consentimiento RespuestaAviso
}

// ConcedeElAcceso dice si esta sesión llegó a tener permiso.
//
// Una sesión que NUNCA tuvo que pedirlo (`libre`, `avisa`) lo tiene por definición: el eje de
// consentimiento no es el de capacidad, y confundirlos cerraría el acceso a toda la flota que no
// usa `pide`.
func (s SesionPantalla) ConcedeElAcceso() bool {
	if s.Consentimiento == "" {
		return true
	}
	return s.Consentimiento.Concede()
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
