package mcp

import (
	"sort"
	"strings"
)

// permisos.go — qué tools de Musubi quedan en «preguntar» cuando `musubi agente instalar` le da
// permiso a Claude Code para usar Musubi sin pedir confirmación.
//
// El instalador permite el servidor ENTERO —memoria, código, tareas, la flota en lectura—, porque
// el agente tiene que poder usar Musubi sin que la persona confirme cada llamada. Medido el
// 2026-09-26 sobre 25 sesiones: el clasificador del modo auto bloqueó tres escrituras de memoria
// (propose_observation y save_observation), y en modo normal CADA llamada pide confirmación.
//
// Menos lo que ACTÚA FUERA de la memoria del proyecto: sobre otra máquina de la flota, o sobre
// credenciales. Eso queda en «preguntar», que es la regla de oro de la sala de mando escrita como
// permiso: un cambio en una máquina ajena se ejecuta con el sí de la persona, en el momento. Una
// regla «ask» le gana a una «allow» del mismo servidor (deny > ask > allow, medido en el binario de
// Claude Code 2.1.283), así que las dos conviven.

// prefijosQueActuanAfuera son los dominios de tools que tocan algo fuera de esta memoria: las
// máquinas de la flota y las credenciales.
var prefijosQueActuanAfuera = []string{"musubi_fleet_", "musubi_token_"}

// actuaAfuera dice si una tool actúa fuera de la memoria del proyecto.
//
// SE DERIVA del nombre y de readOnly, no se marca tool por tool: una tool de flota o de credenciales
// que no es de sólo lectura. Una tool de flota nueva que escriba entra sola; una de lectura
// (fleet_list, fleet_log) no pregunta, porque mirar no cambia nada allá.
func (e toolEntry) actuaAfuera() bool {
	if e.readOnly {
		return false
	}
	for _, p := range prefijosQueActuanAfuera {
		if strings.HasPrefix(e.Name, p) {
			return true
		}
	}
	return false
}

// ToolsQuePreguntan devuelve, ordenadas, las tools que actúan fuera de la memoria del proyecto: las
// que el instalador deja en «preguntar». Incluye las dormidas: siguen siendo invocables por nombre.
func ToolsQuePreguntan() []string {
	s := NewMcpServer(nil, "", nil)
	var out []string
	for _, e := range s.tools {
		if e.actuaAfuera() {
			out = append(out, e.Name)
		}
	}
	sort.Strings(out)
	return out
}
