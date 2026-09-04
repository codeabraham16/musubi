//go:build linux

package main

// servicios_linux.go enumera lo que corre en un Linux: units de systemd y contenedores.
//
// ACÁ VIVE SÓLO EL LADO QUE TOCA EL SISTEMA: qué se ejecuta y cómo se degrada si el runtime no
// entiende un campo. Convertir esas salidas en reportes está en servicios_parsers.go, sin build
// tag, para que las tres plataformas se puedan probar desde cualquier máquina.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// UNA SOLA LLAMADA, NO UNA POR UNIT
//
// La forma obvia es listar las units y después pedirle a cada una sus propiedades. En este
// servidor eso son 70 `exec()` por latido, cada 10 segundos. `systemctl show '*.service'` acepta
// un patrón y devuelve TODAS las propiedades de TODAS las units en una sola salida, con los
// bloques separados por una línea en blanco. Una llamada.

import (
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
