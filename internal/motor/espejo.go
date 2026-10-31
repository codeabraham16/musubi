package motor

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL MCP ESPEJO: CÓMO SE LE DECLARAN AL CLI LAS TOOLS DEL CLIENTE.
//
// El Lanzador existe para facturar contra la SUSCRIPCIÓN, y sin esto sólo sirve para turnos sin
// tools — o sea, para ninguno de los reales: el transcript de pi siempre trae tools plegadas en
// sus mensajes `system`, y un modelo sin tools declaradas no tiene nada que pedir. El
// `ClienteAPI` no tiene el problema porque las manda en el request; el CLI NO las toma por
// bandera, las toma por MCP.
//
// Lo verificado contra el binario local (CLI 2.1.272, `claude --help`):
//
//	--mcp-config <configs...>   «Load MCP servers from JSON files OR STRINGS»
//	--strict-mcp-config         «Only use MCP servers from --mcp-config, ignoring all other»
//	mcp add -t <transport>      «Transport type (stdio, sse, http)»
//
// Las tres juntas son lo que hace que este archivo sea corto: el JSON lo declara el motor —así que
// no hace falta un servidor MCP instalado en la máquina—, el hijo no hereda el `.mcp.json` del
// proyecto —que es el que haría a Musubi recursar sobre sí mismo— y el transporte HTTP permite
// que el espejo viva DENTRO de este proceso, sin un segundo binario que mantener.
//
// ⚠️ ESTE PÁRRAFO DECÍA QUE EL JSON VA «INLINE» Y QUE ASÍ «nunca toca el disco del usuario». Las
// dos mitades envejecieron juntas y conviene saber por qué: la de los «JSON files OR strings»
// sigue siendo cierta, pero la que importaba no era el esquema de tools sino LA URL DEL ESPEJO,
// que lleva el token del turno. Inline, esa credencial viajaba en el argv del hijo, y el argv no
// es privado: en Linux `/proc/<pid>/cmdline` es modo 444 y lo lee cualquier usuario de la
// máquina. Hoy va por un archivo 0600 que se borra al cerrar — ver escribirConfig.
//
// POR QUÉ ACÁ Y NO EN internal/mcp, que ya tiene un servidor MCP entero. Porque ese paquete
// IMPORTA a este: colgar el espejo ahí haría el ciclo. El precio es un JSON-RPC mínimo
// —`initialize`, `tools/list`, `tools/call`— y se paga con que el ciclo de vida del espejo sea
// exactamente el del turno: nace con él, muere con él, y dos turnos concurrentes no comparten
// ni un puerto ni un mapa.
// ────────────────────────────────────────────────────────────────────────────────────────────

// nombreDelEspejo es el nombre del servidor MCP con el que viajan las tools del cliente, y es
// CORTO por aritmética, no por gusto: el CLI re-expone lo que viene por MCP como
// `mcp__<servidor>__<tool>` y el tope de un nombre de tool son 64 caracteres. Con `pi` el
// prefijo cuesta 9; con `cliente` costaría 14, y una tool de 51 caracteres del cliente pasaría a
// ser inválida sin que nadie haya escrito nada mal.
const nombreDelEspejo = "pi"

// prefijoDelEspejo es el renombre que hace el CLI, y hay que DESHACERLO en el camino de vuelta:
// el `tool_use.name` que sale en el stream no es el nombre que el cliente declaró, y el loop del
// cliente busca por el suyo. Es el ÚNICO renombre de la VUELTA, y el más traicionero de todos
// porque no falla: el cliente recibe un `tool_use` con un nombre que no tiene en su tabla y lo
// trata como una tool desconocida.
//
// ⚠️ ACÁ DECÍA «es el cuarto renombre del mismo viaje —después de `arguments`/`input`,
// `parameters`/`input_schema` y `thinkingSignature`/`signature`—», y era falso dos veces:
// contaba tres de los CUATRO de la ida (falta `mimeType`/`media_type`) y los metía en un viaje
// que va al revés. Ésos los hace el traductor de entrada sobre lo que MANDA el cliente; éste lo
// hace el CLI sobre lo que VUELVE del modelo. Los cuatro se enumeran UNA vez, en
// entrada.go:23-25, y el nudo entero vive en specs/motor-proxy-stream/design.md. No se repiten
// acá: repetirlos es exactamente cómo esta frase llegó a estar mal en cuatro lugares a la vez.
//
// Es el mismo formato que este repo ya parsea en cmd/musubi/uso_agente.go.
const prefijoDelEspejo = "mcp__" + nombreDelEspejo + "__"

