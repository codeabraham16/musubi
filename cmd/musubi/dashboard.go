package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

//go:embed assets/dashboard.html
var dashboardAssets embed.FS

// dashboard.go implementa 'musubi dashboard': una UI LOCAL de solo lectura de la
// memoria. Sirve un HTML embebido en / y un snapshot JSON en vivo en /api/snapshot
// (reusa buildExportSnapshot); el navegador hace polling y se actualiza solo. Es
// opt-in y NO gasta tokens: corre como proceso aparte, no se engancha a ningún hook
// ni inyecta nada al contexto del agente —los datos van de SQLite al navegador, sin
// LLM en el medio—. Solo liga a loopback: por diseño es de uso local.

// runDashboard implementa 'musubi dashboard [--addr 127.0.0.1:7777] [--no-open]'.
func runDashboard(args []string) {
	addr := parseFlagValue(args, "--addr")
	if addr == "" {
		addr = "127.0.0.1:7777"
	}
	if !isLoopbackAddr(addr) {
		fmt.Fprintf(os.Stderr, "musubi dashboard: solo se permite bind a loopback (127.0.0.1/localhost/::1), no %q\n", addr)
		os.Exit(1)
	}

	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi dashboard: no se pudo escuchar en %s: %v\n", addr, err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:           dashboardHandler(engine, cfg.Memory.SessionTokenBudget, projectLabel(root)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	url := "http://" + addr + "/"
	fmt.Printf("Musubi dashboard en %s  (solo lectura · loopback · 0 tokens)\n", url)
	fmt.Println("Ctrl+C para detener.")
	if !hasFlag(args, "--no-open") {
		openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "musubi dashboard: %v\n", err)
		os.Exit(1)
	}
}

// dashboardHandler arma el router: el HTML embebido en / y el snapshot JSON en vivo
// en /api/snapshot. Aislado para poder testearlo con httptest.
func dashboardHandler(engine *memory.DbEngine, budget int, project string) http.Handler {
	mux := http.NewServeMux()
	page, _ := dashboardAssets.ReadFile("assets/dashboard.html")

	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		snap, err := buildExportSnapshot(engine, version, budget, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snap.Project = project
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snap)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})

	return mux
}

// isLoopbackAddr indica si addr (host:port) liga SOLO a loopback. Un host vacío
// (":7777" = todas las interfaces) o cualquier IP no-loopback se rechaza: el
// dashboard no se expone a la red.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// openBrowser abre url en el navegador por defecto (best-effort; no falla si no puede).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
