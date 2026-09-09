package mcp

// shell_agente_http.go es LA PUERTA DEL AGENTE para las sesiones de shell (S5c).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// AUTENTICA DISPOSITIVOS, NO PERSONAS — al revés que shell_relay.go, que está al lado.
//
// Y acá esa distinción tiene una consecuencia que hay que decir entera: por este canal viaja
// TODO LO QUE LA PERSONA TECLEA, contraseñas de sudo incluidas. Si una máquina comprometida
// pudiera engancharse a la sesión de OTRA máquina, leería esas teclas.
//
// Por eso la guarda central de este archivo no es «¿el token es válido?» sino
// **«¿esta sesión es de ESTA máquina?»**. El id de sesión no alcanza, igual que del lado de las
// personas: el token dice QUIÉN sos, el id dice CUÁL sesión, y las dos cosas tienen que coincidir.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"io"
	"net/http"
	"time"

	"musubi/internal/fleet"
)

// Las dos rutas del agente. Se nombran desde EL PUNTO DE VISTA DEL AGENTE, porque «entrada» y
// «salida» significan lo contrario según de qué lado se pare uno y ésa es la confusión más fácil
// de cometer en todo el track.
const (
	shellAgenteEntradaPath = "/fleet/shell/agent/in"  // el agente RECOGE lo que la persona tecleó
	shellAgenteSalidaPath  = "/fleet/shell/agent/out" // el agente ENTREGA lo que imprimió el pty
)

// canalDelAgente resuelve el canal comprobando las dos cosas: el token y la pertenencia.
//
// Devuelve el canal sólo si la sesión existe, está viva, y es DE ESA MÁQUINA. Un fallo de
// pertenencia y uno de existencia dan la misma respuesta: distinguirlos convertiría la ruta en un
// oráculo de qué sesiones hay abiertas en la flota.
func (s *McpServer) canalDelAgente(d fleet.Device, id string, ahora time.Time) (*fleet.CanalAgente, bool) {
	ses, existe, err := s.engine.SesionShellPorID(id)
	if err != nil || !existe {
		return nil, false
	}
	// LA GUARDA CENTRAL. Sin esta línea, cualquier máquina de la flota con un token válido puede
	// engancharse a la sesión de cualquier otra y leer lo que se teclea ahí.
	if ses.DeviceID != d.ID {
		return nil, false
	}
	if !ses.Viva(ahora) {
		return nil, false
	}
	canal, hay := s.shells.buscar(id)
	if !hay {
		return nil, false
	}
	agente, ok := canal.(*fleet.CanalAgente)
	if !ok {
		// La sesión existe pero su canal no es de agente (es un Tier B por SSH). Que un agente
		// intente engancharse ahí no debería pasar nunca, y si pasa no se lo deja.
		return nil, false
	}
	return agente, true
}

// handlerShellAgenteEntrada entrega al agente lo que la persona tecleó (long-poll).
func (s *McpServer) handlerShellAgenteEntrada(limiter *authLimiter, auditoria *registroDeAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		d, ok := s.deviceDeRequest(limiter, auditoria, w, r)
		if !ok {
			return
		}
		canal, hay := s.canalDelAgente(d, r.URL.Query().Get("id"), time.Now())
		if !hay {
			http.Error(w, "esa sesión no existe, no es de esta máquina, o ya terminó", http.StatusGone)
			return
		}
		// El agente pidiendo teclas ES la prueba de que llegó. Se marca acá y no al recibir el
		// primer byte: una shell recién abierta no imprime nada hasta que el pty arranca, y sin
		// esto «el agente todavía no vino» sería indistinguible de «vino y no imprime».
		canal.Enganchar()

		datos, err := canal.LeerDeLaPersona(esperaSalidaShell)
		w.Header().Set("Content-Type", "application/octet-stream")
		// nosniff acompaña al octet-stream: sin él un navegador puede OLFATEAR el cuerpo y
		// renderizarlo como HTML pese al tipo declarado. Acá el cuerpo son bytes crudos de un
		// pty, o sea contenido que elige quien esté del otro lado de la shell.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if err != nil {
			// El canal se cerró: se le dice al agente que mate el pty y se vaya.
			w.Header().Set("X-Musubi-Shell", "cerrada")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(datos)
	}
}