// topeDelNombreDeTool es el largo máximo que la API acepta en el nombre de una tool. Se valida
// ACÁ, al abrir el espejo, y no se deja para que lo rechace el modelo: el error del otro lado
// hablaría de un nombre que el cliente nunca escribió, porque el que se pasa de largo es el
// nombre PREFIJADO.
const topeDelNombreDeTool = 64

// esperaDelCierreDelEspejo acota el apagado. Un turno ya terminado no se queda esperando a que
// el hijo suelte una conexión que de todos modos nadie va a leer.
const esperaDelCierreDelEspejo = 2 * time.Second

// espejo es el servidor MCP efímero de UN turno.
type espejo struct {
	// url es lo que se le pasa al hijo, con el token ya adentro.
	url string
	// ruta es el camino secreto. Todo lo demás da 404.
	ruta string
	// tools es la respuesta de `tools/list` ya serializada: se arma una vez, al abrir, porque
	// el set de tools de un turno no cambia a mitad del turno.
	tools json.RawMessage
	// rutaConfig es el archivo que `--mcp-config` recibe, y existe para que la URL de arriba
	// —que lleva el token adentro— NO viaje en el argv del hijo. Ver escribirConfig.
	rutaConfig string
	// dirConfig es el directorio temporal que contiene a rutaConfig. Se guarda aparte porque
	// lo que se borra al cerrar es el directorio entero, no sólo el archivo.
	dirConfig string
	// llamadas cuenta los `tools/call` que llegaron, que en el diseño correcto son CERO. Ver
	// EjecucionesIntentadas.
	llamadas atomic.Int64
	// listados cuenta los `tools/list` servidos, y mide el fallo CONTRARIO al de arriba: en el
	// diseño correcto es ≥1, porque un turno con tools abre el espejo justamente para que el
	// hijo lea el roster. Ver ListadosServidos.
	listados atomic.Int64

	srv *http.Server
}

// margenDelIdle es cuánto le sobrevive el listener del espejo al techo del turno. Existe para que
// el que corte un turno eterno sea el reloj del LANZADOR —que devuelve un error que el cliente
// entiende— y no este listener, que al hijo se le ve como un servidor MCP caído.
//
// UN EPSILON NO ALCANZA, y por eso su test pide un piso PROPIO en vez de repetir esta constante:
// una aserción que la nombrara se movería con ella y el margen volvería a no medirlo nadie. Es
// exactamente lo que pasaba: la única aserción pedía `IdleTimeout > DemoraPorDefecto`, así que
// bajar el minuto a un NANOSEGUNDO dejaba la suite entera en verde (medido el 2026-10-29 con
// `go test -overlay`: exit 0, cero `--- FAIL`).
const margenDelIdle = time.Minute

