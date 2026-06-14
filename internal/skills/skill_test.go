package skills

import (
	"musubi/internal/config"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSkillProvenanciaRoundTrip verifica que los campos de procedencia se
// serialicen y deserialicen correctamente via yaml.Marshal / yaml.Unmarshal.
func TestSkillProvenanciaRoundTrip(t *testing.T) {
	original := Skill{
		Name:        "mi-skill",
		Description: "skill de prueba",
		Triggers:    []string{"*.go"},
		Capabilities: []string{},
		Rules:       "usar SOLID",
		GeneratedBy: "auto-discovery",
		GeneratedAt: "2026-01-01T00:00:00Z",
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var recuperada Skill
	if err := yaml.Unmarshal(data, &recuperada); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if recuperada.GeneratedBy != "auto-discovery" {
		t.Errorf("GeneratedBy: esperaba %q, obtuve %q", "auto-discovery", recuperada.GeneratedBy)
	}
	if recuperada.GeneratedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("GeneratedAt: esperaba %q, obtuve %q", "2026-01-01T00:00:00Z", recuperada.GeneratedAt)
	}
}

// TestSkillSinProvenanciaOmitempty verifica que las skills sin procedencia no
// incluyan los campos generated_by/generated_at en el YAML serializado.
func TestSkillSinProvenanciaOmitempty(t *testing.T) {
	s := Skill{
		Name:    "sin-procedencia",
		Triggers: []string{"*.py"},
		Rules:   "reglas python",
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}
	contenido := string(data)

	if contains(contenido, "generated_by") {
		t.Error("YAML no debería contener 'generated_by' cuando el campo está vacío (omitempty)")
	}
	if contains(contenido, "generated_at") {
		t.Error("YAML no debería contener 'generated_at' cuando el campo está vacío (omitempty)")
	}
}

// TestResolverIgnoraProvenancia verifica que ResolveSkills retorna skills con
// campos de procedencia si coinciden con los triggers, sin importar esos campos.
func TestResolverIgnoraProvenancia(t *testing.T) {
	yamlConProcedencia := `name: go-generada
description: skill generada
triggers:
  - "*.go"
capabilities:
  - go
rules: reglas de go
generated_by: auto-discovery
generated_at: "2026-01-01T00:00:00Z"
`
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go-generada.yaml"), []byte(yamlConProcedencia), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	r := NewResolver(root)

	// Verificar que LoadSkills carga la skill con procedencia
	skills, err := r.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("esperaba 1 skill, obtuve %d", len(skills))
	}
	if skills[0].GeneratedBy != "auto-discovery" {
		t.Errorf("GeneratedBy: esperaba %q, obtuve %q", "auto-discovery", skills[0].GeneratedBy)
	}

	// Verificar que ResolveSkills la retorna al hacer match (go está en PATH durante tests)
	activas, err := r.ResolveSkills([]string{"main.go"})
	if err != nil {
		t.Fatalf("ResolveSkills error: %v", err)
	}
	if len(activas) != 1 {
		t.Fatalf("esperaba 1 skill activa, obtuve %d", len(activas))
	}
	if activas[0].Name != "go-generada" {
		t.Errorf("esperaba 'go-generada', obtuve %q", activas[0].Name)
	}
}

// contains es un helper simple para verificar substrings en texto.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLoop(s, substr))
}

func containsLoop(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
