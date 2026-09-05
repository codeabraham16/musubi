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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
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
	// red volvió.
	//
	// ────────────────────────────────────────────────────────────────────────────────────────
	// POR QUÉ 2 MINUTOS Y NO LOS 5 QUE HABÍA
	//
	// El techo no es «cuánto tarda en reaparecer una máquina», es cuánto tarda en reaparecer
	// LA FLOTA ENTERA cuando el cerebro vuelve — y la flota entera se cae junta cada vez que se
	// redespliega el cerebro, que hoy PARA los servicios. Con 2000 agentes, los que llegaron al
	// escalón más alto tardan el techo entero en enterarse de que el cerebro volvió, y mientras
	// tanto figuran caídos.
	//
	// El presupuesto lo fija `MaquinaCaida` (deploy/musubi-alerts-flota.yml): `for: 5m` sobre
	// `up == 0`, y `up` lo pone en cero el cerebro a los 90 s de silencio (umbralEnLineaDefault).
	// La alerta suena si (corte) + (lo que le queda de sueño al agente) − 90 s ≥ 5 min. El sueño
	// que le queda es, como mucho, el techo CON su jitter (2 min × 1,2 = 2 min 24 s), así que el
	// techo tiene que cumplir:
	//
	//     techo × 1,2  <  5 min − 90 s  =  3 min 30 s
	//
	// Con 5 min no se cumplía: un corte de apenas 90 s alcanzaba para que la alerta sonara en
	// toda la flota, y una vuelta del cerebro dejaba `up=0` hasta ~6,5 min. Con 2 min hace falta
	// un corte de más de 4 min — y a los 4 min la alerta ya dice la verdad. Lo custodia
	// TestElBackoffTieneTecho: si alguien mueve el `for` o el umbral, hay que volver a hacer esta
	// cuenta.
	// ────────────────────────────────────────────────────────────────────────────────────────
	esperaMinima = 5 * time.Second
	esperaMaxima = 2 * time.Minute

	// jitterDeEspera es la dispersión de cada espera del backoff: ±20 %.
	//
	// Sin jitter el backoff es determinista, y determinista es SINCRONIZADO: los 2000 agentes
	// que vieron caer al cerebro en la misma ventana calculan la misma tabla de reintentos y
	// vuelven a golpear la puerta EN EL MISMO SEGUNDO, escalón tras escalón. Eso es una
	// estampida, y contra un cerebro que recién levanta es la forma más segura de volver a
	// tirarlo. El 20 % desparrama cada escalón en una ventana de ±20 % de su largo: en el techo,
	// casi un minuto entero.
	//
	// El intervalo SANO (intervaloLatidoDefault) NO lleva jitter a propósito: está atado al
	// umbral del cerebro por el factor 3, y un +20 % lo convierte en 36 s — tres latidos son
	// 108 s, más que el umbral, y perder DOS ya pinta la máquina de rojo. La dispersión del
	// tráfico sano la da el desfase de arranque, que es de una sola vez.
	jitterDeEspera = 0.20

	// desfaseDeArranqueMaximo es cuánto puede demorar el PRIMER latido al arrancar: 0 a 30 s.
	//
	// Es la otra mitad de la estampida. Un despliegue masivo —o un apagón que vuelve— arranca
	// los 2000 agentes en la misma ventana, y sin desfase el primer latido de todos cae en el
	// mismo segundo, y el segundo, y el tercero: el intervalo es fijo, así que la sincronía del
	// arranque se conserva PARA SIEMPRE. 30 s es un intervalo entero: desparrama la flota sobre
	// todo el ciclo, y ninguna máquina tarda más de lo que ya tarda en latir en figurar viva.
	//
	// NO se aplica con --once: ahí el punto es verificar la instalación y salir, y 30 s de
	// espera se leen como «se colgó».
	desfaseDeArranqueMaximo = 30 * time.Second

	// envToken y envCerebro son de dónde salen las credenciales. Variables de entorno y no un
	// archivo: es lo que ya usan connect-brain-{linux,windows} para el token del cerebro, y
	// mantener UNA forma de configurar la máquina es más importante que la comodidad de un flag.
	envToken   = "MUSUBI_DEVICE_TOKEN"
	envCerebro = "MUSUBI_BRAIN_URL"

	// envTokenFile es el archivo del que sale el token, y es lo que HABILITA la rotación en
	// caliente (Ola 2): una variable de entorno no se puede reescribir desde adentro del proceso,
	// así que un agente que sólo la tiene no puede adoptar el token nuevo que el cerebro le
	// ofrece. El llavero y su porqué están en agent_token.go.
	//
	// Y de paso saca la credencial de la ENV. El unit hacía
	// `MUSUBI_DEVICE_TOKEN=$(cat .../token) exec musubi agent`, así que el token quedaba en
	// /proc/<pid>/environ —legible por cualquier proceso del mismo usuario— y en la línea del
	// unit. Leyéndolo del archivo se va el wrapper y se va esa exposición.
	envTokenFile = "MUSUBI_DEVICE_TOKEN_FILE"

	// envNombreTLS existe por un choque de dos cosas que las dos son ciertas (Ola 0 del plan
	// empresa, 2026-09-03).
	//
	// El cerebro puede servir HTTPS con un certificado de `tailscale cert`, que Let's Encrypt
	// emite para el NOMBRE del nodo en la malla (`musubi-server.tail89e295.ts.net`) y para
	// ningún otro. Pero los agentes laten contra la IP del tailnet a propósito: con NordVPN
	// activo el DNS de la malla NO resuelve los nombres MagicDNS, y eso está escrito en
	// `deploy/README.md` porque costó encontrarlo.
	//
	// Las dos cosas juntas son un certificado que no valida: se disca una IP y el certificado
	// dice un nombre. La salida NO es apagar la verificación —eso convierte el TLS en teatro y
	// deja pasar a cualquiera que se meta en el medio—: es discar la IP y verificar el
	// certificado contra el nombre, que es exactamente para lo que existe ServerName.
	//
	// Vacío ⇒ comportamiento de siempre: el nombre sale de la URL. Sólo hace falta cuando la
	// URL trae una IP y el certificado trae un nombre.
	envNombreTLS = "MUSUBI_BRAIN_TLS_NAME"
)