// abrirEspejo levanta el servidor con las tools del pedido ya traducidas, y devuelve nil cuando
// el pedido no trae ninguna: un turno sin tools no necesita espejo y abrir uno igual sería un
// listener y un `--mcp-config` de más en cada turno pelado.
//
// `espera` ES EL PLAZO REAL DEL TURNO, no el default del motor, y de ahí sale el `IdleTimeout`
// del listener. Colgarlo de la constante era el defecto: `Lanzador.Espera` viene de
// `turn_timeout_seconds` del yaml ([factory.go](factory.go)), así que con 700 s el idle de 660 s
// cortaba ANTES que el reloj del lanzador — justo lo que el comentario de abajo promete que no
// pasa. Cero ⇒ DemoraPorDefecto, el mismo acuerdo que `Lanzador.Espera`.
//
// Y EL OTRO BORDE LO CIERRA LA FÁBRICA, NO ESTA FUNCIÓN. La suma de abajo desborda con un
// `turn_timeout_seconds` cerca de 2⁶³ ns, y acá llega tarde: el desborde ya ocurrió y el valor es
// indistinguible de un plazo legítimo. Por eso `Desde` RECHAZA lo que pasa `TopeDelPlazo` —ver su
// doc, que tiene los dos modos con sus números—, y por eso este comentario ya no dice «nadie
// acota el yaml»: alguien lo acota, y es de lo que depende la promesa de abajo.
//
// FALLA EN VEZ DE DEGRADAR, y es deliberado. Si el listener no abre, la alternativa sería correr
// el turno sin tools: el modelo contestaría algo —no un error— sobre una conversación en la que
// no puede hacer nada, y el cliente lo vería como un agente que dejó de usar sus herramientas.
// Un error acá lo ve el cliente como el `error` del protocolo, que es lo que es.
func abrirEspejo(p Pedido, espera time.Duration) (*espejo, error) {
	if len(bytes.TrimSpace(p.Tools)) == 0 {
		return nil, nil
	}
	// UN CERO NO PUEDE CAER AL `espera + margenDelIdle` DE ABAJO: daría un idle de un minuto
	// pelado, que es PEOR que el defecto que este parámetro arregla — le cortaría el keep-alive
	// al hijo al minuto de estar pensando, que es el caso normal de un turno largo.
	espera = plazoDelTurno(espera)
	tools, err := toolsParaElEspejo(p.Tools)
	if err != nil {
		return nil, err
	}
	tok, err := tokenDelEspejo()
	if err != nil {
		return nil, err
	}

	// 127.0.0.1 Y NO :0 A SECAS: un listener en todas las interfaces publicaría las
	// definiciones de tools del cliente en la red. El puerto sí es 0 —lo elige el sistema—
	// porque un puerto fijo es un puerto que dos turnos concurrentes se pelean.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("abrir el espejo de tools: %w", err)
	}

	e := &espejo{ruta: "/" + tok, tools: tools}
	e.url = "http://" + ln.Addr().String() + e.ruta
	// El listener ya está abierto, así que si esto falla hay que soltarlo a mano: todavía no
	// existe el `srv` al que `cerrar` le haría Shutdown, y un listener huérfano se queda con el
	// puerto hasta que muera el proceso.
	if err := e.escribirConfig(); err != nil {
		_ = ln.Close()
		return nil, err
	}
	e.srv = &http.Server{
		Handler: e,
		// LOS CUATRO TIMEOUTS, Y NINGUNO SOBRA. Acá había un `ReadHeaderTimeout` solo, con un
		// comentario que decía que acotaba «la única forma de colgar esto». Era falso: dejaba
		// sin deadline el CUERPO del request y la espera entre requests keep-alive.
		//
		// ⚠️ Y PONER SÓLO `ReadTimeout` HABRÍA SIDO PEOR QUE NO TOCAR NADA. Con `IdleTimeout`
		// en cero, net/http usa el valor de `ReadTimeout` para la espera del próximo request
		// en una conexión viva, así que un `ReadTimeout` corto le cortaría al hijo la conexión
		// keep-alive mientras el modelo piensa — y pensar es lo que el hijo hace casi todo el
		// turno. Por eso van los dos, y con escalas deliberadamente distintas.
		ReadHeaderTimeout: 10 * time.Second,
		// El request ENTERO, cuerpo incluido. El tope de 1 MiB de `responder` acota el TAMAÑO
		// y no el tiempo: sin esto, un byte por minuto mantiene la goroutine viva para
		// siempre. Desde loopback, 15 s para 1 MiB es holgadísimo.
		ReadTimeout: 15 * time.Second,
		// La espera del PRÓXIMO request en una conexión ya abierta. Cuelga del plazo REAL del
		// turno para que las dos no se despeguen: el hijo tiene su conexión abierta todo el
		// turno, y el silencio entre dos `tools/call` es el modelo pensando, no una conexión
		// abandonada. El margen de más es para que el techo del turno gane SIEMPRE: el que
		// tiene que cortar un turno eterno es el reloj del lanzador, que devuelve un error que
		// el cliente entiende, no un timeout de este listener.
		//
		// 🔴 Y «SIEMPRE» ERA FALSO MIENTRAS ESTO COLGABA DE `DemoraPorDefecto`. Ver la doc de
		// abrirEspejo: el plazo del turno sale del yaml, así que la constante y el plazo real
		// se despegaban y con 700 s el orden de los relojes quedaba invertido. Colgarlo del
		// plazo real arregló ese orden.
		//
		// ⚠️ PERO «CON CUALQUIER VALOR» TAMBIÉN ERA FALSO, y es la otra mitad: ESTA SUMA
		// DESBORDA. Con `turn_timeout_seconds` entre 9223371977 y 9223372036 el IdleTimeout
		// queda NEGATIVO, y net/http no lee un negativo como «ya vencido» sino como «sin
		// deadline» (`if d := c.server.idleTimeout(); d > 0`, server.go:2113): el listener no
		// vence nunca, que es lo contrario exacto de lo que promete el párrafo de arriba. Hoy
		// la promesa vale porque `Desde` ACOTA el yaml y rechaza lo que pasa `TopeDelPlazo`
		// —cuya doc tiene los dos modos de desborde—, no porque la suma sea segura por sí sola.
		IdleTimeout: espera + margenDelIdle,
		// Escribir la respuesta. El hijo es local y lo que sale es un handshake o un
		// tools/list, así que esto sólo acota al cliente que pide y después no lee.
		WriteTimeout: 30 * time.Second,
	}
	// El error de Serve se descarta porque el único camino normal es ErrServerClosed, que es lo
	// que devuelve `cerrar`. Un error real deja al hijo sin tools, y eso ya se ve: el turno
	// vuelve sin haber pedido ninguna.
	go func() { _ = e.srv.Serve(ln) }()
	return e, nil
}

