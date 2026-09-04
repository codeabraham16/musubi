package fleet

// aprobacion.go es LA SEGUNDA PERSONA: qué máquinas exigen que alguien MÁS apruebe antes de que
// se abra una sesión de pantalla o una shell.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ES UN TERCER EJE, Y NO UNA CAPACIDAD NI UN GRADO DE CONSENTIMIENTO
//
// Ya hay dos ejes y contestan preguntas distintas:
//
//   capacidades        QUIÉN puede entrar.            La contesta quien administra la flota.
//   consentimiento     QUÉ SE LE DEBE a quien está    La contesta el DUEÑO de la máquina.
//                      usando la máquina.
//
// Cuatro ojos contesta una tercera: **CUÁNTAS PERSONAS hacen falta**. No la contesta ninguno de
// los dos anteriores, y no se puede expresar con ellos. Con capacidades sólo se puede quitar el
// acceso, que no es lo mismo que exigir compañía. Y con consentimiento se protege a quien está
// SENTADO en la máquina — que en un servidor de producción no es nadie, justo donde este control
// más hace falta.
//
// El caso que lo motiva es concreto y no es hipotético: una shell interactiva se saltea cualquier
// allowlist de comandos, y una sola persona con `shell` sobre el servidor de producción puede
// hacer cualquier cosa sin que nadie se entere hasta después. La bitácora lo cuenta DESPUÉS. Esto
// exige que alguien lo sepa ANTES.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL QUE APRUEBA NECESITA LA MISMA CAPACIDAD, Y NO SER ADMINISTRADOR
//
// La tentación es pedir `admin`. Está mal por los dos lados:
//
//   · De más: obliga a que un administrador esté disponible para cada sesión, y un control que
//     hay que esperar se termina desactivando «mientras tanto».
//   · De menos: administrar la flota no es saber si ESTA sesión corresponde. Quien puede juzgar
//     si conviene abrir una shell en producción es alguien que también podría abrirla.
//
// La barra correcta es «podría haberlo hecho por su cuenta»: aprobar no le concede nada que no
// tuviera. Lo único que este control agrega es que sean DOS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO ES UNA ETIQUETA, QUE ERA EL PLAN
//
// El plan decía marcar las máquinas con una etiqueta reservada. Las etiquetas de este dominio
// son texto libre del administrador, **no se validan** (ver limpiarTags) y **sólo se escriben al
// enrolar**: no hay forma de cambiarlas después. Las dos cosas lo descalifican como control de
// seguridad. La primera porque `cuatro_ojos` en vez de `cuatro-ojos` apagaría el control en
// silencio —una configuración que parece puesta y no lo está, que es el modo de falla que este
// dominio persigue desde el eje de consentimiento—. Y la segunda porque marcar una máquina como
// sensible es algo que se aprende DESPUÉS de enrolarla: con etiquetas habría que revocarla y
// volver a instalarle el agente, o sea ir a la máquina, para activar un control que se quiere
// activar justo cuando ya no se puede ir.
//
// Así que es un campo propio, con su tool de administrador, igual que el consentimiento.

import (
	"fmt"
	"strings"
	"time"
)

// VentanaDeAprobacion es cuánto vale un «sí» antes de tener que volver a pedirlo.
//
// TREINTA MINUTOS ES CORTO A PROPÓSITO. Una aprobación es permiso para UNA sesión, no una
// ventana de trabajo: si durara horas, la segunda persona estaría aprobando algo que no puede
// ver. Y es lo bastante largo para que quien aprueba no tenga que estar mirando la pantalla en
// el mismo minuto — que sería un control que sólo funciona con los dos sentados juntos.
const VentanaDeAprobacion = 30 * time.Minute

// EstadoAprobacion es en qué quedó una solicitud.
type EstadoAprobacion string

const (
	// AprobacionPendiente: se pidió y todavía nadie contestó.
	AprobacionPendiente EstadoAprobacion = "pendiente"
	// AprobacionConcedida: alguien dijo que sí y todavía no se usó.
	AprobacionConcedida EstadoAprobacion = "concedida"
	// AprobacionNegada: alguien dijo que no. NO se vuelve a preguntar sola.
	AprobacionNegada EstadoAprobacion = "negada"
	// AprobacionUsada: ya abrió su sesión.
	//
	// ES UN ESTADO PROPIO Y NO UN BORRADO, por lo mismo que las demás tablas de este dominio son
	// append-only: «esta sesión la aprobó fulano» es exactamente el hecho que este control existe
	// para dejar escrito. Borrar la fila al consumirla dejaría la sesión en la bitácora sin quién
	// la avaló, que es la mitad que importa.
	AprobacionUsada EstadoAprobacion = "usada"
)

