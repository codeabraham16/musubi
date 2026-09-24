package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// El dueño renueva el candado ANTES DE CADA PÁGINA, y si otro lo tomó en el medio, corta: un tick de
// veinte páginas con su timeout dura más que el lease, y vencido a mitad, otro proceso lo tomaba y
// los dos bajaban las mismas páginas. Acá, mientras el central contesta la primera página, el
// candado pasa a otro proceso (como si el del dueño hubiera vencido y el otro lo hubiera tomado).
//
// Sabotaje que la pone roja: no renovar entre páginas.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\tif page > 0 {\n\t\t\tif sigo, rerr := s.engine.ReclamarBajada("
// arnes: a="\t\tif page > 0 && false {\n\t\t\tif sigo, rerr := s.engine.ReclamarBajada("
func TestLaBajadaSeCortaSiOtroTomoElCandadoEntrePaginas(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	var pedidos atomic.Int64
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
		if pedidos.Add(1) == 1 {
			if err := engine.SetMeta("sync:inbound_lease", "otro-proceso|2999-01-01 00:00:00"); err != nil {
				t.Errorf("simular al otro proceso: %v", err)
			}
		}
		// Siempre una página llena (BatchSize 1): sin el corte, el dueño seguiría hasta 20 páginas.
		n := req.Params.Arguments.AfterRowID + 1
		id := strconv.FormatInt(n, 10)
		payload := `{"items":[{"rowid":` + id + `,"id":"c` + id + `","topic_key":"t/a","content":"fila ` + id +
			` del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"}],"next_cursor":` + id + `}`
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer stub.Close()

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 1, DrainIntervalSeconds: 30})

	s.drainInboundOnce(context.Background())
	if n := pedidos.Load(); n != 1 {
		t.Fatalf("otro proceso tomó el candado durante la primera página y el dueño siguió bajando: %d páginas (esperaba 1)", n)
	}
}
