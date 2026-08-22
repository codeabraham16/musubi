//go:build windows

package main

import "os"

// procesoVivo en Windows: OpenProcess falla si el PID no existe, así que FindProcess ya contesta.
// La señal 0 —que es lo idiomático en Unix— NO sirve acá: Windows no tiene señales y Signal
// devolvería error incluso para un proceso vivo, o sea que podaría archivos de daemons activos.
func procesoVivo(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}
