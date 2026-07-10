package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// decodeDetect extrae el detectReport del contenido de la respuesta MCP.
func decodeDetect(t *testing.T, res interface{}) detectReport {
	t.Helper()
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %#v", res)
	}
	var rep detectReport
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &rep); err != nil {
		t.Fatalf("no se pudo decodear el reporte: %v\ntext=%s", err, resp.Content[0].Text)
	}
	return rep
}

func fileByPath(rep detectReport, path string) (fileChange, bool) {
	for _, f := range rep.Files {
		if f.Path == path {
			return f, true
		}
	}
	return fileChange{}, false
}

func TestDetectChangesReportsSymbolsStaleAndMemory(t *testing.T) {
	dir := t.TempDir()
	// Archivo actual: Parse() vive en las líneas 8-10 (tras varias líneas arriba).
	src := "package x\n\n" +
		"// linea\n// linea\n// linea\n\n" +
		"func Untouched() {}\n" +
		"func Parse() int {\n" +
		"\treturn 1\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithPath(t, dir)

	// Gist con fingerprint viejo → debe reportarse stale.
	if err := s.engine.SaveCodeMemory(memory.CodeMemory{
		Path: "foo.go", Gist: "gist viejo", Fingerprint: "hashviejo", Tokens: 3,
	}); err != nil {
		t.Fatal(err)
	}
	// Observación que referencia el archivo → debe aparecer en related_memory.
	if err := s.engine.SaveObservation("obs1", "arch/parser", "La decisión sobre foo.go y su parser.", nil); err != nil {
		t.Fatal(err)
	}

	// Diff con un hunk en el lado nuevo sobre las líneas 8-10 (donde está Parse).
	diff := "diff --git a/foo.go b/foo.go\n" +
		"index 111..222 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -8,2 +8,3 @@\n" +
		" ctx\n+nuevo\n ctx\n"
	s.gitRunner = codeintel.FakeRunner{Out: diff}

	res, rpcErr := call(t, s, "musubi_detect_changes", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("detect_changes devolvió error: %+v", rpcErr)
	}
	rep := decodeDetect(t, res)

	fc, ok := fileByPath(rep, "foo.go")
	if !ok {
		t.Fatalf("foo.go no está en el reporte: %+v", rep)
	}
	if !containsStr(fc.ChangedSymbols, "Parse") {
		t.Errorf("esperaba Parse en changed_symbols, obtuve %v", fc.ChangedSymbols)
	}
	if containsStr(fc.ChangedSymbols, "Untouched") {
		t.Errorf("Untouched NO debería reportarse (fuera del hunk), obtuve %v", fc.ChangedSymbols)
	}
	if !fc.GistStale {
		t.Errorf("el gist con fingerprint viejo debería ser stale")
	}
	if !containsStr(fc.RelatedMemory, "arch/parser") {
		t.Errorf("esperaba arch/parser en related_memory, obtuve %v", fc.RelatedMemory)
	}
}

func TestDetectChangesSkipsBinaryAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	s.gitRunner = codeintel.FakeRunner{Out: "diff --git a/img.png b/img.png\n" +
		"index 1..2 100644\nBinary files a/img.png and b/img.png differ\n"}

	res, rpcErr := call(t, s, "musubi_detect_changes", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("error: %+v", rpcErr)
	}
	if rep := decodeDetect(t, res); len(rep.Files) != 0 {
		t.Errorf("los binarios no deberían reportarse, obtuve %+v", rep.Files)
	}

	// Sanity: la tool está marcada read-only en el registro.
	for i := range s.tools {
		if s.tools[i].Name == "musubi_detect_changes" && !s.tools[i].readOnly {
			t.Errorf("musubi_detect_changes debería ser readOnly")
		}
	}
}

func TestDetectChangesGitError(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	s.gitRunner = codeintel.FakeRunner{Err: context.DeadlineExceeded}
	if _, rpcErr := call(t, s, "musubi_detect_changes", map[string]interface{}{}); rpcErr == nil {
		t.Errorf("un fallo de git debería devolver error JSON-RPC")
	}
}

