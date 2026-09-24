package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"encoding/json"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

func TestCandadoLoBajadoQuedaSinVectorSiElDuenoEsLexico(t *testing.T) {
	var hayFilas atomic.Bool
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Arguments struct {
					AfterRowID int64 `json:"after_rowid"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		after := req.Params.Arguments.AfterRowID
		payload := `{"items":[],"next_cursor":` + strconv.FormatInt(after, 10) + `}`
		if hayFilas.Load() && after < 5 {
			payload = `{"items":[` +
				`{"rowid":3,"id":"c1","topic_key":"t/a","content":"alpha del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"},` +
				`{"rowid":5,"id":"c2","topic_key":"t/b","content":"beta del central","importance":1,"mem_type":"semantic","author":"juan","project_id":"acme"}` +
				`],"next_cursor":5}`
		}
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer stub.Close()

	dir := memtest.DirSembrado(t)
	engLexico, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	engSemantico, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.SyncConfig{BatchSize: 200, DrainIntervalSeconds: 30}
	vec := []float32{0.1, 0.2, 0.3}

	lexica := NewMcpServer(engLexico, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	lexica.SetSyncClient(newTestSyncClient(t, stub.URL), cfg)
	semantica := NewMcpServer(engSemantico, t.TempDir(), fakeEmbedder{vec: vec}, WithMemory(config.MemoryConfig{TeamMode: true}))
	semantica.SetSyncClient(newTestSyncClient(t, stub.URL), cfg)

	lexica.drainInboundOnce(context.Background())
	hayFilas.Store(true)
	semantica.drainInboundOnce(context.Background())
	lexica.drainInboundOnce(context.Background())
	for i := 0; i < 3; i++ {
		semantica.drainInboundOnce(context.Background())
		lexica.drainInboundOnce(context.Background())
	}

	engSemantico.EsperarRelleno()
	engLexico.EsperarRelleno()
	_ = engSemantico.Close()
	_ = engLexico.Close()

	e2, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	if raw, ok, _ := e2.GetMeta(metaInboundCursor); !ok || raw != "5" {
		t.Fatalf("cursor = %q (ok=%v): las filas ni se bajaron", raw, ok)
	}
	e2.SetVectorModelID("fake")
	res, err := e2.SearchObservations(context.Background(), vec, 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range res {
		got[r.ID] = true
	}
	for _, id := range []string{"c1", "c2"} {
		if !got[id] {
			t.Errorf("%s bajó del central y hay una terminal semántica viva sobre la base, pero quedó SIN vector: la bajó sólo la dueña léxica del candado (encontradas: %v)", id, got)
		}
	}
}
