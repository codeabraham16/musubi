package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MEDICIÓN (no es un gate): exporta un arsenal REAL y muestra qué descripción le quedó a cada skill.
// Se saltea sin MUSUBI_EXPORT_ROOT, igual que el resto de las mediciones del repo.
//
// Sirve para mirar con ojos humanos lo único que decide si una skill se activa: su description.
func TestMedicionExportReal(t *testing.T) {
	root := os.Getenv("MUSUBI_EXPORT_ROOT")
	if root == "" {
		t.Skip("MUSUBI_EXPORT_ROOT no seteado: se saltea la medición del export real")
	}
	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		t.Fatalf("exportar: %v", err)
	}
	t.Logf("escritas=%d preservadas=%d retiradas=%d", len(rep.Escritas), len(rep.Preservadas), len(rep.Retiradas))

	dir := filepath.Join(root, ".claude", "skills")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("leer %s: %v", dir, err)
	}
	var conCuando, total int
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			t.Errorf("%s sin SKILL.md: %v", e.Name(), err)
			continue
		}
		total++
		var desc string
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "description:") {
				desc = strings.TrimSpace(strings.TrimPrefix(l, "description:"))
				break
			}
		}
		if strings.Contains(strings.ToLower(desc), "cuando") || strings.Contains(strings.ToLower(desc), "when") {
			conCuando++
		}
		if len(desc) > 120 {
			desc = desc[:120] + "…"
		}
		t.Logf("  %-24s %s", e.Name(), desc)
	}
	t.Logf("con «cuándo» en la descripción: %d de %d", conCuando, total)
}