func TestSaveCodeAutoDerivesSymbols(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nfunc Alpha() {}\n\nfunc Beta() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	s := newTestServerWithPath(t, dir)

	// Sin symbols: se derivan del contenido actual.
	if _, rpcErr := call(t, s, "musubi_save_code", map[string]interface{}{
		"path": "m.go", "gist": "módulo m",
	}); rpcErr != nil {
		t.Fatalf("save_code error: %+v", rpcErr)
	}
	cm, ok, err := s.engine.GetCodeMemory("m.go")
	if err != nil || !ok {
		t.Fatalf("no se guardó el gist: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(cm.Symbols, "Alpha") || !strings.Contains(cm.Symbols, "Beta") {
		t.Errorf("los símbolos deberían derivarse (Alpha, Beta), obtuve %q", cm.Symbols)
	}

	// Con symbols explícito: se respeta (compat hacia atrás).
	if _, rpcErr := call(t, s, "musubi_save_code", map[string]interface{}{
		"path": "m.go", "gist": "módulo m", "symbols": "manual L1",
	}); rpcErr != nil {
		t.Fatalf("save_code error: %+v", rpcErr)
	}
	cm, _, _ = s.engine.GetCodeMemory("m.go")
	if cm.Symbols != "manual L1" {
		t.Errorf("los símbolos explícitos deberían respetarse, obtuve %q", cm.Symbols)
	}
}

// TestDetectChangesEnforcesProjectScope prueba el aislamiento de Track 18: detect_changes es
// una superficie de LECTURA que cruza el diff local con la memoria compartida (código +
// observaciones). Un lector acotado a un proyecto NO debe ver el gist ni las observaciones
// atribuidas a OTRO proyecto (fuga de metadata: gist_stale + topic_keys); un admin federado sí.
func TestDetectChangesEnforcesProjectScope(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nfunc Parse() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	s := newTestServerWithPath(t, dir)

	// Gist + observación atribuidos al proyecto "web", con fingerprint viejo (sería stale si es visible).
	if err := s.engine.SaveCodeMemoryFrom("web", memory.CodeMemory{
		Path: "foo.go", Gist: "gist de web", Fingerprint: "hashviejo", Tokens: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.engine.SaveObservationTypedFrom("web", "obs-web", "web/decision-secreta",
		"La decisión de web sobre foo.go.", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}

	s.gitRunner = codeintel.FakeRunner{Out: "diff --git a/foo.go b/foo.go\n" +
		"index 111..222 100644\n--- a/foo.go\n+++ b/foo.go\n" +
		"@@ -1,3 +1,4 @@\n package x\n+// nuevo\n \n func Parse() int {\n"}

	detectAs := func(p *Principal) fileChange {
		raw, _ := json.Marshal(map[string]interface{}{})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_detect_changes", Arguments: raw})
		ctx := context.Background()
		if p != nil {
			ctx = withPrincipal(ctx, p)
		}
		out, rpcErr := s.handleToolsCall(ctx, params)
		if rpcErr != nil {
			t.Fatalf("detect_changes: %+v", rpcErr)
		}
		fc, ok := fileByPath(decodeDetect(t, out), "foo.go")
		if !ok {
			t.Fatalf("foo.go ausente del reporte")
		}
		return fc
	}

	// Lector acotado a "crm": no ve el gist ni la observación de "web".
	crm := detectAs(&Principal{Name: "alice", Role: RoleReader, ProjectID: "crm"})
	if crm.GistStale {
		t.Errorf("crm no tiene gist para foo.go ⇒ gist_stale debería ser false (fuga del gist de web)")
	}
	if containsStr(crm.RelatedMemory, "web/decision-secreta") {
		t.Errorf("crm no debería ver la observación de web en related_memory, obtuve %v", crm.RelatedMemory)
	}

	// Admin federado: ve el gist viejo de web como stale y su topic_key (el filtro no rompe legacy).
	adm := detectAs(&Principal{Name: "root", Role: RoleAdmin})
	if !adm.GistStale {
		t.Errorf("admin federado debería ver el gist viejo de web como stale")
	}
	if !containsStr(adm.RelatedMemory, "web/decision-secreta") {
		t.Errorf("admin federado debería ver la observación de web, obtuve %v", adm.RelatedMemory)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
