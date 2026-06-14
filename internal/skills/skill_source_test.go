package skills

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSkillSourceURLRoundTrip verifica que Source y SourceURL se serialicen
// y deserialicen correctamente via yaml.Marshal / yaml.Unmarshal.
func TestSkillSourceURLRoundTrip(t *testing.T) {
	original := Skill{
		Name:        "go-gin",
		Description: "Reglas para proyectos Go con Gin",
		Triggers:    []string{"*.go"},
		Rules:       "usar SOLID",
		Source:      "musubi-catalog-v1",
		SourceURL:   "https://raw.githubusercontent.com/example/musubi/main/catalog/skills/go-gin.md",
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var recuperada Skill
	if err := yaml.Unmarshal(data, &recuperada); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if recuperada.Source != "musubi-catalog-v1" {
		t.Errorf("Source: esperaba %q, obtuve %q", "musubi-catalog-v1", recuperada.Source)
	}
	if recuperada.SourceURL != "https://raw.githubusercontent.com/example/musubi/main/catalog/skills/go-gin.md" {
		t.Errorf("SourceURL: esperaba URL completa, obtuve %q", recuperada.SourceURL)
	}
}

// TestSkillLegaciaYAMLSinSourceCargaCorrectamente verifica que YAML sin los campos
// source/source_url se deserialice sin error y con valores cero.
func TestSkillLegaciaYAMLSinSourceCargaCorrectamente(t *testing.T) {
	yamlLegacy := `name: mi-skill
description: skill legada
triggers:
  - "*.go"
capabilities:
  - go
rules: reglas de go
generated_by: auto-discovery
generated_at: "2026-01-01T00:00:00Z"
`
	var s Skill
	if err := yaml.Unmarshal([]byte(yamlLegacy), &s); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if s.Source != "" {
		t.Errorf("Source debería ser vacío para YAML legado, obtuve %q", s.Source)
	}
	if s.SourceURL != "" {
		t.Errorf("SourceURL debería ser vacío para YAML legado, obtuve %q", s.SourceURL)
	}
	// Verificar que los campos existentes siguen correctos
	if s.Name != "mi-skill" {
		t.Errorf("Name: esperaba %q, obtuve %q", "mi-skill", s.Name)
	}
}

// TestSkillSourceOmitemptyNoAparececEnYAMLVacio verifica que los campos source/source_url
// no aparecen en el YAML cuando están vacíos (omitempty).
func TestSkillSourceOmitemptyNoAparececEnYAMLVacio(t *testing.T) {
	s := Skill{
		Name:  "sin-fuente",
		Rules: "reglas",
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}
	contenido := string(data)

	if strings.Contains(contenido, "source") {
		t.Errorf("YAML no debería contener 'source' cuando está vacío (omitempty), contenido: %q", contenido)
	}
	if strings.Contains(contenido, "source_url") {
		t.Errorf("YAML no debería contener 'source_url' cuando está vacío (omitempty), contenido: %q", contenido)
	}
}

// TestResolverIgnoraSource verifica que ResolveSkills no cambia comportamiento
// al agregar campos Source/SourceURL a una skill.
func TestResolverIgnoraSource(t *testing.T) {
	// El resolver solo mira Triggers y Capabilities: Source/SourceURL son ignorados.
	skill := Skill{
		Triggers:    []string{"*.go"},
		Capabilities: []string{},
		Source:      "musubi-catalog-v1",
		SourceURL:   "https://example.com/rules.md",
	}

	r := NewResolver(t.TempDir())
	// matchTriggers debe funcionar igual independiente de Source/SourceURL
	if !r.matchTriggers(skill, []string{"main.go"}) {
		t.Error("matchTriggers debería hacer match en *.go sin importar Source/SourceURL")
	}
}
