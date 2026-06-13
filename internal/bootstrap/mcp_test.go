package bootstrap

import (
	"encoding/json"
	"testing"
)

func parseServers(t *testing.T, data []byte) map[string]MCPServerEntry {
	t.Helper()
	var root struct {
		MCPServers map[string]MCPServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("salida no es JSON válido: %v\n%s", err, data)
	}
	return root.MCPServers
}

func TestMergeIntoEmpty(t *testing.T) {
	out, err := MergeMCPServer(nil, "musubi", MCPServerEntry{
		Command: "musubi.exe",
		Args:    []string{"daemon"},
		Env:     map[string]string{"MUSUBI_HOME": "/proj"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	servers := parseServers(t, out)
	if len(servers) != 1 || servers["musubi"].Command != "musubi.exe" {
		t.Fatalf("entrada musubi incorrecta: %+v", servers)
	}
	if servers["musubi"].Env["MUSUBI_HOME"] != "/proj" {
		t.Errorf("env MUSUBI_HOME no preservada: %+v", servers["musubi"].Env)
	}
}

func TestMergePreservesExistingServers(t *testing.T) {
	existing := []byte(`{"mcpServers":{"otro":{"command":"otro","args":["x"]}},"someKey":123}`)
	out, err := MergeMCPServer(existing, "musubi", MCPServerEntry{Command: "musubi.exe", Args: []string{"daemon"}})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	servers := parseServers(t, out)
	if len(servers) != 2 {
		t.Fatalf("esperaba 2 servidores (otro + musubi), obtuve %d", len(servers))
	}
	if servers["otro"].Command != "otro" {
		t.Error("se perdió el servidor existente 'otro'")
	}
	// Verificar que otras claves de nivel superior se preservan.
	var root map[string]json.RawMessage
	_ = json.Unmarshal(out, &root)
	if _, ok := root["someKey"]; !ok {
		t.Error("se perdió la clave de nivel superior 'someKey'")
	}
}

func TestMergeReplacesSameName(t *testing.T) {
	existing := []byte(`{"mcpServers":{"musubi":{"command":"viejo","args":[]}}}`)
	out, err := MergeMCPServer(existing, "musubi", MCPServerEntry{Command: "nuevo", Args: []string{"daemon"}})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	servers := parseServers(t, out)
	if servers["musubi"].Command != "nuevo" {
		t.Errorf("esperaba reemplazo a 'nuevo', obtuve %q", servers["musubi"].Command)
	}
}

func TestMergeInvalidExisting(t *testing.T) {
	if _, err := MergeMCPServer([]byte("{no es json"), "musubi", MCPServerEntry{}); err == nil {
		t.Fatal("esperaba error con .mcp.json inválido")
	}
}
