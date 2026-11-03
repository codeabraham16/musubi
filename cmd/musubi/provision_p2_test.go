package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/provision"
)

func TestInjectLocalSetup(t *testing.T) {
	dir := t.TempDir()

	steps := injectLocalSetup(dir, "musubi")
	if len(steps) == 0 {
		t.Fatal("esperaba pasos de setup local")
	}
	for _, s := range steps {
		if s.Status != provision.StatusDone {
			t.Errorf("paso %q no quedó done: %+v", s.Name, s)
		}
	}

	// Workspace + skills + settings con el hook Stop.
	if _, err := os.Stat(filepath.Join(dir, ".musubi")); err != nil {
		t.Fatalf(".musubi/ no se creó: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".musubi", "skills")); err != nil {
		t.Fatalf(".musubi/skills/ no se creó: %v", err)
	}
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf(".claude/settings.json no se creó: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"Stop"`) || !strings.Contains(s, "capture --hook-mode") {
		t.Fatalf("settings.json sin el hook Stop de captura:\n%s", s)
	}
	// provision (no sólo setup) tiene que registrar el hook PostToolUse(Skill) → skill-loaded (R7). El
	// loop de arriba sólo exige que cada paso quede Done, cosa que un paso que no escribe nada igual
	// cumple; por eso hace falta mirar que el hook quedó de verdad en settings.json.
	// Sabotaje que la pone en rojo: que provision registre el paso con un error en vez del hook (el
	// `add` toma un error como 2º arg), y así el paso existe pero el matcher Skill no se escribe.
	// arnes: archivo="cmd/musubi/provision.go" prueba="TestInjectLocalSetup" de="add(\"hook-skill-loaded\", writeSkillLoadedHook(projectDir, exePath), \"PostToolUse Skill (cuenta las cargas de skills)\")" a="add(\"hook-skill-loaded\", error(nil), \"PostToolUse Skill (cuenta las cargas de skills)\")"
	if !strings.Contains(s, "skill-loaded --hook-mode") {
		t.Fatalf("settings.json sin el hook PostToolUse(Skill) de carga de skills:\n%s", s)
	}

	// Idempotente: una segunda corrida no duplica el hook Stop.
	_ = injectLocalSetup(dir, "musubi")
	data2, _ := os.ReadFile(settingsPath)
	if n := strings.Count(string(data2), "capture --hook-mode"); n != 1 {
		t.Fatalf("el hook Stop se duplicó (%d veces)", n)
	}
	// Y el hook PostToolUse(Skill) tampoco: R7 exige que el registro sea idempotente, no sólo que
	// ocurra. Un segundo provision que lo vuelva a agregar contaría dos veces cada carga.
	if n := strings.Count(string(data2), "skill-loaded --hook-mode"); n != 1 {
		t.Fatalf("el hook PostToolUse(Skill) se duplicó (%d veces)", n)
	}
}
