package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/bootstrap"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/mcp"
	"musubi/internal/memory"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "init":
		initProject()
	case "setup":
		setupProject()
	case "detect":
		runDetect()
	case "catalog":
		runCatalog(os.Args[2:])
	case "daemon":
		runDaemon()
	default:
		fmt.Printf("Comando desconocido: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Uso: musubi <comando> [argumentos]")
	fmt.Println("Comandos disponibles:")
	fmt.Println("  setup             Inyecta Musubi en el proyecto actual (workspace + .mcp.json + hook SessionStart)")
	fmt.Println("  detect            Detecta el stack del proyecto e imprime JSON en stdout")
	fmt.Println("  detect --hook-mode  Modo hook de Claude Code: silencioso si el sentinel existe, JSON de guía si no")
	fmt.Println("  catalog validate  Valida un index.json de catálogo de skills")
	fmt.Println("  init              Inicializa solo el workspace .musubi/ (config + base de datos)")
	fmt.Println("  daemon            Arranca el servidor MCP sobre stdin/stdout")
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
func setupProject() {
	root, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Error al resolver el directorio del proyecto: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Inyectando Musubi en %s\n", root)

	// 1. Workspace + base de datos.
	if err := ensureWorkspace(root); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Printf("Error al inicializar la base de datos: %v\n", err)
		os.Exit(1)
	}
	engine.Close()
	fmt.Println("  ✓ Workspace .musubi/ (config + memoria) listo")

	// 2. Skill de arranque (solo si no hay ninguno).
	if err := writeStarterSkill(root); err != nil {
		fmt.Printf("  ! No se pudo escribir el skill de arranque: %v\n", err)
	} else {
		fmt.Println("  ✓ Skill de arranque en .musubi/skills/")
	}

	// 3. Registrar el servidor en .mcp.json para carga automática.
	exePath, err := os.Executable()
	if err != nil {
		exePath = "musubi"
	}
	if err := writeMCPConfig(root, exePath); err != nil {
		fmt.Printf("Error al escribir .mcp.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ .mcp.json (Claude Code cargará 'musubi' al abrir el proyecto)")

	// 4. Hook SessionStart en .claude/settings.json para auto-descubrimiento de skills.
	if err := writeClaudeHook(root, exePath); err != nil {
		fmt.Printf("  ! No se pudo registrar el hook SessionStart: %v\n", err)
	} else {
		fmt.Println("  ✓ Hook SessionStart en .claude/settings.json (auto-descubrimiento de skills)")
	}

	// 5. Proteger la base de datos de runtime en git.
	if err := ensureGitignore(root); err == nil {
		fmt.Println("  ✓ .gitignore actualizado (.musubi/memory.db)")
	}

	fmt.Println("\nListo. Reabrí el proyecto en Claude Code y el servidor 'musubi' estará disponible.")
	fmt.Println("En la primera sesión, Claude detectará el stack y generará skills personalizadas automáticamente.")
}

func writeStarterSkill(root string) error {
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(skillsDir, "starter.yaml")
	if _, err := os.Stat(path); err == nil {
		return nil // ya existe, no sobrescribir
	}
	content := `name: starter
description: "Skill de arranque generado por 'musubi setup'. Editalo para tu proyecto."
triggers:
  - "*"
capabilities: []
rules: |
  - Guardá decisiones y aprendizajes con musubi_save_observation.
  - Antes de empezar algo, buscá contexto previo con musubi_search_keyword.
`
	return os.WriteFile(path, []byte(content), 0644)
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
		Command: exePath + " detect --hook-mode",
		Timeout: 10,
	}
	merged, err := bootstrap.MergeClaudeSettings(existing, "startup", hook)
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

	// Arrancar servidor MCP sobre Stdin/Stdout, con sourcing configurado.
	server := mcp.NewMcpServer(engine, root, embedder, mcp.WithSourcing(cfg.Sourcing))
	server.Start()
}
