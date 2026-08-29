package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// Invariantes del GROUNDING FIEL (specs/grounding-fiel). Lo que se prueba acá es el PROMPT que
// recibe el motor —fake.gotUser—, no la respuesta: el defecto que originó el cambio era invisible
// desde la respuesta y sólo apareció volcando el cuerpo que viajaba al motor.

// newAskServer arma un server con motor falso sobre un proyecto real en disco (hace falta para las
// anclas de ranciedad, que se re-derivan del archivo).
func newAskServer(t *testing.T, files map[string]string) (*McpServer, string, *fakeCognition) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	memtest.Sembrar(t, root)
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, root, embedding.NoopProvider{})
	fake := &fakeCognition{}
	s.cognition = fake
	return s, root, fake
}

func guardar(t *testing.T, s *McpServer, args string) {
	t.Helper()
	if _, rerr := s.toolSaveObservation(context.Background(), json.RawMessage(args)); rerr != nil {
		t.Fatalf("save_observation: %v", rerr)
	}
}

func preguntar(t *testing.T, s *McpServer, args string) {
	t.Helper()
	if _, rerr := s.toolAsk(context.Background(), json.RawMessage(args)); rerr != nil {
		t.Fatalf("ask: %v", rerr)
	}
}

// textoBase pesa MUCHO más que un gist a propósito: lo que venga después de él en el contenido
// sólo llega al prompt si se hidrató de verdad.
const textoBase = "El portero de privacidad se para entre la memoria y el motor externo y tapa los secretos " +
	"antes de que crucen. Trabaja con marcadores reversibles, así que el valor real se repone en la respuesta " +
	"que vuelve al caller y el agente no pierde el dato. No tiene interruptor de apagado por config porque una " +
	"guarda de seguridad disponible para apagar termina apagada."

const contenidoLargo = textoBase + " MARCA-DEL-FINAL-DEL-CONTENIDO"

// G1 — el grounding manda el CONTENIDO COMPLETO, no el gist truncado.
func TestG1GroundingMandaContenidoCompleto(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	guardar(t, s, `{"topic_key":"cognicion/portero","content":`+jsonStr(contenidoLargo)+`}`)

	preguntar(t, s, `{"question":"que hace el portero de privacidad con los secretos"}`)

	if !strings.Contains(fake.gotUser, "MARCA-DEL-FINAL-DEL-CONTENIDO") {
		t.Errorf("FUGA G1: al motor le llegó el gist truncado, no el contenido completo.\nprompt=%q", fake.gotUser)
	}
}

// G2 — la hidratación cambia la PROFUNDIDAD, no la SELECCIÓN: las mismas memorias, en el mismo orden.
//
// El caso que importa es el PARCIAL: cuando el presupuesto alcanza para hidratar sólo algunas, las
// otras tienen que seguir en el prompt con su gist. La primera versión de este test usaba memorias
// chiquitas que entraban TODAS, así que la rama de "no entró" nunca se ejecutaba y el test no vio
// un sabotaje que borraba del prompt justo a las no hidratadas. Por eso ahora se verifica primero
// que la hidratación haya quedado incompleta.
func TestG2HidratarNoCambiaLaSeleccion(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	marcas := []string{"UNO", "DOS", "TRES"}
	for _, m := range marcas {
		// El INICIO entra en el gist; el FINAL sólo aparece si esa memoria se hidrató.
		guardar(t, s, `{"topic_key":"t/`+m+`","content":`+jsonStr(
			"MARCA-"+m+"-INICIO "+textoBase+" "+textoBase+" MARCA-"+m+"-FINAL")+`}`)
	}

	// No se corre un Recall aparte para comparar: ese Recall BUMPEA los accesos y le cambia el
	// ranking al Recall que hace ask, y el test terminaba fallando por su propia sonda. Todo lo
	// que se verifica sale del único prompt.
	preguntar(t, s, `{"question":"portero de privacidad secretos motor","token_budget":150}`)

	hidratadas, soloGist := []int{}, []int{}
	for _, m := range marcas {
		ini := strings.Index(fake.gotUser, "MARCA-"+m+"-INICIO")
		if ini < 0 {
			t.Fatalf("FUGA G2: la memoria %s se cayó del grounding (probablemente por no entrar en la hidratación).\nprompt=%q", m, fake.gotUser)
		}
		if strings.Contains(fake.gotUser, "MARCA-"+m+"-FINAL") {
			hidratadas = append(hidratadas, ini)
		} else {
			soloGist = append(soloGist, ini)
		}
	}

	// Control: si entraran todas o ninguna, este test no estaría probando el caso PARCIAL, que es
	// el único donde la distinción entre profundidad y selección se puede romper.
	if len(hidratadas) == 0 || len(soloGist) == 0 {
		t.Fatalf("el control no sirve: %d hidratadas y %d en gist — hace falta hidratación INCOMPLETA",
			len(hidratadas), len(soloGist))
	}
	// La hidratación sirve al TOPE del ranking: las hidratadas van antes que las que quedaron en gist.
	for _, h := range hidratadas {
		for _, g := range soloGist {
			if h > g {
				t.Errorf("FUGA G2: una memoria hidratada quedó DESPUÉS de una sin hidratar; la hidratación no siguió el orden del recall")
			}
		}
	}
}

