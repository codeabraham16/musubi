package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/skills"

	"gopkg.in/yaml.v3"
)

// fakeEmbedder devuelve siempre el mismo vector; cuenta como Enabled (no es Noop).
type fakeEmbedder struct {
	vec []float32
	err error
}

func (f fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vec, nil
}
func (f fakeEmbedder) Dimensions() int { return len(f.vec) }
func (f fakeEmbedder) Name() string    { return "fake" }

func newTestServer(t *testing.T, embedder embedding.Provider) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return NewMcpServer(engine, t.TempDir(), embedder)
}

func TestSaveSkillActualizaHuellaDeStack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module ej\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{})

	_, rpcErr := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":        "mi-skill",
		"description": "una skill de prueba",
		"triggers":    []string{"*.go"},
		"rules":       "Regla suficientemente larga para pasar la validación.",
	})
	if rpcErr != nil {
		t.Fatalf("save_skill devolvió error: %+v", rpcErr)
	}

	fp, ok, err := engine.GetMeta(memory.MetaStackFingerprint)
	if err != nil {
		t.Fatalf("GetMeta error: %v", err)
	}
	if !ok || fp == "" {
		t.Errorf("save_skill debe persistir la huella del stack en meta, obtuve ok=%v fp=%q", ok, fp)
	}
	if !strings.Contains(fp, "Go") {
		t.Errorf("la huella debe reflejar el stack Go detectado, obtuve %q", fp)
	}
}

func call(t *testing.T, s *McpServer, name string, args map[string]interface{}) (interface{}, *RpcError) {
	t.Helper()
	argBytes, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: name, Arguments: argBytes})
	return s.handleToolsCall(context.Background(), params)
}

func TestSaveObservationRequiresFields(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"content": "x"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por topic_key faltante, obtuve %+v", e)
	}
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por content faltante, obtuve %+v", e)
	}
}

func TestSaveObservationAutoUUID(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "hola mundo"})
	if e != nil {
		t.Fatalf("error inesperado: %+v", e)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 || !strings.Contains(resp.Content[0].Text, "id:") {
		t.Fatalf("esperaba confirmación con id, obtuve %+v", res)
	}
}

func TestSearchSemanticDisabledPointsToKeyword(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, e := call(t, s, "musubi_search_semantic", map[string]interface{}{"query": "algo"})
	if e == nil || e.Code != codeInvalidParams {
		t.Fatalf("esperaba invalid params con embeddings desactivados, obtuve %+v", e)
	}
	if !strings.Contains(e.Message, "keyword") {
		t.Errorf("el mensaje debería sugerir keyword, fue %q", e.Message)
	}
}

func TestSearchSemanticWithEmbedder(t *testing.T) {
	emb := fakeEmbedder{vec: []float32{1, 0, 0}}
	s := newTestServer(t, emb)

	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "documento alfa"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	res, e := call(t, s, "musubi_search_semantic", map[string]interface{}{"query": "buscar"})
	if e != nil {
		t.Fatalf("search error: %+v", e)
	}
	resp := res.(CallToolResponse)
	if !strings.Contains(resp.Content[0].Text, "documento alfa") {
		t.Errorf("esperaba encontrar la observación guardada, obtuve %q", resp.Content[0].Text)
	}
}

func TestSearchKeyword(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "patrón singleton en Go"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	res, e := call(t, s, "musubi_search_keyword", map[string]interface{}{"query_text": "singleton"})
	if e != nil {
		t.Fatalf("keyword error: %+v", e)
	}
	if !strings.Contains(res.(CallToolResponse).Content[0].Text, "singleton") {
		t.Error("esperaba resultado de keyword con 'singleton'")
	}
}

func TestLogAndResolveTelemetry(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_log_error", map[string]interface{}{"file_path": "a.go", "error_message": "undefined: x"}); e != nil {
		t.Fatalf("log error: %+v", e)
	}
	if _, e := call(t, s, "musubi_resolve_telemetry", map[string]interface{}{"id": 0}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params con id=0, obtuve %+v", e)
	}
	if _, e := call(t, s, "musubi_resolve_telemetry", map[string]interface{}{"id": 1}); e != nil {
		t.Fatalf("resolve error: %+v", e)
	}
}

func TestResolveSkillsEmpty(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_resolve_skills", map[string]interface{}{"modified_files": []string{}}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params con modified_files vacío, obtuve %+v", e)
	}
}

