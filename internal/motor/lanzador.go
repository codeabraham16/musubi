package motor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Lanzador corre el binario `claude` en modo headless y traduce su stream. Un Lanzador se
// puede compartir entre requests (no tiene estado mutable); el estado por turno vive en el
// Traductor que crea cada llamada a Turno.
type Lanzador struct {
	// Binario a ejecutar. Vacío = "claude", resuelto por PATH.
	Binario string

	// Espera es el tope de tiempo de un turno. Cero = DemoraPorDefecto.
	Espera time.Duration

	// ConservarAPIKey deja pasar ANTHROPIC_API_KEY al hijo. Por defecto se saca, y no es
	// una decisión cosmética: este paquete existe para servir el turno con la SUSCRIPCIÓN
	// del usuario, y una API key en el entorno la haría facturar por API sin que nadie se
	// dé cuenta — el stream sale igual, así que no hay forma de notarlo desde el cliente.
	ConservarAPIKey bool

	// Entorno de base. Nil = el del proceso. Se expone para los tests.
	Entorno []string

	// VersionDeSesion es la versión que se escribe en el archivo de sesión. Vacío =
	// VersionDeSesionPorDefecto. Se expone para poder seguir al CLI sin recompilar.
	VersionDeSesion string
}

// El Lanzador es uno de los dos motores. Esta línea no es decorativa: si la firma de Turno
// se desvía de la interfaz, el build falla acá en vez de en el handler.
var _ Motor = (*Lanzador)(nil)

// DemoraPorDefecto es el tope de un turno. Generoso a propósito: un turno con thinking
// largo sobre un contexto grande tarda, y cortarlo por reloj es peor que esperarlo.
const DemoraPorDefecto = 10 * time.Minute

// TopeDelPlazo es el techo de `turn_timeout_seconds`, y es la otra mitad del acuerdo que
// DemoraPorDefecto abre por abajo. Un día es absurdamente holgado —144× el defecto— a propósito:
// no está para disciplinar a nadie sobre cuánto puede tardar un turno, sino porque arriba de
// cierto número el valor DEJA DE SIGNIFICAR LO QUE DICE.
//
// 🔴 EL PROBLEMA NO ES UN TURNO LARGO: ES QUE LA ARITMÉTICA ENVUELVE Y EL RESULTADO SIGUE SIENDO
// POSITIVO, así que `plazoDelTurno` lo deja pasar y nadie se entera. Dos modos, los dos medidos
// con el compilador y no derivados a mano:
//
//	9223371977 … 9223372036 → la multiplicación todavía entra en int64 (`dur>0` da true), pero la
//	  suma `espera + margenDelIdle` de espejo.go desborda y deja el IdleTimeout NEGATIVO. Y
//	  net/http no lee un negativo como «ya vencido» sino como «sin deadline» —`if d := c.server.
//	  idleTimeout(); d > 0`, server.go:2113—, así que el listener del espejo no vence NUNCA: lo
//	  contrario exacto de lo que ese comentario promete tres líneas más arriba. ⚠️ Esa ventana
//	  CUELGA DE margenDelIdle: con otro margen se corre, así que el número que importa no es el
//	  borde sino que el borde EXISTA. El custodio pide que la suma crezca, no que valga esto.
//	18446744074 y para arriba → envuelve la MULTIPLICACIÓN: 18446744074e9 − 2⁶⁴ = +290448384 ns.
//	  Un tope de 290 ms, positivo y plausible, que mata cada turno antes de la primera palabra con
//	  un error que dice «por tiempo» — el mismo síntoma que el negativo, por la causa opuesta. Y
//	  no es un valor mágico suelto: de 11 enteros consecutivos muestreados ahí, 7 dan un plazo
//	  positivo de menos de una hora (290 ms, 1,29 s, 2,29 s, …).
//
// Y LOS DOS SÓLO SE PUEDEN VER SOBRE EL ENTERO. Después de la multiplicación el desborde ya
// ocurrió y lo que queda es indistinguible de un plazo que alguien quiso de verdad: no hay
// guarda posible aguas abajo. Por eso esto se chequea en la fábrica y no acá en `plazoDelTurno`,
// que recibe un Duration y llega tarde.
const TopeDelPlazo = 24 * time.Hour

// plazoDelTurno resuelve el tope de un turno: CUALQUIER valor no positivo cae a
// DemoraPorDefecto.
//
// Existe porque esta línea estaba escrita TRES veces —en el Lanzador, en la fábrica que lo
// describe y en el espejo que le cuelga el idle—, y tres copias de un acuerdo son tres lugares
// donde puede quedar distinto. No es hipotético: la del espejo ya había estado en `== 0`
// mientras las otras dos decían `<= 0`, y con eso los dos relojes del mismo turno quedaban
// resueltos con criterios opuestos.
//
// EL NEGATIVO NO ES UN CAPRICHO. Un `turn_timeout_seconds: -1` pasa la validación del yaml sin
// chistar y da un Duration que `context.WithTimeout` trata como plazo ya cumplido: el turno
// muere antes de que el hijo escriba un byte, y el error dice «por tiempo» como si el usuario
// hubiera esperado los diez minutos. Es el mismo error de configuración que el cero, tecleado
// distinto, y tiene que caer al mismo lugar.
//
// ⚠️ Y «no positivo ⇒ defecto» NO ES UNA REGLA GENERAL DEL REPO, así que esto no se copia a
// ciegas: `config.CognitionConfig.EffectiveMotorQuotaPerHour` usa la semántica OPUESTA para el
// negativo —cero ⇒ default, negativo ⇒ SIN límite, un opt-out explícito— y ahí un `<= 0`
// convertiría el opt-out en el default. La diferencia tiene su razón: una cuota tiene un «sin
// límite» que alguien puede querer, y un plazo no —un turno eterno es un proceso colgado con un
// cliente esperando del otro lado—.
func plazoDelTurno(d time.Duration) time.Duration {
	if d <= 0 {
		return DemoraPorDefecto
	}
	return d
}

