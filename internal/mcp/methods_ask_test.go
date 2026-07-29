package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// fakeCognition es un motor de cognición de prueba: registra el prompt que recibió y devuelve una
// respuesta fija, para verificar el grounding sin depender de un LLM real.
type fakeCognition struct {
	called             bool
	gotSystem, gotUser string
}

func (f *fakeCognition) Name() string { return "llm:fake" }
func (f *fakeCognition) Ask(_ context.Context, system, user string) (string, error) {
	f.called = true
	f.gotSystem, f.gotUser = system, user
	return "Musubi es un servidor MCP de memoria.", nil
}

// TestAskRequiresCognitionEngine: con el pilar apagado (NoopProvider por default), musubi_ask falla
// explícito en vez de degradar mudo — el caller debe caer a musubi_recall (model-free).
func TestAskRequiresCognitionEngine(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, rerr := s.toolAsk(context.Background(), json.RawMessage(`{"question":"que es musubi"}`)); rerr == nil {
		t.Error("sin motor de cognición, musubi_ask debería fallar (opt-in)")
	}
}

// TestAskGroundsInMemory: con un motor configurado, musubi_ask recupera la memoria relevante y la
// mete en el prompt (RAG), y el system exige citar los ids. Prueba el camino end-to-end sin LLM real.
func TestAskGroundsInMemory(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{}
	s.cognition = fake
	ctx := context.Background()

	// Sembrar una memoria que matchee la pregunta (recall léxico, sin embedder).
	if _, rerr := s.toolSaveObservation(ctx, json.RawMessage(`{"topic_key":"arquitectura","content":"Musubi es un servidor MCP de memoria persistente escrito en Go."}`)); rerr != nil {
		t.Fatalf("save_observation: %v", rerr)
	}

	if _, rerr := s.toolAsk(ctx, json.RawMessage(`{"question":"que es Musubi"}`)); rerr != nil {
		t.Fatalf("ask: %v", rerr)
	}
	if !fake.called {
		t.Fatal("el motor de cognición debió ser invocado")
	}
	if !strings.Contains(fake.gotUser, "servidor MCP de memoria persistente") {
		t.Errorf("el prompt debía FUNDAMENTARSE en la memoria recuperada; user=%q", fake.gotUser)
	}
	if !strings.Contains(strings.ToLower(fake.gotSystem), "cit") {
		t.Errorf("el system debía instruir citar ids; system=%q", fake.gotSystem)
	}
}

// TestAskWithoutRelevantMemorySkipsLLM: si el recall no trae nada, NO se llama al motor (evita
// alucinar y gastar una llamada). newTestServer arranca sin observaciones.
func TestAskWithoutRelevantMemorySkipsLLM(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{}
	s.cognition = fake
	if _, rerr := s.toolAsk(context.Background(), json.RawMessage(`{"question":"xyzzy inexistente qwertzuiop"}`)); rerr != nil {
		t.Fatalf("ask: %v", rerr)
	}
	if fake.called {
		t.Error("sin memoria relevante NO se debería invocar el motor")
	}
}
