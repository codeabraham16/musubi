package fleet

// shell.go es el DOMINIO de una sesión de shell interactiva. Track «Control de flota», S5b.
//
// Dominio puro: no sabe de SSH, ni de HTTP, ni de ptys. Sabe qué es una sesión, cuánto puede
// durar y cuándo hay que matarla.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTA ESTRUCTURA NO TIENE, Y ES DELIBERADO: no hay campo para el CONTENIDO de la sesión.
// Ni lo tecleado ni lo impreso. Eso es GRABACIÓN, y grabar lo que alguien escribe en una terminal
// es una decisión legal antes que técnica — la misma que A14 dejó sin dueño para las sesiones de
// pantalla. Lo que se guarda es que HUBO acceso: quién, a qué máquina, cuándo y por cuánto.
//
// Y es la misma forma que SesionPantalla, por la misma razón: un registro que sirve para auditar
// y que no sirve para entrar.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"errors"
	"fmt"
	"time"
)

// ErrCanalCerrado dice que del otro lado ya no hay nadie: la shell terminó (alguien tecleó
// `exit`), se cayó la conexión, o un techo mató la sesión. Es un final normal, no un fallo, y
// por eso viaja como un error con nombre en vez de como un io.EOF que se confunde con «todavía
// no hay salida».
var ErrCanalCerrado = errors.New("la sesión de shell terminó")

// Los dos techos de una sesión, y son distintos (T5).
const (
	// ShellVidaMax es lo más que puede durar una sesión, se use o no.
	//
	// Una sesión olvidada abierta es una puerta trasera con nombre de nadie. Dos horas alcanzan
	// para arreglar algo; para más, se abre otra y queda otra línea en la bitácora — el mismo
	// criterio que gobierna la duración de una sesión de pantalla.
	ShellVidaMax = 2 * time.Hour

	// ShellInactividadMax es cuánto silencio tolera antes de cerrarse.
	//
	// Va aparte de la vida máxima porque cubre otro caso: la terminal abierta en una pestaña que
	// nadie mira. Con sólo el techo de vida, esa pestaña es un prompt vivo durante dos horas.
	ShellInactividadMax = 15 * time.Minute
)

// EstadoShell es dónde está una sesión.
type EstadoShell string

const (
	ShellAbriendo EstadoShell = "abriendo" // registrada; todavía no se conectó nadie
	ShellActiva   EstadoShell = "activa"   // hay un canal en curso
	ShellCerrada  EstadoShell = "cerrada"  // terminó (por quien la abrió, o por el otro lado)
	ShellVencida  EstadoShell = "vencida"  // la mató un techo: vida o inactividad
	ShellFallida  EstadoShell = "fallida"  // no se pudo abrir
)

// SesionShell es el REGISTRO de que alguien tuvo un prompt en una máquina ajena.
type SesionShell struct {
	ID        string
	DeviceID  string
	ProjectID string
	Principal string // QUIÉN. La columna de la que depende toda la auditoría.
	Estado    EstadoShell

	Creada  time.Time
	Vence   time.Time // Creada + ShellVidaMax: el techo duro
	Cerrada time.Time

	// UltimoTrafico alimenta el techo de INACTIVIDAD. Se mueve con cada byte en cualquiera de
	// las dos direcciones: una sesión donde `tail -f` escupe líneas está viva aunque nadie
	// teclee, y una donde alguien teclea sin salida también.
	UltimoTrafico time.Time

	// Error explica por qué falló o cómo terminó. Nunca contiene nada de lo que pasó por el canal.
	Error string
}

// Vencida dice si algún techo ya la mató, y CUÁL. Se DERIVA y no se guarda: una columna de estado
// que alguien tiene que ir a actualizar miente en cuanto nadie la actualiza — el mismo criterio
// que el «en línea» de un dispositivo y el vencimiento de una sesión de pantalla.
//
// Devuelve el motivo además del booleano porque "se cerró sola" y "se cerró sola porque te fuiste
// a almorzar" son mensajes distintos para quien vuelve y encuentra la terminal muerta.
func (s SesionShell) Vencida(ahora time.Time) (bool, string) {
	if !s.Vence.IsZero() && ahora.After(s.Vence) {
		return true, fmt.Sprintf("la sesión alcanzó su vida máxima (%s)", ShellVidaMax)
	}
	if !s.UltimoTrafico.IsZero() && ahora.Sub(s.UltimoTrafico) > ShellInactividadMax {
		return true, fmt.Sprintf("la sesión se cerró por inactividad (%s sin tráfico)", ShellInactividadMax)
	}
	return false, ""
}

// Viva dice si todavía se puede usar. Es la pregunta que hace CADA request del stream, no sólo la
// que la abrió: una sesión que venció a mitad de un `tail -f` tiene que cortarse ahí.
func (s SesionShell) Viva(ahora time.Time) bool {
	if s.Estado != ShellAbriendo && s.Estado != ShellActiva {
		return false
	}
	vencida, _ := s.Vencida(ahora)
	return !vencida
}

// ValidarAperturaShell revisa lo que se puede saber antes de tocar la red.
func ValidarAperturaShell(d Device) error {
	if !d.Permite(CapShell) {
		return fmt.Errorf("%q no admite shell interactiva: su tier es %s y su concesión es %s",
			d.Name, d.Tier, capsComoTexto(d.Caps))
	}
	// Un Tier B sin dirección no tiene a dónde conectarse. Se dice acá y no en un error de `ssh`
	// tres capas más abajo, que llegaría como "could not resolve hostname" y mandaría a alguien a
	// mirar el DNS.
	if d.Tier == TierProtocolo && d.Address == "" {
		return fmt.Errorf("%q no tiene dirección: un Tier B se alcanza por SSH y hay que decirle a dónde (usuario@host al darlo de alta)", d.Name)
	}
	return nil
}
