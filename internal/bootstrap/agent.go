package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// agent.go define los AGENTES soportados como destino de `musubi setup`. Generaliza
// el bootstrap (antes solo Claude Code) a otros agentes. El registro del servidor MCP
// usa el mismo esquema (`mcpServers`) para todos; solo cambia la RUTA del archivo de
// config. Los hooks (SessionStart/UserPromptSubmit/PreToolUse) son específicos de
// Claude Code: otros agentes registran el MCP pero no hooks (no tienen ese sistema).

// AgentTarget describe dónde y cómo se inyecta Musubi para un agente dado.
type AgentTarget struct {
	// Name es el identificador del agente (ej. "claude", "cursor").
	Name string
	// MCPPath es la ruta (relativa a la raíz del proyecto) del archivo de config MCP.
	MCPPath string
	// SupportsHooks indica si el agente tiene un sistema de hooks que Musubi inyecta.
	SupportsHooks bool
	// detectDir es un directorio cuya presencia sugiere que el agente se usa en el repo.
	detectDir string
}

// claudeTarget: Claude Code — .mcp.json en la raíz + hooks en .claude/settings.json.
func claudeTarget() AgentTarget {
	return AgentTarget{Name: "claude", MCPPath: ".mcp.json", SupportsHooks: true, detectDir: ".claude"}
}

// cursorTarget: Cursor — .cursor/mcp.json (mismo esquema mcpServers); sin hooks.
func cursorTarget() AgentTarget {
	return AgentTarget{Name: "cursor", MCPPath: filepath.Join(".cursor", "mcp.json"), SupportsHooks: false, detectDir: ".cursor"}
}

// knownAgents es el registro de agentes soportados.
func knownAgents() []AgentTarget {
	return []AgentTarget{claudeTarget(), cursorTarget()}
}

// ResolveAgent devuelve el AgentTarget para el nombre dado (case-insensitive).
// Nombre vacío → Claude (default histórico). Devuelve ok=false si es desconocido.
func ResolveAgent(name string) (AgentTarget, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return claudeTarget(), true
	}
	for _, a := range knownAgents() {
		if a.Name == n {
			return a, true
		}
	}
	return AgentTarget{}, false
}

// KnownAgentNames lista los nombres de agentes soportados (orden estable).
func KnownAgentNames() []string {
	var names []string
	for _, a := range knownAgents() {
		names = append(names, a.Name)
	}
	sort.Strings(names)
	return names
}

// DetectAgents devuelve los agentes cuyo directorio característico ya existe en root
// (ej. .cursor/). Sirve para sugerir destinos. Orden estable.
func DetectAgents(root string) []string {
	var found []string
	for _, a := range knownAgents() {
		if a.detectDir == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(root, a.detectDir)); err == nil && fi.IsDir() {
			found = append(found, a.Name)
		}
	}
	sort.Strings(found)
	return found
}
