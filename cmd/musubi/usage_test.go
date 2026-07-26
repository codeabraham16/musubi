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
	printUsage()
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	help := string(out)

	for _, cmd := range []string{"ingest", "catalog harvest", "setup", "provision", "doctor", "ingest"} {
		if !strings.Contains(help, cmd) {
			t.Errorf("printUsage no documenta el comando %q", cmd)
		}
	}
}
