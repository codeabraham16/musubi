package memory

import (
	"encoding/json"
	"strings"
	"testing"
)

// D2 / R7 — el campo exigencia_skill viaja con las claves en minúscula y se OMITE cuando no hay
// exigencia (el camino model-free, que es el default, no paga ni un token por él).

func TestExigenciaSkillMarshalEnMinuscula(t *testing.T) {
	res := RecallResult{
		Count:          1,
		ExigenciaSkill: &ExigenciaSkill{Skill: "go-hygiene", Razon: "la tarea toca .go"},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	// Las claves van en minúscula (musubi-rules #6): un consumidor que parsea 'skill'/'razon' las
	// encuentra. Sin los tags JSON saldría "Skill"/"Razon" y el receptor guardaría un objeto vacío
	// —una falla que se lee como un dato—. Sabotaje: quitar los tags de ExigenciaSkill ⇒ rojo.
	if !strings.Contains(s, `"exigencia_skill":{"skill":"go-hygiene","razon":"la tarea toca .go"}`) {
		t.Fatalf("exigencia_skill no salió con las claves en minúscula:\n%s", s)
	}
}

func TestExigenciaSkillSeOmiteCuandoNoHay(t *testing.T) {
	b, err := json.Marshal(RecallResult{Count: 0})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "exigencia_skill") {
		t.Fatalf("sin exigencia la clave debe omitirse (omitempty); salió:\n%s", string(b))
	}
}
