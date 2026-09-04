package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// rotacionVentanaDefault es cuánto tiempo se le ofrece el token nuevo al agente.
//
// 24 horas y no diez minutos: el agente puede estar apagado, o ser un portátil que se enciende a
// la mañana. Una ventana corta convertiría «esa máquina estaba apagada» en «la rotación falló», y
// el operador tendría que perseguir máquinas en vez de pedir una rotación y olvidarse.
//
// Que sea larga no afloja nada: durante la ventana valen los DOS tokens, y el viejo ya valía. Lo
// que la ventana acota es cuánto tiempo el cerebro guarda un secreto en memoria.
const rotacionVentanaDefault = 24 * time.Hour

// toolFleetRotate abre la rotación del token de una máquina.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ES ADMIN, Y NO ALCANZA CON `metrics` NI CON `exec`
//
// Rotar cambia la credencial con la que una máquina se autentica: quien puede hacerlo puede
// dejar afuera a un agente si la operación sale mal. Es la misma clase de acto que enrolar o
// revocar, y esos ya son admin.
//
// LO QUE ESTA TOOL NO ES: la herramienta de la emergencia. Si el token SE FILTRÓ, lo que
// corresponde es `musubi_fleet_revoke`, que es instantáneo y no depende de que el agente
// coopere. Rotar es higiene, y por eso puede permitirse esperar a que el agente conteste.
// ════════════════════════════════════════════════════════════════════════════════════════════
func (s *McpServer) toolFleetRotate(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized,
			"musubi_fleet_rotate cambia la credencial con la que una máquina se autentica: requiere un principal admin. "+
				"Tener `metrics` o `exec` sobre ella no alcanza — quien rota puede dejar afuera al agente si la operación sale mal.")
	}
	var args struct {
		Device  string `json:"device"`
		Project string `json:"project"`
		Horas   int    `json:"horas"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`")
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if !existe {
		return nil, rpcErrorf(codeInvalidParams, "no hay una máquina %q en el proyecto %q", nombre, proyecto)
	}

	ventana := rotacionVentanaDefault
	if args.Horas > 0 {
		ventana = time.Duration(args.Horas) * time.Hour
	}
	vence := time.Now().Add(ventana)

	token, err := s.engine.AbrirRotacion(d.ID, vence)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// EN MEMORIA, para poder repetirlo en cada latido hasta que el agente lo use. La base sólo
	// guarda su hash: un volcado de la base no puede ser un llavero (misma regla que A74).
	s.recordarRotacion(d.ID, token, vence)

	return jsonResult(map[string]interface{}{
		"device":     d.Name,
		"project_id": d.ProjectID,
		"vence":      vence.UTC().Format(time.RFC3339),
		"token":      token,
		"nota": "los DOS tokens valen hasta que el agente late con el nuevo, o hasta que venza la ventana. " +
			"El agente lo recibe solo en su próximo latido y no hay que tocar la máquina. " +
			"SI EL CEREBRO SE REINICIA la rotación se pierde y hay que volver a pedirla: el token nuevo vive en memoria " +
			"a propósito, para que la base no sea un llavero. El token VIEJO sigue valiendo mientras tanto, así que la " +
			"máquina nunca queda afuera. Si lo que pasó es que el token se FILTRÓ, esto no es lo que querés: usá musubi_fleet_revoke.",
	})
}