// cerrar apaga el espejo. Tolera el nil para que el llamador pueda hacer `defer esp.cerrar()`
// sin preguntar si el pedido traía tools.
func (e *espejo) cerrar() {
	if e == nil {
		return
	}
	// El archivo de config se borra ANTES del Shutdown y sin esperar su resultado: contiene la
	// credencial de este turno y el hijo ya lo leyó al arrancar. Si el Shutdown se demora los
	// 2 s del timeout, el secreto no tiene por qué quedarse en el disco esos 2 s.
	if e.dirConfig != "" {
		_ = os.RemoveAll(e.dirConfig)
	}
	ctx, cancelar := context.WithTimeout(context.Background(), esperaDelCierreDelEspejo)
	defer cancelar()
	// Shutdown y no Close: el hijo puede estar leyendo la respuesta de un tools/list en el
	// momento del cierre, y cortarle la conexión le daría un error de MCP en vez de un turno
	// terminado.
	if err := e.srv.Shutdown(ctx); err != nil {
		// 🔴 UN SHUTDOWN QUE EXPIRA NO CERRÓ NADA, y creerle es cómo se fuga una goroutine por
		// turno. Shutdown cierra los listeners y las conexiones OCIOSAS, pero a las que están
		// a mitad de un request sólo las ESPERA: si el contexto vence, devuelve su error y las
		// deja vivas, con su goroutine y su descriptor, dentro de un `musubi serve` que no se
		// muere nunca. `Close` sí las corta.
		//
		// ⚠️ ACÁ DECÍA «hasta que muera el proceso», y desde que el `srv` de arriba tiene los
		// cuatro timeouts eso es FALSO: medido con este `Close` saboteado, la conexión muere a
		// los 15 s por el `ReadTimeout`. Lo que el `Close` compra es que se corte en los 2 s
		// del contexto en vez de en 15, y cubrir el lado de ESCRITURA, al que ningún plazo de
		// lectura llega. Es un mérito real, pero no el que la frase vieja describía — y el
		// mismo PR que la escribió es el que puso los timeouts que la volvieron falsa.
		//
		// NO HACE FALTA LA CREDENCIAL PARA PROVOCARLO, y eso es lo que lo vuelve del lado de
		// la seguridad y no de la prolijidad: el deadline de lectura corre ANTES de que el
		// handler mire la ruta, así que la ruta-secreta no defiende de esto. Cualquier proceso
		// local puede abrir una conexión y no terminar de mandar el request.
		//
		// Es seguro cortar acá: si se llegó a este `if`, el turno ya terminó y el hijo ya está
		// muerto (lo mata el lanzador), así que no hay un tools/list a medio servir que
		// proteger — para eso se intentó el Shutdown primero.
		_ = e.srv.Close()
	}
}

// escribirConfig deja el JSON de `--mcp-config` en un archivo propio, con la credencial
// adentro, y guarda su ruta en el espejo.
//
// 🔴 POR QUÉ UN ARCHIVO Y NO EL ARGV. `e.url` lleva el token del turno, y antes viajaba como
// el VALOR de `--mcp-config` en la línea de comandos del hijo. El argv de un proceso no es
// privado: en Linux `/proc/<pid>/cmdline` es legible por CUALQUIER usuario de la máquina
// (modo 444, salvo que /proc esté montado con hidepid), así que el token de las tools del
// cliente quedaba a la vista de todo el sistema mientras durara el turno. Con el archivo, el
// argv lleva una ruta y el secreto queda detrás de los permisos del archivo.
//
// Que la bandera acepte una RUTA está medido (2026-10-29, CLI 2.1.272): con la ruta en
// `--mcp-config`, el `system/init` del hijo muestra el servidor conectado y su tool en la
// tabla, igual que con el JSON inline. Con backslashes y con barras, las dos.
//
// ⚠️ EL 0600 NO SE CUMPLE EN WINDOWS, y conviene no creerse protegido ahí: Windows no tiene
// bits POSIX y Go sólo puede traducir el de escritura, así que el archivo queda `0666` para
// `os.Stat` (medido el 2026-10-29). Lo que lo protege de otros usuarios es el ACL del `%TEMP%`
// del perfil. La mejora igual es real de los dos lados: el token deja de aparecer en una línea
// de comandos que cualquier inventario de procesos —WMI, un EDR, un `ps` ajeno— registra.
func (e *espejo) escribirConfig() error {
	// MkdirTemp crea el directorio 0700 en Unix, así que el archivo de adentro no se lista ni
	// desde otra cuenta. El archivo va en un directorio propio y no pelado en el temporal para
	// poder borrar el conjunto con un RemoveAll.
	dir, err := os.MkdirTemp("", "musubi-mcp-")
	if err != nil {
		return fmt.Errorf("directorio para la config del espejo: %w", err)
	}
	ruta := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(ruta, []byte(e.configMCP()), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("escribir la config del espejo: %w", err)
	}
	e.dirConfig = dir
	e.rutaConfig = ruta
	return nil
}

