package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// crearGoProject crea un directorio temporal con go.mod para simular un proyecto Go.
func crearGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gomod := "module ejemplo.com/mi-proyecto\n\ngo 1.26.4\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}
	return dir
}

// crearSentinel escribe el archivo sentinel en el directorio de skills del proyecto.
func crearSentinel(t *testing.T, root string) {
	t.Helper()
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("no se pudo crear skillsDir: %v", err)
	}
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)
	if err := os.WriteFile(sentinelPath, []byte("generado"), 0644); err != nil {
		t.Fatalf("no se pudo crear sentinel: %v", err)
	}
}

// TestDetectOutputModoNormal verifica que en modo normal (hookMode=false) se devuelve
// un JSON válido representando el slice de StackResult con al menos "ecosystem".
func TestDetectOutputModoNormal(t *testing.T) {
	dir := crearGoProject(t)
	out, err := detectOutput(dir, false)
	if err != nil {
		t.Fatalf("error en modo normal: %v", err)
	}
	// Debe ser JSON válido deserializable como slice.
	var resultados []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &resultados); err != nil {
		t.Fatalf("salida no es JSON de slice válido: %v\nSalida: %s", err, out)
	}
	if len(resultados) == 0 {
		t.Fatal("se esperaban resultados para un proyecto Go")
	}
	ecosistema, _ := resultados[0]["ecosystem"].(string)
	if !strings.EqualFold(ecosistema, "go") {
		t.Errorf("se esperaba ecosistema 'Go', obtuve %q", ecosistema)
	}
}

// TestDetectOutputHookModeSentinelPresente verifica que si el sentinel existe,
// detectOutput devuelve string vacío y nil (operación silenciosa idempotente).
func TestDetectOutputHookModeSentinelPresente(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)

	out, err := detectOutput(dir, true)
	if err != nil {
		t.Fatalf("error inesperado con sentinel presente: %v", err)
	}
	if out != "" {
		t.Errorf("se esperaba string vacío (sentinel presente), obtuve: %q", out)
	}
}

// TestDetectOutputHookModeSentinelAusente verifica que sin sentinel se devuelve
// el JSON de hookSpecificOutput con los campos requeridos por el protocolo de
// hooks de Claude Code.
func TestDetectOutputHookModeSentinelAusente(t *testing.T) {
	dir := crearGoProject(t)
	// No creamos sentinel.

	out, err := detectOutput(dir, true)
	if err != nil {
		t.Fatalf("error inesperado sin sentinel: %v", err)
	}
	if out == "" {
		t.Fatal("se esperaba output no vacío (sentinel ausente)")
	}

	// Verificar que es JSON válido con hookSpecificOutput.
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("salida no es JSON válido: %v\nSalida: %s", err, out)
	}
	hso, ok := envelope["hookSpecificOutput"].(map[string]interface{})
	if !ok {
		t.Fatalf("falta clave 'hookSpecificOutput' en el output: %+v", envelope)
	}
	if hso["hookEventName"] != "SessionStart" {
		t.Errorf("hookEventName incorrecto: %v", hso["hookEventName"])
	}
	ctx, _ := hso["additionalContext"].(string)
	if ctx == "" {
		t.Fatal("additionalContext está vacío")
	}
	// Verificar que el contexto menciona las herramientas MCP requeridas.
	if !strings.Contains(ctx, "musubi_detect_stack") {
		t.Error("additionalContext no menciona 'musubi_detect_stack'")
	}
	if !strings.Contains(ctx, "musubi_save_skill") {
		t.Error("additionalContext no menciona 'musubi_save_skill'")
	}
}