// plazoFueraDeRango dice si `turn_timeout_seconds` se pasa de TopeDelPlazo.
//
// Toma el ENTERO DEL YAML y no un Duration, y eso es la función entera: ver el desborde exige
// mirar el valor ANTES de convertirlo. La doc de TopeDelPlazo tiene los dos números.
//
// EL NEGATIVO NO CAE ACÁ A PROPÓSITO. Es el error que `plazoDelTurno` ya manda al defecto, y
// contestarlo también desde esta función sería una segunda copia del mismo acuerdo — justo lo
// que `plazoDelTurno` existe para cerrar. Acá se mira un solo borde: el de arriba.
func plazoFueraDeRango(seg int) bool {
	return seg > int(TopeDelPlazo/time.Second)
}

// topeDeLinea es el tamaño máximo de una línea del stream. Hace falta subirlo del default
// de bufio (64 KB) porque el `system/init` del harness trae el entorno entero —la lista de
// tools, los plugins, las rutas— y pasa holgadamente los 64 KB. Con el default, el turno
// moría con `token too long` en la PRIMERA línea, antes de leer una sola palabra del
// modelo.
const topeDeLinea = 16 << 20

// VersionDeSesionPorDefecto es la versión que va en cada línea del archivo de sesión.
//
// ES UN NÚMERO FRÁGIL Y CONVIENE SABERLO: el formato del archivo de sesión es INTERNO del
// CLI, no una interfaz publicada, y lo que está acá se midió contra la 2.1.272. Una
// actualización del binario puede cambiarlo sin aviso y el síntoma NO va a ser un error:
// va a ser un turno que pierde la historia, que es justo el defecto que este camino existe
// para arreglar. Por eso el E2E multi-mensaje de integracion_test.go no es opcional.
const VersionDeSesionPorDefecto = "2.1.272"

