package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// `musubi setup` REGISTRA EL HOOK PostToolUse(Skill) → skill-loaded, de forma idempotente (R7).
//
// Sin él, un repo cableado a mano no contaría las cargas de skills; y el plugin, que corre los mismos
// hooks que setup, tampoco (lo cuida TestElPluginLlevaLosMismosGanchosQueSetup).
//
// Sabotaje que la hace fallar: que setup no llame al escritor.
// arnes: archivo="cmd/musubi/setup.go"
// arnes: de="\t\tif err := writeSkillLoadedHook(root, exePath); err != nil {\n"
// arnes: a="\t\tif err := error(nil); err != nil {\n"
func TestSetupRegistraElHookDeCargaDeSkills(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	correrMusubi(t, repo, home, "", "setup", "--agent", "claude")
	settingsPath := filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile)
	crudo, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("setup no dejó .claude/settings.json: %v", err)
	}
	var hay bool
	for _, g := range ganchosDeUnSettings(t, crudo) {
		if strings.HasPrefix(g, "PostToolUse|Skill|") && strings.HasSuffix(g, "skill-loaded --hook-mode") {
			hay = true
		}
	}
	if !hay {
		t.Errorf("setup no registró el hook PostToolUse(Skill):\n%s", crudo)
	}

	// Idempotente: una segunda corrida no duplica el hook (R7).
	correrMusubi(t, repo, home, "", "setup", "--agent", "claude")
	crudo2, _ := os.ReadFile(settingsPath)
	if n := strings.Count(string(crudo2), "skill-loaded --hook-mode"); n != 1 {
		t.Errorf("el hook de carga quedó %d veces; setup tiene que ser idempotente", n)
	}
}
