package codeintel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REGRESIÓN (auditoría 2026-07-26, #7): GitRunner.Diff agregaba el `ref` del cliente crudo a la línea
// de git. Un ref con pinta de opción ("--output=/ruta") git lo trataba como FLAG => escritura/truncado
// de un archivo arbitrario. Ahora se rechaza antes de exec y NO se crea nada.
func TestDiffRejectsOptionLikeRef(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "pwned.txt")
	g := GitRunner{Dir: dir}

	_, err := g.Diff("--output="+target, false)
	if err == nil {
		t.Fatal("un ref que empieza con '-' debe ser rechazado")
	}
	if !strings.Contains(err.Error(), "ref inválido") {
		t.Fatalf("esperaba error de validación de ref, obtuve: %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("el fix falló: git escribió el archivo apuntado por --output")
	}
}