// runAgent es el punto de entrada de `musubi agent`.
func runAgent(args []string) {
	cerebro := strings.TrimSpace(os.Getenv(envCerebro))
	cred, errCred := cargarCredencial()
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

	// UN ARCHIVO QUE ESTÁ Y NO SE PUEDE LEER NO ES LO MISMO QUE NO TENER CREDENCIAL, y decir
	// «falta el token» ahí manda a alguien a re-enrolar una máquina que sólo tiene un permiso mal.
	if errCred != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo cargar la credencial del dispositivo.\n", cYellow("✗"))
		fmt.Fprintf(os.Stderr, "  %v\n", errCred)
		os.Exit(1)
	}
	if cred == nil {
		fmt.Fprintf(os.Stderr, "%s falta la credencial del dispositivo.\n", cYellow("✗"))
		fmt.Fprintf(os.Stderr, "  Seteá %s con el archivo que la contiene, o %s con el token.\n", cBold(envTokenFile), cBold(envToken))
		fmt.Fprintf(os.Stderr, "  El token lo devuelve musubi_fleet_enroll UNA sola vez: si lo perdiste, hay que revocar y volver a enrolar.\n")
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
		res := latir(base, cred.Usar(), tomarMuestra(col))
		fmt.Println(res.describir())
		atenderComandos(base, cred.Actual(), res.comandos)
		// CON --once TAMBIÉN SE GUARDA UNA ROTACIÓN OFRECIDA. Es un solo latido, así que no llega
		// a estrenar el token nuevo —eso lo hará la próxima corrida—, pero dejarlo caer haría que
		// una máquina que late por timer no pueda rotar nunca.
		if res.ok {
			if err := cred.Sumar(res.tokenNuevo); err != nil {
				fmt.Fprintf(os.Stderr, "%s %v\n", cYellow("!"), err)
			}
		}
		if !res.ok {
			os.Exit(1)
		}
		return
	}

	// Un agente que vuelve NO hereda una sesión de pantalla de su encarnación anterior: si murió
	// con una contraseña puesta, se llevó el temporizador y la contraseña quedó.
	//
	// ACÁ HABÍA UN `marcarSesionAbierta(true)` Y ERA EL DEFECTO MÁS CARO DEL PLANO VISUAL. Forzaba
	// el booleano para que el cierre de abajo corriera «por las dudas», y el cierre de entonces
	// tenía un solo camino: reemplazar la contraseña PERMANENTE de RustDesk por una al azar. O
	// sea que CADA ARRANQUE DEL AGENTE —un reinicio, una actualización, un reboot de la máquina—
	// destruía la contraseña que había puesto el dueño, hubiera existido una sesión o no. Se
	// reportó como «RustDesk me cambia la contraseña solo», que es exactamente lo que pasaba.
	//
	// Ahora no se fuerza nada: cerrarSesionPantalla lee la marca en disco y cierra SI Y SÓLO SI
	// de verdad quedó una sesión abierta, devolviendo lo que había. Sin marca, no toca nada.
	cerrarSesionColgadaDeArranque()

	desfase := desfaseDeArranque()
	fmt.Printf("%s agente activo · cerebro %s · latido cada %s · el primero en %s\n",
		cGreen("▶"), cBold(cerebro), intervalo, desfase.Round(time.Second))
	if _, err := col.Tomar(); err != nil {
		// D4 — sin colector para este OS, se dice UNA vez al arrancar y el agente sigue latiendo.
		// Estar viva es información útil aunque no se pueda medir cómo está.
		fmt.Printf("%s sin telemetría: %v\n", cYellow("!"), err)
	}
	bucleDeLatidos(base, cred, intervalo, desfase, col)
}

