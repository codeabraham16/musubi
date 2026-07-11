package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestDrainInboundIngestsAndAdvancesCursor valida el client side del sync ENTRANTE (C5.3b-2): el
// drain baja páginas de memoria shared del central (musubi_sync_pull), las ingiere localmente
// (IngestShared, anti-loop) y avanza el cursor persistente. Central = un httptest stub que sirve una
// página con 2 items y luego vacío.
func TestDrainInboundIngestsAndAdvancesCursor(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					AfterRowID int64 `json:"after_rowid"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)

		var payload string
		if req.Params.Arguments.AfterRowID == 0 {
			payload = `{"items":[` +
				`{"rowid":3,"id":"c1","topic_key":"t/a","content":"alpha del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"},` +
				`{"rowid":5,"id":"c2","topic_key":"t/b","content":"beta del central","importance":1,"mem_type":"semantic","author":"juan","project_id":"acme"}` +
				`],"next_cursor":5}`
		} else {
			payload = `{"items":[],"next_cursor":` + strconv.FormatInt(req.Params.Arguments.AfterRowID, 10) + `}`
		}
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}))
	defer stub.Close()

	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	client := newTestSyncClient(t, stub.URL)
	s.SetSyncClient(client, config.SyncConfig{BatchSize: 200})

	s.drainInboundOnce(context.Background())

	// Los 2 items del central se ingirieron localmente (visibles como shared, federado).
	got, err := engine.ListSharedForPull(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]memory.SharedObs{}
	for _, o := range got {
		byID[o.ID] = o
	}
	if _, ok := byID["c1"]; !ok {
		t.Fatalf("c1 no se ingirió; ingeridos: %d", len(got))
	}
	if byID["c1"].Author != "ana" || byID["c2"].Author != "juan" {
		t.Errorf("atribución no preservada al ingerir: c1=%q c2=%q", byID["c1"].Author, byID["c2"].Author)
	}

	// ANTI-LOOP: lo bajado NO se encoló en el outbox local (no rebota al central).
	if p, _, _, _ := engine.OutboxStats(); p != 0 {
		t.Errorf("ANTI-LOOP roto: outbox pending = %d tras ingerir del central, esperaba 0", p)
	}

	// El cursor entrante avanzó al mayor rowid del central (5).
	if raw, ok, _ := engine.GetMeta("sync:inbound_cursor"); !ok || raw != "5" {
		t.Errorf("cursor entrante = %q (ok=%v), esperaba \"5\"", raw, ok)
	}
}
