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
func setupProject() { setupProjectWith("", "") }

// runSetup maneja `musubi setup [--agent <nombre>]`. Sin --agent usa Claude Code.
// Sugiere los agentes detectados en el proyecto si hay alguno además del elegido.
func runSetup(args []string) {
	agent := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--agent=") {
				agent = strings.TrimPrefix(args[i], "--agent=")
			}
		}
	}
	if root, err := filepath.Abs("."); err == nil {
		if detected := bootstrap.DetectAgents(root); len(detected) > 0 {
			fmt.Printf("Agentes detectados en el proyecto: %s\n", strings.Join(detected, ", "))
		}
	}
	setupProjectWith("", agent)
}

// setupProjectWith inyecta Musubi en el directorio actual para el agente dado
// (agent vacío → Claude Code, el default histórico). Si exeOverride no está vacía,
// usa esa ruta para el comando en la config MCP y los hooks (útil para el modo
// global); si está vacía, usa os.Executable() (el binario que está corriendo).
func setupProjectWith(exeOverride, agent string) {
	target, ok := bootstrap.ResolveAgent(agent)
	if !ok {
		fmt.Printf("Agente desconocido: %q. Soportados: %s\n", agent, strings.Join(bootstrap.KnownAgentNames(), ", "))
		os.Exit(1)
	}

	root, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Error al resolver el directorio del proyecto: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Inyectando Musubi en %s (agente: %s)\n", root, target.Name)

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
		fmt.Println("  ✓ Skills cognitivas en .musubi/skills/ (analyze, deduce, plan, profile, orchestrate, audit)")
	}

	// 2b. Templates de artefactos SDD (proposal/spec/design/tasks) — scaffold versionado.
	if err := writeSddTemplates(root); err != nil {
		fmt.Printf("  ! No se pudieron escribir los templates SDD: %v\n", err)
	} else {
		fmt.Println("  ✓ Templates SDD en .musubi/templates/sdd/ (proposal, spec, design, tasks)")
	}

	// 3. Registrar el servidor en .mcp.json para carga automática.
	exePath := exeOverride
	if exePath == "" {
		exePath, err = os.Executable()
		if err != nil {
			exePath = "musubi"
		}
	}
	if err := writeMCPConfigAt(root, exePath, target.MCPPath); err != nil {
		rollback()
		fmt.Printf("Error al escribir %s: %v\n", target.MCPPath, err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ %s (%s cargará 'musubi' al abrir el proyecto)\n", target.MCPPath, target.Name)

	// 4. Hooks: solo para agentes que tienen sistema de hooks (Claude Code). Otros
	//    agentes registran el MCP pero no hooks (no existe el mecanismo).
	if target.SupportsHooks {
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
	} else {
		fmt.Printf("  · %s no tiene sistema de hooks; se registró solo el servidor MCP.\n", target.Name)
	}

	// 5. Proteger la base de datos de runtime en git.
	if err := ensureGitignore(root); err == nil {
		fmt.Println("  ✓ .gitignore actualizado (.musubi/memory.db)")
	}

	fmt.Printf("\nListo. Reabrí el proyecto en %s y el servidor 'musubi' estará disponible.\n", target.Name)
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

// writeMCPConfig registra el servidor en .mcp.json (Claude Code). Envoltorio de
// writeMCPConfigAt para compatibilidad.
func writeMCPConfig(root, exePath string) error {
	return writeMCPConfigAt(root, exePath, ".mcp.json")
}

// writeMCPConfigAt registra (idempotente) el servidor musubi en el archivo de config
// MCP del agente (relPath relativo a root, ej. ".mcp.json" o ".cursor/mcp.json").
// Crea el directorio padre si hace falta. El esquema mcpServers es común a los agentes.
func writeMCPConfigAt(root, exePath, relPath string) error {
	mcpPath := filepath.Join(root, relPath)
	if dir := filepath.Dir(mcpPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("no se pudo crear %s: %w", dir, err)
		}
	}
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
