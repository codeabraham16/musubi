package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

func TestCandadoCierreOrdenadoDelDuenoSueltaLaBajada(t *testing.T) {
	var pedidos atomic.Int64
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		pedidos.Add(1)
		payload := `{"items":[],"next_cursor":0}`
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer stub.Close()

	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	cfg := config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30}
	nuevo := func() *McpServer {
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
		s.SetSyncClient(newTestSyncClient(t, stub.URL), cfg)
		return s
	}
	duena, sobreviviente := nuevo(), nuevo()

	ctx, cancel := context.WithCancel(context.Background())
	termino := make(chan struct{})
	go func() { duena.RunInboundScheduler(ctx, 10*time.Millisecond); close(termino) }()
	limite := time.Now().Add(5 * time.Second)
	for pedidos.Load() == 0 && time.Now().Before(limite) {
		time.Sleep(5 * time.Millisecond)
	}
	if pedidos.Load() == 0 {
		t.Fatal("la dueña nunca bajó")
	}
	cancel()
	<-termino

	antes := pedidos.Load()
	sobreviviente.drainInboundOnce(context.Background())
	if pedidos.Load() == antes {
		t.Fatalf("la terminal que sigue abierta no bajó tras el cierre ORDENADO de la dueña: el candado quedó tomado hasta que venza (%d s)", duena.leaseBajadaSegundos())
	}
}