// G3 — la advertencia de ranciedad SOBREVIVE al reemplazo del gist por el contenido.
// Es la regresión más silenciosa del cambio: el prompt se ve perfecto sin ella.
func TestG3LaAdvertenciaDeRancioSobrevive(t *testing.T) {
	s, root, fake := newAskServer(t, map[string]string{"src/portero.go": "package portero // v1\n"})
	guardar(t, s, `{"topic_key":"cognicion/portero","content":`+jsonStr(contenidoLargo)+`,"origin_paths":["src/portero.go"]}`)

	// Alguien toca el archivo del que habla la nota: la nota queda posiblemente vencida.
	if err := os.WriteFile(filepath.Join(root, "src", "portero.go"), []byte("package portero // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	preguntar(t, s, `{"question":"que hace el portero de privacidad con los secretos"}`)

	if !strings.Contains(fake.gotUser, "MARCA-DEL-FINAL-DEL-CONTENIDO") {
		t.Fatalf("el control no sirve: no se hidrató, así que no se está probando que la advertencia sobreviva a la hidratación")
	}
	if !strings.Contains(fake.gotUser, "src/portero.go") || !strings.Contains(fake.gotUser, "rancia") {
		t.Errorf("FUGA G3: la advertencia de ranciedad se perdió al hidratar; el modelo cree que la nota está vigente.\nprompt=%q", fake.gotUser)
	}
}

// G4 — el sello de procedencia viaja en el prompt: el motor no puede confundir una inferencia de
// otro LLM con una nota verificada. Y 'human' NO se marca (si todo dijera human, sería ruido).
func TestG4ElSelloDeProcedenciaViajaAlMotor(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	ctx := context.Background()

	// OJO con el topic: NO puede contener la subcadena "human" o la aserción de abajo se
	// autoenvenena (pasó con "t/humana": el test fallaba por su propio dato, no por el código).
	guardar(t, s, `{"topic_key":"t/verificada","content":"el circuit breaker del router saca de rotación al motor que falla"}`)

	out, rerr := s.toolProposeObservation(ctx, json.RawMessage(
		`{"topic_key":"t/inferida","content":"el circuit breaker del router se abre tras tres fallas consecutivas","model":"modelo-de-prueba","confidence":0.7}`))
	if rerr != nil {
		t.Fatalf("propose_observation: %v", rerr)
	}
	id := idDeLaPropuesta(t, out)
	if _, rerr := s.toolCorroborate(ctx, json.RawMessage(`{"id":"`+id+`"}`)); rerr != nil {
		t.Fatalf("corroborate: %v", rerr)
	}

	preguntar(t, s, `{"question":"circuit breaker del router"}`)

	if !strings.Contains(fake.gotUser, "llm:modelo-de-prueba") {
		t.Errorf("FUGA G4: la inferencia de un LLM llegó al motor SIN sello, indistinguible de una nota humana.\nprompt=%q", fake.gotUser)
	}
	// El sello va en la cabecera precedido de " · ", así que se busca esa forma exacta y no la
	// palabra suelta, que puede aparecer legítimamente en el contenido de cualquier memoria.
	if strings.Contains(fake.gotUser, "· human") {
		t.Errorf("una memoria humana no debe llevar sello (sería ruido en todas); prompt=%q", fake.gotUser)
	}
}

// G5 — la cuarentena NO entra por la puerta de la hidratación. La hidratación por id no filtra:
// la muralla se sostiene porque los ids salen del recall. Con control, para no pasar en falso.
func TestG5LaCuarentenaNoEntraPorLaHidratacion(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	ctx := context.Background()

	guardar(t, s, `{"topic_key":"t/control","content":"CONTROL-VISIBLE el caché de cognición responde sin llamar cuando ya se preguntó lo mismo"}`)
	if _, rerr := s.toolProposeObservation(ctx, json.RawMessage(
		`{"topic_key":"t/cuarentena","content":"EN-CUARENTENA el caché de cognición responde sin llamar cuando ya se preguntó lo mismo","model":"modelo-de-prueba"}`)); rerr != nil {
		t.Fatalf("propose_observation: %v", rerr)
	}

	preguntar(t, s, `{"question":"cache de cognicion responde sin llamar"}`)

	if !strings.Contains(fake.gotUser, "CONTROL-VISIBLE") {
		t.Fatalf("el control no sirve: la memoria visible tampoco llegó al prompt")
	}
	if strings.Contains(fake.gotUser, "EN-CUARENTENA") {
		t.Errorf("FUGA G5: una observación EN CUARENTENA llegó al motor por la hidratación.\nprompt=%q", fake.gotUser)
	}
}

// hidratacionRota simula un fallo del backend SÓLO en la hidratación del grounding.
type hidratacionRota struct{ memory.StorageBackend }

func (hidratacionRota) HydrateForGroundingCtx(context.Context, []string, int) ([]memory.Observation, int, error) {
	return nil, 0, errors.New("falla simulada de hidratación")
}

// G6 — un fallo hidratando NO tumba la pregunta: se responde con los gists.
func TestG6FalloHidratandoDegradaAGists(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	guardar(t, s, `{"topic_key":"cognicion/portero","content":`+jsonStr(contenidoLargo)+`}`)
	s.engine = hidratacionRota{s.engine}

	preguntar(t, s, `{"question":"que hace el portero de privacidad con los secretos"}`)

	if !fake.called {
		t.Fatal("FUGA G6: un fallo hidratando tumbó la pregunta entera")
	}
	if !strings.Contains(fake.gotUser, "El portero de privacidad se para entre la memoria") {
		t.Errorf("al degradar debe quedar el GIST, no un prompt vacío; prompt=%q", fake.gotUser)
	}
	if strings.Contains(fake.gotUser, "MARCA-DEL-FINAL-DEL-CONTENIDO") {
		t.Errorf("el control no sirve: la hidratación rota igual hidrató")
	}
}

// G7 — el prompt tiene techo: con un presupuesto chico el contenido se recorta en vez de crecer.
func TestG7ElPromptTieneTecho(t *testing.T) {
	s, _, fake := newAskServer(t, nil)
	guardar(t, s, `{"topic_key":"cognicion/portero","content":`+jsonStr(contenidoLargo)+`}`)

	preguntar(t, s, `{"question":"que hace el portero de privacidad con los secretos","token_budget":12}`)

	if strings.Contains(fake.gotUser, "MARCA-DEL-FINAL-DEL-CONTENIDO") {
		t.Errorf("FUGA G7: con token_budget=12 el prompt igual se llevó el contenido entero.\nprompt=%q", fake.gotUser)
	}
	if !strings.Contains(fake.gotUser, "El portero de privacidad") {
		t.Errorf("recortar no es vaciar: el arranque del contenido debe llegar; prompt=%q", fake.gotUser)
	}
}

// jsonStr serializa un string como literal JSON (comillas y escapes) sin armarlo a mano.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// idDeLaPropuesta saca el uuid del texto que devuelve musubi_propose_observation.
func idDeLaPropuesta(t *testing.T, out interface{}) string {
	t.Helper()
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	txt := string(raw)
	i := strings.Index(txt, "id: ")
	if i < 0 {
		t.Fatalf("no encontré el id en la respuesta: %s", txt)
	}
	rest := txt[i+len("id: "):]
	j := strings.IndexAny(rest, ",)")
	if j < 0 {
		t.Fatalf("no pude delimitar el id en: %s", rest)
	}
	return strings.TrimSpace(rest[:j])
}
