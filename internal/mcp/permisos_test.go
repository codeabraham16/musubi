package mcp

import (
	"strings"
	"testing"
)

// PREGUNTA LO QUE ACTÚA FUERA DE LA MEMORIA, Y NADA MÁS.
//
// `musubi agente instalar` permite el servidor entero y deja en «preguntar» esta lista. Si quedara
// afuera una tool que ejecuta en otra máquina, el agente la correría sin el sí de la persona; si
// entrara una de memoria, Musubi volvería a pedir confirmación en cada llamada. Los nombres se
// escriben literales: son el contrato con quien la usa.
//
// Sabotaje que la hace fallar: preguntar también por lo que sólo lee.
// arnes: archivo="internal/mcp/permisos.go"
// arnes: de="\tif e.readOnly {\n\t\treturn false\n\t}\n"
// arnes: a="\tif false && e.readOnly {\n\t\treturn false\n\t}\n"
//
// Sabotaje que la hace fallar: olvidar las credenciales.
// arnes: archivo="internal/mcp/permisos.go"
// arnes: de="var prefijosQueActuanAfuera = []string{\"musubi_fleet_\", \"musubi_token_\"}\n"
// arnes: a="var prefijosQueActuanAfuera = []string{\"musubi_fleet_\"}\n"
func TestPreguntaLoQueActuaFueraDeLaMemoria(t *testing.T) {
	preguntan := map[string]bool{}
	for _, n := range ToolsQuePreguntan() {
		preguntan[n] = true
	}
	for _, n := range []string{"musubi_fleet_exec", "musubi_fleet_shell", "musubi_fleet_screen",
		"musubi_fleet_rotate", "musubi_fleet_revoke", "musubi_fleet_enroll", "musubi_token_new", "musubi_token_revoke"} {
		if !preguntan[n] {
			t.Errorf("%s actúa fuera de la memoria y quedaría permitida sin preguntar", n)
		}
	}
	for _, n := range []string{"musubi_recall", "musubi_save_observation", "musubi_propose_observation",
		"musubi_corroborate", "musubi_discard_proposal", "musubi_work", "musubi_impact",
		"musubi_fleet_list", "musubi_fleet_log", "musubi_fleet_metrics"} {
		if preguntan[n] {
			t.Errorf("%s no actúa fuera de la memoria y pediría confirmación en cada llamada", n)
		}
	}
	for n := range preguntan {
		if !strings.HasPrefix(n, "musubi_fleet_") && !strings.HasPrefix(n, "musubi_token_") {
			t.Errorf("%s pregunta y no es de flota ni de credenciales", n)
		}
	}
}
