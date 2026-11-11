//go:build windows

package main

// El agente de flota COMO SERVICIO DE WINDOWS (cable 0 del instalador soberano).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ UN SERVICIO Y NO LA TAREA PROGRAMADA QUE YA HABÍA
//
// Hasta acá el agente de Windows corría por Tarea Programada (deploy/agente-windows.ps1). Una
// tarea con `-AtLogOn` late sólo con un usuario en sesión; una con `-AlArranque` como SYSTEM
// persiste, pero no tiene políticas de recuperación propias, no aparece en services.msc y no sabe
// reportar su ciclo de vida. El servicio sí: lo arranca el SCM en el boot (sin sesión), lo
// reinicia solo si se cae (SetRecoveryActions), se ve y se opera con las herramientas estándar de
// Windows, y su arranque/parada queda en el Registro de eventos.
//
// LO QUE EL SERVICIO **NO** CAMBIA es el agente: adentro corre el MISMO `correrAgente`. Ese agente
// late hacia afuera y ADEMÁS atiende los comandos que el cerebro le manda (exec/shell/pantalla),
// que bajo el servicio corren como LocalSystem. No inicia nada por su cuenta: cada comando pasa por
// el eje de consentimiento (Device.ConsentimientoEfectivo → las puertas de exec/shell/pantalla del
// cerebro). Y acá está el matiz que NO hay que maquillar: en la sesión 0 ese eje sólo NIEGA cuando el
// grado de la máquina es `pide` o `prohibido` (un `pide` se endurece a `prohibido` porque no hay a
// quién preguntarle — ver internal/fleet/consentimiento.go). Pero el grado POR DEFECTO es `avisa`, y
// `avisa` NO se degrada: con `avisa` o `libre` el comando SE EJECUTA como LocalSystem y el aviso que
// correspondería no se puede dibujar en la sesión 0, sólo queda en el log. O sea: tal como se
// instala, con el grado por defecto, este servicio ejecuta exec/shell/pantalla como SYSTEM sin
// diálogo ni aviso visible. Para que la sesión 0 niegue de verdad hay que fijar `pide`/`prohibido`
// con musubi_fleet_consent. El servicio es el envoltorio de ciclo de vida, no una capacidad nueva.
// Por eso el binPath que registra es `musubi agent`: cuando el SCM lo lanza, `runAgent` detecta la
// sesión de servicio (svc.IsWindowsService) y entra por acá.
//
// SESIÓN 0: un servicio corre aislado del escritorio y NO puede dibujar diálogos (ver
// avisador_windows.go). Por eso el cuerpo elevado-en-sesión y este servicio coexisten: el consent
// visual lo da el cuerpo; la persistencia y el alcance de flota los da el servicio.
//
// SI LA CONFIG FALTA (sin token, sin cerebro), `correrAgente` sale con os.Exit: el proceso muere y
// el SCM marca el servicio como fallido y aplica su política de recuperación. Es aceptable: un
// servicio mal configurado no debe fingir que arranca. La parada LIMPIA —la que no puede dejar un
// proceso colgado— es la que llega por el canal de control del SCM, y ésa sí se maneja acá abajo.
//
// SOBRE LA DEPENDENCIA: esto importa `golang.org/x/sys/windows{,/svc,/svc/mgr,/svc/eventlog}`, lo
// que PROMUEVE `golang.org/x/sys` de indirecta a directa — pasa a ser la 7ª dep directa del repo. El
// repo evita x/sys/windows para cosas triviales (ver avisador_windows.go, que usa
// syscall.NewLazyDLL). Acá el cálculo se invierte: el protocolo del SCM a mano son cientos de líneas
// frágiles de advapi32 —no "no cuesta nada"—, x/sys YA estaba en go.mod (no es un módulo nuevo), y
// es la librería estándar de facto para esto.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	// nombreServicioFlota es el nombre con el que el SCM conoce al servicio: el identificador que
	// usan `sc`, services.msc y las políticas de recuperación. NO se traduce ni se adorna.
	nombreServicioFlota = "Musubi-Flota"
	// displayServicioFlota es lo que se ve en services.msc. Acá sí va texto humano.
	displayServicioFlota = "Musubi · Agente de Flota"
	// descServicioFlota se ve en services.msc. Dice la verdad de seguridad SIN maquillar: el servicio no
	// sólo late, también ATIENDE comandos del cerebro (exec/shell/pantalla) como LocalSystem, y con el
	// grado por defecto (avisa) la sesión 0 NO los bloquea — sólo pide/prohibido niegan. Un admin que lo
	// creyera inerte, o que lo creyera cerrado por defecto, subestimaría lo que concede.
	descServicioFlota = "Mantiene esta máquina viva en la flota de Musubi: late al cerebro y atiende " +
		"sus comandos (exec/shell/pantalla) como LocalSystem. El consentimiento sólo los niega en la " +
		"sesión 0 con grado pide o prohibido; con el grado por defecto (avisa) o libre se ejecutan y el " +
		"aviso no se puede mostrar. Fijá el grado con musubi_fleet_consent."
)

