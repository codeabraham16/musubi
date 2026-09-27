package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// EL RITMO DE LA BAJADA (bajada_ritmo.go): una máquina quieta deja de preguntarle al central en cada
// tick, y una en la que alguien trabaja vuelve a preguntar en cada tick.
//
// Todas estas pruebas cuentan los pedidos que LLEGAN al central, que es el costo que el ritmo viene a
// bajar, y no una variable interna: un corte que el ritmo decide y que no frena el Pull pasaría
// cualquier aserción sobre el ritmo y fallaría éstas. Ninguna duerme: el ritmo cuenta ticks, así que
// un tick es una llamada a drainInboundOnce.

// centralQueCuenta es centralConFilas contando los pedidos que llegan.
func centralQueCuenta(t *testing.T, pedidos, filas *atomic.Int64) *httptest.Server {
	t.Helper()
	cuerpo := centralConFilas(filas)
	t.Cleanup(cuerpo.Close)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		cuerpo.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// daemonConRitmo arma un daemon en team_mode sobre `eng` contra el central `url` con `cfg`, y le fija
// el azar del ritmo en `azar` para que la secuencia de pedidos sea exacta. Una cfg sin
// InboundIdleMaxSeconds ni DrainIntervalSeconds es la de todos los configs que existen: tope de
// 300 s y ticks de 30 s, o sea a lo sumo 10 ticks entre dos pedidos.
func daemonConRitmo(t *testing.T, eng memory.StorageBackend, url string, cfg config.SyncConfig, azar int) *McpServer {
	t.Helper()
	s := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, url), cfg)
	s.ritmoBajada.azar = func(int) int { return azar }
	return s
}

// dosEnginesSobreLaMismaBase son dos procesos sobre una base: el daemon del dueño del candado y la
// terminal donde alguien trabaja, que casi nunca son el mismo.
func dosEnginesSobreLaMismaBase(t *testing.T) (*memory.DbEngine, *memory.DbEngine) {
	t.Helper()
	dir := memtest.DirSembrado(t)
	abrir := func() *memory.DbEngine {
		e, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { e.Close() })
		return e
	}
	return abrir(), abrir()
}

// ticksQueSalen corre los ticks desde..hasta (desde 1) y devuelve en cuáles llegó un pedido al
// central. `antes`, si no es nil, corre al principio de cada tick: es donde una prueba escribe la
// marca de actividad.
func ticksQueSalen(s *McpServer, pedidos *atomic.Int64, desde, hasta int, antes func(tick int)) []int {
	var salieron []int
	for tick := desde; tick <= hasta; tick++ {
		if antes != nil {
			antes(tick)
		}
		p := pedidos.Load()
		s.drainInboundOnce(context.Background())
		if pedidos.Load() > p {
			salieron = append(salieron, tick)
		}
	}
	return salieron
}

// marcar escribe la marca de actividad como la escribe el hook del turno.
func marcar(t *testing.T, eng memory.StorageBackend, unix int64) {
	t.Helper()
	if err := eng.SetMeta(memory.MetaDespertarBajada, strconv.FormatInt(unix, 10)); err != nil {
		t.Fatal(err)
	}
}

