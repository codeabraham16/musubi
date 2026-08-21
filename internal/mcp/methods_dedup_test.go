package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// Tests del AFILADOR del acervo (pilar 'Musubi Renaissance', musubi_sharpen) y del anti-gemelo del
// destilador. Cubren: el gate admin + opt-in por motor, el MERGE que archiva la gemela más débil, el KEEP
// que marca not_duplicate (y así no se re-juzga), el dry-run que no llama al juez, el parser del veredicto,
// y que el destilador NO escribe una tarjeta si ya existe una casi idéntica.

// seedCard guarda una tarjeta curada con un vector explícito (para controlar el coseno en los tests).
func seedCard(t *testing.T, s *McpServer, id, slug, content string, vec []float32) {
	t.Helper()
	if err := s.engine.SaveObservationTypedFrom(designCorpusScope, "seed", id, "design-corpus/"+slug, content, 1.0, "semantic", s.defaultScope(), vec); err != nil {
		t.Fatalf("seed card %s: %v", id, err)
	}
}

func callSharpen(t *testing.T, s *McpServer, p *Principal, args map[string]any) (dedupReport, *RpcError) {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, rpcErr := s.toolSharpen(withPrincipal(context.Background(), p), raw)
	if rpcErr != nil {
		return dedupReport{}, rpcErr
	}
	r, ok := res.(CallToolResponse)
	if !ok || len(r.Content) == 0 {
		t.Fatalf("resultado no es CallToolResponse con content: %#v", res)
	}
	var rep dedupReport
	if err := json.Unmarshal([]byte(r.Content[0].Text), &rep); err != nil {
		t.Fatalf("no pude decodificar el reporte: %v (text=%s)", err, r.Content[0].Text)
	}
	return rep, nil
}

func TestSharpenRequiereAdminYMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `{"verdict":"KEEP"}`}

	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "musubi"}
	if _, rpcErr := callSharpen(t, s, writer, nil); rpcErr == nil || rpcErr.Code != codeUnauthorized {
		t.Errorf("un writer no-admin no debe poder afilar; obtuve %+v", rpcErr)
	}

	admin := &Principal{Name: "root", Role: RoleAdmin}
	sNoop := newTestServer(t, embedding.NoopProvider{}) // cognition default = noop ⇒ opt-in falla
	if _, rpcErr := callSharpen(t, sNoop, admin, nil); rpcErr == nil {
		t.Error("sin motor de cognición, musubi_sharpen debe fallar (opt-in)")
	}
}

func TestSharpenMergeArchivaLaGemela(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `{"verdict":"MERGE"}`}
	seedCard(t, s, "c1", "contraste-a", "contraste mínimo 4.5:1", []float32{1, 0, 0, 0})
	seedCard(t, s, "c2", "contraste-b", "el texto necesita 4.5:1 de contraste", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "pairs": 5})
	if rpcErr != nil {
		t.Fatalf("sharpen falló: %+v", rpcErr)
	}
	if rep.Scanned != 1 || rep.Merged != 1 || rep.Kept != 0 {
		t.Fatalf("esperaba 1 candidato fusionado; scanned=%d merged=%d kept=%d", rep.Scanned, rep.Merged, rep.Kept)
	}
	// Tras fusionar, una tarjeta quedó archivada: un segundo barrido ya no ve el par.
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("tras la fusión no debía quedar par candidato; scanned=%d", rep2.Scanned)
	}
}

func TestSharpenKeepMarcaNotDuplicate(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{answer: `{"verdict":"KEEP"}`}
	s.cognition = fake
	seedCard(t, s, "c1", "jerarquia-a", "una cosa manda por pantalla", []float32{1, 0, 0, 0})
	seedCard(t, s, "c2", "jerarquia-b", "el foco visual va a lo importante", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "pairs": 5})
	if rpcErr != nil {
		t.Fatalf("sharpen falló: %+v", rpcErr)
	}
	if rep.Merged != 0 || rep.Kept != 1 {
		t.Fatalf("con veredicto KEEP no debe fusionar; merged=%d kept=%d", rep.Merged, rep.Kept)
	}
	// El par quedó marcado not_duplicate: un segundo barrido no lo re-juzga (no gasta motor).
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("un par KEEP no debe volver a proponerse; scanned=%d", rep2.Scanned)
	}
}

func TestSharpenDryRunNoLlamaMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{answer: `{"verdict":"MERGE"}`}
	s.cognition = fake
	seedCard(t, s, "c1", "a", "uno", []float32{1, 0, 0, 0})
	seedCard(t, s, "c2", "b", "dos", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rpcErr != nil {
		t.Fatalf("dry-run falló: %+v", rpcErr)
	}
	if fake.called {
		t.Error("dry-run NO debe invocar el juez")
	}
	if rep.Scanned != 1 || rep.Merged != 0 || len(rep.Pairs) != 1 || rep.Pairs[0].Action != "dry_run" {
		t.Fatalf("dry-run inesperado: %+v", rep)
	}
}

