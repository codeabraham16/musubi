package mcp

import (
	"testing"

	"musubi/internal/memory"
	"musubi/internal/skillsource"
)

// TestExcludeRejectedSkills verifica que el filtro de feedback (T6.1) excluye las skills con
// decisión más reciente "rejected" y respeta last-write-wins (una rechazada y luego aceptada
// vuelve a proponerse).
func TestExcludeRejectedSkills(t *testing.T) {
	cands := []skillsource.Candidate{
		{Entry: skillsource.CatalogEntry{ID: "go-gin"}},
		{Entry: skillsource.CatalogEntry{ID: "go-testing"}},
		{Entry: skillsource.CatalogEntry{ID: "rust-axum"}},
		{Entry: skillsource.CatalogEntry{ID: "sin-decision"}},
	}
	decisions := []memory.SkillDecision{
		{SkillID: "go-gin", Decision: "rejected"},
		{SkillID: "go-testing", Decision: "accepted"},
		{SkillID: "rust-axum", Decision: "rejected"},
		{SkillID: "rust-axum", Decision: "accepted"}, // re-aceptada: gana la última
	}

	got := excludeRejectedSkills(cands, decisions)
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.Entry.ID] = true
	}
	if ids["go-gin"] {
		t.Error("go-gin (rejected) debió excluirse")
	}
	if !ids["go-testing"] {
		t.Error("go-testing (accepted) debió quedar")
	}
	if !ids["rust-axum"] {
		t.Error("rust-axum (rejected → accepted) debió quedar por last-write-wins")
	}
	if !ids["sin-decision"] {
		t.Error("una skill sin decisión previa debió quedar")
	}
	if len(got) != 3 {
		t.Errorf("esperaba 3 candidatos tras filtrar, obtuve %d", len(got))
	}
}

// TestExcludeRejectedSkillsNoDecisions: sin decisiones previas, devuelve todo intacto.
func TestExcludeRejectedSkillsNoDecisions(t *testing.T) {
	cands := []skillsource.Candidate{
		{Entry: skillsource.CatalogEntry{ID: "a"}},
		{Entry: skillsource.CatalogEntry{ID: "b"}},
	}
	if got := excludeRejectedSkills(cands, nil); len(got) != 2 {
		t.Errorf("sin decisiones debe devolver todos, obtuve %d", len(got))
	}
}