// TestBajadaVaciaEspaciaLosPedidos: con el central sin nada nuevo y nadie trabajando, el dueño del
// candado espacia los pedidos —2, 4, 8 ticks y de ahí el tope, 10 ticks de 30 s: 300 s— en vez de
// pedir en cada tick. En 40 ticks llegan 6 pedidos, no 40. Con el azar en 0 la secuencia es exacta;
// con el azar en 1 se corre un tick desde la SEGUNDA vacía, nunca en la primera, y nunca pasa el tope.
// El daemon sale de una config sin la clave del tope: el default espacia.
//
// Sabotaje: que las vacías no se cuenten y la bajada no espacie nunca.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="\tr.vacias++\n"
// arnes: a="\tr.vacias = 0\n"
//
// Sabotaje: sin tope, el espaciado sigue duplicando (16, 32… ticks).
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="saltear := min(n, topeTicks-1)"
// arnes: a="saltear := n + 0*topeTicks"
//
// Sabotaje: el azar también en el primer escalón.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="if r.vacias >= 2 {"
// arnes: a="if r.vacias >= 1 {"
//
// Sabotaje: el corte del drain desconectado: el ritmo decide y el Pull sale igual.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if !s.ritmoBajada.tocaIr("
// arnes: a="if false && !s.ritmoBajada.tocaIr("
//
// Sabotaje: una config sin la clave no espacia (el default deja de ser 300 s).
// arnes: archivo="internal/config/config.go"
// arnes: de="\t\treturn 300\n"
// arnes: a="\t\treturn 0\n"
func TestBajadaVaciaEspaciaLosPedidos(t *testing.T) {
	for _, c := range []struct {
		azar   int
		quiero []int
	}{
		{0, []int{1, 3, 7, 15, 25, 35}}, // esperas de 2, 4, 8, 10, 10 ticks
		{1, []int{1, 3, 8, 17, 27, 37}}, // esperas de 2, 5, 9, 10, 10 ticks
	} {
		t.Run("azar "+strconv.Itoa(c.azar), func(t *testing.T) {
			var pedidos, filas atomic.Int64
			central := centralQueCuenta(t, &pedidos, &filas)
			s := daemonConRitmo(t, memtest.NuevoEngine(t, t.TempDir()), central.URL, config.SyncConfig{BatchSize: 50}, c.azar)
			got := ticksQueSalen(s, &pedidos, 1, 40, nil)
			if !slices.Equal(got, c.quiero) {
				t.Errorf("40 ticks con el central vacío y sin actividad: los pedidos llegaron en los ticks %v; esperaba %v", got, c.quiero)
			}
			t.Logf("azar %d: %d pedidos en 40 ticks, en %v", c.azar, len(got), got)
		})
	}
}

// TestElRitmoBaseNoSaltaTicks: con filas en cada bajada, o con alguien trabajando en cada tick, el
// dueño pide en TODOS los ticks: el ritmo base es exactamente el tick, sin saltos ni azar. Es lo que
// rompía el primer diseño, el que comparaba contra el reloj: con actividad quedaba en 60 s y no en 30.
//
// Sabotaje: un tick salteado aunque no quede nada que saltear.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="if r.saltear > 0 {"
// arnes: a="if r.saltear >= 0 {"
//
// Sabotaje: que una bajada con filas no devuelva el ritmo al tick.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="if conFilas > 0 {"
// arnes: a="if conFilas > 0 && false {"
func TestElRitmoBaseNoSaltaTicks(t *testing.T) {
	todos := make([]int, 12)
	for i := range todos {
		todos[i] = i + 1
	}

	t.Run("con filas en cada bajada", func(t *testing.T) {
		var pedidos, filas atomic.Int64
		filas.Store(1)
		central := centralQueCuenta(t, &pedidos, &filas)
		s := daemonConRitmo(t, memtest.NuevoEngine(t, t.TempDir()), central.URL, config.SyncConfig{BatchSize: 50}, 1)
		if got := ticksQueSalen(s, &pedidos, 1, 12, nil); !slices.Equal(got, todos) {
			t.Errorf("con filas en cada bajada tenía que pedir en los 12 ticks, pidió en %v", got)
		}
	})

	t.Run("con un turno en cada tick", func(t *testing.T) {
		var pedidos, filas atomic.Int64
		central := centralQueCuenta(t, &pedidos, &filas)
		dueno, terminal := dosEnginesSobreLaMismaBase(t)
		s := daemonConRitmo(t, dueno, central.URL, config.SyncConfig{BatchSize: 50}, 1)
		got := ticksQueSalen(s, &pedidos, 1, 12, func(tick int) { marcar(t, terminal, int64(1_800_000_000+30*tick)) })
		if !slices.Equal(got, todos) {
			t.Errorf("con un turno en cada tick tenía que pedir en los 12 ticks aunque el central esté vacío, pidió en %v", got)
		}
	})
}

