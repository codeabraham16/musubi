//go:build !windows

package main

import (
	"os"
	"syscall"
)

// procesoVivo en Unix: FindProcess SIEMPRE devuelve un proceso, exista o no, así que por sí solo
// diría que todo está vivo. La señal 0 es la que pregunta de verdad — no entrega nada, sólo hace
// los chequeos de existencia y permisos.
func procesoVivo(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
