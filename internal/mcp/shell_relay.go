package mcp

// shell_relay.go es el RELAY de las sesiones de shell interactiva. Track «Control de flota», S5b.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ESTA PUERTA AUTENTICA PERSONAS, NO DISPOSITIVOS.
//
// Va en un archivo aparte de fleet_http.go por eso: aquél resuelve contra la tabla `devices` —un
// agente presentando su token de máquina—, y esto resuelve contra el registro de principals. Son
// dos almacenes de credenciales que en este track NUNCA se cruzan, y mezclarlos en el mismo
// archivo es cómo se termina llamando al resolver equivocado en una revisión apurada.
//
// DOS STREAMS HALF-DUPLEX Y NO UN WEBSOCKET (T8). La biblioteca estándar no trae WebSocket, y
// traerlo sería la 7ª dependencia directa de un repo que tiene 6 a propósito. Dos requests —uno
// que baja la salida con long-poll, otro que sube lo tecleado— hacen el trabajo, atraviesan
// cualquier proxy y usan el mismo bearer que el resto. La latencia la pone la red, no el diseño:
// el GET vuelve EN CUANTO hay un byte, no al vencer su espera.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// Las tres rutas del relay.
const (
	shellOutPath   = "/fleet/shell/out"
	shellInPath    = "/fleet/shell/in"
	shellClosePath = "/fleet/shell/close"
)

// esperaSalidaShell es cuánto bloquea el GET esperando salida antes de volver vacío.
//
// 25 s: por debajo del deadline del transporte (60 s) con margen de sobra, y bastante largo como
// para que una terminal quieta no genere tráfico. No es latencia: el GET vuelve apenas hay un
// byte. Es cada cuánto se renueva la conexión cuando NO pasa nada.
const esperaSalidaShell = 25 * time.Second

// entradaMaxShell acota cuánto se acepta por request de entrada. Una persona tecleando manda
// decenas de bytes; un pegado grande, unos miles. 64 KiB es holgado y le pone techo a lo que
// alguien puede empujar de una vez.
const entradaMaxShell = 64 * 1024

// registroDeShells son las sesiones VIVAS de este proceso.
//
// EN MEMORIA A PROPÓSITO, y no es lo mismo que el cooldown de las políticas (que sí se persiste):
// una sesión viva ES un proceso ssh hijo de ESTE cerebro. Si el cerebro muere, el ssh muere con
// él; persistir su id sólo serviría para que alguien intente escribirle a un canal que ya no
// existe. La BITÁCORA sí es durable — pero la bitácora es el registro, no el canal.
type registroDeShells struct {
	mu  sync.Mutex
	por map[string]fleet.CanalInteractivo
}

func (r *registroDeShells) guardar(id string, c fleet.CanalInteractivo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.por == nil {
		r.por = make(map[string]fleet.CanalInteractivo)
	}
	r.por[id] = c
}

func (r *registroDeShells) buscar(id string) (fleet.CanalInteractivo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.por[id]
	return c, ok
}

func (r *registroDeShells) quitar(id string) (fleet.CanalInteractivo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.por[id]
	delete(r.por, id)
	return c, ok
}

// vivas devuelve los ids de las sesiones en curso, para que el barrido pueda matar las vencidas.
func (r *registroDeShells) vivas() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.por))
	for id := range r.por {
		out = append(out, id)
	}
	return out
}

// autorizarShell es LA función de este archivo, y corre en CADA request del stream (T6).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL ID DE SESIÓN NO ES UNA CREDENCIAL.
//
// Es el error clásico de las APIs de sesión: se autoriza al abrir, se devuelve un id, y a partir
// de ahí el id ES el permiso. Un id así es un token portador con permisos de shell, sin
// vencimiento propio y sin forma de revocarlo — y encima viaja en una URL, que es donde los
// identificadores terminan en logs de proxy y en el historial de la terminal.
//
// Acá el id sólo dice CUÁL sesión. Quién sos lo dice el bearer, y las cuatro preguntas se
// vuelven a hacer enteras cada vez:
//
//  1. ¿la sesión existe?
//  2. ¿es TUYA? (una sesión ajena no se toca ni sabiendo su id)
//  3. ¿sigue viva? (los techos de vida e inactividad se aplican acá, a mitad de un `tail -f`)
//  4. ¿tu concesión SIGUE vigente? — revocar a alguien en principals.yaml tiene que cortarle la
//     sesión EN CURSO, no sólo impedirle abrir la próxima.
//
// La 4 es la que se olvida siempre, y es la que hace que revocar signifique algo.
// ────────────────────────────────────────────────────────────────────────────────────────────
func (s *McpServer) autorizarShell(p *Principal, id string, ahora time.Time) (fleet.SesionShell, error) {
	ses, existe, err := s.engine.SesionShellPorID(id)
	if err != nil {
		return ses, fmt.Errorf("no se pudo leer la sesión")
	}
	// Una sesión inexistente y una ajena dan LA MISMA respuesta: distinguirlas convertiría la
	// ruta en un oráculo de qué ids existen.
	if !existe || ses.Principal != nombrePrincipal(p) {
		return ses, errShellNoTuya
	}
	if !ses.Viva(ahora) {
		_, motivo := ses.Vencida(ahora)
		if motivo == "" {
			motivo = "la sesión ya está cerrada"
		}
		return ses, &errShellMuerta{motivo}
	}
	// 4 — LA CONCESIÓN SE RE-EVALÚA. El principal llega ya resuelto contra el snapshot VIGENTE
	// del registro (que se recarga en caliente cada 10 s), así que revocar a alguien le corta el
	// prompt que tiene abierto.
	d, hay, err := s.engine.DevicePorID(ses.DeviceID)
	if err != nil || !hay {
		return ses, errShellNoTuya
	}
	if !PuedeSobreDevice(p, d, fleet.CapShell) {
		// Perder la concesión a mitad de sesión es un 403, no un 401: la credencial es válida,
		// lo que cambió es lo que puede. Decir «no autenticado» mandaría a rotar un token sano.
		return ses, &errShellSinPermiso{}
	}
	return ses, nil
}