// handlerShellAgenteSalida recibe del agente lo que imprimió el pty.
func (s *McpServer) handlerShellAgenteSalida(limiter *authLimiter, auditoria *registroDeAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		d, ok := s.deviceDeRequest(limiter, auditoria, w, r)
		if !ok {
			return
		}
		id := r.URL.Query().Get("id")
		canal, hay := s.canalDelAgente(d, id, time.Now())
		if !hay {
			http.Error(w, "esa sesión no existe, no es de esta máquina, o ya terminó", http.StatusGone)
			return
		}
		canal.Enganchar()

		// EL AGENTE ES UNA FUENTE NO CONFIABLE, como toda máquina de la flota. El tope no lo pone
		// su Content-Length —que es un dato que él elige— sino un LimitReader.
		datos, err := io.ReadAll(io.LimitReader(r.Body, salidaMaxPorTramo))
		if err != nil {
			http.Error(w, "no se pudo leer la salida", http.StatusBadRequest)
			return
		}
		// EscribirALaPersona BLOQUEA si el buffer está lleno: es la contrapresión llegando hasta
		// la máquina remota. Un `cat` grande en un Tier A frena al agente, que frena al pty —
		// exactamente lo que hace una terminal real sobre un enlace lento.
		if err := canal.EscribirALaPersona(datos); err != nil {
			http.Error(w, "la sesión se cerró", http.StatusGone)
			return
		}
		if len(datos) > 0 {
			if err := s.engine.TocarSesionShell(id, time.Now()); err != nil {
				_ = err // el reloj de inactividad es best-effort; el canal ya entregó
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// salidaMaxPorTramo acota cuánto entrega el agente de una vez. Holgado para un redibujado de
// pantalla y con techo para que una máquina comprometida no empuje un bloque enorme por request.
const salidaMaxPorTramo = 256 * 1024

// deviceDeRequest autentica contra la tabla `devices`, con el mismo limiter y el mismo motivo
// único de rechazo que el latido: un 401 no puede ser un oráculo de qué tokens existen.
func (s *McpServer) deviceDeRequest(limiter *authLimiter, auditoria *registroDeAuth, w http.ResponseWriter, r *http.Request) (fleet.Device, bool) {
	ip := clientIP(r)
	// EL CANDADO VA DESPUÉS DE MIRAR EL TOKEN, igual que en el endpoint MCP (A88). Acá el orden
	// original tenía una razón mejor —DevicePorToken consulta la base, y el candado ahorraba esa
	// consulta durante una inundación— pero el precio era el mismo y peor ubicado: el candado es
	// POR IP, así que un proceso roto en una máquina de la flota dejaba afuera al AGENTE de esa
	// misma máquina, con su token de device perfectamente válido, y la sesión de shell se caía
	// sola sin que nada dijera por qué. Un SELECT por índice cuesta microsegundos; que un agente
	// legítimo no pueda entrar cuesta una sesión.
	bearer := bearerToken(r.Header.Get("Authorization"))
	d, ok, err := s.engine.DevicePorToken(bearer)
	if err != nil {
		http.Error(w, "device registry unavailable", http.StatusServiceUnavailable)
		return fleet.Device{}, false
	}
	if !ok {
		ahora := time.Now()
		motivo := motivoCredencialDesconocida
		if bearer == "" {
			motivo = motivoSinCredencial
			s.metrics.authSinCredencial.Add(1)
		} else {
			s.metrics.authDesconocida.Add(1)
		}
		limiter.fail(ip, ahora)
		auditoria.fallo(ahora, ip, motivo, r.URL.Path, r.Header.Get("User-Agent"))
		if limiter.locked(ip, ahora) {
			s.metrics.authBloqueado.Add(1)
			http.Error(w, "too many failed auth attempts", http.StatusTooManyRequests)
			return fleet.Device{}, false
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, motivoRechazo, http.StatusUnauthorized)
		return fleet.Device{}, false
	}
	limiter.reset(ip)
	// Y el eje del APARATO: una máquina a la que no se le concedió `shell` no engancha ninguna
	// sesión, ni aunque el cerebro se la haya ofrecido por error.
	if !d.Permite(fleet.CapShell) {
		http.Error(w, "esta máquina no tiene concedida la capacidad `shell`", http.StatusForbidden)
		return fleet.Device{}, false
	}
	return d, true
}
