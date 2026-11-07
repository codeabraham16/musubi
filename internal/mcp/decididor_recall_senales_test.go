package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/skills"
)

// Invariantes de R3 DESPIERTO (spec r3-senales-al-recall). A diferencia de las de decididor_skills_test
// —que llaman a exigirSkillSiCorresponde DIRECTO con un reflejo armado a mano—, estas manejan el camino
// REAL de toolRecall: arman los args del tool (incluidas las señales nuevas), dejan que toolRecall
// cargue el arsenal y resuelva el reflejo, y recién ahí miden. Es la diferencia que convierte al corte
// de "testeado como unidad pero dormido en producción" en "ejercido por el camino que corre de verdad":
// el test del match-fuerte de decididor_skills_test custodiaba un branch que producción nunca ejecutaba
// (el "sitio equivocado" que caza la revisión adversarial).
//
// TODAS fuerzan el BRAZO ON con pidDeBrazo: es lo que vuelve NO vacua la aserción "0 llamadas". El gate
// corta por match-fuerte ANTES de calcular el brazo, pero el brazo off tampoco llama al motor (es el
// control), así que con un pid de brazo off el motor daría 0 sin importar el corte. Con el brazo on, 0
// llamadas ⟺ el corte actuó; 1 llamada ⟺ el gate dejó pasar.
//
// AISLAMIENTO: cada test usa una query DISTINTA a propósito. La caché del decididor
// (decididorCacheGet/Put) es de PAQUETE y su clave es (query + catálogo); dos tests que
// compartan query+arsenal se contaminan —el que llega al motor siembra la respuesta y el que
// corre después lee un hit y mide 0 cuando esperaba 1—. Con queries distintas cada test es
// hermético sin importar el orden ni si el gate de otro test llegó o no a sembrar la caché.

// recallConArgs maneja toolRecall con los args dados (serializados como los mandaría un cliente) y falla
// ante un RpcError: lo que medimos después es el contador del motor-espía, no la forma de la respuesta.
func recallConArgs(t *testing.T, s *McpServer, ctx context.Context, args map[string]any) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal de args: %v", err)
	}
	if _, rpcErr := s.toolRecall(ctx, json.RawMessage(raw)); rpcErr != nil {
		t.Fatalf("toolRecall devolvió error: %v", rpcErr)
	}
}

// R3-a — GLOB DECLARADO APAGA EL MOTOR. El llamador declara un archivo .go; el arsenal tiene una skill
// con trigger `*.go`. El reflejo model-free pina por glob (evidencia fuerte), el gate corta y el motor
// NO se gasta. Sabotaje: volver `reflejas=nil` en el call-site de toolRecall (TRAMO 3b) ⇒ el gate ve
// nil, deja pasar, el motor se llama ⇒ 1 llamada ⇒ rojo.
func TestR3GlobDeclaradoApagaElMotor(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaNinguna}
	s, _ := servidorDecididor(t, motor, 60)
	s.resolver = skills.NewResolverDeCatalogo([]skills.Skill{
		{Name: "go-hygiene", Description: "higiene de Go", Rules: "cuerpo cualquiera", Triggers: []string{"*.go"}},
	})
	ctx := ctxConProyecto("tester", pidDeBrazo(t, "on"))

	recallConArgs(t, s, ctx, map[string]any{
		"query":          "revisar errores tocando un archivo go del recall",
		"modified_files": []string{"internal/foo.go"},
	})

	if n := motor.llamadas.Load(); n != 0 {
		t.Errorf("un glob declarado que casa tiene que apagar el motor (match fuerte); hubo %d llamadas", n)
	}
}

// R3-b — ALCANCE DECLARADO APAGA EL MOTOR. El llamador declara la fase; el arsenal tiene una skill con
// ese alcance. El reflejo pina por alcance (la evidencia más fuerte), el gate corta, 0 llamadas.
// Sabotaje: igual que R3-a (reflejas=nil) ⇒ 1 llamada ⇒ rojo.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="res = s.exigirSkillSiCorresponde(ctx, args.Query, res, reflejas, arsenal)"
// arnes: a="res = s.exigirSkillSiCorresponde(ctx, args.Query, res, nil, arsenal)"
func TestR3AlcanceDeclaradoApagaElMotor(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaNinguna}
	s, _ := servidorDecididor(t, motor, 60)
	s.resolver = skills.NewResolverDeCatalogo([]skills.Skill{
		{Name: "sdd-flow", Description: "flujo SDD", Rules: "cuerpo", AppliesTo: []string{skills.FaseImplementar}},
	})
	ctx := ctxConProyecto("tester", pidDeBrazo(t, "on"))

	recallConArgs(t, s, ctx, map[string]any{
		"query": "implementar el cableado de las señales al recall",
		"phase": skills.FaseImplementar,
	})

	if n := motor.llamadas.Load(); n != 0 {
		t.Errorf("un alcance declarado que casa tiene que apagar el motor; hubo %d llamadas", n)
	}
}

// R3-c — SIN SEÑALES, EL MOTOR CORRE. Es la regresión del camino de texto libre: sin archivos ni fase,
// reflejas queda nil, el gate NO corta por match-fuerte y (en el brazo on) el motor se llama una vez.
// Sabotaje: hacer que el gate corte siempre (hayMatchFuerte→true) ⇒ 0 llamadas ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="return !hayMatchFuerte(reflejas)"
// arnes: a="return false"
func TestR3SinSenalesElMotorCorre(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaNinguna}
	s, _ := servidorDecididor(t, motor, 60)
	s.resolver = skills.NewResolverDeCatalogo([]skills.Skill{
		{Name: "go-hygiene", Description: "higiene de Go", Rules: "cuerpo", Triggers: []string{"*.go"}},
	})
	ctx := ctxConProyecto("tester", pidDeBrazo(t, "on"))

	recallConArgs(t, s, ctx, map[string]any{
		"query": "correr el motor del decididor sin ninguna senal",
	})

	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("sin señales declaradas el gate no corta por match-fuerte y el motor corre; hubo %d llamadas (esperaba 1)", n)
	}
}

// R3-d — SEÑAL QUE NO CASA, EL MOTOR CORRE. El llamador declara un .go pero la única skill dispara con
// `*.py`: ningún reflejo pina fuerte, el gate deja pasar y el motor corre. Es la contraparte de R3-a:
// prueba que el corte depende de que el trigger EFECTIVAMENTE case, no de que haya señales. Sabotaje:
// que el gate corte ante cualquier señal (sin exigir match) ⇒ 0 llamadas ⇒ rojo.
func TestR3SenalQueNoCasaElMotorCorre(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaNinguna}
	s, _ := servidorDecididor(t, motor, 60)
	s.resolver = skills.NewResolverDeCatalogo([]skills.Skill{
		{Name: "py-only", Description: "sólo Python", Rules: "cuerpo", Triggers: []string{"*.py"}},
	})
	ctx := ctxConProyecto("tester", pidDeBrazo(t, "on"))

	recallConArgs(t, s, ctx, map[string]any{
		"query":          "un glob de python que no casa el archivo go",
		"modified_files": []string{"internal/foo.go"},
	})

	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("un glob que NO casa no es match fuerte y el motor corre; hubo %d llamadas (esperaba 1)", n)
	}
}
