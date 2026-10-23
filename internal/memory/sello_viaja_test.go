package memory

import (
	"context"
	"testing"
)

// EL SELLO 'llm:' + confidence VIAJAN DE PUNTA A PUNTA (SDD sello-llm-viaja-al-central).
//
// LO QUE SE PRUEBA, Y POR QUÉ. Hasta este cambio, una inferencia de un modelo corroborada y promovida
// salía al central LAVADA a 'human/1.0': el push reconstruía el payload desde la fila por JOIN pero no
// leía su procedencia, así que el pilar «la procedencia es parte del producto» se cortaba en el primer
// salto de red. Acá se cubren los dos extremos que viven en el paquete memory: el EMISOR (el push
// lleva el sello de la fila, no uno inventado) y la BAJADA (el sello baja del central, la fila espejo
// nace con él y una re-entrega no lo re-lava). El receptor del central —la regla no-human sobre el
// payload entrante— se ejerce por el borde MCP y vive en internal/mcp (sello_receptor_test.go).
//
// R5 en el camino de ESCRITURA (un save posterior no pisa el sello de una fila existente) está cubierto
// en observation_quarantine_test.go: la mitad de provenance/cuarentena por
// TestQ2UnSavePosteriorNoLavaElSello y la de confidence por TestR5LaConfidenceSelladaSobreviveAUnSavePlano.
// Acá se cubre el espejo de R5 del lado de la BAJADA, que es otro UPSERT (el de IngestShared), con su
// mitad de confidence 0.0 en TestIngestSharedRespetaConfidenceCero.

// selloDeLaFila lee provenance/confidence tal como quedaron PERSISTIDOS, por fuera del engine, para
// afirmar el estado en disco y no lo que el engine dice que hizo. ObservationStamp es el lector del
// sello exportado para tests.
func selloDeLaFila(t *testing.T, e *DbEngine, id string) (string, float64) {
	t.Helper()
	prov, conf, _, err := e.ObservationStamp(id)
	if err != nil {
		t.Fatalf("ObservationStamp(%q): %v", id, err)
	}
	return prov, conf
}

// TestElSelloLlmViajaEnElPush — EMISOR (R1). Una obs 'llm:*' corroborada y promovida se encola, y al
// reclamarla para el push el OutboxItem lleva SU procedencia y SU confidence —leídas de la fila—, no
// 'human/1.0'. Es el sello que el Q2 del nodo origen ya estampó, intacto hasta el cable.
//
// Sabotaje: hardcodear 'human' en el SELECT de loadOutboxPayloads ⇒ el payload viaja lavado.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="COALESCE(content_hash, \x27\x27), COALESCE(provenance, \x27\x27), COALESCE(confidence, 1.0)"
// arnes: a="COALESCE(content_hash, \x27\x27), \x27human\x27, 1.0"
func TestElSelloLlmViajaEnElPush(t *testing.T) {
	e := newTestEngine(t)
	id, err := e.ProposeObservation("musubi", "davantis", "t/sello", "una inferencia del modelo que se corrobora y viaja al central", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}
	if err := e.CorroborateObservation(id); err != nil {
		t.Fatalf("CorroborateObservation: %v", err)
	}
	if err := e.PromoteObservation(id); err != nil {
		t.Fatalf("PromoteObservation: %v", err)
	}

	items, err := e.ClaimOutboxBatch(10, 60)
	if err != nil {
		t.Fatalf("ClaimOutboxBatch: %v", err)
	}
	var it *OutboxItem
	for i := range items {
		if items[i].ObsID == id {
			it = &items[i]
			break
		}
	}
	if it == nil {
		t.Fatalf("la obs promovida %q no apareció en el outbox (hay %d filas)", id, len(items))
	}
	if it.Provenance != "llm:groq/llama-3.3" {
		t.Errorf("el push lleva provenance=%q; esperaba \"llm:groq/llama-3.3\" (no lavado a human)", it.Provenance)
	}
	if it.Confidence != 0.8 {
		t.Errorf("el push lleva confidence=%v; esperaba 0.8", it.Confidence)
	}
}