// Turno sirve un turno completo: lanza el proceso, traduce su stream y llama a `emitir`
// por cada evento del protocolo. Vuelve cuando el turno cerró (o falló), y para entonces
// el proceso ya está terminado.
//
// EL CORTE ES LA PIEZA CENTRAL, no una optimización. `claude -p` es un agente con su
// propio loop: si se lo deja seguir, ejecuta la tool que acaba de pedir y arranca otro
// turno contra el modelo. Dos cosas lo hacen inaceptable acá: el cliente remoto espera UNA
// respuesta por request, y las tools las tiene que ejecutar ÉL —con sus permisos y su
// aprobación del usuario—, no este proceso de nuestro lado.
//
// LO QUE EL CORTE HACE Y LO QUE NO. Medido el 2026-10-28, el turno entero del modelo llega
// ANTES del `message_delta`, así que cortar ahí da la respuesta completa. Lo que NO hace es
// impedir que el hijo actúe, y creerlo lleva a diseñar mal: en la captura
// testdata/thinking-con-firma.jsonl la línea 20 es el `tool_result` de una tool que el hijo
// YA EJECUTÓ, y precede al `message_delta` de la línea 21. El corte acota el daño a un
// turno; no es un sandbox. Lo que de verdad limita al hijo son DOS banderas y no una:
// `--setting-sources ""` lo deja sin hooks ni skills, y `--strict-mcp-config` es la que le
// corta los servidores MCP — medido el 2026-10-29, con `--setting-sources ""` puesta el hijo
// igual veía los conectores del dueño (ver argumentosCLI). Tampoco se puede delegar a
// `--max-turns 1`, que significa «un turno de USUARIO» y alcanza a ejecutar las tools del turno.
func (l *Lanzador) Turno(ctx context.Context, p Pedido, emitir func(Evento) error) error {
	if len(p.Mensajes) == 0 {
		return errors.New("pedido sin mensajes")
	}

	// LA HISTORIA SE PARTE EN DOS, y esto es lo que hace que un agent loop funcione. Ver
	// partirHistoria: el CLI NO interpreta su stdin como un transcript.
	prefijo, cola := partirHistoria(p.Mensajes)
	entrada, err := entradaStreamJSON(cola)
	if err != nil {
		return err
	}

	rutaSesion, limpiar, err := l.archivoDeSesion(prefijo, p)
	if err != nil {
		return err
	}
	defer limpiar()

	// EL PLAZO SE RESUELVE ACÁ ARRIBA y no donde nace el ctx, porque el espejo lo necesita: de
	// este valor sale el `IdleTimeout` de su listener, que tiene que sobrevivirle al turno. Con
	// la resolución abajo, el espejo sólo podía colgar de la constante, y ahí el yaml invertía
	// el orden de los relojes (ver la doc de abrirEspejo). Un solo lugar que resuelve el cero
	// Y EL NEGATIVO: un `turn_timeout_seconds: -1` da un Duration que context.WithTimeout
	// trata como plazo ya cumplido, y el turno moría antes de lanzar el proceso.
	espera := plazoDelTurno(l.Espera)

	// EL ESPEJO VA ANTES DEL ctx CON TIMEOUT a propósito: su ciclo de vida es el del turno
	// ENTERO, y atarlo a un ctx que vence dejaría al hijo sin tools justo en el turno largo,
	// que es el que más las necesita. Lo cierra el defer, no un deadline.
	esp, err := abrirEspejo(p, espera)
	if err != nil {
		return err
	}
	defer esp.cerrar()
	// EL CONTADOR DEL ESPEJO TIENE QUE TENER UN LECTOR, o es una guarda definida y
	// desconectada: un número que nadie mira no avisa nada. Y lo que avisa importa, porque es
	// una CARRERA. El turno se corta en el `tool_use` —`bombear` vuelve en cuanto el traductor
	// se da por cerrado, y el `message_delta` con `stop_reason: tool_use` sale antes de que el
	// agente ejecute nada—, así que en el camino normal el hijo nunca llega a pedirle al espejo
	// que corra la tool. Un `tools/call` contado significa que esa carrera se perdió, y
	// entonces el hijo siguió razonando sobre un resultado que nadie produjo.
	//
	// Va a stderr y no al error del turno: cuando esto pasa el turno YA se sirvió —el `done`
	// salió antes—, y devolverle un error al cliente después del último evento sería contarle
	// un fracaso sobre algo que recibió completo.
	defer func() {
		if n := esp.EjecucionesIntentadas(); n > 0 {
			fmt.Fprintf(os.Stderr, "motor: el hijo intentó ejecutar %d tool(s) del cliente en el espejo; "+
				"el turno se sirvió pero el corte en el tool_use llegó tarde\n", n)
		}
		// Y EL FALLO CONTRARIO, QUE ES EL QUE PASA CALLADO. El de arriba se nota: el cliente
		// recibe un resultado que no pidió. Éste no se nota NUNCA sin este aviso — si el hijo no
		// encontró el espejo, el modelo contestó sin saber que las tools del cliente existían, y
		// lo que vuelve es una respuesta perfectamente formada. El cliente no puede distinguirla
		// de una en la que el modelo miró sus tools y eligió no usar ninguna, así que sin esta
		// línea un `--mcp-config` que dejó de llegar se ve como un modelo que se volvió vago.
		//
		// La condición es `esp != nil` y no el contador a secas: un turno sin tools no abre
		// espejo, y ahí cero listados es lo correcto. Sin esa mitad, el aviso saldría en todos
		// los turnos pelados y en dos días nadie lo lee.
		if esp != nil && esp.ListadosServidos() == 0 {
			fmt.Fprintf(os.Stderr, "motor: el hijo nunca le pidió tools/list al espejo; "+
				"el turno se contestó sin que el modelo viera las tools del cliente\n")
		}
	}()
	if esp != nil {
		// El CLI renombra lo que viene por MCP, y acá se deshace. Va envuelto en el `emitir`
		// del Lanzador y NO dentro del traductor, que es el camino que parecía obvio: el
		// traductor lo comparten los dos motores, y el ClienteAPI nunca pone ese prefijo. Una
		// tool que el cliente declare llamada `mcp__pi__algo` —y pi declara sus propias tools
		// MCP con esa forma— volvería renombrada a algo que el cliente no tiene en su tabla.
		emitir = sinElPrefijoDelEspejo(emitir)
	}

	ctx, cancelar := context.WithTimeout(ctx, espera)
	defer cancelar()

	bin := l.Binario
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, argumentosCLI(p, rutaSesion, esp)...)
	cmd.Dir = p.Dir
	cmd.Env = entornoAislado(l.entornoBase(), l.ConservarAPIKey)

	// WaitDelay cubre los PIPES retenidos, no un hijo que ignore la señal: la señal es
	// TerminateProcess/SIGKILL y no se ignora. Lo que sí puede pasar es que un NIETO herede
	// el stdout o el stderr y los mantenga abiertos después de que el hijo muera, y
	// entonces las copiadoras de os/exec se quedan esperando un EOF que no llega
	// (go.dev/issue/23019, citado en exec.go:860). Sin esto, Wait se cuelga.
	//
	// NINGÚN TEST LA ENCIENDE: borrar esta línea deja los 26 en verde, porque ningún guion
	// del proceso falso deja un nieto. Es una guarda definida y desconectada, y queda a
	// propósito —con este comentario— porque el día que `claude.exe` deje de ser un proceso
	// único el síntoma no sería un rojo sino un request que no vuelve. El destrabe real de
	// NUESTRO extremo lo hace el AfterFunc de abajo, que no depende de quién herede qué.
	cmd.WaitDelay = 5 * time.Second

	// NO HACE FALTA MATAR UN ÁRBOL DE PROCESOS, y conviene saberlo antes de agregar un Job
	// Object en Windows o un process group en Linux «por las dudas»: medido el 2026-10-28
	// siguiendo los PIDs de un turno real, `claude` es UN proceso nativo —no un lanzador
	// que deje nietos de node— y tras el corte sobrevivieron 0 procesos.

	cmd.Stdin = bytes.NewReader(entrada)
	salida, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout del proceso: %w", err)
	}
	var errBuf colaDeTexto
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo lanzar %q: %w", bin, err)
	}

	// CIERRA NUESTRO EXTREMO DEL PIPE AL VENCER EL CTX. `bombear` está bloqueado en un Read
	// del stdout, y matar al hijo no lo destraba si alguien más tiene el otro extremo
	// abierto: os/exec sólo cierra los pipes cuando una copiadora SUYA sigue trabada
	// (la rama default de exec.go:857-873), y con un nieto que heredó sólo el stdout no
	// cierra nada. Cerrando de este lado, el Read vuelve por CancelIoEx y el turno sale en
	// ~Espera herede quien herede.
	detenerCierre := context.AfterFunc(ctx, func() { _ = salida.Close() })

	// LA COSECHA VA EN UN DEFER, Y NO ES ESTILO. `emitir` lo escribe el llamador y corre
	// dentro de esta goroutine: si PANICA, el unwinding saltea cualquier cancelar()+Wait()
	// que esté en línea recta. El hijo igual muere por el defer del ctx, pero la goroutine
	// watchCtx de os/exec se queda bloqueada PARA SIEMPRE en su send final (exec.go:881, un
	// canal sin buffer cuyo único receptor es Wait), reteniendo el handle del proceso, el
	// extremo de lectura del stdout y el transcript entero que tiene el Stdin.
	//
	// ⚠️ ACÁ DECÍA «Y net/http recupera el panic por request, así que no se cae: se ACUMULA», y
	// la primera mitad es FALSA. Medida el 2026-10-29 con dos brazos en la misma corrida: el
	// recuperador de net/http (`(*conn).serve.func1`, server.go:1917) aparece en la pila de un
	// panic de la goroutine del HANDLER y está AUSENTE en la del motor, que corre en la
	// goroutine aparte que larga `bombearTurno`. Mientras no hubo otro recover, un panic acá no
	// se acumulaba: mataba el proceso entero. La frase costó una ronda de revisión, que la
	// citó como premisa y concluyó lo contrario de lo que pasaba.
	//
	// La conclusión igual vale hoy, por el recover que SÍ vive en esa goroutine (el de
	// `bombearTurno`, en internal/mcp): el panic se degrada al error de UN turno y el servidor
	// sigue atendiendo, así que una goroutine filtrada acá se ACUMULA turno a turno. Y este
	// defer corre en los dos mundos, porque el unwinding de un panic SÍ corre los defers de su
	// propia goroutine — lo que no corre son los de las otras, que mueren con el proceso.
	//
	// Medido con un A/B de un factor el 2026-10-28: tres turnos que panican dejan 3
	// goroutines con el Wait en línea recta y 0 con el Wait en un defer.
	cosechado := false
	defer func() {
		if !cosechado {
			cancelar()
			_ = cmd.Wait()
		}
	}()

	cerrado, errBombeo := bombear(salida, emitir)
	detenerCierre()

	// El motivo se mira ANTES de cancelar: después, ctx.Err() dice `canceled` siempre y
	// sería imposible distinguir «se venció el tope» de «lo cortamos nosotros».
	motivo := ctx.Err()

	// SE CANCELA SIEMPRE, Y ANTES DEL WAIT. Cuando bombear vuelve ya no hay nada más que
	// leerle al proceso —cerró el turno, falló la emisión o se terminó el stream—, así que
	// en los tres casos sobra. El `defer cancelar()` de arriba NO alcanza: corre al salir
	// de la función, o sea DESPUÉS del Wait, y entonces el Wait se queda esperando a que
	// el proceso termine por su cuenta. Medido con el proceso falso: un error del
	// consumidor colgaba el turno 10 segundos, el tiempo entero que el hijo tardaba en
	// salir solo. En producción eso es un `claude` vivo por cada cliente que se desconecta.
	cancelar()
	errEspera := cmd.Wait()
	cosechado = true

	switch {
	case errors.Is(errBombeo, ErrDelConsumidor):
		// VA ANTES DE `cerrado`, y el orden es el hallazgo: el traductor marca el turno
		// como cerrado ANTES de que su último evento se emita, así que un `emitir` que
		// falle justo en el `done` deja cerrado=true Y un error. Con el orden al revés
		// ganaba `cerrado`, el error del consumidor se perdía y el llamador creía que
		// había servido un turno que el cliente nunca recibió.
		return errBombeo

	case cerrado:
		// El turno se sirvió completo. Que el proceso haya muerto por la señal que le
		// mandamos nosotros NO es un error, y mirar su código de salida acá sería
		// reportar como falla el éxito del corte.
		return nil

	case errors.Is(motivo, context.DeadlineExceeded):
		return fmt.Errorf("el turno se cortó por tiempo (%s)%s", espera, errBuf.cola())

	case errors.Is(motivo, context.Canceled):
		// EL CASO MÁS FRECUENTE DE UN SERVIDOR, y antes salía como un crash. Un cliente
		// HTTP que cierra la conexión cancela el ctx, y sin este case caía al default y
		// se reportaba «el proceso terminó sin respuesta del modelo: exit status 1» —el
		// mismo texto exacto que un binario roto—. Eso cuesta diagnóstico cada vez que
		// algo ande mal de verdad.
		return fmt.Errorf("el llamador canceló el turno%s", errBuf.cola())

	case errBombeo != nil:
		// El stream se cortó solo: una línea de 16 MB, un pipe roto. VA DESPUÉS DE LOS
		// DOS CASES DEL CTX, y el orden lo encontró un test: el AfterFunc de arriba cierra
		// NUESTRO extremo del pipe cuando el plazo vence, así que el Read vuelve con
		// `file already closed` —un error de lectura legítimo, pero causado por nosotros—.
		// Con este case antes del reloj, un turno cortado por tiempo se reportaba
		// «leer el stream: read |0: file already closed» en vez de decir que se venció, o
		// sea que el destrabe tapaba justo el motivo que explica por qué se destrabó.
		return errBombeo

	default:
		// El proceso terminó sin cerrar un turno: no hubo respuesta del modelo. El stderr
		// es lo único que dice por qué, así que va en el error.
		if errEspera != nil {
			return fmt.Errorf("el proceso terminó sin respuesta del modelo: %w%s", errEspera, errBuf.cola())
		}
		return fmt.Errorf("el proceso terminó sin respuesta del modelo%s", errBuf.cola())
	}
}