func TestToolNotFound(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "tool_inexistente", map[string]interface{}{}); e == nil || e.Code != codeMethodNotFound {
		t.Errorf("esperaba method not found, obtuve %+v", e)
	}
}

// textOf extrae el texto de una respuesta de tool.
func textOf(t *testing.T, res interface{}) string {
	t.Helper()
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	return resp.Content[0].Text
}

func TestRecallToolReturnsBudgetedItems(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "patrón observer en Go para eventos"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	res, e := call(t, s, "musubi_recall", map[string]interface{}{"query": "observer", "token_budget": 50})
	if e != nil {
		t.Fatalf("recall error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "\"budget\": 50") {
		t.Errorf("esperaba budget 50 en la respuesta, obtuve %s", txt)
	}
	if !strings.Contains(txt, "items") {
		t.Errorf("esperaba 'items' en la respuesta, obtuve %s", txt)
	}
}

func TestRecallToolRequiresQuery(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_recall", map[string]interface{}{"query": "   "}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por query vacío, obtuve %+v", e)
	}
}

func TestMemoryExpandTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"id": "x1", "topic_key": "t", "content": "contenido completo para expandir"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	res, e := call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": []string{"x1"}})
	if e != nil {
		t.Fatalf("expand error: %+v", e)
	}
	if !strings.Contains(textOf(t, res), "contenido completo para expandir") {
		t.Errorf("esperaba el contenido completo en la respuesta, obtuve %s", textOf(t, res))
	}
}

func TestMemoryExpandRequiresIds(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": []string{}}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por ids vacío, obtuve %+v", e)
	}
}

func TestTokensToolTracksHydration(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"id": "h1", "topic_key": "t", "content": "contenido razonablemente largo para gastar varios tokens al hidratar"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	if _, e := call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": []string{"h1"}}); e != nil {
		t.Fatalf("expand error: %+v", e)
	}
	res, e := call(t, s, "musubi_tokens", map[string]interface{}{"action": "status"})
	if e != nil {
		t.Fatalf("tokens status error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "\"total\"") || !strings.Contains(txt, "hydration") {
		t.Errorf("esperaba total y la superficie hydration en el ledger, obtuve %s", txt)
	}
}

func TestTokensToolReset(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"id": "h2", "topic_key": "t", "content": "algo para hidratar y contar"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": []string{"h2"}})
	if _, e := call(t, s, "musubi_tokens", map[string]interface{}{"action": "reset"}); e != nil {
		t.Fatalf("tokens reset error: %+v", e)
	}
	res, _ := call(t, s, "musubi_tokens", map[string]interface{}{"action": "status"})
	if !strings.Contains(textOf(t, res), "\"total\": 0") {
		t.Errorf("tras reset el total debe ser 0, obtuve %s", textOf(t, res))
	}
}

func TestTokensToolReportsBudget(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// El test server usa la config por defecto (session_token_budget=8000): el status
	// debe traer el estado del gobernador y el desglose por superficie.
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"id": "b1", "topic_key": "t", "content": "algo para hidratar y contar tokens"}); e != nil {
		t.Fatalf("save error: %+v", e)
	}
	call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": []string{"b1"}})
	res, e := call(t, s, "musubi_tokens", map[string]interface{}{"action": "status"})
	if e != nil {
		t.Fatalf("tokens status error: %+v", e)
	}
	txt := textOf(t, res)
	// El reporte del gobernador incluye estado y presupuesto, no solo el total crudo.
	if !strings.Contains(txt, "\"status\"") || !strings.Contains(txt, "\"budget\": 8000") {
		t.Errorf("el status debe reportar estado y presupuesto, obtuve %s", txt)
	}
	if !strings.Contains(txt, "\"surface\": \"hydration\"") {
		t.Errorf("el desglose debe listar superficies como objetos {surface,tokens,pct}, obtuve %s", txt)
	}
}

func TestTokensToolInvalidAction(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_tokens", map[string]interface{}{"action": "raro"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por action inválida, obtuve %+v", e)
	}
}

