package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// grafo_generacion_test.go custodia que un cambio del grafo llegue SIEMPRE al central sin volver al
// push de cada tick: la generación durable (memory/codegraph_generacion.go), pushMu y la higiene.

const conGamma = "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n\nfunc Gamma() {}\n"

// M1 — UN CAMBIO QUE INDEXÓ OTRO DAEMON LLEGA AL CENTRAL.
//
// Sobre la misma .musubi/memory.db corren varios daemons. B no tiene sync (o su sesión muere antes
// de empujar) e indexa Gamma; A, con sync, ve el árbol limpio. Con la marca en memoria de un proceso
// A no empujaba nunca (medido: pushes=1 tras 3 ticks). Sabotaje que la pone roja: que el scheduler
// decida con `cambio` —lo que indexó SU tick— en vez de con la generación de la base.
func TestUnCambioQueIndexoOtroDaemonLlegaAlCentral(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	dir := proyectoGoSinIndexar(t)
	base := memtest.DirSembrado(t)
	abrir := func() *memory.DbEngine {
		eng, err := memory.NewDbEngine(base)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { eng.Close() })
		return eng
	}
	a := NewMcpServer(abrir(), dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	a.SetSyncClient(newTestSyncClient(t, central.urlDelStub), config.SyncConfig{BatchSize: 200})
	central.servidor.Store(a)
	b := NewMcpServer(abrir(), dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))

	a.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("andamio: el primer tick de A indexó y tenía que empujar 1, empujó %d", n)
	}
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conGamma)
	b.reindexCodeGraphOnce(context.Background()) // B indexa Gamma y no federa: no tiene sync
	if plan, _ := a.planearIncremental(context.Background()); !plan.sinCambios() {
		t.Fatal("andamio: A tenía que ver el árbol limpio tras el índice de B")
	}
	for i := 0; i < 3; i++ {
		a.reindexCodeGraphOnce(context.Background())
	}
	if n := central.pushes.Load(); n != 2 {
		t.Errorf("Gamma está en la base y el central no la recibió: pushes=%d tras 3 ticks de A (esperaba 2: uno y sólo uno)", n)
	}
}

// M3 — EL PUSH FALLIDO DE LA TOOL LO REINTENTA EL SCHEDULER.
//
// musubi_codegraph_index indexa y empuja; si ese push falla, la empujada no avanza y el próximo tick
// del scheduler ve generación > empujada. Antes el scheduler no se enteraba (medido: pushes=2,
// esperaba 3). Sabotaje que la pone roja: marcar empujado ANTES de PushGraph.
func TestUnPushFallidoDeLaToolLoReintentaElScheduler(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	dir := proyectoGoSinIndexar(t)
	s := servidorFederado(t, dir, central)
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("andamio: pushes=%d, esperaba 1", n)
	}

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conGamma)
	central.fallarLosPrimeros.Store(1)
	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("andamio: la tool tenía que intentar su push (pushes=%d): %v", n, res)
	}
	for i := 0; i < 3; i++ {
		s.reindexCodeGraphOnce(context.Background())
	}
	if n := central.pushes.Load(); n != 3 {
		t.Errorf("el push de la tool falló y los ticks del scheduler no lo reintentaron UNA vez: pushes=%d (esperaba 3)", n)
	}
}

// HIGIENE — SIN CAMBIOS, UN PUSH CADA 24 H Y NINGUNO ANTES.
//
// Dos daemons pueden cruzar sus fotos en la red y dejar al central con una vieja mientras la base
// dice «empujada». Nada local lo ve, así que se empuja cada 24 h aunque no haya cambios. Y una base
// que nunca escribió el grafo NO empuja por higiene: la foto saldría vacía y un push vacío borra el
// grafo del central. Sabotajes que la ponen roja: sacar la condición de las 24 h, y sacar el
// `Generacion > 0`.
func TestElPushDeHigieneSaleCada24Horas(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, proyectoGoSinIndexar(t), central)
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("andamio: pushes=%d, esperaba 1", n)
	}
	hace := func(d time.Duration) {
		t.Helper()
		if err := s.engine.SetMeta(memory.MetaCodegraphEmpujadoEn, time.Now().Add(-d).UTC().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
	}

	hace(23 * time.Hour)
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("sin cambios y con el último push hace 23 h, el tick empujó (pushes=%d, esperaba 1)", n)
	}
	hace(25 * time.Hour)
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("sin cambios y con el último push hace 25 h, el tick no hizo el push de higiene (pushes=%d, esperaba 2)", n)
	}
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("el push de higiene ya salió y el tick siguiente volvió a empujar (pushes=%d, esperaba 2)", n)
	}

	vacio := nuevoCentralQueCuentaPushes(t)
	sinGrafo := servidorFederado(t, t.TempDir(), vacio) // nada que indexar: generación 0
	sinGrafo.reindexCodeGraphOnce(context.Background())
	if n := vacio.pushes.Load(); n != 0 {
		t.Errorf("una base que nunca escribió el grafo empujó %d foto(s) vacía(s): el central borra el grafo del proyecto", n)
	}
}

