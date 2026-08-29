// Package memtest le da a las pruebas de OTROS paquetes una base ya migrada (A45).
//
// Es un envoltorio fino sobre memory.SembrarPlantillaDePruebas, que es donde está el porqué
// completo y la medición. Existe aparte porque las pruebas de `internal/memory` no pueden
// importarlo —sería un ciclo— y las de `internal/mcp` y `cmd/musubi` sí.
package memtest

import (
	"testing"

	"musubi/internal/memory"
)

// DirSembrado devuelve un directorio temporal que YA tiene una base migrada adentro.
//
// Es el reemplazo directo de `t.TempDir()` en `NewDbEngine(t.TempDir())`: la base que esa llamada
// va a abrir ya está al día, así que no aplica una sola migración.
func DirSembrado(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := memory.SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("memtest: no se pudo sembrar la plantilla del esquema: %v", err)
	}
	return dir
}

// Sembrar deja la plantilla en un directorio que la prueba ya armó (con sus archivos de config,
// sus skills, lo que sea). Es DirSembrado para cuando el directorio no lo crea uno.
func Sembrar(t *testing.T, projectPath string) {
	t.Helper()
	if err := memory.SembrarPlantillaDePruebas(projectPath); err != nil {
		t.Fatalf("memtest: no se pudo sembrar la plantilla del esquema: %v", err)
	}
}

// NuevoEngine abre un engine sobre un directorio ya sembrado y lo cierra al terminar la prueba.
func NuevoEngine(t *testing.T, projectPath string) *memory.DbEngine {
	t.Helper()
	Sembrar(t, projectPath)
	eng, err := memory.NewDbEngine(projectPath)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	return eng
}
