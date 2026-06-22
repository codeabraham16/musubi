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
	meta   map[string]string
	prime  memory.RecallResult
	topics map[string]bool

	ledger        map[string]int
	ledgerSession string
}

func newFakeStore() *fakeStore {
	return &fakeStore{meta: map[string]string{}, topics: map[string]bool{}}
}

func (f *fakeStore) GetMeta(key string) (string, bool, error) {
	v, ok := f.meta[key]
	return v, ok, nil
}
func (f *fakeStore) SetMeta(key, value string) error { f.meta[key] = value; return nil }
func (f *fakeStore) PrimeContext(budget int) (memory.RecallResult, error) {
	return f.prime, nil
}
func (f *fakeStore) TopicExists(topicKey string) (bool, error) {
	return f.topics[topicKey], nil
}
func (f *fakeStore) LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error) {
	if f.ledger == nil {
		f.ledger = map[string]int{}
	}
	f.ledger[surface] += tokens
	f.ledgerSession = sessionID
	return memory.TokenLedger{SessionID: sessionID, Total: tokens, Surfaces: f.ledger}, nil
}

func TestHealthContextSurfacesIssues(t *testing.T) {
	store := newFakeStore()

	// Sin reporte: silencioso.
	if buildHealthContext(store) != "" {
		t.Error("sin MetaLastHealth el bloque de salud debe ser vacío")
	}

	// Reporte sano: silencioso.
	okRep, _ := json.Marshal(memory.DiagnoseReport{
		Status: "ok",
		Checks: []memory.CheckResult{{Code: "db_integrity", Status: "ok"}},
	})
	store.meta[memory.MetaLastHealth] = string(okRep)
	if buildHealthContext(store) != "" {
		t.Error("un reporte 'ok' no debe surfacearse")
	}

	// Reporte con problemas no auto-reparables: surfacea advertencia con el detalle.
	badRep, _ := json.Marshal(memory.DiagnoseReport{
		Status: "issues",
		Checks: []memory.CheckResult{{Code: "db_integrity", Status: "error", Message: "integridad comprometida"}},
	})
	store.meta[memory.MetaLastHealth] = string(badRep)
	out := buildHealthContext(store)
	if !strings.Contains(out, "salud") || !strings.Contains(out, "db_integrity") {
		t.Errorf("debe surfacear el problema con su código, obtuve %q", out)
	}
}

// TestHookAccountsAllStartupSurfaces verifica el ledger HOLÍSTICO (T9.1): el
// SessionStart contabiliza TODAS las superficies que inyecta —no solo el priming—
// estimando el texto final de cada bloque. Acá conviven priming (memoria) y bloque
// cognitivo (proyecto sin perfil); ambos deben quedar medidos bajo el session_id.
func TestHookAccountsAllStartupSurfaces(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir) // skills generadas: aísla priming + cognitivo de la generación
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	store.prime = memory.RecallResult{
		Count:      1,
		UsedTokens: 30,
		Items:      []memory.RecallItem{{ID: "x", TopicKey: "arch/db", Gist: "Usamos SQLite con FTS5"}},
	}
	// Sin perfil → el bloque cognitivo también se inyecta (y debe contabilizarse).
	out, err := buildHookOutput(dir, store, defaultStartup(), "sess-9")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("esperaba contexto de arranque no vacío")
	}
	if store.ledger["startup_priming"] <= 0 {
		t.Errorf("el priming debe contabilizarse, obtuve %d", store.ledger["startup_priming"])
	}
	if store.ledger["startup_cognitive"] <= 0 {
		t.Errorf("el bloque cognitivo debe contabilizarse (antes invisible), obtuve %d", store.ledger["startup_cognitive"])
	}
	if store.ledgerSession != "sess-9" {
		t.Errorf("el ledger debe usar el session_id del SessionStart, obtuve %q", store.ledgerSession)
	}
}

func TestPrimingSeedsDeltaState(t *testing.T) {
	store := newFakeStore()
	store.prime = memory.RecallResult{
		Count: 2,
		Items: []memory.RecallItem{
			{ID: "a", TopicKey: "t", Gist: "uno", ContentHash: "h1"},
			{ID: "b", TopicKey: "t", Gist: "dos", ContentHash: "h2"},
		},
	}
	if buildPrimingContext(store, 300, "s1") == "" {
		t.Fatal("esperaba bloque de priming")
	}
	// El priming debe sembrar el estado del delta con lo que inyectó, para que el
	// recall por turno no repita esos gists en la misma sesión.
	if store.meta[metaDeltaSession] != "s1" {
		t.Errorf("el priming debe fijar la sesión del delta, obtuve %q", store.meta[metaDeltaSession])
	}
	raw := store.meta[metaDeltaInjected]
	if !strings.Contains(raw, "\"a\"") || !strings.Contains(raw, "\"b\"") {
		t.Errorf("el delta debe quedar sembrado con a y b, obtuve %q", raw)
	}
}

