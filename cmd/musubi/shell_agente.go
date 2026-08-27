package main

// shell_agente.go es el lado de la MÁQUINA en una shell interactiva (S5c): abre un pty local y
// lo conecta con el cerebro.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL PTY SE PIDE CON `script`, NO CON IOCTLS SOBRE /dev/ptmx.
//
// Abrir un pty desde Go sin cgo son tres ioctls (TIOCSPTLCK, TIOCGPTN, TIOCSCTTY) con estructuras
// distintas por sistema operativo y números distintos por arquitectura, más el baile de setsid y
// terminal de control al forkear. Es mucho código delicado, imposible de probar bien en una sola
// máquina, y con un modo de fallo silencioso: un pty a medio configurar da una shell donde el
// Ctrl-C no llega y nadie entiende por qué.
//
// `script` hace exactamente eso y lo hace bien: está en util-linux y en el sistema base de macOS,
// y es el mismo criterio con el que el track ya invoca al `ssh` del sistema en vez de implementar
// SSH, y al `stty` del sistema en vez de hablar termios.
//
// LO QUE SE PAGA POR ESE ATAJO, dicho de frente: el pty lo posee `script`, así que no tenemos su
// descriptor maestro y NO se puede redimensionar la ventana a mitad de sesión (TIOCSWINSZ). El
// tamaño se fija al arrancar. Si el redimensionado alguna vez importa de verdad, obliga a
// escribir el pty a mano — y entonces se paga entero.
//
// Windows no tiene `script` (ahí el equivalente es ConPTY) y por eso queda afuera, con su aviso.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// esperaEntradaAgente es cuánto bloquea el agente pidiéndole teclas al cerebro. Igual que del
// lado de la persona: una terminal quieta no genera tráfico y una activa se ve al instante.
// comandoShellAgente es la operación interna con la que el cerebro llama a un Tier A.
const comandoShellAgente = "musubi:shell"

const esperaEntradaAgente = 25 * time.Second

// vidaMaxDeLaShellLocal es el techo del agente, y es DELIBERADAMENTE MÁS LARGO que el del cerebro.
//
// Los techos de verdad los aplica el cerebro (S5b · T5), que es quien tiene el relay. Éste es una
// red de seguridad para el caso en que el cerebro desaparezca: si el agente se quedara esperando
// para siempre, un cerebro caído dejaría un shell huérfano vivo en la máquina. Más largo que el
// del cerebro para que en operación normal NUNCA sea el que corta — si cortara él, el motivo que
// vería la persona sería el equivocado.
const vidaMaxDeLaShellLocal = 3 * time.Hour

// atenderShellDelCerebro abre el pty y bombea hasta que alguno de los dos lados termine.
//
// Se lo llama desde el ejecutor cuando llega `musubi:shell <id> <filas> <columnas>`. Devuelve un
// resultado para la bitácora: el pedido se registró como un comando y tiene que cerrarse como tal.
func atenderShellDelCerebro(comandoID, base, token string, argv []string) resultadoDeComando {
	// EL ID DEL COMANDO VIAJA EN EL RESULTADO, y olvidarlo no falla ruidosamente: el cerebro
	// responde 403 («ese comando no es de esta máquina»), el agente logea y sigue, y el comando
	// queda `entregado` PARA SIEMPRE — la bitácora nunca registra que la sesión terminó. Lo
	// destapó el e2e; a ojo, en el código, no se ve.
	res := resultadoDeComando{ComandoID: comandoID}
	if len(argv) < 2 {
		res.Error = "musubi:shell sin id de sesión"
		return res
	}
	if runtime.GOOS == "windows" {
		// Se dice con nombre en vez de fallar con un «exec: script: not found», que mandaría a
		// alguien a instalar un paquete que en Windows no existe.
		res.Error = "la shell interactiva todavía no funciona en agentes Windows: el pty se pide con `script`, y el equivalente ahí es ConPTY"
		return res
	}
	sesion := argv[1]
	filas, columnas := 24, 80
	if len(argv) >= 4 {
		if n, err := strconv.Atoi(argv[2]); err == nil && n > 0 {
			filas = n
		}
		if n, err := strconv.Atoi(argv[3]); err == nil && n > 0 {
			columnas = n
		}
	}

	pty, err := abrirPtyLocal(filas, columnas)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer pty.cerrar()

	cli := &http.Client{Timeout: 40 * time.Second} // > el long-poll del cerebro (25 s)
	fin := make(chan error, 2)
	// Lo que el cerebro tiene tecleado baja al pty.
	go func() { fin <- bajarTeclas(cli, base, token, sesion, pty) }()
	// Lo que el pty imprime sube al cerebro.
	go func() { fin <- subirSalida(cli, base, token, sesion, pty) }()

	select {
	case err = <-fin:
	case <-time.After(vidaMaxDeLaShellLocal):
		err = fmt.Errorf("la shell local alcanzó su techo de %s sin que el cerebro la cerrara (¿se cayó el cerebro?)", vidaMaxDeLaShellLocal)
	}
	if err != nil && err != io.EOF {
		res.Error = err.Error()
		return res
	}
	cero := 0
	res.ExitCode = &cero
	res.Stdout = "sesión de shell terminada"
	return res
}

