//go:build !linux && !windows && !darwin

package main

// El resto de los sistemas no tiene enumerador. Devuelve vacío SIN error: «no hay inventario» es
// distinto de «el inventario falló», y devolver un error acá haría que el agente avise de algo
// que no está roto. El cerebro ya distingue una lista vacía (no poda nada) de una con contenido.

import "musubi/internal/fleet"

func enumerarServiciosDelSistema() ([]fleet.ReporteServicio, error) {
	return nil, nil
}
