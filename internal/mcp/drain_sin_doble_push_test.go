package mcp

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// EL ENVÍO NO SE DUPLICA NI ESPERA AL TICK (ola 2, frente sync). Medido el 2026-09-25 contra un
// central de juguete con seis daemons por base, como davantis-1: 116 saves por 95 notas, y una nota
// escrita por otro proceso tardaba 22 s de mediana en salir. Las causas y el arreglo están en la doc
// de drainOutboxOnce, subloteDelDrain y RunOutboxScheduler; acá, el drain de verdad contra
// centralConMemoria (edicion_en_vuelo_test.go).

// dosEnginesSobreUnaBase abre n engines sobre la MISMA base, como los daemons y los hooks de una
// máquina.
func dosEnginesSobreUnaBase(t *testing.T, n int) []*memory.DbEngine {
	t.Helper()
	dir := memtest.DirSembrado(t)
	var out []*memory.DbEngine
	for i := 0; i < n; i++ {
		e, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { e.Close() })
		out = append(out, e)
	}
	return out
}

// drainerContra arma un McpServer sobre engine que sube al central de url.
func drainerContra(t *testing.T, engine memory.StorageBackend, url string, cfg config.SyncConfig) *McpServer {
	t.Helper()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, url), cfg)
	return s
}

// guardadosDe devuelve una copia de lo que recibió el central, en orden.
func guardadosDe(c *centralConMemoria) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.guardados...)
}

// guardarShared guarda una observación 'shared' por el camino de siempre.
func guardarShared(t *testing.T, e memory.StorageBackend, id, contenido string) {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/x", contenido, 1, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
}

// TestElSubloteEntraEnSuLease: el drain reclama la décima parte del lease, con piso de una y techo de
// batch_size. Con el default (lease 60 s, batch 50) son 6 filas: diez segundos por push antes de que
// venza el lease, donde el claim de 50 que había antes vencía a mitad de la lista con pushes de 3,5 s.
//
// Sabotaje: reclamar tantas filas como segundos de lease.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="k := lease / 10"
// arnes: a="k := lease"
func TestElSubloteEntraEnSuLease(t *testing.T) {
	for _, c := range []struct {
		cfg  config.SyncConfig
		want int
	}{
		{config.SyncConfig{LeaseSeconds: 60, BatchSize: 50}, 6},
		{config.SyncConfig{}, 6},
		{config.SyncConfig{LeaseSeconds: 1, BatchSize: 50}, 1},
		{config.SyncConfig{LeaseSeconds: 5}, 1},
		{config.SyncConfig{LeaseSeconds: 60, BatchSize: 3}, 3},
		{config.SyncConfig{LeaseSeconds: 600, BatchSize: 200}, 60},
	} {
		if got := subloteDelDrain(c.cfg); got != c.want {
			t.Errorf("subloteDelDrain(lease=%d, batch=%d) = %d; esperaba %d", c.cfg.LeaseSeconds, c.cfg.BatchSize, got, c.want)
		}
	}
}

