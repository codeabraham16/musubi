package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"musubi/internal/selfupdate"
)

// Repositorio público de releases de Musubi.
const (
	repoOwner = "codeabraham16"
	repoName  = "musubi"
)

// metaLastUpdateCheck es la clave de throttle del chequeo de versión al arrancar.
const metaLastUpdateCheck = "last_update_check"

// notifyIfOutdated consulta el último release y, si hay una versión nueva, avisa
// por stderr (nunca stdout). Best-effort y silencioso ante errores; pensado para
// correr en una goroutine al arrancar el daemon (no bloquea ni descarga nada).
func notifyIfOutdated() {
	u := selfupdate.New(repoOwner, repoName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latest, err := u.LatestVersion(ctx)
	if err != nil {
		return
	}
	if selfupdate.NeedsUpdate(version, latest) {
		fmt.Fprintf(os.Stderr, "musubi: hay una versión nueva (%s; tenés %s). Actualizá con: musubi update\n", latest, version)
	}
}

// runUpdate descarga el último release, verifica su SHA-256 y reemplaza el
// binario en ejecución. Es un proceso one-shot (stdout para el reporte al usuario).
func runUpdate() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo resolver el ejecutable: %v\n", err)
		os.Exit(1)
	}

	asset, err := selfupdate.AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	u := selfupdate.New(repoOwner, repoName)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	latest, err := u.LatestVersion(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo consultar el último release: %v\n", err)
		os.Exit(1)
	}

	if selfupdate.NormalizeVersion(latest) == selfupdate.NormalizeVersion(version) {
		fmt.Printf("Ya estás en la última versión (%s).\n", version)
		return
	}

	fmt.Printf("Actualizando de %s a %s ...\n", version, latest)
	bin, err := u.Download(ctx, latest, asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al descargar %s: %v\n", asset, err)
		os.Exit(1)
	}
	sha, err := u.Download(ctx, latest, asset+".sha256")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al descargar el checksum: %v\n", err)
		os.Exit(1)
	}
	if err := selfupdate.VerifyChecksum(bin, string(sha)); err != nil {
		fmt.Fprintf(os.Stderr, "Verificación de integridad fallida: %v\n", err)
		os.Exit(1)
	}
	if err := selfupdate.Apply(exe, bin); err != nil {
		fmt.Fprintf(os.Stderr, "Error al reemplazar el binario: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Listo: actualizado a %s en %s.\n", latest, exe)
	fmt.Println("Reiniciá la sesión (o el daemon) para usar la nueva versión.")
}
