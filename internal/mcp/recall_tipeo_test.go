package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// LAS PRUEBAS DEL CORRECTOR DE TIPEO EN EL BORDE MCP. El corrector mismo se prueba en
// internal/memory/tipeo_test.go; acá se prueba lo que cada tool hace con él: qué texto embebe, qué
// texto busca, qué devuelve, qué ve el motor, y con qué vocabulario corrige cada credencial.

// embebedorQueAnotaTextos anota cada texto que le piden embeber y devuelve siempre el mismo vector.
type embebedorQueAnotaTextos struct {
	mu     sync.Mutex
	textos []string
}

func (e *embebedorQueAnotaTextos) Embed(_ context.Context, txt string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.textos = append(e.textos, txt)
	return []float32{1, 0, 0}, nil
}
func (e *embebedorQueAnotaTextos) Dimensions() int { return 3 }
func (e *embebedorQueAnotaTextos) Name() string    { return "anota-textos" }

// motorQueAnotaConsultas envuelve el motor real y anota cada consulta que le llega a Recall. Hace
// falta porque los items no alcanzan para saber qué se buscó: medido, con el recall buscando
// «fichjae» sin corregir, la nota que sólo dice «fichaje» vuelve igual.
type motorQueAnotaConsultas struct {
	memory.StorageBackend
	mu        sync.Mutex
	consultas []string
}

func (m *motorQueAnotaConsultas) Recall(ctx context.Context, q string, opts memory.RecallOptions) (memory.RecallResult, error) {
	m.mu.Lock()
	m.consultas = append(m.consultas, q)
	m.mu.Unlock()
	return m.StorageBackend.Recall(ctx, q, opts)
}

// CorregirConsulta reenvía al corrector del motor envuelto: sin esto, el borde MCP vería un motor
// que no sabe corregir y buscaría la consulta como vino.
func (m *motorQueAnotaConsultas) CorregirConsulta(ctx context.Context, q string, alcance memory.ProjectScope) (string, []memory.Correccion) {
	return m.StorageBackend.(correctorDeTipeo).CorregirConsulta(ctx, q, alcance)
}

