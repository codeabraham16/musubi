package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// REGRESIÓN (auditoría 2026-07-26, F5): `musubi ingest` y `musubi catalog harvest` estaban cableados
// en el dispatch pero NO aparecían en printUsage ⇒ nadie los descubría. Este test fija que la ayuda
// mencione todos los comandos "de usuario" para que un comando nuevo no vuelva a quedar invisible.
func TestUsageDocumentsUserCommands(t *testing.T) {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	// La lectura corre EN PARALELO con la escritura. Leer recién después de cerrar colgaba la prueba
	// en cuanto la ayuda pasaba el buffer del pipe: en el runner de Windows, 4.132 bytes pasaban y
	// 4.158 dejaron la escritura bloqueada 22 minutos, hasta el timeout de 25 (CI de #697).
	leido := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		leido <- b
	}()
	printUsage()
	_ = w.Close()
	os.Stdout = orig
	help := string(<-leido)

	// `shell` se agrega acá el mismo día que se cablea: es la razón de ser de esta prueba —
	// `ingest` y `catalog harvest` vivieron cableados e invisibles hasta que alguien los buscó.
	for _, cmd := range []string{"ingest", "catalog harvest", "setup", "provision", "doctor", "ingest", "agent", "shell <maquina>", "uso-agente"} {
		if !strings.Contains(help, cmd) {
			t.Errorf("printUsage no documenta el comando %q", cmd)
		}
	}
}
