package embedding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"musubi/internal/config"
)

func TestNoopProviderReturnsDisabled(t *testing.T) {
	var p Provider = NoopProvider{}
	if _, err := p.Embed(context.Background(), "hola"); !errors.Is(err, ErrEmbeddingDisabled) {
		t.Fatalf("esperaba ErrEmbeddingDisabled, obtuve %v", err)
	}
	if Enabled(p) {
		t.Error("NoopProvider no debería contar como Enabled")
	}
}

func TestOllamaProviderEmbedSuccess(t *testing.T) {
	var gotModel, gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		gotPrompt = body.Prompt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": []float32{0.1, 0.2, 0.3}})
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "nomic-embed-text", 3)
	if !Enabled(p) {
		t.Error("OllamaProvider debería contar como Enabled")
	}

	vec, err := p.Embed(context.Background(), "texto de prueba")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("esperaba 3 componentes, obtuve %d", len(vec))
	}
	if gotModel != "nomic-embed-text" || gotPrompt != "texto de prueba" {
		t.Errorf("request inesperado: model=%q prompt=%q", gotModel, gotPrompt)
	}
}

func TestOllamaProviderNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "m", 3)
	if _, err := p.Embed(context.Background(), "x"); err == nil {
		t.Fatal("esperaba error por status no-200")
	}
}

func TestOllamaProviderContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": []float32{1}})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelado antes de llamar
	p := NewOllamaProvider(srv.URL, "m", 1)
	if _, err := p.Embed(ctx, "x"); err == nil {
		t.Fatal("esperaba error por contexto cancelado")
	}
}

func TestNewProviderFactory(t *testing.T) {
	none, err := NewProvider(config.EmbeddingConfig{Provider: ""})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if _, ok := none.(NoopProvider); !ok {
		t.Errorf("provider vacío debería ser NoopProvider, fue %T", none)
	}

	oll, err := NewProvider(config.EmbeddingConfig{Provider: "ollama", BaseURL: "http://x", Model: "m", Dimensions: 768})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if oll.Name() != "ollama" {
		t.Errorf("esperaba ollama, obtuve %q", oll.Name())
	}

	oai, err := NewProvider(config.EmbeddingConfig{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if oai.Name() != "openai" {
		t.Errorf("esperaba openai, obtuve %q", oai.Name())
	}
	if !Enabled(oai) {
		t.Error("OpenAIProvider debería contar como Enabled")
	}

	if _, err := NewProvider(config.EmbeddingConfig{Provider: "desconocido"}); err == nil {
		t.Fatal("esperaba error para proveedor desconocido")
	}
}

func TestOpenAIProviderEmbedSuccess(t *testing.T) {
	var gotModel, gotInput, gotAuth string
	var gotDim int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model      string `json:"model"`
			Input      string `json:"input"`
			Dimensions int    `json:"dimensions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel, gotInput, gotDim = body.Model, body.Input, body.Dimensions
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"embedding": []float32{0.1, 0.2, 0.3, 0.4}}},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "text-embedding-3-small", "sk-test", 4)
	vec, err := p.Embed(context.Background(), "hola mundo")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vec) != 4 {
		t.Fatalf("esperaba 4 componentes, obtuve %d", len(vec))
	}
	if gotModel != "text-embedding-3-small" || gotInput != "hola mundo" || gotDim != 4 {
		t.Errorf("request inesperado: model=%q input=%q dim=%d", gotModel, gotInput, gotDim)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("esperaba header Bearer, obtuve %q", gotAuth)
	}
}

func TestOpenAIProviderDefaults(t *testing.T) {
	p := NewOpenAIProvider("", "", "", 0)
	if p.baseURL != defaultOpenAIBaseURL {
		t.Errorf("esperaba base_url default %q, obtuve %q", defaultOpenAIBaseURL, p.baseURL)
	}
	if p.model != defaultOpenAIModel {
		t.Errorf("esperaba model default %q, obtuve %q", defaultOpenAIModel, p.model)
	}
}

func TestOpenAIProviderAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid API key"},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "m", "bad", 0)
	_, err := p.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("esperaba error por status no-200")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("esperaba el mensaje de la API en el error, obtuve %v", err)
	}
}
