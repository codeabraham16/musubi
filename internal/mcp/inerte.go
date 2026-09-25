package mcp

// Modo inerte: el servidor que arranca donde NO hay un proyecto de Musubi.
//
// Cuando Musubi se registra a nivel usuario (un plugin de Claude Code, no el .mcp.json de cada
// repo), el daemon arranca en CUALQUIER carpeta donde se abra una sesión: el home, Descargas, un
// directorio suelto. Ahí no tiene que crear una memoria —así nació la sombra de `~/.musubi` (A96)—,
// pero tampoco puede morirse: un servidor MCP que no arranca se ve, en el cliente, como Musubi roto.
//
// Así que habla el protocolo y no ofrece nada: tools/list vacío, toda tools/call rechazada con el
// motivo, y unas instrucciones de una línea para que el agente sepa por qué no tiene memoria y
// qué decirle a la persona. Es distinto del modo degradado: ahí la memoria EXISTE y no abrió; acá
// no hay proyecto, y eso es un estado normal.

import (
	"musubi/internal/embedding"
)

// NewServidorInerte arma el servidor para una carpeta sin proyecto. engine en nil a propósito, como
// en el degradado: nada de este servidor puede tocar una base.
func NewServidorInerte(projectPath, version, motivo string) *McpServer {
	s := NewMcpServer(nil, projectPath, embedding.NoopProvider{}, WithVersion(version))
	s.inerte = motivo
	return s
}

// instruccionesInerte es lo que el agente lee en su system prompt cuando Musubi está inerte.
func (s *McpServer) instruccionesInerte() string {
	return "Musubi no está activo en esta carpeta (" + s.inerte + "): en esta sesión no hay memoria ni tools de Musubi. " +
		"Musubi se activa solo al abrir la sesión dentro de un repo git; si la persona lo quiere acá, que corra `musubi init`."
}

// conAvisoDeInerte agrega las instrucciones de inerte al resultado de initialize.
func (s *McpServer) conAvisoDeInerte(res interface{}) interface{} {
	if s.inerte == "" {
		return res
	}
	if m, ok := res.(map[string]interface{}); ok {
		m["instructions"] = s.instruccionesInerte()
	}
	return res
}

// errorInerte es la respuesta a toda tools/call de un servidor inerte.
func (s *McpServer) errorInerte() *RpcError {
	return rpcErrorf(codeInvalidRequest, "Musubi no está activo en esta carpeta (%s): no hay memoria de proyecto. "+
		"Se activa solo dentro de un repo git, o con `musubi init`.", s.inerte)
}
