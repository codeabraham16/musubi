package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// fakeCodeStore implementa codeStore para los tests del hook PreToolUse.
type fakeCodeStore struct {
	mem map[string]memory.CodeMemory
	tel []memory.TelemetryLog
}

func (f *fakeCodeStore) GetCodeMemory(path string) (memory.CodeMemory, bool, error) {
	cm, ok := f.mem[path]
	return cm, ok, nil
}

func (f *fakeCodeStore) GetUnresolvedTelemetryLogsForFiles(files []string) ([]memory.TelemetryLog, error) {
	return f.tel, nil
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPrecheckIgnoresNonReadTool(t *testing.T) {
	store := &fakeCodeStore{mem: map[string]memory.CodeMemory{}}
	in := `{"tool_name":"Bash","tool_input":{"command":"ls"},"session_id":"s"}`
	if out := precheckOutput(store, t.TempDir(), strings.NewReader(in)); out != "" {
		t.Errorf("un tool distinto de Read no debe producir salida, obtuve %q", out)
	}
}

func TestPrecheckSurfacesFreshGist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.go", "package foo\nfunc Bar(){}\n")
	fp, _ := memory.FileFingerprint(root, "foo.go")
	store := &fakeCodeStore{mem: map[string]memory.CodeMemory{
		"foo.go": {Path: "foo.go", Gist: "Paquete foo con Bar().", Symbols: "Bar() L2", Fingerprint: fp},
	}}
	in := `{"tool_name":"Read","tool_input":{"file_path":"` + filepath.ToSlash(filepath.Join(root, "foo.go")) + `"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "Paquete foo con Bar().") || !strings.Contains(strings.ToUpper(ctx), "FRESCO") {
		t.Errorf("esperaba el gist fresco en el contexto, obtuve %q", ctx)
	}
}

func TestPrecheckFlagsStaleGist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.go", "package foo\nfunc Bar(){}\n")
	store := &fakeCodeStore{mem: map[string]memory.CodeMemory{
		"foo.go": {Path: "foo.go", Gist: "viejo", Fingerprint: "hash-viejo-distinto"},
	}}
	in := `{"tool_name":"Read","tool_input":{"file_path":"foo.go"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(strings.ToUpper(ctx), "CAMBIÓ") {
		t.Errorf("un gist con fingerprint distinto debe marcarse como cambiado, obtuve %q", ctx)
	}
}

func TestPrecheckNudgesSaveForBigUnknownFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "big.go", strings.Repeat("// línea de relleno para superar el umbral\n", 80))
	store := &fakeCodeStore{mem: map[string]memory.CodeMemory{}}
	in := `{"tool_name":"Read","tool_input":{"file_path":"big.go"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "musubi_save_code") {
		t.Errorf("un archivo grande sin gist debe sugerir guardarlo, obtuve %q", ctx)
	}
}

func TestPrecheckSurfacesKnownErrors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "svc.go", "package svc\n") // chico y sin gist => sin aviso de código
	store := &fakeCodeStore{
		mem: map[string]memory.CodeMemory{},
		tel: []memory.TelemetryLog{
			{ID: 7, FilePath: "svc.go", ErrorMessage: "nil pointer en Handler", SuggestedPatch: "chequear req != nil"},
		},
	}
	in := `{"tool_name":"Read","tool_input":{"file_path":"svc.go"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "errores conocidos") || !strings.Contains(ctx, "nil pointer en Handler") {
		t.Errorf("debe surfacear el error conocido del archivo, obtuve %q", ctx)
	}
	if !strings.Contains(ctx, "id 7") {
		t.Errorf("debe incluir el id para resolverlo con musubi_resolve_telemetry, obtuve %q", ctx)
	}
	if !strings.Contains(ctx, "chequear req != nil") {
		t.Errorf("debe incluir el fix sugerido, obtuve %q", ctx)
	}
}

func TestPrecheckSilentForSmallUnknownFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "tiny.go", "package x\n")
	store := &fakeCodeStore{mem: map[string]memory.CodeMemory{}}
	in := `{"tool_name":"Read","tool_input":{"file_path":"tiny.go"},"session_id":"s"}`
	if out := precheckOutput(store, root, strings.NewReader(in)); out != "" {
		t.Errorf("un archivo chico sin gist no debe molestar, obtuve %q", out)
	}
}
