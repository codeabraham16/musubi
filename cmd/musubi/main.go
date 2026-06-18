package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"musubi/internal/bootstrap"
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

// noArgs maneja la invocación sin comando. Si se ejecuta en una consola
// interactiva en Windows (típico del doble clic), muestra el menú de instalación;
// si no (lanzado por Claude/otro proceso, o por pipe), imprime la ayuda y sale.
func noArgs() {
	if runtime.GOOS == "windows" && isInteractive() {
		runInteractiveMenu()
		return
	}
	printUsage()
	os.Exit(1)
}

// isInteractive devuelve true si stdin es una consola (no un pipe/redirección).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// menuAction parsea la elección del menú interactivo (case-insensitive).
func menuAction(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "L", "LOCAL":
		return "local"
	case "G", "GLOBAL":
		return "global"
	default:
		return "quit"
	}
}

// runInteractiveMenu muestra el menú de instalación por doble clic y ejecuta la
// opción elegida. Mantiene la ventana abierta hasta que el usuario presiona Enter.
func runInteractiveMenu() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("============================================")
	fmt.Println("                 Musubi")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Println("  Donde queres instalar Musubi?")
	fmt.Println()
	fmt.Println("    [L] Solo esta carpeta (local, NO toca la PC)")
	fmt.Println("    [G] Global en la PC (PATH del usuario, sin admin)")
	fmt.Println("    [Q] Salir")
	fmt.Println()
	fmt.Print("  Eleccion (L/G/Q): ")
	line, _ := reader.ReadString('\n')
	fmt.Println()

	switch menuAction(line) {
	case "local":
		setupProject()
	case "global":
		installGlobalWindows()
	default:
		fmt.Println("Cancelado. No se instalo nada.")
	}

	fmt.Print("\nPresiona Enter para salir...")
	reader.ReadString('\n')
}

// installGlobalWindows copia el binario en ejecución a una carpeta del usuario,
// la agrega al PATH (sin admin) y corre el setup del proyecto actual apuntando a
// esa ubicación estable.
func installGlobalWindows() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Error al ubicar el ejecutable: %v\n", err)
		return
	}
	installDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "musubi")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Printf("Error al crear %s: %v\n", installDir, err)
		return
	}
	dest := filepath.Join(installDir, "musubi.exe")
	if !sameFile(exe, dest) {
		if err := copyFile(exe, dest); err != nil {
			fmt.Printf("Error al copiar el binario: %v\n", err)
			return
		}
	}
	fmt.Printf("  ✓ Binario instalado en %s\n", dest)

	// Agregar installDir al PATH del usuario (sin admin) vía PowerShell.
	psDir := "'" + strings.ReplaceAll(installDir, "'", "''") + "'"
	ps := fmt.Sprintf(`$d=%s; $p=[Environment]::GetEnvironmentVariable('Path','User'); if ($p -notlike "*$d*") { [Environment]::SetEnvironmentVariable('Path', "$p;$d", 'User'); 'PATH actualizado' } else { 'PATH ya incluia el directorio' }`, psDir)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		fmt.Printf("  ! No se pudo actualizar el PATH automaticamente: %v\n", err)
		fmt.Printf("    Agregalo manualmente al PATH: %s\n", installDir)
	} else {
		fmt.Printf("  ✓ %s", string(out))
	}

	// Setup del proyecto actual apuntando al binario instalado (ruta estable).
	setupProjectWith(dest)
	fmt.Println("\nGlobal listo. Abri una terminal NUEVA para usar el comando 'musubi'.")
}

// sameFile compara dos rutas resueltas (case-insensitive, para Windows).
func sameFile(a, b string) bool {
	ap, e1 := filepath.Abs(a)
	bp, e2 := filepath.Abs(b)
	return e1 == nil && e2 == nil && strings.EqualFold(ap, bp)
}

// copyFile copia src a dst (binario).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// workspaceDir resuelve el directorio de trabajo de Musubi.
// Prioriza la variable de entorno MUSUBI_HOME (útil para correr como servidor
// MCP global con una memoria estable), y cae al directorio actual.
func workspaceDir() string {
	if home := os.Getenv("MUSUBI_HOME"); home != "" {
		return home
	}
	return "."
}