func TestReadSessionID(t *testing.T) {
	if got := readSessionID(strings.NewReader(`{"session_id":"abc","hook_event_name":"SessionStart"}`)); got != "abc" {
		t.Errorf("esperaba 'abc', obtuve %q", got)
	}
	if got := readSessionID(strings.NewReader(`no es json`)); got != "" {
		t.Errorf("entrada inválida debe dar \"\", obtuve %q", got)
	}
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
	return config.StartupConfig{PrimeMemory: true, RecallBudget: 300, AutoRegen: true, CognitiveBootstrap: true}
}

func TestHookCognitivoEnBootstrapping(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir) // skills ya generadas: aislar el bloque cognitivo de la generación
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	// Sin perfil → debe inyectar el bloque cognitivo.
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "autoconocimiento") {
		t.Errorf("sin perfil debe inyectar el bloque cognitivo, obtuve: %q", out)
	}
	if !strings.Contains(out, "project/profile") {
		t.Errorf("el bloque cognitivo debe apuntar al topic_key del perfil, obtuve: %q", out)
	}
}

func TestHookCognitivoSilenciadoConPerfil(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	store.topics["project/profile"] = true // ya perfilado
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "autoconocimiento") {
		t.Errorf("con perfil existente NO debe inyectar el bloque cognitivo, obtuve: %q", out)
	}
}

func TestHookCognitivoRespetaToggle(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)
	store := newFakeStore()
	stack, _ := detector.DetectStack(dir)
	store.meta["skills_stack"] = detector.StackFingerprint(stack)
	cfg := defaultStartup()
	cfg.CognitiveBootstrap = false
	out, err := buildHookOutput(dir, store, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "autoconocimiento") {
		t.Errorf("con cognitive_bootstrap=false no debe inyectar el bloque, obtuve: %q", out)
	}
}

func TestHookFullGenerationPrimeraVez(t *testing.T) {
	dir := crearGoProject(t)
	store := newFakeStore() // sin huella, sin sentinel
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
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
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
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
	store.topics["project/profile"] = true // ya perfilado: nada que inyectar
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
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
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
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
	out, err := buildHookOutput(dir, store, defaultStartup(), "")
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
	out, err := buildHookOutput(dir, store, cfg, "")
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
	out, err := buildHookOutput(dir, nil, defaultStartup(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("store nil sin sentinel debe caer al flujo viejo de generación, obtuve: %q", out)
	}
	// Con sentinel + store nil → silencioso.
	crearSentinel(t, dir)
	out, err = buildHookOutput(dir, nil, defaultStartup(), "")
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
	out, err := detectOutput(dir, false, "")
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

// TestDetectOutputHookModeSentinelPresente verifica que con skills generadas
// (sentinel) y stack sin cambios, detectOutput NO re-genera skills ni inyecta el
// bloque cognitivo (el proyecto ya está perfilado). El priming puede aparecer.
func TestDetectOutputHookModeSentinelPresente(t *testing.T) {
	dir := crearGoProject(t)
	crearSentinel(t, dir)
	// Perfilar el proyecto para silenciar el bloque cognitivo de arranque.
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservation("prof", "project/profile", "Perfil: proyecto Go de prueba.", nil); err != nil {
		t.Fatal(err)
	}
	engine.Close()

	out, err := detectOutput(dir, true, "")
	if err != nil {
		t.Fatalf("error inesperado con sentinel presente: %v", err)
	}
	if strings.Contains(out, "musubi_save_skill") {
		t.Errorf("con sentinel presente y stack sin cambios no debe re-generar skills, obtuve: %q", out)
	}
	if strings.Contains(out, "autoconocimiento") {
		t.Errorf("proyecto ya perfilado no debe inyectar el bloque cognitivo, obtuve: %q", out)
	}
}

// TestDetectOutputHookModeSentinelAusente verifica que sin sentinel se devuelve
// el JSON de hookSpecificOutput con los campos requeridos por el protocolo de
// hooks de Claude Code.
func TestDetectOutputHookModeSentinelAusente(t *testing.T) {
	dir := crearGoProject(t)
	// No creamos sentinel.

	out, err := detectOutput(dir, true, "")
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
