package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/skills"

	"gopkg.in/yaml.v3"
)

func skillsByName(sks []skills.Skill) map[string]skills.Skill {
	m := map[string]skills.Skill{}
	for _, s := range sks {
		m[s.Name] = s
	}
	return m
}

func TestCognitiveSkillsBundleCompleto(t *testing.T) {
	sks := cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}})
	m := skillsByName(sks)
	for _, name := range []string{"analyze-project", "deduce-conventions", "plan-ahead", "project-profile", "orchestrate-multiagent", "audit-structure-flow", "sdd-flow", "adversarial-review", "designing-web-ui"} {
		if _, ok := m[name]; !ok {
			t.Errorf("falta la skill cognitiva %q en el bundle: %v", name, m)
		}
	}
}

func TestAuditSkillEnBundle(t *testing.T) {
	m := skillsByName(cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}}))
	sk, ok := m["audit-structure-flow"]
	if !ok {
		t.Fatal("audit-structure-flow debe estar en el bundle cognitivo")
	}
	if len(sk.Triggers) == 0 || sk.Triggers[0] != "*" {
		t.Errorf("audit-structure-flow debe disparar siempre (*), obtuve %v", sk.Triggers)
	}
	for _, must := range []string{"ALTO", "código muerto", "musubi_save_observation"} {
		if !strings.Contains(sk.Rules, must) {
			t.Errorf("las reglas de audit-structure-flow deben mencionar %q", must)
		}
	}
}

// TestCognitiveSkillsPasanElGateDeCalidad es DOGFOODING: las skills que Musubi
// escribe en cada proyecto deben pasar su propio validador de calidad (sin errores).
func TestCognitiveSkillsPasanElGateDeCalidad(t *testing.T) {
	for _, sk := range cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}}) {
		report := skills.ValidateSkillQuality(sk)
		if !report.OK() {
			t.Errorf("la skill cognitiva %q no pasa el gate de calidad: %+v", sk.Name, report.Errors)
		}
	}
}

func TestWebUISkillEnBundle(t *testing.T) {
	m := skillsByName(cognitiveSkills([]detector.StackResult{{Ecosystem: "Node.js"}}))
	sk, ok := m["designing-web-ui"]
	if !ok {
		t.Fatal("falta la skill designing-web-ui en el bundle cognitivo")
	}
	// Debe dispararse en archivos web y traer un ejemplo (bloque de código).
	if len(sk.Triggers) == 0 || !strings.Contains(strings.Join(sk.Triggers, ","), "*.css") {
		t.Errorf("designing-web-ui debe dispararse en archivos web, triggers=%v", sk.Triggers)
	}
	if !strings.Contains(sk.Rules, "```") {
		t.Error("designing-web-ui debería incluir un ejemplo de código")
	}
}

func TestSDDFlowYAdversarialReviewEnBundle(t *testing.T) {
	m := skillsByName(cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}}))

	flow, ok := m["sdd-flow"]
	if !ok {
		t.Fatal("falta la skill sdd-flow en el bundle")
	}
	for _, must := range []string{"musubi_sdd", "action=start", "sdd/<change>"} {
		if !strings.Contains(flow.Rules, must) {
			t.Errorf("sdd-flow debe mencionar %q", must)
		}
	}

	rev, ok := m["adversarial-review"]
	if !ok {
		t.Fatal("falta la skill adversarial-review en el bundle")
	}
	// Debe documentar el patrón judgment-day: escépticos, mayoría y cableado a musubi_judge.
	for _, must := range []string{"musubi_work", "mayoría", "musubi_judge"} {
		if !strings.Contains(rev.Rules, must) {
			t.Errorf("adversarial-review debe mencionar %q", must)
		}
	}
}

func TestOrchestrateSkillDocumentaProtocolo(t *testing.T) {
	m := skillsByName(cognitiveSkills(nil))
	sk, ok := m["orchestrate-multiagent"]
	if !ok {
		t.Fatal("falta la skill orchestrate-multiagent")
	}
	// Debe documentar las tres patas del protocolo y pasar mcpServers a los sub-agentes.
	for _, must := range []string{"musubi_work", "claim", "mcpServers"} {
		if !strings.Contains(sk.Rules, must) {
			t.Errorf("la skill debe mencionar %q en sus reglas: %q", must, sk.Rules)
		}
	}
}

func TestCognitiveAnalyzeStackAware(t *testing.T) {
	m := skillsByName(cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}}))
	analyze := m["analyze-project"]
	tieneGo := false
	for _, tr := range analyze.Triggers {
		if tr == "*.go" {
			tieneGo = true
		}
	}
	if !tieneGo {
		t.Errorf("analyze-project debe heredar triggers del stack (*.go), obtuve %v", analyze.Triggers)
	}
}

func TestCognitivePlanYProfileGenericos(t *testing.T) {
	m := skillsByName(cognitiveSkills(nil))
	for _, name := range []string{"plan-ahead", "project-profile"} {
		sk := m[name]
		if len(sk.Triggers) == 0 || sk.Triggers[0] != "*" {
			t.Errorf("%s debe tener trigger genérico '*', obtuve %v", name, sk.Triggers)
		}
	}
}

func TestCognitiveProfileMencionaTopicKey(t *testing.T) {
	m := skillsByName(cognitiveSkills(nil))
	profile := m["project-profile"]
	if !strings.Contains(profile.Rules, "project/profile") {
		t.Errorf("project-profile debe anclar el perfil en topic_key 'project/profile', reglas: %q", profile.Rules)
	}
}

func TestWriteCognitiveSkillsEscribeArchivos(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module ej\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeCognitiveSkills(dir); err != nil {
		t.Fatalf("writeCognitiveSkills error: %v", err)
	}
	skillsDir := filepath.Join(dir, config.DirName, config.SkillsDir)
	for _, name := range []string{"analyze-project", "deduce-conventions", "plan-ahead", "project-profile"} {
		path := filepath.Join(skillsDir, name+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("falta el archivo de skill %s: %v", name, err)
		}
		var sk skills.Skill
		if err := yaml.Unmarshal(data, &sk); err != nil {
			t.Errorf("%s no es YAML válido: %v", name, err)
		}
		if sk.Name != name {
			t.Errorf("%s tiene name %q", path, sk.Name)
		}
	}
}

func TestWriteCognitiveSkillsNoSobrescribe(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	custom := "name: plan-ahead\ndescription: editado por el usuario\ntriggers:\n  - \"*\"\n"
	planPath := filepath.Join(skillsDir, "plan-ahead.yaml")
	if err := os.WriteFile(planPath, []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeCognitiveSkills(dir); err != nil {
		t.Fatalf("writeCognitiveSkills error: %v", err)
	}
	got, _ := os.ReadFile(planPath)
	if string(got) != custom {
		t.Errorf("writeCognitiveSkills no debe sobrescribir una skill editada por el usuario")
	}
}