// arranqueComoServicioFlota corre el agente bajo el protocolo del SCM si —y sólo si— a este proceso
// lo lanzó el Administrador de Servicios de Windows. Devuelve true cuando tomó el control (el
// llamador no debe seguir) y false en cualquier arranque normal (consola, Tarea Programada), donde
// el agente sigue su camino de siempre.
//
// La detección es lo PRIMERO que hace runAgent: StartServiceCtrlDispatcher (dentro de svc.Run)
// tiene que contestarle al SCM a los pocos segundos del arranque, o el servicio se marca como
// fallido antes de latir una vez.
func arranqueComoServicioFlota(args []string) bool {
	esServicio, err := svc.IsWindowsService()
	if err != nil || !esServicio {
		// Ante la duda, consola: un falso positivo colgaría `musubi agent` en una terminal
		// esperando un SCM que no existe. Un falso negativo lo corrige el SCM (reinicia).
		return false
	}

	// Un servicio no tiene consola: los fmt del agente caen en el vacío. El ciclo de vida va al
	// Registro de eventos. Si no se puede abrir, se sirve IGUAL —el latido que llega al cerebro es
	// la observabilidad de verdad— en vez de no arrancar por no poder loguear (degradar, no abortar).
	elog, err := eventlog.Open(nombreServicioFlota)
	if err != nil {
		elog = nil
	}
	if elog != nil {
		defer elog.Close()
	}

	if err := svc.Run(nombreServicioFlota, &servicioFlota{args: args, elog: elog}); err != nil && elog != nil {
		elog.Error(1, fmt.Sprintf("Musubi-Flota: el despachador del servicio terminó con error: %v", err))
	}
	return true
}

// servicioFlota implementa svc.Handler: traduce los comandos del SCM (Stop/Shutdown/Interrogate) al
// apagado gracioso del bucle de latidos, que se pide cancelando su contexto.
type servicioFlota struct {
	args []string
	elog *eventlog.Log
}

func (h *servicioFlota) logInfo(msg string) {
	if h.elog != nil {
		h.elog.Info(1, msg)
	}
}

// Execute es el cuerpo del servicio. El SCM lo llama en su propia goroutine; adentro se lanza el
// agente en OTRA y se escucha el canal de control hasta que el SCM pida parar.
func (h *servicioFlota) Execute(_ []string, cambios <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	const acepta = svc.AcceptStop | svc.AcceptShutdown
	estado <- svc.Status{State: svc.StartPending}

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	terminado := make(chan struct{})
	go func() {
		defer close(terminado)
		correrAgente(ctx, h.args)
	}()

	estado <- svc.Status{State: svc.Running, Accepts: acepta}
	h.logInfo("Musubi-Flota: en marcha")

bucle:
	for {
		select {
		case <-terminado:
			// El agente retornó por su cuenta: el cerebro dio de baja el dispositivo (401) y
			// `bucleDeLatidos` NO reintenta a propósito —reintentar golpea el lockout—. El servicio
			// acompaña esa decisión y para; no la segunda-adivina reiniciándose.
			h.logInfo("Musubi-Flota: el agente se detuvo por su cuenta (dispositivo dado de baja)")
			break bucle
		case c := <-cambios:
			switch c.Cmd {
			case svc.Interrogate:
				estado <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break bucle
			default:
				// El SCM puede mandar comandos que no declaramos aceptar; se ignoran, como
				// recomienda su documentación, en vez de tratarlos como parada.
			}
		}
	}

	estado <- svc.Status{State: svc.StopPending}
	cancelar()
	// Esperar el cierre REAL del agente antes de devolver. Un Execute que retorna con el bucle
	// todavía latiendo hace que el SCM marque Stopped con el proceso aún vivo: un proceso colgado,
	// que es justo lo que el plan prohíbe. El tope existe porque el SCM nos mata a los ~30 s de
	// todos modos; preferimos reportar Stopped nosotros y no que nos maten a mitad de un latido.
	select {
	case <-terminado:
		h.logInfo("Musubi-Flota: detenido limpio")
	case <-time.After(20 * time.Second):
		if h.elog != nil {
			h.elog.Warning(1, "Musubi-Flota: el agente no cerró en 20 s; se reporta Stopped igual")
		}
	}
	return false, 0
}

