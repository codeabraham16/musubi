package bootstrap

import (
	"encoding/json"
	"strings"
	"testing"
)

// parsearHooksSessionStart decodifica el output de MergeClaudeSettings y extrae
// las entradas de hooks.SessionStart para verificación.
func parsearHooksSessionStart(t *testing.T, data []byte) []sessionStartEntry {
	t.Helper()
	var root struct {
		Hooks struct {
			SessionStart []sessionStartEntry `json:"SessionStart"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("output no es JSON válido: %v\n%s", err, data)
	}
	return root.Hooks.SessionStart
}

// TestMergeClaudeSettingsVacio verifica que un input vacío produce la estructura
// con hooks.SessionStart conteniendo la nueva entrada.
func TestMergeClaudeSettingsVacio(t *testing.T) {
	hook := HookCommand{Type: "command", Command: "/usr/bin/musubi detect --hook-mode", Timeout: 10}
	out, err := MergeClaudeSettings(nil, "startup", hook)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	entradas := parsearHooksSessionStart(t, out)
	if len(entradas) != 1 {
		t.Fatalf("esperaba 1 entrada en SessionStart, obtuve %d", len(entradas))
	}
	if len(entradas[0].Hooks) != 1 {
		t.Fatalf("esperaba 1 hook, obtuve %d", len(entradas[0].Hooks))
	}
	if entradas[0].Hooks[0].Command != hook.Command {
		t.Errorf("Command incorrecto: %q", entradas[0].Hooks[0].Command)
	}
}

// TestMergeClaudeSettingsPreservaHooksExistentes verifica que otros hooks en
// SessionStart se preservan al agregar uno nuevo.
func TestMergeClaudeSettingsPreservaHooksExistentes(t *testing.T) {
	// JSON inicial con un hook pre-existente de otra herramienta.
	existing := []byte(`{
		"hooks": {
			"SessionStart": [
				{"matcher": "startup", "hooks": [{"type": "command", "command": "otro-tool"}]}
			]
		}
	}`)
	hook := HookCommand{Type: "command", Command: "/bin/musubi detect --hook-mode", Timeout: 10}
	out, err := MergeClaudeSettings(existing, "startup", hook)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	entradas := parsearHooksSessionStart(t, out)
	if len(entradas) != 1 {
		t.Fatalf("esperaba 1 entrada de matcher, obtuve %d", len(entradas))
	}
	if len(entradas[0].Hooks) != 2 {
		t.Fatalf("esperaba 2 hooks (original + nuevo), obtuve %d", len(entradas[0].Hooks))
	}
	// Verificar que el hook original sigue presente.
	encontradoOriginal := false
	encontradoNuevo := false
	for _, h := range entradas[0].Hooks {
		if h.Command == "otro-tool" {
			encontradoOriginal = true
		}
		if h.Command == hook.Command {
			encontradoNuevo = true
		}
	}
	if !encontradoOriginal {
		t.Error("se perdió el hook original 'otro-tool'")
	}
	if !encontradoNuevo {
		t.Error("no se encontró el hook nuevo de musubi")
	}
}

// TestMergeClaudeSettingsIdempotente verifica que agregar un hook cuyo Command
// ya está presente no duplica la entrada.
func TestMergeClaudeSettingsIdempotente(t *testing.T) {
	hook := HookCommand{Type: "command", Command: "/bin/musubi detect --hook-mode", Timeout: 10}
	// Primera inserción.
	out1, err := MergeClaudeSettings(nil, "startup", hook)
	if err != nil {
		t.Fatalf("primera inserción: %v", err)
	}
	// Segunda inserción (misma operación sobre el resultado previo).
	out2, err := MergeClaudeSettings(out1, "startup", hook)
	if err != nil {
		t.Fatalf("segunda inserción: %v", err)
	}
	entradas := parsearHooksSessionStart(t, out2)
	if len(entradas) != 1 {
		t.Fatalf("esperaba 1 entrada de matcher, obtuve %d", len(entradas))
	}
	if len(entradas[0].Hooks) != 1 {
		t.Fatalf("esperaba 1 hook (idempotente), obtuve %d: %+v", len(entradas[0].Hooks), entradas[0].Hooks)
	}
}

// TestMergeClaudeSettingsSalidaEsJSONValido verifica que el output siempre es JSON válido.
func TestMergeClaudeSettingsSalidaEsJSONValido(t *testing.T) {
	hook := HookCommand{Type: "command", Command: "/bin/musubi detect --hook-mode"}
	out, err := MergeClaudeSettings([]byte(`{}`), "startup", hook)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var dest map[string]json.RawMessage
	if err := json.Unmarshal(out, &dest); err != nil {
		t.Fatalf("output no es JSON válido: %v\n%s", err, out)
	}
}

// TestMergeClaudeSettingsPreservaClavesSuperiores verifica que claves de nivel
// superior ajenas a hooks se conservan en el output.
func TestMergeClaudeSettingsPreservaClavesSuperiores(t *testing.T) {
	existing := []byte(`{"permissions":{"allow":["read"]},"hooks":{}}`)
	hook := HookCommand{Type: "command", Command: "/bin/musubi detect --hook-mode"}
	out, err := MergeClaudeSettings(existing, "startup", hook)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(string(out), `"permissions"`) {
		t.Error("se perdió la clave de nivel superior 'permissions'")
	}
}

// TestMergeClaudeSettingsJSONInvalido verifica que un input malformado retorna error.
func TestMergeClaudeSettingsJSONInvalido(t *testing.T) {
	_, err := MergeClaudeSettings([]byte("{no es json"), "startup", HookCommand{Command: "x"})
	if err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}
