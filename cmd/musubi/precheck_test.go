package main

import (
	"context"
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

	// Grafo de código (F2-B): nodos por archivo y aristas por from/to key.
	graphNodes map[string][]memory.GraphNode
	outEdges   map[string][]memory.GraphEdge
	inEdges    map[string][]memory.GraphEdge

	ledger        map[string]int
	ledgerSession string
}

func (f *fakeCodeStore) ListGraphNodesForFileCtx(_ context.Context, path string) ([]memory.GraphNode, error) {
	return f.graphNodes[path], nil
}
func (f *fakeCodeStore) GraphOutEdgesCtx(_ context.Context, fromKey string) ([]memory.GraphEdge, error) {
	return f.outEdges[fromKey], nil
}
func (f *fakeCodeStore) GraphInEdgesCtx(_ context.Context, toKey string) ([]memory.GraphEdge, error) {
	return f.inEdges[toKey], nil
}

func (f *fakeCodeStore) GetCodeMemory(path string) (memory.CodeMemory, bool, error) {
	cm, ok := f.mem[path]
	return cm, ok, nil
}

func (f *fakeCodeStore) GetUnresolvedTelemetryLogsForFiles(files []string) ([]memory.TelemetryLog, error) {
	return f.tel, nil
}

func (f *fakeCodeStore) LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error) {
	if f.ledger == nil {
		f.ledger = map[string]int{}
	}
	f.ledger[surface] += tokens
	f.ledgerSession = sessionID
	return memory.TokenLedger{SessionID: sessionID, Total: tokens, Surfaces: f.ledger}, nil
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

func TestPrecheckAccountsTokensInLedger(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "svc.go", "package svc\nfunc Bar(){}\n")
	fp, _ := memory.FileFingerprint(root, "svc.go")
	store := &fakeCodeStore{
		mem: map[string]memory.CodeMemory{
			"svc.go": {Path: "svc.go", Gist: "Paquete svc con Bar().", Symbols: "Bar() L2", Fingerprint: fp},
		},
		tel: []memory.TelemetryLog{
			{ID: 7, FilePath: "svc.go", ErrorMessage: "nil pointer en Handler", SuggestedPatch: "chequear req != nil"},
		},
	}
	in := `{"tool_name":"Read","tool_input":{"file_path":"` + filepath.ToSlash(filepath.Join(root, "svc.go")) + `"},"session_id":"sess-pc"}`
	if out := precheckOutput(store, root, strings.NewReader(in)); out == "" {
		t.Fatal("esperaba contexto inyectado (gist + telemetría)")
	}
	// Ambas superficies del PreToolUse (antes invisibles) deben contabilizarse.
	if store.ledger["precheck_code"] <= 0 {
		t.Errorf("el gist de código debe contabilizarse en el ledger, obtuve %d", store.ledger["precheck_code"])
	}
	if store.ledger["precheck_telemetry"] <= 0 {
		t.Errorf("la telemetría debe contabilizarse en el ledger, obtuve %d", store.ledger["precheck_telemetry"])
	}
	if store.ledgerSession != "sess-pc" {
		t.Errorf("el ledger debe usar el session_id del hook, obtuve %q", store.ledgerSession)
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

// graphStore siembra un fakeCodeStore con el grafo de a.go: Alpha importa fmt y llama a beta.
func graphStore() *fakeCodeStore {
	return &fakeCodeStore{
		mem: map[string]memory.CodeMemory{},
		graphNodes: map[string][]memory.GraphNode{
			"a.go": {
				{Key: "a.go#func:Alpha", Kind: "func", Name: "Alpha", Path: "a.go"},
				{Key: "a.go#func:beta", Kind: "func", Name: "beta", Path: "a.go"},
			},
		},
		outEdges: map[string][]memory.GraphEdge{
			"a.go":            {{FromKey: "a.go", ToKey: "pkg:fmt", Kind: "IMPORTS"}},
			"a.go#func:Alpha": {{FromKey: "a.go#func:Alpha", ToKey: "a.go#func:beta", Kind: "CALLS"}},
		},
		inEdges: map[string][]memory.GraphEdge{
			"a.go#func:beta": {{FromKey: "a.go#func:Alpha", ToKey: "a.go#func:beta", Kind: "CALLS"}},
		},
	}
}

func TestPrecheckSurfacesCodeGraphWhenEnabled(t *testing.T) {
	t.Setenv("MUSUBI_CODEGRAPH_HOOK", "1")
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\nfunc Alpha(){ beta() }\nfunc beta(){}\n")
	store := graphStore()
	in := `{"tool_name":"Read","tool_input":{"file_path":"a.go"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "grafo de código") {
		t.Fatalf("esperaba el contexto del grafo, obtuve %q", ctx)
	}
	// Estructura: imports + Alpha llama a beta + beta lo llama Alpha.
	if !strings.Contains(ctx, "fmt") || !strings.Contains(ctx, "Alpha") || !strings.Contains(ctx, "beta") {
		t.Errorf("el contexto del grafo debe incluir imports y Alpha↔beta, obtuve %q", ctx)
	}
	if store.ledger["precheck_codegraph"] <= 0 {
		t.Errorf("el grafo inyectado debe contabilizarse en el ledger, obtuve %d", store.ledger["precheck_codegraph"])
	}
}

func TestPrecheckCodeGraphOffByDefault(t *testing.T) {
	// Sin MUSUBI_CODEGRAPH_HOOK: aunque el archivo esté indexado, NO se inyecta el grafo.
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\nfunc Alpha(){}\n")
	store := graphStore()
	in := `{"tool_name":"Read","tool_input":{"file_path":"a.go"},"session_id":"s"}`
	out := precheckOutput(store, root, strings.NewReader(in))
	if strings.Contains(out, "grafo de código") {
		t.Errorf("sin el env var de opt-in el grafo NO debe inyectarse, obtuve %q", out)
	}
}