// ErrDelConsumidor envuelve los errores que vienen del callback `emitir`, para separarlos
// de los errores de LECTURA del stream.
//
// La distinción manda el orden del switch de Turno y no es cosmética: un error del
// consumidor tiene que ganarle a «el turno cerró» —el cliente no recibió el último
// evento—, mientras que un error de lectura tiene que PERDER contra el motivo del ctx,
// porque cuando el plazo vence somos nosotros los que cerramos el pipe y el Read vuelve con
// `file already closed`. Sin el sentinel los dos son un `error` pelado y no hay forma de
// darles órdenes distintos.
var ErrDelConsumidor = errors.New("el consumidor rechazó un evento")

// bombear lee el stream línea por línea, traduce y emite. Devuelve si el turno cerró y el
// primer error que impida seguir. Una línea ilegible NO aborta: el harness intercala
// líneas que no son del protocolo, y tirar el turno entero por una sería perder una
// respuesta que ya estaba casi completa.
func bombear(r io.Reader, emitir func(Evento) error) (cerrado bool, err error) {
	tr := NuevoTraductor()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), topeDeLinea)

	for sc.Scan() {
		linea := bytes.TrimSpace(sc.Bytes())
		if len(linea) == 0 {
			continue
		}
		evs, errLinea := tr.Linea(linea)
		if errLinea != nil {
			// Se registra y se sigue. Es el único lugar del paquete que escribe a stderr:
			// perder una línea en silencio haría imposible diagnosticar un cambio de
			// formato del binario.
			fmt.Fprintf(os.Stderr, "motor: %v\n", errLinea)
		}
		// El helper lo comparten los dos motores (ver emitirEventos en api.go): el cliente
		// se fue o el transporte falló, y no hay a quién seguir mandándole eventos.
		if err := emitirEventos(evs, emitir); err != nil {
			return tr.Cerrado(), err
		}
		if tr.Cerrado() {
			return true, nil
		}
	}
	if err := sc.Err(); err != nil {
		// Incluye ErrTooLong, que con el tope de arriba significaría una línea de 16 MB.
		return tr.Cerrado(), fmt.Errorf("leer el stream: %w", err)
	}
	return tr.Cerrado(), nil
}