// EjecucionesIntentadas dice cuántas veces el hijo intentó EJECUTAR una tool del cliente. En el
// diseño correcto es cero —el turno se corta en el `tool_use`, y el que ejecuta es el cliente
// remoto—, así que un número distinto no es un detalle de instrumentación: significa que el hijo
// llegó más lejos de lo que este motor supone, y que lo que el cliente recibió puede estar
// razonado sobre un resultado que nadie produjo.
func (e *espejo) EjecucionesIntentadas() int64 {
	if e == nil {
		return 0
	}
	return e.llamadas.Load()
}

// ListadosServidos dice cuántas veces el hijo pidió la lista de tools, y existe porque el fallo
// que mide es INVISIBLE sin él. Un cero con el espejo abierto significa que el hijo nunca
// encontró el servidor —el `--mcp-config` no llegó, el CLI lo descartó, el handshake se cayó— y
// entonces el modelo contestó el turno entero SIN SABER que las tools del cliente existían. Lo
// peligroso es que el turno vuelve igual de bien: el cliente recibe una respuesta que no se
// distingue de una en la que el modelo evaluó sus tools y eligió no usarlas.
//
// No hay un contador de `initialize` aparte, y es una decisión: el CLI no pide `tools/list` sin
// haber hecho el handshake, así que un cero acá ya cubre los dos. Un segundo contador cuyo cero
// está implicado por éste sólo agrega un número que nadie puede interpretar solo.
func (e *espejo) ListadosServidos() int64 {
	if e == nil {
		return 0
	}
	return e.listados.Load()
}

