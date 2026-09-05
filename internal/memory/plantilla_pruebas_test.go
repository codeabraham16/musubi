package memory

// dirSembrado es el equivalente local de memtest.DirSembrado para las pruebas de ESTE paquete,
// que no pueden importar `memtest` sin cerrar un ciclo (memtest importa memory).
//
// Son cinco líneas y no una copia del mecanismo: el núcleo vive en plantilla.go y esto sólo le
// pone un *testing.T encima.

import (
	"testing"
)

func dirSembrado(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("no se pudo sembrar la plantilla del esquema: %v", err)
	}
	return dir
}