// TestElDespertarReiniciaElEspaciado: una marca de actividad no sólo corta la espera del momento:
// devuelve el ritmo al principio. Si sólo cortara la espera, la primera vacía después del turno
// volvería directo al tope, y «vuelve a los 30 s en cuanto alguien trabaja» sería un pedido suelto.
// Con la bajada en el tope, un turno: el dueño pide en el tick siguiente, y la vacía que vuelve
// espacia la próxima a 2 ticks, no a 10.
//
// Sabotaje: que el despertar corte la espera y deje las vacías acumuladas.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="r.vacias, r.saltear = 0, 0 // el despertar reinicia el espaciado, no sólo la espera"
// arnes: a="r.saltear = 0 // el despertar reinicia el espaciado, no sólo la espera"
func TestElDespertarReiniciaElEspaciado(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	dueno, terminal := dosEnginesSobreLaMismaBase(t)
	s := daemonConRitmo(t, dueno, central.URL, config.SyncConfig{BatchSize: 50}, 0)

	if got := ticksQueSalen(s, &pedidos, 1, 30, nil); !slices.Equal(got, []int{1, 3, 7, 15, 25}) {
		t.Fatalf("precondición: 30 ticks quietos tenían que dejar la bajada en el tope, pidió en %v", got)
	}
	got := ticksQueSalen(s, &pedidos, 31, 40, func(tick int) {
		if tick == 31 {
			marcar(t, terminal, 1_800_000_000)
		}
	})
	if quiero := []int{31, 33, 37}; !slices.Equal(got, quiero) {
		t.Errorf("con un turno en el tick 31 y el central vacío, los pedidos tenían que llegar en %v (el ritmo vuelve a empezar), llegaron en %v", quiero, got)
	}
}

// TestElTurnoDespiertaLaBajada: con la bajada en el tope, un turno en OTRA terminal —otro proceso
// sobre la misma base, que es lo de siempre: el dueño del candado casi nunca es el daemon de la
// terminal donde se trabaja— escribe la marca, como lo hace el hook (cmd/musubi/bajada_marca.go), y
// el dueño pide en su tick siguiente, no a los 5 min. Una vez por marca: la misma marca leída en el
// tick siguiente ya está vista y no despierta de nuevo.
//
// Sabotaje: que el ritmo no escuche el despertar.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="\tif despierto {\n"
// arnes: a="\tif despierto && false {\n"
//
// Sabotaje: que el dueño no lea la marca.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="return s.ritmoBajada.marcaNueva(strings.TrimSpace(raw))"
// arnes: a="return false && s.ritmoBajada.marcaNueva(strings.TrimSpace(raw))"
//
// Sabotaje: que la marca no quede vista y despierte en cada tick.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="\tr.visto = v\n"
// arnes: a="\t_ = v\n"
func TestElTurnoDespiertaLaBajada(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	dueno, terminal := dosEnginesSobreLaMismaBase(t)
	s := daemonConRitmo(t, dueno, central.URL, config.SyncConfig{BatchSize: 50}, 0)

	if got := ticksQueSalen(s, &pedidos, 1, 16, nil); !slices.Equal(got, []int{1, 3, 7, 15}) {
		t.Fatalf("precondición: la bajada tenía que quedar espaciada al tope, pidió en %v", got)
	}
	marcar(t, terminal, time.Now().Unix())
	if got := ticksQueSalen(s, &pedidos, 17, 18, nil); !slices.Equal(got, []int{17}) {
		t.Errorf("un turno en otra terminal tenía que despertar al dueño en su tick siguiente (el 17) y una sola vez; pidió en %v", got)
	}
}

// TestUnErrorAlLeerLaMarcaNoDespiertaLaBajada: si la marca de actividad no se puede leer, el dueño
// sigue espaciando. Despertar ante cada error haría que una base con problemas bajara al ritmo base
// para siempre sin que nada lo diga; el peor caso de no despertar es esperar hasta el tope, que es
// el mismo que sin marca.
//
// Sabotaje: que un error al leer la marca cuente como actividad.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="if err != nil || !ok {\n\t\treturn false\n"
// arnes: a="if err != nil {\n\t\treturn true\n\t}\n\tif !ok {\n\t\treturn false\n"
func TestUnErrorAlLeerLaMarcaNoDespiertaLaBajada(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	eng := engineConMarcaIlegible{memtest.NuevoEngine(t, t.TempDir())}
	s := daemonConRitmo(t, eng, central.URL, config.SyncConfig{BatchSize: 50}, 0)
	if got := ticksQueSalen(s, &pedidos, 1, 8, nil); !slices.Equal(got, []int{1, 3, 7}) {
		t.Errorf("con la marca ilegible la bajada tenía que espaciar igual (ticks 1, 3 y 7), pidió en %v", got)
	}
}

