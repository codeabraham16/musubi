package cognition

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Invariantes del decididor de skills (specs/skills-decididor/) que viven en este paquete: el
// juicio puro. La disciplina del llamador (guard/gate/freno/caché) se prueba en internal/mcp.

var catalogoDePrueba = []SkillN1{
	{Name: "go-hygiene", Description: "cómo escribir Go en este repo", Cuando: "tocás archivos .go"},
	{Name: "designing-web-ui", Description: "estándares de UI web", Cuando: "editás HTML/CSS/componentes"},
	{Name: "adversarial-review", Description: "revisión adversarial con evidencia", Cuando: "cerrás un cambio de riesgo"},
}

// R8 / E10 — ELECCIÓN VÁLIDA: el motor sólo puede exigir una skill del catálogo que vio. Una
// inventada (alucinada) se descarta a "ninguna", no se propaga: exigir cargar algo inexistente
// manda al agente a buscar un archivo que no está.
func TestElegirSkillDescartaLaInventada(t *testing.T) {
	m := &motorGrabador{respuesta: `{"skill":"una-que-no-existe","razon":"me la inventé"}`}
	elec, err := ElegirSkill(context.Background(), m, "auditá el codebase", catalogoDePrueba)
	if err != nil {
		t.Fatalf("una skill inventada no es un error, es 'ninguna': %v", err)
	}
	if elec.Skill != "" {
		t.Fatalf("esperaba 'ninguna' (Skill vacío) ante una skill fuera del catálogo, obtuve %q", elec.Skill)
	}
}

// El camino feliz: una skill del catálogo se devuelve tal cual, con su razón.
func TestElegirSkillValidaSeDevuelve(t *testing.T) {
	m := &motorGrabador{respuesta: `{"skill":"go-hygiene","razon":"la tarea toca .go"}`}
	elec, err := ElegirSkill(context.Background(), m, "arreglá el error en recall.go", catalogoDePrueba)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if elec.Skill != "go-hygiene" || elec.Razon != "la tarea toca .go" {
		t.Fatalf("esperaba go-hygiene con razón, obtuve %+v", elec)
	}
}

// "Ninguna" es una respuesta VÁLIDA y frecuente: la mayoría de las tareas no exige una skill.
// Skill vacío con error nil, no un error.
func TestElegirSkillNingunaEsValida(t *testing.T) {
	m := &motorGrabador{respuesta: `{"skill":"","razon":"ninguna aplica"}`}
	elec, err := ElegirSkill(context.Background(), m, "qué hora es", catalogoDePrueba)
	if err != nil {
		t.Fatalf("'ninguna' no es un error: %v", err)
	}
	if elec.Skill != "" {
		t.Fatalf("esperaba Skill vacío, obtuve %q", elec.Skill)
	}
}

// R6 — BEST-EFFORT (lado cognición): una respuesta imparseable es un ERROR explícito, no un
// Eleccion{} silencioso. El llamador degrada ante el error; un Eleccion{} vacío sin error se vería
// idéntico a "el motor eligió ninguna" y escondería que el motor contestó basura.
func TestElegirSkillImparseableEsError(t *testing.T) {
	basura := []string{
		"no tengo idea, perdón",
		"",
		"{",
		"skill: go-hygiene", // no es JSON
	}
	for _, resp := range basura {
		m := &motorGrabador{respuesta: resp}
		elec, err := ElegirSkill(context.Background(), m, "consulta", catalogoDePrueba)
		if err == nil {
			t.Errorf("respuesta %q: esperaba error, obtuve %+v", resp, elec)
		}
	}
}

// Control de R6: el motor caído es error, y el mensaje conserva la causa (errors.Is) para que el
// llamador pueda loguearla.
func TestElegirSkillMotorCaidoEsError(t *testing.T) {
	roto := errors.New("connection refused")
	m := &motorGrabador{err: roto}
	if _, err := ElegirSkill(context.Background(), m, "consulta", catalogoDePrueba); !errors.Is(err, roto) {
		t.Fatalf("esperaba que el error del motor se propagara envuelto, obtuve %v", err)
	}
}

// Sin catálogo no se gasta una llamada al motor: no hay nada que elegir. Es error, no Eleccion{}
// silencioso, por el mismo motivo que Rerank sin candidatos.
func TestElegirSkillSinCatalogoNoLlamaAlMotor(t *testing.T) {
	m := &motorGrabador{respuesta: `{"skill":"go-hygiene","razon":"x"}`}
	if _, err := ElegirSkill(context.Background(), m, "consulta", nil); err == nil {
		t.Fatal("esperaba error con el catálogo vacío")
	}
	if n := m.llamadas.Load(); n != 0 {
		t.Fatalf("no debería haber llamado al motor, hubo %d llamadas", n)
	}
}

// La tolerancia del parseo es DELIBERADA y tiene su prueba, para que nadie la "arregle" creyendo
// que es laxitud. Los modelos envuelven el objeto en prosa o en fences aunque se les pida que no;
// rescatar el objeto de ahí es mejor que descartar una respuesta que sí sirve.
func TestElegirSkillToleraEnvoltorios(t *testing.T) {
	casos := map[string]string{
		"prosa antes y después": `Claro: {"skill":"go-hygiene","razon":"toca .go"} — listo.`,
		"con fences":            "```json\n{\"skill\":\"go-hygiene\",\"razon\":\"toca .go\"}\n```",
	}
	for nombre, resp := range casos {
		t.Run(nombre, func(t *testing.T) {
			m := &motorGrabador{respuesta: resp}
			elec, err := ElegirSkill(context.Background(), m, "arreglá recall.go", catalogoDePrueba)
			if err != nil {
				t.Fatalf("%s: no esperaba error: %v", nombre, err)
			}
			if elec.Skill != "go-hygiene" {
				t.Fatalf("%s: esperaba go-hygiene, obtuve %q", nombre, elec.Skill)
			}
		})
	}
}

// El prompt lleva TODAS las skills del catálogo con su nombre y qué hacen, y pide el formato de
// salida: si el motor no ve una skill, no la puede elegir.
func TestPromptDecididorLlevaTodoElCatalogo(t *testing.T) {
	system, user := PromptDecididor("mi tarea", catalogoDePrueba)
	if !strings.Contains(system, "objeto JSON") {
		t.Errorf("el system no pide el formato de salida: %q", system)
	}
	if !strings.Contains(user, "mi tarea") {
		t.Errorf("el user no lleva la tarea: %q", user)
	}
	for _, s := range catalogoDePrueba {
		if !strings.Contains(user, s.Name) || !strings.Contains(user, s.Description) {
			t.Errorf("el user no lleva a la skill %q con su descripción:\n%s", s.Name, user)
		}
	}
}