// argumentosCLI arma el argv. Cada bandera está acá por una razón medida:
//
//   - `-p` headless, sin TUI.
//   - `--output-format stream-json` + `--verbose`: el stream de eventos. `--verbose` es
//     obligatorio con stream-json, el binario lo rechaza sin él.
//   - `--include-partial-messages`: SIN esto no hay deltas, sólo mensajes completos — y el
//     protocolo del cliente es todo deltas.
//   - `--input-format stream-json`: deja pasar la COLA de la historia por stdin.
//   - `--setting-sources ""`: aísla al hijo de los settings, hooks y skills del proyecto.
//     Deliberado: lo que se le pide es un turno pelado del modelo, y las tools las ejecuta
//     el cliente remoto. Sin esto el hijo cargaría los hooks de Musubi y recursaría sobre
//     sí mismo. OJO CON LO QUE ESTA BANDERA NO HACE, porque el comentario viejo se lo
//     atribuía: NO corta los servidores MCP del usuario. Medido el 2026-10-29 con la
//     bandera puesta, el hijo igual veía los 12 tools de los conectores del host
//     (`mcp__claude_ai_Claude_Docs__*`, `mcp__pencil__*`). Lo que los corta es
//     `--strict-mcp-config`.
//   - `--strict-mcp-config` + `--mcp-config`: VAN SIEMPRE, no sólo cuando el pedido trae
//     tools. Medido el 2026-10-29 contra el 2.1.272 leyendo el evento `system/init`: sin
//     la bandera el hijo ve 42 tools y 4 servidores MCP —los conectores del dueño entre
//     ellos—; con ella, 30 tools y 1 servidor. Un turno que no pidió tools tampoco tiene
//     por qué ver los conectores de quien corre el servidor. El valor del `--mcp-config`
//     es el mapa VACÍO cuando no hay espejo: `--strict` sin `--mcp-config` dejaría al CLI
//     decidiendo qué hacer con una bandera suelta, y el mapa explícito no deja dudas.
//   - `--tools ""`: saca las tools NATIVAS del CLI (Read/Write/Edit/Bash/Glob/Grep), que no
//     son del cliente y le darían al hijo el disco del host. Medido el 2026-10-29: la
//     bandera las saca TODAS y PRESERVA las del espejo —`init.tools` queda con un solo
//     elemento, la tool MCP del turno—, y además se lleva `ToolSearch`, con lo que ninguna
//     tool del cliente le puede llegar DIFERIDA al modelo. El comentario de motor.go que
//     decía que esto «no se puede» era falso: ver el `--help` del propio binario, líneas
//     248-252, «Use "" to disable all tools».
//   - `--resume <archivo>`: el PREFIJO de la historia. Ver partirHistoria.
//   - `--no-session-persistence`: VA SIEMPRE, y antes iba sólo con `--resume`. Sin la
//     bandera el CLI escribe la sesión que arma este motor en el disco del usuario
//     (`~/.claude/projects/<cwd>/<uuid>.jsonl`), tenga o no un `--resume` que reanudar: un
//     turno servido a un cliente remoto no es una conversación de quien corre el servidor.
//   - `--settings {"attribution":…}`: vacía las plantillas de atribución de commits. Sin
//     esto el CLI inyecta un `<system-reminder>` sobre cómo firmar commits de git DENTRO
//     del mensaje del cliente (medido en el cuerpo del request), y un cliente que pidió
//     «resumime este texto» recibe un modelo recién instruido en otra cosa.
//   - `--strict-mcp-config --mcp-config <mapa vacío|ruta> --allowedTools mcp__pi`: las dos
//     primeras van SIEMPRE; `--allowedTools`, sólo con espejo. Ver espejo.go: el CLI no toma
//     las tools por bandera, las toma por MCP. `--strict` es lo que impide que el hijo cargue
//     el `.mcp.json` del proyecto y recurse sobre Musubi; y el `--allowedTools` con el
//     nombre del servidor es lo que evita que una tool del cliente se frene pidiendo un
//     permiso que en headless nadie va a contestar (mismo patrón que el protocolo propio
//     del repo usa con `mcp__musubi`, .claude/fase2/fase2-protocolo.md).
func argumentosCLI(p Pedido, rutaSesion string, esp *espejo) []string {
	// El valor de `--mcp-config` sale de UN solo sitio: el mapa vacío, o la RUTA del archivo
	// del espejo cuando el pedido trae tools. Un único `--mcp-config` en el argv evita
	// depender de cómo el CLI resuelve la bandera repetida —si gana el primero, el último, o
	// se acumulan—, que no está medido y no hace falta averiguar para esto.
	//
	// EL MAPA VACÍO VA INLINE Y EL DEL ESPEJO NO, y la asimetría es el punto: el mapa vacío no
	// tiene nada que esconder, mientras que el del espejo lleva el token del turno dentro de la
	// URL y el argv de un proceso lo lee cualquiera (ver escribirConfig en espejo.go).
	//
	// Un espejo sin `rutaConfig` no puede pasar: abrirEspejo escribe el archivo o devuelve
	// error. Si igual llegara —un espejo armado a mano—, cae en el mapa vacío: el turno se
	// queda sin tools, que es visible, en vez de filtrar la credencial, que no lo es.
	configMCP := `{"mcpServers":{}}`
	if esp != nil && esp.rutaConfig != "" {
		configMCP = esp.rutaConfig
	}
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--input-format", "stream-json",
		"--verbose",
		"--setting-sources", "",
		"--settings", `{"attribution":{"commit":"","pr":""}}`,
		"--strict-mcp-config",
		"--mcp-config", configMCP,
		"--tools", "",
		"--no-session-persistence",
	}
	if rutaSesion != "" {
		args = append(args, "--resume", rutaSesion)
	}
	if p.Modelo != "" {
		args = append(args, "--model", p.Modelo)
	}
	if p.SystemPrompt != "" {
		// Reemplaza el system prompt del CLI (medido), que es lo que corresponde: el
		// cliente remoto trae el suyo y el del binario describiría otra herramienta.
		//
		// NO lo reemplaza del todo, y eso no se puede arreglar desde acá: quedan dos
		// bloques fijos por delante —el de facturación y uno que arranca «You are a Claude
		// agent, built on Anthropic's Claud…»— en TODAS las variantes probadas, `--bare`
		// incluida. Un cliente de este motor nunca controla su system prompt entero; el
		// que lo necesite exacto va por ClienteAPI.
		args = append(args, "--system-prompt", p.SystemPrompt)
	}
	if esp != nil {
		// Lo único que depende del espejo: el permiso. Medido el 2026-10-29, `--allowedTools`
		// hace match por PREFIJO DE SERVIDOR, así que `mcp__pi` autoriza a `mcp__pi__<tool>`
		// sin enumerarlas. Y sin la bandera, en `-p` no interactivo la tool MCP se DENIEGA:
		// el espejo no recibe un solo `tools/call` y el turno termina diciéndole al cliente
		// que le falta un permiso que en headless nadie va a contestar.
		args = append(args, "--allowedTools", "mcp__"+nombreDelEspejo)
	}
	return args
}