// nuevoTimer es el seam del reloj del bucle, por la misma razón que azarDelAgente: para que una
// prueba pueda ver QUÉ ESPERA se pidió sin tener que esperarla.
var nuevoTimer = time.NewTimer

// azarDelAgente es la ÚNICA fuente de aleatoriedad del agente: un número en [0, 1).
//
// Es `var` por la misma razón que enumerarServicios: para que las pruebas lo claven y el jitter
// y el desfase se vuelvan deterministas. Una prueba que mide «está dentro de ±20 %» contra un
// generador real pasa casi siempre, y «casi siempre» en CI es un flaky con nombre propio.
var azarDelAgente = rand.Float64

// conJitter desparrama una espera en ±jitterDeEspera de su largo. La base (`espera`) queda
// intacta en el llamador: el jitter se aplica a lo que se duerme, no a lo que se duplica, o el
// azar se acumularía escalón a escalón y el techo dejaría de ser un techo.
func conJitter(espera time.Duration) time.Duration {
	factor := 1 - jitterDeEspera + 2*jitterDeEspera*azarDelAgente()
	return time.Duration(float64(espera) * factor)
}

// siguienteEspera es el escalón que sigue en el backoff: el doble, acotado por esperaMaxima.
func siguienteEspera(espera time.Duration) time.Duration {
	if espera *= 2; espera > esperaMaxima {
		espera = esperaMaxima
	}
	return espera
}

// desfaseDeArranque es cuánto espera el agente antes del PRIMER latido: [0, desfaseDeArranqueMaximo).
func desfaseDeArranque() time.Duration {
	return time.Duration(azarDelAgente() * float64(desfaseDeArranqueMaximo))
}

