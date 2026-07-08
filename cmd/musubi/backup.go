package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/memory"
)

// runBackup crea un snapshot CONSISTENTE de la base (VACUUM INTO) en el directorio
// --out (o <workspace>/.musubi/backups por default) e imprime SOLO la ruta del snapshot
// en stdout, para que un script la capture con $(musubi backup ...). Es puro-Go: no
// requiere el CLI sqlite3 en el host, así que el timer de backup del cerebro central
// (deploy/musubi-backup.sh) puede generar el snapshot antes de shipearlo off-host sin
// dependencias extra en el servidor.
func runBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "directorio destino del snapshot (default: <workspace>/.musubi/backups)")
	_ = fs.Parse(args)

	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	destDir := *out
	if destDir == "" {
		destDir = filepath.Join(root, ".musubi", "backups")
	}
	path, err := engine.BackupTo(destDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al respaldar: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)
}
