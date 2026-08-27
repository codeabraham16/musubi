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

// binarioRustdesk es el ejecutable del cliente. Es `var` para poder apuntarlo a un doble en las
// pruebas: la integración con el binario real es lo único de este archivo que no se puede
// verificar sin una máquina con RustDesk instalado.
var binarioRustdesk = "rustdesk"

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
	cmd := exec.Command(binarioRustdesk, "--password", pass)
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
// o no responde: no es un error, es una máquina sin pantalla configurada todavía.
func idRustdeskLocal() string {
	cmd := exec.Command(binarioRustdesk, "--get-id")
	salida, err := cmd.Output()
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(salida))
	if len(id) > 32 {
		return ""
	}
	return id
}
