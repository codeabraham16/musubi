package mcp

// EL SELLO DEL PUSH CRUZA EL CABLE Y EL RECEPTOR LO HONRA BAJO LA REGLA NO-HUMAN
// (SDD sello-llm-viaja-al-central).
//
// LO QUE SE PRUEBA, Y POR QUÉ. Que el OutboxItem lleve el sello está cubierto en internal/memory
// (sello_viaja_test.go). Acá se ejercen los dos eslabones que viven en el paquete mcp:
//
//  1. El PUSH copia el sello del item al payload que CRUZA EL CABLE y el central lo honra de punta a
//     punta (TestElSelloLlmCruzaElCableEnElPush). Es el sitio exacto del fix (SyncClient.Push): sin
//     este test, borrar la copia dejaba la suite verde, porque los tests del receptor inyectan el
//     sello a mano y el reintento de compresión lo vuelve a pasar explícito.
//  2. El RECEPTOR (toolSaveObservation, camino con-id = la forma de un push) aplica la regla no-human
//     sobre el sello ENTRANTE: un 'llm:*' válido se honra con su confidence (T6a); un 'human' colado
//     se ignora y cae a human/1.0 (T6b); un sello fuera de la taxonomía se RECHAZA en vez de lavarse
//     a 'human' (T6c); una confidence fuera de rango se rechaza en vez de recortarse (T6d); y un 0.0
//     legítimo de un 'llm' sobrevive sin re-inflarse a 1.0 (T6f).
//
// El sello se lee del raw, NO del input schema del registry (R4): por eso el golden de tools/list NO
// cambia (se verifica aparte, en TestToolsListGolden). R5 —que un re-save no pise el sello de una fila
// existente— está cubierto en el paquete memory: la mitad de provenance/cuarentena por
// TestQ2UnSavePosteriorNoLavaElSello y la de confidence por TestR5LaConfidenceSelladaSobreviveAUnSavePlano.

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// selloYConfianza lee provenance/confidence de una observación por SQL crudo, por fuera del engine
// (misma técnica que filaObs, que no devuelve confidence): afirma el ESTADO PERSISTIDO.
func selloYConfianza(t *testing.T, dir, id string) (provenance string, confidence float64) {
	t.Helper()
	ruta := filepath.Join(dir, config.DirName, config.DBFile)
	db, err := sql.Open("sqlite", ruta+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	defer db.Close()
	row := db.QueryRow(`SELECT COALESCE(provenance,''), COALESCE(confidence,1.0) FROM observations WHERE id = ?`, id)
	if err := row.Scan(&provenance, &confidence); err != nil {
		t.Fatalf("leer el sello de %q: %v", id, err)
	}
	return
}

// guardarConSello llama a musubi_save_observation con un id explícito (la forma de un push del outbox)
// y el sello crudo (provenance/confidence), con el principal dado. Devuelve el RpcError (nil si guardó).
// Pasa el sello en el raw, que es de donde el handler lo lee —NO está en el input schema—, igual que
// lo hace SyncClient.Push.
func guardarConSello(t *testing.T, s *McpServer, p *Principal, id, provenance string, confidence float64) *RpcError {
	t.Helper()
	argBytes, _ := json.Marshal(map[string]interface{}{
		"id":         id,
		"topic_key":  "t/sello",
		"content":    "contenido sellado " + id,
		"provenance": provenance,
		"confidence": confidence,
	})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_save_observation", Arguments: argBytes})
	_, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
	return rpcErr
}

// T6a (R3/D2) — UN 'llm:*' VÁLIDO SE HONRA. El nodo origen ya lo selló por su Q2; perderlo acá sería
// el lavado que esta fase existe para impedir. Sin compuerta por principal (D2): este push llega con
// write=own (duenoOwn) y se honra igual, porque el sello baja la autoridad del contenido, nunca la sube.
//
// Sabotaje: la rama que honra pasa 'human' en vez del sello recibido ⇒ la fila nace lavada.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="emb, args.Provenance, args.Confidence)"
// arnes: a="emb, \x22human\x22, args.Confidence)"
func TestSaveConSelloLlmLoConserva(t *testing.T) {
	s, _, dir := servidorCorroborar(t, false)
	if rpcErr := guardarConSello(t, s, duenoOwn(), "push-llm", "llm:groq/llama-3.3", 0.7); rpcErr != nil {
		t.Fatalf("un sello válido debía guardarse, obtuve %+v", rpcErr)
	}
	prov, _ := selloYConfianza(t, dir, "push-llm")
	if prov != "llm:groq/llama-3.3" {
		t.Errorf("provenance=%q: un 'llm:*' válido NO debe lavarse (esperaba \"llm:groq/llama-3.3\")", prov)
	}
}

// T6b (D3) — UN 'human' COLADO CAE AL DEFAULT. Un push con provenance='human' y una confidence a
// medida se IGNORA: cae a human/1.0. Así 'human' es siempre POR-DEFAULT y nunca POR-CLAIM — la puerta
// pública no puede declararse 'human' con la confianza que quiera (protege el espíritu del Q2). Esto
// también es la puerta pública (Q2): un agente con write=own que cuele 'human' no gana autoridad.
//
// Sabotaje: que HonrarSello deje de excluir 'human' ⇒ se honra, la confidence 0.3 se pega y el test cae.
// arnes: archivo="internal/memory/observation_quarantine.go"
// arnes: de="return validProvenance(provenance) && provenance != provenanceHuman"
// arnes: a="return validProvenance(provenance)"
func TestSaveConSelloHumanoCaeAlDefault(t *testing.T) {
	s, _, dir := servidorCorroborar(t, false)
	if rpcErr := guardarConSello(t, s, duenoOwn(), "push-humano", "human", 0.3); rpcErr != nil {
		t.Fatalf("un 'human' debía guardarse por el camino de siempre, obtuve %+v", rpcErr)
	}
	prov, conf := selloYConfianza(t, dir, "push-humano")
	if prov != "human" {
		t.Errorf("provenance=%q: esperaba \"human\"", prov)
	}
	if conf != 1.0 {
		t.Errorf("confidence=%v: un 'human' es siempre 1.0, nunca a medida (se mandó 0.3)", conf)
	}
}

// T6c (STRICT) — UN SELLO FUERA DE LA TAXONOMÍA SE RECHAZA. Un sello PRESENTE que no es ni 'human' ni
// una procedencia válida se rechaza con ErrInvalidProvenance (⇒ codeInvalidParams, que el cliente de
// sync trata como dead-letter permanente), NO se lava a 'human'. Lavar un 'llm:...' corrupto a humano
// sería el lavado-por-corrupción que la rama leniente dejaba pasar — justo lo que el track prohíbe.
//
// Sabotaje: que SelloFueraDeTaxonomia nunca dispare ⇒ el sello inválido cae al default y se guarda.
// arnes: archivo="internal/memory/observation_quarantine.go"
// arnes: de="return provenance != \x22\x22 && !validProvenance(provenance)"
// arnes: a="return false"
func TestSaveConSelloInvalidoSeRechaza(t *testing.T) {
	s, _, _ := servidorCorroborar(t, false)
	rpcErr := guardarConSello(t, s, duenoOwn(), "push-ruido", "ruido", 0.5)
	if rpcErr == nil {
		t.Fatal("un sello fuera de la taxonomía debía rechazarse, no guardarse lavado a 'human'")
	}
	if rpcErr.Code != codeInvalidParams {
		t.Errorf("code=%d: esperaba codeInvalidParams (%d) para que el push lo mande a dead-letter, no a reintento infinito", rpcErr.Code, codeInvalidParams)
	}
}

// T6d (R6) — UNA CONFIDENCE FUERA DE [0,1] SE RECHAZA. Un push 'llm:*' con confidence fuera de rango
// se rechaza (ErrInvalidConfidence ⇒ codeInvalidParams, dead-letter), no se recorta en silencio:
// recortar convierte el error de quien empuja en un dato plausible y equivocado guardado para siempre.
//
// Sabotaje: desarmar el chequeo de rango en el cuello de escritura (saveObservation) ⇒ se guarda 1.5.
// arnes: archivo="internal/memory/operations.go"
// arnes: de="if conf < 0 || conf > 1 {"
// arnes: a="if false {"
func TestSaveConConfidenceFueraDeRangoSeRechaza(t *testing.T) {
	s, _, _ := servidorCorroborar(t, false)
	rpcErr := guardarConSello(t, s, duenoOwn(), "push-conf-mala", "llm:groq/llama-3.3", 1.5)
	if rpcErr == nil {
		t.Fatal("una confidence fuera de [0,1] debía rechazarse, no recortarse")
	}
	if rpcErr.Code != codeInvalidParams {
		t.Errorf("code=%d: esperaba codeInvalidParams (%d)", rpcErr.Code, codeInvalidParams)
	}
}

// T6f (D4) — UN 0.0 EN UN 'llm' SE RESPETA. Un 'llm:*' con confidence 0.0 ("no le creo nada", una
// afirmación legítima y distinta de "me olvidé del campo") se guarda 0.0, NO se re-infla a 1.0: el
// default 1.0 vive SÓLO en la rama 'human'. Que el 0.0 sobreviva prueba además que el receptor lee la
// confidence recibida y no una constante.
//
// Sabotaje: meter el default 1.0 en la rama que honra el sello (la no-human) ⇒ el 0.0 se pisa.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="case memory.HonrarSello(args.Provenance):"
// arnes: a="case memory.HonrarSello(args.Provenance):\n\t\t\tif args.Confidence == 0 {\n\t\t\t\targs.Confidence = 1.0\n\t\t\t}"
func TestSaveConConfidenceCeroEnUnLlmSeRespeta(t *testing.T) {
	s, _, dir := servidorCorroborar(t, false)
	if rpcErr := guardarConSello(t, s, duenoOwn(), "push-conf-cero", "llm:groq/llama-3.3", 0.0); rpcErr != nil {
		t.Fatalf("confidence 0.0 es un borde VÁLIDO, no debía fallar: %+v", rpcErr)
	}
	_, conf := selloYConfianza(t, dir, "push-conf-cero")
	if conf != 0.0 {
		t.Errorf("confidence=%v: un 0.0 legítimo de un 'llm' NO debe re-inflarse a 1.0 (esperaba 0.0)", conf)
	}
}

// T1 (D1, extremo a extremo) — EL SELLO CRUZA EL CABLE EN EL PUSH. Un OutboxItem ya sellado por el Q2
// del nodo origen (el sello que loadOutboxPayloads arma desde la fila —lo prueba
// TestElSelloLlmViajaEnElPush en memory—) se empuja por SyncClient.Push, y el test mira el eslabón que
// ningún otro tocaba: que Push COPIE provenance y confidence del item al payload que cruza la red, y
// que el central lo honre. El sello NO se re-inyecta del lado del central: viaja en el item y nada más.
// Por eso borrar la copia del push dejaba la suite verde —los T6* inyectan el sello en el raw y el
// reintento de TestLaSubidaComprimidaCuadraByteAByte lo vuelve a pasar explícito—, salvo por este test.
//
// Sabotaje: Push manda 'human' en vez de la provenance del item ⇒ el sello cruza lavado.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="Provenance: item.Provenance,"
// arnes: a="Provenance: \x22human\x22,"
//
// Sabotaje: Push manda 1.0 en vez de la confidence del item ⇒ el 0.7 no cruza.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="Confidence: item.Confidence,"
// arnes: a="Confidence: 1.0,"
func TestElSelloLlmCruzaElCableEnElPush(t *testing.T) {
	nodo, _, mirador, central := subidaContraCentralReal(t, nil)
	// El item que loadOutboxPayloads le entrega a Push, ya sellado por el Q2 del nodo origen.
	item := memory.OutboxItem{
		ObsID: "push-sellado", TopicKey: "t/sello", Content: "una inferencia del modelo que sube al central",
		Importance: 1, MemType: "semantic", ProjectID: "musubi",
		Provenance: "llm:groq/llama-3.3", Confidence: 0.7,
	}
	if err := nodo.syncClient.Push(item); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// EL PAYLOAD QUE CRUZÓ LA RED lleva el sello: el eslabón que faltaba es que Push lo copie del item.
	vistos := mirador.visto("push-sellado")
	if len(vistos) != 1 {
		t.Fatalf("esperaba un POST; llegaron %d", len(vistos))
	}
	if prov := vistos[0].args.Provenance; prov != "llm:groq/llama-3.3" {
		t.Errorf("el payload cruzó con provenance=%q; esperaba \"llm:groq/llama-3.3\" (Push no copió el sello del item)", prov)
	}
	if conf := vistos[0].args.Confidence; conf != 0.7 {
		t.Errorf("el payload cruzó con confidence=%v; esperaba 0.7", conf)
	}

	// Y EL CENTRAL LO GUARDÓ SELLADO, no lavado: la regla no-human lo honró de punta a punta.
	prov, conf, _, err := central.ObservationStamp("push-sellado")
	if err != nil {
		t.Fatalf("ObservationStamp en el central: %v", err)
	}
	if prov != "llm:groq/llama-3.3" || conf != 0.7 {
		t.Errorf("el central guardó la obs con sello %q/%v; esperaba \"llm:groq/llama-3.3\"/0.7 (sello lavado en el camino)", prov, conf)
	}
}
