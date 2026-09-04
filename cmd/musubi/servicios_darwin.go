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
	ahora := time.Now()
	salida, err := salidaDeComando("launchctl", "list")
	if err != nil {
		return nil, err
	}
	todo := parsearLaunchctl(salida, ahora)

	// A76 — los contenedores también, por el mismo motivo que en Windows: Docker Desktop corre en
	// macOS y expone el mismo `docker ps`. Acá no hay una medición que lo respalde —no hay ningún
	// Mac en la flota (A3)— así que se cablea por SIMETRÍA y no por evidencia, que es exactamente
	// lo que evita que esta plataforma sea la próxima que se queda afuera cuando aparezca un Mac.
	cont, err := enumerarContenedores(ahora)
	if err != nil {
		return nil, err
	}
	return append(todo, cont...), nil
}
