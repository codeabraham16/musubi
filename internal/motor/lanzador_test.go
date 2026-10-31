package motor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain convierte al binario de este test en un `claude` falso cuando se lo invoca con
// MOTOR_FALSO en el entorno. Es el patrón estándar de la stdlib para probar código que
// lanza procesos: sin esto no hay forma de verificar el CORTE —que el proceso muere de
// verdad— sin depender del binario real, de la red y de la cuota del usuario.
func TestMain(m *testing.M) {
	if guion := os.Getenv("MOTOR_FALSO"); guion != "" {
		claudeFalso(guion)
		return
	}
	os.Exit(m.Run())
}

// claudeFalso imita al binario según el guion que le toque:
//
//	emite:<archivo>  escupe ese fixture y después DUERME: si el lanzador no lo mata, se
//	                 queda vivo, y eso es justo lo que el test del corte mide.
//	llama-y-emite:<archivo>
//	                 PRIMERO le hace al espejo el `tools/list` que hace cualquier cliente MCP y
//	                 después un `tools/call` —el camino que el diseño da por imposible—, y recién
//	                 entonces emite el fixture. Es la única forma de encender los dos contadores
//	                 del espejo desde un proceso de verdad.
//	muere            escribe a stderr y sale con 1 sin emitir nada.
//	muda             no escribe nada y sale con 0.
func claudeFalso(guion string) {
	switch {
	case strings.HasPrefix(guion, "llama-y-emite:"):
		// El orden importa: el `tools/call` tiene que llegar ANTES del fixture, porque el
		// fixture cierra el turno y el lanzador mata este proceso ahí mismo.
		llamarAlEspejo()
		crudo, err := os.ReadFile(strings.TrimPrefix(guion, "llama-y-emite:"))
		if err != nil {
			os.Stderr.WriteString("falso: " + err.Error())
			os.Exit(1)
		}
		os.Stdout.Write(crudo)
		os.Exit(0)
	case strings.HasPrefix(guion, "emite:"):
		crudo, err := os.ReadFile(strings.TrimPrefix(guion, "emite:"))
		if err != nil {
			os.Stderr.WriteString("falso: " + err.Error())
			os.Exit(1)
		}
		os.Stdout.Write(crudo)
		// Deja marca de que siguió vivo tras emitir todo. Si el corte funciona, el
		// proceso muere acá y el archivo no aparece nunca.
		time.Sleep(10 * time.Second)
		if marca := os.Getenv("MOTOR_FALSO_MARCA"); marca != "" {
			os.WriteFile(marca, []byte("segui vivo"), 0o600)
		}
		os.Exit(0)
	case guion == "linea-larga":
		// Imita el `system/init` real del harness, que trae el entorno entero (tools,
		// plugins, rutas) y pasa holgadamente los 64 KB del default de bufio. Se genera
		// acá en vez de versionar un fixture de cientos de KB por un solo invariante.
		relleno := strings.Repeat("x", 200<<10)
		os.Stdout.WriteString(`{"type":"system","subtype":"init","entorno":"` + relleno + `"}` + "\n")
		os.Stdout.WriteString(`{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}` + "\n")
		os.Stdout.WriteString(`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}}` + "\n")
		os.Exit(0)
	case guion == "cuelga":
		// Arranca un turno y no lo cierra nunca: el caso del proceso que se quedó
		// esperando algo que no va a llegar.
		os.Stdout.WriteString(`{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}` + "\n")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case guion == "muere":
		os.Stderr.WriteString("Error: credit balance is too low\n")
		os.Exit(1)
	default:
		os.Exit(0)
	}
}

