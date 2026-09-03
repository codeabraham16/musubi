package main

// pantalla.go es la mitad del plano visual que corre EN LA MÁQUINA (S6).
//
// Musubi no transporta video: el que lo hace es RustDesk, y de eso no se escribe una línea acá.
// Lo único que hace este archivo es aplicar la contraseña de sesión que acuñó el cerebro, y
// —esto es lo importante— PROGRAMAR SU PROPIO VENCIMIENTO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL VENCIMIENTO VIVE ACÁ Y NO EN EL CEREBRO
//
// La alternativa obvia es que el cerebro encole un segundo comando cuando la sesión vence. Es
// peor, y por una razón concreta: si el cerebro se cae, si la red se corta, si alguien apaga
// Musubi — la contraseña queda puesta para siempre. Una caducidad que depende de que otra máquina
// siga viva no es una caducidad, es una promesa.
//
// Con el temporizador local, lo peor que puede pasar es que el AGENTE muera; y si el agente
// muere, se lleva el temporizador y la contraseña queda... puesta. Por eso el reemplazo también
// se intenta al arrancar (ver `cerrarSesionesColgadas`): un agente que vuelve no hereda una
// sesión abierta de su encarnación anterior.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"musubi/internal/fleet"
)

// sesionAbierta recuerda si hay una contraseña puesta, para poder cerrarla al arrancar o al
// vencer. Es estado de proceso, no de disco: si el agente muere, el arranque siguiente cierra por
// las dudas.
var sesionAbierta struct {
	sync.Mutex
	hay bool
}

// aplicarSesionPantalla ejecuta la operación interna `musubi:pantalla <sesion> <pass> <ttl>`.
//
// Devuelve un resultado con la MISMA forma que un comando normal, para que el agente lo reporte
// por el camino que ya existe. Lo que NUNCA hace es incluir la contraseña en el resultado: la
// bitácora no puede aprenderla por la puerta de atrás.
func aplicarSesionPantalla(c comandoRecibido) resultadoDeComando {
	res := resultadoDeComando{ComandoID: c.ID}
	if len(c.Argv) < 4 {
		res.Error = "operación de pantalla mal formada"
		return res
	}
	sesion, pass, ttlTexto := c.Argv[1], c.Argv[2], c.Argv[3]
	ttl, err := time.ParseDuration(ttlTexto)
	if err != nil || ttl <= 0 || ttl > fleet.SesionDuracionMax {
		ttl = fleet.SesionDuracionDefault
	}

	if err := ponerPassRustdesk(pass); err != nil {
		// El error NO puede llevar la contraseña: va derecho a la bitácora.
		res.Error = "no se pudo aplicar la contraseña de pantalla: " + sinSecreto(err.Error(), pass)
		return res
	}
	marcarSesionAbierta(true)

	// El temporizador propio. Reemplaza la contraseña por una al azar QUE NADIE CONOCE — no la
	// borra: dejar a RustDesk sin contraseña sería abrir la máquina, no cerrarla.
	time.AfterFunc(ttl, func() { cerrarSesionPantalla("venció la ventana") })

	cero := 0
	res.ExitCode = &cero
	res.Stdout = fmt.Sprintf("sesión %s aplicada, vence en %s", sesion, ttl)
	return res
}

// ponerPassRustdesk aplica una contraseña permanente en el cliente RustDesk local.
//
// `rustdesk --password <x>` es la interfaz soportada del cliente. Es la ÚNICA línea de todo el
// track que depende de un binario externo, y por eso está aislada acá: todo lo demás del slice
// —la acuñación, el vencimiento, la bitácora, la compuerta— se prueba sin RustDesk instalado.
func ponerPassRustdesk(pass string) error {
	bin, err := rutaRustdesk()
	if err != nil {
		// Este error se ve en la bitácora del comando y en la respuesta de musubi_fleet_screen.
		// Que diga DÓNDE se buscó es la diferencia entre «arreglalo en un minuto» y «probá cosas».
		return err
	}
	cmd := exec.Command(bin, "--password", pass)
	salida, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(salida)))
	}
	return nil
}