var errShellNoTuya = errors.New("esa sesión no existe o no es tuya")

// errShellMuerta distingue «tu sesión se terminó» de «esa sesión no es tuya», y no es un detalle
// de cortesía: son dos códigos HTTP distintos y dos acciones distintas del otro lado.
//
// Un 401 sobre una sesión PROPIA que venció manda a alguien a revisar su token, que está
// perfecto. Y no abre ningún oráculo: para llegar a este error hay que haber pasado ya el chequeo
// de propiedad, o sea que quien lo recibe es la dueña. Lo que no se distingue —a propósito— es
// «no existe» de «no es tuya», que es donde SÍ habría un oráculo de qué ids existen.
type errShellMuerta struct{ motivo string }

func (e *errShellMuerta) Error() string { return e.motivo }

// errShellSinPermiso es la concesión revocada a mitad de sesión.
type errShellSinPermiso struct{}

func (e *errShellSinPermiso) Error() string {
	return "tu credencial ya no tiene la capacidad `shell` sobre esa máquina: la sesión se corta acá"
}

// estadoHTTPDeShell traduce el error de autorización al código que corresponde.
//
// Los tres son distintos y quien está del otro lado hace cosas distintas con cada uno: 401 =
// revisá tu credencial; 403 = tu credencial está bien y ya no te alcanza; 410 = la sesión
// terminó, abrí otra. Devolver 401 para los tres —el atajo— manda a rotar tokens sanos.
func estadoHTTPDeShell(err error) int {
	var muerta *errShellMuerta
	var sinPermiso *errShellSinPermiso
	switch {
	case errors.As(err, &muerta):
		return http.StatusGone
	case errors.As(err, &sinPermiso):
		return http.StatusForbidden
	default:
		return http.StatusUnauthorized
	}
}

