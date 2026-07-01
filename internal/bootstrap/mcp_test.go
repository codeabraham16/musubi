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

func TestRemoteEntryTokenPorEnv(t *testing.T) {
	e := RemoteEntry("https://box.tailnet:7717/mcp", "MUSUBI_TOKEN")
	if e.Type != "http" || e.URL != "https://box.tailnet:7717/mcp" {
		t.Fatalf("entrada remota mal formada: %+v", e)
	}
	// El secreto NO va en el archivo: solo una referencia ${ENV} que el cliente expande.
	if got := e.Headers["Authorization"]; got != "Bearer ${MUSUBI_TOKEN}" {
		t.Errorf("header Authorization = %q, quería referencia por env", got)
	}
	// Sin tokenEnv no hay headers (bind loopback/confiable).
	if bare := RemoteEntry("http://127.0.0.1:7717/mcp", ""); len(bare.Headers) != 0 {
		t.Errorf("sin tokenEnv no debería haber headers, obtuve %+v", bare.Headers)
	}
}

func TestMergeRemotePreservaYReemplaza(t *testing.T) {
	// Cerebro remoto conviviendo con un servidor stdio existente.
	existing := []byte(`{"mcpServers":{"local":{"command":"musubi.exe","args":["daemon"]}}}`)
	out, err := MergeRemoteMCPServer(existing, "musubi-brain", RemoteEntry("https://box:7717/mcp", "MUSUBI_TOKEN"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var root struct {
		MCPServers map[string]map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatalf("salida inválida: %v\n%s", err, out)
	}
	if len(root.MCPServers) != 2 {
		t.Fatalf("esperaba 2 servidores (local + musubi-brain), obtuve %d", len(root.MCPServers))
	}
	brain := root.MCPServers["musubi-brain"]
	if string(brain["type"]) != `"http"` {
		t.Errorf("el cerebro remoto debería ser type http, obtuve %s", brain["type"])
	}
	if _, ok := root.MCPServers["local"]["command"]; !ok {
		t.Error("se perdió el servidor stdio 'local'")
	}
}
