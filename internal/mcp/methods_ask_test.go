package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
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

// TestCitedSources: sources devuelve SÓLO las memorias citadas — por id completo o por el prefijo de
// 8 hex de un uuid ([822784c1]) — y NO las que quedaron sin citar en el grounding.
func TestCitedSources(t *testing.T) {
	items := []memory.RecallItem{
		{ID: "822784c1-e66c-4f22-a505-bedee7e62716"}, // citada por prefijo
		{ID: "8abb93fe-8405-4fb9-af56-31bf7200b31d"}, // citada por id completo
		{ID: "deadbeef-0000-0000-0000-000000000000"}, // NO citada
		{ID: "commit-b1dbbc4a72a06659"},              // no-uuid, sin cita
	}
	answer := "El extractor [822784c1] extiende el B1 [8abb93fe-8405-4fb9-af56-31bf7200b31d]."
	got := citedSources(answer, items)
	if len(got) != 2 || got[0] != items[0].ID || got[1] != items[1].ID {
		t.Errorf("citedSources=%v, esperaba [%s %s]", got, items[0].ID, items[1].ID)
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