// servidorDeTipeo arma un servidor sobre una base propia, sin proyecto propio, con las notas dadas
// como proyecto → contenidos. Un proyecto "" es una nota sin atribuir.
func servidorDeTipeo(t *testing.T, emb *embebedorQueAnotaTextos, notas map[string][]string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	for proyecto, contenidos := range notas {
		for i, c := range contenidos {
			id := proyecto + "-" + string(rune('a'+i))
			if err := engine.SaveObservationTypedFrom(proyecto, "", id, "tipeo/"+proyecto, c, 1.0, "semantic", "shared", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if emb == nil {
		return NewMcpServer(engine, t.TempDir(), fakeEmbedder{vec: []float32{1, 0, 0}})
	}
	return NewMcpServer(engine, t.TempDir(), emb)
}

// llamarTool llama una tool por el despacho, como un cliente, y devuelve el texto JSON de la respuesta.
func llamarTool(t *testing.T, s *McpServer, p *Principal, tool string, args map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
	ctx := context.Background()
	if p != nil {
		ctx = withPrincipal(ctx, p)
	}
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("%s: %+v", tool, rpcErr)
	}
	return out.(CallToolResponse).Content[0].Text
}

// correccionesDe devuelve el campo `correcciones` de una respuesta como «tipeado→buscado», y si la
// respuesta tenía la clave.
func correccionesDe(t *testing.T, texto string) (string, bool) {
	t.Helper()
	var claves map[string]json.RawMessage
	if err := json.Unmarshal([]byte(texto), &claves); err != nil {
		t.Fatalf("respuesta que no es un objeto JSON: %v\n%s", err, texto)
	}
	raw, ok := claves["correcciones"]
	if !ok {
		return "", false
	}
	var cs []memory.Correccion
	if err := json.Unmarshal(raw, &cs); err != nil {
		t.Fatalf("correcciones que no son una lista: %v\n%s", err, raw)
	}
	partes := make([]string, 0, len(cs))
	for _, c := range cs {
		partes = append(partes, c.Tipeado+"→"+c.Buscado)
	}
	return strings.Join(partes, ", "), true
}

// TestRecallCorrigeAntesDeEmbeberYLoDice: musubi_recall embebe y busca la consulta CORREGIDA, y la
// respuesta dice qué corrigió. Lo que se buscó se mira en la consulta que le llega a Recall y no en
// los items (ver motorQueAnotaConsultas).
//
// Sabotaje: el recall embebe la consulta como vino.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\t\tvec, eerr := s.embedder.Embed(embCtx, consulta)\n"
// arnes: a="\t\tvec, eerr := s.embedder.Embed(embCtx, args.Query)\n"
//
// Sabotaje: el recall busca la consulta como vino.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\t\tres, err = s.engine.Recall(s.scopedCtx(ctx), consulta, opts)\n"
// arnes: a="\t\tres, err = s.engine.Recall(s.scopedCtx(ctx), args.Query, opts)\n"
//
// Sabotaje: la respuesta no lleva las correcciones.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\tres.Correcciones = correcciones\n"
// arnes: a="\tres.Correcciones, _ = nil, correcciones\n"
func TestRecallCorrigeAntesDeEmbeberYLoDice(t *testing.T) {
	emb := &embebedorQueAnotaTextos{}
	s := servidorDeTipeo(t, emb, map[string][]string{"": {
		"El fichaje del kiosko quedó en cero.",
		"El fichaje de la raspberry se cortó.",
	}})
	motor := &motorQueAnotaConsultas{StorageBackend: s.engine}
	s.engine = motor
	texto := llamarTool(t, s, nil, "musubi_recall", map[string]any{"query": "el fichjae del kiosko"})
	if got, _ := correccionesDe(t, texto); got != "fichjae→fichaje" {
		t.Errorf("correcciones = %q, quería fichjae→fichaje\n%s", got, texto)
	}
	if len(emb.textos) != 1 || emb.textos[0] != "el fichaje del kiosko" {
		t.Errorf("se embebió %q, quería la consulta corregida", emb.textos)
	}
	if len(motor.consultas) != 1 || motor.consultas[0] != "el fichaje del kiosko" {
		t.Errorf("se buscó %q, quería la consulta corregida", motor.consultas)
	}
}

// TestRecallSinCorreccionNoCambiaLaRespuesta: sin nada que corregir, la respuesta de musubi_recall no
// lleva la clave `correcciones` —el JSON sale como antes del corrector—, y con
// memory.recall_typo_correction apagado tampoco la lleva aunque haya un tipeo, que se embebe y se
// busca como vino.
//
// Sabotaje: el campo sale siempre, aunque esté vacío.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="Correcciones []Correccion `json:\"correcciones,omitempty\"`"
// arnes: a="Correcciones []Correccion `json:\"correcciones\"`"
//
// Sabotaje: el borde MCP ignora recall_typo_correction.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\tif !s.memory.RecallTypoCorrection {\n\t\treturn q, nil\n"
// arnes: a="\tif false {\n\t\treturn q, nil\n"
func TestRecallSinCorreccionNoCambiaLaRespuesta(t *testing.T) {
	emb := &embebedorQueAnotaTextos{}
	s := servidorDeTipeo(t, emb, map[string][]string{"": {
		"El fichaje del kiosko quedó en cero.",
		"El fichaje de la raspberry se cortó.",
	}})
	texto := llamarTool(t, s, nil, "musubi_recall", map[string]any{"query": "el fichaje del kiosko"})
	if got, ok := correccionesDe(t, texto); ok {
		t.Errorf("sin nada que corregir, la respuesta lleva `correcciones` (%q): el JSON ya no es el de antes\n%s", got, texto)
	}

	s.memory.RecallTypoCorrection = false
	emb.textos = nil
	texto = llamarTool(t, s, nil, "musubi_recall", map[string]any{"query": "el fichjae del kiosko"})
	if got, ok := correccionesDe(t, texto); ok {
		t.Errorf("con el corrector apagado, la respuesta lleva `correcciones` (%q)", got)
	}
	if len(emb.textos) != 1 || emb.textos[0] != "el fichjae del kiosko" {
		t.Errorf("con el corrector apagado se embebió %q, quería la consulta como vino", emb.textos)
	}
}

// TestRecallCorrigeConElVocabularioDeLaCredencial (decisión 3 del dueño): en el central, el corrector
// usa sólo el vocabulario del proyecto de quien pregunta. «raspberry» existe sólo en el proyecto web:
// un writer de crm no recibe esa corrección, ni en musubi_recall ni en musubi_ask, y un admin
// (federado) sí — si no, la prueba no probaría nada.
//
// Sabotaje: el corrector del borde MCP corre federado para todos.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\talcance := memory.ProjectScope{ProjectID: opts.ProjectScope, Federate: opts.Federate}\n\ts.withReadLock"
// arnes: a="\talcance := memory.ProjectScope{}\n\ts.withReadLock"
func TestRecallCorrigeConElVocabularioDeLaCredencial(t *testing.T) {
	s := servidorDeTipeo(t, nil, map[string][]string{
		"crm": {"El fichaje del crm quedó en cero.", "Otro fichaje del crm."},
		"web": {"La raspberry de la web.", "Reiniciar la raspberry de la web."},
	})
	q := "el fichjae de la rasberry"
	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	if got, _ := correccionesDe(t, llamarTool(t, s, crm, "musubi_recall", map[string]any{"query": q})); got != "fichjae→fichaje" {
		t.Errorf("writer de crm: correcciones = %q, quería sólo fichjae→fichaje (raspberry es de otro proyecto)", got)
	}
	admin := &Principal{Name: "root", Role: RoleAdmin}
	if got, _ := correccionesDe(t, llamarTool(t, s, admin, "musubi_recall", map[string]any{"query": q})); got != "fichjae→fichaje, rasberry→raspberry" {
		t.Errorf("admin: correcciones = %q, quería las dos (la prueba no probaría nada)", got)
	}

	s.cognition = &fakeCognition{}
	if got, _ := correccionesDe(t, llamarTool(t, s, crm, "musubi_ask", map[string]any{"question": q})); got != "fichjae→fichaje" {
		t.Errorf("musubi_ask de un writer de crm: correcciones = %q, quería sólo fichjae→fichaje", got)
	}
}

// TestAskBuscaCorregidoYLePreguntaAlMotorComoVino: musubi_ask busca el grounding con la pregunta
// corregida y lo dice en `correcciones`, pero al motor le llega la PREGUNTA como la escribió la
// persona: un modelo de lenguaje lee un tipeo sin ayuda, y si la corrección fue mala no tiene que
// responder otra pregunta que la que le hicieron. «sevridor» no está en ninguna nota: sin corregir,
// no habría grounding y el motor ni se llamaría.
//
// Sabotaje: el grounding se busca con la pregunta como vino.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="if res, err = s.engine.Recall(ctx, consulta, opts); err != nil"
// arnes: a="if res, err = s.engine.Recall(ctx, args.Question, opts); err != nil"
//
// Sabotaje: al motor le llega la pregunta corregida.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="\tuser := \"PREGUNTA:\\n\" + args.Question + "
// arnes: a="\tuser := \"PREGUNTA:\\n\" + consulta + "
//
// Sabotaje: la respuesta de musubi_ask no lleva las correcciones.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="\tif len(cs) > 0 {\n\t\tr[\"correcciones\"] = cs\n"
// arnes: a="\tif len(cs) < 0 {\n\t\tr[\"correcciones\"] = cs\n"
func TestAskBuscaCorregidoYLePreguntaAlMotorComoVino(t *testing.T) {
	s := servidorDeTipeo(t, nil, map[string][]string{"": {
		"Musubi es un servidor MCP de memoria persistente escrito en Go.",
		"El servidor central corre en Rocky Linux.",
	}})
	fake := &fakeCognition{}
	s.cognition = fake
	texto := llamarTool(t, s, nil, "musubi_ask", map[string]any{"question": "que hace el sevridor"})
	if !fake.called {
		t.Fatalf("el motor no se llamó: el grounding se buscó sin corregir y no encontró nada\n%s", texto)
	}
	if !strings.Contains(fake.gotUser, "PREGUNTA:\nque hace el sevridor\n") {
		t.Errorf("al motor no le llegó la pregunta como vino:\n%s", fake.gotUser)
	}
	if !strings.Contains(fake.gotUser, "memoria persistente") {
		t.Errorf("el grounding no trae la nota del servidor:\n%s", fake.gotUser)
	}
	if got, _ := correccionesDe(t, texto); got != "sevridor→servidor" {
		t.Errorf("correcciones = %q, quería sevridor→servidor\n%s", got, texto)
	}
}

// TestElJuezVeLaConsultaComoLaTipearon: el juez de pertinencia de musubi_recall recibe la consulta
// como la escribió la persona, no la corregida. Juzga pertinencia contra lo que se pidió, y si la
// corrección fue mala es el único que todavía puede bajar lo que respondió a la palabra equivocada.
//
// Sabotaje: el juez recibe la consulta corregida.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\tres = s.rerankSiCorresponde(ctx, args.Query, res, args.Rerank)\n"
// arnes: a="\tres = s.rerankSiCorresponde(ctx, consulta, res, args.Rerank)\n"
func TestElJuezVeLaConsultaComoLaTipearon(t *testing.T) {
	s := servidorDeTipeo(t, nil, map[string][]string{"": {
		"El fichaje del kiosko quedó en cero.",
		"El fichaje de la raspberry se cortó.",
	}})
	fake := &fakeCognition{answer: `["-b","-a"]`}
	s.cognition = fake
	// Una consulta que ninguna otra prueba usa: la caché del juez es del paquete.
	texto := llamarTool(t, s, nil, "musubi_recall", map[string]any{"query": "juez: el fichjae de la raspberry", "rerank": true})
	if !fake.called {
		t.Fatalf("el juez no se llamó:\n%s", texto)
	}
	if !strings.Contains(fake.gotUser, "fichjae") {
		t.Errorf("el juez no vio la consulta como la tipearon:\n%s", fake.gotUser)
	}
	if got, _ := correccionesDe(t, texto); got != "fichjae→fichaje" {
		t.Errorf("correcciones = %q, quería fichjae→fichaje", got)
	}
}
