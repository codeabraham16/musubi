package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/config"
	"musubi/internal/crudobackup"
	"musubi/internal/memory"
)

// runBackup crea un snapshot CONSISTENTE de la base (VACUUM INTO) en el directorio
// --out (o <workspace>/.musubi/backups por default) e imprime SOLO la ruta del snapshot
// en stdout, para que un script la capture con $(musubi backup ...). Es puro-Go: no
// requiere el CLI sqlite3 en el host, así que el timer de backup del cerebro central
// (deploy/musubi-backup.sh) puede generar el snapshot antes de shipearlo off-host sin
// dependencias extra en el servidor.
//
// Con --crudo NO respalda la base: respalda el CRUDO de Claude Code (los transcripts
// .jsonl del proyecto) off-host, vía internal/crudobackup. SIN --crudo el comando es
// bit-idéntico a antes de esa función.
func runBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "directorio destino (default: <workspace>/.musubi/backups)")
	crudo := fs.Bool("crudo", false, "respaldar el CRUDO de Claude Code (transcripts .jsonl) off-host, en vez del snapshot de la base")
	todos := fs.Bool("todos", false, "con --crudo: respaldar TODAS las carpetas de proyecto de la máquina, no sólo la del workspace actual")
	localOnly := fs.Bool("local-only", false, "con --crudo: armar y verificar el paquete en disco SIN enviarlo off-host (desactiva el fallo-cerrado)")
	_ = fs.Parse(args)

	root := workspaceDir()

	if *crudo {
		runBackupCrudo(root, *out, *todos, *localOnly)
		return
	}

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

// runBackupCrudo despacha al paquete crudobackup. Construye las Opciones desde los flags y el bloque
// crudo del .musubi/config.yaml, deja que crudobackup resuelva los defaults (raíz de transcripts,
// LocalDir), e imprime SOLO la ruta del paquete en stdout —igual que el modo base—; el progreso y el
// resumen van a stderr para no contaminar lo que captura un $(musubi backup --crudo).
func runBackupCrudo(root, out string, todos, localOnly bool) {
	cfg, err := config.Load(root)
	if err != nil {
		// Un config ilegible/mal-parseado aborta acá: proceder con defaults llevaría a un
		// fallo-cerrado con un mensaje que culparía a la falta de remote, no al YAML roto.
		fmt.Fprintf(os.Stderr, "Error al cargar la configuración: %v\n", err)
		os.Exit(1)
	}

	// Sólo scp esta tanda (design D6). Un method desconocido se rechaza en vez de caer en silencio al
	// scp y hacer creer que rsync/rclone funcionan.
	if m := cfg.Crudo.EffectiveMethod(); m != "scp" {
		fmt.Fprintf(os.Stderr, "Error: método de respaldo del crudo %q no soportado todavía (sólo 'scp')\n", m)
		os.Exit(1)
	}

	alcance := crudobackup.AlcanceProyecto
	if todos {
		alcance = crudobackup.AlcanceTodos
	}

	// Precedencia del directorio local: --out (explícito) > crudo.local_dir (config) > el default de
	// crudobackup (<workspace>/.musubi/backups, que es donde mira el doctor-check crudo_offhost).
	localDir := out
	if localDir == "" {
		localDir = cfg.Crudo.LocalDir
	}

	op := crudobackup.Opciones{
		Alcance:            alcance,
		Workspace:          root,
		LocalDir:           localDir,
		Remote:             cfg.Crudo.Remote,
		AllowLocalOnly:     localOnly || cfg.Crudo.AllowLocalOnly,
		RetentionDays:      cfg.Crudo.EffectiveRetentionDays(),
		WaitNetworkSeconds: cfg.Crudo.WaitNetworkSeconds,
		Log:                func(s string) { fmt.Fprintln(os.Stderr, s) },
	}

	res, err := crudobackup.ArmarYShipear(op)
	if err != nil {
		// Si el paquete local alcanzó a armarse, su ruta sigue siendo útil (quedó en disco): va a
		// stdout igual, y el error con su motivo a stderr.
		if res.Paquete != "" {
			fmt.Println(res.Paquete)
		}
		fmt.Fprintf(os.Stderr, "Error al respaldar el crudo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(res.Paquete)
	if res.Enviado {
		fmt.Fprintf(os.Stderr, "Respaldo del crudo OK: %d archivo(s), %d bytes, enviado a %s y confirmado por sha256.\n",
			res.Entradas, res.Bytes, res.Remote)
	} else {
		fmt.Fprintf(os.Stderr, "Respaldo del crudo armado y verificado SÓLO LOCAL (no enviado): %d archivo(s), %d bytes.\n",
			res.Entradas, res.Bytes)
	}
	if res.Purgados > 0 {
		fmt.Fprintf(os.Stderr, "Retención: %d paquete(s) viejo(s) purgado(s).\n", res.Purgados)
	}
}
