package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// authorOut es la forma de la respuesta de musubi_author_skill.
type authorOut struct {
	Skill       string `json:"skill"`
	Score       int    `json:"score"`
	OK          bool   `json:"ok"`
	Saved       bool   `json:"saved"`
	Path        string `json:"path"`
	SourceTrust string `json:"source_trust"`
	Note        string `json:"note"`
	Errors      []struct {
		Code string `json:"code"`
	} `json:"errors"`
	Warnings []struct {
		Code string `json:"code"`
	} `json:"warnings"`
}

func parseAuthor(t *testing.T, res interface{}) authorOut {
	t.Helper()
	var out authorOut
	if err := json.Unmarshal([]byte(textOf(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear la respuesta author: %v", err)
	}
	return out
}

func TestAuthorSkillIteratesThenSaves(t *testing.T) { // E6
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	// 1) Sin save: una skill vaga/over-broad devuelve reporte con warnings y NO guarda.
	res, e := call(t, s, "musubi_author_skill", map[string]interface{}{
		"name":     "stuff-doer",
		"triggers": []string{"*"},
		"rules":    "hacé cosas útiles según el contexto del proyecto y del usuario",
	})
	if e != nil {
		t.Fatalf("author sin save: %+v", e)
	}
	out := parseAuthor(t, res)
	if out.Saved {
		t.Fatal("sin save=true no debería guardar")
	}
	if len(out.Warnings) == 0 {
		t.Fatal("una skill vaga debería traer warnings accionables")
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName, config.SkillsDir, "stuff-doer.yaml")); err == nil {
		t.Fatal("no debería haber escrito el archivo sin save=true")
	}

	// 2) Skill corregida con save=true: pasa el gate y se guarda.
	res, e = call(t, s, "musubi_author_skill", map[string]interface{}{
		"name":        "processing-go-files",
		"description": "Aplica convenciones de Go y manejo de errores. Use when editando archivos .go.",
		"triggers":    []string{"*.go"},
		"rules":       "Envolvé errores con fmt.Errorf.\n\nEjemplo:\n```go\nreturn fmt.Errorf(\"ctx: %w\", err)\n```\n",
		"source_url":  "https://github.com/anthropics/skills",
		"save":        true,
	})
	if e != nil {
		t.Fatalf("author con save: %+v", e)
	}
	out = parseAuthor(t, res)
	if !out.Saved || !out.OK {
		t.Fatalf("la skill corregida debería guardarse y pasar el gate: %+v", out)
	}
	if out.Score < 85 {
		t.Errorf("una skill corregida debería puntuar alto, obtuve %d", out.Score)
	}
	if out.SourceTrust != "official" {
		t.Errorf("source_trust de anthropics/skills debería ser official, obtuve %q", out.SourceTrust)
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName, config.SkillsDir, "processing-go-files.yaml")); err != nil {
		t.Errorf("la skill debería haberse escrito: %v", err)
	}
}

func TestAuthorSkillSaveBlockedByQualityGate(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	// save=true con description faltante → el gate de calidad bloquea (error, no guarda).
	_, e := call(t, s, "musubi_author_skill", map[string]interface{}{
		"name":     "no-desc-skill",
		"triggers": []string{"*.go"},
		"rules":    "una regla suficientemente larga para pasar el mínimo estructural",
		"save":     true,
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Fatalf("guardar sin description debería bloquear por el gate, obtuve %+v", e)
	}
}

func TestSaveSkillRejectsEmptyDescription(t *testing.T) { // E1 vía el gate en save
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "sin-desc",
		"triggers": []string{"*.go"},
		"rules":    "una regla suficientemente larga para pasar el mínimo estructural",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Fatalf("save sin description debería rechazarse por el gate de calidad, obtuve %+v", e)
	}
}
