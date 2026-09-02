package mcp

// Transporte HTTP del servidor MCP (Track 4): expone el mismo dispatch que el stdio
// sobre un endpoint HTTP, para usar Musubi como servicio. Es OPT-IN
// (config.Service.Enabled). Seguridad por capas:
//   - Bind loopback (default): sin auth obligatoria; defensa anti DNS-rebinding por
//     validación de Host loopback + Origin local.
//   - Bind no-loopback (remoto): EXIGE un bearer token (service.auth_token_env); sin él
//     `serve` se niega a arrancar. El token es el gate de autenticación.
//   - TLS opcional (service.tls_cert_file + tls_key_file).
//
// Modelo de concurrencia: las peticiones se SERIALIZAN sobre un mutex (línea base
// segura, sin riesgo de read-modify-write en el motor). La concurrencia real es un
// slice posterior, tras la auditoría RMW. El seam Dispatch (puro, sin estado mutable
// compartido) ya deja ese cambio listo.

import (
	"compress/gzip"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/logx"
)

const (
	mcpHTTPPath    = "/mcp"
	maxRequestBody = 4 << 20 // 4 MiB: techo del body JSON-RPC entrante (EN EL CABLE).

	// maxDecodedBody es el techo del body YA DESCOMPRIMIDO cuando llega con Content-Encoding:
	// gzip. Existe porque descomprimir sin tope es una bomba: 4 MiB de gzip pueden expandirse a
	// gigabytes y voltear el central always-on. El de arriba sigue rigiendo en el cable, así que
	// aceptar gzip NO afloja el control anti-DoS de la red — sólo agrega un segundo tope, aguas
	// abajo, para el trabajo de descompresión.
	//
	// Que sea seguro subirlo tanto depende de un detalle del handler que conviene no romper: la
	// AUTENTICACIÓN corre ANTES de leer el body. Un cuerpo comprimido sólo lo manda un principal
	// que ya presentó un token válido, el mismo al que la tenencia ya le confía escrituras. Si
	// alguna vez se mueve el chequeo de auth después de la lectura, este número hay que revisarlo.
	maxDecodedBody = 64 << 20 // 64 MiB
)

// readRequestBody lee el cuerpo de un POST a /mcp respetando los dos topes, y acepta
// Content-Encoding: gzip.
//
// El gzip existe por una razón concreta y medida: la federación del grafo de código
// (musubi_codegraph_push) manda el grafo ENTERO en un solo POST, y a partir de cierto tamaño de
// proyecto ese cuerpo pasa los 4 MiB y el central lo rechaza. El síntoma era pésimo: el push es
// best-effort, así que el index local quedaba bien y devolvía `federated:false` sin más, mientras el
// central se quedaba congelado con un grafo viejo. Nadie se enteraba.
//
// Los errores se devuelven DISTINGUIDOS a propósito. Antes los tres casos —pasarse de tamaño, gzip
// corrupto, y una lectura cortada— colapsaban en un mismo "error leyendo el body", y desde el lado
// del cliente eso es indistinguible de un bug de serialización. Un error que dice cuál de los tres
// fue, y contra qué tope, es la diferencia entre leerlo y tener que reproducirlo.
func readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// El tope del CABLE se aplica siempre y primero, comprimido o no.
	limitado := http.MaxBytesReader(w, r.Body, maxRequestBody)

	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
		body, err := io.ReadAll(limitado)
		if err != nil {
			var tope *http.MaxBytesError
			if errors.As(err, &tope) {
				return nil, fmt.Errorf("el body supera el tope de %d MiB; si es un push del grafo, mandalo con Content-Encoding: gzip", maxRequestBody>>20)
			}
			return nil, fmt.Errorf("error leyendo el body: %v", err)
		}
		return body, nil
	}

	zr, err := gzip.NewReader(limitado)
	if err != nil {
		return nil, fmt.Errorf("Content-Encoding: gzip pero el body no es gzip válido: %v", err)
	}
	defer zr.Close()

	// +1 byte para poder DISTINGUIR "entró justo" de "se truncó en el tope". io.LimitReader corta
	// sin error, así que sin este byte de más un cuerpo gigante llegaría como JSON cortado y el
	// error saldría por el Unmarshal de más abajo, diciendo cualquier otra cosa.
	body, err := io.ReadAll(io.LimitReader(zr, maxDecodedBody+1))
	if err != nil {
		var tope *http.MaxBytesError
		if errors.As(err, &tope) {
			return nil, fmt.Errorf("el body comprimido supera el tope de %d MiB en el cable", maxRequestBody>>20)
		}
		return nil, fmt.Errorf("error descomprimiendo el body: %v", err)
	}
	if len(body) > maxDecodedBody {
		return nil, fmt.Errorf("el body descomprimido supera el tope de %d MiB", maxDecodedBody>>20)
	}
	return body, nil
}

