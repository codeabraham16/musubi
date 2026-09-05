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
	// EstadoPerdido es «el agente se lo llevó y no volvió» (A60). Se DERIVA, nunca se guarda:
	// una columna que hay que ir a actualizar miente en cuanto nadie la actualiza.
	EstadoPerdido EstadoComando = "perdido"
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

	// ComandosPorEntregaMax es cuántos comandos se lleva un agente de una sola vez. Vive ACÁ y no
	// en el transporte porque la derivación de `perdido` depende de él: si el transporte lo sube
	// y esta constante no se entera, la cota se vuelve incorrecta en silencio.
	ComandosPorEntregaMax = 10

	// MargenDeReporte cubre el viaje de vuelta del resultado. El agente reporta apenas termina
	// cada comando —no espera al próximo latido— así que es un round-trip HTTP, no un ciclo de
	// sondeo. Dos minutos es holgado a propósito.
	MargenDeReporte = 2 * time.Minute

	// EsperaMaxDeEntregado es cuánto puede estar `entregado` un comando VIVO, en el peor caso.
	//
	// LA REGLA OBVIA —`entregado + timeout + margen`— ES INCORRECTA, y descubrirlo es lo que
	// costó de A60. El agente ejecuta la tanda EN ORDEN Y DE A UNO, reportando cada resultado
	// antes de pasar al siguiente. Así que el último de una tanda de diez espera a los nueve de
	// adelante ANTES de que su propio timeout empiece a correr: puede estar legítimamente
	// `entregado` casi cien minutos sin que nada esté mal.
	//
	// Con la regla obvia, ese comando se dibujaría muerto mientras corre. Es el error CARO de los
	// dos: un comando vivo marcado perdido manda a alguien a relanzarlo —dos veces el mismo
	// `systemctl`, dos veces el mismo borrado— mientras que uno perdido marcado tarde sólo se ve
	// tarde. Se prefiere tarde y cierto a temprano y falso.
	//
	// SE PODRÍA AJUSTAR y a propósito no se hace todavía: como el agente reporta de a uno, los
	// comandos que siguen `entregado` en una máquina son exactamente los que faltan, y los creados
	// antes que uno son los que tiene por delante. Esa cota decae sola y sería mucho más fina.
	// Pide contexto de la máquina entera, y `EstadoActual` es un método de UN comando con tres
	// llamadores, dos de los cuales no tienen la lista. Se deja anotado, no olvidado.
	EsperaMaxDeEntregado = ComandosPorEntregaMax*ComandoTimeoutMax + MargenDeReporte

	// ColaMaxPorDevice es cuántos comandos TODAVÍA EJECUTABLES puede tener encolados una máquina.
	//
	// ────────────────────────────────────────────────────────────────────────────────────────
	// EL TECHO QUE FALTABA, Y LA MEDICIÓN QUE LO PIDIÓ
	//
	// Hasta acá `EncolarComando` aceptaba lo que le dieran. Medido en producción el 2026-08-30:
	// una máquina caída hacía dos días tenía **11.007 comandos pendientes**, encolados por un
	// lazo que corrió treinta y cuatro horas a cinco por minuto contra un agente que no iba a
	// levantar ninguno.
	//
	// No explotó nada —F10 los vence a todos antes de entregarlos, así que ni uno se habría
	// ejecutado— pero eso es suerte de diseño, no un límite: la tabla crece sin tope y la
	// bitácora de esa máquina queda ilegible para siempre. Y el día que el lazo corra contra una
	// máquina VIVA, lo que se acumula sí se ejecuta.
	//
	// CINCUENTA PORQUE MÁS QUE ESO NO ES UNA COLA, ES UN LAZO. Una persona encola un puñado; una
	// política encola uno por disparo y tiene enfriamiento. Cincuenta pendientes significan que
	// la máquina va a correr cincuenta cosas en cuanto vuelva, que ya es mucho.
	ColaMaxPorDevice = 50

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
	// ErrColaLlena es que esta MÁQUINA ya tiene demasiado esperando. Es un error de la máquina,
	// no del comando: el mismo argv sobre otro host entra sin problema, y por eso el mensaje
	// tiene que hablar de la máquina y no del pedido.
	ErrColaLlena = errors.New("la cola de esa máquina está llena")
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
	// Origen es QUIÉN lo originó (A59). Vacío = no se sabe, que es el valor de todo lo anterior
	// a la migración 41 y NO significa «una persona».
	Origen OrigenComando

	// Clasificacion es a qué plano pertenece esta fila, DECLARADO por quien encola. Sólo hace
	// falta para las operaciones internas cuyo argv es idéntico en varios planos
	// (fleet.OpsClasificadasPorFila): sin ella, el aviso de un exec se leía como pantalla y lo
	// veía cualquiera con `screen:view`. Vacío = no se sabe, que es el valor de todo lo anterior
	// a la migración 46 y NO significa «pantalla». Ver TipoDeComando.
	Clasificacion TipoDeHecho

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