func TestCodeMemoryFreshAndStale(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(file, []byte("package foo\n\nfunc Bar() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := newTestServerWithPath(t, dir)

	// Guardar el gist del archivo.
	if _, e := call(t, s, "musubi_save_code", map[string]interface{}{
		"path": "foo.go", "gist": "Paquete foo con Bar().", "symbols": "Bar() L3",
	}); e != nil {
		t.Fatalf("save_code error: %+v", e)
	}

	// Recall: el archivo no cambió -> fresh.
	res, e := call(t, s, "musubi_recall_code", map[string]interface{}{"path": "foo.go"})
	if e != nil {
		t.Fatalf("recall_code error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "\"fresh\": true") {
		t.Errorf("el archivo sin cambios debe ser fresh, obtuve %s", txt)
	}
	if !strings.Contains(txt, "Paquete foo con Bar().") {
		t.Errorf("el recall debe devolver el gist, obtuve %s", txt)
	}

	// Modificar el archivo -> stale.
	if err := os.WriteFile(file, []byte("package foo\n\nfunc Bar() {}\nfunc Baz() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	res2, _ := call(t, s, "musubi_recall_code", map[string]interface{}{"path": "foo.go"})
	if !strings.Contains(textOf(t, res2), "\"fresh\": false") {
		t.Errorf("tras modificar el archivo el gist debe quedar no-fresco, obtuve %s", textOf(t, res2))
	}
}

func TestRecallCodeNotFound(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_recall_code", map[string]interface{}{"path": "no/existe.go"})
	if e != nil {
		t.Fatalf("recall_code error: %+v", e)
	}
	if !strings.Contains(textOf(t, res), "\"found\": false") {
		t.Errorf("un path sin memoria debe devolver found:false, obtuve %s", textOf(t, res))
	}
}

func TestSaveCodeRequiresPathAndGist(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_code", map[string]interface{}{"path": "x.go"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por gist faltante, obtuve %+v", e)
	}
}

func TestSaveAndRecallFactsTools(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	for _, f := range [][3]string{
		{"Alice", "works_at", "ACME"},
		{"Alice", "knows", "Bob"},
		{"ACME", "located_in", "NYC"},
	} {
		if _, e := call(t, s, "musubi_save_fact", map[string]interface{}{"subject": f[0], "predicate": f[1], "object": f[2]}); e != nil {
			t.Fatalf("save_fact error: %+v", e)
		}
	}

	res, e := call(t, s, "musubi_recall_facts", map[string]interface{}{"entity": "Alice", "max_hops": 2})
	if e != nil {
		t.Fatalf("recall_facts error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "works_at") || !strings.Contains(txt, "located_in") {
		t.Errorf("esperaba hechos a 2 hops (incluye located_in), obtuve %s", txt)
	}
}

func TestEntityContextTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"id": "o1", "topic_key": "infra", "content": "desplegamos kubernetes en produccion"}); e != nil {
		t.Fatalf("save obs error: %+v", e)
	}
	if _, e := call(t, s, "musubi_save_fact", map[string]interface{}{"subject": "kubernetes", "predicate": "corre_en", "object": "cloud"}); e != nil {
		t.Fatalf("save fact error: %+v", e)
	}

	res, e := call(t, s, "musubi_entity_context", map[string]interface{}{"entity": "kubernetes"})
	if e != nil {
		t.Fatalf("entity_context error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "corre_en") || !strings.Contains(txt, "observations") {
		t.Errorf("esperaba hechos + observaciones en el contexto, obtuve %s", txt)
	}
}

func TestEntityContextRequiresEntity(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_entity_context", map[string]interface{}{"entity": " "}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por entity vacío, obtuve %+v", e)
	}
}

func TestSaveFactRequiresFields(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_fact", map[string]interface{}{"subject": "A", "predicate": "p"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por object faltante, obtuve %+v", e)
	}
}

func TestRecallFactsRequiresEntity(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_recall_facts", map[string]interface{}{"entity": "  "}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params por entity vacío, obtuve %+v", e)
	}
}

func TestMaintainTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Aislar la consolidación de la detección de conflictos al guardar: el auto-supersede
	// de casi-duplicados sólo dispara si la 2da observación es ESTRICTAMENTE más nueva
	// (created_at con resolución de 1s), así que en CI lento podía ocultar una y dejar
	// el consolidate con 1 sola viva (flaky). La consolidación es lo que se prueba acá.
	s.conflicts.Enabled = false
	// Dos casi-duplicados (no exactos): el maintain debe consolidarlos.
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "el patron observer en go sirve para eventos"}); e != nil {
		t.Fatalf("save 1 error: %+v", e)
	}
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "el patron observer en go sirve para eventos."}); e != nil {
		t.Fatalf("save 2 error: %+v", e)
	}

	res, e := call(t, s, "musubi_maintain", map[string]interface{}{})
	if e != nil {
		t.Fatalf("maintain error: %+v", e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "consolidate") || !strings.Contains(txt, "decay") {
		t.Errorf("esperaba resumen con consolidate y decay, obtuve %s", txt)
	}
	if !strings.Contains(txt, "\"merged\": 1") {
		t.Errorf("esperaba 1 merge de casi-duplicados, obtuve %s", txt)
	}
}

func TestSaveObservationDedupViaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "memoria única"}); e != nil {
		t.Fatalf("save 1 error: %+v", e)
	}
	res, e := call(t, s, "musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": "memoria única"})
	if e != nil {
		t.Fatalf("save 2 error: %+v", e)
	}
	if !strings.Contains(textOf(t, res), "ya existente") {
		t.Errorf("esperaba mensaje de dedup, obtuve %s", textOf(t, res))
	}
}

// newTestServerWithPath construye un McpServer con un projectPath explícito.
func newTestServerWithPath(t *testing.T, projectPath string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return NewMcpServer(engine, projectPath, embedding.NoopProvider{})
}

// --- Tests para musubi_detect_stack ---

func TestDetectStackGoProject(t *testing.T) {
	// RED: toolDetectStack no existe aún; este test debe fallar (tool not found).
	root := t.TempDir()
	// Crear un go.mod mínimo para que el detector identifique Go.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module miproyecto\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}
	s := newTestServerWithPath(t, root)

	res, e := call(t, s, "musubi_detect_stack", map[string]interface{}{})
	if e != nil {
		t.Fatalf("esperaba éxito, obtuve error: %+v", e)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	if !strings.Contains(strings.ToLower(resp.Content[0].Text), "go") {
		t.Errorf("esperaba ecosistema Go en la respuesta, obtuve: %s", resp.Content[0].Text)
	}
}

func TestDetectStackEmptyDir(t *testing.T) {
	// Directorio vacío: debe retornar [] sin error.
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	res, e := call(t, s, "musubi_detect_stack", map[string]interface{}{})
	if e != nil {
		t.Fatalf("esperaba éxito con dir vacío, obtuve error: %+v", e)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	// El JSON debe ser un array vacío.
	if !strings.Contains(resp.Content[0].Text, "[]") {
		t.Errorf("esperaba array vacío, obtuve: %s", resp.Content[0].Text)
	}
}

func TestDetectStackUsesProjectPath(t *testing.T) {
	// Asegura que el handler usa s.projectPath cuando no se pasa path explícito.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module testmod\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}
	s := newTestServerWithPath(t, root)

	res, e := call(t, s, "musubi_detect_stack", map[string]interface{}{})
	if e != nil {
		t.Fatalf("error inesperado: %+v", e)
	}
	resp := res.(CallToolResponse)
	// El module name "testmod" debe aparecer en el JSON.
	if !strings.Contains(resp.Content[0].Text, "testmod") {
		t.Errorf("esperaba module_name 'testmod' en la respuesta, obtuve: %s", resp.Content[0].Text)
	}
}

// --- Tests para musubi_save_skill ---

func TestSaveSkillValidInputsCreatesFiles(t *testing.T) {
	// Caso feliz: guardado exitoso crea el YAML y el sentinel.
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	res, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":        "mi-skill",
		"description": "Skill de prueba",
		"triggers":    []string{"*.go"},
		"rules":       "Siempre usar fmt.Errorf para wrappear errores en Go.",
	})
	if e != nil {
		t.Fatalf("esperaba éxito, obtuve error: %+v", e)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}

	// Verificar que el archivo YAML fue creado.
	yamlPath := filepath.Join(root, config.DirName, config.SkillsDir, "mi-skill.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("no se creó el archivo YAML: %v", err)
	}

	// El YAML debe contener provenance.
	var sk skills.Skill
	if err := yaml.Unmarshal(data, &sk); err != nil {
		t.Fatalf("YAML inválido: %v", err)
	}
	if sk.GeneratedBy != "auto-discovery" {
		t.Errorf("esperaba generated_by=auto-discovery, obtuve %q", sk.GeneratedBy)
	}
	if sk.GeneratedAt == "" {
		t.Error("generated_at no debe estar vacío")
	}

	// Verificar que el sentinel existe.
	sentinelPath := filepath.Join(root, config.DirName, config.SkillsDir, config.SentinelFile)
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Errorf("sentinel no fue creado: %v", err)
	}
}