// httpOptions configura el handler HTTP.
type httpOptions struct {
	reqTimeout time.Duration
	// token, si no es vacío, exige Authorization: Bearer <token> en cada request.
	token string
	// loopbackOnly activa la defensa anti DNS-rebinding (Host loopback + Origin local).
	// Se usa en modo loopback; en modo remoto el bearer token es el gate y estos checks
	// romperían a clientes legítimos (que usan un Host no-loopback).
	loopbackOnly bool
	// registry, si no es nil, activa la IDENTIDAD por-principal (16.1c): cada request se
	// autentica contra el snapshot VIGENTE del registro (o el token legacy) y el principal
	// resuelto viaja en el ctx para la autorización por rol. Nil ⇒ modo legacy (el único
	// `token` de arriba). Es un principalResolver: recargable en caliente (Track 18) cuando
	// hay archivo, o el registro estático en modo legacy.
	registry principalResolver
	// bodyDir, si no es vacío, habilita GET /body/<archivo>: sirve el manifiesto + binarios
	// del auto-update del cuerpo (musubi-body) desde ese directorio. SIN auth (como /readyz):
	// la frontera es el tailnet. Es la contraparte de `musubi fetch` (canal de update por la
	// malla). Vacío ⇒ la ruta no se registra.
	bodyDir string
}

