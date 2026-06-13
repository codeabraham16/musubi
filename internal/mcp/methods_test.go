package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
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

func call(t *testing.T, s *McpServer, name string, args map[string]interface{}) (interface{}, *RpcError) {
	t.Helper()
	argBytes, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: name, Arguments: argBytes})
	return s.handleToolsCall(params)
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
