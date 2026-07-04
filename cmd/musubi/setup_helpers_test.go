package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

func TestEnsureWorkspaceCreaConfig(t *testing.T) {
	root := t.TempDir()
	if err := ensureWorkspace(root); err != nil {
		t.Fatalf("ensureWorkspace error: %v", err)
	}
	cfgPath := filepath.Join(root, config.DirName, config.ConfigFile)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("esperaba %s creado: %v", cfgPath, err)
	}

	// Idempotente: re-ejecutar no debe fallar ni pisar el config existente.
	if err := os.WriteFile(cfgPath, []byte("version: \"1.0\"\nmarca: presente\n"), 0644); err != nil {
		t.Fatalf("preparando config: %v", err)
	}
	if err := ensureWorkspace(root); err != nil {
		t.Fatalf("segunda ensureWorkspace error: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data), "marca: presente") {
		t.Error("ensureWorkspace sobrescribió un config existente")
	}
}

func TestWriteMCPConfigRegistraServidor(t *testing.T) {
	root := t.TempDir()
	if err := writeMCPConfigAt(root, "/ruta/al/musubi", ".mcp.json", true); err != nil {
		t.Fatalf("writeMCPConfigAt error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("esperaba .mcp.json: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "musubi") || !strings.Contains(s, "daemon") {
		t.Errorf(".mcp.json no registró el servidor musubi: %s", s)
	}
}

func TestEnsureGitignoreAgregaEntradaUnaVez(t *testing.T) {
	root := t.TempDir()
	entry := config.DirName + "/" + config.DBFile

	if err := ensureGitignore(root); err != nil {
		t.Fatalf("ensureGitignore error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(data), entry) {
		t.Fatalf(".gitignore no contiene %q: %q", entry, string(data))
	}

	// Idempotente: no duplica la entrada.
	if err := ensureGitignore(root); err != nil {
		t.Fatalf("segunda ensureGitignore error: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Count(string(data), entry) != 1 {
		t.Errorf("esperaba la entrada una sola vez, hubo %d", strings.Count(string(data), entry))
	}
}

func TestWriteCodeMemoryHookRegistraPrecheck(t *testing.T) {
	root := t.TempDir()
	if err := writeCodeMemoryHook(root, "/ruta/musubi"); err != nil {
		t.Fatalf("writeCodeMemoryHook error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile))
	if err != nil {
		t.Fatalf("esperaba settings.json: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "precheck") || !strings.Contains(s, "PreToolUse") {
		t.Errorf("settings.json no registró el hook precheck/PreToolUse: %s", s)
	}
}

func TestWorkspaceDirHonraMusubiHome(t *testing.T) {
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("MUSUBI_HOME", "/un/home/explicito")
	if got := workspaceDir(); got != "/un/home/explicito" {
		t.Errorf("esperaba el valor de MUSUBI_HOME, obtuve %q", got)
	}

	t.Setenv("MUSUBI_HOME", "")
	if got := workspaceDir(); got != "." {
		t.Errorf("sin MUSUBI_HOME ni CLAUDE_PROJECT_DIR esperaba %q, obtuve %q", ".", got)
	}
}

// TestWorkspaceDirUsaClaudeProjectDir verifica el fallback portable: sin MUSUBI_HOME,
// la raíz del proyecto se toma de CLAUDE_PROJECT_DIR (que Claude Code inyecta), y
// MUSUBI_HOME mantiene prioridad cuando está presente.
func TestWorkspaceDirUsaClaudeProjectDir(t *testing.T) {
	t.Setenv("MUSUBI_HOME", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "/proj/inyectado")
	if got := workspaceDir(); got != "/proj/inyectado" {
		t.Errorf("esperaba CLAUDE_PROJECT_DIR, obtuve %q", got)
	}
	t.Setenv("MUSUBI_HOME", "/home/explicito")
	if got := workspaceDir(); got != "/home/explicito" {
		t.Errorf("MUSUBI_HOME debe primar sobre CLAUDE_PROJECT_DIR, obtuve %q", got)
	}
}

func TestSameFile(t *testing.T) {
	if !sameFile("a/b/c.txt", "a/b/c.txt") {
		t.Error("rutas idénticas deberían ser sameFile")
	}
	if sameFile("a/b/c.txt", "a/b/d.txt") {
		t.Error("rutas distintas no deberían ser sameFile")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	want := []byte("contenido binario\x00\x01\x02")
	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatalf("preparando src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile error: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("leyendo dst: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("contenido copiado no coincide: %q vs %q", got, want)
	}
}

func TestSetupProjectWithInyectaTodo(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	// exePath ficticio: setup solo lo escribe como string en .mcp.json/hooks,
	// no lo ejecuta, así que no necesita existir.
	setupProjectWith(filepath.Join(root, "musubi-fake"), "")

	checks := []string{
		filepath.Join(config.DirName, config.ConfigFile),
		".mcp.json",
		filepath.Join(config.ClaudeDir, config.ClaudeSettingsFile),
		".gitignore",
	}
	for _, rel := range checks {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("setup no creó %s: %v", rel, err)
		}
	}

	// settings.json debe tener los tres hooks que inyecta setup.
	data, _ := os.ReadFile(filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile))
	for _, hook := range []string{"detect --hook-mode", "turn --hook-mode", "precheck --hook-mode"} {
		if !strings.Contains(string(data), hook) {
			t.Errorf("settings.json no registró el hook %q", hook)
		}
	}
}

func TestMaintenanceCycleDBVacia(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	rep, err := maintenanceCycle(engine, config.Default().Maintenance)
	if err != nil {
		t.Fatalf("maintenanceCycle error: %v", err)
	}
	if rep.Consolidate.Merged != 0 || rep.Decay.Archived != 0 || rep.Purged != 0 {
		t.Errorf("en DB vacía no debería fusionar/archivar/purgar: merged=%d archived=%d purged=%d", rep.Consolidate.Merged, rep.Decay.Archived, rep.Purged)
	}
}

func TestWriteMCPConfigAtCursorPath(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join(".cursor", "mcp.json")
	if err := writeMCPConfigAt(root, "/ruta/musubi", rel, false); err != nil {
		t.Fatalf("writeMCPConfigAt error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("esperaba %s creado (con su dir padre): %v", rel, err)
	}
	if s := string(data); !strings.Contains(s, "musubi") || !strings.Contains(s, "daemon") {
		t.Errorf("%s no registró el servidor musubi: %s", rel, s)
	}
}

// TestWriteMCPConfigPortable verifica que el modo portable (Claude) escribe un command
// resoluble por MUSUBI_BIN y NO hardcodea MUSUBI_HOME (la raíz la da CLAUDE_PROJECT_DIR).
func TestWriteMCPConfigPortable(t *testing.T) {
	root := t.TempDir()
	if err := writeMCPConfigAt(root, `C:\bin\musubi.exe`, ".mcp.json", true); err != nil {
		t.Fatalf("writeMCPConfigAt error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("esperaba .mcp.json: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "${MUSUBI_BIN:-") {
		t.Errorf("command portable debería usar ${MUSUBI_BIN:-...}: %s", s)
	}
	if strings.Contains(s, "MUSUBI_HOME") {
		t.Errorf("modo portable no debería hardcodear MUSUBI_HOME: %s", s)
	}
	if !strings.Contains(s, "daemon") {
		t.Errorf("falta el arg daemon: %s", s)
	}
}

// TestWriteMCPConfigAbsoluto verifica que el modo no-portable (ej. Cursor) usa la ruta
// absoluta del binario y MUSUBI_HOME con la raíz del proyecto.
func TestWriteMCPConfigAbsoluto(t *testing.T) {
	root := t.TempDir()
	if err := writeMCPConfigAt(root, `C:\bin\musubi.exe`, ".mcp.json", false); err != nil {
		t.Fatalf("writeMCPConfigAt error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("esperaba .mcp.json: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "${MUSUBI_BIN") {
		t.Errorf("modo absoluto no debería usar ${MUSUBI_BIN}: %s", s)
	}
	if !strings.Contains(s, "MUSUBI_HOME") {
		t.Errorf("modo absoluto debería incluir MUSUBI_HOME: %s", s)
	}
}