// engineConMarcaIlegible falla al leer la marca de actividad, y sólo esa clave.
type engineConMarcaIlegible struct{ memory.StorageBackend }

func (e engineConMarcaIlegible) GetMeta(key string) (string, bool, error) {
	if key == memory.MetaDespertarBajada {
		return "", false, errors.New("la marca no se puede leer, a propósito")
	}
	return e.StorageBackend.GetMeta(key)
}

// TestElDuenoRenuevaElCandadoAunqueNoSalgaALaRed: el corte del ritmo va DESPUÉS de reclamar. Un tick
// que el ritmo saltea igual renueva el candado: si no lo renovara, el lease vencería durante la
// espera —hasta 300 s contra un lease de 120—, la otra terminal lo tomaría y las dos volverían a
// alternarse bajando lo mismo, que es lo que el candado vino a cerrar. La prueba no espera a que
// venza: lo deja vencido a mano, que es como queda después de dos minutos de ticks sin renovar.
//
// Sabotaje: que el corte del ritmo vaya antes de reclamar el candado.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if hasta := s.bajadaCedidaHasta.Load(); hasta > 0 && time.Now().UnixNano() < hasta {"
// arnes: a="if hasta := s.bajadaCedidaHasta.Load(); !s.ritmoBajada.tocaIr(s.hayActividadLocal()) || hasta > 0 && time.Now().UnixNano() < hasta {"
func TestElDuenoRenuevaElCandadoAunqueNoSalgaALaRed(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	engA, engB := dosEnginesSobreLaMismaBase(t)
	a := daemonConRitmo(t, engA, central.URL, config.SyncConfig{BatchSize: 50}, 0)
	b := daemonConRitmo(t, engB, central.URL, config.SyncConfig{BatchSize: 50}, 0)

	a.drainInboundOnce(context.Background()) // A toma el candado y baja vacío: el tick siguiente se saltea
	if pedidos.Load() != 1 {
		t.Fatalf("precondición: A tenía que bajar una vez, hubo %d pedidos", pedidos.Load())
	}
	const candado = "sync:inbound_lease" // memory.metaBajadaLease
	if err := engB.SetMeta(candado, a.duenoBajada+"|2000-01-01 00:00:00"); err != nil {
		t.Fatal(err)
	}

	a.drainInboundOnce(context.Background()) // el tick que el ritmo saltea
	if got := pedidos.Load(); got != 1 {
		t.Fatalf("precondición: el tick siguiente a una vacía no sale a la red, y hubo %d pedidos", got)
	}
	v, _, err := engB.GetMeta(candado)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v, a.duenoBajada+"|") || strings.HasSuffix(v, "|2000-01-01 00:00:00") {
		t.Errorf("el tick salteado tenía que renovar el candado de A, y quedó %q", v)
	}

	b.drainInboundOnce(context.Background())
	if got := pedidos.Load(); got != 1 {
		t.Errorf("la otra terminal tomó el candado mientras A esperaba y bajó lo mismo (%d pedidos)", got)
	}
}