// configMCP es el JSON que declara este espejo como servidor MCP. Es el CONTENIDO del archivo
// que `--mcp-config` recibe; ya no va inline en el argv (ver escribirConfig).
func (e *espejo) configMCP() string {
	// Se arma con el encoder y no con Sprintf: la URL lleva un token y el nombre del servidor
	// es una constante, pero un `%s` acá sería la clase de lugar donde después entra algo sin
	// escapar.
	cfg := map[string]any{
		"mcpServers": map[string]any{
			nombreDelEspejo: map[string]any{
				"type": "http",
				"url":  e.url,
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		// Imposible: tres mapas de strings. Si pasara, devolver vacío deja al hijo sin tools,
		// que es menos malo que un argv a medio armar.
		return ""
	}
	return string(b)
}

// ServeHTTP atiende el JSON-RPC del hijo.
func (e *espejo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// LA RUTA ES LA CREDENCIAL. Un listener en loopback lo alcanza cualquier proceso local, y
	// lo que hay detrás son las definiciones de tools del cliente. El 404 va ANTES de mirar el
	// método para que un sondeo no pueda distinguir «ruta equivocada» de «método equivocado».
	//
	// ConstantTimeCompare y no `!=`: comparar un secreto con el operador corta en el primer
	// byte distinto, y eso convierte 2^128 intentos en 32 pasadas de 16 valores cada una. Lo
	// que esto compra es acotado y conviene no creerle más de lo que da —sobre HTTP el ruido
	// del socket tapa varios órdenes de magnitud de la diferencia, y el largo de la ruta se
	// filtra igual porque la función devuelve 0 enseguida cuando no coincide—, pero cuesta una
	// línea y saca de la ecuación la única parte que el atacante controla.
	if subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(e.ruta)) != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		// El spec de Streamable HTTP permite que el servidor no soporte el canal SSE del
		// servidor al cliente, y este no lo necesita: no manda notificaciones. El `Allow` dice
		// la verdad para que el hijo no quede adivinando.
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	// El tope existe porque el cuerpo lo escribe un proceso de afuera. Un handshake de MCP son
	// cientos de bytes; 1 MiB es holgura, no un límite que alguien pueda encontrar.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		e.responder(w, nil, nil, &errorRPC{Code: -32700, Message: "parse error"})
		return
	}

	// 🔴 EL CONTADOR VA ANTES DE LA PUERTA DE LAS NOTIFICACIONES, y no dentro del `case` de abajo.
	// Un `tools/call` SIN `id` sale por el 202 que sigue y NUNCA llega al switch, así que
	// contándolo allá el contador quedaba en cero justo en el caso que más se parece a un hijo
	// haciendo algo que este motor no previó. Lo que el número mide es el INTENTO de ejecutar, y
	// el `id` lo pone QUIEN INTENTA: no puede ser él el que decide si su intento queda
	// registrado. El aviso del lanzador se cuelga de este contador, y un aviso que el intruso
	// puede apagar omitiendo un campo no es un aviso.
	if req.Method == "tools/call" {
		e.llamadas.Add(1)
	}

	// SIN `id` ES UNA NOTIFICACIÓN Y NO SE CONTESTA. `notifications/initialized` llega así, y
	// mandarle una respuesta JSON-RPC a una notificación es un error de protocolo que algunos
	// clientes reportan como servidor roto.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		e.responder(w, req.ID, map[string]any{
			// La misma que declara el servidor MCP de este repo (internal/mcp/registry.go).
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]string{
				"name": "musubi-motor-espejo",
				// La versión del binario NO se cablea acá: nadie la consume —este servidor vive
				// lo que vive un turno y su único cliente es el hijo— y traerla obligaría a
				// pasarla por tres funciones para un campo que nunca se lee.
				"version": "1",
			},
		}, nil)

	case "tools/list":
		// ESTE CONTADOR ES LA ÚNICA PRUEBA DE QUE EL HIJO LLEGÓ HASTA ACÁ. Se suma antes de
		// responder porque lo que interesa es que el pedido existió, no que la respuesta se
		// escribiera entera: un cliente que corta la conexión a mitad del roster igual vio el
		// espejo. Ver ListadosServidos.
		e.listados.Add(1)
		e.responder(w, req.ID, json.RawMessage(`{"tools":`+string(e.tools)+`}`), nil)

	case "tools/call":
		// 🔴 ESTE CAMINO NO DEBERÍA EXISTIR, Y POR ESO CONTESTA UN ERROR. El turno se corta en
		// el `tool_use`: el que ejecuta es el cliente remoto. Pero «no debería» no es «no
		// puede», y un resultado vacío sería lo peor posible —el modelo seguiría razonando
		// sobre una tool que nadie corrió—. El error lo dice, y el contador —ya sumado arriba,
		// con `id` o sin él— lo deja ver.
		e.responder(w, req.ID, nil, &errorRPC{
			Code: -32603,
			Message: "las tools de este servidor las ejecuta el cliente remoto, no el agente: " +
				"el turno tendría que haber cerrado en el tool_use",
		})

	default:
		e.responder(w, req.ID, nil, &errorRPC{
			Code:    -32601,
			Message: "method not found: " + req.Method,
		})
	}
}

// errorRPC es el objeto `error` de JSON-RPC.
type errorRPC struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// responder escribe una respuesta JSON-RPC. El `id` viaja CRUDO: el spec deja que sea un número
// o un string, y re-tipearlo acá es la forma de devolverle al hijo un id que no es el que mandó.
func (e *espejo) responder(w http.ResponseWriter, id json.RawMessage, result any, rpcErr *errorRPC) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	resp := struct {
		JsonRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result,omitempty"`
		Error   *errorRPC       `json:"error,omitempty"`
	}{JsonRPC: "2.0", ID: id, Result: result, Error: rpcErr}

	w.Header().Set("Content-Type", "application/json")
	// El error de escritura se descarta: si el hijo se fue, no hay nada que hacer con eso que
	// no sea ruido en el stderr de un turno que igual va a terminar.
	_ = json.NewEncoder(w).Encode(resp)
}