// instalarServicioFlota crea el servicio en el SCM. Es una operación PRIVILEGIADA: necesita que el
// `musubi` que la ejecuta corra elevado (el instalador lo hace tras el UAC). Registra además la
// fuente del Registro de eventos para que los mensajes del servicio se vean con texto.
func instalarServicioFlota() error {
	ruta, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo ubicar el binario de musubi: %w", err)
	}
	// Un servicio LocalSystem que apunta a un .exe reemplazable por un usuario sin privilegios es
	// escalada a SYSTEM en cada boot. El instalador soberano deja el binario en Program Files; este
	// aviso cubre el `musubi flota instalar` corrido a mano desde el árbol de dev o Descargas. Se
	// avisa y se sigue (el dueño puede estar probando a propósito), pero que quede dicho.
	advertirSiRutaDeServicioEsInsegura(ruta)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("no se pudo conectar al Administrador de Servicios (¿falta elevación?): %w", err)
	}
	defer m.Disconnect()

	// Idempotencia a medias a propósito: si el servicio ya existe, NO se recrea en silencio —podría
	// estar corriendo con otra config—. Se dice, y quien instala decide (quitar y reinstalar).
	if s, err := m.OpenService(nombreServicioFlota); err == nil {
		s.Close()
		return fmt.Errorf("el servicio %s ya existe: quitalo primero con `musubi flota quitar`", nombreServicioFlota)
	}

	s, err := m.CreateService(nombreServicioFlota, ruta, mgr.Config{
		DisplayName: displayServicioFlota,
		Description: descServicioFlota,
		StartType:   mgr.StartAutomatic, // arranca en el boot, sin sesión de usuario
	}, "agent")
	if err != nil {
		return fmt.Errorf("no se pudo crear el servicio: %w", err)
	}
	defer s.Close()

	// Recuperación y fuente de eventos son MEJORAS best-effort: creado el servicio, YA late. Si una
	// falla se AVISA y se sigue, en vez de salir con error dejando el servicio creado y a un instalador
	// reintentando contra el "ya existe" de arriba. Degradar, no abortar (go-hygiene #3): un servicio
	// sin política de recuperación vale más que ninguno.
	//
	// El período de reset (en segundos) devuelve la cuenta de fallos a cero tras un día sano, para que
	// una caída vieja no agote los reintentos.
	recup := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	if err := s.SetRecoveryActions(recup, 24*60*60); err != nil {
		fmt.Fprintf(os.Stderr, "%s el servicio se creó, pero no se fijaron las políticas de recuperación: %v\n", cYellow("!"), err)
	}
	// La fuente del Event Log se registra en HKLM. Falla, típicamente, si la clave ya existe de una
	// instalación anterior cuyo `quitar` no la limpió: no es fatal —la fuente vieja sirve igual—.
	if err := eventlog.InstallAsEventCreate(nombreServicioFlota, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		fmt.Fprintf(os.Stderr, "%s el servicio se creó, pero no se registró la fuente del Registro de eventos (puede ya existir): %v\n", cYellow("!"), err)
	}
	return nil
}

// quitarServicioFlota detiene y borra el servicio y su fuente de eventos. También es privilegiada.
func quitarServicioFlota() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("no se pudo conectar al Administrador de Servicios (¿falta elevación?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(nombreServicioFlota)
	if err != nil {
		// El servicio no está. Igual se intenta limpiar la fuente de eventos, que pudo quedar huérfana
		// de un `quitar` anterior donde el Delete anduvo y el Remove no.
		limpiarFuenteDeEventos()
		return fmt.Errorf("el servicio %s no está instalado: %w", nombreServicioFlota, err)
	}
	defer s.Close()

	// PARAR ANTES DE BORRAR. DeleteService sólo MARCA el servicio para borrado: si sigue corriendo, el
	// proceso LocalSystem sigue latiendo y atendiendo comandos, y un segundo `quitar` choca contra
	// ERROR_SERVICE_MARKED_FOR_DELETE —callejón hasta un `sc stop` o un reboot—. Imprimir "✓ quitado"
	// sobre un proceso aún vivo es la mentira de estado que el plan prohíbe. Se detiene y se espera.
	if err := detenerYEsperarParada(s); err != nil {
		return err
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("no se pudo borrar el servicio: %w", err)
	}
	limpiarFuenteDeEventos()
	return nil
}

