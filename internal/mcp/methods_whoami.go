package mcp

import (
	"context"
	"encoding/json"
)

// toolWhoami devuelve la IDENTIDAD del principal que hace la llamada: nombre
// (persona), proyecto y capacidades efectivas (read: own|all, write:
// none|own|any). Es read-only y TODO principal puede invocarla (incluida la
// cabina write=none): es el "¿quién soy?" del cerebro de empresa.
//
// La usa el cuerpo (musubi-body) tras el gate de conexión para mostrar "conectado
// como <persona>". La identidad se deriva SIEMPRE server-side del principal
// autenticado por el token (authorFrom) — nunca del cliente. En stdio local (sin
// principal: confianza local) reporta authenticated=false y acceso local pleno.
func (s *McpServer) toolWhoami(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if p == nil {
		return jsonResult(map[string]interface{}{
			"authenticated": false,
			"name":          "",
			"project_id":    "",
			"role":          "",
			"read":          ReadAll,
			"write":         WriteAny,
			"scope":         "local",
		})
	}
	read, write := p.caps()
	return jsonResult(map[string]interface{}{
		"authenticated": true,
		"name":          authorFrom(p), // "" para el admin legacy (token único sin persona)
		"project_id":    p.ProjectID,
		"role":          p.Role,
		"read":          read,
		"write":         write,
		"scope":         "central",
	})
}