// bucleDeLatidos late hasta que lo maten o hasta que el cerebro diga que este dispositivo ya no
// pertenece a la flota.
//
// Escucha SIGINT/SIGTERM: un agente que ignora la señal de apagado deja al systemd de cada máquina
// esperando el timeout de kill, y eso se nota en el apagado de un servidor.
//
// `desfase` es cuánto tarda el PRIMER latido. Se recibe y no se sortea acá adentro para que el
// desfase quede bajo el mismo select que las señales —un `systemctl stop` durante esos 30 s
// tiene que cortar en el acto, no al final del sueño— y para que las pruebas del bucle pasen 0.
func bucleDeLatidos(base string, cred *credencial, intervalo, desfase time.Duration, col fleet.Colector) {
	señales := make(chan os.Signal, 1)
	signal.Notify(señales, os.Interrupt, syscall.SIGTERM)

	espera := esperaMinima
	// EL TIMER SALE DE UN SEAM, Y NO ES CEREMONIA: ES LA ÚNICA MANERA DE PROBAR ESTO.
	//
	// La primera versión de la prueba medía el reloj de pared —«el primer latido tardó al menos
	// el desfase»— y era HUECA: el arranque del agente gasta ~2,4 s antes del primer POST
	// (idRustdeskLocal y direccionPropia salen a preguntarle cosas al sistema), así que cualquier
	// umbral chico se cumple solo, con desfase o sin él. Se midió: con `NewTimer(0)` el primer
	// latido llegó igual a los 2,37 s contra un umbral de 150 ms, y la prueba pasaba en verde.
	//
	// Subir el umbral por encima del ruido haría la prueba lenta y flaky en CI. Mirar la
	// DURACIÓN QUE SE PIDE en vez de la que se sufre la vuelve exacta y de microsegundos.
	tick := nuevoTimer(desfase) // sin desfase, el primer latido sale ya: no espera un intervalo entero
	defer tick.Stop()

	for {
		select {
		case <-señales:
			fmt.Printf("\n%s agente detenido\n", cDim("■"))
			return
		case <-tick.C:
		}

		res := latir(base, cred.Usar(), tomarMuestra(col))
		switch {
		case res.revocado:
			// ANTES DE DARSE DE BAJA SE PRUEBA EL OTRO TOKEN DEL LLAVERO, si el archivo tenía dos.
			// Es el camino que rescata a la máquina que se cortó justo después de que el cerebro
			// promoviera la rotación: el token viejo ya murió y el nuevo está en disco, sin
			// estrenar. Sin esto ese caso es un 401 eterno y una visita a la máquina.
			//
			// NO afloja el kill-switch ni golpea el lockout: revocar borra los DOS hashes, así que
			// los dos dan 401 y se cae al return de abajo. Es un intento por token que el archivo
			// YA tenía, nunca un reintento del mismo.
			if cred.Rechazado() {
				fmt.Printf("%s la credencial en uso fue rechazada; se prueba la otra del llavero.\n", cYellow("!"))
				tick.Reset(0)
				continue
			}
			// B5 — el kill-switch tiene que ser entendible DESDE LA MÁQUINA. Un agente que no
			// interpreta el 401 obliga a ir a apagarlo a mano, que es exactamente lo que no se
			// puede hacer con un equipo remoto.
			fmt.Printf("%s el cerebro rechazó la credencial: este dispositivo fue dado de baja.\n", cYellow("!"))
			fmt.Printf("  %s\n", cDim(res.motivo))
			fmt.Printf("%s agente detenido (no se reintenta: reintentar sería golpear el lockout del cerebro).\n", cDim("■"))
			return
		case res.ok:
			espera = esperaMinima // la red volvió: se resetea el backoff
			// EL COLAPSO VA ANTES DEL SUMAR, y en ese orden: el token que acaba de servir queda
			// solo en el archivo, y recién ahí se agrega el de una rotación nueva. Al revés, el
			// colapso se llevaría el que se acaba de guardar.
			if err := cred.Funciono(); err != nil {
				fmt.Fprintf(os.Stderr, "%s no se pudo dejar sólo la credencial en uso: %v\n", cYellow("!"), err)
			}
			if err := cred.Sumar(res.tokenNuevo); err != nil {
				// Se DICE y no se traga: una rotación que no se puede adoptar la abandona el
				// cerebro al vencer, y sin este aviso nadie sabría por qué nunca se completó.
				fmt.Fprintf(os.Stderr, "%s %v\n", cYellow("!"), err)
			}
			atenderComandos(base, cred.Actual(), res.comandos)
			tick.Reset(intervalo)
		default:
			// B7 — el cerebro caído NO mata al agente. Backoff exponencial acotado, y con jitter:
			// lo que se duerme es el escalón desparramado; lo que se duplica es el escalón limpio.
			demora := conJitter(espera)
			fmt.Fprintf(os.Stderr, "%s %s · reintento en %s\n", cYellow("!"), res.describir(), demora.Round(time.Second))
			tick.Reset(demora)
			espera = siguienteEspera(espera)
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
	// tokenNuevo es la credencial de una rotación en curso, si hay una. Vacío es lo normal.
	tokenNuevo string
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
var clienteLatido = clienteParaElCerebro(os.Getenv(envNombreTLS))

// clienteParaElCerebro arma el cliente del latido, con el nombre contra el que verificar el
// certificado si hace falta declararlo aparte de la URL.
//
// SÓLO toca ServerName. No apaga la verificación, no cambia el pool de raíces y no baja el piso
// de versión: un cliente que "arregla" el TLS relajándolo es peor que no tener TLS, porque el
// candado del panel dice que está seguro. Con `nombre` vacío devuelve el cliente de siempre,
// sin Transport propio, para que el default del stdlib siga siendo el default.
func clienteParaElCerebro(nombre string) *http.Client {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return &http.Client{Timeout: 10 * time.Second}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.ServerName = nombre
	tr.TLSClientConfig.MinVersion = tls.VersionTLS12
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

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
	//
	// VA AUNQUE ESTÉ VACÍA, y `confirmar` se llama recién cuando el cerebro aceptó (A78). El
	// `len(svs) > 0` que había acá se contradecía en silencio con el sellado de adentro: una
	// lista vacía se daba por enviada y no se enviaba, para siempre.
	svs, mandarInventario, confirmarInventario := serviciosDelLatido()
	if mandarInventario {
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
		// SE DECODIFICA EL TIPO DEL CONTRATO, no un struct escrito acá. El struct anónimo que
		// estaba en su lugar tenía sólo `muestra` y `comandos`, y encoding/json descarta en
		// silencio lo que el receptor no declara: así se perdieron `token_nuevo` —la rotación de
		// la Ola 2 no podía completarse— y `servicios`, cuyo único propósito era que un inventario
		// descartado NO desapareciera en silencio. Con el tipo compartido, un campo nuevo del lado
		// del cerebro llega acá sin que nadie se acuerde de copiarlo.
		var r fleet.RespuestaLatido
		if err := json.NewDecoder(resp.Body).Decode(&r); err == nil {
			if r.Muestra != "" {
				motivo += " · muestra " + r.Muestra
			}
			// `servicios` se imprime por el mismo motivo que `muestra`, y es la razón por la
			// que el cerebro lo manda: quien administra ESTA máquina no ve los logs del cerebro,
			// así que un inventario rechazado tiene que verse acá o no se ve en ningún lado.
			if r.Servicios != "" {
				motivo += " · servicios " + r.Servicios
			}
			if n := len(r.Comandos); n > 0 {
				motivo += fmt.Sprintf(" · %d comando(s)", n)
			}
		}
		// EL SELLO DEL INVENTARIO VA ACÁ Y EN NINGÚN OTRO LADO: es el único punto del programa
		// donde consta que el cerebro se llevó la lista. Sellar al armarla —que es lo que hacía
		// antes— es creerle al remitente en vez de al receptor.
		if confirmarInventario != nil {
			confirmarInventario()
		}
		return resultadoLatido{ok: true, motivo: motivo, comandos: r.Comandos, tokenNuevo: r.TokenNuevo}
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
	fmt.Printf("  %s  archivo con el token del dispositivo. ES EL RECOMENDADO: es el único\n", cBold("MUSUBI_DEVICE_TOKEN_FILE"))
	fmt.Printf("                            de los dos con el que se puede ROTAR la credencial en\n")
	fmt.Printf("                            caliente, y deja el token fuera de /proc/<pid>/environ.\n")
	fmt.Printf("  %s       el token en la variable. Late igual, pero no puede adoptar una\n", cBold("MUSUBI_DEVICE_TOKEN"))
	fmt.Printf("                            rotación: un proceso no puede reescribir su propio entorno.\n")
	fmt.Printf("  %s          dirección del cerebro, ej http://100.x.y.z:7717\n", cBold("MUSUBI_BRAIN_URL"))
	fmt.Printf("  %s     nombre contra el que verificar el certificado, si la URL trae una\n", cBold("MUSUBI_BRAIN_TLS_NAME"))
	fmt.Printf("                            IP (ej: musubi-server.tail89e295.ts.net). Vacío: sale de la URL.\n")
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