// HTTPHandler devuelve el http.Handler que sirve MCP sobre HTTP. POST /mcp recibe un
// request JSON-RPC y responde el resultado; GET /mcp (upgrade SSE) queda reservado
// (405) porque Musubi no emite mensajes server-initiated todavía.
func (s *McpServer) HTTPHandler(opt httpOptions) http.Handler {
	// Métricas compartidas del server (Track 16 F3.1). Fallback defensivo si se construyó el
	// McpServer sin NewMcpServer (p.ej. un literal en un test viejo).
	if s.metrics == nil {
		s.metrics = &serverMetrics{}
	}
	metrics := s.metrics
	// Lockout contra fuerza bruta del bearer (16.1e): 5 fallos por IP ⇒ 60s de bloqueo.
	limiter := newAuthLimiter(5, time.Minute)
	mux := http.NewServeMux()

	// Endpoint MCP, envuelto en observabilidad (correlation ID + métricas por resultado).
	mux.Handle(mcpHTTPPath, withObservability(metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Defensa anti DNS-rebinding SOLO en modo loopback (guía de seguridad del
		// transporte HTTP de MCP). En remoto, el bearer token es el gate.
		if opt.loopbackOnly {
			if !isLoopbackHost(r.Host) {
				http.Error(w, "forbidden: non-loopback host", http.StatusForbidden)
				return
			}
			if o := r.Header.Get("Origin"); o != "" && !isLocalOrigin(o) {
				http.Error(w, "forbidden: cross-origin", http.StatusForbidden)
				return
			}
		}
		// Autenticación. Con registro de principals (16.1c): el bearer debe resolver a un
		// principal (o al token legacy) — si no, 401. Sin registro (modo legacy): un único
		// token, comparado en tiempo constante. El principal resuelto viaja en el ctx.
		// Lockout anti fuerza-bruta (16.1e): si la IP acumuló demasiados 401, se rechaza con
		// 429 antes de tocar el token; un auth OK resetea su contador.
		authActive := opt.registry != nil || opt.token != ""
		ip := clientIP(r)
		if authActive && limiter.locked(ip, time.Now()) {
			http.Error(w, "too many failed auth attempts", http.StatusTooManyRequests)
			return
		}
		var principal *Principal
		if opt.registry != nil {
			p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
			if !ok {
				limiter.fail(ip, time.Now())
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			principal = p
		} else if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			limiter.fail(ip, time.Now())
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if authActive {
			limiter.reset(ip)
		}
		if r.Method == http.MethodGet {
			// SSE reservado: no hay tráfico server-initiated en esta versión.
			http.Error(w, "SSE stream not supported", http.StatusMethodNotAllowed)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := readRequestBody(w, r)
		if err != nil {
			writeHTTPJSON(w, errResponse(nil, rpcErrorf(codeParseError, "%v", err)))
			return
		}
		var req JsonRpcRequest
		if jerr := json.Unmarshal(body, &req); jerr != nil {
			writeHTTPJSON(w, errResponse(nil, rpcErrorf(codeParseError, "Parse error")))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), opt.reqTimeout)
		defer cancel()
		ctx = withPrincipal(ctx, principal) // nil en modo legacy ⇒ acceso pleno

		// Dispatch es seguro para llamarse concurrentemente: serializa internamente las
		// tools que mutan (Lock) y deja correr en paralelo las de solo-lectura (RLock).
		resp, ok := s.Dispatch(ctx, req)

		if !ok {
			// Notificación (sin id): por JSON-RPC no hay respuesta. 202 sin cuerpo.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeHTTPJSON(w, resp)
	})))

	// Liveness y readiness: sin auth (los sondea un orquestador/proxy; no exponen secretos).
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", s.readyzHandler)

	// LA PUERTA DEL DISPOSITIVO (track «Control de flota»). Autentica contra la tabla `devices`,
	// NO contra el registro de principals: una credencial de máquina no abre /mcp y una de
	// persona no abre esto. La separación y su porqué están en fleet_http.go — en dos líneas:
	// un agente corre en la superficie más expuesta de la flota, y su credencial no puede ser
	// la llave de la memoria de la empresa. Comparte el `limiter` con /mcp a propósito.
	mux.HandleFunc(fleetHeartbeatPath, s.handlerLatido(limiter))
	// Y la contraparte: por acá el agente reporta cómo salió un comando (S5). Mismo almacén de
	// credenciales, mismo limiter.
	mux.HandleFunc(fleetResultPath, s.handlerResultado(limiter))
	// La puerta del RENDIMIENTO (fase 4): salud para servicios DECLARADOS que ninguna máquina
	// enumera —un bot, un puente—. Mismo token que el latido; ni poda ni estampa señal de vida.
	mux.HandleFunc(fleetSaludPath, s.handlerSaludDeServicios(limiter))

	// EL RELAY DE SHELL INTERACTIVA (S5b). OJO: estas tres rutas autentican PERSONAS (registro de
	// principals), al revés que las dos de arriba, que autentican DISPOSITIVOS (tabla `devices`).
	// Están pegadas en el mux y son puertas distintas; el detalle, en shell_relay.go.
	mux.HandleFunc(shellOutPath, s.handlerShellOut(opt))
	mux.HandleFunc(shellInPath, s.handlerShellIn(opt))
	mux.HandleFunc(shellClosePath, s.handlerShellClose(opt))
	// Y las dos del AGENTE (S5c), que vuelven a autenticar DISPOSITIVOS. Por ellas viaja todo lo
	// que la persona teclea, así que su guarda central no es «¿el token vale?» sino «¿esta sesión
	// es de ESTA máquina?». Ver shell_agente_http.go.
	mux.HandleFunc(shellAgenteEntradaPath, s.handlerShellAgenteEntrada(limiter))
	mux.HandleFunc(shellAgenteSalidaPath, s.handlerShellAgenteSalida(limiter))

	// Auto-update del cuerpo por la malla: sirve manifest + binarios desde bodyDir, sin
	// auth (la frontera es el tailnet, como /readyz). Solo si está configurado.
	if opt.bodyDir != "" {
		mux.HandleFunc("/body/", bodyUpdateHandler(opt.bodyDir))
	}

	// Métricas: datos operativos (uso por tool, profundidad del outbox, gauges de la DB). Detrás de
	// auth SIEMPRE que la auth esté activa. SEGURIDAD (auditoría 2026-07-26 #9): antes gateaba sólo por
	// opt.token, así que en el setup multi-tenant recomendado (principals.yaml, sin token legacy) el
	// token quedaba "" y /metrics caía ABIERTO en el bind del tailnet. Ahora usa la MISMA regla que /mcp:
	// con registry, exige un principal válido; con token legacy, exige el bearer.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		deny := func() {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		// El principal se CAPTURA, no se descarta. Antes alcanzaba con «¿es válido?» porque
		// todo lo que se exportaba era del propio servidor; desde el track de flota la salida
		// incluye telemetría POR MÁQUINA, y qué máquinas se ven depende de quién scrapea.
		var quien *Principal
		if opt.registry != nil {
			p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
			if !ok {
				deny()
				return
			}
			quien = p
		} else if opt.token != "" {
			if !validBearer(r.Header.Get("Authorization"), opt.token) {
				deny()
				return
			}
			// Token legacy: admin federado SIN capacidades de flota (C1). No ve ninguna máquina,
			// y el render lo dice en un comentario en vez de quedarse mudo.
			read, write := capsFromRole(RoleAdmin)
			quien = &Principal{Name: "legacy", Role: RoleAdmin, Read: read, Write: write}
		}
		// quien == nil sólo en loopback sin auth: confianza local, ve todo. Misma regla que el
		// resto del código (canCall, isAdmin, PuedeSobreDevice).

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		salida := metrics.render(s.engine)
		var b strings.Builder
		b.WriteString(salida)
		ahora := time.Now()
		renderFlota(&b, s.engine, quien, ahora, s.sondaIntervalo, s.version)
		// La AUTO-VIGILANCIA del empuje sale por el tirón, no por el empuje: un mecanismo de
		// monitoreo cuya única forma de avisar de su propia muerte es él mismo no avisa nunca.
		s.renderEmpuje(&b, ahora)
		_, _ = w.Write([]byte(b.String()))
	})

	// /api/actores — el CENSO: quién llama al cerebro, del ledger histórico. Es la contraparte
	// de /api/stream: el riel es el PRESENTE (quién está llamando ahora) y esto es la HISTORIA
	// (cuánto llamó cada uno). El panel necesita las dos: sin la historia, un actor que trabajó
	// todo el día y se calló hace un minuto no existe. Ver actores.go.
	mux.HandleFunc("/api/actores", s.handlerActores(opt))

	// /api/flota — la telemetría de las máquinas de la flota (flota.go): cada daemon local con
	// sync empuja acá su trabajo (nunca su sondeo, nunca contenido) y el central lo publica en su
	// propio feed con origen "flota". Así el panel del central muestra a las terminales
	// trabajando EN VIVO, no sólo lo que le llega por sync.
	mux.HandleFunc("/api/flota", s.handlerFlota(opt))

	// /api/stream — el FEED EN VIVO por SSE (livefeed.go). Cada invocación de tool sale acá en el
	// instante en que termina: qué tool, cómo salió, cuánto tardó y de quién fue.
	//
	// AUTH IGUAL QUE /metrics, y TENANCY ADEMÁS. El gate de /metrics sólo pregunta "¿es un
	// principal válido?"; acá no alcanza: el feed lleva `principal` y `project` en cada evento, así
	// que un miembro acotado a lo suyo (read=own) vería en tiempo real qué está haciendo OTRO
	// equipo — cuándo trabajan, con qué herramientas y a qué ritmo. El filtro se aplica adentro del
	// feed (subscribe), no acá, para que no dependa de que cada endpoint futuro se acuerde.
	//
	// POR QUÉ SSE Y NO WEBSOCKET: el tráfico es de una sola dirección y SSE reconecta solo. Y por
	// qué el cliente lo consume con fetch() y no con EventSource: EventSource NO puede mandar el
	// header Authorization, y la alternativa —el token en la query string— lo deja escrito en los
	// logs de acceso y en el historial del navegador. Con fetch + ReadableStream el bearer viaja
	// donde tiene que viajar.
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var principal *Principal
		if opt.registry != nil {
			p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			principal = p
		} else if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if s.live == nil {
			http.Error(w, "live feed unavailable", http.StatusServiceUnavailable)
			return
		}
		soloTrabajo := r.URL.Query().Get("kind") == KindTrabajo

		rc := http.NewResponseController(w)
		// SIN ESTO EL STREAM MUERE A LOS 90 s. El WriteTimeout del server está calibrado para las
		// respuestas cortas de /mcp (timeout + 30 s); una conexión que por diseño no termina nunca
		// lo viola siempre. Misma maniobra que la descarga del cuerpo, por el mismo motivo.
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // que ningún proxy intermedio lo bufferee
		w.WriteHeader(http.StatusOK)

		ps, fed := recallScopeFor(principal)
		// filtrar sólo si el principal está ACOTADO y tiene a qué acotarse. El registro ya
		// garantiza que read=own implica project_id no vacío (fail-closed en loadPrincipals), así
		// que un ps vacío acá significa federado o stdio local, no un agujero.
		id, ch, backlog := s.live.subscribe(ps, !fed && ps != "")
		defer s.live.unsubscribe(id)

		emitir := func(evento string, v any) bool {
			b, err := json.Marshal(v)
			if err != nil {
				return true // un evento que no serializa no puede cortar el stream
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evento, b); err != nil {
				return false
			}
			return rc.Flush() == nil
		}

		// El backlog va PRIMERO y como un solo frame: un panel que abre tiene que ver el minuto
		// anterior en vez de una pantalla en blanco esperando a que alguien haga algo. En un
		// sistema donde el trabajo real son ~23 eventos por hora, esa espera puede ser larga.
		if soloTrabajo {
			filtrado := backlog[:0]
			for _, ev := range backlog {
				if ev.Kind == KindTrabajo {
					filtrado = append(filtrado, ev)
				}
			}
			backlog = filtrado
		}
		if backlog == nil {
			backlog = []LiveEvent{}
		}
		if !emitir("backlog", backlog) {
			return
		}

		// Latido cada 20 s. No es decorativo: sin tráfico, ni el cliente ni los intermedios
		// distinguen "conexión viva y silenciosa" de "conexión muerta", y un feed que se cayó
		// pero se ve igual que uno tranquilo es peor que no tener feed.
		hb := time.NewTicker(20 * time.Second)
		defer hb.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if soloTrabajo && ev.Kind != KindTrabajo {
					continue
				}
				if !emitir("uso", ev) {
					return
				}
			case <-hb.C:
				if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
					return
				}
				if rc.Flush() != nil {
					return
				}
			}
		}
	})

	return mux
}

