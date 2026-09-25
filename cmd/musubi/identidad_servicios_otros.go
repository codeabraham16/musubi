//go:build !unix

package main

// Fuera de Unix no hay uid al que bajar: resolverIdentidadDeServicios nunca devuelve Bajar=true
// acá (uid<0 con la variable puesta es un error), así que estas tres sólo existen para compilar.

import "syscall"

func atributosPara(identidadDeServicios) *syscall.SysProcAttr { return nil }

func esDeUid(string, uint32) bool { return false }

func duenoRealDeArchivo(string) (int, int, bool) { return -1, -1, false }