// toolsParaElEspejo traduce las tools del formato del REQUEST (`input_schema`, que es lo que
// `Pedido.Tools` ya trae armado para el ClienteAPI) al de MCP (`inputSchema`). Es el mismo
// renombre de camelCase que ya muerde en tres lugares de este viaje, y acá va en la dirección
// contraria a `toolsParaLaAPI`.
//
// Y valida el TOPE con el prefijo puesto, que es el único lugar donde se puede: quien conoce el
// prefijo es este archivo.
func toolsParaElEspejo(toolsDeLaAPI json.RawMessage) (json.RawMessage, error) {
	var crudas []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	if err := json.Unmarshal(toolsDeLaAPI, &crudas); err != nil {
		return nil, fmt.Errorf("leer las tools del pedido para el espejo: %w", err)
	}

	// 🔴 EL `_meta` NO ES DECORACIÓN: sin él, el CLI le entrega la tool al modelo DIFERIDA.
	// Medido en vivo el 2026-10-29 contra el 2.1.272, mismo prompt y mismo host, con un
	// servidor MCP de juguete de una sola tool:
	//
	//	sin _meta -> el modelo emite [ToolSearch, mcp__juguete__sumar_dos_numeros]
	//	con _meta -> el modelo emite [mcp__juguete__sumar_dos_numeros]
	//
	// O sea que sin la marca, el primer `tool_use` del turno es una tool del CLI que el
	// cliente remoto no declaró y no sabe ejecutar. El literal va REPETIDO a propósito y no
	// importado de internal/mcp (que ya tiene la constante `metaAlwaysLoad`): la dependencia
	// va al revés —es internal/mcp el que importa este paquete— y cerrar el ciclo no compila.
	type toolMCP struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		InputSchema json.RawMessage `json:"inputSchema"`
		Meta        map[string]any  `json:"_meta,omitempty"`
	}
	out := make([]toolMCP, 0, len(crudas))
	for _, t := range crudas {
		if t.Name == "" {
			return nil, fmt.Errorf("una tool del pedido no tiene nombre: el CLI la expondría como %q", prefijoDelEspejo)
		}
		// 🔴 EL CLI RENOMBRA LO QUE NO ENTRA EN EL CHARSET, Y ESE ES EL PROBLEMA. Medido el
		// 2026-10-29 contra el 2.1.272, sirviendo nombres a propósito ilegales y leyendo la
		// tabla de tools del evento `system/init`:
		//
		//	declarado      el modelo ve     declarado      el modelo ve
		//	tool-ok        tool-ok          tool.punto     tool_punto
		//	ToolOk         ToolOk           tool@arroba    tool_arroba
		//	tool_ok        tool_ok          tool(par)      tool_par_
		//	                                toolñ          tool_
		//
		// Cada carácter fuera de `^[a-zA-Z0-9_-]$` se reemplaza por `_`, uno por uno. No se
		// descarta la tool ni se cae el servidor: el `init` lo muestra `connected` y con todas
		// las tools adentro, RENOMBRADAS. Y lo que vuelve al cliente es ese nombre: el espejo
		// sólo le saca el prefijo `mcp__pi__` (sinElPrefijoDelEspejo), así que un cliente que
		// declaró `read.file` recibe un `tool_use` para `read_file`, que no está en su tabla.
		// El agent loop remoto se corta con una «tool desconocida» que no explica nada.
		//
		// Y HAY UN CASO PEOR, también medido: `tool mala` y `tool/mala` normalizan las DOS a
		// `tool_mala`, el CLI deduplica y de tres tools el modelo vio dos. Dos entradas
		// distintas de la tabla del cliente colapsadas en una sola, en silencio.
		//
		// Por eso esto RECHAZA en vez de normalizar por su cuenta: el motor no puede inventarle
		// al cliente un nombre que no escribió, y un error que nombra la tool y el carácter es
		// lo único que el otro lado puede arreglar.
		if i := strings.IndexFunc(t.Name, fueraDelCharsetDeTool); i >= 0 {
			// El rune se decodifica en vez de cortar `t.Name[i:i+1]`: un carácter de más de un
			// byte —una `ñ`, cualquier acento— saldría partido a la mitad y el error diría
			// `"\xc3"`, que no le sirve a nadie para encontrar el carácter en su nombre.
			culpable, _ := utf8.DecodeRuneInString(t.Name[i:])
			return nil, fmt.Errorf(
				"la tool %q tiene un %q en la posición %d, y el CLI reemplaza por «_» todo lo "+
					"que no sea [a-zA-Z0-9_-]: el cliente recibiría un nombre que no declaró",
				t.Name, culpable, i)
		}
		// Con el charset validado el nombre es ASCII, así que contar bytes es contar caracteres.
		if n := len(prefijoDelEspejo) + len(t.Name); n > topeDelNombreDeTool {
			return nil, fmt.Errorf(
				"la tool %q queda en %d caracteres con el prefijo %q que le pone el CLI, y el tope es %d",
				t.Name, n, prefijoDelEspejo, topeDelNombreDeTool)
		}
		esquema := t.InputSchema
		if len(bytes.TrimSpace(esquema)) == 0 || bytes.Equal(bytes.TrimSpace(esquema), []byte("null")) {
			// Mismo motivo que en `toolsParaLaAPI`: `inputSchema` es obligatorio, y una tool sin
			// parámetros es legítima. Omitirlo haría que el hijo rechace el servidor entero, o
			// sea que una tool sin argumentos dejaría al turno sin NINGUNA tool.
			esquema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, toolMCP{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: esquema,
			Meta:        map[string]any{"anthropic/alwaysLoad": true},
		})
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("serializar las tools del espejo: %w", err)
	}
	return b, nil
}

