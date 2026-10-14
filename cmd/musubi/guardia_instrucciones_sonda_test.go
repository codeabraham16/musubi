//go:build sonda

package main

// guardia_instrucciones_sonda_test.go — LA SONDA de la guardia de instrucciones: mide los archivos
// REALES de esta máquina para cada raíz y muestra las dos líneas que saldrían.
//
//	MUSUBI_SONDA_RAICES='C:/Proyectos/Musubi;C:/Proyectos/Proyecto menir/altura-erp' \
//	  go test -tags sonda -count=1 -run TestSondaGuardiaDeInstrucciones -v ./cmd/musubi/
//
// SÓLO LEE: llama a medirInstrucciones y armarAvisoDeInstrucciones, que no abren la memoria ni
// escriben la marca del día. Va detrás del tag `sonda` porque depende de la PC que la corre, y no
// falla por umbral: reporta.

import (
	"os"
	"strings"
	"testing"
)

func TestSondaGuardiaDeInstrucciones(t *testing.T) {
	raices := strings.Split(os.Getenv("MUSUBI_SONDA_RAICES"), ";")
	dirConfig, _ := dirConfigDeClaude()
	home, _ := os.UserHomeDir()
	for _, root := range raices {
		if root = strings.TrimSpace(root); root == "" {
			continue
		}
		t.Logf("== %s", root)
		medidas := medirInstrucciones(root, dirConfig, home)
		for _, m := range medidas {
			marca := "  "
			if m.pasaElTope() {
				marca = "!!"
			}
			var disco int64
			if fi, err := os.Stat(m.ruta); err == nil {
				disco = fi.Size()
			}
			t.Logf("%s %-58s %4d líneas %6d unidades (bytes en disco %6d, importados %d)",
				marca, m.etiqueta, m.lineas, m.tamano, disco, m.importados)
		}
		aviso := armarAvisoDeInstrucciones(medidas)
		t.Logf("systemMessage:     %s", aviso.dueno)
		t.Logf("additionalContext: %s", aviso.agente)
	}
}
