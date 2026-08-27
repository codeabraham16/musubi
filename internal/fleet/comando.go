package fleet

// comando.go es el DOMINIO de la ejecución remota: qué es un comando, en qué estados vive, y qué
// tiene que ser cierto para que se ejecute. Dominio puro, igual que el resto de este paquete.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EstadoComando es dónde está un comando en su ciclo de vida. Los estados son POCOS a propósito:
// cada uno de más es una transición que alguien tiene que acordarse de manejar.
type EstadoComando string

const (
	EstadoPendiente EstadoComando = "pendiente" // encolado, todavía no lo pidió el agente
	EstadoEntregado EstadoComando = "entregado" // el agente se lo llevó; está corriendo
	EstadoTerminado EstadoComando = "terminado" // hay resultado (haya salido bien o mal)
	EstadoExpirado  EstadoComando = "expirado"  // venció antes de que nadie lo levantara (F10)
)

// Límites del canal. Todos existen porque del otro lado hay una máquina que no se puede voltear.
const (
	// ComandoTimeoutDefault y ComandoTimeoutMax acotan cuánto puede correr un comando.
	//
	// El máximo no es una comodidad: sin techo, un comando que espera una entrada que nunca llega
	// deja al agente ocupado para siempre y la máquina desaparece del inventario justo cuando
	// alguien está tratando de arreglarla.
	ComandoTimeoutDefault = 30 * time.Second
	ComandoTimeoutMax     = 10 * time.Minute

	// ComandoVidaMax es cuánto sobrevive un comando SIN QUE NADIE LO LEVANTE (F10).
	//
	// Es el peor pie de bala de una cola: el reinicio de un servicio pedido en una emergencia,
	// ejecutándose siete días después sobre un estado que ya no existe. 15 minutos alcanza para
	// cubrir un agente que se reinicia y es demasiado poco para que el mundo cambie debajo.
	ComandoVidaMax = 15 * time.Minute

	// SalidaMaxBytes acota stdout y stderr POR SEPARADO. Un `cat` sobre un log de 4 GB no puede
	// volcar 4 GB ni en el agente ni en el cerebro. 64 KiB alcanza para el resultado de cualquier
	// comando de operación; lo que necesita más, necesita un archivo, no una consola.
	SalidaMaxBytes = 64 << 10

	// ArgvMaxPartes y ArgvMaxBytes acotan el comando en sí.
	ArgvMaxPartes = 64
	ArgvMaxBytes  = 8 << 10
)

// AvisoTruncado es lo que se agrega a una salida cortada. Cortar en silencio es peor que cortar:
// quien lee un log truncado sin marca saca conclusiones de datos que no están.
const AvisoTruncado = "\n[musubi: salida truncada a 64 KiB]"

var (
	ErrArgvVacio     = errors.New("un comando necesita al menos un ejecutable")
	ErrArgvDemasiado = errors.New("el comando excede los límites de tamaño")
	ErrTimeoutMalo   = errors.New("timeout fuera de rango")
)

// Comando es un pedido de ejecución sobre una máquina.
//
// `Principal` guarda QUIÉN lo pidió y no se deriva de nada en tiempo de lectura: es la columna de
// la que depende toda la auditoría, y tiene que sobrevivir a que esa persona sea dada de baja.
type Comando struct {
	ID        string
	DeviceID  string
	ProjectID string
	Principal string
	Argv      []string
	Timeout   time.Duration
	Estado    EstadoComando

	Creado    time.Time
	Entregado time.Time
	Terminado time.Time

	// El resultado. ExitCode es puntero porque «todavía no terminó» y «terminó con 0» son cosas
	// distintas, y un 0 por default las confundiría — el mismo criterio que gobierna las métricas.
	ExitCode *int
	Stdout   string
	Stderr   string
	// Error es un fallo del CANAL, no del comando: no se pudo lanzar el ejecutable, venció el
	// timeout, el agente murió. Un comando que corre y devuelve 1 NO es un error acá.
	Error string
}

// ValidarComando chequea lo que tiene que ser cierto antes de encolar. Fail-closed.
func ValidarComando(argv []string, timeout time.Duration) error {
	limpio := LimpiarArgv(argv)
	if len(limpio) == 0 {
		return ErrArgvVacio
	}
	if len(limpio) > ArgvMaxPartes {
		return fmt.Errorf("%w: %d partes (máximo %d)", ErrArgvDemasiado, len(limpio), ArgvMaxPartes)
	}
	total := 0
	for _, a := range limpio {
		total += len(a)
	}
	if total > ArgvMaxBytes {
		return fmt.Errorf("%w: %d bytes (máximo %d)", ErrArgvDemasiado, total, ArgvMaxBytes)
	}
	if timeout <= 0 || timeout > ComandoTimeoutMax {
		return fmt.Errorf("%w: %s (entre 1s y %s)", ErrTimeoutMalo, timeout, ComandoTimeoutMax)
	}
	return nil
}

// LimpiarArgv saca las partes vacías del final y de los bordes.
//
// NO toca el contenido de cada parte: un argumento con espacios, comillas o saltos de línea es
// legítimo y el shell no interviene (F7). Alterarlo silenciosamente haría que el comando ejecutado
// difiera del comando registrado en la bitácora, que es exactamente lo que no puede pasar.
func LimpiarArgv(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i, a := range argv {
		if i == 0 {
			a = strings.TrimSpace(a) // el ejecutable sí: un espacio ahí es siempre un error de tipeo
		}
		if a == "" && i == 0 {
			continue
		}
		out = append(out, a)
	}
	// Un argv que quedó sin ejecutable no es un comando.
	if len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		return nil
	}
	return out
}

// Vencido dice si el comando esperó demasiado sin que nadie lo levantara (F10).
//
// Sólo aplica a los PENDIENTES: uno ya entregado está corriendo, y su reloj es el timeout, no
// éste. Confundirlos haría que un comando legítimo de 9 minutos se marque expirado a los 15 y
// aparezca dos veces en la bitácora.
func (c Comando) Vencido(ahora time.Time) bool {
	return c.Estado == EstadoPendiente && ahora.Sub(c.Creado) > ComandoVidaMax
}

// TruncarSalida acota una salida y DEJA LA MARCA. Devuelve también si cortó, para que el
// llamador pueda decirlo por otro canal si quiere.
func TruncarSalida(s string) (string, bool) {
	if len(s) <= SalidaMaxBytes {
		return s, false
	}
	return s[:SalidaMaxBytes] + AvisoTruncado, true
}

// ArgvComoTexto serializa el argv para guardarlo. JSON y no un join por espacios: un argumento
// con un espacio adentro (`--message=hola mundo`) volvería del join como DOS argumentos, y el
// comando registrado dejaría de ser el comando ejecutado.
func ArgvComoTexto(argv []string) (string, error) {
	b, err := json.Marshal(argv)
	if err != nil {
		return "", fmt.Errorf("no se pudo serializar el comando: %w", err)
	}
	return string(b), nil
}

// ArgvDesdeTexto revierte ArgvComoTexto. Un texto ilegible devuelve nil: la fila se puede seguir
// listando en la bitácora aunque su argv no se pueda reconstruir.
func ArgvDesdeTexto(s string) []string {
	var out []string
	if json.Unmarshal([]byte(s), &out) != nil {
		return nil
	}
	return out
}

// ResumenArgv arma una línea legible para logs y paneles. NO se guarda: la fuente es el argv.
func ResumenArgv(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return strings.Join(argv, " ")
}
