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

func initProject() {
	fmt.Printf("Inicializando entorno de Musubi (%s/)...\n", config.DirName)
	if err := os.MkdirAll(config.DirName, 0755); err != nil {
		fmt.Printf("Error creando %s: %v\n", config.DirName, err)
		os.Exit(1)
	}

	configContent, err := config.Default().Marshal()
	if err != nil {
		fmt.Printf("Error generando config por defecto: %v\n", err)
		os.Exit(1)
	}
	configPath := filepath.Join(config.DirName, config.ConfigFile)
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		fmt.Printf("Error escribiendo %s: %v\n", configPath, err)
		os.Exit(1)
	}

	// Crear base de datos inicial vacía
	engine, err := memory.NewDbEngine(".")
	if err != nil {
		fmt.Printf("Error al inicializar la base de datos de memoria: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("Entorno Musubi inicializado correctamente. Listo para inyección MCP.")
}

func runDaemon() {
	// Verificar que el entorno ya está inicializado
	if _, err := os.Stat(config.DirName); os.IsNotExist(err) {
		fmt.Printf("Error: Musubi no está inicializado en este directorio. Ejecuta 'musubi init' primero.\n")
		os.Exit(1)
	}

	cfg, err := config.Load(".")
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
	engine, err := memory.NewDbEngine(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Arrancar servidor MCP sobre Stdin/Stdout
	server := mcp.NewMcpServer(engine, ".", embedder)
	server.Start()
}
