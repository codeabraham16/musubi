package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// parseClaudeSettings lee el archivo .claude/settings.json de un directorio raíz
// y devuelve las entradas del array hooks.SessionStart.
func parseClaudeSettings(t *testing.T, root string) []map[string]interface{} {
	t.Helper()
	settingsPath := filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", settingsPath, err)
	}
	var parsed struct {
		Hooks struct {
			SessionStart []map[string]interface{} `json:"SessionStart"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings.json no es JSON válido: %v\n%s", err, data)
	}
	return parsed.Hooks.SessionStart
}

// contarHooksDetect cuenta cuántas entradas en SessionStart tienen un hook
// cuyo command termina con "detect --hook-mode".
func contarHooksDetect(entradas []map[string]interface{}) int {
	count := 0
	for _, entrada := range entradas {
		hooksRaw, ok := entrada["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range hooksRaw {
			hMap, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hMap["command"].(string)
			if strings.HasSuffix(cmd, "detect --hook-mode") {
				count++
			}
		}
	}
	return count
}

// TestWriteClaudeHookCreaArchivo verifica que writeClaudeHook crea
// .claude/settings.json con un hook SessionStart cuyo command termina en
// "detect --hook-mode".
func TestWriteClaudeHookCreaArchivo(t *testing.T) {
	root := t.TempDir()
	exePath := "/usr/local/bin/musubi"

	if err := writeClaudeHook(root, exePath); err != nil {
		t.Fatalf("writeClaudeHook falló: %v", err)
	}

	// Verificar que el archivo existe.
	settingsPath := filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile)
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings.json no fue creado: %v", err)
	}

	// Verificar el contenido.
	entradas := parseClaudeSettings(t, root)
	n := contarHooksDetect(entradas)
	if n != 1 {
		t.Errorf("se esperaba exactamente 1 hook detect, encontré %d", n)
	}
}

// TestWriteTurnHookRegistraUserPromptSubmit verifica que writeTurnHook agrega un
// hook UserPromptSubmit (sin matcher) cuyo command termina en "turn --hook-mode",
// y que preserva el hook SessionStart escrito por writeClaudeHook.
func TestWriteTurnHookRegistraUserPromptSubmit(t *testing.T) {
	root := t.TempDir()
	exePath := "/usr/local/bin/musubi"

	if err := writeClaudeHook(root, exePath); err != nil {
		t.Fatalf("writeClaudeHook falló: %v", err)
	}
	if err := writeTurnHook(root, exePath); err != nil {
		t.Fatalf("writeTurnHook falló: %v", err)
	}

	// SessionStart sigue presente.
	if n := contarHooksDetect(parseClaudeSettings(t, root)); n != 1 {
		t.Errorf("se esperaba 1 hook detect tras writeTurnHook, encontré %d", n)
	}

	// UserPromptSubmit registra el comando 'turn --hook-mode'.
	settingsPath := filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("no se pudo leer settings.json: %v", err)
	}
	var parsed struct {
		Hooks struct {
			UserPromptSubmit []map[string]interface{} `json:"UserPromptSubmit"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings.json no es JSON válido: %v\n%s", err, data)
	}
	if len(parsed.Hooks.UserPromptSubmit) != 1 {
		t.Fatalf("esperaba 1 entrada UserPromptSubmit, obtuve %d", len(parsed.Hooks.UserPromptSubmit))
	}
	entrada := parsed.Hooks.UserPromptSubmit[0]
	if m, ok := entrada["matcher"].(string); ok && m != "" {
		t.Errorf("UserPromptSubmit no debe llevar matcher, obtuve %q", m)
	}
	hooksRaw, _ := entrada["hooks"].([]interface{})
	if len(hooksRaw) != 1 {
		t.Fatalf("esperaba 1 hook en UserPromptSubmit, obtuve %d", len(hooksRaw))
	}
	cmd, _ := hooksRaw[0].(map[string]interface{})["command"].(string)
	if !strings.HasSuffix(cmd, "turn --hook-mode") {
		t.Errorf("el command de UserPromptSubmit debe terminar en 'turn --hook-mode', obtuve %q", cmd)
	}
}

// TestWriteTurnHookIdempotente verifica que llamar a writeTurnHook dos veces no
// duplica el hook UserPromptSubmit.
func TestWriteTurnHookIdempotente(t *testing.T) {
	root := t.TempDir()
	exePath := "/usr/local/bin/musubi"

	if err := writeTurnHook(root, exePath); err != nil {
		t.Fatalf("primera llamada falló: %v", err)
	}
	if err := writeTurnHook(root, exePath); err != nil {
		t.Fatalf("segunda llamada falló: %v", err)
	}

	settingsPath := filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile)
	data, _ := os.ReadFile(settingsPath)
	var parsed struct {
		Hooks struct {
			UserPromptSubmit []map[string]interface{} `json:"UserPromptSubmit"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings.json no es JSON válido: %v", err)
	}
	total := 0
	for _, e := range parsed.Hooks.UserPromptSubmit {
		if h, ok := e["hooks"].([]interface{}); ok {
			total += len(h)
		}
	}
	if total != 1 {
		t.Errorf("esperaba 1 hook UserPromptSubmit (idempotente), obtuve %d", total)
	}
}

// TestWriteClaudeHookIdempotente verifica que llamar a writeClaudeHook dos veces
// con el mismo exePath no duplica el hook.
func TestWriteClaudeHookIdempotente(t *testing.T) {
	root := t.TempDir()
	exePath := "/usr/local/bin/musubi"

	// Primera llamada.
	if err := writeClaudeHook(root, exePath); err != nil {
		t.Fatalf("primera llamada falló: %v", err)
	}
	// Segunda llamada (debe ser no-op respecto al hook).
	if err := writeClaudeHook(root, exePath); err != nil {
		t.Fatalf("segunda llamada falló: %v", err)
	}

	entradas := parseClaudeSettings(t, root)
	n := contarHooksDetect(entradas)
	if n != 1 {
		t.Errorf("se esperaba 1 hook detect (idempotente), encontré %d", n)
	}
}

// TestWriteClaudeHookPreservaHooksExistentes verifica que otros hooks en
// settings.json no se pierden al agregar el de Musubi.
func TestWriteClaudeHookPreservaHooksExistentes(t *testing.T) {
	root := t.TempDir()

	// Crear settings.json pre-existente con otro hook.
	claudeDir := filepath.Join(root, config.ClaudeDir)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("no se pudo crear .claude: %v", err)
	}
	preExistente := `{
		"hooks": {
			"SessionStart": [
				{"matcher": "startup", "hooks": [{"type": "command", "command": "otro-tool"}]}
			]
		}
	}`
	settingsPath := filepath.Join(claudeDir, config.ClaudeSettingsFile)
	if err := os.WriteFile(settingsPath, []byte(preExistente), 0644); err != nil {
		t.Fatalf("no se pudo crear settings.json: %v", err)
	}

	if err := writeClaudeHook(root, "/bin/musubi"); err != nil {
		t.Fatalf("writeClaudeHook falló: %v", err)
	}

	entradas := parseClaudeSettings(t, root)
	// Debe haber exactamente 1 entrada de matcher con 2 hooks.
	if len(entradas) != 1 {
		t.Fatalf("se esperaba 1 entrada de matcher, encontré %d", len(entradas))
	}
	hooksRaw, _ := entradas[0]["hooks"].([]interface{})
	if len(hooksRaw) != 2 {
		t.Errorf("se esperaban 2 hooks (otro-tool + musubi), encontré %d", len(hooksRaw))
	}
}
