package main

import (
	"strings"
	"testing"

	"musubi/internal/detector"
	"musubi/internal/skills"

	"gopkg.in/yaml.v3"
)

func parseStarter(t *testing.T, content string) skills.Skill {
	t.Helper()
	var sk skills.Skill
	if err := yaml.Unmarshal([]byte(content), &sk); err != nil {
		t.Fatalf("el starter skill no es YAML válido: %v\n%s", err, content)
	}
	return sk
}

func TestStarterSkillStackAware(t *testing.T) {
	stack := []detector.StackResult{{Ecosystem: "Go"}}
	sk := parseStarter(t, starterSkillContent(stack))

	tieneGo := false
	for _, tr := range sk.Triggers {
		if tr == "*.go" {
			tieneGo = true
		}
	}
	if !tieneGo {
		t.Errorf("un proyecto Go debe tener trigger *.go, obtuve %v", sk.Triggers)
	}
	if !strings.Contains(sk.Description, "Go") {
		t.Errorf("la descripción debe nombrar el stack detectado, obtuve %q", sk.Description)
	}
}

func TestStarterSkillPoliglota(t *testing.T) {
	stack := []detector.StackResult{
		{Ecosystem: "Go"},
		{Ecosystem: "Node.js", Frameworks: []string{"react"}},
	}
	sk := parseStarter(t, starterSkillContent(stack))
	joined := strings.Join(sk.Triggers, ",")
	if !strings.Contains(joined, "*.go") || !strings.Contains(joined, "*.ts") {
		t.Errorf("un proyecto Go+Node debe incluir triggers de ambos, obtuve %v", sk.Triggers)
	}
}

func TestStarterSkillGenericoSinStack(t *testing.T) {
	sk := parseStarter(t, starterSkillContent(nil))
	if len(sk.Triggers) == 0 || sk.Triggers[0] != "*" {
		t.Errorf("sin stack detectado el starter debe caer al trigger genérico '*', obtuve %v", sk.Triggers)
	}
}