// partirHistoria divide el transcript en el PREFIJO (que viaja por un archivo de sesión) y
// la COLA (que viaja por stdin).
//
// 🔴 POR QUÉ EXISTE ESTA FUNCIÓN. `claude -p --input-format stream-json` NO interpreta su
// stdin como un transcript: cada línea `user` es un PROMPT NUEVO que se encola y las líneas
// `assistant` se inyectan por delante. Medido el 2026-10-28 leyendo el cuerpo JSON que el
// CLI 2.1.272 le mandó a un servidor falso:
//
//	enviado [user A, user B]                      -> el request lleva sólo A; B no aparece
//	enviado [user 1, assistant 1, user 2]         -> [assistant 1, user 1]; user 2 ausente
//	enviado [user 1, assistant{tool_use}, user{tool_result}]
//	                                              -> el tool_result llega como
//	                                                 "[Tool result missing due to internal error]"
//
// O sea que en toda continuación de un agent loop —TODO lo que un cliente hace después del
// primer turno— el modelo respondía al PRIMER mensaje y nunca veía el resultado de la tool.
// El CLI le fabricaba un placeholder de error. Pasaba desapercibido porque el test validaba
// el payload contra un proceso falso, que lo acepta porque es un `echo`.
//
// LA PARTICIÓN, medida como la que funciona: el prefijo entra como una sesión que se
// reanuda, y la cola —el turno del asistente más el user que lo responde— por stdin, que es
// lo que dispara el request. Así el `tool_result` llega REAL y en orden.
//
// La cola es el último mensaje más el `assistant` que lo precede, si lo hay: el par
// tool_use/tool_result tiene que viajar junto por el mismo canal para que el CLI lo empareje.
func partirHistoria(ms []Mensaje) (prefijo, cola []Mensaje) {
	if len(ms) <= 1 {
		return nil, ms
	}
	corte := len(ms) - 1
	if ms[corte-1].Rol == "assistant" {
		corte--
	}
	return ms[:corte], ms[corte:]
}

