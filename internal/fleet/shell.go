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

	// ────────────────────────────────────────────────────────────────────────────────────────
	// LOS TRES ESTADOS DEL EJE DE CONSENTIMIENTO (A75), CALCADOS DE LA PANTALLA
	//
	// Una máquina en `pide` no da un prompt hasta que quien la está usando diga que sí, y eso
	// parte la apertura en DOS pedidos: uno que pregunta y otro que conecta. Los nombres son los
	// mismos que en SesionPantalla a propósito —`esperando_permiso`, `sin_permiso`— porque
	// SesionViva.Abierta los mira por su TEXTO para no dibujar a nadie adentro de una máquina
	// donde todavía nadie entró. Dos vocabularios para el mismo estado dejarían ese panel
	// mintiendo en la mitad de sus filas.

	// ShellEsperandoPermiso es un `pide` en curso: se preguntó y nadie contestó todavía. NO hay
	// canal abierto ni nada reservado del otro lado; es un PEDIDO con fecha de vencimiento.
	ShellEsperandoPermiso EstadoShell = "esperando_permiso"
	// ShellSinPermiso es un `pide` que no se concedió. El POR QUÉ vive en Consentimiento y no
	// acá: «me dijeron que no», «nadie contestó» y «no había con qué preguntar» terminan las
	// tres en este estado y se arreglan distinto.
	ShellSinPermiso EstadoShell = "sin_permiso"
	// ShellPermitida es el permiso ya dado y la shell TODAVÍA SIN CONECTAR.
	//
	// ES UN ESTADO PROPIO Y NO UN `abriendo` CON UNA MARCA, y la razón es T7: `abriendo` cuenta
	// como sesión VIVA, así que el permiso concedido se leería como «ya tenías una shell abierta
	// acá» y el segundo pedido —el que viene a conectar— recibiría esa fila en vez de un prompt.
	// El permiso no es la sesión: dura lo que dura la ventana del pedido y se consume una vez.
	ShellPermitida EstadoShell = "permitida"
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

	// Consentimiento es CÓMO contestó quien está usando la máquina, cuando hubo que preguntarle
	// (A75). Vacío = no hizo falta preguntar (`libre` o `avisa`).
	//
	// TIENE COLUMNA PROPIA Y NO VIAJA EN `Error`, por lo mismo que en SesionPantalla: «me dijeron
	// que no» no es un error, es el sistema funcionando. Y las tres formas de no conceder
	// —negada, sin_respuesta, no_se_pudo— se arreglan distinto: la primera es una decisión que
	// hay que respetar, la segunda dice que esa máquina quizás no debería estar en `pide`, y la
	// tercera que le falta con qué preguntar. Metidas en un texto libre, la diferencia sobrevive
	// hasta que alguien mejora la redacción del mensaje.
	Consentimiento RespuestaAviso
}

// ConcedeElAcceso dice si esta sesión llegó a tener permiso.
//
// Una que NUNCA tuvo que pedirlo (`libre`, `avisa`) lo tiene por definición: el eje de
// consentimiento no es el de capacidad, y confundirlos cerraría la shell de toda la flota que no
// usa `pide`.
func (s SesionShell) ConcedeElAcceso() bool {
	if s.Consentimiento == "" {
		return true
	}
	return s.Consentimiento.Concede()
}

// Vencida dice si algún techo ya la mató, y CUÁL. Se DERIVA y no se guarda: una columna de estado
// que alguien tiene que ir a actualizar miente en cuanto nadie la actualiza — el mismo criterio
// que el «en línea» de un dispositivo y el vencimiento de una sesión de pantalla.
//
// Devuelve el motivo además del booleano porque "se cerró sola" y "se cerró sola porque te fuiste
// a almorzar" son mensajes distintos para quien vuelve y encuentra la terminal muerta.
func (s SesionShell) Vencida(ahora time.Time) (bool, string) {
	// UN PEDIDO DE PERMISO VENCIDO NO ES UNA SESIÓN QUE LLEGÓ A SU VIDA MÁXIMA (A75), y el
	// motivo importa porque es lo que el barrendero deja escrito en la bitácora. Estas filas
	// vencen en VentanaDePermiso —tres minutos— y no en las dos horas del techo de vida: decir
	// «alcanzó su vida máxima (2h)» sobre un pedido de tres minutos manda a mirar el techo
	// equivocado, y sobre todo esconde el único diagnóstico útil, que es que nadie contestó.
	if s.Estado == ShellEsperandoPermiso || s.Estado == ShellPermitida {
		if !s.Vence.IsZero() && ahora.After(s.Vence) {
			if s.Estado == ShellPermitida {
				return true, fmt.Sprintf("el permiso se concedió y nadie vino a conectarse en %s", VentanaDePermiso)
			}
			return true, fmt.Sprintf("nadie contestó el pedido de permiso en %s", VentanaDePermiso)
		}
		// NO se le aplica el techo de INACTIVIDAD: no hay canal, así que «sin tráfico» es su
		// estado normal y no una sesión olvidada. Con el techo puesto, la fila moriría por el
		// motivo equivocado en cuanto la ventana pasara los quince minutos.
		return false, ""
	}
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
