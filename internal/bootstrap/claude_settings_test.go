package bootstrap

import (
	"encoding/json"
	"strings"
	"testing"
)

// parsearHooksEvento decodifica el output de MergeClaudeSettings y extrae las
// entradas de hooks.<event> para verificación.
func parsearHooksEvento(t *testing.T, data []byte, event string) []hookEntry {
	t.Helper()
	var root struct {
		Hooks map[string][]hookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("output no es JSON válido: %v\n%s", err, data)
	}
	return root.Hooks[event]
}

// parsearHooksSessionStart es un atajo para el evento SessionStart.
func parsearHooksSessionStart(t *testing.T, data []byte) []hookEntry {
	t.Helper()
	return parsearHooksEvento(t, data, "SessionStart")
}

// TestMergeClaudeSettingsVacio verifica que un input vacío produce la estructura
// con hooks.SessionStart conteniendo la nueva entrada.
func TestMergeClaudeSettingsVacio(t *testing.T) {
	hook := HookCommand{Type: "command", Command: "/usr/bin/musubi detect --hook-mode", Timeout: 10}
	out, err := MergeClaudeSettings(nil, "SessionStart", "startup", hook)
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
	out, err := MergeClaudeSettings(existing, "SessionStart", "startup", hook)
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
	out1, err := MergeClaudeSettings(nil, "SessionStart", "startup", hook)
	if err != nil {
		t.Fatalf("primera inserción: %v", err)
	}
	// Segunda inserción (misma operación sobre el resultado previo).
	out2, err := MergeClaudeSettings(out1, "SessionStart", "startup", hook)
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
	out, err := MergeClaudeSettings([]byte(`{}`), "SessionStart", "startup", hook)
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
	out, err := MergeClaudeSettings(existing, "SessionStart", "startup", hook)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(string(out), `"permissions"`) {
		t.Error("se perdió la clave de nivel superior 'permissions'")
	}
}

// TestMergeClaudeSettingsUserPromptSubmit verifica que se puede registrar un hook
// UserPromptSubmit (sin matcher) y que coexiste con uno de SessionStart existente.
func TestMergeClaudeSettingsUserPromptSubmit(t *testing.T) {
	ss := HookCommand{Type: "command", Command: "/bin/musubi detect --hook-mode", Timeout: 10}
	out, err := MergeClaudeSettings(nil, "SessionStart", "startup", ss)
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	ups := HookCommand{Type: "command", Command: "/bin/musubi turn --hook-mode", Timeout: 10}
	out, err = MergeClaudeSettings(out, "UserPromptSubmit", "", ups)
	if err != nil {
		t.Fatalf("UserPromptSubmit: %v", err)
	}

	// SessionStart se preserva.
	if entradas := parsearHooksEvento(t, out, "SessionStart"); len(entradas) != 1 || len(entradas[0].Hooks) != 1 {
		t.Fatalf("SessionStart debe seguir teniendo 1 hook, obtuve %+v", entradas)
	}
	// UserPromptSubmit queda registrado, sin matcher.
	ups2 := parsearHooksEvento(t, out, "UserPromptSubmit")
	if len(ups2) != 1 || len(ups2[0].Hooks) != 1 {
		t.Fatalf("esperaba 1 hook UserPromptSubmit, obtuve %+v", ups2)
	}
	if ups2[0].Matcher != "" {
		t.Errorf("UserPromptSubmit no debe llevar matcher, obtuve %q", ups2[0].Matcher)
	}
	if ups2[0].Hooks[0].Command != ups.Command {
		t.Errorf("Command incorrecto en UserPromptSubmit: %q", ups2[0].Hooks[0].Command)
	}
}

// TestMergeClaudeSettingsReemplazaRutaVieja verifica que re-registrar el mismo
// subcomando con OTRA ruta del binario reemplaza el hook viejo en vez de duplicarlo.
func TestMergeClaudeSettingsReemplazaRutaVieja(t *testing.T) {
	vieja := HookCommand{Type: "command", Command: `"/old/path/musubi" detect --hook-mode`, Timeout: 10}
	out, err := MergeClaudeSettings(nil, "SessionStart", "startup", vieja)
	if err != nil {
		t.Fatalf("primer merge: %v", err)
	}
	nueva := HookCommand{Type: "command", Command: `"/new/path/musubi" detect --hook-mode`, Timeout: 10}
	out, err = MergeClaudeSettings(out, "SessionStart", "startup", nueva)
	if err != nil {
		t.Fatalf("segundo merge: %v", err)
	}

	entradas := parsearHooksSessionStart(t, out)
	total := 0
	var cmd string
	for _, e := range entradas {
		for _, h := range e.Hooks {
			total++
			cmd = h.Command
		}
	}
	if total != 1 {
		t.Fatalf("esperaba 1 hook (reemplazo, no duplicado), obtuve %d", total)
	}
	if cmd != nueva.Command {
		t.Errorf("debe quedar la ruta nueva, obtuve %q", cmd)
	}
}

// TestMergeClaudeSettingsJSONInvalido verifica que un input malformado retorna error.
func TestMergeClaudeSettingsJSONInvalido(t *testing.T) {
	_, err := MergeClaudeSettings([]byte("{no es json"), "SessionStart", "startup", HookCommand{Command: "x"})
	if err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}
