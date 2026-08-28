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
	// StartType se pide para quedarse con lo que alguien DECIDIÓ que corra (Automatic) más lo
	// que está roto, igual que el criterio de systemd. Un servicio Manual y detenido es ruido:
	// Windows trae cientos.
	const ps = `Get-Service | Select-Object Name,Status,StartType | ConvertTo-Csv -NoTypeInformation`
	salida, err := salidaDeComando("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		return nil, err
	}
	return parsearGetService(salida, time.Now()), nil
}
