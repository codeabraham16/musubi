package skills

import (
	"musubi/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchGlobNormalizesSeparators(t *testing.T) {
	// En Windows WalkDir entrega paths con '\'; un trigger estilo ruta con '/' debe
	// matchear igual. Normalizamos separadores de forma determinista (cross-OS).
	if !MatchGlob("cmd/*", `cmd\main.go`) {
		t.Error(`cmd/* debe matchear cmd\main.go (normalización de separadores)`)
	}
	if !MatchGlob(`cmd\*`, "cmd/main.go") {
		t.Error(`cmd\* debe matchear cmd/main.go (normalización de separadores)`)
	}
	if MatchGlob("cmd/*", "internal/main.go") {
		t.Error("cmd/* NO debe matchear internal/main.go")
	}
}

// setupSkillsDir crea un proyecto temporal con .musubi/skills y escribe los archivos dados.
// files es un mapa nombre->contenido.
func setupSkillsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s error: %v", name, err)
		}
	}
	return root
}

func TestLoadSkillsMissingDirReturnsEmptyNotNil(t *testing.T) {
	r := NewResolver(t.TempDir()) // sin .musubi/skills
	skills, err := r.LoadSkills()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if skills == nil {
		t.Fatal("esperaba slice vacío no-nil")
	}
	if len(skills) != 0 {
		t.Fatalf("esperaba 0 skills, obtuve %d", len(skills))
	}
}

func TestLoadSkillsParsesValidAndSkipsInvalid(t *testing.T) {
	root := setupSkillsDir(t, map[string]string{
		"go.yaml": "name: go-rules\ndescription: reglas go\ntriggers:\n  - \"*.go\"\ncapabilities:\n  - go\nrules: |\n  - usar SOLID\n",
		"broken.yaml": "name: [esto no es: yaml válido\n  - roto",
		"notes.txt":   "esto no es un skill",
	})

	r := NewResolver(root)
	skills, err := r.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("esperaba 1 skill válido (broken e ignorado .txt fuera), obtuve %d", len(skills))
	}
	if skills[0].Name != "go-rules" {
		t.Errorf("esperaba go-rules, obtuve %q", skills[0].Name)
	}
}

func TestMatchTriggers(t *testing.T) {
	r := NewResolver(t.TempDir())
	skill := Skill{Triggers: []string{"*.go"}}

	if !r.matchTriggers(skill, []string{"internal/memory/database.go"}) {
		t.Error("esperaba match por basename *.go")
	}
	if r.matchTriggers(skill, []string{"README.md"}) {
		t.Error("no esperaba match para README.md")
	}

	pathSkill := Skill{Triggers: []string{"cmd/*"}}
	if !r.matchTriggers(pathSkill, []string{"cmd/main.go"}) {
		t.Error("esperaba match por ruta cmd/*")
	}
}

func TestVerifyCapabilities(t *testing.T) {
	r := NewResolver(t.TempDir())

	if !r.verifyCapabilities(Skill{Capabilities: []string{"go"}}) {
		t.Error("esperaba que 'go' esté en PATH durante los tests")
	}
	if r.verifyCapabilities(Skill{Capabilities: []string{"comando-que-no-existe-xyz"}}) {
		t.Error("no esperaba resolver un comando inexistente")
	}
}

func TestResolveSkillsEndToEnd(t *testing.T) {
	root := setupSkillsDir(t, map[string]string{
		"go.yaml": "name: go-rules\ntriggers:\n  - \"*.go\"\ncapabilities:\n  - go\nrules: regla\n",
		"py.yaml": "name: py-rules\ntriggers:\n  - \"*.py\"\ncapabilities:\n  - comando-inexistente-xyz\nrules: regla\n",
	})
	r := NewResolver(root)

	active, err := r.ResolveSkills([]string{"main.go"})
	if err != nil {
		t.Fatalf("ResolveSkills error: %v", err)
	}
	if len(active) != 1 || active[0].Name != "go-rules" {
		t.Fatalf("esperaba solo go-rules activo, obtuve %+v", active)
	}
}
