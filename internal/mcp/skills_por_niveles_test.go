package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Invariantes A7–A13 del spec «Niveles en musubi_resolve_skills» (specs/skills-por-niveles/).
//
// Lo que se sella acá es que el ahorro NO se paga en silencio: la skill sigue apareciendo, dice por
// qué apareció, dice que le falta el cuerpo, y trae el «cuándo» con el que se decide pedirlo.

const (
	skillPorGlob = `name: por-glob
description: Como revisar codigo Go en este repo
triggers:
  - "*.go"
rules: |
  Correr go vet antes de proponer cualquier cambio en Go.
`
	// La forma real de las 6 skills que el arsenal tiene con '*': su «cuándo» NO vive en la
	// description sino en always_because.
	skillComodin = `name: solo-comodin
description: Orquesta trabajo en paralelo
triggers:
  - "*"
always_because: 'se activa por la FORMA de la tarea (grande y paralelizable), no por el archivo'
rules: |
  Partir el trabajo en tareas independientes y lanzarlas juntas.
`
	skillOtroComodin = `name: otro-comodin
description: Planea antes de actuar
triggers:
  - "*"
always_because: 'planificar es una FASE del trabajo, no un tipo de archivo'
rules: |
  Recuperar contexto y armar un plan corto antes de tocar nada.
`
)

// arsenalDeNiveles deja las tres skills y devuelve el server.
func arsenalDeNiveles(t *testing.T) *McpServer {
	t.Helper()
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	escribirSkill(t, root, "por-glob.yaml", skillPorGlob)
	escribirSkill(t, root, "solo-comodin.yaml", skillComodin)
	escribirSkill(t, root, "otro-comodin.yaml", skillOtroComodin)
	return s
}

// resolverNiveles invoca la tool y devuelve la respuesta parseada.
func resolverNiveles(t *testing.T, s *McpServer, args map[string]interface{}) map[string]interface{} {
	t.Helper()
	out, rerr := call(t, s, "musubi_resolve_skills", args)
	if rerr != nil {
		t.Fatalf("musubi_resolve_skills fallo: %+v", rerr)
	}
	resp, ok := out.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %#v", out)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &m); err != nil {
		t.Fatalf("la respuesta no es JSON: %v\n%s", err, resp.Content[0].Text)
	}
	return m
}

func activas(t *testing.T, m map[string]interface{}) []map[string]interface{} {
	t.Helper()
	crudas, ok := m["active_skills"].([]interface{})
	if !ok {
		t.Fatalf("active_skills no es una lista: %#v", m["active_skills"])
	}
	out := make([]map[string]interface{}, 0, len(crudas))
	for _, c := range crudas {
		e, ok := c.(map[string]interface{})
		if !ok {
			t.Fatalf("entrada inesperada: %#v", c)
		}
		out = append(out, e)
	}
	return out
}

// ★ A7 — TODA SKILL MATCHEADA APARECE; LO QUE SE OMITE ES EL CUERPO, Y SE DECLARA.
//
// Una skill que desapareciera de la lista sería indistinguible de una que no matcheó: el peor modo
// de falla posible, el mismo que el vocabulario cerrado evita en applies_to.
func TestA7NingunaSkillDesapareceAlBajarElNivel(t *testing.T) {
	s := arsenalDeNiveles(t)
	args := map[string]interface{}{"modified_files": []string{"main.go"}}

	auto := activas(t, resolverNiveles(t, s, args))
	full := activas(t, resolverNiveles(t, s, map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "full",
	}))

	if len(auto) != len(full) {
		t.Fatalf("el nivel cambió CUÁNTAS skills se devuelven: auto=%d full=%d", len(auto), len(full))
	}
	if len(auto) != 3 {
		t.Fatalf("las tres tienen que matchear main.go (una por glob, dos por '*'); llegaron %d", len(auto))
	}

	var omitidas int
	for _, e := range auto {
		omitido, ok := e["body_omitted"].(bool)
		if !ok {
			t.Errorf("%v no declara body_omitted: la ausencia del cuerpo tiene que ser un dato explícito", e["name"])
			continue
		}
		if omitido {
			omitidas++
			if _, tiene := e["rules"]; tiene {
				t.Errorf("%v dice body_omitted pero trae rules", e["name"])
			}
		}
	}
	if omitidas != 2 {
		t.Errorf("las dos que entraron sólo por '*' tienen que venir sin cuerpo; omitidas=%d", omitidas)
	}
	if n, _ := m2f(auto[0]["rules_bytes"]); n == 0 && auto[0]["name"] == "por-glob" {
		t.Error("rules_bytes tiene que informar el peso del cuerpo esté incluido o no")
	}
}

// ★ A8 — EL NIVEL 1 NO QUEDA MUDO.
//
// Corolario del §4 de la investigación: para las skills con '*', el «cuándo» vive en always_because.
// Un nivel 1 sin él las deja mudas justo donde se decide si se cargan, y entonces el ahorro se paga
// en calidad — que es exactamente lo que no se quiere.
func TestA8ElNivel1TraeElCuando(t *testing.T) {
	s := arsenalDeNiveles(t)

	for _, e := range activas(t, resolverNiveles(t, s, map[string]interface{}{
		"modified_files": []string{"main.go"},
	})) {
		if omitido, _ := e["body_omitted"].(bool); !omitido {
			continue
		}
		cuando, _ := e["cuando"].(string)
		if strings.TrimSpace(cuando) == "" {
			t.Errorf("%v llegó sin cuerpo y sin «cuando»: no hay con qué decidir si pedirlo", e["name"])
		}
	}
}

