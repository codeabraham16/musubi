package main

import (
	"fmt"
	"os"
	"sync"
)

// style.go es un helper minúsculo de color ANSI para el CLI interactivo (menú,
// setup). Es seguro por defecto: solo emite secuencias de color cuando stdout es una
// TERMINAL real, el VT está habilitado y NO_COLOR no está seteada. En los hooks, el
// daemon y los pipes/redirecciones (donde stdout es el canal JSON-RPC o una captura)
// devuelve el texto tal cual, sin contaminar la salida.

var (
	colorOnce sync.Once
	colorOn   bool
)

// useColor decide una sola vez (memoizado) si conviene colorear. Se evalúa en tiempo
// de uso —después de los init de consola— así vtEnabled ya refleja si se pudo
// habilitar el VT en Windows.
func useColor() bool {
	colorOnce.Do(func() {
		if os.Getenv("NO_COLOR") != "" || !vtEnabled {
			return
		}
		fi, err := os.Stdout.Stat()
		colorOn = err == nil && fi.Mode()&os.ModeCharDevice != 0
	})
	return colorOn
}

// applyColor envuelve s en un código SGR si enabled; si no, devuelve s tal cual.
// Separado de colorize para poder testearlo sin depender del TTY.
func applyColor(enabled bool, code, s string) string {
	if !enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func colorize(code, s string) string { return applyColor(useColor(), code, s) }

func cGreen(s string) string  { return colorize("32", s) }   // ✓ y confirmaciones
func cCyan(s string) string   { return colorize("36", s) }   // títulos / headers
func cYellow(s string) string { return colorize("33", s) }   // advertencias
func cBold(s string) string   { return colorize("1", s) }    // énfasis
func cDim(s string) string    { return colorize("2;37", s) } // detalles secundarios

// printOK / printWarn / printInfo imprimen un paso del setup con su marca a color
// (verde ✓ / amarillo ! / gris ·). Centralizan el estilo de la salida interactiva.
func printOK(msg string)   { fmt.Println("  " + cGreen("✓") + " " + msg) }
func printWarn(msg string) { fmt.Println("  " + cYellow("!") + " " + msg) }
func printInfo(msg string) { fmt.Println("  " + cDim("·") + " " + msg) }
