package skills

import (
	"strings"
	"testing"
)

// goodSkill es una skill de alta calidad usada como línea base (E4).
func goodSkill() Skill {
	return Skill{
		Name:        "processing-go-files",
		Description: "Enforces Go conventions and error handling. Use when editing .go files or reviewing Go code.",
		Triggers:    []string{"*.go"},
		Rules:       "Seguí las convenciones del proyecto.\n\nEjemplo:\n```go\nif err != nil {\n\treturn fmt.Errorf(\"ctx: %w\", err)\n}\n```\n",
	}
}

func codesOf(issues []QualityIssue) map[string]bool {
	m := map[string]bool{}
	for _, i := range issues {
		m[i.Code] = true
	}
	return m
}

func TestQualityGoodSkillScoresHigh(t *testing.T) { // E4
	r := ValidateSkillQuality(goodSkill())
	if !r.OK() {
		t.Fatalf("una skill buena no debería tener errores: %+v", r.Errors)
	}
	if r.Score < 85 {
		t.Fatalf("una skill buena debería puntuar ≥85, obtuve %d (warnings: %+v)", r.Score, r.Warnings)
	}
}

func TestQualityDescriptionEmptyIsError(t *testing.T) { // E1
	s := goodSkill()
	s.Description = "   "
	r := ValidateSkillQuality(s)
	if r.OK() || !codesOf(r.Errors)["desc_empty"] {
		t.Fatalf("description vacía debe ser error desc_empty, obtuve %+v", r.Errors)
	}
}

func TestQualityDescriptionTooLongIsError(t *testing.T) { // E2
	s := goodSkill()
	s.Description = strings.Repeat("a", DescMaxChars+1)
	r := ValidateSkillQuality(s)
	if !codesOf(r.Errors)["desc_too_long"] {
		t.Fatalf("description >1024 debe ser error desc_too_long, obtuve %+v", r.Errors)
	}
}

func TestQualityReservedNameIsError(t *testing.T) { // E3
	for _, name := range []string{"claude-helper", "my-anthropic-skill"} {
		s := goodSkill()
		s.Name = name
		r := ValidateSkillQuality(s)
		if !codesOf(r.Errors)["name_reserved"] {
			t.Errorf("name %q debe ser error name_reserved, obtuve %+v", name, r.Errors)
		}
	}
}

func TestQualityVagueOverBroadIsWarnedNotBlocked(t *testing.T) { // E5
	s := Skill{
		Name:        "stuff-doer",
		Description: "helps with stuff",
		Triggers:    []string{"*"},
		Rules:       "hacé cosas útiles según el contexto del proyecto y del usuario final",
	}
	r := ValidateSkillQuality(s)
	if !r.OK() {
		t.Fatalf("una skill vaga+over-broad NO debería tener errores (solo warnings): %+v", r.Errors)
	}
	w := codesOf(r.Warnings)
	if !w["desc_no_trigger"] || !w["triggers_over_broad"] || !w["rules_no_example"] {
		t.Fatalf("esperaba warnings desc_no_trigger + triggers_over_broad + rules_no_example, obtuve %+v", r.Warnings)
	}
	if r.Score >= ValidateSkillQuality(goodSkill()).Score {
		t.Fatalf("una skill vaga debería puntuar por debajo de una buena, obtuve %d", r.Score)
	}
}

func TestQualityWindowsPathAndPersonAreWarnings(t *testing.T) {
	s := goodSkill()
	s.Description = "You can process files. Use when editing code."
	s.Rules = s.Rules + "\ncorré scripts\\helper.py"
	r := ValidateSkillQuality(s)
	w := codesOf(r.Warnings)
	if !w["desc_person"] {
		t.Errorf("description en 2ª persona debería avisar desc_person: %+v", r.Warnings)
	}
	if !w["rules_windows_paths"] {
		t.Errorf("path estilo Windows en rules debería avisar rules_windows_paths: %+v", r.Warnings)
	}
	if !r.OK() {
		t.Errorf("estos son warnings, no deberían bloquear: %+v", r.Errors)
	}
}

func TestQualityScoreFloorAtZero(t *testing.T) {
	// Muchos errores + warnings no bajan de 0.
	if got := scoreFor(5, 5); got != 0 {
		t.Errorf("el score debería tener piso 0, obtuve %d", got)
	}
}

func TestSourceTrustTier(t *testing.T) {
	cases := map[string]string{
		"https://github.com/anthropics/skills/blob/main/x/SKILL.md": "official",
		"https://docs.claude.com/skills":                            "official",
		"https://github.com/PatrickJS/awesome-cursorrules":          "curated",
		"https://github.com/Gentleman-Programming/Gentleman-Skills":  "curated",
		"https://github.com/random/repo":                            "community",
		"": "unknown",
	}
	for url, want := range cases {
		if got := SourceTrustTier(url); got != want {
			t.Errorf("SourceTrustTier(%q) = %q, quería %q", url, got, want)
		}
	}
}
