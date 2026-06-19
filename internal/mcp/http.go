package mcp

// Transporte HTTP del servidor MCP (Track 4 / T4.2): expone el mismo dispatch que el
// stdio sobre un endpoint HTTP, para usar Musubi como servicio. Es OPT-IN
// (config.Service.Enabled) y por seguridad SOLO admite bind a loopback en este release;
// la autenticación y el bind remoto llegan en un slice posterior.
//
// Modelo de concurrencia: las peticiones se SERIALIZAN sobre un mutex (línea base
// segura, sin riesgo de read-modify-write en el motor). La concurrencia real es un
// slice posterior, tras la auditoría RMW. El seam Dispatch (puro, sin estado mutable
// compartido) ya deja ese cambio listo.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

// HTTPHandler devuelve el http.Handler que sirve MCP sobre HTTP. POST /mcp recibe un
// request JSON-RPC y responde el resultado; GET /mcp (upgrade SSE) queda reservado
// (405) porque Musubi no emite mensajes server-initiated todavía. reqTimeout acota
// cada request (espejo del deadline de 60s del stdio).
func (s *McpServer) HTTPHandler(reqTimeout time.Duration) http.Handler {
	var mu sync.Mutex // serializa el dispatch: línea base segura (sin RMW concurrente).
	mux := http.NewServeMux()
	mux.HandleFunc(mcpHTTPPath, func(w http.ResponseWriter, r *http.Request) {
		// Defensa de superficie de red (guía de seguridad del transporte HTTP de MCP):
		// solo loopback y sin Origin cross-site, aunque el bind ya esté forzado a loopback.
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback host", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !isLocalOrigin(o) {
			http.Error(w, "forbidden: cross-origin", http.StatusForbidden)
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

		ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
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

// ListenAndServeHTTP arranca el servidor HTTP en cfg.Addr y BLOQUEA hasta que ctx se
// cancela (shutdown graceful). En este release solo admite bind a loopback.
func (s *McpServer) ListenAndServeHTTP(ctx context.Context, cfg config.ServiceConfig) error {
	if !isLoopbackHost(cfg.Addr) {
		return fmt.Errorf("service.addr %q no es loopback: el bind remoto requiere autenticación (slice posterior); usá 127.0.0.1", cfg.Addr)
	}
	timeout := time.Duration(cfg.RequestTimeoutSeconds * float64(time.Second))
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: s.HTTPHandler(timeout),
		// Timeouts contra slow-loris y conexiones colgadas. WriteTimeout deja margen
		// sobre el budget por request para no cortar una respuesta legítima a mitad.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      timeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	logx.Info("musubi: servidor HTTP escuchando", "addr", cfg.Addr, "path", mcpHTTPPath)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		// Señal (SIGINT/SIGTERM en el caller): shutdown graceful, drena lo en curso.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-serveErr:
		// ListenAndServe retornó por sí solo (típicamente un fallo de bind). El
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