// detenerYEsperarParada pide la parada del servicio y espera a que el SCM lo reporte detenido, con
// tope. Un servicio ya detenido NO es error: es el estado que se busca. El tope de 30 s cubre el de
// 20 s que Execute se da para cerrar el agente, más holgura; si se vence, NO se borra —borrar algo
// que no paró lo deja marcado-para-borrado y vivo, el peor de los mundos—.
func detenerYEsperarParada(s *mgr.Service) error {
	st, err := s.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil // ya estaba detenido: es lo que queremos
		}
		return fmt.Errorf("no se pudo pedir la parada del servicio antes de borrarlo: %w", err)
	}
	limite := time.Now().Add(30 * time.Second)
	for st.State != svc.Stopped {
		if time.Now().After(limite) {
			return fmt.Errorf("el servicio no se detuvo en 30 s; no se borra para no dejarlo marcado-para-borrado y todavía corriendo")
		}
		time.Sleep(300 * time.Millisecond)
		if st, err = s.Query(); err != nil {
			return fmt.Errorf("no se pudo consultar el estado mientras se detenía: %w", err)
		}
	}
	return nil
}

// limpiarFuenteDeEventos quita la fuente del Event Log sin hacer fallar a nadie: que no exista —ya
// removida, o nunca registrada— es un resultado válido, no un error. Se informa en voz baja.
func limpiarFuenteDeEventos() {
	if err := eventlog.Remove(nombreServicioFlota); err != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo quitar la fuente del Registro de eventos (puede no existir): %v\n", cDim("·"), err)
	}
}

// advertirSiRutaDeServicioEsInsegura avisa si el binario a registrar como servicio SYSTEM NO cuelga
// de una carpeta protegida (Program Files / Windows). Ver instalarServicioFlota para el porqué.
//
// RESIDUO CONOCIDO (bajo): SystemRoot se toma entero como protegido, pero tiene bolsillos escribibles
// por un usuario estándar (C:\Windows\Temp, \Tasks). Un binario ahí NO dispara el aviso aunque sea
// reemplazable. No se enumeran esos subdirectorios —la lista es abierta y olvidar uno es peor que no
// tenerla—; el aviso es una barrera blanda y el instalador soberano deja el binario en Program Files,
// no en Windows. Si hiciera falta cerrarlo, la vía segura es sacar SystemRoot de las bases de arriba.
func advertirSiRutaDeServicioEsInsegura(ruta string) {
	for _, base := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramW6432"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("SystemRoot"),
	} {
		if base != "" && rutaDentroDe(ruta, base) {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "%s seguridad: se registra como LocalSystem un binario FUERA de Program Files:\n    %s\n  Si un proceso sin privilegios puede reemplazarlo, escala a SYSTEM en el próximo arranque.\n", cYellow("⚠"), ruta)
}

// rutaDentroDe dice si `ruta` cuelga de `base`. Usa filepath.Rel + IsLocal para no caer en la trampa
// del prefijo textual (C:\Program FilesMalo NO está dentro de C:\Program Files) ni cruzar de volumen.
func rutaDentroDe(ruta, base string) bool {
	r, err1 := filepath.Abs(ruta)
	b, err2 := filepath.Abs(base)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(b, r)
	return err == nil && filepath.IsLocal(rel)
}

