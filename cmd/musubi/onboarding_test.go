package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLooksLikeProject verifica la heurística que distingue una raíz de proyecto
// (tiene un manifiesto/repo conocido) de una carpeta cualquiera (ej. Descargas).
func TestLooksLikeProject(t *testing.T) {
	empty := t.TempDir()
	if looksLikeProject(empty) {
		t.Errorf("una carpeta vacía no debería parecer un proyecto")
	}

	withGoMod := t.TempDir()
	if err := os.WriteFile(filepath.Join(withGoMod, "go.mod"), []byte("module x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !looksLikeProject(withGoMod) {
		t.Errorf("una carpeta con go.mod debería parecer un proyecto")
	}

	withGit := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withGit, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !looksLikeProject(withGit) {
		t.Errorf("una carpeta con .git debería parecer un proyecto")
	}
}

// TestIsYes cubre las respuestas afirmativas y negativas de la confirmación.
func TestIsYes(t *testing.T) {
	for _, in := range []string{"s", "S", "si", "Si", "sí", "y", "YES", " yes \n"} {
		if !isYes(in) {
			t.Errorf("isYes(%q) debería ser true", in)
		}
	}
	for _, in := range []string{"", "n", "no", "x", "nope", "\n"} {
		if isYes(in) {
			t.Errorf("isYes(%q) debería ser false", in)
		}
	}
}