func TestSaveSkillMissingName(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"triggers": []string{"*.go"},
		"rules":    "Regla de prueba suficientemente larga para pasar validación.",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por name faltante, obtuve %+v", e)
	}
	if e != nil && !strings.Contains(strings.ToLower(e.Message), "name") {
		t.Errorf("el mensaje debe mencionar 'name', fue: %q", e.Message)
	}
}

func TestSaveSkillEmptyTriggers(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "mi-skill",
		"triggers": []string{},
		"rules":    "Regla de prueba suficientemente larga para pasar validación.",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por triggers vacío, obtuve %+v", e)
	}
}

func TestSaveSkillRulesEmpty(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "mi-skill",
		"triggers": []string{"*.go"},
		"rules":    "corto",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por rules demasiado corto, obtuve %+v", e)
	}
	if e != nil && !strings.Contains(strings.ToLower(e.Message), "rules") {
		t.Errorf("el mensaje debe mencionar 'rules', fue: %q", e.Message)
	}
}

func TestSaveSkillInvalidSlugSpaces(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "mi skill",
		"triggers": []string{"*.go"},
		"rules":    "Regla de prueba suficientemente larga para pasar validación.",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por nombre con espacios, obtuve %+v", e)
	}
}

func TestSaveSkillPathTraversalRejected(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "../evil",
		"triggers": []string{"*.go"},
		"rules":    "Regla de prueba suficientemente larga para pasar validación.",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba rechazo de path traversal, obtuve %+v", e)
	}
}

func TestSaveSkillExistingFileNoOverwrite(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	// Crear el archivo primero.
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("no se pudo crear directorio: %v", err)
	}
	existing := filepath.Join(skillsDir, "mi-skill.yaml")
	if err := os.WriteFile(existing, []byte("name: mi-skill\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear archivo existente: %v", err)
	}

	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":     "mi-skill",
		"triggers": []string{"*.go"},
		"rules":    "Regla de prueba suficientemente larga para pasar validación.",
	})
	if e == nil || e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por archivo existente sin overwrite, obtuve %+v", e)
	}
}

func TestSaveSkillExistingFileWithOverwrite(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	// Crear archivo existente.
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("no se pudo crear directorio: %v", err)
	}
	existing := filepath.Join(skillsDir, "mi-skill.yaml")
	if err := os.WriteFile(existing, []byte("name: mi-skill\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear archivo existente: %v", err)
	}

	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":        "mi-skill",
		"description": "Aplica convenciones de Go. Use when editando archivos .go.",
		"triggers":    []string{"*.go"},
		"rules":       "Regla de prueba suficientemente larga para pasar validación.",
		"overwrite":   true,
	})
	if e != nil {
		t.Errorf("esperaba éxito con overwrite=true, obtuve %+v", e)
	}
}

func TestSaveSkillRoundTrip(t *testing.T) {
	// La skill guardada debe poder cargarse con el Resolver (vía LoadSkills).
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	_, e := call(t, s, "musubi_save_skill", map[string]interface{}{
		"name":        "roundtrip-skill",
		"description": "Aplica convenciones al editar. Use when trabajando con .go o .ts.",
		"triggers":    []string{"*.go", "*.ts"},
		"rules":       "Regla de prueba suficientemente larga para verificar el round-trip completo.",
	})
	if e != nil {
		t.Fatalf("guardado falló: %+v", e)
	}

	// Usar el Resolver sobre el mismo root para verificar carga.
	resolver := skills.NewResolver(root)
	loaded, err := resolver.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills falló: %v", err)
	}

	found := false
	for _, sk := range loaded {
		if sk.Name == "roundtrip-skill" {
			found = true
			if sk.GeneratedBy != "auto-discovery" {
				t.Errorf("GeneratedBy incorrecto: %q", sk.GeneratedBy)
			}
			break
		}
	}
	if !found {
		t.Error("skill 'roundtrip-skill' no encontrada después del guardado")
	}
}
