package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/bootstrap"
	"musubi/internal/config"
	"musubi/internal/memory"
)

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
