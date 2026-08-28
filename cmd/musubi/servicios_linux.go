//go:build linux

package main

// servicios_linux.go enumera lo que corre en un Linux: units de systemd y contenedores.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// UNA SOLA LLAMADA, NO UNA POR UNIT
//
// La forma obvia es listar las units y después pedirle a cada una sus propiedades. En este
// servidor eso son 70 `exec()` por latido, cada 10 segundos. `systemctl show '*.service'` acepta
// un patrón y devuelve TODAS las propiedades de TODAS las units en una sola salida, con los
// bloques separados por una línea en blanco. Una llamada.

import (
	"strconv"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// propiedadesPedidas es lo que se le pide a systemd. El orden en que las devuelve NO está
// garantizado, así que el parser trabaja con un mapa por bloque y nunca por posición.
var propiedadesPedidas = []string{
	"Id", "ActiveState", "SubState", "MainPID", "NRestarts",
	"ActiveEnterTimestamp", "Result", "UnitFileState",
}

func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	var todo []fleet.ReporteServicio

	salida, err := salidaDeComando("systemctl", append([]string{"show", "*.service", "--no-pager"},
		"--property="+strings.Join(propiedadesPedidas, ","))...)
	avisarDeEnumeracionParcial("systemd", err)
	if err == nil {
		todo = append(todo, parsearSystemctlShow(salida, time.Now())...)
	}

	// Los contenedores son la otra mitad de «qué corre acá», y en este servidor son 18. Se
	// prueban las dos herramientas porque una máquina puede tener cualquiera de las dos, y
	// tenerlas a las dos es normal.
	for _, cli := range []string{"podman", "docker"} {
		s, err := salidaDeComando(cli, "ps", "--all", "--format",
			"{{.Names}}\t{{.State}}\t{{.Status}}")
		if err != nil {
			// No se avisa por cada uno: no tener docker instalado es lo normal, no una falla.
			continue
		}
		todo = append(todo, parsearContenedores(s, cli, time.Now())...)
	}
	return todo, nil
}

// parsearSystemctlShow convierte la salida de bloques en reportes.
//
// SE QUEDA CON LO QUE ALGUIEN DECIDIÓ QUE CORRA, MÁS LO ROTO. Una unit deshabilitada e inactiva
// es ruido —hay cientos—; una habilitada y detenida es exactamente la fila que uno quiere ver.
func parsearSystemctlShow(salida string, ahora time.Time) []fleet.ReporteServicio {
	var rs []fleet.ReporteServicio
	for _, bloque := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n\n") {
		p := map[string]string{}
		for _, l := range strings.Split(bloque, "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok {
				p[k] = v
			}
		}
		id := p["Id"]
		if id == "" {
			continue
		}
		habilitada := strings.HasPrefix(p["UnitFileState"], "enabled")
		fallada := p["ActiveState"] == "failed"
		if !habilitada && !fallada {
			continue
		}
		salud := fleet.SaludServicio{
			Tomada:  ahora,
			Estado:  estadoDeSystemd(p["ActiveState"], p["SubState"]),
			Detalle: detalleDeSystemd(p),
		}
		if pid, err := strconv.Atoi(p["MainPID"]); err == nil && pid > 0 {
			salud.PID = &pid
		}
		if n, err := strconv.Atoi(p["NRestarts"]); err == nil {
			salud.Reinicios = &n
		}
		if t := fechaDeSystemd(p["ActiveEnterTimestamp"]); t != nil {
			salud.Desde = t
		}
		rs = append(rs, fleet.ReporteServicio{
			Nombre: strings.TrimSuffix(id, ".service"),
			Clase:  "systemd",
			Salud:  salud,
		})
	}
	return rs
}

// estadoDeSystemd traduce el par (ActiveState, SubState) al vocabulario del dominio.
//
// `activating` y `deactivating` NO son «corriendo»: son estados de transición, y llamarlos
// corriendo haría que un servicio que se reinicia en loop se vea sano en el panel.
func estadoDeSystemd(activo, sub string) fleet.EstadoServicio {
	switch activo {
	case "active":
		if sub == "running" || sub == "exited" {
			return fleet.EstadoCorriendo
		}
		return fleet.EstadoDesconocido
	case "failed":
		return fleet.EstadoFallado
	case "inactive":
		return fleet.EstadoDetenido
	case "activating", "deactivating", "reloading":
		return fleet.EstadoDesconocido
	default:
		return fleet.EstadoDesconocido
	}
}

// detalleDeSystemd arma el texto corto que ve el operador. `Result` es lo que dice POR QUÉ murió
// (`exit-code`, `signal`, `oom-kill`), que es la mitad del diagnóstico.
func detalleDeSystemd(p map[string]string) string {
	res := p["Result"]
	if res == "" || res == "success" {
		return ""
	}
	return "result=" + res
}

// fechaDeSystemd parsea el formato de systemd. Devuelve nil si no se entiende o si viene vacío:
// systemd manda `ActiveEnterTimestamp=` a secas para lo que nunca arrancó, y eso NO es la época
// Unix — una fecha inventada es peor que ninguna.
func fechaDeSystemd(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "n/a" {
		return nil
	}
	for _, f := range []string{"Mon 2006-01-02 15:04:05 MST", "Mon 2006-01-02 15:04:05 -0700"} {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}

// parsearContenedores lee `<cli> ps --all` con formato de tabulaciones.
func parsearContenedores(salida, cli string, ahora time.Time) []fleet.ReporteServicio {
	var rs []fleet.ReporteServicio
	for _, l := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n") {
		campos := strings.Split(strings.TrimSpace(l), "\t")
		if len(campos) < 2 || campos[0] == "" {
			continue
		}
		rs = append(rs, fleet.ReporteServicio{
			Nombre: campos[0],
			Clase:  cli,
			Salud: fleet.SaludServicio{
				Tomada:  ahora,
				Estado:  estadoDeContenedor(campos[1]),
				Detalle: detalleDeContenedor(campos),
			},
		})
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

// detalleDeContenedor guarda el `Status` legible («Up 2 weeks (healthy)»), que dice más que el
// estado: un contenedor `running` y `(unhealthy)` está corriendo y no está sano.
func detalleDeContenedor(campos []string) string {
	if len(campos) < 3 {
		return ""
	}
	return strings.TrimSpace(campos[2])
}
