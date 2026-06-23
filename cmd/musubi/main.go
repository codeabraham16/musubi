// Command musubi es la CLI y el daemon MCP de Musubi: instala el binario, prepara
// el workspace, corre mantenimiento y sirve el servidor MCP de memoria persistente.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/mcp"
	"musubi/internal/memory"
)

// version es la versión del binario. Se inyecta en el release vía
// -ldflags "-X main.version=<tag>"; en builds locales queda "dev".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		noArgs()
		return
	}

	command := os.Args[1]
	switch command {
	case "init":
		initProject()
	case "setup":
		runSetup(os.Args[2:])
	case "detect":
		runDetect()
	case "turn":
		runTurn()
	case "precheck":
		runPrecheck()
	case "catalog":
		runCatalog(os.Args[2:])
	case "daemon":
		runDaemon()
	case "serve":
		runServe(os.Args[2:])
	case "maintain":
		runMaintain()
	case "doctor":
		runDoctor(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("musubi %s\n", version)
	case "update":
		runUpdate()
	case "calibrate":
		runCalibrate(os.Args[2:])
	default:
		fmt.Printf("Comando desconocido: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(cBold("Uso:") + " musubi <comando> [argumentos]")

	// section imprime un encabezado de grupo; cmd imprime un comando alineado. El
	// padding se aplica ANTES de colorear, así las secuencias ANSI no descuadran la
	// columna (cuando el color está apagado, queda igual de alineado).
	section := func(title string) { fmt.Println("\n" + cCyan(title)) }
	cmd := func(name, desc string) { fmt.Printf("  %s  %s\n", cBold(fmt.Sprintf("%-32s", name)), desc) }

	section("Instalación")
	cmd("setup [--agent <claude|cursor>]", "Inyecta Musubi en el proyecto actual (workspace + MCP + hooks)")
	cmd("init", "Inicializa solo el workspace .musubi/ (config + base de datos)")

	section("Servidor MCP")
	cmd("daemon", "Arranca el servidor MCP sobre stdin/stdout")
	cmd("serve [--addr host:port]", "Servidor MCP sobre HTTP (modo servicio, opt-in; solo loopback)")

	section("Memoria")
	cmd("maintain", "Fusiona casi-duplicados y archiva memorias frías")
	cmd("doctor", "Diagnostica la memoria; 'doctor repair --check X --apply' repara")
	cmd("export [--out <ruta>]", "Vuelca un snapshot JSON (salud + tokens + grafo) para dashboards")
	cmd("calibrate", "(opt-in) Mide el estimador de tokens vs count_tokens (requiere ANTHROPIC_API_KEY)")

	section("Catálogo de skills")
	cmd("catalog validate", "Valida un index.json de catálogo de skills")
	cmd("catalog merge <url> [--output <ruta>]", "Obtiene y fusiona un catálogo remoto en index.json")

	section("Binario")
	cmd("update", "Descarga el último release, verifica el checksum y se auto-reemplaza")
	cmd("version", "Muestra la versión del binario")

	section("Hooks (uso interno de Claude Code)")
	cmd("detect [--hook-mode]", "Detecta el stack / SessionStart: auto-descubrimiento + priming")
	cmd("turn --hook-mode", "UserPromptSubmit: inyecta contexto relevante al prompt")
	cmd("precheck --hook-mode", "PreToolUse(Read): gist de un archivo antes de leerlo")
}

// runMaintain corre el auto-mantenimiento de la memoria (consolidar + olvidar)
// como proceso one-shot e imprime un resumen en stdout.
func runMaintain() {
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
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	rep, err := maintenanceCycle(engine, cfg.Maintenance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error en el mantenimiento: %v\n", err)
		os.Exit(1)
	}
	_ = engine.MarkMaintenanceNow()

	fmt.Printf("Mantenimiento de memoria completo:\n")
	fmt.Printf("  Consolidación: %d fusionadas de %d escaneadas\n", rep.Consolidate.Merged, rep.Consolidate.Scanned)
	fmt.Printf("  Olvido: %d archivadas de %d escaneadas\n", rep.Decay.Archived, rep.Decay.Scanned)
	fmt.Printf("  Retención: %d purgadas\n", rep.Purged)
}

// maintenanceCycle corre el ciclo de mantenimiento completo (consolidar + olvidar +
// purgar + compactar) con la config dada, delegando en engine.Maintain. Lo usan el
// subcomando `maintain` y el auto-mantenimiento del daemon.
func maintenanceCycle(engine *memory.DbEngine, m config.MaintenanceConfig) (memory.MaintenanceReport, error) {
	return engine.Maintain(memory.MaintenanceOptions{
		DedupThreshold:         m.DedupThreshold,
		DecayHalfLifeDays:      m.DecayHalfLifeDays,
		DecayMinSalience:       m.DecayMinSalience,
		DecayMinAgeDays:        m.DecayMinAgeDays,
		DecayProtectImportance: m.DecayProtectImportance,
		PurgeArchivedAfterDays: m.PurgeArchivedAfterDays,
		Vacuum:                 m.Vacuum,
	})
}

// runServe arranca el servidor MCP sobre HTTP (modo servicio, Track 4). Es opt-in:
// requiere service.enabled en la config o un --addr explícito. Solo bind a loopback.
// Comparte toda la configuración del motor y las tools con el modo daemon (stdio).
func runServe(args []string) {
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

	// Overrides por flag: --addr <host:port> (o --addr=...) habilita el modo servicio
	// con esa dirección; --enable lo habilita con la addr de la config.
	svc := cfg.Service
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--addr" && i+1 < len(args):
			svc.Addr = args[i+1]
			svc.Enabled = true
			i++
		case strings.HasPrefix(args[i], "--addr="):
			svc.Addr = strings.TrimPrefix(args[i], "--addr=")
			svc.Enabled = true
		case args[i] == "--enable":
			svc.Enabled = true
		}
	}
	if !svc.Enabled {
		fmt.Fprintln(os.Stderr, "musubi serve: el modo servicio está desactivado. Activá 'service.enabled: true' en .musubi/config.yaml o pasá --addr <host:port>.")
		os.Exit(1)
	}
	if svc.Addr == "" {
		svc.Addr = config.Default().Service.Addr
	}
	if svc.RequestTimeoutSeconds == 0 {
		svc.RequestTimeoutSeconds = config.Default().Service.RequestTimeoutSeconds
	}

	embedder, err := embedding.NewProvider(cfg.Embedding)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al configurar embeddings: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	server := mcp.NewMcpServer(engine, root, embedder, mcp.WithSourcing(cfg.Sourcing), mcp.WithMemory(cfg.Memory), mcp.WithMaintenance(cfg.Maintenance), mcp.WithGraph(cfg.Graph), mcp.WithConflicts(cfg.Conflicts), mcp.WithPipeline(cfg.Pipeline), mcp.WithMultiAgent(cfg.MultiAgent))

	// Shutdown graceful: ctx se cancela con SIGINT/SIGTERM; ListenAndServeHTTP retorna.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.ListenAndServeHTTP(ctx, svc); err != nil {
		fmt.Fprintf(os.Stderr, "musubi serve: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon() {
	root := workspaceDir()

	// Auto-inicializa el workspace si falta (robusto para uso como MCP server).
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}

	embedder, err := embedding.NewProvider(cfg.Embedding)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al configurar embeddings: %v\n", err)
		os.Exit(1)
	}

	// Cargar motor de base de datos local
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Chequeo de versión throttled: avisa por stderr si hay una versión nueva
	// (no descarga ni reemplaza nada). Corre en goroutine para no demorar el
	// arranque. CheckIntervalHours <= 0 lo desactiva.
	if cfg.Update.CheckIntervalHours > 0 {
		if due, derr := engine.MetaDue(metaLastUpdateCheck, cfg.Update.CheckIntervalHours); derr == nil && due {
			_ = engine.MarkMetaNow(metaLastUpdateCheck)
			go notifyIfOutdated()
		}
	}

	// Arrancar servidor MCP sobre Stdin/Stdout, con sourcing y memoria configurados.
	server := mcp.NewMcpServer(engine, root, embedder, mcp.WithSourcing(cfg.Sourcing), mcp.WithMemory(cfg.Memory), mcp.WithMaintenance(cfg.Maintenance), mcp.WithGraph(cfg.Graph), mcp.WithConflicts(cfg.Conflicts), mcp.WithPipeline(cfg.Pipeline), mcp.WithMultiAgent(cfg.MultiAgent))

	// Auto-mantenimiento de fondo (Track 5 / T5.2): el daemon es long-running; sin esto el
	// ciclo cognitivo (consolidar/olvidar/purgar) solo correría una vez al arrancar. Dos
	// goroutines best-effort que serializan contra el dispatch vía el write-lock del server:
	//   (1) una corrida de arranque NO bloqueante (un VACUUM grande no demora el primer pedido);
	//   (2) un ticker periódico que repite el ciclo intra-sesión.
	// El ctx se cancela al retornar de runDaemon (señal o EOF de stdin), parando el ticker.
	maintCtx, stopMaint := context.WithCancel(context.Background())
	defer stopMaint()
	if cfg.Maintenance.AutoIntervalHours > 0 {
		go func() {
			if ran, rep, mErr := server.RunScheduledMaintenance(); mErr != nil {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento de arranque falló: %v\n", mErr)
			} else if ran {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento: %d fusionadas, %d archivadas, %d purgadas\n", rep.Consolidate.Merged, rep.Decay.Archived, rep.Purged)
			}
		}()
		go server.RunMaintenanceScheduler(maintCtx, time.Duration(cfg.Maintenance.AutoIntervalHours*float64(time.Hour)))
	}

	// Capturar SIGINT/SIGTERM para graceful shutdown: el select espera hasta que el
	// servidor termine (EOF de stdin) o llegue una señal. En ambos casos se retorna
	// de runDaemon y el defer engine.Close() cierra la DB limpiamente.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		defer close(done)
		server.Start()
	}()

	select {
	case sig := <-sigs:
		fmt.Fprintf(os.Stderr, "musubi: señal %v recibida, cerrando\n", sig)
	case <-done:
	}
}
