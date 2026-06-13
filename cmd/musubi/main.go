package main

import (
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/mcp"
	"musubi/internal/memory"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: musubi <comando> [argumentos]")
		fmt.Println("Comandos disponibles: init, daemon")
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "init":
		initProject()
	case "daemon":
		runDaemon()
	default:
		fmt.Printf("Comando desconocido: %s\n", command)
		os.Exit(1)
	}
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

	// Arrancar servidor MCP sobre Stdin/Stdout
	server := mcp.NewMcpServer(engine, root, embedder)
	server.Start()
}
