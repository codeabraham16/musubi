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

// UN DUEÑO QUE FALLA LENTO tampoco puede dejar sin bajada a la terminal sana. Un Pull que muere por
// timeout (request_timeout_seconds=30 = drain_interval_seconds=30 por defecto) retiene el candado
// todo el tick; al soltarlo, el tick siguiente ya está encolado en el Ticker y lo retoma en
// microsegundos. Escala: intervalo 1 s y timeout 1 s (relación 1:1), 6 s de reloj.
func TestDrainInboundUnDuenoQueFallaLentoNoDejaSinBajadaALaSana(t *testing.T) {
	liberar := make(chan struct{})
	colgado := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		select { // no responde nunca: el cliente corta por timeout
		case <-r.Context().Done():
		case <-liberar:
		}
	}))
	defer colgado.Close()
	defer close(liberar)

	var pedidosSano atomic.Int64
	sano := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		pedidosSano.Add(1)
		payload := `{"items":[{"rowid":3,"id":"c1","topic_key":"t/a","content":"alpha del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"}],"next_cursor":3}`
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer sano.Close()

	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	t.Setenv("MUSUBI_TEST_TOKEN", "secreto-abc")
	nuevo := func(url string) *McpServer {
		cfg := config.SyncConfig{
			CentralURL:            url,
			AuthTokenEnv:          "MUSUBI_TEST_TOKEN",
			AllowInsecureToken:    true,
			RequestTimeoutSeconds: 1, // = intervalo, como los defaults 30/30
			BatchSize:             50,
			DrainIntervalSeconds:  30,
		}
		client, err := NewSyncClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
		s.SetSyncClient(client, cfg)
		return s
	}
	rota, sana := nuevo(colgado.URL), nuevo(sano.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	hecho := make(chan struct{}, 2)
	go func() { rota.RunInboundScheduler(ctx, time.Second); hecho <- struct{}{} }()
	time.Sleep(400 * time.Millisecond) // la sana ticka a otra fase, como dos terminales reales
	go func() { sana.RunInboundScheduler(ctx, time.Second); hecho <- struct{}{} }()
	<-hecho
	<-hecho

	if pedidosSano.Load() == 0 {
		t.Fatal("la terminal sana NUNCA bajó en 6 ticks: el dueño que falla por timeout retiene el candado")
	}
}