// cerrarSesionPantalla reemplaza la contraseña por una al azar que nadie conoce.
func cerrarSesionPantalla(motivo string) {
	sesionAbierta.Lock()
	hay := sesionAbierta.hay
	sesionAbierta.Unlock()
	if !hay {
		return
	}
	nueva, err := fleet.NuevaPassPantalla()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
		return
	}
	if err := ponerPassRustdesk(nueva); err != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
		return
	}
	marcarSesionAbierta(false)
	fmt.Printf("%s sesión de pantalla cerrada (%s)\n", cDim("■"), motivo)
}

func marcarSesionAbierta(v bool) {
	sesionAbierta.Lock()
	sesionAbierta.hay = v
	sesionAbierta.Unlock()
}

// sinSecreto tapa la contraseña si el mensaje de error la trajo de vuelta. Un binario externo
// puede echar sus argumentos en el mensaje, y ese mensaje va derecho a la bitácora.
func sinSecreto(msg, pass string) string {
	if pass == "" {
		return msg
	}
	return strings.ReplaceAll(msg, pass, "[oculto]")
}

// idRustdeskLocal lee el identificador público del cliente. Vacío si RustDesk no está instalado
// o no responde.
//
// Devolver "" SIGUE SIENDO correcto —el latido no puede fallar porque falte una pieza opcional—
// pero ya no es mudo. Antes, una máquina con RustDesk instalado y una sin él producían el mismo
// silencio, y eso ocultó durante todo un track que el binario simplemente no se encontraba
// (Windows no lo pone en el PATH). El aviso sale UNA vez por motivo: el latido corre cada pocos
// segundos y un aviso por latido es ruido, que es otra forma de silencio.
func idRustdeskLocal() string {
	bin, err := rutaRustdesk()
	if err != nil {
		avisarUnaVez("rustdesk-ausente", "el plano de pantalla no está disponible: %v", err)
		return ""
	}
	cmd := exec.Command(bin, "--get-id")
	salida, err := cmd.Output()
	if err != nil {
		// Acá SÍ hay algo roto: el binario está y no contesta. Un permiso, una instalación a
		// medias, un cliente que no arrancó todavía.
		avisarUnaVez("rustdesk-mudo", "RustDesk está en %s pero no devuelve su id: %v", bin, err)
		return ""
	}
	id := strings.TrimSpace(string(salida))
	if len(id) > 32 {
		return ""
	}
	return id
}

// avisarUnaVez imprime un aviso a stderr la PRIMERA vez que se ve cada motivo.
//
// El agente late cada pocos segundos; un aviso por latido llena el journal y deja de leerse, que
// es exactamente el mismo resultado que no avisar. Una vez por motivo y por vida del proceso: si
// el problema se arregla y vuelve, el reinicio del agente lo vuelve a decir.
var avisosDados sync.Map

func avisarUnaVez(motivo, formato string, args ...any) {
	if _, ya := avisosDados.LoadOrStore(motivo, true); ya {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", cYellow("!"), fmt.Sprintf(formato, args...))
}

// avisarCada es avisarUnaVez para los problemas que DURAN.
//
// «Una vez por vida del proceso» es la política correcta para un hecho que no cambia —no hay
// RustDesk instalado, el inventario se recortó— y es la política equivocada para una falla que
// puede estar pasando ahora: el agente arranca, lo dice, y tres días después ese renglón se fue
// con la rotación del journal. Quien mira el log de hoy ve una máquina en silencio y concluye que
// está sana.
//
// Se separó de avisarUnaVez en vez de agregarle un parámetro porque son dos decisiones distintas
// y conviene que se lean distinto en el punto de uso: una dice «esto es así», la otra «esto sigue
// roto».
var avisosConReloj sync.Map

func avisarCada(motivo string, cada time.Duration, formato string, args ...any) {
	ahora := time.Now()
	if previo, hab := avisosConReloj.Load(motivo); hab {
		if ahora.Sub(previo.(time.Time)) < cada {
			return
		}
	}
	avisosConReloj.Store(motivo, ahora)
	fmt.Fprintf(os.Stderr, "%s %s\n", cYellow("!"), fmt.Sprintf(formato, args...))
}