// SolicitudDeAprobacion es el pedido de una segunda persona para UNA sesión.
//
// No es una sesión ni una credencial: es el permiso que la habilita, y vive aparte de
// screen_sessions y shell_sessions a propósito. Las dos sesiones tienen modelos distintos, y
// meter el mismo campo en las dos dejaría dos lugares donde recordar consumirlo una sola vez.
type SolicitudDeAprobacion struct {
	ID          string
	DeviceID    string
	ProjectID   string
	Solicitante string // quién quiere entrar
	Capacidad   Cap    // qué quiere hacer: `shell`, `screen` o `screen:view`
	Motivo      string // lo que el solicitante declara. Es para el que aprueba, no para la máquina.
	Estado      EstadoAprobacion
	// Aprobador es quién contestó. Vacío mientras está pendiente.
	Aprobador string
	// Nota es lo que contestó el que aprueba. Un «no» sin motivo manda a preguntar por otro lado.
	Nota string

	Creada   time.Time
	Vence    time.Time
	Resuelta time.Time
	Usada    time.Time
}

// Vencida dice si el pedido pasó su ventana.
func (s SolicitudDeAprobacion) Vencida(ahora time.Time) bool {
	return !s.Vence.IsZero() && !ahora.Before(s.Vence)
}

// Utilizable dice si este permiso alcanza para abrir la sesión AHORA.
//
// Las tres condiciones son necesarias y ninguna se puede aflojar: concedida (nadie dijo que sí
// todavía si está pendiente), sin usar (es de un solo uso) y dentro de la ventana.
func (s SolicitudDeAprobacion) Utilizable(ahora time.Time) bool {
	return s.Estado == AprobacionConcedida && !s.Vencida(ahora)
}

// CapAprobable dice si una capacidad puede quedar bajo cuatro ojos.
//
// LA LISTA ES BLANCA Y NO NEGRA. Con una lista negra, una capacidad nueva quedaría aprobable sin
// que nadie lo decidiera, y «aprobable» acá significa que hay un camino de código que la
// consume: una capacidad que nadie gatea aceptaría solicitudes que no habilitan nada, y quien
// las apruebe creería estar autorizando algo.
//
// `metrics` queda afuera y es deliberado: leer telemetría no se le pide a nadie. Poner cuatro
// ojos sobre eso convertiría el panel en un formulario y enseñaría a apagar el control.
// `exec` también queda afuera POR AHORA — no porque no corresponda, sino porque su camino no lo
// consume todavía, y una lista que promete más de lo que el código hace es peor que una corta.
func CapAprobable(c Cap) bool {
	switch c {
	case CapShell, CapScreen, CapScreenView:
		return true
	}
	return false
}

// ValidarSolicitud comprueba lo que no puede depender de quien llama.
func ValidarSolicitud(s SolicitudDeAprobacion) error {
	if strings.TrimSpace(s.DeviceID) == "" {
		return fmt.Errorf("la solicitud de aprobación no nombra ninguna máquina")
	}
	if strings.TrimSpace(s.Solicitante) == "" {
		return fmt.Errorf("la solicitud de aprobación no dice quién la pide")
	}
	if !CapAprobable(s.Capacidad) {
		return fmt.Errorf("la capacidad %q no se aprueba por cuatro ojos: sólo %q, %q y %q tienen un camino que consuma la aprobación",
			s.Capacidad, CapShell, CapScreen, CapScreenView)
	}
	if s.Vence.Before(s.Creada) || s.Vence.Equal(s.Creada) {
		return fmt.Errorf("la solicitud de aprobación vence antes de existir")
	}
	return nil
}

// ErrSeAprueboSolo es el rechazo que da sentido a todo lo demás.
//
// Se nombra como variable —y no como un string armado en el sitio— para que la prueba de
// sabotaje pueda apuntarle. Si esta comprobación se cae, el control sigue pareciendo puesto:
// hay tabla, hay tool, hay estado «concedida» en la bitácora, y una sola persona abre la sesión.
// Es el único falso verde que este archivo no puede permitirse.
var ErrSeAprueboSolo = fmt.Errorf("nadie puede aprobar su propia solicitud: eso son dos ojos, no cuatro")
