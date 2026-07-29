package cognition

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

// TestNoopAskDisabled: sin motor, Ask falla explícito con ErrCognitionDisabled (no degrada mudo).
func TestNoopAskDisabled(t *testing.T) {
	_, err := (NoopProvider{}).Ask(context.Background(), "sys", "user")
	if !errors.Is(err, ErrCognitionDisabled) {
		t.Fatalf("NoopProvider.Ask: esperaba ErrCognitionDisabled, obtuve %v", err)
	}
}

// TestOpenAICompatAskSuccess: arma un chat de 2 mensajes contra un endpoint OpenAI-compatible y
// devuelve el content de la primera choice. Verifica que manda model + Bearer + el system y user.
func TestOpenAICompatAskSuccess(t *testing.T) {
	var gotModel, gotAuth string
	var gotRoles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		for _, m := range body.Messages {
			gotRoles = append(gotRoles, m.Role)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "  respuesta fundamentada  "}},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(srv.URL, "sonnet", "sk-cog-xyz", 0)
	if !Enabled(p) {
		t.Error("OpenAICompatProvider debería contar como habilitado")
	}
	if p.Name() != "llm:sonnet" {
		t.Errorf("Name()=%q, esperaba llm:sonnet", p.Name())
	}

	ans, err := p.Ask(context.Background(), "sos el asistente", "que es musubi")
	if err != nil {
		t.Fatalf("Ask error: %v", err)
	}
	if ans != "respuesta fundamentada" { // trim aplicado
		t.Errorf("respuesta=%q, esperaba 'respuesta fundamentada' (trim)", ans)
	}
	if gotModel != "sonnet" {
		t.Errorf("model=%q, esperaba sonnet", gotModel)
	}
	if gotAuth != "Bearer sk-cog-xyz" {
		t.Errorf("auth=%q, esperaba Bearer sk-cog-xyz", gotAuth)
	}
	if len(gotRoles) != 2 || gotRoles[0] != "system" || gotRoles[1] != "user" {
		t.Errorf("roles=%v, esperaba [system user]", gotRoles)
	}
}

// TestOpenAICompatAskOmitsEmptySystem: sin system, se manda sólo el mensaje user.
func TestOpenAICompatAskOmitsEmptySystem(t *testing.T) {
	var roles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			roles = append(roles, m.Role)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(srv.URL, "haiku", "", 0)
	if _, err := p.Ask(context.Background(), "  ", "hola"); err != nil {
		t.Fatalf("Ask error: %v", err)
	}
	if len(roles) != 1 || roles[0] != "user" {
		t.Errorf("roles=%v, esperaba sólo [user]", roles)
	}
}

// TestOpenAICompatAskAPIError: un status no-200 con {"error":{"message":...}} propaga el mensaje.
func TestOpenAICompatAskAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid master key"},
		})
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(srv.URL, "sonnet", "bad", 0)
	_, err := p.Ask(context.Background(), "", "x")
	if err == nil || !strings.Contains(err.Error(), "Invalid master key") {
		t.Fatalf("esperaba error con el mensaje de la API, obtuve %v", err)
	}
}

// TestFactoryBuildsOpenAICompat: provider 'openai-compat' arma el motor real y lee la master key
// de la env var nombrada en auth_token_env (nunca del yaml).
func TestFactoryBuildsOpenAICompat(t *testing.T) {
	t.Setenv("COG_TEST_KEY", "sk-cog-fromenv")
	p, err := NewProvider(config.CognitionConfig{
		Provider:     "openai-compat",
		Endpoint:     "http://127.0.0.1:4000/v1",
		Model:        "sonnet",
		AuthTokenEnv: "COG_TEST_KEY",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if !Enabled(p) {
		t.Fatal("openai-compat debería estar habilitado")
	}
	oc, ok := p.(*OpenAICompatProvider)
	if !ok {
		t.Fatalf("esperaba *OpenAICompatProvider, obtuve %T", p)
	}
	if oc.apiKey != "sk-cog-fromenv" {
		t.Errorf("apiKey=%q, esperaba la de la env var", oc.apiKey)
	}
}
