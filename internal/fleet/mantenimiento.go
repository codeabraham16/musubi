package fleet

import (
	"fmt"
	"time"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA VENTANA DE MANTENIMIENTO
//
// «De tal hora a tal hora, esta máquina va a estar rara a propósito.»
//
// Existe en el DOMINIO y no en la configuración de Alertmanager, y la diferencia no es de
// prolijidad: un `silence` calla el aviso y no frena nada más. Las políticas no leen alertas —
// leen la muestra y actúan solas—, así que un reinicio planificado de postgres dispara
// `servicio_caido`, el auto-heal lo levanta EN MITAD DEL MANTENIMIENTO, y el silence sólo
// garantiza que nadie se entere. Es la peor combinación: la automatización sigue actuando y el
// canal que lo contaría está apagado.
// ════════════════════════════════════════════════════════════════════════════════════════════

// MantenimientoMax es el largo máximo de una ventana.
//
// EL TECHO NO ES BUROCRACIA, ES EL ÚNICO ANTÍDOTO CONTRA LA VENTANA ETERNA. Sin él, un `hasta`
// escrito con un dedo de más —2027 en vez de 2026— deja una máquina ciega para siempre: sin
// alertas, sin políticas, y con todo en verde. Es exactamente la forma de falla de A62 (una cola
// sin techo) y de `ComandoVidaMax`, y se cierra igual: el dominio no deja expresarlo.
//
// 24 horas cubre lo que un mantenimiento real tarda —una migración larga, un fin de semana de
// mudanza son dos ventanas— y obliga a que alguien vuelva a decir «sigue». Un mantenimiento que
// nadie renueva en un día no es un mantenimiento: es algo que se olvidaron de cerrar.
const MantenimientoMax = 24 * time.Hour

// Mantenimiento es una ventana declarada sobre UNA máquina.
type Mantenimiento struct {
	ID        string
	DeviceID  string
	ProjectID string
	// Principal es quién la declaró. Va como columna y no como referencia, igual que en
	// device_commands: quién lo pidió es un hecho del pasado y no puede depender de que esa
	// credencial siga existiendo hoy.
	Principal string
	Desde     time.Time
	Hasta     time.Time
	Motivo    string
	// Cancelada marca la fila como retirada. NO se borra: la cronología se construye sólo sobre
	// tablas append-only, y «hubo un mantenimiento y lo cancelaron a los diez minutos» explica el
	// comportamiento de esa máquina mejor que la ausencia de toda fila.
	Cancelada bool
	Creado    time.Time
}

// Activa dice si la ventana cubre ese instante. Una cancelada no cubre nada.
//
// El borde: `desde` INCLUSIVE y `hasta` EXCLUSIVO. La alternativa —los dos inclusive— hace que
// dos ventanas consecutivas se solapen un instante, y el solapamiento de algo que silencia
// alertas es la clase de detalle que nadie mira hasta que importa.
func (m Mantenimiento) Activa(ahora time.Time) bool {
	if m.Cancelada {
		return false
	}
	return !ahora.Before(m.Desde) && ahora.Before(m.Hasta)
}

// ValidarMantenimiento chequea lo que tiene que ser cierto ANTES de que la ventana exista.
//
// Fail-closed, igual que ValidarAlta y ValidarComando: ante la duda, no se declara. Una ventana
// mal formada no falla al crearse sino al cubrir de más, y para entonces la máquina ya estuvo
// ciega el tiempo que no correspondía.
func ValidarMantenimiento(m Mantenimiento) error {
	if m.DeviceID == "" {
		return fmt.Errorf("la ventana de mantenimiento no dice sobre qué máquina es")
	}
	if m.Principal == "" {
		return fmt.Errorf("la ventana de mantenimiento no dice quién la declaró: una máquina que se calla sin dueño es lo que esto viene a evitar")
	}
	if m.Desde.IsZero() || m.Hasta.IsZero() {
		return fmt.Errorf("la ventana necesita `desde` y `hasta`")
	}
	if !m.Hasta.After(m.Desde) {
		return fmt.Errorf("la ventana termina antes de empezar (desde %s, hasta %s)",
			m.Desde.Format(time.RFC3339), m.Hasta.Format(time.RFC3339))
	}
	if d := m.Hasta.Sub(m.Desde); d > MantenimientoMax {
		return fmt.Errorf("la ventana dura %s y el máximo es %s: una ventana sin techo deja una máquina ciega para siempre —sin alertas, sin políticas y con todo en verde— por un `hasta` escrito con un dedo de más. Si el mantenimiento sigue, se declara otra",
			d.Round(time.Minute), MantenimientoMax)
	}
	if len(m.Motivo) > 200 {
		return fmt.Errorf("el motivo tiene %d caracteres y el máximo es 200", len(m.Motivo))
	}
	return nil
}
