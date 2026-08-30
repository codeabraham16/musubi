package mcp

import (
	"errors"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// El invariante: UN FALLO DEL EMBEDDER NO PIERDE LA OBSERVACIÓN. El save degrada — guarda sin
// vector — y el backfill la embebe después. Medido en el central (2026-08-26): tres
// musubi_save_observation muertos en 10 minutos con ms=30047 clavados porque Ollama se colgó y el
// save era «embed o muerte»; cada uno era una captura del usuario perdida entera. La regla del
// repo lo dice desde siempre: embeddings opcionales CON FALLBACK.
//
// Sabotaje que lo pone rojo: volver fatal el error de embed en el camino del save (el estado en
// que estaba el código cuando este test se escribió — nació rojo contra ese código, verificado).
func TestSaveObservationSobreviveAlEmbedderCaido(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), fakeEmbedder{vec: []float32{1, 0, 0, 0}, err: errors.New("embedder colgado")})

	// 1 · El save con el embedder CAÍDO tiene que entrar igual.
	res, rpcErr := call(t, s, "musubi_save_observation", map[string]interface{}{
		"topic_key": "prueba/embedder-caido",
		"content":   "la observación que antes se perdía cuando ollama se colgaba",
	})
	if rpcErr != nil {
		t.Fatalf("el save murió con el embedder caído — la observación se perdió: %+v", rpcErr)
	}
	if res == nil {
		t.Fatal("save sin resultado")
	}

	// 2 · Y tiene que estar DE VERDAD en la base (léxico, sin vector).
	found, rpcErr := call(t, s, "musubi_search_keyword", map[string]interface{}{
		"query_text": "ollama se colgaba",
	})
	if rpcErr != nil {
		t.Fatalf("search_keyword error: %+v", rpcErr)
	}
	fr, ok := found.(CallToolResponse)
	if !ok || len(fr.Content) == 0 {
		t.Fatalf("resultado de search_keyword no es CallToolResponse con content: %#v", found)
	}
	if !strings.Contains(fr.Content[0].Text, "embedder-caido") {
		t.Fatalf("la observación no aparece por keyword: %s", fr.Content[0].Text)
	}

	// 3 · El ciclo se cierra: el backfill la encuentra pendiente y la embebe cuando el embedder
	//     vuelve. Sin esta mitad, «degradar» sería un eufemismo de «memoria sin semántica».
	//     La procedencia se enciende como lo hace serve/daemon con un embedder nombrado.
	engine.SetVectorModelID("fake:test")
	bf, err := engine.EmbedBackfill(func(textos []string) ([][]float32, error) {
		vecs := make([][]float32, len(textos))
		for i := range textos {
			vecs[i] = []float32{1, 0, 0, 0}
		}
		return vecs, nil
	})
	if err != nil {
		t.Fatalf("EmbedBackfill error: %v", err)
	}
	if bf.Embedded < 1 {
		t.Fatalf("el backfill no embebió la observación pendiente: %+v", bf)
	}
}
