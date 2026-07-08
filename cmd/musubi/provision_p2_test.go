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

	// Idempotente: una segunda corrida no duplica el hook Stop.
	_ = injectLocalSetup(dir, "musubi")
	data2, _ := os.ReadFile(settingsPath)
	if n := strings.Count(string(data2), "capture --hook-mode"); n != 1 {
		t.Fatalf("el hook Stop se duplicó (%d veces)", n)
	}
}