// A9 — LA RESPUESTA DICE CUÁNTOS CUERPOS FALTAN Y CÓMO PEDIRLOS.
func TestA9SeDiceCuantoFaltaYComoPedirlo(t *testing.T) {
	s := arsenalDeNiveles(t)
	m := resolverNiveles(t, s, map[string]interface{}{"modified_files": []string{"main.go"}})

	n, ok := m2f(m["bodies_omitted"])
	if !ok || n != 2 {
		t.Errorf("bodies_omitted = %v, se esperaban 2", m["bodies_omitted"])
	}
	hint, _ := m["hint"].(string)
	if !strings.Contains(hint, "musubi_list_skills") {
		t.Errorf("el hint tiene que nombrar la tool que trae el cuerpo; hint = %q", hint)
	}

	// Y cuando no falta nada, no se dice nada.
	sinFaltantes := resolverNiveles(t, s, map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "full",
	})
	if _, hay := sinFaltantes["hint"]; hay {
		t.Error("con todos los cuerpos incluidos no corresponde un hint")
	}
}

// A10 — detail:"full" DEVUELVE TODOS LOS CUERPOS. Es la red para quien dependiera de lo de antes.
func TestA10FullDevuelveTodosLosCuerpos(t *testing.T) {
	s := arsenalDeNiveles(t)

	for _, e := range activas(t, resolverNiveles(t, s, map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "full",
	})) {
		if omitido, _ := e["body_omitted"].(bool); omitido {
			t.Errorf("con detail=full, %v no puede venir sin cuerpo", e["name"])
		}
		if r, _ := e["rules"].(string); strings.TrimSpace(r) == "" {
			t.Errorf("con detail=full, %v tiene que traer rules", e["name"])
		}
	}
}

// A11 — detail:"summary" NO DEVUELVE NINGUNO, ni siquiera los que tienen evidencia.
func TestA11SummaryNoDevuelveNingunCuerpo(t *testing.T) {
	s := arsenalDeNiveles(t)
	m := resolverNiveles(t, s, map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "summary",
	})

	lista := activas(t, m)
	for _, e := range lista {
		if _, tiene := e["rules"]; tiene {
			t.Errorf("con detail=summary, %v no puede traer rules", e["name"])
		}
	}
	if n, _ := m2f(m["bodies_omitted"]); int(n) != len(lista) {
		t.Errorf("bodies_omitted = %v con %d skills: en summary se omiten todos", m["bodies_omitted"], len(lista))
	}
}

// A12 — UN detail INVÁLIDO ES ERROR, no un default silencioso.
//
// Precedente G6 de musubi_list_skills: degradar un typo en silencio produce una respuesta que se lee
// como un dato y no como una falla.
func TestA12DetailInvalidoEsError(t *testing.T) {
	s := arsenalDeNiveles(t)

	_, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "summry",
	})
	if rerr == nil {
		t.Fatal("un detail con typo tiene que fallar, no caer al default")
	}
	if rerr.Code != codeInvalidParams {
		t.Errorf("code = %d, se esperaba codeInvalidParams (%d)", rerr.Code, codeInvalidParams)
	}
	if !strings.Contains(rerr.Message, "auto") {
		t.Errorf("el error tiene que decir qué SÍ se acepta; mensaje = %q", rerr.Message)
	}
}

// ★ A13 — LAS CLAVES SON snake_case, NO LOS NOMBRES DE CAMPO DE Go.
//
// skills.Skill tiene sólo tags YAML: serializarla directo —lo que esta tool hacía— emite "Name",
// "AppliesTo" y filtra ManagedChecksum y GeneratedAt. Un cliente que parsea en minúscula no recibe
// un error: recibe una lista del largo correcto con todo vacío.
func TestA13LasClavesNoSonLosCamposDeGo(t *testing.T) {
	s := arsenalDeNiveles(t)
	out, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
		"modified_files": []string{"main.go"}, "detail": "full",
	})
	if rerr != nil {
		t.Fatalf("musubi_resolve_skills fallo: %+v", rerr)
	}
	txt := out.(CallToolResponse).Content[0].Text

	for _, clave := range []string{`"name"`, `"description"`, `"cuando"`, `"triggers"`,
		`"capabilities"`, `"matched_by"`, `"rules_bytes"`, `"body_omitted"`} {
		if !strings.Contains(txt, clave) {
			t.Errorf("falta la clave %s en la respuesta", clave)
		}
	}
	for _, fuga := range []string{`"Name"`, `"Description"`, `"AppliesTo"`, `"Triggers"`,
		`"ManagedChecksum"`, `"managed_checksum"`, `"GeneratedAt"`, `"generated_at"`} {
		if strings.Contains(txt, fuga) {
			t.Errorf("la respuesta filtra %s: es un nombre de campo de Go o un metadato de disco", fuga)
		}
	}
}

// m2f desempaqueta un número de JSON (que llega como float64).
func m2f(v interface{}) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}