// EstadoActual es el estado que hay que MOSTRAR, a esta hora.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE DERIVA AL LEER, Y NO ERA ASÍ — LO ENCONTRÓ USAR LA CRONOLOGÍA, NO LEER EL CÓDIGO
//
// `expirado` se estampa en UN solo lugar: adentro de `TomarComandos`, o sea CUANDO EL AGENTE
// VIENE A PEDIR SU COLA. Eso está bien para lo que esa función decide —que el comando no se
// ejecute— y es exactamente insuficiente para lo que un panel MUESTRA: si el agente no vuelve
// nunca, nadie estampa nada y la fila dice `pendiente` para siempre.
//
// Medido en producción el 2026-08-30: cincuenta comandos sobre una máquina cuyo agente estaba
// caído, encolados diez HORAS antes, todos figurando `pendiente` con una vida máxima de 15
// minutos. Los mostraban las dos superficies de lectura, la vieja y la nueva.
//
// Es la misma regla que el resto del dominio aplica en todos lados —«una columna de estado que
// hay que ir a actualizar miente en cuanto nadie la actualiza»— y que las dos clases de sesión ya
// respetaban (`SesionPantalla.Vencida`, `SesionShell.Vencida`). Los comandos se habían quedado
// afuera, y el modo de falla sólo se ve cuando una máquina se cae y NO VUELVE: mientras los
// agentes andan, cada latido estampa el `expirado` de rebote y la mentira dura minutos.
//
// LA ESCRITURA SIGUE EXISTIENDO Y NO SE TOCA: la de `TomarComandos` es la que impide que se
// ejecute, que es una decisión, no una vista. Ésta sólo dice qué mostrar.
//
// SON DOS MUERTES DISTINTAS Y TIENEN DOS ESTADOS DISTINTOS. `Vencido` es «nadie lo levantó
// nunca» (`expirado`); `Perdido` es «el agente se lo llevó y no volvió» (`perdido`, A60). Darles
// el mismo nombre haría indistinguible un agente que nunca vino de uno que se murió a mitad, que
// son problemas de máquinas distintas y se arreglan mirando lugares distintos.
//
// EXISTE PARA QUE LAS SUPERFICIES NO REPITAN LA CONDICIÓN: dos consumidores decidiendo cada uno
// cuándo vence un comando terminan discrepando, y el que se olvida es el que dibuja `pendiente`
// sobre algo muerto hace horas.
func (c Comando) EstadoActual(ahora time.Time) EstadoComando {
	if c.Vencido(ahora) {
		return EstadoExpirado
	}
	if c.Perdido(ahora) {
		return EstadoPerdido
	}
	return c.Estado
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

// OrigenComando es QUIÉN originó un comando: una persona o una regla (A59).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EXISTE PORQUE UNA LÍNEA DE TIEMPO ES UN RELATO, Y «QUIÉN» ES LA MITAD DEL RELATO
//
// La acción de una política va a la MISMA bitácora que la de una persona, con el mismo peso y
// las mismas columnas (I16) — y eso está bien: un registro aparte «para lo automático» es cómo
// se termina auditando sólo la mitad de lo que pasa, y la mitad automática es justo la que nadie
// miró ejecutarse.
//
// Lo que faltaba es poder DISTINGUIRLAS al leer. Hasta acá la diferencia se leía del nombre del
// principal, por convención; con cuarenta reinicios seguidos en una cronología, «auto-heal
// reinició nginx cuarenta veces» y «alguien llamado auto-heal lo reinició cuarenta veces» son dos
// relatos distintos y sólo uno es cierto.
type OrigenComando string

const (
	// OrigenPersona: alguien lo pidió, por una tool o por el canal de una sesión.
	OrigenPersona OrigenComando = "persona"
	// OrigenPolitica: lo disparó una regla del motor de políticas, sin que nadie mirara.
	OrigenPolitica OrigenComando = "politica"
	// OrigenDesconocido es el VACÍO, y es el valor de todas las filas anteriores a la migración
	// 41. NO es sinónimo de `persona`: rellenarlas así le atribuiría a alguien acciones que
	// disparó una regla. Se muestra como null, no como un tercer nombre inventado.
	OrigenDesconocido OrigenComando = ""
)

// EsAutomatico dice si esto lo disparó una regla. Es una lista BLANCA: lo desconocido NO es
// automático y tampoco es manual — no se sabe, y la superficie lo dice.
func (o OrigenComando) EsAutomatico() bool { return o == OrigenPolitica }

// OrigenValido acota lo que se puede guardar. Fail-closed: un valor que no está en la lista se
// guarda como desconocido en vez de inventar una categoría nueva desde el borde.
func OrigenValido(o OrigenComando) OrigenComando {
	switch o {
	case OrigenPersona, OrigenPolitica:
		return o
	}
	return OrigenDesconocido
}

// Vencido dice si el comando esperó demasiado sin que nadie lo levantara (F10).
//
// ESTUVO ESCRITA Y SIN LLAMAR desde S5. La derivación existía, estaba probada, y ninguna
// superficie la usaba — así que la bitácora dibujaba `pendiente` sobre comandos vencidos hacía
// horas. Su único llamador es `EstadoActual`, que es por donde tienen que pasar todas las vistas.
//
// Sólo aplica a los PENDIENTES: uno ya entregado está corriendo, y su reloj es el timeout, no
// éste. Confundirlos haría que un comando legítimo de 9 minutos se marque expirado a los 15 y
// aparezca dos veces en la bitácora.
func (c Comando) Vencido(ahora time.Time) bool {
	return c.Estado == EstadoPendiente && ahora.Sub(c.Creado) > ComandoVidaMax
}

// Perdido dice si un comando ENTREGADO ya no va a reportar nunca (A60).
//
// El agente se lo llevó y se murió a mitad: la fila queda en `entregado` para siempre, porque
// `terminado` lo estampa el reporte y ese reporte no va a llegar. No es lo mismo que `expirado`
// —ése ni se levantó— y por eso no se puede derivar con la regla de los pendientes.
//
// `Entregado` en cero se trata como NO perdido: una fila vieja sin ese dato es un agujero de
// datos, no un comando muerto, y dibujar `perdido` sobre un agujero es inventar.
func (c Comando) Perdido(ahora time.Time) bool {
	if c.Estado != EstadoEntregado || c.Entregado.IsZero() {
		return false
	}
	return ahora.Sub(c.Entregado) > EsperaMaxDeEntregado
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