// TestSharpenCascadaEnLote: en un cluster de 3 tarjetas mutuamente parecidas, cuando una se fusiona, el
// par que la vuelve a tocar se SALTEA sin gastar el juez (guard `gone`), y el reporte no lo mislabelea.
func TestSharpenCascadaEnLote(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `{"verdict":"MERGE"}`}
	// A, B, C casi paralelas (coseno mutuo > 0.99): un cluster. Cuál queda de canónica es determinista
	// pero no importa para el test — lo que se verifica es que 3 pares colapsan con 2 fusiones + 1 salteo.
	seedCard(t, s, "A", "contraste-a", "contraste 4.5:1", []float32{1, 0, 0, 0})
	seedCard(t, s, "B", "contraste-b", "el texto necesita 4.5:1", []float32{0.99, 0.14, 0, 0})
	seedCard(t, s, "C", "contraste-c", "ratio mínimo 4.5 a 1 en texto", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "pairs": 10})
	if rpcErr != nil {
		t.Fatalf("sharpen falló: %+v", rpcErr)
	}
	// De 3 pares candidatos, 2 fusiones colapsan el cluster a 1 tarjeta; el par sobrante se saltea.
	if rep.Merged != 2 {
		t.Fatalf("un cluster de 3 debe colapsar con 2 fusiones; merged=%d (%+v)", rep.Merged, rep.Pairs)
	}
	skipped := 0
	for _, p := range rep.Pairs {
		if p.Action == "skipped: una de las dos ya se fusionó en esta tanda" {
			skipped++
			if p.Verdict != "" {
				t.Errorf("un par salteado NO debe haber llamado al juez (Verdict vacío); obtuve %q", p.Verdict)
			}
		}
	}
	if skipped != 1 {
		t.Errorf("esperaba exactamente 1 par salteado por el guard de cascada; obtuve %d", skipped)
	}
	// El cluster quedó colapsado: un segundo barrido no ve más pares.
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("tras colapsar el cluster no debía quedar par; scanned=%d", rep2.Scanned)
	}
}

func TestParseDedupVerdict(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"verdict":"MERGE"}`, dedupMerge},
		{`{"verdict":"KEEP"}`, dedupKeep},
		{"algo de prosa {\"verdict\":\"merge\"} y más", dedupMerge}, // minúsculas + envoltorio
		{"creo que son iguales, MERGE", dedupMerge},                 // fallback por palabra
		{"no tengo idea", dedupKeep},                                // default fail-safe
		{"", dedupKeep},
	}
	for _, c := range cases {
		if got := parseDedupVerdict(c.in); got != c.want {
			t.Errorf("parseDedupVerdict(%q) = %q, esperaba %q", c.in, got, c.want)
		}
	}
}

// TestDistillAntiGemelo: si al destilar la tarjeta se parece a una que YA existe (coseno >= piso), NO se
// escribe la gemela — el blob se absorbe como corroboración (queda marcado, no reprocesable).
func TestDistillAntiGemelo(t *testing.T) {
	// El embedder fake devuelve SIEMPRE el mismo vector, así la tarjeta destilada calca a la existente.
	s := newTestServer(t, fakeEmbedder{vec: []float32{1, 0, 0, 0}})
	s.cognition = &fakeCognition{answer: `[{"slug":"contraste","content":"contraste 4.5:1"}]`}
	seedCard(t, s, "existente", "contraste-ya", "ya existe una tarjeta de contraste", []float32{1, 0, 0, 0})
	seedBlob(t, s, "b1", "ingested/web/x", "un artículo sobre contraste")

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callDistill(t, s, admin, map[string]any{"limit": 5})
	if rpcErr != nil {
		t.Fatalf("distill falló: %+v", rpcErr)
	}
	// La tarjeta gemela NO se escribió, pero el blob quedó marcado (no queda backlog).
	if rep.Distilled != 0 || rep.Cards != 0 || rep.Remaining != 0 {
		t.Fatalf("la gemela debía saltearse y el blob quedar absorbido; distilled=%d cards=%d remaining=%d (%+v)", rep.Distilled, rep.Cards, rep.Remaining, rep)
	}
	if _, found, _ := s.engine.LatestObservationByTopicInProject("design-corpus/contraste", designCorpusScope); found {
		t.Error("no debía escribirse la tarjeta gemela design-corpus/contraste")
	}
	// Re-correr sigue siendo no-op (el blob ya tiene su arista de corroboración).
	rep2, _ := callDistill(t, s, admin, map[string]any{"limit": 5})
	if rep2.Distilled != 0 || rep2.Remaining != 0 {
		t.Fatalf("re-run tras absorber debía ser no-op; distilled=%d remaining=%d", rep2.Distilled, rep2.Remaining)
	}
}
