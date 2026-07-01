package memory

import (
	"strings"
	"testing"
)

func TestSlugifyChange(t *testing.T) {
	cases := map[string]string{
		"Add Auth":          "add-auth",
		"  Trim Me  ":       "trim-me",
		"already-slug":      "already-slug",
		"Múltiples   gaps!": "múltiples-gaps",
		"UPPER_snake.case":  "upper-snake-case",
		"---edges---":       "edges",
	}
	for in, want := range cases {
		if got := SlugifyChange(in); got != want {
			t.Errorf("SlugifyChange(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestSDDWorkflowDefIsLinearChain(t *testing.T) {
	def := SDDWorkflowDef("My Change")
	if def.ID != "sdd-my-change" {
		t.Fatalf("ID = %q, quería sdd-my-change", def.ID)
	}
	if len(def.Steps) != len(SDDPhases) {
		t.Fatalf("steps = %d, quería %d", len(def.Steps), len(SDDPhases))
	}
	// Cada fase (salvo la primera) depende de la anterior; el orden coincide con SDDPhases.
	for i, s := range def.Steps {
		if s.ID != SDDPhases[i] {
			t.Errorf("step[%d].ID = %q, quería %q", i, s.ID, SDDPhases[i])
		}
		if i == 0 {
			if len(s.Needs) != 0 {
				t.Errorf("primera fase no debería tener needs, tiene %v", s.Needs)
			}
			continue
		}
		if len(s.Needs) != 1 || s.Needs[0] != SDDPhases[i-1] {
			t.Errorf("step %q needs = %v, quería [%q]", s.ID, s.Needs, SDDPhases[i-1])
		}
	}
	// La definición canónica debe ser un DAG válido.
	if errs := def.Validate(); len(errs) != 0 {
		t.Fatalf("la def SDD canónica no validó: %v", errs)
	}
}

func TestSDDTopicKeyAndRunID(t *testing.T) {
	if got := SDDRunID("Add Auth"); got != "sdd-add-auth" {
		t.Errorf("SDDRunID = %q", got)
	}
	if got := SDDTopicKey("Add Auth", "spec"); got != "sdd/add-auth/spec" {
		t.Errorf("SDDTopicKey = %q", got)
	}
}

func TestSDDTemplatePath(t *testing.T) {
	for _, p := range []string{"proposal", "spec", "design", "tasks"} {
		path, ok := SDDTemplatePath(p)
		if !ok || !strings.HasSuffix(path, p+".md") {
			t.Errorf("fase documental %q: path=%q ok=%v", p, path, ok)
		}
	}
	for _, p := range []string{"implement", "verify", "archive"} {
		if _, ok := SDDTemplatePath(p); ok {
			t.Errorf("fase de acción %q no debería tener plantilla", p)
		}
	}
}

func TestSDDContractMemo(t *testing.T) {
	c := SDDContract{
		Summary:         "Se definió el esquema de auth.",
		Artifacts:       []string{"specs/auth.md", "sdd/add-auth/spec"},
		Risks:           []string{"migración de sesiones"},
		NextRecommended: "diseñar el middleware",
	}
	memo := c.Memo("add-auth", "spec")
	for _, want := range []string{
		"## SDD spec — add-auth",
		"Se definió el esquema de auth.",
		"**Artefactos:**", "- specs/auth.md",
		"**Riesgos:**", "- migración de sesiones",
		"**Siguiente recomendado:** diseñar el middleware",
	} {
		if !strings.Contains(memo, want) {
			t.Errorf("memo no contiene %q\n--- memo ---\n%s", want, memo)
		}
	}

	// Sin opcionales, solo el encabezado + summary (sin secciones vacías).
	bare := SDDContract{Summary: "solo resumen"}.Memo("c", "proposal")
	if strings.Contains(bare, "Artefactos") || strings.Contains(bare, "Riesgos") {
		t.Errorf("memo mínimo no debería incluir secciones vacías:\n%s", bare)
	}
}

func TestSDDPhaseDirectiveNeverEmpty(t *testing.T) {
	for _, p := range SDDPhases {
		if strings.TrimSpace(SDDPhaseDirective(p, "add-auth")) == "" {
			t.Errorf("directiva vacía para la fase %q", p)
		}
	}
	// La directiva de implement debe orientar el recall del handoff.
	if !strings.Contains(SDDPhaseDirective("implement", "Add Auth"), "sdd/add-auth") {
		t.Errorf("la directiva de implement debería referenciar sdd/add-auth")
	}
}
