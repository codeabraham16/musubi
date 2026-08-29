//go:build !linux && !windows && !darwin

package main

// El avisador en un sistema para el que no hay implementación. Declara que NO puede, con el
// motivo dicho: es la misma honestidad que el colector de telemetría, que en un OS sin soporte
// late igual y lo dice en vez de mandar una muestra de ceros.

import (
	"context"
	"runtime"

	"musubi/internal/fleet"
)

func detectarAvisador() fleet.CapacidadDeAvisar {
	return fleet.CapacidadDeAvisar{Motivo: "no hay avisador implementado para " + runtime.GOOS}
}

func correrAvisador(context.Context, fleet.CapacidadDeAvisar, string) error {
	return errNoSePuedeAvisar{motivo: "sin avisador para " + runtime.GOOS}
}

func correrPregunta(context.Context, fleet.CapacidadDeAvisar, string) fleet.RespuestaAviso {
	return fleet.RespuestaNoSePudo
}
