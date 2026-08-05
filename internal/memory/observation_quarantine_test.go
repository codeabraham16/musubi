package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Invariantes de la CUARENTENA DE ESCRITURA Y PROCEDENCIA del LIBRO MAYOR
// (Murallas 2+3 · F4). Ver specs/cuarentena-escritura-procedencia/spec.md.
//
// OJO: quarantine_test.go es otra cosa — la cuarentena de HECHOS del grafo (relations.source),
// que ya existía desde el pilar Cognición F1. Son dos murallas distintas sobre dos superficies
// distintas; esta fase construyó la del libro mayor porque era la que faltaba.
//
// Cada test se verificó FALLANDO al sabotear la implementación. Un test que nunca se vio en rojo
// no prueba nada.

const obsQuarantineText = "zarandaja incandescente del perimetro"

// proponerObs mete una observación en cuarentena.
func proponerObs(t *testing.T, e *DbEngine, topic, content string) string {
	t.Helper()
	id, err := e.ProposeObservation("", "", topic, content, "modelo-de-prueba", 0.7, "", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}
	return id
}

func contieneID(items []RecallItem, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

// --- Q0: invisible en TODO camino de listado --------------------------------------------------

// Q0 por el recall LÉXICO. El control es imprescindible: sin él, un recall que no devuelve nada
// por cualquier otro motivo haría pasar el test en falso.
func TestQ0ObsCuarentenaInvisibleAlRecallLexico(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("visible", "t/control", obsQuarantineText+" visible", nil); err != nil {
		t.Fatalf("save control: %v", err)
	}
	oculta := proponerObs(t, e, "t/cuarentena", obsQuarantineText+" oculta")

	res, err := e.Recall(context.Background(), obsQuarantineText, RecallOptions{TokenBudget: 4000})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if !contieneID(res.Items, "visible") {
		t.Fatalf("el control no apareció: el test no está probando nada (items=%d)", len(res.Items))
	}
	if contieneID(res.Items, oculta) {
		t.Errorf("FUGA Q0: %s (en cuarentena) apareció en el recall léxico", oculta)
	}
}

// Q0 por el PRIMING, que lista sin query: es otro camino, no una variante del anterior.
func TestQ0ObsCuarentenaInvisibleAlPriming(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("visible", "t/control", "observacion normal de control", nil); err != nil {
		t.Fatalf("save control: %v", err)
	}
	oculta := proponerObs(t, e, "t/cuarentena", obsQuarantineText)

	res, err := e.PrimeContext(4000)
	if err != nil {
		t.Fatalf("PrimeContext: %v", err)
	}
	if !contieneID(res.Items, "visible") {
		t.Fatalf("el control no apareció en el priming: el test no está probando nada")
	}
	if contieneID(res.Items, oculta) {
		t.Errorf("FUGA Q0: %s (en cuarentena) apareció en el priming", oculta)
	}
}

// Q0 por el GRAFO NEURONAL del dashboard. Este es EL camino que no usaba el predicado canónico y
// se descubrió al revisar el diseño: es la razón de que Q0 hable de "listado" y no de "recall".
func TestQ0ObsCuarentenaInvisibleAlGrafoNeuronal(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("visible", "t/control", "observacion normal de control", nil); err != nil {
		t.Fatalf("save control: %v", err)
	}
	oculta := proponerObs(t, e, "t/cuarentena", obsQuarantineText)

	g, err := e.BrainGraph(100)
	if err != nil {
		t.Fatalf("BrainGraph: %v", err)
	}
	var vioControl, vioOculta bool
	for _, n := range g.Neurons {
		if n.ID == "visible" {
			vioControl = true
		}
		if n.ID == oculta {
			vioOculta = true
		}
	}
	if !vioControl {
		t.Fatalf("el control no apareció como neurona: el test no está probando nada (neuronas=%d)", len(g.Neurons))
	}
	if vioOculta {
		t.Errorf("FUGA Q0: %s (en cuarentena) se dibujó como neurona en el dashboard", oculta)
	}
}

// Q0b — la hidratación POR ID explícito queda afuera A PROPÓSITO. El test existe para que, si
// alguien lo "arregla" algún día, sea una decisión y no un accidente.
func TestQ0bHidratacionPorIdSigueLeyendoLaCuarentena(t *testing.T) {
	e := newTestEngine(t)
	id := proponerObs(t, e, "t/cuarentena", obsQuarantineText)

	obs, _, err := e.GetObservationsBudget([]string{id}, 4000)
	if err != nil {
		t.Fatalf("GetObservationsBudget: %v", err)
	}
	if len(obs) != 1 {
		t.Errorf("Q0b: la hidratación por id explícito debe seguir devolviendo la observación (Q0 impide DESCUBRIR, no LEER); obtuve %d", len(obs))
	}
}

// --- Q1 y Q5: sello universal y bit-identidad del camino de siempre ---------------------------

func TestQ1YQ5ElCaminoDeSiempreQuedaSelladoComoHumano(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("normal", "t/x", "una nota cualquiera", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	prov, conf, quar, err := e.ObservationStamp("normal")
	if err != nil {
		t.Fatalf("ObservationStamp: %v", err)
	}
	if prov != provenanceHuman {
		t.Errorf("Q1: procedencia = %q, esperaba %q", prov, provenanceHuman)
	}
	if conf != 1.0 {
		t.Errorf("Q5: confianza = %v, esperaba 1.0", conf)
	}
	if quar {
		t.Errorf("Q5: el camino de siempre NO debe dejar nada en cuarentena")
	}
}

// --- Q2: el sello es POR DÓNDE ENTRASTE, y no se puede pisar ----------------------------------

func TestQ2ProposeSellaLLMYCuarentena(t *testing.T) {
	e := newTestEngine(t)
	id := proponerObs(t, e, "t/x", obsQuarantineText)

	prov, conf, quar, err := e.ObservationStamp(id)
	if err != nil {
		t.Fatalf("ObservationStamp: %v", err)
	}
	if !strings.HasPrefix(prov, provenanceLLMPrefix) {
		t.Errorf("Q2: procedencia = %q, esperaba prefijo %q", prov, provenanceLLMPrefix)
	}
	if !strings.Contains(prov, "modelo-de-prueba") {
		t.Errorf("Q2: el sello debe llevar el modelo pegado para ser auditable, obtuve %q", prov)
	}
	if conf != 0.7 {
		t.Errorf("Q2: confianza = %v, esperaba 0.7", conf)
	}
	if !quar {
		t.Errorf("Q2: propose DEBE dejar la observación en cuarentena")
	}
}

// Q2 — el borde que de verdad importa: un save_observation posterior sobre el MISMO id no puede
// lavar el sello ni sacar la fila de cuarentena. Sin esto, la muralla se saltea con dos llamadas.
func TestQ2UnSavePosteriorNoLavaElSello(t *testing.T) {
	e := newTestEngine(t)
	id := proponerObs(t, e, "t/x", obsQuarantineText)

	if err := e.SaveObservation(id, "t/x", "contenido reescrito por el camino normal", nil); err != nil {
		t.Fatalf("save sobre el id en cuarentena: %v", err)
	}
	prov, _, quar, err := e.ObservationStamp(id)
	if err != nil {
		t.Fatalf("ObservationStamp: %v", err)
	}
	if !quar {
		t.Errorf("FUGA Q2: un save_observation sobre el id lo sacó de cuarentena")
	}
	if !strings.HasPrefix(prov, provenanceLLMPrefix) {
		t.Errorf("FUGA Q2: un save_observation sobre el id lavó la procedencia a %q", prov)
	}
}

// Q2 — propose NO deduplica. Si dedupara, un texto igual a una observación autoritativa devolvería
// el id de ésta y la propuesta quedaría "confirmada" sin que nadie la corroborara.
func TestQ2ProposeNoDeduplicaContraLoAutoritativo(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("autoritativa", "t/x", obsQuarantineText, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	id := proponerObs(t, e, "t/x", obsQuarantineText) // MISMO contenido
	if id == "autoritativa" {
		t.Fatalf("FUGA Q2: propose dedupó contra una observación autoritativa y devolvió su id")
	}
	if quar, err := e.IsQuarantined(id); err != nil || !quar {
		t.Errorf("Q2: la propuesta debe quedar en cuarentena (quar=%v, err=%v)", quar, err)
	}
}

// --- Q3: el recall marca lo que no es human ---------------------------------------------------

func TestQ3ElRecallMarcaLaProcedenciaNoHumana(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("humana", "t/control", obsQuarantineText+" humana", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	id := proponerObs(t, e, "t/x", obsQuarantineText+" propuesta")
	if err := e.CorroborateObservation(id); err != nil {
		t.Fatalf("Corroborate: %v", err)
	}

	res, err := e.Recall(context.Background(), obsQuarantineText, RecallOptions{TokenBudget: 4000})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	var vioCorroborada bool
	for _, it := range res.Items {
		if it.ID == id {
			vioCorroborada = true
			if !strings.HasPrefix(it.Provenance, provenanceLLMPrefix) {
				t.Errorf("Q3: la corroborada llegó con Provenance=%q, esperaba el sello de LLM", it.Provenance)
			}
		}
		if it.ID == "humana" && it.Provenance != "" {
			t.Errorf("Q3: una observación humana no debe llevar marca (obtuve %q); si todas la llevan, deja de leerse", it.Provenance)
		}
	}
	if !vioCorroborada {
		t.Fatalf("la corroborada no apareció en el recall: el test no está probando nada")
	}
}

// --- Q4: salir de cuarentena es explícito y CONSERVA el sello ---------------------------------

func TestQ4CorroborarHaceVisibleYConservaElSello(t *testing.T) {
	e := newTestEngine(t)
	id := proponerObs(t, e, "t/x", obsQuarantineText)

	if err := e.CorroborateObservation(id); err != nil {
		t.Fatalf("Corroborate: %v", err)
	}
	prov, conf, quar, err := e.ObservationStamp(id)
	if err != nil {
		t.Fatalf("ObservationStamp: %v", err)
	}
	if quar {
		t.Errorf("Q4: corroborar debe sacar de cuarentena")
	}
	if !strings.HasPrefix(prov, provenanceLLMPrefix) {
		t.Errorf("Q4: corroborar NO debe lavar la procedencia; obtuve %q", prov)
	}
	if conf != 0.7 {
		t.Errorf("Q4: corroborar no debe tocar la confianza; obtuve %v", conf)
	}

	res, err := e.Recall(context.Background(), obsQuarantineText, RecallOptions{TokenBudget: 4000})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if !contieneID(res.Items, id) {
		t.Errorf("Q4: tras corroborar, la observación debe aparecer en el recall")
	}
}

// Q4 — corroborar lo que no está en cuarentena es ERROR, no un no-op silencioso: un corroborate
// por el id equivocado tiene que notarse, en vez de reportar éxito dejando la real invisible.
func TestQ4CorroborarLoQueNoEstaEnCuarentenaEsError(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("normal", "t/x", "nota normal", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := e.CorroborateObservation("normal"); !errors.Is(err, ErrNotQuarantined) {
		t.Errorf("Q4: esperaba ErrNotQuarantined, obtuve %v", err)
	}
	if err := e.CorroborateObservation("no-existe"); !errors.Is(err, ErrObservationNotFound) {
		t.Errorf("Q4: esperaba ErrObservationNotFound, obtuve %v", err)
	}
}

// --- Q6: la cuarentena no se filtra al cerebro central ----------------------------------------

func TestQ6NoSePuedePromoverEnCuarentena(t *testing.T) {
	e := newTestEngine(t)
	id := proponerObs(t, e, "t/x", obsQuarantineText)

	if err := e.PromoteObservationCtx(context.Background(), id); !errors.Is(err, ErrQuarantined) {
		t.Errorf("Q6: promover algo en cuarentena debe fallar con ErrQuarantined, obtuve %v", err)
	}

	// Y tras corroborar SÍ se puede: la guarda bloquea la cuarentena, no la promoción.
	if err := e.CorroborateObservation(id); err != nil {
		t.Fatalf("Corroborate: %v", err)
	}
	if err := e.PromoteObservationCtx(context.Background(), id); err != nil {
		t.Errorf("Q6: tras corroborar, promover debe funcionar; obtuve %v", err)
	}
}

// --- Q7: la confianza es acotada y fuera de rango se RECHAZA ----------------------------------

func TestQ7ConfianzaFueraDeRangoSeRechaza(t *testing.T) {
	e := newTestEngine(t)
	for _, c := range []float64{-0.01, 1.01, 42} {
		if _, err := e.ProposeObservation("", "", "t/x", "contenido", "m", c, "", nil); !errors.Is(err, ErrInvalidConfidence) {
			t.Errorf("Q7: confidence=%v debía rechazarse con ErrInvalidConfidence, obtuve %v", c, err)
		}
	}
	// Los bordes SÍ son válidos: 0 significa "no le creo nada", que es una afirmación legítima y
	// distinta de "me olvidé del campo".
	for _, c := range []float64{0, 1} {
		if _, err := e.ProposeObservation("", "", "t/x", "contenido borde", "m", c, "", nil); err != nil {
			t.Errorf("Q7: confidence=%v es un borde VÁLIDO, no debía fallar: %v", c, err)
		}
	}
}

// --- Taxonomía: cerrada, sin defaults silenciosos ---------------------------------------------

func TestTaxonomiaDeProcedenciaEsCerrada(t *testing.T) {
	for _, p := range []string{provenanceHuman, provenanceDeterministic, "llm:groq/llama-3.3", "llm:x"} {
		if !validProvenance(p) {
			t.Errorf("%q debería ser una procedencia válida", p)
		}
	}
	// 'llm:' pelado NO alcanza: sin el modelo, el sello no es auditable.
	for _, p := range []string{"", "humano", "LLM:x", "llm:", "llm:   ", "maquina", "agent"} {
		if validProvenance(p) {
			t.Errorf("%q NO debería ser una procedencia válida", p)
		}
	}
}

// stampProvenance sólo debe hablar cuando hay algo que decir.
func TestStampProvenanceCallaLoQueEsDefault(t *testing.T) {
	if got := stampProvenance(provenanceHuman); got != "" {
		t.Errorf("'human' no debe viajar al caller, obtuve %q", got)
	}
	if got := stampProvenance(""); got != "" {
		t.Errorf("el vacío de una fila legacy no debe viajar, obtuve %q", got)
	}
	if got := stampProvenance("llm:x"); got != "llm:x" {
		t.Errorf("un sello de LLM SÍ debe viajar, obtuve %q", got)
	}
	if got := stampProvenance(provenanceDeterministic); got != provenanceDeterministic {
		t.Errorf("'deterministic' debe viajar, obtuve %q", got)
	}
}
