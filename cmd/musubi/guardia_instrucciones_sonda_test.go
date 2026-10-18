package main

// guardia_instrucciones_sonda_test.go — LA SONDA de la guardia de instrucciones: mide los archivos
// REALES de esta máquina para cada raíz y muestra las dos líneas que saldrían.
//
//	MUSUBI_SONDA_RAICES='C:/Proyectos/Musubi;C:/Proyectos/Proyecto menir/altura-erp' \
//	  go test -count=1 -run TestSondaGuardiaDeInstrucciones -v ./cmd/musubi/
//
// SÓLO LEE: llama a medirInstrucciones y armarAvisoDeInstrucciones, que no abren la memoria ni
// escriben la marca del día. No falla por umbral: reporta, porque lo que mide depende de la PC.
//
// POR QUÉ NO VA DETRÁS DEL TAG `sonda`, como internal/mcp/sonda_diseno_test.go: aquélla sale a la
// red y necesita una credencial; ésta sólo lee archivos locales. Detrás del tag, CI no la
// compilaba nunca —su `go vet -tags sonda` cubre sólo internal/mcp— y la guarda
// TestNingunaPruebaQuedaFueraDeLaCompilacionSinDecirlo (internal/mcp/pruebas_que_no_corren_test.go)
// la acusó con razón: un cambio de firma en la guardia la habría roto sin que nadie se enterara.
// Sin el tag compila en cada `go test ./...`, y sin MUSUBI_SONDA_RAICES se saltea.

import (
	"os"
	"strings"
	"testing"
)

func TestSondaGuardiaDeInstrucciones(t *testing.T) {
	if strings.TrimSpace(os.Getenv("MUSUBI_SONDA_RAICES")) == "" {
		t.Skip("sonda: sin MUSUBI_SONDA_RAICES no hay raíces que medir")
	}
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
