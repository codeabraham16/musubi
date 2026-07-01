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

// RemoteMCPServerEntry describe un servidor MCP REMOTO (transporte HTTP) dentro de
// .mcp.json: apunta el cliente al "cerebro central" (un `musubi serve` en el servidor
// casero, sobre la malla VPN privada) en vez de a un binario local. Como el daemon
// remoto sirve TODAS las tools, un cliente así obtiene memoria Y orquestación
// COMPARTIDAS entre máquinas (S2 + S3) sin motor nuevo: es pura configuración.
type RemoteMCPServerEntry struct {
	Type    string            `json:"type"` // "http"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// RemoteEntry construye la entrada remota para url. Si tokenEnv != "", inyecta el
// bearer token por REFERENCIA a una variable de entorno (`${VAR}`, que el cliente MCP
// expande), de modo que el secreto nunca toca el archivo —igual que Musubi hace con
// el YAML—.
func RemoteEntry(url, tokenEnv string) RemoteMCPServerEntry {
	e := RemoteMCPServerEntry{Type: "http", URL: url}
	if tokenEnv != "" {
		e.Headers = map[string]string{"Authorization": "Bearer ${" + tokenEnv + "}"}
	}
	return e
}

// MergeMCPServer agrega o reemplaza un servidor STDIO en el contenido de un .mcp.json.
// Si existing está vacío crea una estructura nueva. Preserva otros servidores
// y cualquier otra clave de nivel superior que ya exista.
func MergeMCPServer(existing []byte, name string, entry MCPServerEntry) ([]byte, error) {
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("error al serializar entrada de servidor: %w", err)
	}
	return mergeServerRaw(existing, name, entryBytes)
}

// MergeRemoteMCPServer agrega o reemplaza un servidor REMOTO (HTTP) en un .mcp.json,
// con la misma semántica de preservación que MergeMCPServer.
func MergeRemoteMCPServer(existing []byte, name string, entry RemoteMCPServerEntry) ([]byte, error) {
	if entry.Type == "" {
		entry.Type = "http"
	}
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("error al serializar entrada remota: %w", err)
	}
	return mergeServerRaw(existing, name, entryBytes)
}

// mergeServerRaw inserta entryJSON bajo mcpServers[name] preservando el resto del
// documento. Es el núcleo compartido por las variantes stdio y remota.
func mergeServerRaw(existing []byte, name string, entryJSON []byte) ([]byte, error) {
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

	servers[name] = entryJSON

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
