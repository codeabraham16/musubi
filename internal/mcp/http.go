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
	"strings"
	"sync"
	"time"

	"musubi/internal/config"
	"musubi/internal/logx"
)

const (
	mcpHTTPPath    = "/mcp"
	maxRequestBody = 4 << 20 // 4 MiB: techo del body JSON-RPC entrante.
)

// httpOptions configura el handler HTTP.
type httpOptions struct {
	reqTimeout time.Duration
	// token, si no es vacío, exige Authorization: Bearer <token> en cada request.
	token string
	// loopbackOnly activa la defensa anti DNS-rebinding (Host loopback + Origin local).
	// Se usa en modo loopback; en modo remoto el bearer token es el gate y estos checks
	// romperían a clientes legítimos (que usan un Host no-loopback).
	loopbackOnly bool
}

// HTTPHandler devuelve el http.Handler que sirve MCP sobre HTTP. POST /mcp recibe un
// request JSON-RPC y responde el resultado; GET /mcp (upgrade SSE) queda reservado
// (405) porque Musubi no emite mensajes server-initiated todavía.
func (s *McpServer) HTTPHandler(opt httpOptions) http.Handler {
	var mu sync.Mutex // serializa el dispatch: línea base segura (sin RMW concurrente).
	metrics := &httpMetrics{}
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
		// Autenticación: si hay token configurado, exigir Bearer válido (comparación
		// en tiempo constante para no filtrar el token por timing).
		if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
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

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
		if err != nil {
			writeHTTPJSON(w, errResponse(nil, rpcErrorf(codeParseError, "error leyendo el body")))
			return
		}
		var req JsonRpcRequest
		if jerr := json.Unmarshal(body, &req); jerr != nil {
			writeHTTPJSON(w, errResponse(nil, rpcErrorf(codeParseError, "Parse error")))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), opt.reqTimeout)
		defer cancel()

		// Sección crítica acotada al dispatch; defer garantiza el unlock aunque
		// Dispatch paniquee (no debería: recupera internamente), evitando un deadlock
		// que colgaría todas las peticiones siguientes.
		resp, ok := func() (JsonRpcResponse, bool) {
			mu.Lock()
			defer mu.Unlock()
			return s.Dispatch(ctx, req)
		}()

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

	// Métricas: detrás de auth si hay token (son datos operativos).
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics.render()))
	})
	return mux
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

// ListenAndServeHTTP arranca el servidor HTTP en cfg.Addr y BLOQUEA hasta que ctx se
// cancela (shutdown graceful). Aplica el gating de auth (un bind no-loopback exige
// token) y TLS si está configurado.
func (s *McpServer) ListenAndServeHTTP(ctx context.Context, cfg config.ServiceConfig) error {
	token, loopback, err := resolveServiceAuth(cfg)
	if err != nil {
		return err
	}
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
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: s.HTTPHandler(httpOptions{reqTimeout: timeout, token: token, loopbackOnly: loopback}),
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
