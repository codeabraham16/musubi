package main

// agent.go implementa `musubi agent`: el proceso que corre EN CADA MÁQUINA de la flota y le dice
// al cerebro «sigo viva». Es el productor real del registro que creó S1 — hasta acá la tabla
// `devices` existía y nadie la escribía.
//
// LO QUE ESTE PROCESO **NO** PUEDE HACER, y es el diseño del track:
//
//   - No lee memoria. Su credencial NO abre /mcp (ver internal/mcp/fleet_http.go). Un agente
//     comprometido —y corre en la superficie más expuesta que hay: la máquina de un cliente, un
//     portátil que viaja— no entrega la memoria de la empresa.
//   - No ejecuta nada. Exec llega en S5, detrás de su propia capacidad.
//   - No dice más de lo que mide. Desde S4 el latido lleva telemetría del HOST (CPU, RAM, disco,
//     carga, uptime), y nada más: ni qué procesos corren, ni qué archivos hay, ni un byte de
//     contenido. Cuanto menos superficie tenga el proceso que corre en todas partes, mejor.
//
// EN UN SISTEMA OPERATIVO SIN COLECTOR el agente late IGUAL y lo dice. No manda una muestra de
// ceros: un panel que pinta 0 % de CPU en todos los Windows se cree y no se arregla.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"musubi/internal/fleet"
)

const (
	// intervaloLatidoDefault es cada cuánto late el agente.
	//
	// 30 s está atado al umbral con el que el cerebro da una máquina por caída
	// (umbralEnLineaDefault = 90 s en internal/mcp/methods_fleet.go): son exactamente 3
	// intervalos. Con un solo intervalo de margen cualquier hipo de red pinta la flota de rojo;
	// con tres, hace falta perder tres latidos seguidos. Si se cambia uno, hay que mirar el otro
	// — están atados por el FACTOR 3, no por los valores.
	intervaloLatidoDefault = 30 * time.Second

	// esperaMinima y esperaMaxima acotan el backoff exponencial cuando el cerebro no responde.
	//
	// El techo existe por una razón concreta: un backoff sin tope, tras una noche de cerebro
	// caído, deja al agente esperando horas y la máquina figura muerta MUCHO después de que la
	// red volvió. 5 minutos es el peor caso de "cuánto tarda en reaparecer" y es aceptable.
	esperaMinima = 5 * time.Second
	esperaMaxima = 5 * time.Minute

	// envToken y envCerebro son de dónde salen las credenciales. Variables de entorno y no un
	// archivo: es lo que ya usan connect-brain-{linux,windows} para el token del cerebro, y
	// mantener UNA forma de configurar la máquina es más importante que la comodidad de un flag.
	envToken   = "MUSUBI_DEVICE_TOKEN"
	envCerebro = "MUSUBI_BRAIN_URL"
)

