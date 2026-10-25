package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestRecallAdelgazaElEnvoltorio fija lo que musubi_recall EMITE al modelo (specs/recall-envoltorio/).
// El gist y el valor no se tocan; lo que se recorta es envoltorio que el agente re-lee en cada turno
// sin usarlo. Un cambio así regresa callado si nada lo guarda, que es lo que este test impide.
func TestRecallAdelgazaElEnvoltorio(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	// WithConflicts apagado: la siembra son casi-duplicados y el auto-supersede ocultaría parte.
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithConflicts(config.ConflictConfig{}))
	sembrar(t, s, 5)
	exigeSembradas(t, engine, 5)

	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_recall","arguments":{"query":"` + preguntaSembrada + `","token_budget":4000}}`)
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("recall: %+v", rpcErr)
	}
	texto := out.(CallToolResponse).Content[0].Text

	var res memory.RecallResult
	if err := json.Unmarshal([]byte(texto), &res); err != nil {
		t.Fatalf("la respuesta del recall no es JSON: %v\n%s", err, texto)
	}
	if len(res.Items) < 2 {
		t.Fatalf("esperaba varios items para medir el envoltorio, vinieron %d: %s", len(res.Items), texto)
	}

	// 1.1 — el puntaje model-free VIAJA en la respuesta: musubi-body (cmd/musubi-body-ui/memoria.go)
	// lo copia a su Item, y quitarlo lo dejaría con 0 en silencio. Sin rerank (este test) todos los
	// items traen score > 0, así que la clave está presente. Esta guarda frena que alguien vuelva a
	// ponerle json:"-" al campo y rompa a ese consumidor sin que ningún test lo note.
	// Sabotaje: ponerle a RecallItem.Score el tag `json:"-"`.
	// arnes: archivo="internal/memory/recall.go"
	// arnes: de="json:\"score,omitempty\""
	// arnes: a="json:\"-\""
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	if !strings.Contains(texto, `"score"`) {
		t.Errorf("el puntaje model-free no viajó en la respuesta del recall (lo lee musubi-body): %s", texto)
	}

	// 1.2 — created_at viaja a resolución de DÍA (YYYY-MM-DD), sin hora.
	// Sabotaje: quitar soloFecha() en empaquetar (volver a `CreatedAt: c.createdAt`).
	// arnes: archivo="internal/memory/recall.go"
	// arnes: de="CreatedAt:   soloFecha(c.createdAt),"
	// arnes: a="CreatedAt:   c.createdAt,"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	for _, it := range res.Items {
		if it.CreatedAt == "" {
			continue // una nota sin fecha sale sin la clave: no es lo que este invariante mide
		}
		if len(it.CreatedAt) != 10 || strings.ContainsAny(it.CreatedAt, "T: ") {
			t.Errorf("created_at de %s viajó con hora (%q); esperaba sólo la fecha YYYY-MM-DD", it.ID, it.CreatedAt)
		}
	}

	// 3 — used_tokens informa el COSTO REAL del payload, no la suma de gists. Se fija con DOS cotas:
	//   (a) el envoltorio (id, topic_key, created_at, las claves JSON repetidas) pesa más que el gist,
	//       así que el número honesto SUPERA la suma de gists; el camino viejo informaba sólo esa suma.
	//   (b) y es, salvo redondeo, el tamaño estimado del payload EMITIDO: used_tokens ≈ EstimateTokens(texto).
	//       La (b) es la que mide de verdad: la (a) sola la pasa un subconteo de hasta el 50%.
	// Sabotaje: quitar en toolRecall `res.UsedTokens = memory.EstimateTokens(string(raw))` ⇒ caen (a) y (b);
	// devolver la mitad, `EstimateTokens(string(raw)) / 2` ⇒ (a) sobrevive, (b) cae.
	// arnes: archivo="internal/mcp/methods.go"
	// arnes: de="res.UsedTokens = memory.EstimateTokens(string(raw))"
	// arnes: a="res.UsedTokens = memory.EstimateTokens(string(raw)) / 2"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	gistSum := 0
	for _, it := range res.Items {
		gistSum += memory.EstimateTokens(it.Gist)
	}
	if res.UsedTokens <= gistSum {
		t.Errorf("used_tokens=%d no supera la suma de gists=%d: informa sólo el gist, no el payload real que entra al contexto",
			res.UsedTokens, gistSum)
	}
	// La medición interna marshalla ANTES de fijar used_tokens a sí mismo, así que difiere del texto
	// final sólo en sus propios dígitos: una tolerancia de ±2 tokens cubre ese desfase y nada más.
	if emitido := memory.EstimateTokens(texto); res.UsedTokens-emitido < -2 || res.UsedTokens-emitido > 2 {
		t.Errorf("used_tokens=%d no refleja el payload emitido EstimateTokens(texto)=%d (dif %d, tolerancia ±2): no mide el costo real",
			res.UsedTokens, emitido, res.UsedTokens-emitido)
	}
}