// ensureWorkspace crea el directorio .musubi y un config.yaml por defecto si faltan.
// No escribe a stdout (lo usa el daemon, cuyo stdout es el canal JSON-RPC).
func ensureWorkspace(root string) error {
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", dir, err)
	}
	configPath := filepath.Join(dir, config.ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		content, err := config.Default().Marshal()
		if err != nil {
			return fmt.Errorf("error generando config por defecto: %w", err)
		}
		if err := os.WriteFile(configPath, content, 0644); err != nil {
			return fmt.Errorf("error escribiendo %s: %w", configPath, err)
		}
	}
	return nil
}

func initProject() {
	root := workspaceDir()
	fmt.Printf("Inicializando entorno de Musubi en %s...\n", filepath.Join(root, config.DirName))

	if err := ensureWorkspace(root); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Crear base de datos inicial vacía
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Printf("Error al inicializar la base de datos de memoria: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("Entorno Musubi inicializado correctamente. Listo para inyección MCP.")
}

// setupProject inyecta Musubi en el proyecto actual de punta a punta:
// crea el workspace, un skill de arranque, registra el servidor en .mcp.json
// (para que Claude Code lo cargue solo) y protege la base de datos en .gitignore.
func setupProject() { setupProjectWith("") }

// setupProjectWith inyecta Musubi en el directorio actual. Si exeOverride no está
// vacía, usa esa ruta para el comando en .mcp.json y el hook (útil para el modo
// global, donde el binario se copia a una ubicación estable del PATH); si está
// vacía, usa os.Executable() (el binario que está corriendo).
func setupProjectWith(exeOverride string) {
	root, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Error al resolver el directorio del proyecto: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Inyectando Musubi en %s\n", root)

	// Detectar si .musubi/ ya existe para poder hacer rollback atómico en proyectos
	// nuevos: si creamos el workspace y luego falla un paso crítico, lo eliminamos
	// para dejar el proyecto limpio (re-ejecutar setup funciona sin estado parcial).
	musubiDir := filepath.Join(root, config.DirName)
	_, statErr := os.Stat(musubiDir)
	freshWorkspace := os.IsNotExist(statErr)
	rollback := func() {
		if freshWorkspace {
			os.RemoveAll(musubiDir)
		}
	}

	// 1. Workspace + base de datos.
	if err := ensureWorkspace(root); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		rollback()
		fmt.Printf("Error al inicializar la base de datos: %v\n", err)
		os.Exit(1)
	}
	engine.Close()
	fmt.Println("  ✓ Workspace .musubi/ (config + memoria) listo")

	// 2. Bundle de skills cognitivas de arranque (analizar/deducir/planear + perfil).
	if err := writeCognitiveSkills(root); err != nil {
		fmt.Printf("  ! No se pudieron escribir las skills cognitivas: %v\n", err)
	} else {
		fmt.Println("  ✓ Skills cognitivas en .musubi/skills/ (analyze, deduce, plan, profile)")
	}

	// 3. Registrar el servidor en .mcp.json para carga automática.
	exePath := exeOverride
	if exePath == "" {
		exePath, err = os.Executable()
		if err != nil {
			exePath = "musubi"
		}
	}
	if err := writeMCPConfig(root, exePath); err != nil {
		rollback()
		fmt.Printf("Error al escribir .mcp.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ .mcp.json (Claude Code cargará 'musubi' al abrir el proyecto)")

	// 4. Hooks en .claude/settings.json: SessionStart (arranque) + UserPromptSubmit
	//    (loop dirigido: contexto por turno).
	if err := writeClaudeHook(root, exePath); err != nil {
		fmt.Printf("  ! No se pudo registrar el hook SessionStart: %v\n", err)
	} else {
		fmt.Println("  ✓ Hook SessionStart en .claude/settings.json (auto-descubrimiento de skills)")
	}
	if err := writeTurnHook(root, exePath); err != nil {
		fmt.Printf("  ! No se pudo registrar el hook UserPromptSubmit: %v\n", err)
	} else {
		fmt.Println("  ✓ Hook UserPromptSubmit en .claude/settings.json (loop dirigido: contexto por turno)")
	}
	if err := writeCodeMemoryHook(root, exePath); err != nil {
		fmt.Printf("  ! No se pudo registrar el hook PreToolUse(Read): %v\n", err)
	} else {
		fmt.Println("  ✓ Hook PreToolUse(Read) en .claude/settings.json (memoria de código: gist antes de leer)")
	}

	// 5. Proteger la base de datos de runtime en git.
	if err := ensureGitignore(root); err == nil {
		fmt.Println("  ✓ .gitignore actualizado (.musubi/memory.db)")
	}

	fmt.Println("\nListo. Reabrí el proyecto en Claude Code y el servidor 'musubi' estará disponible.")
	fmt.Println("En la primera sesión, Claude detectará el stack y generará skills personalizadas automáticamente.")
}

// quoteExe entrecomilla la ruta del ejecutable para el comando del hook (string de
// shell), de modo que una ruta con espacios (ej. "C:\Users\First Last\...") no se
// parta. Idempotente: no re-entrecomilla si ya viene citada.
func quoteExe(exePath string) string {
	if strings.HasPrefix(exePath, "\"") {
		return exePath
	}
	return "\"" + exePath + "\""
}

// writeClaudeHook inyecta (idempotente) el hook SessionStart de auto-descubrimiento
// de skills en {root}/.claude/settings.json usando bootstrap.MergeClaudeSettings.
// Si el archivo no existe, lo crea. Si ya contiene el hook de Musubi, no lo duplica.
func writeClaudeHook(root, exePath string) error {
	claudeDir := filepath.Join(root, config.ClaudeDir)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", claudeDir, err)
	}
	settingsPath := filepath.Join(claudeDir, config.ClaudeSettingsFile)
	existing, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al leer %s: %w", settingsPath, err)
	}
	hook := bootstrap.HookCommand{
		Type:    "command",
		Command: quoteExe(exePath) + " detect --hook-mode",
		Timeout: 10,
	}
	merged, err := bootstrap.MergeClaudeSettings(existing, "SessionStart", "startup", hook)
	if err != nil {
		return fmt.Errorf("error al mergear settings.json: %w", err)
	}
	return os.WriteFile(settingsPath, merged, 0644)
}

