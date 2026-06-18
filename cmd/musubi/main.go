package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		setupProject()
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
	case "maintain":
		runMaintain()
	case "doctor":
		runDoctor(os.Args[2:])
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
	fmt.Println("Uso: musubi <comando> [argumentos]")
	fmt.Println("Comandos disponibles:")
	fmt.Println("  setup             Inyecta Musubi en el proyecto actual (workspace + .mcp.json + hooks SessionStart/UserPromptSubmit)")
	fmt.Println("  detect            Detecta el stack del proyecto e imprime JSON en stdout")
	fmt.Println("  detect --hook-mode  Modo hook de Claude Code: silencioso si el sentinel existe, JSON de guía si no")
	fmt.Println("  turn --hook-mode  Modo hook UserPromptSubmit: inyecta contexto relevante al prompt del usuario")
	fmt.Println("  precheck --hook-mode  Modo hook PreToolUse(Read): surface el gist de un archivo en memoria de código antes de leerlo")
	fmt.Println("  catalog validate  Valida un index.json de catálogo de skills")
	fmt.Println("  catalog merge <url> [--output <ruta>]  Obtiene y fusiona un catálogo remoto en index.json")
	fmt.Println("  init              Inicializa solo el workspace .musubi/ (config + base de datos)")
	fmt.Println("  daemon            Arranca el servidor MCP sobre stdin/stdout")
	fmt.Println("  maintain          Mantiene la memoria: fusiona casi-duplicados y archiva memorias frías")
	fmt.Println("  doctor            Diagnostica la memoria; 'doctor repair --check X --apply' repara (con backup)")
	fmt.Println("  update            Descarga el último release, verifica el checksum y se auto-reemplaza")
	fmt.Println("  calibrate         (opt-in) Mide el estimador de tokens vs count_tokens; --apply persiste divisores. Requiere ANTHROPIC_API_KEY")
	fmt.Println("  version           Muestra la versión del binario")
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

	cons, dec, err := maintenanceCycle(engine, cfg.Maintenance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error en el mantenimiento: %v\n", err)
		os.Exit(1)
	}
	_ = engine.MarkMaintenanceNow()

	fmt.Printf("Mantenimiento de memoria completo:\n")
	fmt.Printf("  Consolidación: %d fusionadas de %d escaneadas\n", cons.Merged, cons.Scanned)
	fmt.Printf("  Olvido: %d archivadas de %d escaneadas\n", dec.Archived, dec.Scanned)
}

// maintenanceCycle corre consolidación + olvido con la config dada. Lo usan el
// subcomando `maintain` y el auto-mantenimiento del daemon.
func maintenanceCycle(engine *memory.DbEngine, m config.MaintenanceConfig) (memory.ConsolidateResult, memory.DecayResult, error) {
	cons, err := engine.Consolidate(m.DedupThreshold)
	if err != nil {
		return cons, memory.DecayResult{}, err
	}
	dec, err := engine.Decay(memory.DecayOptions{
		HalfLifeDays: m.DecayHalfLifeDays,
		MinSalience:  m.DecayMinSalience,
		MinAgeDays:   m.DecayMinAgeDays,
	})
	return cons, dec, err
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

	// Auto-mantenimiento throttled: si está activado y corresponde según el
	// intervalo, corre una vez en este arranque (consolidar + olvidar). Todo a
	// stderr y best-effort: nunca bloquea ni rompe el arranque del daemon.
	if cfg.Maintenance.AutoIntervalHours > 0 {
		if due, derr := engine.MaintenanceDue(cfg.Maintenance.AutoIntervalHours); derr == nil && due {
			cons, dec, mErr := maintenanceCycle(engine, cfg.Maintenance)
			if mErr != nil {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento falló: %v\n", mErr)
			} else {
				_ = engine.MarkMaintenanceNow()
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento: %d fusionadas, %d archivadas\n", cons.Merged, dec.Archived)
			}
		}
	}

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
