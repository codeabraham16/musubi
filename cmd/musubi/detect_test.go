package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/memory"
)

// fakeStore implementa startupStore para tests deterministas del hook, sin DB real.
type fakeStore struct {
	meta  map[string]string
	prime memory.RecallResult
}

func newFakeStore() *fakeStore { return &fakeStore{meta: map[string]string{}} }

func (f *fakeStore) GetMeta(key string) (string, bool, error) {
	v, ok := f.meta[key]
	return v, ok, nil
}
func (f *fakeStore) SetMeta(key, value string) error { f.meta[key] = value; return nil }
func (f *fakeStore) PrimeContext(budget int) (memory.RecallResult, error) {
	return f.prime, nil
}

// crearGoNodeProject crea un proyecto políglota (Go + Node.js con React).
func crearGoNodeProject(t *testing.T) string {
	t.Helper()
	dir := crearGoProject(t)
	pkg := `{"name":"app","dependencies":{"react":"^18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("no se pudo crear package.json: %v", err)
	}
	return dir
}

func defaultStartup() config.StartupConfig {
	return config.StartupConfig{PrimeMemory: true, RecallBudget: 300, AutoRegen: true}
}

func TestHookFullGenerationPrimeraVez(t *testing.T) {
	dir := crearGoProject(t)
	store := newFakeStore() // sin huella, sin sentinel
	out, err := buildHookOutput(dir, store, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("primera vez debe disparar generación completa, obtuve: %q", out)
	}
}

func TestHookDeltaRegeneracion(t *testing.T) {
	dir := crearGoNodeProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	// La huella guardada solo cubría Go; ahora hay Node.js (delta).
	store.meta["skills_stack"] = detector.StackFingerprint([]detector.StackResult{{Ecosystem: "Go"}})
	out, err := buildHookOutput(dir, store, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("un delta de stack debe disparar generación, obtuve: %q", out)
	}
	if !strings.Contains(out, "Node.js") {
		t.Errorf("la generación del delta debe mencionar lo nuevo (Node.js), obtuve: %q", out)
	}
}

func TestHookSinCambiosNiMemoria(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	out, err := buildHookOutput(dir, store, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("sin cambios de stack ni memoria, el hook debe ser silencioso, obtuve: %q", out)
	}
}

func TestHookPrimingInyectado(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	store.prime = memory.RecallResult{
		Count:      1,
		UsedTokens: 10,
		Items:      []memory.RecallItem{{ID: "1", TopicKey: "arch/db", Gist: "Usamos SQLite con FTS5"}},
	}
	out, err := buildHookOutput(dir, store, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "recuerda") {
		t.Errorf("el priming debe inyectar contexto recordado, obtuve: %q", out)
	}
	if !strings.Contains(out, "SQLite con FTS5") {
		t.Errorf("el priming debe incluir el gist, obtuve: %q", out)
	}
	if strings.Contains(out, "musubi_save_skill") {
		t.Errorf("sin cambios de stack no debe haber instrucciones de generación, obtuve: %q", out)
	}
}

func TestHookMigracionBackfill(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir) // proyecto viejo: sentinel pero sin huella en meta
	store := newFakeStore()
	out, err := buildHookOutput(dir, store, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "musubi_save_skill") {
		t.Errorf("la migración no debe re-generar skills, obtuve: %q", out)
	}
	if _, ok := store.meta["skills_stack"]; !ok {
		t.Error("la migración debe backfillear la huella del stack en meta")
	}
}

func TestHookAutoRegenDesactivado(t *testing.T) {
	dir := crearGoNodeProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	store.meta["skills_stack"] = detector.StackFingerprint([]detector.StackResult{{Ecosystem: "Go"}})
	cfg := defaultStartup()
	cfg.AutoRegen = false
	out, err := buildHookOutput(dir, store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "musubi_save_skill") {
		t.Errorf("con auto_regen=false un delta no debe re-generar, obtuve: %q", out)
	}
}

func TestHookStoreNilFallback(t *testing.T) {
	dir := crearGoProject(t)
	// Sin sentinel + store nil → comportamiento viejo: generación completa.
	out, err := buildHookOutput(dir, nil, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("store nil sin sentinel debe caer al flujo viejo de generación, obtuve: %q", out)
	}
	// Con sentinel + store nil → silencioso.
	crearSentinel(t, dir)
	out, err = buildHookOutput(dir, nil, defaultStartup())
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("store nil con sentinel debe ser silencioso, obtuve: %q", out)
	}
}

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
	if !strings.Contains(ctx, "musubi_search_skills") {
		t.Error("additionalContext no menciona 'musubi_search_skills'")
	}
}