// TestDosDrainersNoSubenDosVecesLaMisma: dos daemons sobre la misma base drenan a la vez 25 notas
// contra un central que tarda 200 ms por save, con un lease de 1 s. Ninguna nota llega dos veces.
// En main el primero reclamaba las 25 con un solo lease, a los cinco pushes le vencía, el segundo
// reclamaba las veinte de atrás, y los dos las subían.
//
// Sabotaje: el drain de main (un claim de batch_size y sin preguntar antes de empujar).
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="k, vigente := subloteDelDrain(s.syncCfg), s.sigueSiendoMia"
// arnes: a="k, vigente := s.syncCfg.BatchSize, func(memory.OutboxItem) bool { return true }"
func TestDosDrainersNoSubenDosVecesLaMisma(t *testing.T) {
	const n = 25
	engines := dosEnginesSobreUnaBase(t, 2)
	central := newCentralConMemoria()
	central.mientrasViaja = func(string) { time.Sleep(200 * time.Millisecond) }
	srv := httptest.NewServer(central)
	defer srv.Close()
	cfg := config.SyncConfig{BatchSize: 50, LeaseSeconds: 1, BackoffBaseSeconds: 1, BackoffMaxSeconds: 5}
	a := drainerContra(t, engines[0], srv.URL, cfg)
	b := drainerContra(t, engines[1], srv.URL, cfg)
	for i := 0; i < n; i++ {
		guardarShared(t, engines[0], fmt.Sprintf("doble-%02d", i), fmt.Sprintf("la nota %02d", i))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inicio := time.Now()
	var wg sync.WaitGroup
	for _, s := range []*McpServer{a, b} {
		wg.Add(1)
		go func(s *McpServer) {
			defer wg.Done()
			for ctx.Err() == nil {
				s.drainOutboxOnce(ctx)
				if p, _, _, err := engines[0].OutboxStats(); err != nil || p == 0 {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(s)
	}
	wg.Wait()

	guardados := guardadosDe(central)
	veces := map[string]int{}
	for _, c := range guardados {
		veces[c]++
	}
	for c, v := range veces {
		if v > 1 {
			t.Errorf("ENVÍO DOBLE: %q llegó %d veces al central", c, v)
		}
	}
	if len(veces) != n {
		t.Errorf("llegaron %d notas distintas; esperaba las %d", len(veces), n)
	}
	t.Logf("saves=%d para %d notas, en %v con dos drainers", len(guardados), n, time.Since(inicio).Round(time.Millisecond))
}

// contarClaims cuenta los claims del drain sobre un engine de verdad.
type contarClaims struct {
	memory.StorageBackend
	claims atomic.Int32
}

func (c *contarClaims) ClaimOutboxBatch(limit, leaseSeconds int) ([]memory.OutboxItem, error) {
	c.claims.Add(1)
	return c.StorageBackend.ClaimOutboxBatch(limit, leaseSeconds)
}

// TestUnaNotaDeOtroProcesoNoEsperaAlTick: con un tick de una hora, una nota que guarda OTRO proceso
// sobre la misma base llega al central en menos de 5 s, por el sondeo. Y mientras no hay nada por
// subir, el sondeo no reclama: un claim es una escritura, y con seis daemons por base serían tres por
// segundo sobre una base que no tiene nada que mandar.
//
// Sabotaje: sin sondeo (que la nota espere al tick, como antes).
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="const sondeoDelOutbox = 2 * time.Second"
// arnes: a="const sondeoDelOutbox = time.Hour"
//
// Sabotaje: drenar en cada sondeo aunque no haya nada por subir.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\t\tif hay {\n\t\t\t\ts.drainOutboxOnce(ctx)"
// arnes: a="\t\t\tif hay || true {\n\t\t\t\ts.drainOutboxOnce(ctx)"
func TestUnaNotaDeOtroProcesoNoEsperaAlTick(t *testing.T) {
	engines := dosEnginesSobreUnaBase(t, 2)
	daemon, hook := engines[0], engines[1]
	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	contado := &contarClaims{StorageBackend: daemon}
	s := drainerContra(t, contado, srv.URL, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, DrainIntervalSeconds: 3600,
		BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})

	ctx, cancel := context.WithCancel(context.Background())
	termino := make(chan struct{})
	go func() {
		s.RunOutboxScheduler(ctx, time.Hour)
		close(termino)
	}()
	defer func() {
		cancel()
		<-termino
	}()

	time.Sleep(2600 * time.Millisecond) // pasa al menos un sondeo sin nada por subir
	if c := contado.claims.Load(); c != 0 {
		t.Errorf("con nada por subir el sondeo reclamó %d veces; esperaba ninguna", c)
	}

	guardarShared(t, hook, "otro-proceso", "la nota que guardo el hook")
	guardada := time.Now()
	for time.Since(guardada) < 5*time.Second && len(guardadosDe(central)) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	demora := time.Since(guardada)
	if g := guardadosDe(central); len(g) != 1 || g[0] != "la nota que guardo el hook" {
		t.Fatalf("NOTA ESPERANDO AL TICK: a los %v el central tenía %q; esperaba la nota del hook", demora.Round(time.Millisecond), g)
	}
	t.Logf("la nota del otro proceso llegó al central en %v", demora.Round(time.Millisecond))
}

// TestUnPayloadViejoNoLlegaDespuesDelNuevo: A reclama dos notas en el mismo sublote. Mientras empuja
// la primera, otro proceso edita la segunda, y el daemon B reclama la edición y la sube. Cuando A
// llega a la segunda tiene en la mano la versión vieja: no la sube. En main la subía, y el central se
// quedaba con la versión vieja DESPUÉS de la nueva, la fila local 'sent' por la nueva, y la bajada
// siguiente pisaba la nueva también acá.
//
// Sabotaje: empujar sin preguntar si la fila sigue siendo de este claim.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if !vigente(item) {"
// arnes: a="if false && !vigente(item) {"
func TestUnPayloadViejoNoLlegaDespuesDelNuevo(t *testing.T) {
	engines := dosEnginesSobreUnaBase(t, 3)
	ea, eb, hook := engines[0], engines[1], engines[2]
	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	// Lease de 60 s: sublote de 6, así que A se lleva las dos notas en el mismo claim.
	cfg := config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300}
	a := drainerContra(t, ea, srv.URL, cfg)
	b := drainerContra(t, eb, srv.URL, cfg)
	ctx := context.Background()

	viejas := map[string]string{"orden-1": "la nota uno tal como la reclamo A", "orden-2": "la nota dos tal como la reclamo A"}
	for id, c := range viejas {
		guardarShared(t, hook, id, c)
	}
	editada := func(id string) string { return id + " editada mientras A empujaba la otra" }

	// Dispara UNA vez, y sin bloquear a los que llegan después: el save de B entra por este mismo
	// handler mientras el de A todavía está adentro, y un sync.Once lo dejaría esperando al de A,
	// que espera a B.
	var disparado atomic.Bool
	var otra string
	central.mu.Lock()
	central.mientrasViaja = func(content string) {
		if disparado.CompareAndSwap(false, true) {
			for id, c := range viejas {
				if c != content {
					otra = id
				}
			}
			if err := hook.SaveObservationTyped(otra, "t/x", editada(otra), 1, "semantic", memory.ScopeShared, nil); err != nil {
				t.Errorf("la edición en vuelo no se pudo guardar: %v", err)
			}
			b.drainOutboxOnce(ctx) // B reclama la edición y la sube mientras A tiene la vieja en la mano
		}
	}
	central.mu.Unlock()

	a.drainOutboxOnce(ctx)

	central.mu.Lock()
	enElCentral := central.notas[otra].Content
	central.mu.Unlock()
	guardados := guardadosDe(central)
	if enElCentral != editada(otra) {
		t.Errorf("V1 DESPUÉS DE V2: el central terminó con %q para %s; esperaba la edición. Saves en orden: %q", enElCentral, otra, guardados)
	}
	for _, c := range guardados {
		if c == viejas[otra] {
			t.Errorf("la versión vieja de %s llegó al central (saves en orden: %q)", otra, guardados)
		}
	}
	if pending, sent, dead, err := ea.OutboxStats(); err != nil || pending != 0 || sent != 2 || dead != 0 {
		t.Errorf("outbox pending=%d sent=%d dead=%d (err %v); esperaba 0/2/0", pending, sent, dead, err)
	}
	t.Logf("saves en orden: %q", guardados)
}

// TestUnChoqueSobreLaReclamadaVuelveASubirLaLocal: el push de la versión de esta máquina está en
// vuelo y la bajada trae una versión que gio guardó en el central DESPUÉS. La conservación deja acá la
// local (choque); el 200 del push no cierra la fila, y la local vuelve a subir en el drain siguiente:
// el central y esta máquina terminan con la misma, la última. En main la fila quedaba 'sent' y cada
// lado con la suya, sin push ni pull que lo arreglara.
//
// El choque que devuelve la fila a la cola ya lo custodia TestUnaReclamadaNoLaPisaElPull (internal/
// memory) con ese mismo corte; lo que ésta mira además es CUÁNDO vuelve a subir: el 200 del push en
// vuelo suelta el lease que el choque conservó, y la local sale en el drain siguiente, no dentro de
// un minuto (lease_seconds=60).
//
// Sabotaje: que el 200 sobre la fila devuelta a la cola no suelte el lease.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="next_attempt_at = CASE WHEN status = 'pending' AND ? = `+hashActual+` THEN `+ahoraMs+` ELSE next_attempt_at END,"
// arnes: a="next_attempt_at = CASE WHEN 0 AND ? = `+hashActual+` THEN `+ahoraMs+` ELSE next_attempt_at END,"
func TestUnChoqueSobreLaReclamadaVuelveASubirLaLocal(t *testing.T) {
	const mia, deGio = "la version de esta maquina", "la version que gio guardo despues en el central"
	daemon := dosEnginesSobreUnaBase(t, 1)[0]
	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	s := drainerContra(t, daemon, srv.URL, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	ctx := context.Background()
	guardarShared(t, daemon, "choque-1", mia)

	var disparado atomic.Bool
	central.mu.Lock()
	central.mientrasViaja = func(content string) {
		if content != mia || !disparado.CompareAndSwap(false, true) {
			return
		}
		func() {
			central.mu.Lock()
			central.seq++
			nueva := central.notas["choque-1"]
			nueva.RowID, nueva.Content, nueva.Author = central.seq, deGio, "gio"
			central.notas["choque-1"] = nueva
			central.mu.Unlock()
			s.drainInboundOnce(ctx) // baja la de gio con el push de la mía en vuelo
		}()
	}
	central.mu.Unlock()

	s.drainOutboxOnce(ctx) // sube la mía; en vuelo, choque con la de gio
	s.drainOutboxOnce(ctx) // la mía vuelve a subir

	central.mu.Lock()
	enElCentral := central.notas["choque-1"].Content
	central.mu.Unlock()
	obs, err := daemon.GetObservations([]string{"choque-1"})
	if err != nil || len(obs) != 1 {
		t.Fatalf("leer la nota local: %v (%d filas)", err, len(obs))
	}
	if enElCentral != obs[0].Content {
		t.Errorf("DIVERGENCIA: el central quedó con %q y esta máquina con %q", enElCentral, obs[0].Content)
	}
	if obs[0].Content != mia {
		t.Errorf("la conservación tenía que dejar acá la local, quedó %q", obs[0].Content)
	}
	if pending, sent, dead, err := daemon.OutboxStats(); err != nil || pending != 0 || sent != 1 || dead != 0 {
		t.Errorf("outbox pending=%d sent=%d dead=%d (err %v); esperaba 0/1/0", pending, sent, dead, err)
	}
	t.Logf("saves en orden: %q · central=%q · local=%q", guardadosDe(central), enElCentral, obs[0].Content)
}

// contarViajes cuenta los viajes de subida que el drain registra.
type contarViajes struct {
	memory.StorageBackend
	subidas atomic.Int32
}

func (c *contarViajes) RegistrarViaje(sentido string, v memory.Viaje) error {
	if sentido == memory.ViajeSubida {
		c.subidas.Add(1)
	}
	return c.StorageBackend.RegistrarViaje(sentido, v)
}

// TestUnaRafagaSaleEnteraEnUnSoloViaje: 95 notas —la ráfaga medida— salen enteras en un solo drain,
// en sublotes de 6, cada una una vez, y el drain deja UN viaje de subida en sync_viajes, no uno por
// sublote: cada viaje es una escritura sobre una base que comparten varios procesos.
//
// Sabotaje: soltar el drain después del primer sublote.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if len(items) < k || ctx.Err() != nil || !time.Now().Before(limite) {"
// arnes: a="if len(items) < k || ctx.Err() != nil || !time.Now().Before(limite) || true {"
//
// Sabotaje: registrar un viaje por sublote.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="aceptadas += s.empujarSublote(ctx, items, vigente)"
// arnes: a="aceptadas += s.empujarSublote(ctx, items, vigente)\n\t\t_ = s.engine.RegistrarViaje(memory.ViajeSubida, s.syncClient.subidaDesde(antes))"
func TestUnaRafagaSaleEnteraEnUnSoloViaje(t *testing.T) {
	const n = 95
	e := dosEnginesSobreUnaBase(t, 1)[0]
	contado := &contarViajes{StorageBackend: e}
	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	s := drainerContra(t, contado, srv.URL, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	for i := 0; i < n; i++ {
		guardarShared(t, e, fmt.Sprintf("rafaga-%02d", i), fmt.Sprintf("la nota %02d de la rafaga", i))
	}

	s.drainOutboxOnce(context.Background())

	guardados := guardadosDe(central)
	distintas := map[string]bool{}
	for _, c := range guardados {
		distintas[c] = true
	}
	if len(guardados) != n || len(distintas) != n {
		t.Errorf("un drain subió %d saves (%d distintos); esperaba las %d notas, una vez cada una", len(guardados), len(distintas), n)
	}
	if pending, sent, dead, err := e.OutboxStats(); err != nil || pending != 0 || sent != n || dead != 0 {
		t.Errorf("outbox pending=%d sent=%d dead=%d (err %v); esperaba 0/%d/0", pending, sent, dead, err, n)
	}
	if v := contado.subidas.Load(); v != 1 {
		t.Errorf("el drain registró %d viajes de subida; esperaba uno solo con todos sus sublotes", v)
	}
	r, err := e.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.SubidaHoy.Filas != n {
		t.Errorf("el viaje de subida anotó %d filas; esperaba %d", r.SubidaHoy.Filas, n)
	}
}

// TestElDrainNoPasaDelTick: con un tick de 1 s el drain deja de reclamar sublotes al 80 % del tick,
// aunque quede cola: lo que sigue lo toma el sondeo o el tick siguiente. Sin tope, una cola larga
// con un central lento dejaba al drain empujando indefinidamente.
//
// Sabotaje: sin tope de tiempo.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="limite := time.Now().Add(tiempoDelTick(s.syncCfg))"
// arnes: a="limite := time.Now().Add(time.Hour)"
func TestElDrainNoPasaDelTick(t *testing.T) {
	const n = 30
	e := dosEnginesSobreUnaBase(t, 1)[0]
	central := newCentralConMemoria()
	central.mientrasViaja = func(string) { time.Sleep(100 * time.Millisecond) }
	srv := httptest.NewServer(central)
	defer srv.Close()
	// Lease de 10 s: sublote de una, así que el tope se mira después de cada push.
	s := drainerContra(t, e, srv.URL, config.SyncConfig{BatchSize: 50, LeaseSeconds: 10, DrainIntervalSeconds: 1,
		BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	for i := 0; i < n; i++ {
		guardarShared(t, e, fmt.Sprintf("tope-%02d", i), fmt.Sprintf("la nota %02d", i))
	}

	inicio := time.Now()
	s.drainOutboxOnce(context.Background())
	duro := time.Since(inicio)

	subidas := len(guardadosDe(central))
	if subidas == 0 || subidas >= n {
		t.Errorf("EL DRAIN NO SOLTÓ EL TICK: subió %d de %d en %v con un tick de 1 s; esperaba que parara antes de vaciar la cola", subidas, n, duro.Round(time.Millisecond))
	}
	if duro > 2500*time.Millisecond {
		t.Errorf("el drain tardó %v con un tick de 1 s", duro.Round(time.Millisecond))
	}
	t.Logf("subió %d de %d en %v", subidas, n, duro.Round(time.Millisecond))
}