// handlerShellOut baja lo que la máquina imprimió (long-poll).
func (s *McpServer) handlerShellOut(opt httpOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p, ok := s.principalDeRequest(opt, w, r)
		if !ok {
			return
		}
		id := r.URL.Query().Get("id")
		ses, err := s.autorizarShell(p, id, time.Now())
		if err != nil {
			http.Error(w, err.Error(), estadoHTTPDeShell(err))
			return
		}
		canal, hay := s.shells.buscar(ses.ID)
		if !hay {
			// La sesión figura viva en la bitácora pero su canal no está en ESTE proceso: el
			// cerebro se reinició. Se dice claro en vez de devolver un vacío eterno, que se leería
			// como una terminal quieta.
			s.cerrarShell(ses.ID, fleet.ShellFallida, "el cerebro se reinició y el canal se perdió", time.Now())
			http.Error(w, "el canal de esa sesión ya no existe (¿se reinició el cerebro?): abrí una nueva", http.StatusGone)
			return
		}

		datos, lerr := canal.Leer(esperaSalidaShell)
		ahora := time.Now()
		if len(datos) > 0 {
			// El reloj de inactividad se mueve con la SALIDA también: una sesión donde `tail -f`
			// escupe líneas está viva aunque nadie teclee.
			if err := s.engine.TocarSesionShell(ses.ID, ahora); err != nil {
				logx.Warn("shell: no se pudo tocar la sesión", "sesion", ses.ID, "error", err)
			}
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		if errors.Is(lerr, fleet.ErrCanalCerrado) {
			// El final NORMAL: alguien tecleó `exit`. Se avisa por cabecera y NO por código de
			// error, porque los bytes que vienen en este mismo cuerpo son las últimas líneas
			// antes de morir — justamente las que se quieren ver.
			w.Header().Set("X-Musubi-Shell", "cerrada")
			s.cerrarShell(ses.ID, fleet.ShellCerrada, "la shell remota terminó", ahora)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(datos)
	}
}

// handlerShellIn sube lo tecleado.
func (s *McpServer) handlerShellIn(opt httpOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p, ok := s.principalDeRequest(opt, w, r)
		if !ok {
			return
		}
		ses, err := s.autorizarShell(p, r.URL.Query().Get("id"), time.Now())
		if err != nil {
			http.Error(w, err.Error(), estadoHTTPDeShell(err))
			return
		}
		canal, hay := s.shells.buscar(ses.ID)
		if !hay {
			http.Error(w, "el canal de esa sesión ya no existe: abrí una nueva", http.StatusGone)
			return
		}
		// LimitReader y no ContentLength: la cabecera la pone el cliente y se puede mentir.
		datos, err := io.ReadAll(io.LimitReader(r.Body, entradaMaxShell))
		if err != nil {
			http.Error(w, "no se pudo leer la entrada", http.StatusBadRequest)
			return
		}
		if err := canal.Escribir(datos); err != nil {
			s.cerrarShell(ses.ID, fleet.ShellCerrada, "el canal se cortó al escribir", time.Now())
			http.Error(w, "la sesión se cortó", http.StatusGone)
			return
		}
		if len(datos) > 0 {
			if err := s.engine.TocarSesionShell(ses.ID, time.Now()); err != nil {
				logx.Warn("shell: no se pudo tocar la sesión", "sesion", ses.ID, "error", err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handlerShellClose la termina a pedido de quien la abrió.
func (s *McpServer) handlerShellClose(opt httpOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p, ok := s.principalDeRequest(opt, w, r)
		if !ok {
			return
		}
		// Cerrar exige la misma autorización que escribir: si no, cualquiera con un id ajeno
		// podría cortarle la sesión a otro. Es un ataque chico y gratuito de cerrar.
		ses, err := s.autorizarShell(p, r.URL.Query().Get("id"), time.Now())
		if err != nil {
			http.Error(w, err.Error(), estadoHTTPDeShell(err))
			return
		}
		s.cerrarShell(ses.ID, fleet.ShellCerrada, "cerrada por quien la abrió", time.Now())
		w.WriteHeader(http.StatusNoContent)
	}
}

// cerrarShell mata el canal y cierra la fila. Idempotente en las dos mitades.
func (s *McpServer) cerrarShell(id string, estado fleet.EstadoShell, motivo string, ahora time.Time) {
	if canal, hay := s.shells.quitar(id); hay {
		_ = canal.Cerrar()
	}
	if err := s.engine.CerrarSesionShell(id, estado, motivo, ahora); err != nil {
		logx.Warn("shell: no se pudo cerrar la fila de la sesión", "sesion", id, "error", err)
	}
}

// cerrarShellsVencidas mata las que algún techo alcanzó. Cuelga del barrido de flota (S10).
//
// LOS TECHOS LOS APLICA EL CEREBRO Y NO LA MÁQUINA REMOTA (T5). Si dependieran del otro lado, una
// máquina comprometida se los saltearía — y el otro lado es justamente aquél del que uno se está
// protegiendo cuando pone un techo a una sesión de shell.
func (s *McpServer) cerrarShellsVencidas(ahora time.Time) int {
	n := 0
	for _, id := range s.shells.vivas() {
		ses, existe, err := s.engine.SesionShellPorID(id)
		if err != nil {
			continue
		}
		if !existe {
			// La fila desapareció y el canal sigue vivo: se mata igual. Un canal sin fila es un
			// proceso ssh que ya nadie puede auditar.
			s.cerrarShell(id, fleet.ShellFallida, "la sesión perdió su registro", ahora)
			n++
			continue
		}
		if vencida, motivo := ses.Vencida(ahora); vencida {
			s.cerrarShell(id, fleet.ShellVencida, motivo, ahora)
			n++
		}
	}
	// Y las filas que quedaron abiertas sin canal (reinicio del cerebro): se cierran para que la
	// bitácora no acumule sesiones «activas» de hace tres días.
	if _, err := s.engine.CerrarSesionesShellVencidas(ahora); err != nil {
		logx.Warn("shell: no se pudieron cerrar las sesiones vencidas", "error", err)
	}
	return n
}

// principalDeRequest resuelve el bearer con la MISMA regla que /mcp y /metrics.
func (s *McpServer) principalDeRequest(opt httpOptions, w http.ResponseWriter, r *http.Request) (*Principal, bool) {
	if opt.registry != nil {
		p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return nil, false
		}
		return p, true
	}
	if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return nil, true // sin registro ni token: confianza local, igual que el resto
}

// EL TAMAÑO DE LA TERMINAL NO SE LEE ACÁ, y conviene decir por qué en vez de dejar el hueco.
// Vivía en este archivo un `tamanoDeTerminal` que sacaba filas/columnas de la query string y que
// NADIE llamaba: lo delató `unused` la primera vez que el linter corrió sobre esta rama. No era un
// cabo suelto sino un duplicado — quien fija el tamaño es el AGENTE, al abrir el PTY
// (cmd/musubi/shell_agente.go, con default 24x80 y los valores que vienen en el argv del comando).
// El relay sólo mueve bytes. Se borró en vez de cablearse porque cablearlo habría creado una
// SEGUNDA fuente para el mismo dato, y dos fuentes de un tamaño de terminal se contradicen el día
// que alguien redimensiona.

var _ = json.Marshal // el paquete se usa en las respuestas de la tool, en methods_shell.go
