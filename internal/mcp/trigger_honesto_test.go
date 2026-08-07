package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// trigger_honesto_test.go sella el contrato del spec «trigger-honesto» del lado de las puertas:
// la promoción al arsenal compartido y el viaje del campo always_because por la red.
//
// El invariante que más importa es G7. Hasta este spec, promote era la ÚNICA de las cuatro puertas
// sin gate de calidad —save_skill, author_skill e install_skill lo corrían, y justo la que publica
// para todos no—. Una skill escrita a mano, que nunca pasó por ninguna tool, subía tal cual al
// arsenal de todos los proyectos.

// razonTH es una justificación que supera el piso de skills.MinAlwaysBecauseChars.
const razonTH = "se activa por tipo de tarea (planificar), no por archivo"

// skillSiempre es una skill local con trigger '*' y una razón declarada.
const skillSiempre = `name: planificar
description: Planifica antes de actuar. Use when se arranca una tarea de varios pasos.
triggers:
  - "*"
always_because: ` + razonTH + `
rules: |
  Escribir el plan antes de tocar codigo.

  ` + "```sh\n  echo plan > PLAN.md\n  ```" + `
`

// skillSiempreSinRazon es la misma pero sin declarar por qué se activa siempre.
const skillSiempreSinRazon = `name: planificar
description: Planifica antes de actuar. Use when se arranca una tarea de varios pasos.
triggers:
  - "*"
rules: |
  Escribir el plan antes de tocar codigo.

  ` + "```sh\n  echo plan > PLAN.md\n  ```" + `
`

// promover invoca musubi_promote_skill y devuelve el error RPC (nil si salió bien).
func promover(t *testing.T, s *McpServer, nombre string) *RpcError {
	t.Helper()
	_, rerr := call(t, s, "musubi_promote_skill", map[string]interface{}{"name": nombre})
	return rerr
}

// TestTHElCampoCruzaLaRedEntera (G2) — promote → central → list → install. Son cuatro sobres y
// cada uno puede tirar el campo EN SILENCIO: serializar con los tags equivocados no falla,
// guarda vacío. Es la trampa que ya documenta skillPayload.
func TestTHElCampoCruzaLaRedEntera(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "planificar.yaml", skillSiempre)

	// 1 · Subida: lo que sale por el cable.
	if rerr := promover(t, s, "planificar"); rerr != nil {
		t.Fatalf("promover una skill declarada debe funcionar: %+v", rerr)
	}
	central.mu.Lock()
	crudo := central.args
	central.mu.Unlock()
	var recibido map[string]interface{}
	if err := json.Unmarshal(crudo, &recibido); err != nil {
		t.Fatalf("argumentos ilegibles: %v", err)
	}
	if recibido["always_because"] != razonTH {
		t.Fatalf("la razón no llegó al central: %#v", recibido["always_because"])
	}

	// 2 · Catálogo: lo que se ve del otro lado. Se usa OTRO proyecto para que la entrada
	// venga del arsenal y no del disco local.
	central2 := &centralFalso{arsenal: []skillPayload{{
		Name: "planificar", Description: "Planifica antes de actuar. Use when arranca una tarea.",
		Triggers: []string{"*"}, Rules: "Escribir el plan.\n\n```sh\necho plan\n```",
		AlwaysBecause: razonTH,
	}}}
	s2, root2 := servidorConCentral(t, central2)

	lista := listarArsenal(t, s2, map[string]interface{}{"source": fuenteCentral})
	e := porNombre(lista, "planificar")
	if e == nil {
		t.Fatal("la skill no apareció en el catálogo")
	}
	if e.AlwaysBecause != razonTH {
		t.Errorf("el catálogo no muestra la razón: %q — sin eso no se puede decidir antes de instalar", e.AlwaysBecause)
	}

	// 3 · Bajada: lo que queda escrito en el proyecto.
	if _, rerr := call(t, s2, "musubi_install_skill", map[string]interface{}{"name": "planificar"}); rerr != nil {
		t.Fatalf("install falló: %+v", rerr)
	}
	body, err := os.ReadFile(rutaSkill(root2, "planificar"))
	if err != nil {
		t.Fatalf("no se escribió la skill: %v", err)
	}
	if !strings.Contains(string(body), "always_because:") || !strings.Contains(string(body), razonTH) {
		t.Errorf("la skill instalada perdió la razón:\n%s", body)
	}
}