// archivoDeSesion escribe el prefijo como un archivo de sesión del CLI y devuelve su ruta y
// una función de limpieza. Con prefijo vacío no escribe nada y devuelve ruta "".
func (l *Lanzador) archivoDeSesion(prefijo []Mensaje, p Pedido) (string, func(), error) {
	nada := func() {}
	if len(prefijo) == 0 {
		return "", nada, nil
	}

	cwd := p.Dir
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return "", nada, fmt.Errorf("cwd para la sesión: %w", err)
		}
	}
	version := l.VersionDeSesion
	if version == "" {
		version = VersionDeSesionPorDefecto
	}
	cuerpo, err := sesionJSONL(prefijo, p.modeloParaElTranscript(), cwd, version)
	if err != nil {
		return "", nada, err
	}

	dir, err := os.MkdirTemp("", "musubi-motor-")
	if err != nil {
		return "", nada, fmt.Errorf("directorio temporal para la sesión: %w", err)
	}
	limpiar := func() { _ = os.RemoveAll(dir) }

	// `--resume` quiere una ruta ABSOLUTA a un archivo .jsonl (medido).
	ruta := filepath.Join(dir, uuid.NewString()+".jsonl")
	if err := os.WriteFile(ruta, cuerpo, 0o600); err != nil {
		limpiar()
		return "", nada, fmt.Errorf("escribir la sesión: %w", err)
	}
	return ruta, limpiar, nil
}

// lineaDeSesion es una línea del archivo de sesión. El formato es INTERNO del CLI: los
// campos de acá son el subconjunto mínimo que la 2.1.272 acepta, medido probando de a uno.
type lineaDeSesion struct {
	ParentUUID *string         `json:"parentUuid"`
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"message"`
	UUID       string          `json:"uuid"`
	Timestamp  string          `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	Cwd        string          `json:"cwd"`
	Version    string          `json:"version"`

	// IsMeta marca la última línea del prefijo. Sin esto, desde la segunda ronda el CLI
	// agrega un bloque «Continue from where you left off.» al contexto del modelo: un
	// mensaje que el cliente nunca escribió y que cambia lo que se le está pidiendo.
	IsMeta bool `json:"isMeta,omitempty"`
}