// writeTurnHook inyecta (idempotente) el hook UserPromptSubmit del loop dirigido
// en {root}/.claude/settings.json: antes de cada prompt, Musubi inyecta el
// contexto relevante a lo que el usuario pidió. UserPromptSubmit no usa matcher.
func writeTurnHook(root, exePath string) error {
	claudeDir := filepath.Join(root, config.ClaudeDir)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", claudeDir, err)
	}
	settingsPath := filepath.Join(claudeDir, config.ClaudeSettingsFile)
	existing, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al leer %s: %w", settingsPath, err)
	}
	hook := bootstrap.HookCommand{
		Type:    "command",
		Command: quoteExe(exePath) + " turn --hook-mode",
		Timeout: 10,
	}
	merged, err := bootstrap.MergeClaudeSettings(existing, "UserPromptSubmit", "", hook)
	if err != nil {
		return fmt.Errorf("error al mergear settings.json: %w", err)
	}
	return os.WriteFile(settingsPath, merged, 0644)
}

// writeCodeMemoryHook inyecta (idempotente) el hook PreToolUse con matcher "Read"
// en {root}/.claude/settings.json: antes de cada lectura de archivo, Musubi
// surface el gist en memoria de código (o recuerda guardarlo). Hace automático el
// uso de la memoria de código sin que el agente deba acordarse.
func writeCodeMemoryHook(root, exePath string) error {
	claudeDir := filepath.Join(root, config.ClaudeDir)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", claudeDir, err)
	}
	settingsPath := filepath.Join(claudeDir, config.ClaudeSettingsFile)
	existing, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al leer %s: %w", settingsPath, err)
	}
	hook := bootstrap.HookCommand{
		Type:    "command",
		Command: quoteExe(exePath) + " precheck --hook-mode",
		Timeout: 10,
	}
	merged, err := bootstrap.MergeClaudeSettings(existing, "PreToolUse", "Read", hook)
	if err != nil {
		return fmt.Errorf("error al mergear settings.json: %w", err)
	}
	return os.WriteFile(settingsPath, merged, 0644)
}

func writeMCPConfig(root, exePath string) error {
	mcpPath := filepath.Join(root, ".mcp.json")
	existing, err := os.ReadFile(mcpPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	merged, err := bootstrap.MergeMCPServer(existing, "musubi", bootstrap.MCPServerEntry{
		Command: exePath,
		Args:    []string{"daemon"},
		Env:     map[string]string{"MUSUBI_HOME": root},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(mcpPath, merged, 0644)
}

func ensureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	entry := config.DirName + "/" + config.DBFile
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(data), entry) {
		return nil
	}
	line := entry + "\n"
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		line = "\n" + line
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
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