// ptyLocal es el shell corriendo detrás de un pty.
type ptyLocal struct {
	cmd     *exec.Cmd
	entrada io.WriteCloser
	salida  io.ReadCloser
}

func (p *ptyLocal) cerrar() {
	_ = p.entrada.Close()
	_ = p.salida.Close()
	if p.cmd.Process == nil {
		return
	}
	// Se MATA, no se espera a que la shell reaccione a un EOF: una shell puede ignorarlo
	// indefinidamente si está corriendo algo, y dejarla viva sería dejar un proceso sin dueño.
	_ = p.cmd.Process.Kill()
	// Y SE COSECHA. Matar sin esperar deja un ZOMBIE —una entrada de proceso sin padre que la
	// recoja— por cada sesión. En un agente que vive semanas es una fuga de PIDs lenta y
	// silenciosa: no consume CPU ni memoria, y un día la máquina no puede crear procesos.
	//
	// Lo destapó un e2e: `pgrep script` devolvía `[script] <defunct>` después de cerrar la
	// sesión, y a ojo parecía un pty que no había muerto.
	_ = p.cmd.Wait()
}

// abrirPtyLocal arranca `script` con la shell del usuario adentro.
func abrirPtyLocal(filas, columnas int) (*ptyLocal, error) {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	// -q: sin el «Script started/done» que ensuciaría la terminal. -e: el código de salida es el
	// del comando, no el de script. -c: qué correr adentro del pty.
	//
	// El nombre del archivo de transcripción es /dev/null A PROPÓSITO y no es un detalle:
	// `script` GRABA todo lo que pasa por el pty, y grabar lo que alguien teclea en una máquina
	// es exactamente la decisión legal que este track dejó sin tomar. /dev/null es la forma de
	// usar su pty sin usar su grabadora.
	cmd := exec.Command("script", "-qec", shell, "/dev/null")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("LINES=%d", filas),
		fmt.Sprintf("COLUMNS=%d", columnas),
		"TERM=xterm-256color",
	)
	entrada, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la entrada del pty: %w", err)
	}
	salida, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la salida del pty: %w", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no se pudo abrir un pty con `script` (¿está instalado? viene en util-linux): %w", err)
	}
	return &ptyLocal{cmd: cmd, entrada: entrada, salida: salida}, nil
}

// bajarTeclas trae del cerebro lo que la persona tecleó y se lo da al pty.
func bajarTeclas(cli *http.Client, base, token, sesion string, pty *ptyLocal) error {
	url := rutaShellAgente(base, "in", sesion)
	for {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := cli.Do(req)
		if err != nil {
			return err
		}
		datos, _ := io.ReadAll(resp.Body)
		cerrada := resp.Header.Get("X-Musubi-Shell") == "cerrada"
		codigo := resp.StatusCode
		resp.Body.Close()
		if codigo >= 400 {
			return fmt.Errorf("el cerebro cerró la sesión: %s", strings.TrimSpace(string(datos)))
		}
		if len(datos) > 0 {
			if _, err := pty.entrada.Write(datos); err != nil {
				return err
			}
		}
		if cerrada {
			return io.EOF
		}
	}
}

// subirSalida manda al cerebro lo que el pty imprimió.
func subirSalida(cli *http.Client, base, token, sesion string, pty *ptyLocal) error {
	url := rutaShellAgente(base, "out", sesion)
	buf := make([]byte, 8192)
	for {
		n, err := pty.salida.Read(buf)
		if n > 0 {
			req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf[:n]))
			req.Header.Set("Authorization", "Bearer "+token)
			resp, perr := cli.Do(req)
			if perr != nil {
				return perr
			}
			cuerpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			codigo := resp.StatusCode
			resp.Body.Close()
			if codigo >= 400 {
				return fmt.Errorf("el cerebro rechazó la salida: %s", strings.TrimSpace(string(cuerpo)))
			}
		}
		if err != nil {
			return io.EOF // la shell terminó (alguien tecleó `exit`)
		}
	}
}

// rutaShellAgente arma la ruta desde la BASE del cerebro, igual que rutaLatido y rutaResultado.
//
// Existen por lo mismo que aquéllas: una base con o sin barra final producía `//fleet/...` o
// rutas pegadas, y eso ya costó un 404 entero en S5.
func rutaShellAgente(base, cual, sesion string) string {
	return normalizarBase(base) + "/fleet/shell/agent/" + cual + "?id=" + sesion
}