// TestElSelloViajaEnElPull — BAJADA, lista (R7). Lo que el central lista para el pull lleva la
// procedencia y la confidence de la fila. Sin esto, B baja el contenido sin saber qué clase de proceso
// lo generó y lo guarda como 'human'. Se siembra con un sello crudo por SQL para probar SÓLO el SELECT
// de ListSharedForPull, independiente de IngestShared (que es lo que cubren los tests de abajo).
//
// Sabotaje: el SELECT deja de leer el sello (viaja vacío ⇒ B lo trata como un central viejo).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="COALESCE(provenance,\x27\x27), COALESCE(confidence,1.0)"
// arnes: a="\x27\x27, 1.0"
func TestElSelloViajaEnElPull(t *testing.T) {
	central := newTestEngine(t)
	if err := central.SaveObservationTyped("baja-sellada", "t/bajada", "inferencia que el central comparte", 1.0, "semantic", ScopeShared, nil); err != nil {
		t.Fatalf("SaveObservationTyped: %v", err)
	}
	if _, err := central.db.Exec(`UPDATE observations SET provenance = 'llm:groq/llama-3.3', confidence = 0.7 WHERE id = 'baja-sellada'`); err != nil {
		t.Fatalf("sellar la fila: %v", err)
	}

	items, err := central.ListSharedForPull(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("ListSharedForPull: %v", err)
	}
	var o *SharedObs
	for i := range items {
		if items[i].ID == "baja-sellada" {
			o = &items[i]
			break
		}
	}
	if o == nil {
		t.Fatalf("la obs shared no apareció en el pull (hay %d filas)", len(items))
	}
	if o.Provenance != "llm:groq/llama-3.3" {
		t.Errorf("el pull lleva provenance=%q; esperaba \"llm:groq/llama-3.3\"", o.Provenance)
	}
	if o.Confidence != 0.7 {
		t.Errorf("el pull lleva confidence=%v; esperaba 0.7", o.Confidence)
	}
}

// TestIngestSharedEscribeElSello — BAJADA, ingest (R7/D5). La fila espejo nace con el sello que baja,
// escrito VERBATIM: el central ya saneó en el borde del push, así que la bajada es relay confiable.
//
// Sabotaje: ignorar el sello entrante y escribir siempre 'human' ⇒ la fila espejo pierde la procedencia.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="prov = o.Provenance"
// arnes: a="prov = provenanceHuman"
func TestIngestSharedEscribeElSello(t *testing.T) {
	b := newTestEngine(t)
	if _, err := b.IngestShared(SharedObs{ID: "espejo", TopicKey: "t/bajada", Content: "inferencia bajada del central", Importance: 1, Provenance: "llm:groq/llama-3.3", Confidence: 0.7, CreatedAt: "2026-06-29 10:11:12"}); err != nil {
		t.Fatalf("IngestShared: %v", err)
	}
	prov, conf := selloDeLaFila(t, b, "espejo")
	if prov != "llm:groq/llama-3.3" {
		t.Errorf("la fila espejo quedó con provenance=%q; esperaba \"llm:groq/llama-3.3\"", prov)
	}
	if conf != 0.7 {
		t.Errorf("la fila espejo quedó con confidence=%v; esperaba 0.7", conf)
	}
}

// TestReingestarNoPisaElSello — BAJADA, UPSERT (espejo de R5). Una re-entrega con OTRO sello no cambia
// el de una fila existente: el DO UPDATE SET de IngestShared no toca provenance/confidence, igual que
// el UPSERT del central tampoco degrada un 'llm:*' ya guardado. Sin esto, una edición que baja (misma
// fila, otro sello en el payload) re-lavaría la procedencia en cada vuelta.
//
// Sabotaje: el DO UPDATE también pisa el sello con el que llega (ancla en la línea de `importance`,
// distinta de la de `sync_seq` que ataca el test de la fecha, para no colisionar con él).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="importance=excluded.importance,"
// arnes: a="importance=excluded.importance,\n\t\t\t\tprovenance=excluded.provenance,\n\t\t\t\tconfidence=excluded.confidence,"
func TestReingestarNoPisaElSello(t *testing.T) {
	b := newTestEngine(t)
	if _, err := b.IngestShared(SharedObs{ID: "espejo", TopicKey: "t/bajada", Content: "v1 sellada por el modelo", Importance: 1, Provenance: "llm:groq/llama-3.3", Confidence: 0.7, CreatedAt: "2026-06-29 10:11:12"}); err != nil {
		t.Fatalf("IngestShared v1: %v", err)
	}
	// Re-entrega de la MISMA fila con un sello distinto (una edición que baja, p. ej.).
	if _, err := b.IngestShared(SharedObs{ID: "espejo", TopicKey: "t/bajada", Content: "v2 editada", Importance: 1, Provenance: "human", Confidence: 1.0, CreatedAt: "2026-06-29 10:11:12"}); err != nil {
		t.Fatalf("IngestShared v2: %v", err)
	}
	prov, conf := selloDeLaFila(t, b, "espejo")
	if prov != "llm:groq/llama-3.3" {
		t.Errorf("la re-entrega cambió provenance a %q; el DO UPDATE no debe tocar el sello (esperaba \"llm:groq/llama-3.3\")", prov)
	}
	if conf != 0.7 {
		t.Errorf("la re-entrega cambió confidence a %v; esperaba 0.7", conf)
	}
}

