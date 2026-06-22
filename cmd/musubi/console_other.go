//go:build !windows

package main

// console_other.go: en Unix/macOS las terminales soportan UTF-8 y secuencias ANSI
// nativamente, así que no hace falta inicializar nada. vtEnabled queda en true y el
// helper de estilo decide por TTY/NO_COLOR como en cualquier CLI POSIX.
const vtEnabled = true
