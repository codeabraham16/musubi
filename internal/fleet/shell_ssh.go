package fleet

// shell_ssh.go abre una shell INTERACTIVA en un Tier B invocando al `ssh` del sistema.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ TIER B NO NECESITA UN SOLO SYSCALL DE PTY
//
// `ssh -tt` fuerza la asignación de un pty DEL LADO REMOTO. O sea que el pty —lo caro, lo que en
// Go sin cgo obliga a hacer ioctls a mano sobre /dev/ptmx— lo pone el sshd de la otra punta. Este
// lado sólo necesita tres tuberías y no interpretar nada.
//
// Es exactamente por eso que S5b entrega Tier B y deja Tier A para S5c: la mitad interactiva ya
// está resuelta por una herramienta que todas las máquinas tienen.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// CanalInteractivo es una sesión de shell viva: se le escribe lo tecleado y se le lee lo impreso.
//
// El transporte queda detrás de esta interfaz para que el relay del cerebro no sepa si del otro
// lado hay un ssh, un agente con pty propio (S5c) o un doble de prueba.
type CanalInteractivo interface {
	// Escribir manda bytes hacia la máquina. Sin interpretar: lo que se teclea llega tal cual.
	Escribir(p []byte) error
	// Leer devuelve lo que la máquina imprimió, BLOQUEANDO hasta que haya algo o hasta `espera`.
	// Devuelve (nil, nil) si venció la espera sin salida: no es un error, es una terminal quieta.
	Leer(espera time.Duration) ([]byte, error)
	// Cerrar mata la sesión. Idempotente.
	Cerrar() error
	// Terminado se cierra cuando el proceso del otro lado murió por su cuenta (alguien tecleó
	// `exit`, se cayó la red). Permite avisar sin sondear.
	Terminado() <-chan struct{}
}

// AbrirShellPorSSH arranca `ssh -tt` contra el destino y devuelve el canal.
//
// `filas`/`columnas` fijan el tamaño de la terminal al abrir. NO se redimensiona después: eso es
// SIGWINCH y va en S5c, y hasta entonces `top` se dibuja con el ancho inicial (declarado, no
// escondido).
func AbrirShellPorSSH(destino string, filas, columnas int) (CanalInteractivo, error) {
	destino = strings.TrimSpace(destino)
	if destino == "" {
		return nil, fmt.Errorf("el dispositivo no tiene dirección: enrolalo con `address` (host o user@host)")
	}
	cmd := exec.Command(binarioSSH, argumentosShellSSH(destino, filas, columnas)...)

	entrada, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la entrada de ssh: %w", err)
	}
	salida, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la salida de ssh: %w", err)
	}
	// stderr va a la MISMA tubería que stdout. Con `-tt` el remoto ya mezcla las dos en el pty;
	// separarlas de este lado sólo lograría que los mensajes de error de ssh (host key, conexión
	// rechazada) se pierdan en vez de aparecer en la terminal de quien está mirando.
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no se pudo invocar ssh: %w", err)
	}

	c := &canalSSH{cmd: cmd, entrada: entrada, fin: make(chan struct{})}
	c.buf = nuevoBufferInteractivo()
	go c.bombear(salida)
	go func() {
		_ = cmd.Wait()
		c.buf.cerrar()
		close(c.fin)
	}()
	return c, nil
}

// argumentosShellSSH arma la invocación interactiva.
func argumentosShellSSH(destino string, filas, columnas int) []string {
	if filas <= 0 {
		filas = 24
	}
	if columnas <= 0 {
		columnas = 80
	}
	args := []string{
		// -tt FUERZA el pty aunque la entrada de ESTE lado no sea una terminal — y no lo es:
		// es una tubería que alimenta el relay. Con un solo -t, ssh detecta que no hay tty local
		// y no pide pty remoto, y entonces no hay shell interactiva: hay un pipe mudo donde
		// `top` no dibuja, `sudo` no pregunta y Ctrl-C no llega.
		"-tt",
		// Las mismas tres guardas que el one-shot de S5, por las mismas razones.
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=15",
		// StrictHostKeyChecking NO SE DESACTIVA, y acá menos que nunca: por este canal viaja
		// todo lo que la persona teclee, contraseñas de sudo incluidas.
		"-o", "StrictHostKeyChecking=yes",
		// El tamaño inicial de la terminal remota. Va por variable de entorno porque ssh no
		// tiene una opción para fijarlo y el pty remoto lo hereda del entorno de la shell.
		"-o", fmt.Sprintf("SetEnv=LINES=%d COLUMNS=%d", filas, columnas),
	}
	// El puerto se separa igual que en el one-shot: `gio@nas:2222` es la forma que cualquiera
	// escribe y ssh no la entiende. Ver destinoYPuertoSSH.
	host, puerto := destinoYPuertoSSH(destino)
	if puerto != "" {
		args = append(args, "-p", puerto)
	}
	return append(args, "--", host)
}

// canalSSH es CanalInteractivo sobre un `ssh -tt`.
type canalSSH struct {
	cmd     *exec.Cmd
	entrada io.WriteCloser
	buf     *bufferInteractivo
	fin     chan struct{}
}

func (c *canalSSH) Escribir(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	_, err := c.entrada.Write(p)
	return err
}

func (c *canalSSH) Leer(espera time.Duration) ([]byte, error) { return c.buf.leer(espera) }
func (c *canalSSH) Terminado() <-chan struct{}                { return c.fin }

func (c *canalSSH) Cerrar() error {
	_ = c.entrada.Close()
	c.buf.cerrar()
	if c.cmd.Process != nil {
		// Se mata el proceso, no se espera a que la shell remota reaccione a un EOF: T10 dice que
		// si el relay se corta la sesión MUERE, y una shell remota puede ignorar el EOF de su
		// entrada durante mucho tiempo (o para siempre, si está corriendo algo).
		_ = c.cmd.Process.Kill()
	}
	return nil
}

// bombear vuelca lo que imprime la máquina en el buffer, hasta que la tubería se cierra.
func (c *canalSSH) bombear(r io.Reader) {
	p := make([]byte, 8192)
	for {
		n, err := r.Read(p)
		if n > 0 {
			// escribir BLOQUEA si el buffer está lleno. Es contrapresión a propósito: ver
			// bufferInteractivo.
			if !c.buf.escribir(p[:n]) {
				return // el buffer se cerró: no hay a quién entregarle esto
			}
		}
		if err != nil {
			c.buf.cerrar()
			return
		}
	}
}