// estadoServicioFlota devuelve el estado del servicio en texto, o "no instalado" si no existe. Es
// sólo lectura de verdad: no necesita elevación (ver el porqué de los permisos mínimos adentro).
func estadoServicioFlota() (string, error) {
	// SÓLO LECTURA DE VERDAD. mgr.Connect()/OpenService piden ALL_ACCESS y exigirían elevación —se
	// midió: sin elevar dan "Access is denied"—, rompiendo la promesa de que `estado` no la necesita.
	// Se abre el SCM con SC_MANAGER_CONNECT y el servicio con SERVICE_QUERY_STATUS, los permisos
	// mínimos para consultar, que un usuario estándar sí tiene.
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return "", fmt.Errorf("no se pudo conectar al Administrador de Servicios: %w", err)
	}
	m := &mgr.Mgr{Handle: h}
	defer m.Disconnect()

	nombre, err := windows.UTF16PtrFromString(nombreServicioFlota)
	if err != nil {
		return "", fmt.Errorf("nombre de servicio inválido: %w", err)
	}
	svcH, err := windows.OpenService(m.Handle, nombre, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		// SÓLO "no existe" es "no instalado". Cualquier otro error (permisos, SCM caído) se DICE:
		// tragarlo y responder "no instalado" mentiría sobre un servicio que en realidad está corriendo.
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return "no instalado", nil
		}
		return "", fmt.Errorf("no se pudo abrir el servicio %s: %w", nombreServicioFlota, err)
	}
	s := &mgr.Service{Name: nombreServicioFlota, Handle: svcH}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return "", fmt.Errorf("no se pudo consultar el estado del servicio: %w", err)
	}
	return describirEstadoSvc(st.State), nil
}

// describirEstadoSvc traduce el estado del SCM a una palabra. Función PURA: es la parte del archivo
// que se puede probar sin un SCM ni elevación.
func describirEstadoSvc(e svc.State) string {
	switch e {
	case svc.Stopped:
		return "detenido"
	case svc.StartPending:
		return "arrancando"
	case svc.StopPending:
		return "deteniéndose"
	case svc.Running:
		return "en marcha"
	case svc.PausePending:
		return "pausándose"
	case svc.Paused:
		return "pausado"
	case svc.ContinuePending:
		return "reanudándose"
	default:
		return fmt.Sprintf("estado desconocido (%d)", e)
	}
}

// runFlota despacha `musubi flota <instalar|quitar|estado>`: el ciclo de vida del servicio de
// Windows del agente. instalar/quitar exigen elevación; estado no.
func runFlota(args []string) {
	if len(args) == 0 {
		ayudaFlota()
		return
	}
	switch args[0] {
	case "instalar":
		if err := instalarServicioFlota(); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", cYellow("✗"), err)
			os.Exit(1)
		}
		fmt.Printf("%s servicio %s instalado (arranque automático · reinicio ante caída)\n", cGreen("✓"), nombreServicioFlota)
		fmt.Printf("  %s arrancalo ahora con `sc start %s` o desde services.msc\n", cDim("›"), nombreServicioFlota)
	case "quitar":
		if err := quitarServicioFlota(); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", cYellow("✗"), err)
			os.Exit(1)
		}
		fmt.Printf("%s servicio %s quitado\n", cGreen("✓"), nombreServicioFlota)
	case "estado":
		est, err := estadoServicioFlota()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", cYellow("✗"), err)
			os.Exit(1)
		}
		fmt.Printf("%s %s: %s\n", cDim("■"), nombreServicioFlota, est)
	case "--help", "-h":
		ayudaFlota()
	default:
		fmt.Fprintf(os.Stderr, "subcomando de flota desconocido: %s\n", args[0])
		ayudaFlota()
		os.Exit(1)
	}
}

func ayudaFlota() {
	fmt.Println(cBold("musubi flota") + " — el agente de flota como servicio de Windows")
	fmt.Println()
	fmt.Println(cCyan("Uso:"))
	fmt.Println("  musubi flota instalar    crea el servicio " + cBold(nombreServicioFlota) + " (arranque automático · reinicio ante caída). Pide elevación.")
	fmt.Println("  musubi flota quitar      borra el servicio. Pide elevación.")
	fmt.Println("  musubi flota estado      dice si el servicio está instalado y en qué estado. Sólo lectura.")
	fmt.Println()
	fmt.Println(cCyan("Notas:"))
	fmt.Println("  · Corre como LocalSystem en la sesión 0: late y atiende comandos del cerebro")
	fmt.Println("    (exec/shell/pantalla) con ESE privilegio. El consentimiento sólo los niega en la")
	fmt.Println("    sesión 0 con grado pide o prohibido; con el grado por defecto (avisa) o libre se")
	fmt.Println("    ejecutan y el aviso no se puede mostrar. Fijá el grado con musubi_fleet_consent.")
	fmt.Println("  · Las credenciales salen de las variables de máquina " + cBold("MUSUBI_DEVICE_TOKEN_FILE") + " y " + cBold("MUSUBI_BRAIN_URL") + ".")
	fmt.Println("  · Lo lanza el SCM como `musubi agent`; el mismo binario detecta que corre como servicio.")
}