// fueraDelCharsetDeTool dice si el rune no entra en `^[a-zA-Z0-9_-]$`, que es el charset que la
// API acepta en el nombre de una tool. Se escribe a mano y no con regexp porque corre una vez
// por tool de cada turno y una expresión regular acá sería compilarla en el camino caliente.
func fueraDelCharsetDeTool(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	case r == '_' || r == '-':
		return false
	}
	return true
}

// tokenDelEspejo genera el camino secreto. crypto/rand y no math/rand: es la credencial que
// separa al hijo de cualquier otro proceso de la máquina, y el error NO se degrada a un valor
// fijo —un token predecible es lo mismo que no tener token—.
//
// ⚠️ LA RAMA DEL ERROR ES INALCANZABLE EN ESTE GO, Y NINGÚN TEST PUEDE CUBRIRLA. Queda escrito
// acá para que nadie la busque como hueco de cobertura ni la «arregle» con un mock imposible:
// desde Go 1.24 `crypto/rand.Read` «never returns an error, and always fills b entirely», y el
// repo pide 1.26 (go.mod). Peor todavía para quien le crea a este `if`: la doc sigue con que
// Read «crashes the program irrecoverably» si la fuente del sistema falla, así que el modo de
// falla real no es un error que se pueda envolver —es el proceso muriéndose—. La guarda se
// queda igual porque cuesta una línea y falla CERRADA: si algún día el contrato se afloja, o
// este código se compila con un Go anterior, acá no sale un token vacío.
func tokenDelEspejo() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generar el token del espejo: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// nombreSinElPrefijoDelEspejo deshace el renombre del CLI. Devuelve el nombre TAL CUAL cuando no
// lleva el prefijo, y eso no es una cortesía: un `tool_use` sin `mcp__pi__` no salió del espejo
// —es una tool nativa del CLI, o de otro servidor MCP— y recortarle lo que no tiene le
// inventaría al cliente una tool que nunca declaró.
//
// Hoy el camino normal no debería traer ninguna de ésas: `--tools ""` deja las nativas en cero y
// `--strict-mcp-config` saca los otros servidores (medido el 2026-10-29, ver el campo Tools de
// Pedido). La guarda igual se queda, porque cuesta un CutPrefix y falla CERRADA: si una versión
// del CLI deja de honrar alguna de las dos banderas, acá el nombre pasa intacto en vez de salir
// mutilado. La versión vieja justificaba la guarda con «las 24 propias del CLI, que no se pueden
// sacar» — el número no se midió nunca y el «no se pueden» resultó falso. Lo que la sostiene es
// que falla cerrada, no que las nativas sean inevitables.
func nombreSinElPrefijoDelEspejo(nombre string) string {
	if resto, ok := strings.CutPrefix(nombre, prefijoDelEspejo); ok {
		return resto
	}
	return nombre
}

// sinElPrefijoDelEspejo envuelve el `emitir` para deshacer el renombre en los DOS eventos que
// llevan el nombre de una tool: el `toolcall_start`, que lo trae en `ToolName`, y el
// `toolcall_end`, que lo trae adentro del `ToolCall`. Olvidar uno de los dos es peor que no
// hacer nada: el cliente armaría el bloque con un nombre y lo cerraría con otro.
//
// El `ToolCall` se COPIA en vez de mutarse donde está, aunque hoy el traductor lo construye
// nuevo en cada cierre y mutarlo sería inocuo. Un envoltorio que escribe adentro de la
// estructura que le prestaron es la clase de atajo que funciona hasta el día que alguien cachea
// el evento del otro lado, y entonces el defecto no se ve acá.
func sinElPrefijoDelEspejo(emitir func(Evento) error) func(Evento) error {
	return func(ev Evento) error {
		if ev.ToolName != "" {
			ev.ToolName = nombreSinElPrefijoDelEspejo(ev.ToolName)
		}
		if ev.ToolCall != nil && ev.ToolCall.Name != "" {
			tc := *ev.ToolCall
			tc.Name = nombreSinElPrefijoDelEspejo(tc.Name)
			ev.ToolCall = &tc
		}
		return emitir(ev)
	}
}
