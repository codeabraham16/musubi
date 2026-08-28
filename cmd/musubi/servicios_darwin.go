//go:build darwin

package main

// servicios_darwin.go enumera por launchd.
//
// `launchctl list` devuelve tres columnas: PID, último código de salida, y etiqueta. Un `-` en el
// PID significa «cargado y no corriendo», y el código de salida es lo único que hay para saber si
// murió mal. No hay contador de reinicios ni marca de tiempo: quedan en nil.
//
// SE FILTRA POR PREFIJO. launchd carga cientos de agentes del sistema (`com.apple.*`); reportar
// todos llenaría el techo de 64 con cosas de Apple y dejaría afuera lo que el operador instaló.

import (
	"time"

	"musubi/internal/fleet"
)

func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	salida, err := salidaDeComando("launchctl", "list")
	if err != nil {
		return nil, err
	}
	return parsearLaunchctl(salida, time.Now()), nil
}
