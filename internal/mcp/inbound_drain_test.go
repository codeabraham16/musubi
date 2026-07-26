package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// failingIngestEngine envuelve un engine real y hace fallar IngestShared para un id concreto,
// simulando un error transitorio (p. ej. SQLITE_BUSY) al ingerir esa fila.
type failingIngestEngine struct {
	*memory.DbEngine
	failID string
}

func (f *failingIngestEngine) IngestShared(o memory.SharedObs) (bool, error) {
	if o.ID == f.failID {
		return false, fmt.Errorf("simulado: fallo transitorio al ingerir %s", o.ID)
	}
	return f.DbEngine.IngestShared(o)
}

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

// REGRESIÓN (auditoría 2026-07-26, #5): un IngestShared fallido avanzaba el cursor igual hasta el
// mayor rowid del batch ⇒ esa fila quedaba saltada PARA SIEMPRE (hueco permanente). Ahora el cursor
// avanza sólo hasta la última fila ingerida OK de forma contigua; la que falló se re-baja.
func TestDrainInboundDoesNotAdvancePastFailedRow(t *testing.T) {
	// Central: c1(rowid3), c2(rowid5), c3(rowid7), next_cursor=7.
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
		var payload string
		if req.Params.Arguments.AfterRowID < 3 {
			payload = `{"items":[` +
				`{"rowid":3,"id":"c1","topic_key":"t/a","content":"alpha","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"},` +
				`{"rowid":5,"id":"c2","topic_key":"t/b","content":"beta","importance":1,"mem_type":"semantic","author":"juan","project_id":"acme"},` +
				`{"rowid":7,"id":"c3","topic_key":"t/c","content":"gamma","importance":1,"mem_type":"semantic","author":"eva","project_id":"acme"}` +
				`],"next_cursor":7}`
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
	// c2 (rowid 5) falla al ingerir.
	wrapped := &failingIngestEngine{DbEngine: engine, failID: "c2"}
	s := NewMcpServer(wrapped, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 200})

	s.drainInboundOnce(context.Background())

	// El cursor se detuvo en 3 (última fila OK contigua), NO saltó a 7.
	if raw, ok, _ := engine.GetMeta("sync:inbound_cursor"); !ok || raw != "3" {
		t.Errorf("cursor entrante = %q (ok=%v), esperaba \"3\" (no debe pasar la fila fallida)", raw, ok)
	}
	// c1 se ingirió; c3 (después del fallo) NO.
	got, _ := engine.ListSharedForPull(context.Background(), 0, 100)
	ids := map[string]bool{}
	for _, o := range got {
		ids[o.ID] = true
	}
	if !ids["c1"] {
		t.Error("c1 (antes del fallo) debía ingerirse")
	}
	if ids["c3"] {
		t.Error("c3 (después del fallo) NO debía ingerirse: el drain corta en el fallo")
	}
}
