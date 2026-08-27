package main

// shell.go es `musubi shell <maquina>`: una terminal interactiva en una máquina de la flota,
// pasando por el cerebro. Track «Control de flota», S5b.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL MODO CRUDO SE PIDE CON `stty`, NO CON IOCTLS A MANO.
//
// Poner la terminal en raw desde Go sin dependencias significa hablar termios por syscall, y eso
// es una estructura distinta por sistema operativo y números de ioctl distintos por arquitectura
// (TCGETS no vale lo mismo en x86 que en mips). Es mucho código delicado, difícil de probar en
// una máquina que no sea la del que lo escribe, y con un modo de fallo horrible: la terminal
// queda inutilizable.
//
// `stty` está en toda máquina unix, sabe lo suyo de termios mejor que nosotros, y el track ya
// tiene ese criterio tomado: para llegar a un Tier B se invoca al `ssh` DEL SISTEMA en vez de
// implementar SSH. Windows no tiene stty, y ahí la CLI lo dice en vez de dejar una terminal a
// medio configurar.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// runShell abre una sesión interactiva y conecta la terminal local con ella.
func runShell(args []string) {
	cerebro := strings.TrimSpace(os.Getenv(envCerebro))
	token := strings.TrimSpace(os.Getenv("MUSUBI_TOKEN"))
	var maquina, proyecto string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--brain":
			if i+1 < len(args) {
				cerebro = strings.TrimSpace(args[i+1])
				i++
			}
		case "--token-env":
			if i+1 < len(args) {
				token = strings.TrimSpace(os.Getenv(args[i+1]))
				i++
			}
		case "--project":
			if i+1 < len(args) {
				proyecto = strings.TrimSpace(args[i+1])
				i++
			}
		case "--help", "-h":
			ayudaShell()
			return
		default:
			if !strings.HasPrefix(args[i], "-") && maquina == "" {
				maquina = args[i]
			}
		}
	}

	if maquina == "" {
		ayudaShell()
		os.Exit(1)
	}
	if cerebro == "" || token == "" {
		fmt.Fprintf(os.Stderr, "%s falta cómo llegar al cerebro.\n", cYellow("✗"))
		fmt.Fprintf(os.Stderr, "  Seteá %s con su URL y %s con tu token (o pasá --brain / --token-env).\n",
			cBold(envCerebro), cBold("MUSUBI_TOKEN"))
		os.Exit(1)
	}

	if err := sesionInteractiva(cerebro, token, maquina, proyecto); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n", cYellow("✗"), err)
		os.Exit(1)
	}
}

// sesionInteractiva hace todo el trabajo: abre, conecta la terminal y limpia al salir.
func sesionInteractiva(cerebro, token, maquina, proyecto string) error {
	base := normalizarBase(cerebro)
	cli := &http.Client{Timeout: 40 * time.Second} // > la espera del long-poll (25 s), con margen

	// ────────────────────────────────────────────────────────────────────────────────────────
	// LA TERMINAL SE PREPARA ANTES DE ABRIR LA SESIÓN, Y EL ORDEN NO ES CASUAL.
	//
	// La primera versión abría la sesión y después pedía el modo crudo. Cuando el modo crudo
	// falla —no hay terminal de verdad, stdin redirigido, un cron— la sesión YA ESTÁ ABIERTA: se
	// queda huérfana en el cerebro, y como sólo se permite una viva por persona y máquina (T7),
	// el próximo intento durante los 15 minutos siguientes recibe esa sesión muerta en vez de una
	// nueva. Lo destapó el e2e, corriendo la CLI sin tty.
	//
	// Preparar primero cuesta lo mismo y falla sin haber tocado nada del otro lado.
	// ────────────────────────────────────────────────────────────────────────────────────────
	restaurar, err := ponerTerminalEnCrudo()
	if err != nil {
		return fmt.Errorf("no se pudo poner la terminal en modo crudo: %w", err)
	}
	// LA TERMINAL SE RESTAURA PASE LO QUE PASE. Un `defer` solo no alcanza: si el proceso muere
	// por una señal, la terminal queda sin echo y sin canónico —o sea, inutilizable, y quien lo
	// sufre tiene que teclear `reset` a ciegas—. Por eso además se atrapan las señales.
	var unaVez sync.Once
	limpiar := func() { unaVez.Do(restaurar) }
	defer limpiar()

	filas, columnas := tamanoDeLaTerminal()
	ses, err := abrirSesionRemota(cli, base, token, maquina, proyecto, filas, columnas)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s shell en %s · la sesión vence %s · Ctrl-D o `exit` para salir\r\n",
		cGreen("▶"), cBold(maquina), ses.Vence)

	señales := make(chan os.Signal, 1)
	signal.Notify(señales, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(señales)

	fin := make(chan error, 2)
	// Lo tecleado sube. Sin interpretar nada: Ctrl-C viaja como el byte 0x03 y lo maneja la shell
	// remota, que es lo que uno espera de una terminal remota.
	go func() { fin <- bombearEntrada(cli, base, token, ses.ID) }()
	// Lo impreso baja.
	go func() { fin <- bombearSalida(cli, base, token, ses.ID) }()

	var salida error
	select {
	case salida = <-fin:
	case <-señales:
		salida = nil
	}
	limpiar()
	cerrarSesionRemota(cli, base, token, ses.ID)
	fmt.Fprintf(os.Stderr, "\r\n%s sesión cerrada\r\n", cGreen("▪"))
	if errors.Is(salida, io.EOF) {
		return nil
	}
	return salida
}