// TestUnDuenoNuevoNoHeredaElEspaciado: un proceso que pierde el candado sin cerrarse —la máquina
// durmió y el lease venció, o un tick duró más que el lease— y lo recupera más tarde, cuando el otro
// cierra, baja en ese mismo tick y con el ritmo desde el principio. Sin el reinicio seguía salteando
// los ticks que le quedaban de cuando era dueño, congelados mientras no lo fue: hasta el tope entero
// después del cambio de dueño, sumado a lo que ya se había esperado con el anterior.
//
// Sabotaje: que el proceso sin el candado no reinicie su ritmo.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\ts.ritmoBajada.cedido()\n"
// arnes: a="\t\t_ = s.ritmoBajada\n"
//
// Sabotaje: que el reinicio corte la espera y deje las vacías acumuladas.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="\tr.vacias, r.saltear = 0, 0\n}\n"
// arnes: a="\tr.saltear = 0\n}\n"
func TestUnDuenoNuevoNoHeredaElEspaciado(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	engA, engB := dosEnginesSobreLaMismaBase(t)
	a := daemonConRitmo(t, engA, central.URL, config.SyncConfig{BatchSize: 50}, 0)
	b := daemonConRitmo(t, engB, central.URL, config.SyncConfig{BatchSize: 50}, 0)
	ctx := context.Background()

	if got := ticksQueSalen(b, &pedidos, 1, 16, nil); !slices.Equal(got, []int{1, 3, 7, 15}) {
		t.Fatalf("precondición: B tenía que quedar espaciado al tope, pidió en %v", got)
	}
	// La máquina durmió: el lease de B quedó vencido y A, que tickea primero al despertar, lo toma.
	const candado = "sync:inbound_lease" // memory.metaBajadaLease
	if err := engA.SetMeta(candado, b.duenoBajada+"|2000-01-01 00:00:00"); err != nil {
		t.Fatal(err)
	}
	a.drainInboundOnce(ctx)
	if got := pedidos.Load(); got != 5 {
		t.Fatalf("precondición: A tenía que tomar el candado vencido y bajar (5 pedidos), hubo %d", got)
	}
	b.drainInboundOnce(ctx) // B, sin el candado, no baja
	if got := pedidos.Load(); got != 5 {
		t.Fatalf("precondición: B no tiene el candado y bajó igual (%d pedidos)", got)
	}
	a.SoltarBajada() // A cierra

	if got := ticksQueSalen(b, &pedidos, 1, 8, nil); !slices.Equal(got, []int{1, 3, 7}) {
		t.Errorf("B recuperó el candado: tenía que bajar en ese mismo tick y con el ritmo desde el principio (ticks 1, 3 y 7), pidió en %v", got)
	}
}

// TestLaBajadaArrancaSinEsperarElPrimerTick: al arrancar, el daemon baja UNA vez antes del primer
// tick —con 30 s de tick, una terminal recién abierta esperaba medio minuto con lo de la sesión
// anterior— y anota la marca de actividad: si el dueño del candado es otro, lo despierta.
//
// Sabotaje: sin el pull del arranque, la primera bajada espera un tick entero.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\ts.drainInboundOnce(ctx) // pull al arrancar\n"
// arnes: a="\t// pull al arrancar\n"
//
// Sabotaje: sin la marca del arranque, un daemon nuevo no despierta al dueño espaciado.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if err := s.engine.SetMeta(memory.MetaDespertarBajada, strconv.FormatInt(time.Now().Unix(), 10)); err != nil {"
// arnes: a="if err := error(nil); err != nil {"
func TestLaBajadaArrancaSinEsperarElPrimerTick(t *testing.T) {
	t.Run("baja antes del primer tick", func(t *testing.T) {
		llego := make(chan struct{}, 1)
		var filas atomic.Int64
		cuerpo := centralConFilas(&filas)
		defer cuerpo.Close()
		central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cuerpo.Config.Handler.ServeHTTP(w, r)
			select {
			case llego <- struct{}{}:
			default:
			}
		}))
		defer central.Close()
		s := daemonConRitmo(t, memtest.NuevoEngine(t, t.TempDir()), central.URL, config.SyncConfig{BatchSize: 50}, 0)

		ctx, cancel := context.WithCancel(context.Background())
		termino := make(chan struct{})
		go func() { s.RunInboundScheduler(ctx, time.Hour); close(termino) }()
		select {
		case <-llego:
		case <-time.After(10 * time.Second):
			t.Error("con un tick de una hora, el daemon no bajó al arrancar: la primera bajada esperaba el tick")
		}
		cancel()
		<-termino
	})

	t.Run("un daemon que arranca despierta al dueño", func(t *testing.T) {
		var pedidos, filas atomic.Int64
		central := centralQueCuenta(t, &pedidos, &filas)
		engA, engB := dosEnginesSobreLaMismaBase(t)
		dueno := daemonConRitmo(t, engA, central.URL, config.SyncConfig{BatchSize: 50}, 0)
		nuevo := daemonConRitmo(t, engB, central.URL, config.SyncConfig{BatchSize: 50}, 0)

		dueno.drainInboundOnce(context.Background()) // baja vacío: el tick siguiente se saltea
		// El arranque del otro daemon, sin ticks: con el contexto ya cancelado corre la marca y el
		// pull del arranque —que no baja: el candado es del dueño— y sale.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		nuevo.RunInboundScheduler(ctx, time.Hour)
		if got := pedidos.Load(); got != 1 {
			t.Fatalf("precondición: el daemon nuevo no tiene el candado y no podía bajar; hubo %d pedidos", got)
		}

		dueno.drainInboundOnce(context.Background())
		if got := pedidos.Load(); got != 2 {
			t.Errorf("un daemon que arranca tenía que despertar al dueño en su tick siguiente, y el dueño no pidió (%d pedidos)", got)
		}
	})
}

