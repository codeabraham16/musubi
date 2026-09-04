//go:build !linux && !windows && !darwin

package main

// El resto de los sistemas no tiene enumerador DE SERVICIOS DEL SISTEMA. Devuelve vacío SIN error:
// «no hay inventario» es distinto de «el inventario falló», y devolver un error acá haría que el
// agente avise de algo que no está roto. El cerebro ya distingue una lista vacía (no poda nada) de
// una con contenido.
//
// PERO LOS CONTENEDORES SÍ SE ENUMERAN, y esto lo destapó la guarda de A76 al exigirle la llamada
// a los CUATRO archivos y no a los tres que el cabo nombraba. Es gratis y es más correcto: `docker`
// y `podman` no son de un sistema operativo, y `contenedoresDe` ya trata «la herramienta no está»
// como `hay == false` en vez de como un error — que es exactamente el caso de una plataforma sin
// colector propio. Eximir este archivo habría dejado la misma trampa que A76 cerró, un escalón más
// abajo.

import (
	"time"

	"musubi/internal/fleet"
)

func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	return enumerarContenedores(time.Now())
}