// datosSesion es lo poco que la CLI necesita de la respuesta de la tool.
type datosSesion struct {
	ID    string
	Vence string
}

// abrirSesionRemota llama a musubi_fleet_shell por JSON-RPC, igual que cualquier cliente MCP.
func abrirSesionRemota(cli *http.Client, base, token, maquina, proyecto string, filas, columnas int) (datosSesion, error) {
	argumentos := map[string]any{"device": maquina, "filas": filas, "columnas": columnas}
	if proyecto != "" {
		argumentos["project"] = proyecto
	}
	cuerpo, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "musubi_fleet_shell", "arguments": argumentos},
	})
	req, _ := http.NewRequest(http.MethodPost, base+"/mcp", bytes.NewReader(cuerpo))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return datosSesion{}, fmt.Errorf("no se pudo hablar con el cerebro en %s: %w", base, err)
	}
	defer resp.Body.Close()

	var rpc struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return datosSesion{}, fmt.Errorf("respuesta ilegible del cerebro: %w", err)
	}
	if rpc.Error != nil {
		return datosSesion{}, errors.New(rpc.Error.Message)
	}
	if len(rpc.Result.Content) == 0 {
		return datosSesion{}, errors.New("el cerebro no devolvió la sesión")
	}
	var out struct {
		ID    string `json:"session_id"`
		Vence string `json:"vence"`
		Nota  string `json:"nota"`
	}
	if err := json.Unmarshal([]byte(rpc.Result.Content[0].Text), &out); err != nil {
		return datosSesion{}, fmt.Errorf("no se entendió la sesión: %w", err)
	}
	if out.Nota != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", cYellow("·"), out.Nota)
	}
	return datosSesion{ID: out.ID, Vence: out.Vence}, nil
}

// bombearEntrada manda lo tecleado al cerebro, tramo por tramo.
func bombearEntrada(cli *http.Client, base, token, id string) error {
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			req, _ := http.NewRequest(http.MethodPost, base+"/fleet/shell/in?id="+id, bytes.NewReader(buf[:n]))
			req.Header.Set("Authorization", "Bearer "+token)
			resp, perr := cli.Do(req)
			if perr != nil {
				return perr
			}
			cuerpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				return fmt.Errorf("%s", strings.TrimSpace(string(cuerpo)))
			}
		}
		if err != nil {
			return err // io.EOF cuando alguien aprieta Ctrl-D
		}
	}
}