// TestTopeNegativoDejaElRitmoFijo: sync.inbound_idle_max_seconds negativo deja la conducta de antes
// —un pedido por tick, vacías incluidas— y la próxima que anota es un tick, no «ya». Lo mismo un tope
// menor que dos ticks, que no deja espacio para saltear ninguno.
//
// Sabotaje: que el negativo se lea como el default.
// arnes: archivo="internal/config/config.go"
// arnes: de="if s.InboundIdleMaxSeconds == 0 {"
// arnes: a="if s.InboundIdleMaxSeconds <= 0 {"
//
// Sabotaje: que con el ritmo fijo la próxima quede a cero ticks.
// arnes: archivo="internal/mcp/bajada_ritmo.go"
// arnes: de="r.saltear = max(0, saltear)"
// arnes: a="r.saltear = saltear"
func TestTopeNegativoDejaElRitmoFijo(t *testing.T) {
	for _, tope := range []int{-1, 45} {
		t.Run("tope "+strconv.Itoa(tope), func(t *testing.T) {
			var pedidos, filas atomic.Int64
			central := centralQueCuenta(t, &pedidos, &filas)
			s := daemonConRitmo(t, memtest.NuevoEngine(t, t.TempDir()), central.URL, config.SyncConfig{BatchSize: 50, InboundIdleMaxSeconds: tope}, 1)
			if got := ticksQueSalen(s, &pedidos, 1, 12, nil); len(got) != 12 {
				t.Errorf("con el tope en %d la bajada tenía que pedir en los 12 ticks, pidió en %v", tope, got)
			}
			l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
			if n := proximaEnSegundos(l); n < 20 || n > 30 {
				t.Errorf("con el ritmo fijo la próxima tenía que caer a ~30 s (un tick), vino %d:\n%s", n, l)
			}
		})
	}
}

// TestUnaPaginaQueNoEntraNoEspaciaLaBajada: «vacía» es una página SIN FILAS, no una que no pudo
// ingerir ninguna. Una fila que esta base rechaza deja el cursor quieto y vuelve en cada tick; si eso
// contara como vacía, el ritmo espaciaría justo la bajada atascada, y la fila que se destrabara —un
// SQLITE_BUSY que pasó— tardaría hasta el tope en entrar.
//
// Sabotaje: que el ritmo cuente como vacía la página que trajo filas y no ingirió ninguna.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="s.ritmoBajada.anotar(u.ConFilas, tick)"
// arnes: a="s.ritmoBajada.anotar(u.Filas, tick)"
func TestUnaPaginaQueNoEntraNoEspaciaLaBajada(t *testing.T) {
	var pedidos, filas atomic.Int64
	filas.Store(1)
	central := centralQueCuenta(t, &pedidos, &filas)
	s, _, db := serverConLaBase(t, central.URL, true)
	s.ritmoBajada.azar = func(int) int { return 0 }
	if _, err := db.Exec(`CREATE TRIGGER veneno BEFORE INSERT ON observations WHEN NEW.id = 'e1'
		BEGIN SELECT RAISE(ABORT, 'fila veneno a propósito'); END`); err != nil {
		t.Fatal(err)
	}
	if got := ticksQueSalen(s, &pedidos, 1, 6, nil); len(got) != 6 {
		t.Errorf("una página con una fila que no entra tenía que volver a pedirse en cada tick (6), se pidió en %v", got)
	}
}

