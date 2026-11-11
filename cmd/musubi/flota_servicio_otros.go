//go:build !windows

package main

// En cualquier sistema que no sea Windows no hay Administrador de Servicios: el agente de flota se
// instala por el mecanismo nativo del OS (systemd en Linux, launchd en macOS), no por este
// subcomando. Estos stubs mantienen el build cross-platform y dicen la verdad en vez de fingir.

import (
	"fmt"
	"os"
	"runtime"
)

// arranqueComoServicioFlota nunca toma el control fuera de Windows: no hay SCM que detectar, así que
// el agente siempre sigue su camino normal.
func arranqueComoServicioFlota(_ []string) bool { return false }

// runFlota existe en todos los OS porque el dispatch de main es cross-platform, pero fuera de
// Windows no tiene qué gestionar: lo dice y sale con error, en vez de fingir un servicio que no hay.
func runFlota(_ []string) {
	fmt.Fprintf(os.Stderr, "`musubi flota` gestiona el servicio de Windows del agente; en %s no aplica.\n", runtime.GOOS)
	fmt.Fprintln(os.Stderr, "  En Linux el agente corre por systemd (ver deploy/); en macOS, por launchd.")
	os.Exit(1)
}