// bodyUpdateHandler sirve archivos de dir bajo GET /body/<archivo> (manifest + binarios
// del auto-update del cuerpo). ANTI-TRAVERSAL: la ruta se limpia contra la raíz y se
// verifica con filepath.Rel que el destino quede DENTRO de dir; cualquier `..` o escape
// da 404. Solo GET/HEAD; no lista directorios. Sin auth (frontera = tailnet).
func bodyUpdateHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/body/")
		if name == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Clean("/"+name) neutraliza los `..` (no puede subir por encima de la raíz);
		// el chequeo con Rel es la garantía extra de que no se escapa de dir.
		full := filepath.Join(dir, filepath.Clean("/"+name))
		if rel, err := filepath.Rel(dir, full); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		f, err := os.Open(full)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil || fi.IsDir() {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// El binario del cuerpo son decenas de MB; el WriteTimeout global del server (pensado
		// para respuestas chicas de /mcp) podría cortar una descarga legítima por un enlace
		// lento del tailnet. Estos son archivos estáticos de confianza: limpiamos el deadline
		// de escritura para no truncarlos. Best-effort (si el server no lo soporta, sigue igual).
		if rc := http.NewResponseController(w); rc != nil {
			_ = rc.SetWriteDeadline(time.Time{})
		}
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	}
}

