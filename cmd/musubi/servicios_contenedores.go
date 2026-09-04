package main

// servicios_contenedores.go es la mitad del inventario que NO depende del sistema operativo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ SALIÓ DE servicios_linux.go
//
// Estaba detrás de `//go:build linux`, y no le correspondía: `docker` y `podman` corren en las
// tres plataformas. Lo único de systemd es `systemctl`. Con el tag puesto, la prueba de la
// degradación de formato —que no toca el sistema, porque sustituye `ejecutarParaEnumerar`— sólo
// se podía compilar en Linux, y `go vet` no podía ni armar el paquete en Windows ni en macOS.
//
// OJO CON LO QUE ESTO NO ARREGLA: hoy sólo el enumerador de Linux llama a contenedoresDe. Un
// Docker Desktop en Windows o en macOS existe y no se reporta. Mover el código no lo cablea —
// eso es una decisión aparte, y no se toma acá.

import (
	"strconv"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// formatoContenedores son los campos que se le piden a cada runtime, del más rico al más pobre.
//
// EL PRIMERO LLEVA `{{.Restarts}}` Y EL SEGUNDO NO, Y ESO NO ES REDUNDANCIA.
//
// Sin los reinicios, `ServicioReiniciandose` no puede disparar sobre un contenedor — y un
// contenedor con `restart: always` en bucle de caída es EXACTAMENTE el caso para el que existe
// esa alerta. Medido en producción: 54 servicios con serie `up` y sólo 36 con serie de reinicios;
// los 18 que faltaban eran los contenedores, y la alerta estaba ciega justo para ellos.
//
// Pero `.Restarts` no lo entiende todo el mundo: `docker ps` no tiene ese campo, y un podman
// viejo tampoco. Con la regla nueva —una fuente que está y falla ABORTA el inventario— pedirlo a
// secas convertiría «este docker no conoce ese campo» en «esta máquina no reporta nada». Por eso
// se degrada: se intenta el formato rico, y si el runtime lo rechaza se cae al pobre. Recién si
// los DOS fallan es una falla de verdad.
var formatoContenedores = []string{
	"{{.Names}}\t{{.State}}\t{{.Status}}\t{{.Restarts}}",
	"{{.Names}}\t{{.State}}\t{{.Status}}",
}

// contenedoresDe consulta un runtime, degradando el formato si hace falta.
func contenedoresDe(cli string) (salida string, hayFuente bool, err error) {
	for _, formato := range formatoContenedores {
		s, hay, e := enumerarFuente(cli, "ps", "--all", "--format", formato)
		if !hay {
			// La herramienta no está: no tiene sentido probar el otro formato.
			return "", false, nil
		}
		if e == nil {
			return s, true, nil
		}
		err = e
	}
	// Está y ningún formato anduvo. Se devuelve el error del ÚLTIMO intento —el del formato más
	// pobre— porque es el que dice algo sobre el runtime y no sobre un campo que no conoce.
	return "", true, err
}

// parsearContenedores lee `<cli> ps --all` con formato de tabulaciones.
func parsearContenedores(salida, cli string, ahora time.Time) []fleet.ReporteServicio {
	var rs []fleet.ReporteServicio
	for _, l := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n") {
		campos := strings.Split(strings.TrimSpace(l), "\t")
		if len(campos) < 2 || campos[0] == "" {
			continue
		}
		salud := fleet.SaludServicio{
			Tomada:  ahora,
			Estado:  estadoDeContenedor(campos[1]),
			Detalle: detalleDeContenedor(campos),
		}
		// El cuarto campo es opcional: viene sólo si el runtime entendió `{{.Restarts}}`. Un
		// campo ausente deja Reinicios en nil —«esta plataforma no lo sabe»— y no en 0, que
		// significaría «no se reinició nunca». La distinción no es teórica: 0 apaga la alerta
		// con confianza y nil la deja sin serie, que es lo que un hueco tiene que verse.
		if n, ok := reiniciosDeContenedor(campos); ok {
			salud.Reinicios = &n
		}
		rs = append(rs, fleet.ReporteServicio{Nombre: campos[0], Clase: cli, Salud: salud})
	}
	return rs
}

func estadoDeContenedor(estado string) fleet.EstadoServicio {
	switch strings.ToLower(strings.TrimSpace(estado)) {
	case "running":
		return fleet.EstadoCorriendo
	case "exited", "stopped", "created", "configured":
		return fleet.EstadoDetenido
	case "dead", "removing":
		return fleet.EstadoFallado
	default:
		return fleet.EstadoDesconocido
	}
}

// reiniciosDeContenedor lee el cuarto campo. Devuelve ok=false si no vino o no es un número:
// un runtime que no conoce `{{.Restarts}}` puede imprimir el literal `<no value>`, y parsearlo
// como 0 sería inventar «no se reinició nunca».
func reiniciosDeContenedor(campos []string) (int, bool) {
	if len(campos) < 4 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(campos[3]))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// detalleDeContenedor guarda el `Status` legible («Up 2 weeks (healthy)»), que dice más que el
// estado: un contenedor `running` y `(unhealthy)` está corriendo y no está sano.
func detalleDeContenedor(campos []string) string {
	if len(campos) < 3 {
		return ""
	}
	return strings.TrimSpace(campos[2])
}