// bombearSalida baja lo impreso y lo escribe en la terminal.
//
// Long-poll: cada GET bloquea hasta 25 s del lado del cerebro y vuelve APENAS hay un byte. Una
// terminal quieta no genera tráfico y una que escupe se ve al instante.
func bombearSalida(cli *http.Client, base, token, id string) error {
	for {
		req, _ := http.NewRequest(http.MethodGet, base+"/fleet/shell/out?id="+id, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := cli.Do(req)
		if err != nil {
			return err
		}
		datos, _ := io.ReadAll(resp.Body)
		cerrada := resp.Header.Get("X-Musubi-Shell") == "cerrada"
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("%s", strings.TrimSpace(string(datos)))
		}
		if len(datos) > 0 {
			// Se escriben los bytes que quedaban ANTES de anunciar el final: las últimas líneas
			// de una shell que muere son justamente las que uno quiere leer.
			if _, err := os.Stdout.Write(datos); err != nil {
				return err
			}
		}
		if cerrada {
			return io.EOF
		}
	}
}

func cerrarSesionRemota(cli *http.Client, base, token, id string) {
	req, _ := http.NewRequest(http.MethodPost, base+"/fleet/shell/close?id="+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if resp, err := cli.Do(req); err == nil {
		resp.Body.Close()
	}
}

// ponerTerminalEnCrudo delega en `stty` y devuelve cómo volver atrás.
//
// `stty -g` devuelve el estado ENTERO en un formato que el propio stty sabe restaurar. Es mejor
// que acordarse de qué banderas se tocaron: si mañana se agrega una, la restauración sigue siendo
// exacta sin cambiar nada acá.
func ponerTerminalEnCrudo() (func(), error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("la terminal interactiva todavía no funciona en Windows: la consola no se configura con stty sino con SetConsoleMode. Desde Linux o macOS sí")
	}
	previo, err := stty("-g")
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el estado de la terminal (¿estás en una terminal de verdad?): %w", err)
	}
	// raw: sin canónico, sin señales, byte a byte. -echo: lo que se teclea lo dibuja la máquina
	// remota, no la local — si no, todo aparece dos veces.
	if _, err := stty("raw", "-echo"); err != nil {
		return nil, err
	}
	return func() { _, _ = stty(strings.TrimSpace(previo)) }, nil
}

// stty invoca al stty del sistema CONTRA LA TERMINAL, no contra stdin.
//
// La distinción importa: stdin puede estar redirigido, y entonces stty configuraría un pipe.
// /dev/tty siempre es la terminal de control.
func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer tty.Close()
	cmd.Stdin = tty
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("stty %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// tamanoDeLaTerminal pregunta cuántas filas y columnas hay, para que el pty remoto nazca del
// tamaño correcto. Si no se puede saber, se devuelve (0,0) y el otro lado usa 24×80: un default
// es mucho mejor que fallar por no poder medir la ventana.
func tamanoDeLaTerminal() (filas, columnas int) {
	salida, err := stty("size")
	if err != nil {
		return 0, 0
	}
	_, _ = fmt.Sscanf(strings.TrimSpace(salida), "%d %d", &filas, &columnas)
	return filas, columnas
}

func ayudaShell() {
	fmt.Printf(`%s — terminal interactiva en una máquina de la flota

  musubi shell <maquina> [--project <id>] [--brain <url>] [--token-env <VAR>]

Exige que tu credencial tenga la capacidad %s sobre esa máquina. NO alcanza con %s:
una shell interactiva se saltea cualquier allowlist de comandos, así que se concede aparte.

%s la shell corre como el usuario que ejecuta el AGENTE (Tier A) o como el usuario
del SSH (Tier B). Si el agente corre como servicio de systemd, es una shell de root:
conceder esa capacidad sobre una máquina es conceder ese usuario, entero.

La sesión tiene dos techos que aplica el cerebro: vida máxima (%s) e inactividad (%s).
Queda auditada —quién, dónde, cuándo, cuánto— y su CONTENIDO no se graba.

Variables: %s (URL del cerebro) · %s (tu token)
`, cBold("musubi shell"), cBold("`shell`"), cBold("`exec`"), cYellow("⚠"),
		cBold("2 h"), cBold("15 min"), cBold(envCerebro), cBold("MUSUBI_TOKEN"))
}
