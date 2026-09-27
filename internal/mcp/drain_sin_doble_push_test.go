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

// capturarClaims guarda lo que devolvió cada claim del drain, para preguntarle a la base por ESE
// reclamo mientras el push todavía viaja.
type capturarClaims struct {
	memory.StorageBackend
	mu    sync.Mutex
	items []memory.OutboxItem
}

func (c *capturarClaims) ClaimOutboxBatch(limit, leaseSeconds int) ([]memory.OutboxItem, error) {
	items, err := c.StorageBackend.ClaimOutboxBatch(limit, leaseSeconds)
	c.mu.Lock()
	c.items = append(c.items, items...)
	c.mu.Unlock()
	return items, err
}

// reclamoDe devuelve el último ítem reclamado de id con ese contenido.
func (c *capturarClaims) reclamoDe(id, contenido string) (memory.OutboxItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.items) - 1; i >= 0; i-- {
		if it := c.items[i]; it.ObsID == id && it.Content == contenido {
			return it, true
		}
	}
	return memory.OutboxItem{}, false
}

// TestUnChoqueTardioNoPisaLaVersionMasNueva: dos máquinas de verdad, M y G, cada una con su base,
// contra un mismo central de juguete:
//
//  1. v0 está en todos lados.
//  2. M edita v1 y la empuja. El central la GUARDA y, antes de contestar, G guarda v3 y la sube: v3
//     llega al central DESPUÉS que v1, es la escritura más nueva.
//  3. Con el push de v1 todavía sin marcar ('claimed'), la bajada de M trae v3: choque.
//  4. Unos ticks más de cada lado.
//
// Gana la última, que es lo que decidió el dueño —esta ola sólo cuenta rebotes y choques—: el
// central y G terminan con v3, y el choque queda contado en la bajada de M. M se queda con v1 hasta
// su próxima edición; es la divergencia conocida que cierra #15, y acá va sólo al log, sin afirmarla.
//
// Mira DOS capas, porque cada una sola deja a v3 en el central. La primera es que el choque no toque
// el outbox: se afirma DURANTE el vuelo, con el reclamo de v1 todavía vigente. La segunda es que la
// marca cierre la fila con el 200 de v1, aunque haya quedado 'pending' con el mismo contenido
// (TestUnaEntregaTardiaCierraLaMismaVersion, internal/memory). Con las dos al revés —la fila devuelta
// a la cola y una marca que sólo cierra una 'claimed'— v1 volvía a subir, pisaba a v3 en el central, G
// bajaba v1, y v3 se perdía en las tres máquinas. Por eso el sabotaje de abajo, que devuelve la fila a
// la cola y nada más, se ve en la afirmación del medio y no en la del final.
//
// Sabotaje: que el choque sobre una reclamada la devuelva a la cola para volver a subir la local.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="\t\t\treturn Ingesta{Rebote: true}, nil\n\t\t}\n"
// arnes: a="\t\t\treturn Ingesta{Rebote: true}, nil\n\t\t}\n\t\tif estado == outboxClaimed {\n\t\t\tif _, err := tx.Exec(`UPDATE outbox SET status = \x27pending\x27, updated_at = datetime(\x27now\x27) WHERE obs_id = ? AND status = \x27claimed\x27`, o.ID); err != nil {\n\t\t\t\treturn Ingesta{}, err\n\t\t\t}\n\t\t\tif err := tx.Commit(); err != nil {\n\t\t\t\treturn Ingesta{}, err\n\t\t\t}\n\t\t}\n"
func TestUnChoqueTardioNoPisaLaVersionMasNueva(t *testing.T) {
	const id = "nota-compartida"
	const v0 = "v0: la nota como estaba en todos lados"
	const v1 = "v1: la edicion de M, que llega primero al central"
	const v3 = "v3: la edicion de G, que llega al central DESPUES que v1"

	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	ctx := context.Background()
	cfg := config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300}
	eM := dosEnginesSobreUnaBase(t, 1)[0]
	eG := dosEnginesSobreUnaBase(t, 1)[0]
	capM := &capturarClaims{StorageBackend: eM}
	m := drainerContra(t, capM, srv.URL, cfg)
	g := drainerContra(t, eG, srv.URL, cfg)

	// (1) v0 en todos lados.
	guardarShared(t, eM, id, v0)
	m.drainOutboxOnce(ctx)
	g.drainInboundOnce(ctx)
	m.drainInboundOnce(ctx)
	if got := contenidoEn(t, eG, id); got != v0 {
		t.Fatalf("precondición: G tenía que bajar v0, tiene %q", got)
	}

	// (2)-(3)
	guardarShared(t, eM, id, v1)
	var disparado atomic.Bool
	var vigente, reclamado bool
	var errVigente error
	central.mu.Lock()
	central.mientrasViaja = func(content string) {
		if content != v1 || !disparado.CompareAndSwap(false, true) {
			return
		}
		guardarShared(t, eG, id, v3)
		g.drainOutboxOnce(ctx)  // v3 llega al central después que v1
		m.drainInboundOnce(ctx) // M baja v3 con v1 todavía reclamada: choque
		var it memory.OutboxItem
		if it, reclamado = capM.reclamoDe(id, v1); reclamado {
			vigente, errVigente = eM.ReclamoVigente(it.ObsID, it.Hash, it.Reclamo)
		}
	}
	central.mu.Unlock()
	m.drainOutboxOnce(ctx)
	central.mu.Lock()
	central.mientrasViaja = nil
	central.mu.Unlock()
	if !disparado.Load() || !reclamado {
		t.Fatal("el push de v1 no pasó por el central: la prueba no reprodujo nada")
	}
	if errVigente != nil || !vigente {
		t.Errorf("EL CHOQUE TOCÓ EL OUTBOX: con v1 en vuelo, el choque le sacó la fila a su drain (vigente=%v, err %v); tenía que contarse y nada más", vigente, errVigente)
	}

	// (4) Unos ticks más de cada lado, hasta que no quede nada por mover.
	for i := 0; i < 3; i++ {
		m.drainOutboxOnce(ctx)
		g.drainOutboxOnce(ctx)
		m.drainInboundOnce(ctx)
		g.drainInboundOnce(ctx)
	}

	central.mu.Lock()
	enCentral := central.notas[id].Content
	central.mu.Unlock()
	enM, enG := contenidoEn(t, eM, id), contenidoEn(t, eG, id)
	if enCentral != v3 || enG != v3 {
		t.Errorf("GANÓ LA PRIMERA: v3 llegó al central después que v1 y quedó pisada: central=%q, G=%q", enCentral, enG)
	}
	r, err := eM.ResumenDelSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.BajadaHoy.Choques < 1 {
		t.Errorf("la bajada de M no contó el choque (choques=%d)", r.BajadaHoy.Choques)
	}
	if pending, sent, dead, err := eM.OutboxStats(); err != nil || pending != 0 || sent != 1 || dead != 0 {
		t.Errorf("outbox de M pending=%d sent=%d dead=%d (err %v); esperaba 0/1/0", pending, sent, dead, err)
	}
	// M se queda con v1 hasta su próxima edición: la divergencia conocida, que cierra #15.
	t.Logf("saves en orden: %q · central=%q · G=%q · M=%q (#15)", guardadosDe(central), enCentral, enG, enM)
}

// contenidoEn lee el contenido que la base de e tiene para id.
func contenidoEn(t *testing.T, e *memory.DbEngine, id string) string {
	t.Helper()
	obs, err := e.GetObservations([]string{id})
	if err != nil || len(obs) != 1 {
		t.Fatalf("leer %s: %v (%d filas)", id, err, len(obs))
	}
	return obs[0].Content
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