// runAgent es el punto de entrada de `musubi agent`.
func runAgent(args []string) {
	cerebro := strings.TrimSpace(os.Getenv(envCerebro))
	token := strings.TrimSpace(os.Getenv(envToken))
	intervalo := intervaloLatidoDefault
	unaVez := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--brain":
			if i+1 < len(args) {
				cerebro = strings.TrimSpace(args[i+1])
				i++
			}
		case "--interval":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					intervalo = time.Duration(n) * time.Second
				}
				i++
			}
		case "--once":
			// Un solo latido y sale. Es lo que necesita un cron/systemd timer, y lo que hace
			// verificable la instalación sin dejar un proceso colgado.
			unaVez = true
		case "--revisar-blindaje":
			// SALE ANTES DE PEDIR CREDENCIAL. Verificar el blindaje es lo que se hace ANTES de
			// tener el agente andando —o cuando dejó de andar—, y exigirle un token a un
			// diagnóstico lo vuelve inútil justo cuando hace falta.
			os.Exit(revisarBlindajeDelAgente())
		case "--help", "-h":
			ayudaAgent()
			return
		}
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "%s falta la credencial del dispositivo.\n", cYellow("✗"))
		fmt.Fprintf(os.Stderr, "  Seteá %s con el token que devolvió musubi_fleet_enroll.\n", cBold(envToken))
		fmt.Fprintf(os.Stderr, "  Ese token se muestra UNA sola vez: si lo perdiste, hay que revocar y volver a enrolar.\n")
		os.Exit(1)
	}
	if cerebro == "" {
		fmt.Fprintf(os.Stderr, "%s falta la dirección del cerebro.\n", cYellow("✗"))
		fmt.Fprintf(os.Stderr, "  Seteá %s (ej: http://100.x.y.z:7717) o pasá --brain.\n", cBold(envCerebro))
		os.Exit(1)
	}

	// LA BASE, no una ruta concreta. El agente habla por DOS rutas —el latido y el reporte de
	// resultados— y pasar la del latido como base construía `/fleet/heartbeat/fleet/result`.
	// Lo encontró la prueba end-to-end: los unitarios apuntaban a un httptest que responde a
	// cualquier ruta, así que el 404 no aparecía. Que la base sea la base es el invariante.
	base := strings.TrimSuffix(cerebro, "/")

	// El colector guarda la lectura anterior de CPU entre llamadas, así que se crea UNO y se
	// reusa: uno nuevo por latido nunca tendría contra qué restar y el porcentaje sería
	// siempre nil.
	col := fleet.NuevoColector()

	if unaVez {
		res := latir(base, token, tomarMuestra(col))
		fmt.Println(res.describir())
		atenderComandos(base, token, res.comandos)
		if !res.ok {
			os.Exit(1)
		}
		return
	}

	// Un agente que vuelve NO hereda una sesión de pantalla de su encarnación anterior: si murió
	// con una contraseña puesta, se llevó el temporizador y la contraseña quedó. Cerrar acá es
	// barato y cubre ese hueco.
	marcarSesionAbierta(true)
	cerrarSesionPantalla("arranque del agente")

	fmt.Printf("%s agente activo · cerebro %s · latido cada %s\n", cGreen("▶"), cBold(cerebro), intervalo)
	if _, err := col.Tomar(); err != nil {
		// D4 — sin colector para este OS, se dice UNA vez al arrancar y el agente sigue latiendo.
		// Estar viva es información útil aunque no se pueda medir cómo está.
		fmt.Printf("%s sin telemetría: %v\n", cYellow("!"), err)
	}
	bucleDeLatidos(base, token, intervalo, col)
}

// bucleDeLatidos late hasta que lo maten o hasta que el cerebro diga que este dispositivo ya no
// pertenece a la flota.
//
// Escucha SIGINT/SIGTERM: un agente que ignora la señal de apagado deja al systemd de cada máquina
// esperando el timeout de kill, y eso se nota en el apagado de un servidor.
func bucleDeLatidos(base, token string, intervalo time.Duration, col fleet.Colector) {
	señales := make(chan os.Signal, 1)
	signal.Notify(señales, os.Interrupt, syscall.SIGTERM)

	espera := esperaMinima
	tick := time.NewTimer(0) // el primer latido sale ya, sin esperar un intervalo entero
	defer tick.Stop()

	for {
		select {
		case <-señales:
			fmt.Printf("\n%s agente detenido\n", cDim("■"))
			return
		case <-tick.C:
		}

		res := latir(base, token, tomarMuestra(col))
		switch {
		case res.revocado:
			// B5 — el kill-switch tiene que ser entendible DESDE LA MÁQUINA. Un agente que no
			// interpreta el 401 obliga a ir a apagarlo a mano, que es exactamente lo que no se
			// puede hacer con un equipo remoto.
			fmt.Printf("%s el cerebro rechazó la credencial: este dispositivo fue dado de baja.\n", cYellow("!"))
			fmt.Printf("  %s\n", cDim(res.motivo))
			fmt.Printf("%s agente detenido (no se reintenta: reintentar sería golpear el lockout del cerebro).\n", cDim("■"))
			return
		case res.ok:
			espera = esperaMinima // la red volvió: se resetea el backoff
			atenderComandos(base, token, res.comandos)
			tick.Reset(intervalo)
		default:
			// B7 — el cerebro caído NO mata al agente. Backoff exponencial acotado.
			fmt.Fprintf(os.Stderr, "%s %s · reintento en %s\n", cYellow("!"), res.describir(), espera)
			tick.Reset(espera)
			if espera *= 2; espera > esperaMaxima {
				espera = esperaMaxima
			}
		}
	}
}

