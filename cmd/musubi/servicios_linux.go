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

// enumerarServiciosDelSistema junta las dos fuentes. Cualquiera que ESTÉ y falle aborta el
// inventario entero, porque el cerebro poda por ausencia y media lista es una baja falsa — el
// porqué largo está en enumerarFuente, en servicios.go.
//
// Una sola marca de tiempo para las dos fuentes: son la misma foto, y dos `time.Now()` separados
// harían que servicios medidos en la misma corrida tengan `Tomada` distinto sin motivo.
func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	var todo []fleet.ReporteServicio
	ahora := time.Now()

	salida, hay, err := enumerarFuente("systemctl", append([]string{"show", "*.service", "--no-pager"},
		"--property="+strings.Join(propiedadesPedidas, ","))...)
	if err != nil {
		return nil, err
	}
	if hay {
		todo = append(todo, parsearSystemctlShow(salida, ahora)...)
	}

	// Los contenedores son la otra mitad de «qué corre acá», y en este servidor son 18. Se
	// prueban las dos herramientas porque una máquina puede tener cualquiera de las dos, y
	// tenerlas a las dos es normal. No tenerlas es normal también: eso es `hay == false`.
	for _, cli := range []string{"podman", "docker"} {
		s, hay, err := contenedoresDe(cli)
		if err != nil {
			return nil, err
		}
		if hay {
			todo = append(todo, parsearContenedores(s, cli, ahora)...)
		}
	}
	return todo, nil
}

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

// detalleDeContenedor guarda el `Status` legible («Up 2 weeks (healthy)»), que dice más que el
// estado: un contenedor `running` y `(unhealthy)` está corriendo y no está sano.
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

func detalleDeContenedor(campos []string) string {
	if len(campos) < 3 {
		return ""
	}
	return strings.TrimSpace(campos[2])
}