// M2 — UNA ESCRITURA DEL GRAFO DURANTE EL PUSH SE FEDERA EN EL TICK SIGUIENTE.
//
// El push del scheduler viaja sin dispatchMu, así que un save_code puede re-derivar un paquete
// mientras la foto vieja está en la red. Con el atomic, el Store(!ok) final pisaba la marca del
// save_code (medido: pushes=1, pendiente=false). Ahora se marca empujada la generación DE LA FOTO.
// Sabotaje que la pone roja: marcar con la generación leída después del push.
func TestUnaEscrituraDuranteElPushSeFederaAlTickSiguiente(t *testing.T) {
	dir := proyectoGoSinIndexar(t)
	var srv atomic.Pointer[McpServer]
	var pushes atomic.Int64
	var saveErr atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pushes.Add(1) == 1 {
			writeFile(t, filepath.Join(dir, "pkg", "a.go"), conGamma)
			if _, rpcErr := call(t, srv.Load(), "musubi_save_code", map[string]interface{}{"path": "pkg/a.go", "gist": "con Gamma"}); rpcErr != nil {
				saveErr.Store(rpcErr.Message)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	t.Cleanup(ts.Close)
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})
	srv.Store(s)

	s.reindexCodeGraphOnce(context.Background()) // indexa + push, con el save_code en vuelo
	if v := saveErr.Load(); v != nil {
		t.Fatalf("andamio: save_code falló: %v", v)
	}
	if plan, _ := s.planearIncremental(context.Background()); !plan.sinCambios() {
		t.Fatal("andamio: save_code tenía que dejar el árbol limpio")
	}
	for i := 0; i < 3; i++ {
		s.reindexCodeGraphOnce(context.Background())
	}
	if n := pushes.Load(); n != 2 {
		t.Errorf("save_code cambió el grafo durante el push y ningún tick lo federó UNA vez: pushes=%d (esperaba 2)", n)
	}
}

// pushMu — LOS PUSH DE LA TOOL Y DEL SCHEDULER NO SE CRUZAN.
//
// El protocolo es de reemplazo: si dos push viajan a la vez, el que llega último gana aunque traiga
// la foto más vieja. El stub retiene el primer push (el del scheduler) y mira si llega un segundo
// (el de la tool) antes de soltarlo. Sabotaje que la pone roja: sacar el pushMu.Lock de
// pushCodeGraphToCentral.
func TestLosPushDeLaToolYDelSchedulerNoSeCruzan(t *testing.T) {
	dir := proyectoGoSinIndexar(t)
	var llegados atomic.Int64
	primero, soltar := make(chan struct{}), make(chan struct{})
	var unaVez sync.Once
	liberar := func() { unaVez.Do(func() { close(soltar) }) }
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if llegados.Add(1) == 1 {
			close(primero)
			<-soltar
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	t.Cleanup(ts.Close)
	t.Cleanup(liberar) // corre ANTES que ts.Close: un handler retenido no cuelga el cierre
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})

	finScheduler, finTool := make(chan struct{}), make(chan struct{})
	go func() { defer close(finScheduler); s.reindexCodeGraphOnce(context.Background()) }()
	select {
	case <-primero:
	case <-time.After(10 * time.Second):
		t.Fatal("andamio: el push del scheduler no llegó al central en 10 s")
	}
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conGamma)
	go func() {
		defer close(finTool)
		_, _ = call(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	}()

	time.Sleep(500 * time.Millisecond) // de sobra para indexar un paquete y salir a la red
	enVuelo := llegados.Load()
	liberar()
	for _, fin := range []chan struct{}{finScheduler, finTool} {
		select {
		case <-fin:
		case <-time.After(10 * time.Second):
			t.Fatal("un push no terminó 10 s después de soltar el central")
		}
	}
	if enVuelo != 1 {
		t.Errorf("con el push del scheduler en vuelo, el de la tool salió igual (%d push a la vez): el central se queda con el que llegue último", enVuelo)
	}
	if n := llegados.Load(); n != 2 {
		t.Errorf("después de soltar, la tool tenía que empujar: llegaron %d push, esperaba 2", n)
	}
}

// M4 — UNA FOTO QUE FALLA NO MANDA NADA AL CENTRAL.
//
// El protocolo es de reemplazo: listas vacías no dicen «no pude leer» sino «borrá todo lo mío». Un
// contexto cancelado hace fallar el BeginTx de la foto de forma determinista. Sabotaje que la pone
// roja: sacar el `return false` tras el error de FotoDelGrafoCtx (S1).
func TestUnaFotoQueFallaNoMandaNadaAlCentral(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, proyectoGoSinIndexar(t), central)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempted, ok := s.pushCodeGraphToCentral(ctx)
	if n := central.pushes.Load(); n != 0 {
		t.Fatalf("la foto falló y llegaron %d request(s) al central: un push vacío BORRA el grafo del proyecto", n)
	}
	if !attempted || ok {
		t.Errorf("con el gate encendido y la foto rota tenía que ser attempted=true ok=false, fue %v %v", attempted, ok)
	}
	if est, _ := s.engine.EstadoDelPushDelGrafo(); !est.EmpujadoEn.IsZero() {
		t.Errorf("un push que no salió quedó registrado como exitoso: %+v", est)
	}
}
