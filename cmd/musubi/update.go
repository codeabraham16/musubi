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
	// ── PROCEDENCIA ANTES QUE INTEGRIDAD (Ola 2) ────────────────────────────────────────────
	//
	// El `.sha256` lo publica quien publica el binario, así que verificarlo sólo dice que el
	// archivo llegó entero — no que lo hayamos hecho nosotros. El manifiesto firmado sí: la clave
	// privada no vive en el CI ni en el repo, y la pública va embebida acá.
	//
	// Primero la firma y DESPUÉS el hash: el hash es la integridad de un dato cuya procedencia
	// recién establece la firma. Al revés, el segundo paso no agrega nada al primero.
	manifiesto, err := u.Download(ctx, latest, nombreDelManifiesto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al descargar el manifiesto del release: %v\n", err)
		fmt.Fprintln(os.Stderr, "Un release sin manifiesto firmado no se instala: no hay forma de saber si lo publicamos nosotros.")
		os.Exit(1)
	}
	firma, err := u.Download(ctx, latest, nombreDelManifiesto+".sig")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al descargar la firma del manifiesto: %v\n", err)
		os.Exit(1)
	}
	if err := selfupdate.VerificarFirma(clavePublicaDeRelease(), manifiesto, string(firma)); err != nil {
		fmt.Fprintf(os.Stderr, "FIRMA INVÁLIDA: %v\n", err)
		os.Exit(1)
	}
	man, err := selfupdate.ParsearManifiesto(manifiesto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	// EL HASH SALE DEL MANIFIESTO FIRMADO, no de un `.sha256` suelto del release. Bajar el hash
	// por separado sería volver a confiar en quien publica.
	sha, err := man.ShaDeAsset(asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := selfupdate.VerifyChecksum(bin, sha); err != nil {
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