// sesionJSONL serializa el prefijo al formato de archivo de sesión del CLI.
func sesionJSONL(ms []Mensaje, modelo, cwd, version string) ([]byte, error) {
	sesion := uuid.NewString()
	marca := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	var buf bytes.Buffer
	var padre *string
	for i, m := range ms {
		if m.Rol != "user" && m.Rol != "assistant" {
			return nil, fmt.Errorf("mensaje %d: rol %q no es user ni assistant", i, m.Rol)
		}
		if len(m.Contenido) == 0 {
			return nil, fmt.Errorf("mensaje %d: sin contenido", i)
		}
		cuerpo, err := mensajeDeSesion(m, modelo)
		if err != nil {
			return nil, fmt.Errorf("mensaje %d: %w", i, err)
		}
		id := uuid.NewString()
		linea := lineaDeSesion{
			ParentUUID: padre,
			Type:       m.Rol,
			Message:    cuerpo,
			UUID:       id,
			Timestamp:  marca,
			SessionID:  sesion,
			Cwd:        cwd,
			Version:    version,
			IsMeta:     i == len(ms)-1,
		}
		b, err := json.Marshal(linea)
		if err != nil {
			return nil, fmt.Errorf("serializar la línea %d de la sesión: %w", i, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
		padre = &id
	}
	return buf.Bytes(), nil
}

// mensajeDeSesion arma el campo `message` de una línea de sesión. Los dos roles no llevan
// la misma forma, y el del asistente tiene el gotcha que más cuesta: ver `model`.
func mensajeDeSesion(m Mensaje, modelo string) (json.RawMessage, error) {
	if m.Rol == "user" {
		return json.Marshal(struct {
			Role    string            `json:"role"`
			Content []json.RawMessage `json:"content"`
		}{Role: m.Rol, Content: m.Contenido})
	}

	razon := "end_turn"
	if m.tieneToolUse() {
		// Un turno que pidió una tool y se declara "end_turn" le dice al modelo que la
		// conversación había terminado cuando en realidad estaba esperando un resultado.
		razon = "tool_use"
	}
	return json.Marshal(struct {
		ID         string            `json:"id"`
		Type       string            `json:"type"`
		Role       string            `json:"role"`
		Model      string            `json:"model"`
		Content    []json.RawMessage `json:"content"`
		StopReason string            `json:"stop_reason"`
		Usage      usoCrudo          `json:"usage"`
	}{
		ID:   "msg_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Type: "message",
		Role: m.Rol,
		// EL MODELO VA RESUELTO, Y SI NO EL CLI DESCARTA EL THINKING FIRMADO EN SILENCIO.
		// A/B de un factor el 2026-10-28: con el id completo ("claude-sonnet-5") los
		// bloques de thinking con firma se conservan; sin el campo, caen; con otro modelo,
		// caen; y con el ALIAS ("sonnet") también caen. No hay error, no hay warning: el
		// turno simplemente llega sin su razonamiento.
		Model:      modelo,
		Content:    m.Contenido,
		StopReason: razon,
	})
}

// entradaStreamJSON serializa la COLA al formato que toma `--input-format stream-json`: un
// objeto por línea. Es lo que dispara el request; el resto de la historia va por el archivo
// de sesión (ver partirHistoria).
func entradaStreamJSON(ms []Mensaje) ([]byte, error) {
	var buf bytes.Buffer
	for i, m := range ms {
		if m.Rol != "user" && m.Rol != "assistant" {
			return nil, fmt.Errorf("mensaje %d: rol %q no es user ni assistant", i, m.Rol)
		}
		if len(m.Contenido) == 0 {
			return nil, fmt.Errorf("mensaje %d: sin contenido", i)
		}
		linea := struct {
			Type    string `json:"type"`
			Message struct {
				Role    string            `json:"role"`
				Content []json.RawMessage `json:"content"`
			} `json:"message"`
		}{Type: m.Rol}
		linea.Message.Role = m.Rol
		linea.Message.Content = m.Contenido

		b, err := json.Marshal(linea)
		if err != nil {
			return nil, fmt.Errorf("serializar mensaje %d: %w", i, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func (l *Lanzador) entornoBase() []string {
	if l.Entorno != nil {
		return l.Entorno
	}
	return os.Environ()
}

// prefijosFuera son las variables que NO pueden llegar al hijo. Van por prefijo y no por
// nombre exacto para que una variable nueva del harness quede cubierta sola:
//
//   - CLAUDECODE / CLAUDE_CODE_*: le dicen al binario que corre DENTRO de una sesión de
//     Claude Code. Heredarlas lo hace hablarle a la sesión padre en vez de servir un turno.
//   - MUSUBI_*: el hijo cargaría el MCP de Musubi y recursaría sobre esta misma máquina.
//
// OJO CON LO QUE ESTA GUARDA NO PUEDE HACER: saca las variables del ENTORNO, y el CLI
// reconstruye su contexto por su cuenta y lo manda igual. Medido el 2026-10-28, el request
// lleva un `{"role":"system"}` con el `# Environment` del host —cwd, plataforma, versión del
// SO—, y eso NO viene de una variable: va por el prompt. La única bandera que lo quita es
// `--bare`, que exige ANTHROPIC_API_KEY y descarta OAuth. Un cliente que no pueda ver el
// host del servidor tiene que ir por ClienteAPI.
var prefijosFuera = []string{"CLAUDECODE", "CLAUDE_CODE_", "MUSUBI_"}

func entornoAislado(base []string, conservarAPIKey bool) []string {
	out := make([]string, 0, len(base))
	for _, kv := range base {
		nombre := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			nombre = kv[:i]
		}
		if nombre == "ANTHROPIC_API_KEY" && !conservarAPIKey {
			continue
		}
		fuera := false
		for _, p := range prefijosFuera {
			if strings.HasPrefix(nombre, p) {
				fuera = true
				break
			}
		}
		if !fuera {
			out = append(out, kv)
		}
	}
	return out
}

// colaDeTexto guarda sólo el final de lo que se le escribe. El stderr de un proceso que
// falló puede ser enorme (un stack de node entero) y lo que explica la falla está al final.
// Es un io.Writer con tope, para no acumular megabytes de un turno que ya fracasó.
type colaDeTexto struct {
	mu  sync.Mutex
	buf []byte
}

const topeStderr = 4 << 10

func (c *colaDeTexto) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf = append(c.buf, p...)
	if len(c.buf) > topeStderr {
		c.buf = c.buf[len(c.buf)-topeStderr:]
	}
	return len(p), nil
}

// cola devuelve el stderr listo para pegar en un mensaje de error, o "" si no hubo nada.
func (c *colaDeTexto) cola() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := strings.TrimSpace(string(c.buf))
	if s == "" {
		return ""
	}
	return "; stderr: " + s
}
