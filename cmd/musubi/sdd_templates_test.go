package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

func TestWriteSddTemplatesEscribeLosCuatro(t *testing.T) {
	root := t.TempDir()
	if err := writeSddTemplates(root); err != nil {
		t.Fatalf("writeSddTemplates error: %v", err)
	}
	dir := filepath.Join(root, config.DirName, sddTemplatesSubdir)
	for _, name := range []string{"proposal.md", "spec.md", "design.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("esperaba %s creado: %v", name, err)
		}
		s := string(data)
		if !strings.Contains(s, "schema_version") {
			t.Errorf("%s debe tener schema_version en el frontmatter", name)
		}
	}
}

func TestWriteSddTemplatesNoSobrescribe(t *testing.T) {
	root := t.TempDir()
	if err := writeSddTemplates(root); err != nil {
		t.Fatalf("primera escritura: %v", err)
	}
	proposal := filepath.Join(root, config.DirName, sddTemplatesSubdir, "proposal.md")
	if err := os.WriteFile(proposal, []byte("EDITADO POR EL USUARIO"), 0644); err != nil {
		t.Fatalf("preparando edición: %v", err)
	}
	if err := writeSddTemplates(root); err != nil {
		t.Fatalf("segunda escritura: %v", err)
	}
	data, _ := os.ReadFile(proposal)
	if string(data) != "EDITADO POR EL USUARIO" {
		t.Error("writeSddTemplates sobrescribió un template ya editado por el usuario")
	}
}
