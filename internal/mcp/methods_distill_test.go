package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// Tests del DESTILADOR del acervo (pilar 'Musubi Renaissance', musubi_distill). Cubren: el gate admin, el
// opt-in por motor, el camino end-to-end (blob crudo → tarjetas curadas + arista de procedencia), la
// idempotencia al re-correr, el dry-run que no escribe, y el parser tolerante de la salida del motor.

func seedBlob(t *testing.T, s *McpServer, id, topic, content string) {
	t.Helper()
	if err := s.engine.SaveObservationTypedFrom(designCorpusScope, "seed", id, topic, content, 1.0, "semantic", s.defaultScope(), nil); err != nil {
		t.Fatalf("seed blob %s: %v", id, err)
	}
}

func callDistill(t *testing.T, s *McpServer, p *Principal, args map[string]any) (distillReport, *RpcError) {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, rpcErr := s.toolDistill(withPrincipal(context.Background(), p), raw)
	if rpcErr != nil {
		return distillReport{}, rpcErr
	}
	r, ok := res.(CallToolResponse)
	if !ok || len(r.Content) == 0 {
		t.Fatalf("resultado no es CallToolResponse con content: %#v", res)
	}
	var rep distillReport
	if err := json.Unmarshal([]byte(r.Content[0].Text), &rep); err != nil {
		t.Fatalf("no pude decodificar el reporte: %v (text=%s)", err, r.Content[0].Text)
	}
	return rep, nil
}

// TestDistillRequiereAdminYMotor: un no-admin se rechaza; y con motor apagado (opt-in) falla explícito.
func TestDistillRequiereAdminYMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `[{"slug":"x","content":"y"}]`}

	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "musubi"}
	if _, rpcErr := callDistill(t, s, writer, nil); rpcErr == nil || rpcErr.Code != codeUnauthorized {
		t.Errorf("un writer no-admin no debe poder destilar; obtuve %+v", rpcErr)
	}

	// Motor apagado ⇒ error explícito aún para admin.
	s.cognition = nil // sin motor real
	admin := &Principal{Name: "root", Role: RoleAdmin}
	sNoop := newTestServer(t, embedding.NoopProvider{}) // cognition default = NoopProvider
	if _, rpcErr := callDistill(t, sNoop, admin, nil); rpcErr == nil {
		t.Error("sin motor de cognición, musubi_distill debe fallar (opt-in)")
	}
}

// TestDistillCreaTarjetasYEsIdempotente: destila blobs crudos en tarjetas curadas + arista derived_from,
// y re-correr no reprocesa (idempotente por el marcador).
func TestDistillCreaTarjetasYEsIdempotente(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// El fake devuelve el MISMO array para cualquier blob, con prosa y un slug "sucio" (espacios) para
	// probar el parser tolerante y el saneo del slug.
	s.cognition = &fakeCognition{answer: `claro, acá van: [{"slug":"jerarquia-clara","content":"Una sola cosa manda por pantalla."},{"slug":"Espaciado 4pt","content":"Todo en múltiplos de 4."}] listo.`}

	seedBlob(t, s, "b1", "ingested/youtube/aaa", "un artículo sobre jerarquía")
	seedBlob(t, s, "b2", "ingested/web/bbb", "otro artículo de diseño")

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callDistill(t, s, admin, map[string]any{"limit": 5})
	if rpcErr != nil {
		t.Fatalf("distill falló: %+v", rpcErr)
	}
	if rep.Distilled != 2 || rep.Cards != 4 || rep.Remaining != 0 {
		t.Fatalf("reporte inesperado: distilled=%d cards=%d remaining=%d (%+v)", rep.Distilled, rep.Cards, rep.Remaining, rep)
	}

	// Las tarjetas existen en el acervo, con el slug saneado ("Espaciado 4pt" → "espaciado-4pt").
	if _, found, _ := s.engine.LatestObservationByTopicInProject("design-corpus/jerarquia-clara", designCorpusScope); !found {
		t.Error("falta la tarjeta design-corpus/jerarquia-clara")
	}
	if _, found, _ := s.engine.LatestObservationByTopicInProject("design-corpus/espaciado-4pt", designCorpusScope); !found {
		t.Error("el slug con mayúsculas y espacio no se saneó a design-corpus/espaciado-4pt")
	}

	// Re-correr: idempotente. Los blobs ya tienen su arista, así que no hay nada pendiente.
	rep2, rpcErr := callDistill(t, s, admin, map[string]any{"limit": 5})
	if rpcErr != nil {
		t.Fatalf("re-run falló: %+v", rpcErr)
	}
	if rep2.Distilled != 0 || rep2.Remaining != 0 {
		t.Fatalf("re-run debía ser no-op: distilled=%d remaining=%d", rep2.Distilled, rep2.Remaining)
	}
}

// TestDistillDryRunNoEscribe: dry_run lista lo que se destilaría sin llamar al motor ni escribir nada.
func TestDistillDryRunNoEscribe(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{answer: `[{"slug":"x","content":"y"}]`}
	s.cognition = fake
	seedBlob(t, s, "b1", "ingested/youtube/aaa", "artículo")

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callDistill(t, s, admin, map[string]any{"dry_run": true})
	if rpcErr != nil {
		t.Fatalf("dry-run falló: %+v", rpcErr)
	}
	if fake.called {
		t.Error("dry-run NO debe invocar el motor")
	}
	if rep.Distilled != 0 || rep.Remaining != 1 || len(rep.Blobs) != 1 {
		t.Fatalf("dry-run inesperado: %+v", rep)
	}
	// Nada se escribió: el blob sigue pendiente en una corrida real.
	rep2, _ := callDistill(t, s, admin, map[string]any{"limit": 5})
	if rep2.Distilled != 1 {
		t.Fatalf("tras dry-run el blob debía seguir pendiente; distilled=%d", rep2.Distilled)
	}
}

// TestParseDistillCards: el parser recorta el array JSON entre prosa/fences, descarta tarjetas vacías,
// sanea slugs (acentos, mayúsculas, símbolos) y corta a distillMaxCards.
func TestParseDistillCards(t *testing.T) {
	// Prosa + fence alrededor del array; una tarjeta con content vacío se descarta.
	in := "```json\n[{\"slug\":\"Jerarquía Visual\",\"content\":\"algo\"},{\"slug\":\"vacía\",\"content\":\"  \"},{\"slug\":\"contraste-por-peso\",\"content\":\"otra\"}]\n```"
	cards := parseDistillCards(in)
	if len(cards) != 2 {
		t.Fatalf("esperaba 2 tarjetas (la vacía se descarta), obtuve %d: %+v", len(cards), cards)
	}
	if cards[0].Slug != "jerarquia-visual" {
		t.Errorf("slug mal saneado: %q (esperaba jerarquia-visual)", cards[0].Slug)
	}

	// Sin array ⇒ nil (el caller lo trata como "sin tarjetas", no error).
	if parseDistillCards("no hay ningún json acá") != nil {
		t.Error("sin array JSON, parseDistillCards debe devolver nil")
	}

	// Tope de tarjetas.
	var b []byte
	b = append(b, '[')
	for i := 0; i < distillMaxCards+3; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(`{"slug":"s`+string(rune('a'+i))+`","content":"c"}`)...)
	}
	b = append(b, ']')
	if got := parseDistillCards(string(b)); len(got) != distillMaxCards {
		t.Errorf("esperaba corte a %d tarjetas, obtuve %d", distillMaxCards, len(got))
	}
}
