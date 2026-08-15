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
	answer             string // respuesta a devolver (si vacío, una por default)
}

func (f *fakeCognition) Name() string { return "llm:fake" }
func (f *fakeCognition) Ask(_ context.Context, system, user string) (string, error) {
	f.called = true
	f.gotSystem, f.gotUser = system, user
	if f.answer != "" {
		return f.answer, nil
	}
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

func recallRes(ids ...string) memory.RecallResult {
	items := make([]memory.RecallItem, len(ids))
	for i, id := range ids {
		items[i] = memory.RecallItem{ID: id, Gist: "gist de " + id}
	}
	return memory.RecallResult{Count: len(items), Items: items}
}

// TestRerankDisabledIsNoop: con la flag apagada (default), el recall queda intacto y NO se llama al motor.
func TestRerankDisabledIsNoop(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{answer: `["c","b","a"]`}
	s.cognition = fake // motor presente...
	// ...pero ReadTimeRerank sigue en false (default)
	in := recallRes("a", "b", "c")
	out := s.rerankIfEnabled(context.Background(), "q", in)
	if fake.called {
		t.Error("con la flag apagada NO se debe invocar el juez")
	}
	if out.Items[0].ID != "a" || out.Items[2].ID != "c" {
		t.Errorf("orden alterado con la flag apagada: %v", out.Items)
	}
}

// rerankOn es el true al que apuntan los tests que prenden el juez. ReadTimeRerank es *bool desde
// F5 para que el dial de potencia distinga "no lo escribieron" de "lo apagaron a mano".
var rerankOn = true

// TestRerankReordersByJudge: con la flag encendida, el juez re-ordena el tope según su array de ids;
// los no mencionados quedan al final sin perderse.
func TestRerankReordersByJudge(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `El orden es ["c","a"]`} // 'b' omitido por el juez
	s.cognitionCfg.ReadTimeRerank = &rerankOn
	out := s.rerankIfEnabled(context.Background(), "q-reorder", recallRes("a", "b", "c"))
	got := []string{out.Items[0].ID, out.Items[1].ID, out.Items[2].ID}
	// c y a primero (orden del juez), b preservado al final.
	if got[0] != "c" || got[1] != "a" || got[2] != "b" {
		t.Errorf("reorden inesperado: %v (esperaba [c a b])", got)
	}
}

// TestRerankBadJSONFallsBack: si el juez no devuelve un array parseable, se mantiene el orden model-free.
func TestRerankBadJSONFallsBack(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: "no tengo idea, no hay json"}
	s.cognitionCfg.ReadTimeRerank = &rerankOn
	out := s.rerankIfEnabled(context.Background(), "q-badjson", recallRes("a", "b", "c"))
	if out.Items[0].ID != "a" || out.Items[1].ID != "b" || out.Items[2].ID != "c" {
		t.Errorf("ante JSON malo debería mantener el orden model-free, got %v", out.Items)
	}
}

// recallResConPuntaje arma un recall con puntajes DECRECIENTES, que es como sale del camino
// model-free: el orden y el número cuentan la misma historia.
func recallResConPuntaje(ids ...string) memory.RecallResult {
	res := recallRes(ids...)
	for i := range res.Items {
		res.Items[i].Score = 1.0 - float64(i)/10.0
	}
	return res
}

// EL BUG QUE ESTE TEST FIJA: el juez re-ordenaba los items y dejaba intacto el `score` model-free,
// así que la respuesta salía con un orden y un número que se contradecían. Cualquier consumidor que
// ordenara por `score` deshacía, en silencio, un juicio que cuesta ~8,5 s.
func TestRerankBorraElPuntajeQueYaNoExplicaElOrden(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `["c","a","b"]`}
	s.cognitionCfg.ReadTimeRerank = &rerankOn

	out := s.rerankIfEnabled(context.Background(), "q-puntaje", recallResConPuntaje("a", "b", "c"))

	if !out.Reranked {
		t.Error("el reordenamiento no se declaró: sin `reranked` el caller no puede explicar por qué falta el score")
	}
	for _, it := range out.Items {
		if it.Score != 0 {
			t.Errorf("item %s conservó el puntaje model-free (%v) después de que lo reordenara el juez", it.ID, it.Score)
		}
	}
}

// La contracara: sin juez, el puntaje es la ÚNICA explicación del orden y tiene que sobrevivir
// intacto. Borrarlo siempre habría cambiado el camino model-free, que es el 100 % del uso normal.
func TestSinJuezElPuntajeSobreviveIntacto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `["c","b","a"]`} // motor presente, flag apagada

	out := s.rerankIfEnabled(context.Background(), "q", recallResConPuntaje("a", "b", "c"))

	if out.Reranked {
		t.Error("se declaró un reordenamiento que no ocurrió")
	}
	if out.Items[0].Score != 1.0 {
		t.Errorf("el camino model-free perdió su puntaje: %v", out.Items[0].Score)
	}
}

// EL CASO QUE IMPORTA MÁS QUE EL FELIZ: si el juez falla, el orden que se devuelve ES el model-free,
// así que el puntaje vuelve a explicarlo y NO se puede borrar ni declarar un reorden que no pasó.
// Fallar de la manera equivocada acá dejaría al caller sin ninguna forma de ordenar.
func TestJuezCaidoConservaPuntajeYNoDeclaraReorden(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: "el motor devolvió cualquier cosa"}
	s.cognitionCfg.ReadTimeRerank = &rerankOn

	out := s.rerankIfEnabled(context.Background(), "q-caido", recallResConPuntaje("a", "b", "c"))

	if out.Reranked {
		t.Error("el juez falló y aun así se declaró reordenado")
	}
	if out.Items[0].Score != 1.0 || out.Items[2].Score != 0.8 {
		t.Errorf("el juez falló y se perdieron los puntajes model-free: %v", out.Items)
	}
}

// La COLA (lo que queda fuera del top-K que ve el juez) no la reordena nadie, así que su puntaje
// sigue explicando su orden y tiene que quedar en pie.
func TestLaColaFueraDelTopKConservaSuPuntaje(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `["b","a"]`}
	s.cognitionCfg.ReadTimeRerank = &rerankOn
	s.cognitionCfg.ReadTimeRerankTopK = 2 // el juez sólo ve 'a' y 'b'; 'c' queda en la cola

	out := s.rerankIfEnabled(context.Background(), "q-cola", recallResConPuntaje("a", "b", "c"))

	if out.Items[2].ID != "c" {
		t.Fatalf("la cola se movió: %v", out.Items)
	}
	if out.Items[2].Score != 0.8 {
		t.Errorf("la cola perdió su puntaje sin que nadie la reordenara: %v", out.Items[2].Score)
	}
}
