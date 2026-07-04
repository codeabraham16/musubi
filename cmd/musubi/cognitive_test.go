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
	// Debe documentar el patrón judgment-day sobre el subsistema de debate: escépticos,
	// tally por mayoría determinista y cableado a musubi_judge para el desempate.
	for _, must := range []string{"musubi_debate", "tally", "mayoría", "musubi_judge"} {
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
	if _, err := writeCognitiveSkills(dir); err != nil {
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
	if _, err := writeCognitiveSkills(dir); err != nil {
		t.Fatalf("writeCognitiveSkills error: %v", err)
	}
	got, _ := os.ReadFile(planPath)
	if string(got) != custom {
		t.Errorf("writeCognitiveSkills no debe sobrescribir una skill editada por el usuario")
	}
}

// --- refresh de skills manejadas (sin pisar ediciones del usuario) ---

// canonicalSkill devuelve la skill canónica del bundle por nombre (stack Go).
func canonicalSkill(t *testing.T, name string) skills.Skill {
	t.Helper()
	for _, sk := range cognitiveSkills([]detector.StackResult{{Ecosystem: "Go"}}) {
		if sk.Name == name {
			return sk
		}
	}
	t.Fatalf("no existe la skill canónica %q", name)
	return skills.Skill{}
}

// goSkillDir prepara un temp con go.mod (para que DetectStack dé Go) y devuelve el dir de skills.
func goSkillDir(t *testing.T) (root, skillsDir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module ej\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	skillsDir = filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	return root, skillsDir
}

func writeSkillYAML(t *testing.T, skillsDir string, sk skills.Skill) string {
	t.Helper()
	data, err := yaml.Marshal(sk)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillsDir, sk.Name+".yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readSkillYAML(t *testing.T, path string) skills.Skill {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sk skills.Skill
	if err := yaml.Unmarshal(data, &sk); err != nil {
		t.Fatalf("YAML inválido en %s: %v", path, err)
	}
	return sk
}

// R2: una skill manejada INTACTA (checksum coincide) pero de versión vieja → se refresca.
func TestManagedSkillRefreshesWhenIntact(t *testing.T) {
	root, skillsDir := goSkillDir(t)
	canon := canonicalSkill(t, "adversarial-review")

	// Simular una versión VIEJA manejada: mismo esqueleto, reglas viejas, checksum válido.
	old := canon
	old.Rules = "REGLA VIEJA que un binario anterior escribió.\n"
	sum, err := skillContentChecksum(old)
	if err != nil {
		t.Fatal(err)
	}
	old.ManagedChecksum = sum
	path := writeSkillYAML(t, skillsDir, old)

	refreshed, err := writeCognitiveSkills(root)
	if err != nil {
		t.Fatalf("writeCognitiveSkills: %v", err)
	}
	got := readSkillYAML(t, path)
	if got.Rules != canon.Rules {
		t.Errorf("una skill manejada intacta debe refrescarse a la canónica; reglas=%q", got.Rules)
	}
	found := false
	for _, n := range refreshed {
		if n == "adversarial-review" {
			found = true
		}
	}
	if !found {
		t.Errorf("adversarial-review debía reportarse como refrescada, obtuve %v", refreshed)
	}
}

// R3: una skill EDITADA por el usuario (checksum ya no coincide) → se preserva.
func TestManagedSkillPreservedWhenEdited(t *testing.T) {
	root, skillsDir := goSkillDir(t)
	canon := canonicalSkill(t, "adversarial-review")

	// Escribir una versión manejada con checksum de OTRO contenido (el usuario editó las reglas
	// después de que Musubi la escribió, sin recomputar el checksum).
	edited := canon
	baseSum, _ := skillContentChecksum(canon) // checksum del contenido canónico...
	edited.ManagedChecksum = baseSum
	edited.Rules = "REGLAS EDITADAS A MANO POR EL USUARIO.\n" // ...pero el contenido ya no matchea
	path := writeSkillYAML(t, skillsDir, edited)

	refreshed, err := writeCognitiveSkills(root)
	if err != nil {
		t.Fatalf("writeCognitiveSkills: %v", err)
	}
	got := readSkillYAML(t, path)
	if got.Rules != "REGLAS EDITADAS A MANO POR EL USUARIO.\n" {
		t.Errorf("una skill editada NO debe pisarse; reglas=%q", got.Rules)
	}
	for _, n := range refreshed {
		if n == "adversarial-review" {
			t.Error("una skill editada no debe reportarse como refrescada")
		}
	}
}

// R4: un archivo legacy SIN checksum pero idéntico a la canónica → se adopta (agrega checksum).
func TestManagedSkillBootstrapAdopts(t *testing.T) {
	root, skillsDir := goSkillDir(t)
	canon := canonicalSkill(t, "adversarial-review") // sin ManagedChecksum
	path := writeSkillYAML(t, skillsDir, canon)

	// Antes: sin checksum.
	if readSkillYAML(t, path).ManagedChecksum != "" {
		t.Fatal("precondición: el archivo legacy no debe tener checksum")
	}
	refreshed, err := writeCognitiveSkills(root)
	if err != nil {
		t.Fatalf("writeCognitiveSkills: %v", err)
	}
	got := readSkillYAML(t, path)
	if got.ManagedChecksum == "" {
		t.Error("un archivo idéntico a la canónica debe ADOPTARSE (agregar checksum)")
	}
	if got.Rules != canon.Rules {
		t.Error("la adopción no debe cambiar el contenido")
	}
	found := false
	for _, n := range refreshed {
		if n == "adversarial-review" {
			found = true
		}
	}
	if !found {
		t.Errorf("la adopción debía reportarse como refrescada, obtuve %v", refreshed)
	}
}

// Idempotencia: correr writeCognitiveSkills de nuevo sobre skills ya en esta versión NO debe
// reescribir ni reportar nada como refrescado (sin cry-wolf ni churn).
func TestManagedSkillIdempotentNoChurn(t *testing.T) {
	root, _ := goSkillDir(t)
	// Primera corrida: escribe todo el bundle fresco (con checksums).
	if _, err := writeCognitiveSkills(root); err != nil {
		t.Fatal(err)
	}
	// Segunda corrida: nada cambió → refreshed vacío.
	refreshed, err := writeCognitiveSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed) != 0 {
		t.Errorf("una segunda corrida sin cambios no debe refrescar nada, obtuve %v", refreshed)
	}
}

// R0: un YAML corrupto no debe pisarse ni causar panic.
func TestManagedSkillCorruptPreserved(t *testing.T) {
	root, skillsDir := goSkillDir(t)
	garbage := "esto: no es: yaml válido: [\n"
	path := filepath.Join(skillsDir, "adversarial-review.yaml")
	if err := os.WriteFile(path, []byte(garbage), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCognitiveSkills(root); err != nil {
		t.Fatalf("writeCognitiveSkills no debe fallar ante un archivo corrupto: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != garbage {
		t.Error("un archivo corrupto debe preservarse (no pisarse)")
	}
}