// llamarAlEspejo busca la ruta del `--mcp-config` en el argv de ESTE proceso, saca de ahí la
// URL del espejo y le manda un `tools/call`. Corre dentro del proceso falso, así que no puede
// usar `t` ni fallar el test: lo único que hace es provocar el efecto que el test mide.
//
// Que esto sea posible es el punto. La URL del espejo no está en el argv —desde escribirConfig
// vive en un archivo 0600—, pero la RUTA de ese archivo sí, y el hijo la lee porque para eso se
// la pasamos. El aislamiento del espejo no es el secreto de la URL frente al hijo: es que
// ningún OTRO proceso de la máquina la vea en la línea de comandos.
func llamarAlEspejo() {
	var ruta string
	for i, a := range os.Args {
		if a == "--mcp-config" && i+1 < len(os.Args) {
			ruta = os.Args[i+1]
		}
	}
	// Sin espejo el valor de la bandera es el mapa vacío inline, no una ruta: ReadFile falla y
	// no hay nada que llamar. Es el camino normal de los demás guiones.
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return
	}
	var cfg struct {
		MCPServers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(crudo, &cfg); err != nil {
		os.Stderr.WriteString("falso: config MCP ilegible: " + err.Error())
		return
	}
	url := cfg.MCPServers[nombreDelEspejo].URL
	if url == "" {
		os.Stderr.WriteString("falso: la config MCP no declara el espejo")
		return
	}
	// EL `tools/list` VA PRIMERO PORQUE ASÍ LO HACE EL CLI DE VERDAD: nadie llama una tool que
	// no listó. No es decorado — es lo que hace que este guion sirva de CONTROL del aviso de
	// "el hijo nunca vio las tools": sin esta llamada, los dos brazos de ese test mostrarían el
	// mismo stderr y ninguno probaría nada.
	lista, err := http.Post(url, "application/json", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		os.Stderr.WriteString("falso: el tools/list no llegó: " + err.Error())
		return
	}
	lista.Body.Close()

	resp, err := http.Post(url, "application/json", strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"leer","arguments":{}}}`))
	if err != nil {
		os.Stderr.WriteString("falso: el tools/call no llegó: " + err.Error())
		return
	}
	resp.Body.Close()
}

// lanzadorFalso devuelve un Lanzador que ejecuta este mismo binario de test con el guion
// dado, en vez del `claude` real.
func lanzadorFalso(guion string, extra ...string) *Lanzador {
	return &Lanzador{
		Binario: os.Args[0],
		Entorno: append(append(os.Environ(), "MOTOR_FALSO="+guion), extra...),
	}
}

// EL CORTE, de punta a punta y con un proceso de verdad. Dos mitades, y las dos importan:
// que se emita sólo el primer turno, y que el proceso MUERA. Si lo segundo falla queda un
// `claude` huérfano por request, consumiendo la cuota del usuario para una respuesta que
// nadie va a leer.
func TestElCorteMataElProceso(t *testing.T) {
	marca := filepath.Join(t.TempDir(), "segui-vivo")
	fix, err := filepath.Abs(filepath.Join("testdata", "texto-y-tool-dos-turnos.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	l := lanzadorFalso("emite:"+fix, "MOTOR_FALSO_MARCA="+marca)

	var evs []Evento
	arranque := time.Now()
	err = l.Turno(context.Background(), Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error { evs = append(evs, e); return nil })
	tardo := time.Since(arranque)
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}

	// 1) Un solo turno emitido, aunque el fixture trae dos.
	var dones int
	for _, e := range evs {
		if e.Type == EvDone || e.Type == EvError {
			dones++
		}
	}
	if dones != 1 {
		t.Errorf("cierres emitidos: got %d, want 1 (forma: %v)", dones, forma(evs))
	}

	// 2) El proceso murió: ni esperó sus 10 s de sueño ni dejó la marca.
	if tardo > 4*time.Second {
		t.Errorf("Turno tardó %s: no mató el proceso, esperó a que terminara solo", tardo.Round(time.Millisecond))
	}
	if _, err := os.Stat(marca); err == nil {
		t.Error("el proceso sobrevivió al corte y dejó su marca: queda un claude huérfano por request")
	}
}

// Un error del consumidor (el cliente se desconectó a mitad del SSE) tiene que cortar el
// turno y volver, no quedarse bombeando eventos a un socket muerto.
func TestSiElConsumidorFallaElTurnoVuelve(t *testing.T) {
	fix, err := filepath.Abs(filepath.Join("testdata", "texto-y-tool-dos-turnos.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	l := lanzadorFalso("emite:" + fix)

	arranque := time.Now()
	var vistos int
	err = l.Turno(context.Background(), Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error {
			vistos++
			if vistos == 2 {
				return os.ErrClosed
			}
			return nil
		})
	if err == nil {
		t.Fatal("se esperaba el error del consumidor")
	}
	if !strings.Contains(err.Error(), "emitir") {
		t.Errorf("el error debe decir que falló la emisión, got %v", err)
	}
	if tardo := time.Since(arranque); tardo > 4*time.Second {
		t.Errorf("tardó %s: no mató el proceso al fallar la emisión", tardo.Round(time.Millisecond))
	}
	if vistos != 2 {
		t.Errorf("siguió emitiendo después del error: %d eventos", vistos)
	}
}

// Un proceso que muere sin servir un turno no puede devolver nil: el handler tiene que
// poder contestar algo. Y el stderr es lo único que dice POR QUÉ murió, así que va adentro
// del error — sin eso, un «credit balance too low» llega como un fallo mudo.
func TestUnProcesoQueMuereLlevaSuStderrAlError(t *testing.T) {
	err := lanzadorFalso("muere").Turno(context.Background(),
		Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(Evento) error { return nil })
	if err == nil {
		t.Fatal("se esperaba error")
	}
	if !strings.Contains(err.Error(), "credit balance") {
		t.Errorf("el stderr del proceso no llegó al error: %v", err)
	}
}

// Un proceso que sale limpio pero sin emitir nada tampoco es un turno servido.
func TestUnProcesoMudoEsError(t *testing.T) {
	err := lanzadorFalso("muda").Turno(context.Background(),
		Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(Evento) error { return nil })
	if err == nil {
		t.Fatal("un proceso que no emitió nada no sirvió el turno")
	}
	if !strings.Contains(err.Error(), "sin respuesta del modelo") {
		t.Errorf("error poco claro: %v", err)
	}
}

// Un proceso que arranca un turno y no lo cierra se corta por reloj, y el error tiene que
// DECIR que fue el reloj. Si no se distingue, un turno vencido se reporta igual que un
// proceso que murió, y son problemas distintos: uno se arregla subiendo el tope y el otro no.
func TestElTurnoSeCortaPorTiempoYLoDice(t *testing.T) {
	l := lanzadorFalso("cuelga")
	l.Espera = 400 * time.Millisecond

	arranque := time.Now()
	var evs []Evento
	err := l.Turno(context.Background(), Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error { evs = append(evs, e); return nil })
	tardo := time.Since(arranque)

	if err == nil {
		t.Fatal("se esperaba el error de tiempo")
	}
	if !strings.Contains(err.Error(), "por tiempo") {
		t.Errorf("el error no dice que fue el reloj: %v", err)
	}
	if tardo > 5*time.Second {
		t.Errorf("tardó %s: no respetó su propio tope de %s", tardo.Round(time.Millisecond), l.Espera)
	}
	// Lo que sí alcanzó a llegar se emitió: el cliente vio el arranque del turno.
	if len(evs) != 1 || evs[0].Type != EvStart {
		t.Errorf("eventos emitidos antes del corte: %v", tipos(evs))
	}
}

// Un ctx que el llamador cancela (el request HTTP se cayó) tiene que volver enseguida, no
// quedarse hasta el tope de tiempo del turno.
func TestElCancelDelLlamadorCortaElTurno(t *testing.T) {
	l := lanzadorFalso("cuelga")
	ctx, cancelar := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancelar()
	}()

	arranque := time.Now()
	err := l.Turno(ctx, Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(Evento) error { return nil })
	if tardo := time.Since(arranque); tardo > 5*time.Second {
		t.Errorf("tardó %s: ignoró el cancel del llamador", tardo.Round(time.Millisecond))
	}
	if err == nil {
		t.Fatal("un turno que no se sirvió no puede volver nil")
	}
	// EL MENSAJE IMPORTA, y es el caso MÁS FRECUENTE de un servidor: un cliente HTTP que
	// cierra la conexión cancela el ctx. Sin un case propio esto caía al default y salía
	// como «el proceso terminó sin respuesta del modelo: exit status 1» —el texto exacto de
	// un binario roto—, así que un log lleno de desconexiones normales se leía como un
	// binario que falla.
	if !strings.Contains(err.Error(), "canceló") {
		t.Errorf("el error tiene que decir que lo cortó el llamador, no parecer un crash: %v", err)
	}
}

// 🔴 EL ERROR EN EL ÚLTIMO EVENTO. Este test existe porque un sabotaje quedó VERDE: el que
// ya había (TestSiElConsumidorFallaElTurnoVuelve) falla en el PRIMER evento, y ahí el turno
// todavía no cerró, así que medía otra cosa.
//
// El caso real es éste: el traductor marca el turno como cerrado ANTES de que su último
// evento se emita, así que un `emitir` que falle justo en el `done` deja cerrado=true Y un
// error. Con `cerrado` ganando en el switch, Turno devolvía nil y el llamador creía que
// había servido un turno que el cliente nunca recibió entero — que es exactamente el caso
// de un cliente que se desconecta mientras llega la respuesta.
func TestUnErrorEnElUltimoEventoNoSeReportaComoTurnoServido(t *testing.T) {
	roto := errors.New("el transporte se cayó")
	var vistos []string
	err := lanzadorFalso("linea-larga").Turno(context.Background(),
		Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error {
			vistos = append(vistos, e.Type)
			if e.Type == EvDone {
				return roto
			}
			return nil
		})

	// Primero: que el turno haya llegado de verdad hasta el `done`. Sin esto el test
	// pasaría también si el stream se cortara antes, y entonces no mediría el caso.
	if len(vistos) == 0 || vistos[len(vistos)-1] != EvDone {
		t.Fatalf("el turno no llegó al done: %v", vistos)
	}
	if err == nil {
		t.Fatal("un turno cuyo último evento no se pudo entregar NO se sirvió: no puede volver nil")
	}
	if !errors.Is(err, roto) {
		t.Errorf("el error del consumidor tiene que viajar envuelto: %v", err)
	}
	if !errors.Is(err, ErrDelConsumidor) {
		t.Errorf("tiene que marcarse como error del consumidor, no de lectura: %v", err)
	}
}

// 🔴 LA FUGA. `emitir` lo escribe el llamador y corre dentro de la goroutine del turno: si
// PANICA, el unwinding saltea cualquier cosecha que esté en línea recta, y entonces la
// goroutine watchCtx de os/exec se queda bloqueada PARA SIEMPRE en su send final (exec.go:881,
// un canal sin buffer cuyo único receptor es Wait), reteniendo el handle del proceso, el
// extremo de lectura del stdout y el transcript entero que tiene el Stdin.
//
// ⚠️ ACÁ DECÍA «net/http recupera el panic por request, así que el servidor no se cae», y eso es
// FALSO: esta goroutine no es la del handler y el recuperador de net/http no la alcanza (medido
// el 2026-10-29, `(*conn).serve.func1` presente en la pila de un panic del handler y ausente en
// la de ésta). Quien hace que el servidor sobreviva —y que por lo tanto la fuga se ACUMULE
// turno a turno en vez de morir con el proceso— es el recover de `bombearTurno`
// (internal/mcp/turno.go). El motivo del test no cambia; el mecanismo que lo justifica, sí.
//
// Medido con un A/B de un factor el 2026-10-28: tres turnos que panican dejaban 3 goroutines
// con el Wait en línea recta y 0 con el Wait en un defer. Este test lo pinea contando las
// goroutines del paquete os/exec, que es lo que de verdad queda colgado.
func TestUnPanicDelConsumidorNoDejaLaCosechaSinHacer(t *testing.T) {
	antes := goroutinesDeExec()

	const turnos = 3
	for i := 0; i < turnos; i++ {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Error("el panic del consumidor tiene que llegar al llamador, no comerse acá")
				}
			}()
			// `cuelga` emite el `message_start` —que dispara el primer evento, y con él
			// el panic— y después duerme 30 s. El proceso está VIVO en el momento del
			// panic, que es exactamente cuando la cosecha importa: con el proceso ya
			// terminado el watchCtx de os/exec se va solo y el test no mediría nada.
			_ = lanzadorFalso("cuelga").Turno(context.Background(),
				Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
				func(Evento) error { panic("el consumidor explotó") })
		}()
	}

	// Las goroutines de os/exec mueren en el Wait que hace el defer, pero no
	// instantáneamente: el runtime las desprograma. Se le da una ventana corta y se mide.
	for i := 0; i < 50; i++ {
		if goroutinesDeExec() <= antes {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ahora := goroutinesDeExec(); ahora > antes {
		t.Errorf("quedaron %d goroutines de os/exec vivas (antes %d) tras %d panics: la cosecha no corrió",
			ahora, antes, turnos)
	}
}

// goroutinesDeExec cuenta las goroutines paradas dentro de os/exec. Se mira ESE paquete y
// no el total de runtime.NumGoroutine() porque el total sube y baja por el testing y por el
// runtime, y un contador ruidoso no distingue una fuga de un pico.
func goroutinesDeExec() int {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	return strings.Count(string(buf), "os/exec.(*Cmd).watchCtx")
}

// Una línea larga del harness no puede tirar el turno. El `system/init` real pasa los
// 64 KB que bufio.Scanner admite por defecto, y es la PRIMERA línea del stream: con el
// default, el turno moría antes de leer una sola palabra del modelo, y el error
// (`token too long`) no insinúa de dónde viene.
func TestUnaLineaLargaDelHarnessNoTiraElTurno(t *testing.T) {
	var evs []Evento
	err := lanzadorFalso("linea-larga").Turno(context.Background(),
		Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error { evs = append(evs, e); return nil })
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}
	if got := tipos(evs); len(got) != 2 || got[0] != EvStart || got[1] != EvDone {
		t.Errorf("el turno no se sirvió completo: %v", got)
	}
}

func TestBinarioInexistente(t *testing.T) {
	l := &Lanzador{Binario: filepath.Join(t.TempDir(), "no-existe-claude")}
	err := l.Turno(context.Background(), Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(Evento) error { return nil })
	if err == nil {
		t.Fatal("se esperaba error")
	}
	if !strings.Contains(err.Error(), "no se pudo lanzar") {
		t.Errorf("el error debe nombrar el problema real: %v", err)
	}
}

// LAS BANDERAS. Cada una de estas tres cambia si esto funciona o no, y la falla de las dos
// primeras es silenciosa: sin `--include-partial-messages` el binario emite mensajes
// completos y el cliente —que es todo deltas— no muestra NADA; sin
// `--input-format stream-json` no se puede mandar un transcript con turnos `assistant` y
// esto deja de ser un proveedor de modelo para ser un chat de una sola pregunta.
//
// Sabotaje M15: el hijo hereda los MCP del host y le muestra al modelo los conectores
// personales de quien corre el servidor
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\t\"--strict-mcp-config\",\n"
// arnes: a=""
//
// Sabotaje M17: el hijo se queda con las tools nativas del CLI, que le dan el disco del
// host y que el cliente remoto no declaró
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\t\"--tools\", \"\",\n"
// arnes: a=""
//
// Sabotaje M18: la sesión sintética del turno queda escrita en el disco del usuario
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\t\"--no-session-persistence\",\n"
// arnes: a=""
//
// Sabotaje M19: sin espejo no se declara el mapa MCP vacío, y `--strict` queda suelta
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\tconfigMCP := `{\"mcpServers\":{}}`"
// arnes: a="\tconfigMCP := \"\""
func TestElArgvLlevaLasBanderasQueHabilitanElProtocolo(t *testing.T) {
	args := argumentosCLI(Pedido{Modelo: "claude-opus-5", SystemPrompt: "sos un motor"}, `C:\tmp\s.jsonl`, nil)
	junto := strings.Join(args, " ")

	for _, quiero := range []string{
		"-p",
		"--output-format stream-json",
		"--include-partial-messages",
		"--input-format stream-json",
		"--verbose", // obligatorio con stream-json: el binario lo rechaza sin él
		"--model claude-opus-5",
		"--system-prompt sos un motor",
		// El prefijo de la historia entra por acá. Ver partirHistoria: sin esto el CLI
		// desarma el transcript y el modelo responde al primer mensaje.
		`--resume C:\tmp\s.jsonl`,
		// La sesión es sintética: no tiene que quedar en el disco del usuario.
		"--no-session-persistence",
		// Vacía las plantillas de atribución. Sin esto el CLI inyecta un
		// `<system-reminder>` sobre cómo firmar commits DENTRO del mensaje del cliente.
		`--settings {"attribution":{"commit":"","pr":""}}`,
	} {
		if !strings.Contains(junto, quiero) {
			t.Errorf("falta %q en el argv: %v", quiero, args)
		}
	}

	// Sin prefijo no hay sesión que reanudar, y pasar `--resume` con valor vacío haría que
	// el binario busque un archivo llamado "" y falle.
	//
	// 🔴 `--no-session-persistence` YA NO ESTÁ EN ESTA LISTA, y sacarla de acá es el arreglo.
	// La versión vieja exigía que un turno sin prefijo NO la llevara, atando la bandera al
	// `--resume` como si fueran la misma cosa. No lo son: sin `--resume` el CLI no reanuda
	// nada pero IGUAL escribe la sesión nueva en `~/.claude/projects/<cwd>/<uuid>.jsonl`, o
	// sea en el disco de quien corre el servidor. El test anterior dejaba ese agujero fijado.
	sinSesion := strings.Join(argumentosCLI(Pedido{}, "", nil), " ")
	if strings.Contains(sinSesion, "--resume") {
		t.Errorf("un turno de un solo mensaje no debe reanudar nada: %s", sinSesion)
	}
	if !strings.Contains(sinSesion, "--no-session-persistence") {
		t.Errorf("un turno sin prefijo también tiene que llevar --no-session-persistence, o "+
			"la sesión sintética queda en el disco del usuario: %s", sinSesion)
	}

	// Las banderas de valor VACÍO no se pueden buscar por substring: hay que mirar el PAR.
	conValorVacio := func(bandera string) bool {
		for i, a := range args {
			if a == bandera && i+1 < len(args) && args[i+1] == "" {
				return true
			}
		}
		return false
	}
	for _, c := range []struct{ bandera, porque string }{
		{"--setting-sources", "el hijo cargaría los hooks y skills de Musubi y recursaría"},
		{"--tools", "el hijo se quedaría con las tools nativas del CLI, que le dan el disco " +
			"del host y que el cliente remoto no declaró ni sabe ejecutar"},
	} {
		if !conValorVacio(c.bandera) {
			t.Errorf("falta %s con valor vacío: %s. argv: %q", c.bandera, c.porque, args)
		}
	}

	// Sin modelo ni system prompt, esas banderas no van con valor vacío: el binario las
	// tomaría como un modelo llamado "" y fallaría.
	pelado := strings.Join(argumentosCLI(Pedido{}, "", nil), " ")
	if strings.Contains(pelado, "--model") || strings.Contains(pelado, "--system-prompt") {
		t.Errorf("un pedido sin modelo no debe pasar la bandera: %s", pelado)
	}

	// 🔴 ACÁ ESTABA EL AGUJERO FIJADO POR UN TEST, y este bloque es el arreglo. La versión
	// vieja exigía que sin espejo NO fuera ninguna de las tres banderas del MCP, con este
	// argumento: «`--strict-mcp-config` suelta sería peor que no pasar nada: le apagaría al
	// hijo los MCP del proyecto sin darle ninguno a cambio». La objeción era buena y la
	// contesta el mapa VACÍO EXPLÍCITO: `--strict-mcp-config --mcp-config '{"mcpServers":{}}'`
	// no lo deja a la deriva, le DECLARA que no tiene ninguno.
	//
	// Y el costo de la versión vieja está medido (2026-10-29, evento `system/init` del
	// 2.1.272): sin esas banderas, un turno que no pidió NINGUNA tool igual le mostraba al
	// modelo 42 tools, con los 12 de los conectores personales de quien corre el servidor
	// entre ellas. Con las banderas, 30; y con `--tools ""` además, 0.
	sinEspejo := argumentosCLI(Pedido{}, "", nil)
	juntoSinEspejo := strings.Join(sinEspejo, " ")
	for _, falta := range []string{"--strict-mcp-config", `--mcp-config {"mcpServers":{}}`} {
		if !strings.Contains(juntoSinEspejo, falta) {
			t.Errorf("un turno sin tools también tiene que llevar %q, o el hijo ve los "+
				"conectores de quien corre el servidor: %v", falta, sinEspejo)
		}
	}

	// `--allowedTools` SÍ depende del espejo, y es la única de las tres que depende: sin
	// servidor MCP no hay ninguna tool que autorizar.
	if strings.Contains(juntoSinEspejo, "--allowedTools") {
		t.Errorf("sin espejo no hay tool que autorizar: %v", sinEspejo)
	}

	// Un solo `--mcp-config` en el argv. Con dos, el resultado depende de cómo el CLI
	// resuelve la bandera repetida —si gana el primero, el último o se acumulan—, y eso no
	// está medido: el espejo podría quedar tapado por el mapa vacío sin que nada falle.
	conEspejo := argumentosCLI(Pedido{}, "", &espejo{url: "http://127.0.0.1:1/x", ruta: "/x"})
	var veces int
	for _, a := range conEspejo {
		if a == "--mcp-config" {
			veces++
		}
	}
	if veces != 1 {
		t.Errorf("--mcp-config tiene que ir una sola vez y fue %d: %v", veces, conEspejo)
	}
}

// 🔴 CÓMO LLEGAN LAS TOOLS DEL CLIENTE. El CLI no tiene una bandera para declarar tools: la
// única puerta es MCP (medido contra el 2.1.272 el 2026-10-29). Las tres banderas de acá son
// lo que convierte al espejo en la tabla de tools del turno, y las tres fallan distinto:
//   - sin `--mcp-config` el modelo no ve una sola tool del cliente y contesta en prosa;
//   - sin `--strict-mcp-config` el hijo ADEMÁS carga el `.mcp.json` del proyecto y recursa
//     sobre Musubi, lo que en headless no se ve: se ve como un turno lento;
//   - sin `--allowedTools` cada tool del cliente se frena pidiendo un permiso que nadie va
//     a contestar, y el turno muere por timeout sin un error que lo explique.
//
// El espejo se arma a mano en vez de con abrirEspejo: esto juzga el argv, no el socket.
//
// M15 ya no vive acá: `--strict-mcp-config` dejó de depender del espejo y su sabotaje se
// mudó a TestElArgvLlevaLasBanderasQueHabilitanElProtocolo, que es el test que ahora lo
// cubre. Al moverla también le cambió el ancla —la línea pasó de tres tabs a dos—, y una
// directiva `arnes:` que apunta a un texto que ya no existe NO falla: el sabotaje
// simplemente no se aplica y el arnés informa verde.
//
// Sabotaje M16: las tools del cliente llegan sin permiso y cada una frena el turno. Medido
// el 2026-10-29: sin la bandera, en `-p` no interactivo el espejo no recibe un solo
// `tools/call` y el turno se le va al cliente diciendo que falta un permiso.
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\targs = append(args, \"--allowedTools\", \"mcp__\"+nombreDelEspejo)"
// arnes: a=""
//
// Sabotaje M21: la credencial del turno vuelve a viajar en la línea de comandos del hijo
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\tconfigMCP = esp.rutaConfig\n"
// arnes: a="\t\tconfigMCP = esp.configMCP()\n"
// M21 pone rojo DOS pruebas, y la segunda es legítima: el proceso falso de
// TestElLectorDelContadorDelEspejoAvisaCuandoElHijoEjecuta encuentra el espejo LEYENDO el archivo
// que la bandera nombra, así que con la config de vuelta inline no hay ruta que abrir y el
// `tools/call` nunca sale. No lleva `colision_ok`: para el arnés una colisión es que un sabotaje
// rompa el ANCLA de otra directiva —daño al corpus—, y acá no se rompe ninguna. Declararlo con esa
// clave sería contestarle a algo que no pasó, y el censo lo denuncia como respuesta rancia.
func TestElArgvDeclaraElEspejoCuandoElPedidoTraeTools(t *testing.T) {
	// El token es distintivo a propósito: las aserciones de abajo preguntan si aparece en
	// ALGUNA parte del argv, y un valor genérico podría coincidir con la ruta temporal.
	esp := &espejo{
		url:        "http://127.0.0.1:54321/t0k3nd3lturn0",
		ruta:       "/t0k3nd3lturn0",
		rutaConfig: filepath.Join(t.TempDir(), "mcp.json"),
	}
	args := argumentosCLI(Pedido{Modelo: "claude-opus-5"}, "", esp)
	junto := strings.Join(args, " ")

	for _, quiero := range []string{
		"--strict-mcp-config",
		"--allowedTools mcp__pi",
	} {
		if !strings.Contains(junto, quiero) {
			t.Errorf("falta %q en el argv: %v", quiero, args)
		}
	}

	// 🔴 LAS DOS ASERCIONES DE ACÁ ESTABAN AL REVÉS, y esto es el arreglo. La versión vieja
	// exigía que el valor de `--mcp-config` empezara con `{` y que CONTUVIERA `esp.url`:
	// o sea, fijaba que la credencial del turno viajara dentro de la línea de comandos. El
	// argumento era «la bandera acepta JSON files or strings y así el esquema no toca el
	// disco», y el esquema de tools no es el problema: la URL del espejo lleva el token, y el
	// argv de un proceso no es privado (en Linux `/proc/<pid>/cmdline` lo lee cualquier
	// usuario). Ver escribirConfig en espejo.go.
	var cfg string
	for i, a := range args {
		if a == "--mcp-config" && i+1 < len(args) {
			cfg = args[i+1]
		}
	}
	if cfg == "" {
		t.Fatalf("falta --mcp-config con valor: %q", args)
	}
	if cfg != esp.rutaConfig {
		t.Errorf("--mcp-config tiene que ser la RUTA del archivo del espejo (%s) y fue %q",
			esp.rutaConfig, cfg)
	}
	// Lo que de verdad importa, y por eso se mira el argv ENTERO y no sólo el valor de la
	// bandera: el token no puede estar en ninguna parte de la línea de comandos.
	for _, secreto := range []string{esp.url, esp.ruta} {
		if strings.Contains(junto, secreto) {
			t.Errorf("la credencial del turno (%s) viaja en el argv, donde la lee cualquier "+
				"proceso de la máquina: %v", secreto, args)
		}
	}
}

// 🔴 EL CAMINO DE PRODUCCIÓN QUE NINGÚN TEST EJECUTABA. Los tres tests del desprefijado que
// ya había (en espejo_test.go) llaman a sinElPrefijoDelEspejo a mano; ninguno pasa por
// `Lanzador.Turno` con `Pedido.Tools`, que es la ÚNICA rama donde el envoltorio se instala
// (lanzador.go:131-138) y la única que corre en un turno real. Borrar esas líneas dejaba todo
// el paquete en verde y le entregaba al cliente un `tool_use` llamado `mcp__pi__leer`: un
// nombre que su tabla no tiene, con lo que su agent loop se corta sin un error que lo explique.
//
// LOS DOS EVENTOS IMPORTAN, y se arman por caminos distintos del traductor: el
// `content_block_start` emite el EvToolCallStart con `ToolName` (traductor.go:288) y el
// `content_block_stop` emite el EvToolCallEnd con `ToolCall.Name` (traductor.go:371-383).
// Desprefijar uno y no el otro es un modo de falla que mirar un solo evento no detecta.
//
// Sabotaje M26: el envoltorio no se instala y el cliente recibe el nombre que el CLI inventó
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\temitir = sinElPrefijoDelEspejo(emitir)"
// arnes: a="\t\t_ = sinElPrefijoDelEspejo"
func TestElTurnoConToolsDelClienteLeSacaElPrefijoDelCLI(t *testing.T) {
	fix, err := filepath.Abs(filepath.Join("testdata", "tool-del-cliente-con-prefijo.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// La tool va con el nombre PELADO, que es como la declara el cliente. El prefijo lo pone el
	// CLI al exponerla por MCP, y el fixture ya viene con él puesto: es una captura del formato
	// real, no una invención del test.
	p := Pedido{
		Mensajes: []Mensaje{TextoDeUsuario("leé el archivo")},
		Tools: json.RawMessage(
			`[{"name":"leer","description":"lee un archivo","input_schema":{"type":"object"}}]`),
	}

	var evs []Evento
	if err := lanzadorFalso("emite:"+fix).Turno(context.Background(), p,
		func(e Evento) error { evs = append(evs, e); return nil }); err != nil {
		t.Fatalf("Turno: %v", err)
	}

	// Que el turno haya llegado de verdad a los dos eventos de la tool. Sin esta guarda el test
	// pasaría igual con un stream que nunca emitió un tool_use —el verde VACUO— y no mediría
	// nada: no hay nombre que desprefijar donde no hay tool.
	var inicio, fin *Evento
	for i := range evs {
		switch evs[i].Type {
		case EvToolCallStart:
			inicio = &evs[i]
		case EvToolCallEnd:
			fin = &evs[i]
		}
	}
	if inicio == nil || fin == nil {
		t.Fatalf("el turno no emitió los dos eventos de la tool: %v", tipos(evs))
	}
	if fin.ToolCall == nil {
		t.Fatal("el toolcall_end llegó sin su ToolCall: el cliente no tiene qué ejecutar")
	}

	if inicio.ToolName != "leer" {
		t.Errorf("toolcall_start: got %q, want %q", inicio.ToolName, "leer")
	}
	if fin.ToolCall.Name != "leer" {
		t.Errorf("toolcall_end: got %q, want %q", fin.ToolCall.Name, "leer")
	}
	// Barrido del turno entero: el prefijo no puede sobrevivir en NINGÚN evento. Las dos
	// aserciones de arriba fijan el camino conocido; esto cubre el que todavía no existe.
	for i, e := range evs {
		if strings.Contains(e.ToolName, prefijoDelEspejo) {
			t.Errorf("evento %d (%s): el prefijo del CLI llegó al cliente en ToolName: %q",
				i, e.Type, e.ToolName)
		}
		if e.ToolCall != nil && strings.Contains(e.ToolCall.Name, prefijoDelEspejo) {
			t.Errorf("evento %d (%s): el prefijo del CLI llegó al cliente en ToolCall.Name: %q",
				i, e.Type, e.ToolCall.Name)
		}
	}
	// Y el resto del tool_use sigue entero: tocar el nombre no puede comerse los argumentos ni
	// el id, que es con lo que el cliente empareja el tool_result que va a devolver.
	if fin.ToolCall.ID != "toolu_01DelEspejo" {
		t.Errorf("el id del tool_use se perdió: %q", fin.ToolCall.ID)
	}
	if got := string(fin.ToolCall.Arguments); got != `{"ruta": "/tmp/dato.txt"}` {
		t.Errorf("los argumentos no llegaron enteros: %s", got)
	}
	if fin.ToolCall.ArgumentsError != "" {
		t.Errorf("argumentos ilegibles: %s", fin.ToolCall.ArgumentsError)
	}
}

// stderrDelTurno corre un turno con os.Stderr desviado y devuelve lo que el motor escribió ahí.
//
// Los avisos del lector van a os.Stderr —ver el defer de `Lanzador.Turno`: cuando salen, el
// turno YA se sirvió, así que no pueden volverse un error del cliente—, así que medirlos exige
// desviar la variable global. Un ARCHIVO y no un os.Pipe: el buffer de un pipe son 64 KB y
// nadie lo está drenando, así que el día que un aviso crezca el test colgaría en vez de fallar,
// y un test que cuelga no dice qué se rompió.
func stderrDelTurno(t *testing.T, l *Lanzador, p Pedido) (string, error) {
	t.Helper()
	desvio, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	viejo := os.Stderr
	os.Stderr = desvio
	errTurno := l.Turno(context.Background(), p, func(Evento) error { return nil })
	os.Stderr = viejo
	if cerr := desvio.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	crudo, err := os.ReadFile(desvio.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(crudo), errTurno
}

// 🔴 EL AVISO CONTRARIO, Y ES EL ÚNICO DE LOS DOS QUE PASA CALLADO. El de abajo se nota solo: el
// cliente recibe un resultado que no pidió. Éste no se nota NUNCA sin el aviso — si el hijo no
// encontró el espejo, el modelo contestó el turno entero sin saber que las tools del cliente
// existían, y lo que vuelve es una respuesta perfectamente formada. El cliente no puede
// distinguirla de una en la que el modelo evaluó sus tools y eligió no usar ninguna, así que un
// `--mcp-config` que dejó de llegar se lee como un modelo que se volvió vago.
//
// LAS DOS MITADES SE MIDEN, y la que falta en la mayoría de los tests de avisos es la segunda:
// que CALLE cuando corresponde. Un aviso que sale siempre deja de ser información en dos días.
// El único cambio entre los brazos es el guion del proceso falso — mismo fixture, mismo pedido,
// mismo lanzador—, así que un aviso de más o de menos sólo puede venir de que el hijo pidió la
// lista o no.
//
// Sabotaje M30: el aviso desaparece y el hijo que nunca vio las tools queda invisible
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\tif esp != nil && esp.ListadosServidos() == 0 {"
// arnes: a="\t\tif esp != nil && esp.ListadosServidos() < 0 {"
// arnes: colision_ok="TestElLectorAvisaCuandoElHijoNuncaVioLasTools"
//
// Sabotaje M31: el aviso pierde la guarda del nil y grita en todos los turnos sin tools
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\tif esp != nil && esp.ListadosServidos() == 0 {"
// arnes: a="\t\tif esp.ListadosServidos() == 0 {"
// arnes: colision_ok="TestElLectorAvisaCuandoElHijoNuncaVioLasTools"
//
// ⚠️ M30 Y M31 SÍ SE PISAN: las dos apuntan al MISMO texto —la línea entera del `if`—, así que
// aplicar cualquiera borra el ancla de la otra. Es daño al corpus, que es lo único que
// `arnes.Colisiones` modela, y por eso se contesta EN LAS DOS. Que no bastaba una por par lo
// midió el censo: con la clave sólo en M31, el aviso de que M30 pisa a M31 seguía saliendo —
// cada directiva responde por el daño que hace ELLA—. Y los dos motivos son distintos: M30
// pregunta si el cero avisa; M31, si el aviso sabe callarse cuando no hay espejo.
func TestElLectorAvisaCuandoElHijoNuncaVioLasTools(t *testing.T) {
	fix, err := filepath.Abs(filepath.Join("testdata", "tool-del-cliente-con-prefijo.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	conTools := Pedido{
		Mensajes: []Mensaje{TextoDeUsuario("leé el archivo")},
		Tools:    json.RawMessage(`[{"name":"leer","input_schema":{"type":"object"}}]`),
	}
	// SIN TOOLS NO HAY ESPEJO, y entonces cero listados es lo correcto: el aviso tiene que
	// callarse. Es la mitad que mata la guarda del nil, y además el caso más común de todos —un
	// turno de charla pelado—, así que es donde un aviso de más haría más ruido.
	sinTools := Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}}

	for _, caso := range []struct {
		nombre string
		guion  string
		p      Pedido
		avisa  bool
	}{
		{"el hijo pide tools/list y no hay nada que avisar", "llama-y-emite:" + fix, conTools, false},
		{"el hijo no toca el espejo y el aviso sale", "emite:" + fix, conTools, true},
		{"un turno sin tools no dispara el aviso", "emite:" + fix, sinTools, false},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			aviso, err := stderrDelTurno(t, lanzadorFalso(caso.guion), caso.p)
			if err != nil {
				t.Fatalf("Turno: %v", err)
			}
			hubo := strings.Contains(aviso, "nunca le pidió tools/list")
			if hubo != caso.avisa {
				t.Errorf("aviso=%v, se esperaba %v. stderr del motor: %q", hubo, caso.avisa, aviso)
			}
		})
	}
}

// 🔴 EL LECTOR DEL CONTADOR, que es lo que convierte al contador del espejo en un aviso. El
// contador ya tiene sus tests (espejo_test.go), pero el único que lo MIRA es el defer de
// `Lanzador.Turno` (lanzador.go:125-130), y ese defer no lo ejecutaba ningún test: borrarlo
// dejaba todo en verde y convertía el `tools/call` del hijo —que significa que el corte en el
// tool_use llegó tarde, y que el cliente recibió una respuesta razonada sobre un resultado que
// nadie produjo— en un hecho sin una sola línea en ninguna parte. Es el patrón «guarda
// definida y desconectada» del lado del lector.
//
// Hace falta un proceso que de verdad golpee el espejo: el guion `llama-y-emite` lee la ruta
// del `--mcp-config` de su propio argv, saca la URL y manda el `tools/call`. Ningún otro guion
// puede encender el contador, y por eso no alcanzaba con los que ya había.
//
// Sabotaje M27: el aviso desaparece y un corte tardío queda sin rastro
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\t\tif n := esp.EjecucionesIntentadas(); n > 0 {"
// arnes: a="\t\tif n := esp.EjecucionesIntentadas(); n < 0 {"
func TestElLectorDelContadorDelEspejoAvisaCuandoElHijoEjecuta(t *testing.T) {
	fix, err := filepath.Abs(filepath.Join("testdata", "tool-del-cliente-con-prefijo.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	p := Pedido{
		Mensajes: []Mensaje{TextoDeUsuario("leé el archivo")},
		Tools:    json.RawMessage(`[{"name":"leer","input_schema":{"type":"object"}}]`),
	}

	aviso, err := stderrDelTurno(t, lanzadorFalso("llama-y-emite:"+fix), p)
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}
	// Que el hijo haya llegado al espejo de verdad. Si el `tools/call` no llegó, el contador es
	// 0 y la ausencia del aviso sería CORRECTA: el test estaría midiendo un pedido que nunca se
	// hizo, no al lector. Un fallo acá es del guion del proceso falso, no del lanzador.
	if !strings.Contains(aviso, "intentó ejecutar") {
		t.Fatalf("el lector no avisó del tools/call del hijo. stderr del motor: %q", aviso)
	}
	// Y que diga CUÁNTOS: un aviso sin el número no distingue una carrera perdida una vez de un
	// hijo que se quedó ejecutando el turno entero.
	if !strings.Contains(aviso, "1 tool(s)") {
		t.Errorf("el aviso no dice cuántas ejecuciones hubo: %q", aviso)
	}
}

// 🔴 LA PARTICIÓN DE LA HISTORIA. Este test reemplaza a uno que pineaba el
// comportamiento EQUIVOCADO: mandar todo el transcript por stdin y dar por bueno el payload
// porque el proceso falso lo aceptaba. Medido contra el CLI 2.1.272 el 2026-10-28, ese
// camino descartaba mensajes y reemplazaba el tool_result por
// "[Tool result missing due to internal error]". Ver partirHistoria.
//
// El par assistant+user del final tiene que viajar JUNTO por stdin: es lo que empareja el
// tool_use con su tool_result. El resto va por el archivo de sesión.
func TestLaHistoriaSePartePrefijoYCola(t *testing.T) {
	u := func(s string) Mensaje { return TextoDeUsuario(s) }
	a := func(s string) Mensaje { return TextoDeAsistente(s) }

	casos := []struct {
		nombre        string
		ms            []Mensaje
		prefijo, cola []string // los roles esperados de cada lado
	}{
		{
			// Un turno nuevo no reanuda nada: no hay sesión que escribir.
			nombre: "un solo mensaje va entero por stdin",
			ms:     []Mensaje{u("hola")},
			cola:   []string{"user"},
		},
		{
			// EL CASO QUE ROMPÍA. El assistant con el tool_use y el user con el
			// tool_result tienen que salir por el mismo canal.
			nombre:  "el par tool_use/tool_result sale junto",
			ms:      []Mensaje{u("leé el archivo"), a("ya voy"), u("ok")},
			prefijo: []string{"user"},
			cola:    []string{"assistant", "user"},
		},
		{
			nombre:  "dos rondas completas",
			ms:      []Mensaje{u("1"), a("2"), u("3"), a("4"), u("5")},
			prefijo: []string{"user", "assistant", "user"},
			cola:    []string{"assistant", "user"},
		},
		{
			// Sin assistant delante, la cola es sólo el último: no hay par que romper.
			nombre:  "dos user seguidos no arrastran al anterior",
			ms:      []Mensaje{u("1"), u("2")},
			prefijo: []string{"user"},
			cola:    []string{"user"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p, co := partirHistoria(c.ms)
			if got := roles(p); !iguales(got, c.prefijo) {
				t.Errorf("prefijo: got %v, want %v", got, c.prefijo)
			}
			if got := roles(co); !iguales(got, c.cola) {
				t.Errorf("cola: got %v, want %v", got, c.cola)
			}
			// NO SE PUEDE PERDER NI DUPLICAR UN MENSAJE: la suma de las dos partes es el
			// transcript entero. Sin esto, la partición podría comerse un turno sin que
			// ningún caso de arriba lo note.
			if len(p)+len(co) != len(c.ms) {
				t.Errorf("la partición no conserva los mensajes: %d+%d != %d", len(p), len(co), len(c.ms))
			}
		})
	}
}

// LA COLA, en el formato que toma `--input-format stream-json`: un objeto por línea.
func TestLaColaViajaPorStdinConSusBloquesIntactos(t *testing.T) {
	tool, _ := json.Marshal(map[string]any{
		"type": "tool_result", "tool_use_id": "toolu_1", "content": "ok",
	})
	ms := []Mensaje{
		{Rol: "assistant", Contenido: []json.RawMessage{json.RawMessage(`{"type":"text","text":"ya voy"}`)}},
		{Rol: "user", Contenido: []json.RawMessage{tool}},
	}
	crudo, err := entradaStreamJSON(ms)
	if err != nil {
		t.Fatalf("entradaStreamJSON: %v", err)
	}

	lineas := strings.Split(strings.TrimRight(string(crudo), "\n"), "\n")
	if len(lineas) != 2 {
		t.Fatalf("líneas: got %d, want 2 (una por mensaje)", len(lineas))
	}
	quiero := []string{"assistant", "user"}
	for i, l := range lineas {
		var o struct {
			Type    string `json:"type"`
			Message struct {
				Role    string            `json:"role"`
				Content []json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(l), &o); err != nil {
			t.Fatalf("línea %d no es JSON: %v", i, err)
		}
		if o.Type != quiero[i] || o.Message.Role != quiero[i] {
			t.Errorf("línea %d: type=%q role=%q, want %q en los dos", i, o.Type, o.Message.Role, quiero[i])
		}
		if len(o.Message.Content) != 1 {
			t.Errorf("línea %d: %d bloques, want 1", i, len(o.Message.Content))
		}
	}
	// El tool_result tiene que llegar INTACTO: es lo que hace avanzar el loop del cliente,
	// y aplanarlo a texto le sacaría el tool_use_id con el que se empareja.
	if !strings.Contains(lineas[1], `"tool_use_id":"toolu_1"`) {
		t.Errorf("el tool_result perdió su tool_use_id: %s", lineas[1])
	}

	// Un rol que la API no acepta se rechaza acá, no en el proceso: el error del binario
	// sería un exit 1 con un stderr que no dice cuál mensaje estaba mal.
	if _, err := entradaStreamJSON([]Mensaje{{Rol: "system", Contenido: []json.RawMessage{[]byte(`{}`)}}}); err == nil {
		t.Error("un rol inválido debe rechazarse antes de lanzar el proceso")
	}
	if _, err := entradaStreamJSON([]Mensaje{{Rol: "user"}}); err == nil {
		t.Error("un mensaje sin contenido debe rechazarse")
	}
}

// EL PREFIJO, en el formato de archivo de sesión. Tres de estos campos tienen una falla
// SILENCIOSA, que es la razón por la que este test existe en vez de confiar en el E2E:
// el `model` resuelto (sin él se cae el thinking firmado), el `isMeta` de la última línea
// (sin él aparece un "Continue from where you left off." que el cliente nunca escribió) y
// el `stop_reason` del assistant con tool_use.
func TestElPrefijoViajaComoArchivoDeSesion(t *testing.T) {
	toolUse := json.RawMessage(`{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}`)
	ms := []Mensaje{
		TextoDeUsuario("leé el archivo"),
		{Rol: "assistant", Contenido: []json.RawMessage{toolUse}},
	}
	crudo, err := sesionJSONL(ms, "claude-sonnet-5", `C:\Proyectos\Musubi`, "2.1.272")
	if err != nil {
		t.Fatalf("sesionJSONL: %v", err)
	}

	lineas := strings.Split(strings.TrimRight(string(crudo), "\n"), "\n")
	if len(lineas) != 2 {
		t.Fatalf("líneas: got %d, want 2", len(lineas))
	}

	type linea struct {
		ParentUUID *string         `json:"parentUuid"`
		Type       string          `json:"type"`
		UUID       string          `json:"uuid"`
		SessionID  string          `json:"sessionId"`
		Cwd        string          `json:"cwd"`
		Version    string          `json:"version"`
		IsMeta     bool            `json:"isMeta"`
		Message    json.RawMessage `json:"message"`
	}
	var ls []linea
	for i, l := range lineas {
		var o linea
		if err := json.Unmarshal([]byte(l), &o); err != nil {
			t.Fatalf("línea %d no es JSON: %v", i, err)
		}
		ls = append(ls, o)
	}

	// LA CADENA. La primera línea no tiene padre y cada siguiente apunta al uuid de la
	// anterior: así el CLI reconstruye el orden. Encadenar mal no da un error, da una
	// historia desordenada, que es justo el defecto que este camino vino a arreglar.
	if ls[0].ParentUUID != nil {
		t.Errorf("la primera línea no debe tener padre: %v", *ls[0].ParentUUID)
	}
	if ls[1].ParentUUID == nil || *ls[1].ParentUUID != ls[0].UUID {
		t.Errorf("la línea 1 no apunta al uuid de la 0: %v vs %q", ls[1].ParentUUID, ls[0].UUID)
	}
	if ls[0].SessionID != ls[1].SessionID || ls[0].SessionID == "" {
		t.Errorf("las dos líneas tienen que compartir un sessionId no vacío: %q vs %q", ls[0].SessionID, ls[1].SessionID)
	}
	if ls[0].Cwd != `C:\Proyectos\Musubi` || ls[0].Version != "2.1.272" {
		t.Errorf("cwd/version: got %q/%q", ls[0].Cwd, ls[0].Version)
	}

	// isMeta SÓLO en la última. Marcar la primera no haría nada visible; no marcar la
	// última mete un mensaje ajeno en el contexto del modelo.
	if ls[0].IsMeta {
		t.Error("isMeta no va en una línea del medio")
	}
	if !ls[1].IsMeta {
		t.Error("falta isMeta en la última línea del prefijo: el CLI va a agregar «Continue from where you left off.»")
	}

	var asistente struct {
		Role       string `json:"role"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		ID         string `json:"id"`
	}
	if err := json.Unmarshal(ls[1].Message, &asistente); err != nil {
		t.Fatalf("el message del assistant no es JSON: %v", err)
	}
	if asistente.Model != "claude-sonnet-5" {
		t.Errorf("model: got %q, want el id resuelto — con un alias el CLI descarta el thinking firmado", asistente.Model)
	}
	if asistente.StopReason != "tool_use" {
		t.Errorf("stop_reason: got %q, want tool_use — un turno con tool_use que diga end_turn le miente al modelo", asistente.StopReason)
	}
	if !strings.HasPrefix(asistente.ID, "msg_") {
		t.Errorf("el id del mensaje tiene que parecerse al de la API: %q", asistente.ID)
	}

	// Un assistant sin tool_use cierra el turno, y declararlo tool_use dejaría al modelo
	// esperando un resultado que nunca va a llegar.
	solo, err := sesionJSONL([]Mensaje{TextoDeAsistente("listo")}, "claude-sonnet-5", ".", "2.1.272")
	if err != nil {
		t.Fatalf("sesionJSONL: %v", err)
	}
	if !strings.Contains(string(solo), `"stop_reason":"end_turn"`) {
		t.Errorf("un assistant de texto tiene que cerrar con end_turn: %s", solo)
	}

	// Las mismas validaciones que la cola: el prefijo también se rechaza acá y no en el
	// proceso.
	if _, err := sesionJSONL([]Mensaje{{Rol: "system", Contenido: []json.RawMessage{[]byte(`{}`)}}}, "m", ".", "v"); err == nil {
		t.Error("un rol inválido debe rechazarse antes de escribir la sesión")
	}
	if _, err := sesionJSONL([]Mensaje{{Rol: "user"}}, "m", ".", "v"); err == nil {
		t.Error("un mensaje sin contenido debe rechazarse")
	}
}

// EL ARCHIVO EN EL DISCO. `--resume` quiere una ruta absoluta a un .jsonl, y el temporal
// tiene que desaparecer: un turno servido no deja rastro de la conversación del cliente.
func TestElArchivoDeSesionEsAbsolutoYSeLimpia(t *testing.T) {
	l := &Lanzador{}
	ruta, limpiar, err := l.archivoDeSesion([]Mensaje{TextoDeUsuario("hola")}, Pedido{Modelo: "claude-sonnet-5"})
	if err != nil {
		t.Fatalf("archivoDeSesion: %v", err)
	}
	if !filepath.IsAbs(ruta) {
		t.Errorf("la ruta tiene que ser absoluta: %q", ruta)
	}
	if filepath.Ext(ruta) != ".jsonl" {
		t.Errorf("la extensión tiene que ser .jsonl: %q", ruta)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("el archivo no quedó escrito: %v", err)
	}
	limpiar()
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("el temporal tiene que borrarse: %v", err)
	}

	// Sin prefijo no hay archivo, y la limpieza igual tiene que ser invocable: el llamador
	// la difiere sin mirar si hubo sesión.
	ruta, limpiar, err = l.archivoDeSesion(nil, Pedido{})
	if err != nil || ruta != "" {
		t.Errorf("sin prefijo: ruta %q, err %v", ruta, err)
	}
	limpiar()
}

func roles(ms []Mensaje) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Rol)
	}
	return out
}

func iguales(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// EL AISLAMIENTO DEL ENTORNO. Las CLAUDE_CODE_* heredadas hacen que el hijo le hable a la
// sesión padre, y las MUSUBI_* le hacen cargar el MCP de Musubi y recursar sobre esta misma
// máquina. La API key se saca porque este paquete existe para servir el turno con la
// SUSCRIPCIÓN: heredarla lo haría facturar por API con un stream idéntico, o sea sin
// ninguna señal de que pasó.
func TestElEntornoDelHijoNoHeredaLaSesionNiLaAPIKey(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"HOME=/home/x",
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"CLAUDE_CODE_SSE_PORT=5000",
		"MUSUBI_TOKEN=secreto",
		"MUSUBI_CENTRAL_URL=https://x",
		"ANTHROPIC_API_KEY=sk-ant-xxx",
		"ANTHROPIC_MODEL=claude-opus-5",
	}

	got := strings.Join(entornoAislado(base, false), " ")
	for _, fuera := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT", "MUSUBI_TOKEN", "MUSUBI_CENTRAL_URL", "ANTHROPIC_API_KEY"} {
		if strings.Contains(got, fuera) {
			t.Errorf("%s llegó al hijo: %s", fuera, got)
		}
	}
	// Y lo que NO se toca: sin PATH el binario no se resuelve, sin HOME no encuentra las
	// credenciales de la suscripción.
	for _, dentro := range []string{"PATH=/usr/bin", "HOME=/home/x", "ANTHROPIC_MODEL=claude-opus-5"} {
		if !strings.Contains(got, dentro) {
			t.Errorf("falta %s en el entorno del hijo: %s", dentro, got)
		}
	}

	// Con el opt-in explícito, la key pasa.
	if !strings.Contains(strings.Join(entornoAislado(base, true), " "), "ANTHROPIC_API_KEY") {
		t.Error("ConservarAPIKey no dejó pasar la key")
	}
}

func TestPedidoSinMensajes(t *testing.T) {
	err := (&Lanzador{}).Turno(context.Background(), Pedido{}, func(Evento) error { return nil })
	if err == nil {
		t.Fatal("se esperaba error")
	}
	// No tiene que haber intentado lanzar nada.
	if strings.Contains(err.Error(), "no se pudo lanzar") {
		t.Errorf("lanzó el proceso con un pedido vacío: %v", err)
	}
}

// El stderr de un proceso que falló puede ser un stack de node entero, y lo que explica la
// falla está al final. Se guarda sólo la cola para no acumular megabytes de un turno que ya
// fracasó.
func TestElStderrSeAcotaYGuardaElFinal(t *testing.T) {
	var c colaDeTexto
	c.Write([]byte(strings.Repeat("a", topeStderr*2)))
	c.Write([]byte("LA CAUSA REAL"))
	got := c.cola()
	if !strings.Contains(got, "LA CAUSA REAL") {
		t.Error("se perdió el final del stderr, que es donde está la causa")
	}
	if len(got) > topeStderr+32 {
		t.Errorf("la cola no se acotó: %d bytes", len(got))
	}
	var vacia colaDeTexto
	if vacia.cola() != "" {
		t.Errorf("sin stderr no debe agregar nada al error, got %q", vacia.cola())
	}
}

// UN TOPE NEGATIVO NO ES UN PLAZO YA VENCIDO. El yaml acepta `turn_timeout_seconds: -1` sin
// chistar, y de ahí sale un Duration negativo que `context.WithTimeout` trata como un plazo
// cumplido: el turno muere antes de que el hijo escriba un byte, y el error dice «por tiempo»
// como si el usuario hubiera esperado los diez minutos. El cero cae al defecto desde el
// principio; el negativo tiene que caer al MISMO lugar, porque es el mismo error de
// configuración tecleado distinto. Y el espejo ya se protege solo (ver abrirEspejo), así que
// sin esta guarda los dos relojes del turno quedan resueltos con criterios opuestos.
//
// Sabotaje M39: el lanzador no resuelve el plazo y se queda con el crudo del yaml
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\tespera := plazoDelTurno(l.Espera)"
// arnes: a="\tespera := l.Espera"
func TestUnTopeNegativoNoMataElTurnoEnElActo(t *testing.T) {
	fix, err := filepath.Abs(filepath.Join("testdata", "texto-y-tool-dos-turnos.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	l := lanzadorFalso("emite:" + fix)
	l.Espera = -5 * time.Second

	var evs []Evento
	err = l.Turno(context.Background(), Pedido{Mensajes: []Mensaje{TextoDeUsuario("hola")}},
		func(e Evento) error { evs = append(evs, e); return nil })
	if err != nil {
		t.Fatalf("un tope negativo cortó el turno: %v", err)
	}
	// Y sirvió el turno de verdad, no sólo su arranque: con el plazo vencido lo único que
	// alcanza a salir es el `start`, que es lo que mide TestElTurnoSeCortaPorTiempoYLoDice.
	if len(evs) < 2 {
		t.Fatalf("eventos emitidos con el tope negativo: %v", tipos(evs))
	}
}

// EL RESOLVEDOR DEL PLAZO CURA TODO EL DOMINIO NO POSITIVO, y el plural es el invariante: no
// dice «el cero y el -1 caen al defecto», dice «cualquier valor con el que el turno no podría
// arrancar cae al defecto». La diferencia no es retórica y se midió el 2026-10-29: los dos
// custodios del negativo que existían eran tests de PUNTO —el de acá arriba con -5s, el de la
// fábrica con -1—, así que una guarda en `== 0 || == -1` los dejaba verdes a los dos y un
// `turn_timeout_seconds: -2` seguía naciendo con el contexto vencido. Un punto no cubre un
// dominio, y este dominio es infinito por abajo.
//
// Se mide la función y no los tres sitios que la llaman: el invariante vive en un lugar desde
// que se extrajo, y medirlo tres veces por los costados sería volver a tener tres copias —esta
// vez de la prueba—.
//
// Sabotaje M42: el resolvedor del plazo solo cura el cero
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="func plazoDelTurno(d time.Duration) time.Duration {\n\tif d <= 0 {"
// arnes: a="func plazoDelTurno(d time.Duration) time.Duration {\n\tif d == 0 {"
func TestElPlazoDelTurnoCuraTodoElDominioNoPositivo(t *testing.T) {
	// El mínimo de la máquina, escrito sin importar `math`: es el negativo más extremo que un
	// Duration puede tener, y el que un desbordamiento aguas arriba produciría.
	const minimoDeLaMaquina = time.Duration(-1 << 63)

	for _, caso := range []struct {
		nombre string
		dado   time.Duration
		quiero time.Duration
	}{
		{"el cero de un yaml sin la clave", 0, DemoraPorDefecto},
		{"el -1 que se teclea a mano", -1 * time.Second, DemoraPorDefecto},
		{"un -2, que ningún test de punto cubría", -2 * time.Second, DemoraPorDefecto},
		{"un nanosegundo negativo, que sigue siendo plazo vencido", -1, DemoraPorDefecto},
		{"el negativo del propio defecto", -DemoraPorDefecto, DemoraPorDefecto},
		{"el mínimo de la máquina", minimoDeLaMaquina, DemoraPorDefecto},
		{"un positivo pasa intacto", 2 * time.Minute, 2 * time.Minute},
		{"un nanosegundo positivo también pasa intacto", 1, 1},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			if got := plazoDelTurno(caso.dado); got != caso.quiero {
				t.Errorf("plazoDelTurno(%v) = %v, quiero %v", caso.dado, got, caso.quiero)
			}
		})
	}
}

// EL DEFECTO SON DIEZ MINUTOS, Y ESTA IGUALDAD ES LO ÚNICO QUE LO ANCLA. Medido el 2026-10-29
// sobre las 30 menciones de la constante en el paquete: TODAS la usan como referencia —`.String()`
// en la descripción de la fábrica, `> DemoraPorDefecto` en los timeouts del espejo, pasarla como
// argumento a `abrirEspejo`— y ninguna la compara contra un valor. O sea que bajarla a un segundo
// dejaba el paquete ENTERO en verde, con todos los turnos largos muriendo por reloj: un custodio
// que mide un PREDICADO sobre una constante no custodia su VALOR. Y acá el valor ES el invariante,
// porque este paquete existe para servir turnos con thinking largo sobre contexto grande, que es
// exactamente lo que un plazo corto mata.
//
// La igualdad y no un `>=`: una cota deja pasar cualquier valor mayor, incluida una hora, y un
// turno que cuelga una hora es un proceso zombi con un cliente esperando del otro lado. Si el
// número tiene que cambiar, que cambie acá a mano y que se vea en el diff.
//
// Sabotaje M43: el defecto del plazo se achica a un segundo
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="const DemoraPorDefecto = 10 * time.Minute"
// arnes: a="const DemoraPorDefecto = 1 * time.Second"
func TestElDefectoDelPlazoSonDiezMinutos(t *testing.T) {
	if DemoraPorDefecto != 10*time.Minute {
		t.Errorf("DemoraPorDefecto = %v, quiero 10m: el defecto es el techo de TODO turno que no "+
			"configure el suyo, y un plazo corto corta el turno largo —el caso que este paquete "+
			"existe para servir— con un error que dice «por tiempo» y no nombra la causa",
			DemoraPorDefecto)
	}
}