// TestBajadaDeUnCentralViejoCaeAHumano — COMPAT (R9). Un central viejo no manda el sello: SharedObs
// llega con Provenance vacío y la fila espejo toma el default human/1.0, la conducta de siempre.
// validProvenance es sólo el portón default-vs-verbatim: lo vacío cae al default, no es un error.
//
// Sabotaje: escribir el sello vacío verbatim ⇒ la fila nace con provenance='' en vez de 'human'.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="prov, conf := provenanceHuman, 1.0"
// arnes: a="prov, conf := o.Provenance, o.Confidence"
func TestBajadaDeUnCentralViejoCaeAHumano(t *testing.T) {
	b := newTestEngine(t)
	if _, err := b.IngestShared(SharedObs{ID: "sin-sello", TopicKey: "t/bajada", Content: "nota de un central viejo", Importance: 1, CreatedAt: "2026-06-29 10:11:12"}); err != nil {
		t.Fatalf("IngestShared: %v", err)
	}
	prov, conf := selloDeLaFila(t, b, "sin-sello")
	if prov != "human" {
		t.Errorf("sin sello la fila quedó con provenance=%q; esperaba \"human\" (default)", prov)
	}
	if conf != 1.0 {
		t.Errorf("sin sello la fila quedó con confidence=%v; esperaba 1.0", conf)
	}
}

// TestIngestSharedRespetaConfidenceCero — BAJADA, 0.0 verbatim (D4/D5). Un 'llm:*' que baja con
// confidence 0.0 ("el central no le cree nada", distinto de "no mandó el campo") se escribe 0.0, NO se
// re-infla a 1.0: el default 1.0 vive sólo en la rama human de la bajada. Es el espejo de T6f (que lo
// prueba en el receptor). TestIngestSharedEscribeElSello usa 0.7 y no vería una re-inflación del 0.0,
// así que ésta cubre que `conf = o.Confidence` lea la confidence bajada y no una constante.
//
// Sabotaje: re-inflar un 0.0 a 1.0 en la rama que honra el sello de la bajada.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="conf = o.Confidence"
// arnes: a="conf = o.Confidence\n\t\t\tif conf == 0 {\n\t\t\t\tconf = 1.0\n\t\t\t}"
func TestIngestSharedRespetaConfidenceCero(t *testing.T) {
	b := newTestEngine(t)
	if _, err := b.IngestShared(SharedObs{ID: "espejo-cero", TopicKey: "t/bajada", Content: "inferencia que el central no cree", Importance: 1, Provenance: "llm:groq/llama-3.3", Confidence: 0.0, CreatedAt: "2026-06-29 10:11:12"}); err != nil {
		t.Fatalf("IngestShared: %v", err)
	}
	prov, conf := selloDeLaFila(t, b, "espejo-cero")
	if prov != "llm:groq/llama-3.3" {
		t.Errorf("la fila espejo quedó con provenance=%q; esperaba \"llm:groq/llama-3.3\"", prov)
	}
	if conf != 0.0 {
		t.Errorf("la fila espejo quedó con confidence=%v; un 0.0 que baja NO debe re-inflarse a 1.0 (esperaba 0.0)", conf)
	}
}