// resultadoLatido separa los tres desenlaces que el agente trata DISTINTO: latió, hay que
// reintentar, o este dispositivo ya no pertenece a la flota.
type resultadoLatido struct {
	ok       bool
	revocado bool // 401: la credencial no vale más. NO se reintenta.
	motivo   string
	// comandos son los pedidos de ejecución que el cerebro devolvió en este latido (S5).
	comandos []comandoRecibido
}

func (r resultadoLatido) describir() string {
	if r.ok {
		return cGreen("✓") + " " + r.motivo
	}
	return r.motivo
}

// clienteLatido tiene timeout propio y corto.
//
// Sin timeout, http.Client espera para siempre: un cerebro que acepta la conexión y no responde
// —un proxy a medio caer, un tailnet que se degradó— colgaría el bucle entero y la máquina
// figuraría viva por el resto de la eternidad sin volver a latir jamás. El timeout tiene que ser
// menor que el intervalo, o los latidos se apilan.
var clienteLatido = &http.Client{Timeout: 10 * time.Second}

// latir hace UN POST, con la muestra si hay. El cuerpo lleva MEDICIONES y nunca IDENTIDAD: no
// hay ningún campo con el que el dispositivo pueda decir quién es (invariante B4/D5). Quién es
// lo decide el token, del lado del cerebro.
func latir(base, token string, m *fleet.Muestra) resultadoLatido {
	// El cuerpo lleva la muestra y el autorreporte (qué build corre, por dónde se la alcanza).
	// Ni un campo de identidad: quién es lo decide el token, del lado del cerebro.
	carga := map[string]any{"version": version}
	if rid := idRustdeskLocal(); rid != "" {
		carga["rustdesk_id"] = rid
	}
	if m != nil {
		carga["muestra"] = m
	}
	if d := direccionPropia(); d != "" {
		carga["direccion"] = d
	}
	// LA CAPACIDAD DE PREGUNTAR (A57), MEDIDA EN ESTA MÁQUINA. Va SIEMPRE, aunque sea `false`:
	// el campo es opcional en el cuerpo justamente para que un agente VIEJO —que no lo manda— se
	// distinga de uno nuevo que midió y dijo que no. Si este agente se lo saltea cuando no puede,
	// se hace pasar por viejo y el cerebro conserva un valor que ya no es cierto.
	//
	// Y el MOTIVO viaja pegado: sin él, un `pide` endurecido a `prohibido` en toda la flota es un
	// cero sin explicación, y las tres causas —no hay escritorio, falta un paquete, el agente
	// corre como servicio— se arreglan distinto.
	cap := medirCapacidadDeAvisar()
	carga["puede_preguntar"] = cap.Puede
	if !cap.Puede && cap.Motivo != "" {
		carga["motivo_no_preguntar"] = cap.Motivo
	}
	// QUÉ CORRE ADENTRO de esta máquina (S12 · A42). Va con la muestra y no por un camino aparte:
	// el inventario tiene el mismo dueño que la telemetría —el token del dispositivo—, y darle su
	// propia puerta sería un segundo camino de autoridad para el mismo dato.
	//
	// Como todo lo demás del latido, NO LLEVA IDENTIDAD: el reporte dice qué corre, y de quién es
	// la máquina lo decide el token del lado del cerebro (invariante B4/D5).
	if svs := serviciosDelLatido(); len(svs) > 0 {
		carga["servicios"] = svs
	}
	var cuerpo io.Reader
	if b, err := json.Marshal(carga); err == nil {
		cuerpo = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, rutaLatido(base), cuerpo)
	if err != nil {
		return resultadoLatido{motivo: fmt.Sprintf("URL inválida: %v", err)}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := clienteLatido.Do(req)
	if err != nil {
		return resultadoLatido{motivo: fmt.Sprintf("no se pudo alcanzar el cerebro: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		motivo := "latido registrado"
		// El cerebro dice qué hizo con la telemetría. Se imprime para que una capacidad que
		// falta o una muestra rechazada se vean DESDE LA MÁQUINA, en vez de desaparecer en
		// silencio del otro lado.
		var r struct {
			Muestra  string            `json:"muestra"`
			Comandos []comandoRecibido `json:"comandos"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err == nil {
			if r.Muestra != "" {
				motivo += " · muestra " + r.Muestra
			}
			if n := len(r.Comandos); n > 0 {
				motivo += fmt.Sprintf(" · %d comando(s)", n)
			}
		}
		return resultadoLatido{ok: true, motivo: motivo, comandos: r.Comandos}
	case http.StatusUnauthorized:
		return resultadoLatido{revocado: true, motivo: "credencial inválida o revocada"}
	case http.StatusTooManyRequests:
		// El lockout del cerebro. Reintentar rápido lo único que hace es extenderlo, así que
		// esto entra por el camino de backoff como cualquier otro fallo transitorio.
		return resultadoLatido{motivo: "el cerebro aplicó lockout por intentos fallidos"}
	default:
		return resultadoLatido{motivo: fmt.Sprintf("el cerebro respondió %d", resp.StatusCode)}
	}
}

func ayudaAgent() {
	fmt.Println(cBold("musubi agent") + " — late contra el cerebro para que esta máquina figure en la flota")
	fmt.Println()
	fmt.Println(cCyan("Uso:"))
	fmt.Println("  musubi agent [--brain <url>] [--interval <segundos>] [--once]")
	fmt.Println("  musubi agent --revisar-blindaje")
	fmt.Println()
	fmt.Println(cCyan("Entorno:"))
	fmt.Printf("  %s  token del dispositivo (lo devuelve musubi_fleet_enroll, UNA sola vez)\n", cBold(envToken))
	fmt.Printf("  %s     dirección del cerebro, ej http://100.x.y.z:7717\n", cBold(envCerebro))
	fmt.Println()
	fmt.Println(cCyan("Notas:"))
	fmt.Println("  · El token del dispositivo NO sirve para /mcp: no da acceso a la memoria.")
	fmt.Println("  · Si el cerebro responde 401, el dispositivo fue dado de baja y el agente se detiene.")
	fmt.Println("  · Con --once late una vez y sale (para cron o systemd timer).")
	fmt.Println("  · --revisar-blindaje prueba, tocándolas de verdad, las rutas que este agente")
	fmt.Println("    necesita en ESTA máquina, y dice la línea de systemd que falta. No pide token.")
	fmt.Println("    " + cBold("Correlo adentro del confinamiento o no prueba nada") + ": desde tu shell")
	fmt.Println("    va a salir todo en verde porque tu shell no tiene ProtectHome. Así:")
	fmt.Println("      systemd-run -p ProtectHome=read-only -p ProtectSystem=strict \\")
	fmt.Println("        --uid=musubi --pty --wait /usr/local/bin/musubi agent --revisar-blindaje")
	fmt.Println("    Agregale los -p que la unidad tenga de más; los que importan son los que")
	fmt.Println("    tocan el MONTAJE (Protect*, ReadWritePaths, PrivateTmp), que son los que")
	fmt.Println("    hacen que una ruta escribible deje de serlo:")
	fmt.Println("      systemctl cat musubi-agente.service | grep -E 'Protect|ReadWrite|Private'")
}

// tomarMuestra devuelve la telemetría del host, o nil si este sistema operativo no tiene colector
// o si la lectura falló.
//
// nil NO es un error para el llamador: el latido sale igual y el cerebro conserva la última
// muestra buena. Un colector roto no puede hacer desaparecer una máquina del inventario — es
// justo cuando más querés verla.
func tomarMuestra(col fleet.Colector) *fleet.Muestra {
	if col == nil {
		return nil
	}
	m, err := col.Tomar()
	if err != nil {
		return nil
	}
	// LAS SONDAS DE ALCANCE VIAJAN CON LA MUESTRA (A67), no por una puerta propia: son una
	// medición que esta máquina toma de su entorno, con el mismo dueño y la misma frecuencia que
	// la CPU o el disco. Sin destinos configurados devuelve nil y el campo ni aparece.
	m.Alcance = sondearAlcance()
	return &m
}

// direccionPropia devuelve la dirección por la que esta máquina es alcanzable, o "" si no se
// puede determinar.
//
// Se prefiere la del TAILNET (100.64.0.0/10, el rango CGNAT que usa Tailscale) porque es la que
// sirve para alcanzar la máquina desde el cerebro: la IP de la LAN de una oficina no le dice nada
// a nadie fuera de esa oficina, y hay una por cada red a la que el equipo se conecte. Si no hay
// tailnet, se cae a la primera IPv4 no-loopback.
//
// Es informativa: el cerebro NO la usa para autenticar ni para decidir nada. Si fuera de otro
// modo, un device podría mentir sobre su dirección — y por eso mismo esto es sólo un dato de
// inventario que ahorra ir a buscarlo a mano.
func direccionPropia() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	respaldo := ""
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil {
				continue
			}
			// 100.64.0.0/10 — el rango del tailnet.
			if ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127 {
				return ip.String()
			}
			if respaldo == "" {
				respaldo = ip.String()
			}
		}
	}
	return respaldo
}

// atenderComandos ejecuta los pedidos que trajo el latido y reporta cada resultado.
//
// SECUENCIAL a propósito. Correr en paralelo sería más rápido y equivocado: los comandos de una
// misma máquina suelen tener orden (parar un servicio, tocar un archivo, arrancarlo), y el cerebro
// los entrega ordenados por antigüedad justamente porque ese orden importa. Además, N comandos
// pesados en paralelo son exactamente la carga que el agente existe para vigilar.
//
// Se reporta el resultado de CADA UNO antes de pasar al siguiente: si el agente muere a mitad de
// la tanda, lo ya hecho queda registrado.
func atenderComandos(urlBase, token string, comandos []comandoRecibido) {
	for _, c := range comandos {
		fmt.Printf("%s ejecutando: %s\n", cDim("›"), fleet.ResumenArgv(c.Argv))
		res := ejecutar(c, urlBase, token)
		if err := reportar(urlBase, token, res); err != nil {
			// El resultado se pierde, pero NO el registro del pedido: la bitácora tiene la fila
			// desde que se encoló, y el comando queda visible como entregado-sin-terminar, que
			// es información honesta.
			fmt.Fprintf(os.Stderr, "%s no se pudo reportar el resultado de %s: %v\n", cYellow("!"), c.ID, err)
			continue
		}
		switch {
		case res.Error != "":
			fmt.Printf("  %s %s\n", cYellow("!"), res.Error)
		case res.ExitCode != nil && *res.ExitCode != 0:
			fmt.Printf("  %s exit %d\n", cYellow("!"), *res.ExitCode)
		default:
			fmt.Printf("  %s exit 0\n", cGreen("✓"))
		}
	}
}

// rutaLatido y rutaResultado derivan las dos rutas de la BASE del cerebro. Existen para que el
// agente nunca tenga que acordarse de cuál URL le tocaba: se pasa la base y cada llamada arma la
// suya. Se toleran las dos formas de base —con y sin la ruta ya pegada— porque un operador que
// setea MUSUBI_BRAIN_URL con `/fleet/heartbeat` al final no debería quedarse sin exec por eso.
func rutaLatido(base string) string { return normalizarBase(base) + "/fleet/heartbeat" }

func rutaResultado(base string) string { return normalizarBase(base) + "/fleet/result" }

func normalizarBase(base string) string {
	base = strings.TrimSuffix(base, "/")
	for _, sufijo := range []string{"/fleet/heartbeat", "/fleet/result", "/mcp"} {
		base = strings.TrimSuffix(base, sufijo)
	}
	return strings.TrimSuffix(base, "/")
}
