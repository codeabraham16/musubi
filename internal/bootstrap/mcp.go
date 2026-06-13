// Package bootstrap implementa la inyección de Musubi en un proyecto:
// genera el archivo .mcp.json que hace que Claude Code cargue el servidor
// automáticamente al abrir el proyecto.
package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MCPServerEntry describe un servidor MCP stdio dentro de .mcp.json.
type MCPServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// MergeMCPServer agrega o reemplaza un servidor en el contenido de un .mcp.json.
// Si existing está vacío crea una estructura nueva. Preserva otros servidores
// y cualquier otra clave de nivel superior que ya exista.
func MergeMCPServer(existing []byte, name string, entry MCPServerEntry) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("error al parsear .mcp.json existente: %w", err)
		}
	}

	servers := map[string]json.RawMessage{}
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, fmt.Errorf("error al parsear mcpServers: %w", err)
		}
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("error al serializar entrada de servidor: %w", err)
	}
	servers[name] = entryBytes

	serversBytes, err := json.Marshal(servers)
	if err != nil {
		return nil, fmt.Errorf("error al serializar mcpServers: %w", err)
	}
	root["mcpServers"] = serversBytes

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error al serializar .mcp.json: %w", err)
	}
	return out, nil
}
