package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"musubi/internal/config"
)

// sddTemplatesFS contiene los templates de artefactos SDD (proposal/spec/design/tasks)
// embebidos en el binario. Son el scaffold versionado que `musubi setup` deja en el
// proyecto; NO duplican el orquestador SDD ni la memoria (engram), solo dan forma
// consistente a los artefactos.
//
//go:embed assets/sdd/*.md
var sddTemplatesFS embed.FS

// sddTemplatesSubdir es la ruta (relativa a .musubi/) donde se escriben los templates.
const sddTemplatesSubdir = "templates/sdd"

// writeSddTemplates escribe los templates SDD embebidos en
// {root}/.musubi/templates/sdd/. No sobrescribe un template ya editado por el usuario
// (idempotente y respetuoso, igual que el bundle cognitivo).
func writeSddTemplates(root string) error {
	dir := filepath.Join(root, config.DirName, sddTemplatesSubdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", dir, err)
	}

	entries, err := fs.ReadDir(sddTemplatesFS, "assets/sdd")
	if err != nil {
		return fmt.Errorf("no se pudieron leer los templates embebidos: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dest := filepath.Join(dir, e.Name())
		if _, statErr := os.Stat(dest); statErr == nil {
			continue // no sobrescribir lo que el usuario ya tocó
		}
		data, rerr := sddTemplatesFS.ReadFile("assets/sdd/" + e.Name())
		if rerr != nil {
			return fmt.Errorf("no se pudo leer el template %s: %w", e.Name(), rerr)
		}
		if werr := os.WriteFile(dest, data, 0644); werr != nil {
			return fmt.Errorf("no se pudo escribir %s: %w", dest, werr)
		}
	}
	return nil
}