// TestTHLaPuertaDeRecepcionNoTiraLaRazon (G2, cuarto sobre) — éste es el lado que corre EN EL
// CENTRAL: PushSkill llama a musubi_save_skill allá. El test de arriba verifica lo que sale por el
// cable; éste, que lo que entra se guarde. Sin él, el campo podría viajar perfecto y morir al
// aterrizar —y como el central corre otro binario, el síntoma aparecería en otra máquina.
func TestTHLaPuertaDeRecepcionNoTiraLaRazon(t *testing.T) {
	s, root := servidorConCentral(t, nil)

	if _, rerr := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":           "planificar",
		"description":    "Planifica antes de actuar. Use when se arranca una tarea de varios pasos.",
		"triggers":       []string{"*"},
		"always_because": razonTH,
		"rules":          "Escribir el plan antes de tocar codigo.\n\n```sh\necho plan\n```",
	}); rerr != nil {
		t.Fatalf("save_skill falló: %+v", rerr)
	}

	body, err := os.ReadFile(rutaSkill(root, "planificar"))
	if err != nil {
		t.Fatalf("no se escribió la skill: %v", err)
	}
	if !strings.Contains(string(body), razonTH) {
		t.Errorf("la puerta de recepción tiró la razón:\n%s", body)
	}
}

// TestTHLoQueNoPasaElGateNoSube (G7) — y no basta con devolver error: se cuenta que NO haya
// llamada al central. Un rechazo que igual hace el POST no es un rechazo.
func TestTHLoQueNoPasaElGateNoSube(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	// 'claude' en el name es un error duro del gate (name_reserved).
	escribirSkill(t, root, "usar-claude.yaml", `name: usar-claude
description: Hace cosas. Use when hay que hacer cosas.
triggers:
  - "*.go"
rules: |
  Hacer las cosas con cuidado y verificarlas.
`)

	rerr := promover(t, s, "usar-claude")
	if rerr == nil {
		t.Fatal("una skill que no pasa el gate de calidad NO debe subir al arsenal")
	}
	central.mu.Lock()
	tool := central.tool
	central.mu.Unlock()
	if tool != "" {
		t.Errorf("el rechazo igual salió a la red (llamó a %q)", tool)
	}
}

// TestTHElWildcardSinDeclararNoSube (G8) — y el mensaje tiene que decir cómo arreglarlo.
func TestTHElWildcardSinDeclararNoSube(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "planificar.yaml", skillSiempreSinRazon)

	rerr := promover(t, s, "planificar")
	if rerr == nil {
		t.Fatal("un '*' sin declarar no debe entrar al arsenal compartido")
	}
	if !strings.Contains(rerr.Message, "always_because") {
		t.Errorf("el error debe nombrar el campo que lo resuelve: %q", rerr.Message)
	}
	central.mu.Lock()
	tool := central.tool
	central.mu.Unlock()
	if tool != "" {
		t.Errorf("el rechazo igual salió a la red (llamó a %q)", tool)
	}
}

// TestTHElWildcardDeclaradoSubeSinFriccion (G9) — control de G8. Sin esto, romper la tool entera
// dejaría el test del rechazo en verde: todo fallaría, incluido lo que debe funcionar.
func TestTHElWildcardDeclaradoSubeSinFriccion(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "planificar.yaml", skillSiempre)

	if rerr := promover(t, s, "planificar"); rerr != nil {
		t.Fatalf("con la razón declarada la promoción debe funcionar: %+v", rerr)
	}
	central.mu.Lock()
	tool := central.tool
	central.mu.Unlock()
	if tool != "musubi_save_skill" {
		t.Errorf("esperaba una promoción real al central, tool=%q", tool)
	}
}

// TestTHLoAcotadoPromueveIgualQueAntes (G10) — la mayoría del arsenal está acotada y no debe
// pagar por un problema que no tiene. Es el control de que el gate nuevo no rompió la puerta.
func TestTHLoAcotadoPromueveIgualQueAntes(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	if rerr := promover(t, s, "revisar-go"); rerr != nil {
		t.Fatalf("una skill acotada debe promoverse sin fricción: %+v", rerr)
	}
	central.mu.Lock()
	crudo := central.args
	central.mu.Unlock()
	// Y sin always_because: el campo es omitempty y no debe ensuciar lo que no lo necesita.
	if strings.Contains(string(crudo), "always_because") {
		t.Errorf("una skill acotada no debe mandar always_because: %s", crudo)
	}
}