// writeHTTPJSON serializa una respuesta JSON-RPC al ResponseWriter. Reporta fallos de
// marshal a stderr (nunca corrompe el cuerpo).
func writeHTTPJSON(w http.ResponseWriter, resp JsonRpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		logx.Error("error serializando respuesta HTTP JSON-RPC", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

// resolveServiceAuth resuelve el token (desde la env var nombrada) y si el bind es
// loopback, aplicando el gating de seguridad: un bind NO-loopback exige token. Devuelve
// error si la combinación es insegura. Es la lógica crítica de seguridad, aislada para
// poder testearla sin abrir un socket.
func resolveServiceAuth(cfg config.ServiceConfig) (token string, loopback bool, err error) {
	loopback = isLoopbackHost(cfg.Addr)
	if cfg.AuthTokenEnv != "" {
		token = strings.TrimSpace(os.Getenv(cfg.AuthTokenEnv))
		// Nombrar la env var señala intención de exigir auth. Si está vacía/ausente,
		// fail-closed: arrancar sin auth violaría esa intención en silencio.
		if token == "" {
			return "", loopback, fmt.Errorf("service.auth_token_env apunta a %q pero esa variable de entorno está vacía o no existe: exportala con el bearer token, o quitá auth_token_env para correr sin auth (solo válido en loopback)", cfg.AuthTokenEnv)
		}
	}
	if !loopback && token == "" {
		return "", loopback, fmt.Errorf("service.addr %q es no-loopback pero no hay token: seteá service.auth_token_env apuntando a una variable de entorno con el bearer token, o usá una dirección loopback (127.0.0.1)", cfg.Addr)
	}
	return token, loopback, nil
}

// principalsPath resuelve la ruta del registro de principals: cfg.PrincipalsFile si está
// seteada, si no el default .musubi/principals.yaml bajo la raíz del proyecto (MUSUBI_HOME).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// UNA RUTA RELATIVA SE RESUELVE CONTRA EL WORKSPACE, NUNCA CONTRA EL CWD DEL PROCESO.
//
// Antes se devolvía cfg.PrincipalsFile tal cual, y eso abría un hueco silencioso y feo. El
// `principals_file: ".musubi/principals.yaml"` que cualquiera escribe a mano se resolvía contra
// el directorio de trabajo del proceso — que en un servicio de systemd es `/`, porque el unit de
// deploy/install-musubi-brain.sh no fija WorkingDirectory.
//
// Y el fallo NO es ruidoso: loadPrincipals con un archivo inexistente NO falla, devuelve el
// registro LEGACY. O sea que el cerebro arranca perfecto, sirve perfecto, y toda la identidad
// por-miembro se degrada en silencio a UN SOLO bearer admin-federado que ve todos los proyectos.
// Hay un WARNING para binds no-loopback (isRemoteLegacyTenancy) y strict_tenancy lo rechaza, pero
// depender de que alguien lea un warning de arranque para no perder el aislamiento entre tenants
// es apoyar una garantía de seguridad sobre la atención de una persona.
//
// Lo encontró un e2e de S9b: el mismo binario, la misma config y otro directorio de trabajo se
// negó a arrancar porque leyó OTRO registro. Se negó porque había una política que validar; sin
// políticas habría arrancado, y nadie se habría enterado.
// ────────────────────────────────────────────────────────────────────────────────────────────
func (s *McpServer) principalsPath(cfg config.ServiceConfig) string {
	p := strings.TrimSpace(cfg.PrincipalsFile)
	if p == "" {
		return filepath.Join(s.projectPath, ".musubi", "principals.yaml")
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.projectPath, p)
}

// validBearer compara en tiempo constante el header Authorization contra el token
// esperado (formato "Bearer <token>").
func validBearer(authHeader, want string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}
	got := strings.TrimSpace(authHeader[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// clientIP devuelve la IP del cliente (sin el puerto) para el lockout anti fuerza-bruta.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// bearerToken extrae el token de un header "Authorization: Bearer <token>" ("" si no
// tiene el formato). Lo usa la resolución por-principal del registro (16.1c).
func bearerToken(authHeader string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return ""
	}
	return strings.TrimSpace(authHeader[len(prefix):])
}

// ListenAndServeHTTP arranca el servidor HTTP en cfg.Addr y BLOQUEA hasta que ctx se
// cancela (shutdown graceful). Aplica el gating de auth (un bind no-loopback exige
// token) y TLS si está configurado.
func (s *McpServer) ListenAndServeHTTP(ctx context.Context, cfg config.ServiceConfig) error {
	token, loopback, err := resolveServiceAuth(cfg)
	if err != nil {
		return err
	}
	// Identidad por-principal (16.1c): cargar el registro de tokens. Ruta explícita
	// (cfg.PrincipalsFile) o el default .musubi/principals.yaml del workspace. Si no existe,
	// loadPrincipals devuelve nil ⇒ modo legacy (un único bearer). Un archivo malformado
	// es error de arranque (fail-closed). El token legacy queda admitido como admin.
	principalsFile := s.principalsPath(cfg)
	s.principalsFile = principalsFile // las tools admin (musubi_token_*) mutan este mismo registro
	registry, err := loadPrincipals(principalsFile, token)
	if err != nil {
		return err
	}
	// Recarga en caliente (Track 18): si hay ARCHIVO de registro, vigilar su mtime para que altas
	// y revocaciones surtan efecto sin reiniciar el daemon (una revocación diferida es un agujero).
	// Sin archivo (legacy-only) no hay qué vigilar: se usa el registro estático tal cual.
	var resolver principalResolver
	var reload *reloadableRegistry
	if registry != nil {
		if fi, statErr := os.Stat(principalsFile); statErr == nil {
			reload = newReloadableRegistry(principalsFile, token, registry, fi.ModTime())
			resolver = reload
		} else {
			resolver = registry
		}
	}
	// Las POLÍTICAS DE FLOTA (S10) se atan al registro recién acá, que es cuando existe, y su
	// validación restante es de ARRANQUE: una política que nombra a un principal inexistente, o a
	// uno sin ninguna concesión `exec`, impide servir. Está garantizadamente muerta, y una alarma
	// muerta que nadie sabe que está muerta es peor que no tener alarma.
	if err := s.vincularRegistroDeFlota(resolver); err != nil {
		return err
	}
	// Tenancy en bind remoto (Track 18): "legacy admin-federado" = sin registro de principals (o
	// solo el bearer legacy) ⇒ un token con acceso TOTAL a todos los proyectos. En un bind
	// no-loopback eso es infra compartida SIN aislamiento por miembro. StrictTenancy lo rechaza al
	// arranque (fail-closed opt-in); apagado, un WARNING siempre lo hace visible.
	if isRemoteLegacyTenancy(loopback, registry) {
		if cfg.StrictTenancy {
			return fmt.Errorf("strict_tenancy: un bind no-loopback (%q) exige un registro de principals con al menos un miembro; configurá principals.yaml (musubi token new) o desactivá service.strict_tenancy a conciencia", cfg.Addr)
		}
		logx.Warn("musubi: sirviendo en bind remoto en modo legacy admin-federado (un único bearer con acceso total a todos los proyectos); configurá principals.yaml (musubi token new) para aislamiento por miembro, o service.strict_tenancy: true para exigirlo", "addr", cfg.Addr)
	}
	// Redacción forzada server-side (16.1d): un bind no-loopback es infra compartida ⇒
	// redactar SIEMPRE (fail-closed, no se puede desactivar); un loopback puede optar por
	// config. Cierra el hueco de un cliente que manda scope=local con un secreto crudo.
	s.forceRedact = !loopback || cfg.ForceRedact
	timeout := time.Duration(cfg.RequestTimeoutSeconds * float64(time.Second))
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	// TLS medio-seteado (solo cert o solo key) es error, no un downgrade silencioso.
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return fmt.Errorf("config TLS incompleta: seteá AMBOS service.tls_cert_file y service.tls_key_file (o ninguno)")
	}
	useTLS := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	if !loopback && !useTLS && !cfg.AllowInsecureToken {
		// Bind remoto con token pero sin TLS: el token viajaría en texto plano.
		// Fail-closed: hay que optar explícitamente (típico tras un proxy que termina TLS).
		return fmt.Errorf("bind no-loopback %q sin TLS: el bearer token viajaría en texto plano. Configurá service.tls_cert_file/tls_key_file, o seteá service.allow_insecure_token: true si un proxy termina TLS por delante", cfg.Addr)
	}
	// Auto-update del cuerpo: validar la raíz ANTES de servir. Sin esto, un valor mal puesto
	// (dir inexistente, o peor, un archivo/dir sensible) quedaría expuesto en el tailnet sin
	// auth o daría 404 mudos. Si es inválido, se deshabilita /body/ y se avisa; si es válido,
	// se loguea la raíz efectiva para que el operador vea qué se está exponiendo.
	bodyDir := strings.TrimSpace(os.Getenv("MUSUBI_BODY_UPDATE_DIR"))
	if bodyDir != "" {
		if fi, err := os.Stat(bodyDir); err != nil || !fi.IsDir() {
			logx.Warn("musubi: MUSUBI_BODY_UPDATE_DIR no es un directorio válido; /body/ deshabilitado", "dir", bodyDir)
			bodyDir = ""
		} else {
			logx.Info("musubi: auto-update del cuerpo habilitado (GET /body/)", "dir", bodyDir)
		}
	}
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: s.HTTPHandler(httpOptions{reqTimeout: timeout, token: token, loopbackOnly: loopback, registry: resolver, bodyDir: bodyDir}),
		// Timeouts contra slow-loris y conexiones colgadas. WriteTimeout deja margen
		// sobre el budget por request para no cortar una respuesta legítima a mitad.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      timeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if useTLS {
		// Pinear el piso de TLS explícitamente en vez de heredar el default del stdlib.
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	logx.Info("musubi: servidor HTTP escuchando", "addr", cfg.Addr, "path", mcpHTTPPath, "tls", useTLS, "auth", token != "")
	// Vigilar el registro para recarga en caliente; el goroutine muere con ctx (mismo shutdown).
	if reload != nil {
		go reload.watch(ctx)
	}
	// El LATIDO PROPIO DE LA FLOTA (S10): sondea a los que no tienen agente, poda las salidas
	// viejas y aplica las políticas. Arranca acá y no en el entrypoint por la misma razón que el
	// watch del registro: recién acá el registro existe, y una política sin registro no tiene a
	// quién nombrar. No-op si el intervalo está en 0 (sondeo desactivado a mano).
	go s.RunFlotaScheduler(ctx, s.sondaIntervalo)
	// Y EL EMPUJE OTLP (S11), acá y por lo mismo: recién en este punto el registro existe, y un
	// empujador sin registro no tiene a quién nombrar — y un empujador sin principal exportaría la
	// telemetría de todos los tenants. En su PROPIO ticker: la cadencia del export es la del scrape
	// (30 s) y la del sondeo es la del gasto de SSH (5 min). No-op si el empuje está apagado, que
	// es el default.
	go s.RunEmpujeOTLP(ctx)
	serveErr := make(chan error, 1)
	go func() {
		if useTLS {
			serveErr <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr <- srv.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		// Señal (SIGINT/SIGTERM en el caller): shutdown graceful, drena lo en curso.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-serveErr:
		// ListenAndServe(TLS) retornó por sí solo (típicamente un fallo de bind). El
		// goroutine no queda colgado: ya envió a serveErr (buffer 1) y termina.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// isRemoteLegacyTenancy reporta si el server escucharía en un bind NO-loopback en modo "legacy
// admin-federado": sin registro de principals real (registry nil, o solo el bearer legacy sin
// miembros). Es la condición que StrictTenancy rechaza y que el WARNING de arranque hace visible.
func isRemoteLegacyTenancy(loopback bool, registry *PrincipalRegistry) bool {
	return !loopback && (registry == nil || len(registry.principals) == 0)
}

// isLoopbackHost indica si host (con o sin puerto) resuelve a loopback o "localhost".
// Un host vacío (p.ej. ":7717", que escucha en todas las interfaces) NO es loopback.
func isLoopbackHost(host string) bool {
	h := host
	if hostPart, _, err := net.SplitHostPort(host); err == nil {
		h = hostPart
	}
	if h == "" {
		return false
	}
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// isLocalOrigin acepta solo Origins loopback (http(s)://127.0.0.1[:port] | localhost).
func isLocalOrigin(origin string) bool {
	u := origin
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return isLoopbackHost(u)
}
