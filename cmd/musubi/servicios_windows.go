//go:build windows

package main

// servicios_windows.go enumera los servicios de Windows por el SCM.
//
// UNA SOLA LLAMADA, igual que en Linux y por el mismo motivo: `Get-Service` devuelve todos de
// una. Se pide CSV porque los nombres para mostrar traen espacios, comas y acentos, y partir por
// espacios sobre esa salida es el error clásico que corta «SQL Server (MSSQLSERVER)» al medio.
//
// LO QUE **NO** SE PUEDE DAR ACÁ, Y SE DEJA EN nil EN VEZ DE INVENTARLO: el SCM no expone por
// esta vía ni el PID, ni cuántas veces se reinició, ni desde cuándo está en ese estado. Los tres
// campos son punteros justamente para esto — un 0 en «reinicios» se lee «nunca se reinició», que
// es una afirmación, y acá no sabemos nada.

import (
	"time"

	"musubi/internal/fleet"
)

func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	// SE PIDE `Win32_Service` Y NO `Get-Service`, POR UN CAMPO: `ExitCode`.
	//
	// `StartMode` se pide por el mismo motivo de siempre: quedarse con lo que alguien DECIDIÓ que
	// corra más lo que está roto. Un servicio Manual y detenido es ruido; Windows trae cientos.
	//
	// PERO `Automatic` NO SIGNIFICA «tiene que estar corriendo», y creer que sí costó dieciséis
	// alarmas falsas (A70). En systemd, `enabled` sí lo significa. En Windows un servicio
	// automático puede ser *delayed* o *trigger-start* y quedarse apagado hasta que algo lo
	// despierte: `sppsvc` corre cuando se valida la licencia y se apaga, `MapsBroker` sólo si
	// alguien abre Mapas, los updaters cuando toca actualizar. Medido en `gio` el 2026-09-02: 8
	// de 102 automáticos detenidos, ninguno roto.
	//
	// `ExitCode` es lo que separa las dos cosas CON UN DATO en vez de una heurística sobre el
	// tipo de arranque: cero es «terminó bien» —se apagó porque nadie lo necesitaba— y cualquier
	// otro (1067 murió, 1077 nunca arrancó desde el boot) es una falla de verdad. `Get-Service`
	// no expone ese campo, y por eso se cambia de fuente.
	const ps = `Get-CimInstance Win32_Service | Select-Object Name,State,StartMode,ExitCode | ConvertTo-Csv -NoTypeInformation`
	salida, err := salidaDeComando("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		return nil, err
	}
	return parsearServiciosWindows(salida, time.Now()), nil
}
