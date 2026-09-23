package mcp

import (
	"context"
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

// EL INVARIANTE QUE EL DUEÑO VE: dos terminales abiertas en el mismo proyecto NO bajan las mismas
// páginas. Medido en el central el 2026-09-23, en 24 h y contra las 2.880 consultas de UN proceso
// cada 30 s: la laptop tiró 4.997 (1,7 procesos) y Altura 4.374 (1,5). La subida estaba protegida
// por el lease del outbox desde siempre; la bajada no tenía nada.
//
// La prueba cuenta los pedidos que LLEGAN al central, que es el costo real, y no una variable
// interna: un candado que se toma pero no frena el Pull pasaría cualquier aserción sobre el candado
// y fallaría ésta.
func TestDrainInboundDosProcesosNoBajanLoMismo(t *testing.T) {
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

	// Dos servidores sobre la MISMA base: dos terminales abiertas en el mismo proyecto.
	nuevo := func() *McpServer {
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
		s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})
		return s
	}
	terminalA, terminalB := nuevo(), nuevo()
	if terminalA.duenoBajada == terminalB.duenoBajada {
		t.Fatal("dos servidores tienen que tener identidades distintas ante el candado")
	}

	terminalA.drainInboundOnce(context.Background())
	despuesDeA := pedidos.Load()
	if despuesDeA == 0 {
		t.Fatal("la primera terminal tenía que bajar: el candado estaba libre")
	}

	terminalB.drainInboundOnce(context.Background())
	if got := pedidos.Load(); got != despuesDeA {
		t.Fatalf("la segunda terminal le pegó al central (%d pedidos, quería %d): bajan las dos lo mismo", got, despuesDeA)
	}

	// Y el dueño SÍ sigue bajando en su tick siguiente: el candado no se queda trabado.
	terminalA.drainInboundOnce(context.Background())
	if got := pedidos.Load(); got <= despuesDeA {
		t.Fatalf("el dueño del candado tenía que seguir bajando; pedidos=%d", got)
	}
}

// El lease tiene que durar más que el intervalo: si venciera entre dos ticks, el otro proceso lo
// tomaría y los dos se alternarían bajando lo mismo, que es el defecto disfrazado de arreglo.
func TestLeaseBajadaDuraMasQueElTick(t *testing.T) {
	for _, intervalo := range []int{0, 10, 30, 60, 300} {
		s := &McpServer{syncCfg: config.SyncConfig{DrainIntervalSeconds: intervalo}}
		if got := s.leaseBajadaSegundos(); got <= intervalo || got < 120 {
			t.Errorf("intervalo %ds: el lease de %ds no le gana al tick (o baja del piso de 120)", intervalo, got)
		}
	}
}

// El SCHEDULER escribe el cursor de forma monótona — no alcanza con que exista la función monótona,
// tiene que ser la que se llama. Acá el central falso, mientras atiende el pedido, adelanta el cursor
// a 999: es la otra terminal, que tomó el candado vencido y llegó más lejos durante este tick lento.
// Con SetMeta a secas esta terminal escribiría 5 encima y el cursor RETROCEDERÍA: la próxima bajada
// re-traería del 5 al 999, justo el tráfico que el candado viene a sacar.
func TestDrainInboundNoRetrocedeElCursor(t *testing.T) {
	var engine *memory.DbEngine
	var primera atomic.Bool
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		if primera.CompareAndSwap(false, true) {
			if err := engine.SetMeta(metaInboundCursor, "999"); err != nil {
				t.Errorf("setup: %v", err)
			}
		}
		payload := `{"items":[` +
			`{"rowid":3,"id":"r1","topic_key":"t/a","content":"uno del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"},` +
			`{"rowid":5,"id":"r2","topic_key":"t/b","content":"dos del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"}` +
			`],"next_cursor":5}`
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer stub.Close()

	var err error
	engine, err = memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})

	s.drainInboundOnce(context.Background())

	if got, _, _ := engine.GetMeta(metaInboundCursor); got != "999" {
		t.Fatalf("el cursor RETROCEDIÓ a %q: la escritura del scheduler no es monótona", got)
	}
}

// LO QUE ENCONTRÓ LA REVISIÓN ADVERSARIAL (y su prueba, adaptada): una terminal que falla SIEMPRE
// —token vencido de una que arrancó antes de rotarlo, un central_url viejo— renovaba el candado en
// cada tick antes de ir a la red, y la terminal sana no bajaba nunca más. Reproducido antes del
// arreglo: cinco ticks cada una, la sana hizo CERO pedidos. Sin candado la sana bajaba sola; el
// candado no puede dejarla peor.
func TestDrainInboundUnDuenoQueFallaNoDejaSinBajadaALosDemas(t *testing.T) {
	roto := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer roto.Close()
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
	nuevo := func(url string) *McpServer {
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
		s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})
		return s
	}
	rota, sana := nuevo(roto.URL), nuevo(sano.URL)
	// La rota arranca primero y toma el candado: es el peor orden.
	for tick := 0; tick < 3; tick++ {
		rota.drainInboundOnce(context.Background())
		sana.drainInboundOnce(context.Background())
	}
	if pedidosSano.Load() == 0 {
		t.Fatal("la terminal sana NUNCA bajó: el dueño que falla se quedó con el candado")
	}
	if cur, _, _ := engine.GetMeta(metaInboundCursor); cur != "3" {
		t.Fatalf("la bajada de la sana tenía que llegar a la base: cursor=%q", cur)
	}
}