// TestUnTickSalteadoNoEscribeMasQueElCandado: un tick que el ritmo saltea cuesta el reclamo del
// candado —una escritura, la que mantiene al dueño— y una lectura de la marca, y NADA MÁS: ni el
// viaje, ni la edad de la bajada, ni el cursor. Se cuentan las filas que cambian en la base con
// triggers sobre todas sus tablas, y no las llamadas al engine: una escritura por un camino que la
// prueba no previó también cuenta.
//
// Sabotaje: que el tick salteado se anote como una bajada.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\treturn\n\t}\n\tvar cur int64\n"
// arnes: a="\t\tsalioALaRed = true\n\t\treturn\n\t}\n\tvar cur int64\n"
func TestUnTickSalteadoNoEscribeMasQueElCandado(t *testing.T) {
	var pedidos, filas atomic.Int64
	central := centralQueCuenta(t, &pedidos, &filas)
	s, _, db := serverConLaBase(t, central.URL, true)
	lecturas := &engineQueCuentaLecturas{StorageBackend: s.engine}
	s.engine = lecturas

	if _, err := db.Exec(`CREATE TABLE zz_escrituras (tabla TEXT, op TEXT, clave TEXT)`); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		AND name != 'zz_escrituras' AND sql NOT LIKE 'CREATE VIRTUAL%'`)
	if err != nil {
		t.Fatal(err)
	}
	var tablas []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		tablas = append(tablas, n)
	}
	rows.Close()
	vigiladas := 0
	for _, tabla := range tablas {
		clave := func(fila string) string {
			if tabla == "meta" {
				return fila + ".key"
			}
			return "NULL"
		}
		for _, op := range []struct{ nombre, fila string }{{"INSERT", "NEW"}, {"UPDATE", "NEW"}, {"DELETE", "OLD"}} {
			q := `CREATE TRIGGER "zz_` + tabla + `_` + op.nombre + `" AFTER ` + op.nombre + ` ON "` + tabla + `"
				BEGIN INSERT INTO zz_escrituras VALUES ('` + tabla + `', '` + op.nombre + `', ` + clave(op.fila) + `); END`
			if _, err := db.Exec(q); err != nil {
				t.Logf("sin trigger en %s (%s): %v", tabla, op.nombre, err)
				continue
			}
			vigiladas++
		}
	}
	if vigiladas < 30 {
		t.Fatalf("precondición: se vigilan %d triggers sobre %d tablas; así no se ve la base", vigiladas, len(tablas))
	}
	escrituras := func() []string {
		t.Helper()
		rows, err := db.Query(`SELECT tabla || ' ' || op || ' ' || coalesce(clave, '') FROM zz_escrituras`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err != nil {
				t.Fatal(err)
			}
			out = append(out, strings.TrimSpace(e))
		}
		return out
	}

	s.drainInboundOnce(context.Background()) // sale a la red y baja vacío
	vacio := escrituras()
	if pedidos.Load() != 1 || len(vacio) < 2 {
		t.Fatalf("precondición: el primer tick tenía que salir a la red y anotar la bajada (%d pedidos, escrituras %v)", pedidos.Load(), vacio)
	}
	if _, err := db.Exec(`DELETE FROM zz_escrituras`); err != nil {
		t.Fatal(err)
	}
	lecturas.reiniciar()

	s.drainInboundOnce(context.Background()) // el tick que el ritmo saltea
	if pedidos.Load() != 1 {
		t.Fatalf("precondición: el tick siguiente a una vacía no sale a la red, y hubo %d pedidos", pedidos.Load())
	}
	got := escrituras()
	if !slices.Equal(got, []string{"meta UPDATE sync:inbound_lease"}) {
		t.Errorf("un tick salteado tenía que escribir sólo el candado, y escribió %v", got)
	}
	t.Logf("%d triggers sobre %d tablas · tick que sale a la red y vuelve vacío: %d escrituras %v · tick salteado: %d %v, y lee de la meta %v",
		vigiladas, len(tablas), len(vacio), vacio, len(got), got, lecturas.claves())
}

// engineQueCuentaLecturas cuenta las lecturas de la meta, clave por clave.
type engineQueCuentaLecturas struct {
	memory.StorageBackend
	n atomic.Int64
	k atomic.Value // []string
}

func (e *engineQueCuentaLecturas) GetMeta(key string) (string, bool, error) {
	e.n.Add(1)
	prev, _ := e.k.Load().([]string)
	e.k.Store(append(slices.Clone(prev), key))
	return e.StorageBackend.GetMeta(key)
}

func (e *engineQueCuentaLecturas) reiniciar() { e.k.Store([]string(nil)); e.n.Store(0) }

func (e *engineQueCuentaLecturas) claves() []string {
	k, _ := e.k.Load().([]string)
	return k
}

// TestLaLineaDiceQueUnTurnoAdelantaLaProxima: con la bajada espaciada, la próxima anotada puede estar
// a minutos; un turno posterior la adelanta al tick siguiente del dueño, y la línea lo dice —«próxima
// en ≤N s (hubo actividad después de la última)»— en vez de prometer la espera vieja durante el turno
// mismo que la está pidiendo. Una marca anterior a la última bajada no cambia nada: ya se leyó.
//
// Sabotaje: que la línea no mire la marca de actividad.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="despierta := despertar > u.Unix && "
// arnes: a="despierta := false && despertar > u.Unix && "
//
// Sabotaje: que la línea diga que hubo actividad y siga prometiendo la espera vieja.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="u.ProximaUnix = despertar + int64(tick/time.Second)"
// arnes: a="u.ProximaUnix = u.ProximaUnix + 0*despertar"
func TestLaLineaDiceQueUnTurnoAdelantaLaProxima(t *testing.T) {
	s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
	ahora := time.Now().Unix()
	u := memory.UltimaBajada{Unix: ahora - 100, ProximaUnix: ahora + 200}
	if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 1, Vacias: 1}, u); err != nil {
		t.Fatal(err)
	}
	espaciada := func(t *testing.T, cuando string) {
		t.Helper()
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " (vacía), próxima en ~3 min") || strings.Contains(l, "hubo actividad") {
			t.Errorf("%s, la línea tenía que prometer la próxima anotada, «próxima en ~3 min»:\n%s", cuando, l)
		}
		t.Logf("%s: %s", cuando, l)
	}

	espaciada(t, "sin marca")
	marcar(t, eng, ahora-150)
	espaciada(t, "con una marca anterior a la última bajada")

	marcar(t, eng, ahora-10)
	l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
	const cola = " s (hubo actividad después de la última)"
	i := strings.Index(l, "próxima en ≤")
	if i < 0 || !strings.HasSuffix(l, cola) {
		t.Fatalf("con un turno después de la última bajada la línea tenía que decir «próxima en ≤N s (hubo actividad después de la última)»:\n%s", l)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(l[i+len("próxima en ≤"):], cola))
	if err != nil || n < 15 || n > 20 {
		t.Errorf("la marca es de hace 10 s y el tick de 30: la próxima tenía que caer a ≤20 s, vino %q:\n%s", l[i:], l)
	}
	t.Logf("con un turno después: %s", l)
}

// TestLaBajadaEnUnaHoraSimulada es la MEDICIÓN del ritmo, no una guarda de una línea: cuántos pedidos
// le llegan al central en una hora —120 ticks de 30 s— con el central vacío, según cuánto se trabaje
// sobre la base. Quieta: los del espaciado, 14. Con un turno cada 10 min (cada 20 ticks): cada turno
// reinicia el espaciado, 24. Con un turno por tick: 120, igual que antes. Las cotas dejan lugar al
// azar de producción, que corre acá sin fijar.
func TestLaBajadaEnUnaHoraSimulada(t *testing.T) {
	for _, c := range []struct {
		nombre     string
		turnoCada  int // en ticks; 0 = nadie trabaja
		min, techo int
	}{
		{"quieta", 0, 12, 14},
		{"un turno cada 10 min", 20, 22, 24},
		{"un turno por tick", 1, 120, 120},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			var pedidos, filas atomic.Int64
			central := centralQueCuenta(t, &pedidos, &filas)
			dueno, terminal := dosEnginesSobreLaMismaBase(t)
			s := NewMcpServer(dueno, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
			s.SetSyncClient(newTestSyncClient(t, central.URL), config.SyncConfig{BatchSize: 50})
			got := ticksQueSalen(s, &pedidos, 1, 120, func(tick int) {
				if c.turnoCada > 0 && (tick-1)%c.turnoCada == 0 {
					marcar(t, terminal, int64(1_800_000_000+30*tick))
				}
			})
			if len(got) < c.min || len(got) > c.techo {
				t.Errorf("%s: %d pedidos en 120 ticks; esperaba entre %d y %d", c.nombre, len(got), c.min, c.techo)
			}
			t.Logf("%s: %d pedidos en una hora (antes: 120), %.0f %% menos", c.nombre, len(got), 100-float64(len(got))*100/120)
		})
	}
}
