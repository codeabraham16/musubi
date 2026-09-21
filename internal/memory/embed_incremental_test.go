package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// conCtx adapta un embebedor de prueba sin ctx a la firma de IncrementalEmbedBackfill.
func conCtx(f func([]string) ([][]float32, error)) func(context.Context, []string) ([][]float32, error) {
	return func(_ context.Context, textos []string) ([][]float32, error) { return f(textos) }
}

// El backfill incremental COALESCE: si llega un pedido mientras una pasada está embebiendo, no se
// lanza otra en paralelo, pero tampoco se pierde. La pasada en vuelo ya hizo su SELECT y no ve la
// fila que entró después; el pedido que llegó corriendo se cobra como UNA vuelta más al terminar.
//
// Es el caso real del sync entrante: un pull ingiere mientras la vectorización del anterior sigue
// trabajando. Sin la vuelta extra, esa fila esperaría al próximo pull con filas —o al reinicio—,
// que es exactamente el hueco que el backfill incremental vino a cerrar.
//
// Sabotaje que la hace fallar: no anotar la vuelta extra cuando ya hay una corrida en vuelo → a2
// llega mientras a1 se embebe, su pedido se descarta y queda sin vector.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="\t\te.rellenoOtraVuelta = true\n"
// arnes: a=""
func TestIncrementalEmbedBackfillNoPierdeElPedidoQueLlegaCorriendo(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a1", "t/x", "contenido a1", nil); err != nil {
		t.Fatal(err)
	}
	e.SetVectorModelID("static:tabla@aaaa")

	entro := make(chan struct{})
	soltar := make(chan struct{})
	var primera sync.Once
	embed := func(textos []string) ([][]float32, error) {
		// La primera llamada se queda adentro hasta que la prueba la suelte: así la pasada en vuelo
		// ya listó sus pendientes (sólo a1) cuando llega el segundo pedido.
		primera.Do(func() { close(entro); <-soltar })
		return enLote(fixedEmbed)(textos)
	}

	e.IncrementalEmbedBackfill(conCtx(embed))
	<-entro
	if err := e.SaveObservation("a2", "t/x", "contenido a2", nil); err != nil {
		t.Fatal(err)
	}
	e.IncrementalEmbedBackfill(conCtx(embed)) // llega corriendo: tiene que coalescer, no perderse
	close(soltar)
	e.bgWG.Wait()

	if n := countEmbeddingsWithModel(t, e, "static:tabla@aaaa"); n != 2 {
		t.Errorf("esperaba a1 y a2 con vector tras coalescer, hay %d", n)
	}
	if n, err := e.countStaleEmbeddings(); err != nil || n != 0 {
		t.Errorf("no debería quedar nada pendiente, quedan n=%d err=%v", n, err)
	}
}

// Sin embebedor nombrado (Noop/none) no hay semántica: el incremental no lanza nada ni llama al
// callback. Es la mitad del contrato que evita embeber con un proveedor que no existe.
func TestIncrementalEmbedBackfillNoopSinModelo(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t/x", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	llamadas := 0
	e.IncrementalEmbedBackfill(conCtx(enLote(func(s string) ([]float32, error) { llamadas++; return fixedEmbed(s) })))
	e.bgWG.Wait()
	if llamadas != 0 {
		t.Errorf("sin vectorModelID no debe embeber nada, hubo %d llamadas", llamadas)
	}
}

// El relleno del ARRANQUE y el que dispara el PULL pasan por la misma puerta: si el primer pull con
// filas llega mientras AutoEmbedBackfill todavía embebe, no se lanza una segunda corrida que liste
// el mismo conjunto pendiente. Cada observación se embebe UNA vez, y la fila que bajó durante la
// corrida del arranque se cobra como la vuelta extra.
//
// Es el caso que midió el revisor: antes la exclusión cubría sólo la puerta incremental, y las dos
// corridas embebían todo dos veces.
//
// Sabotaje que la hace fallar: que el arranque lance su corrida por fuera de la puerta compartida
// → el pull lanza otra en paralelo, las dos listan a1 y a2, y cada una se embebe dos veces.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="\te.pedirRelleno(embed, true)\n"
// arnes: a="\te.spawnBackground(func() { e.pasadaCompleta(embed) })\n"
func TestElRellenoDelArranqueYElDelPullNoEmbebenDosVeces(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a1", "a2"} {
		if err := e.SaveObservation(id, "t/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	e.SetVectorModelID("static:tabla@aaaa")

	var mu sync.Mutex
	veces := map[string]int{}
	entro, soltar, segunda := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var primera, otra sync.Once
	llamadas := 0
	embed := func(textos []string) ([][]float32, error) {
		mu.Lock()
		for _, tx := range textos {
			veces[tx]++
		}
		llamadas++
		n := llamadas
		mu.Unlock()
		if n > 1 {
			otra.Do(func() { close(segunda) })
		}
		// La primera llamada es la del arranque: se queda adentro hasta que la prueba la suelte,
		// así el pull llega con la corrida del arranque en vuelo y sus pendientes ya listadas.
		primera.Do(func() { close(entro); <-soltar })
		return enLote(fixedEmbed)(textos)
	}

	e.AutoEmbedBackfill(embed)
	<-entro
	if err := e.SaveObservation("a3", "t/x", "contenido a3", nil); err != nil {
		t.Fatal(err)
	}
	e.IncrementalEmbedBackfill(conCtx(embed)) // el primer pull con filas, con el arranque en vuelo

	// Si el pedido quedó anotado, la puerta es compartida y se puede soltar ya. Si no, hay una
	// segunda corrida suelta: se espera a que embeba en paralelo, que es el defecto a medir.
	e.rellenoMu.Lock()
	anotado := e.rellenoOtraVuelta
	e.rellenoMu.Unlock()
	if !anotado {
		select {
		case <-segunda:
		case <-time.After(5 * time.Second):
		}
	}
	close(soltar)
	e.bgWG.Wait()

	for _, c := range []string{"contenido a1", "contenido a2", "contenido a3"} {
		if veces[c] != 1 {
			t.Errorf("%q se embebió %d veces, esperaba 1 (todas: %v)", c, veces[c], veces)
		}
	}
	if n, err := e.countStaleEmbeddings(); err != nil || n != 0 {
		t.Errorf("no debería quedar nada pendiente, quedan n=%d err=%v", n, err)
	}
	// El arranque sigue siendo la corrida COMPLETA: declara la marca de modelo.
	if raw, ok, _ := e.GetMeta(MetaEmbedModel); !ok || raw != "static:tabla@aaaa" {
		t.Errorf("MetaEmbedModel = %q (ok=%v): la corrida del arranque dejó de ser la completa", raw, ok)
	}
}

// cierreConPasadaEnVuelo arma un engine con tres lotes pendientes, lanza el relleno incremental con
// el embebedor dado, espera a que la primera llamada entre y cierra el engine. Falla si Close no
// vuelve a tiempo; lo que pasó adentro lo cuenta el embebedor de cada prueba.
func cierreConPasadaEnVuelo(t *testing.T, embed func(context.Context, []string) ([][]float32, error), entro <-chan struct{}) {
	t.Helper()
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3*embedBatchSize; i++ {
		if err := e.SaveObservation(fmt.Sprintf("c%02d", i), "t/x", fmt.Sprintf("contenido %02d", i), nil); err != nil {
			t.Fatal(err)
		}
	}
	e.SetVectorModelID("static:tabla@aaaa")
	e.IncrementalEmbedBackfill(embed)
	<-entro
	cerro := make(chan error, 1)
	go func() { cerro <- e.Close() }()
	select {
	case err := <-cerro:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close no volvió en 10 s con una pasada de relleno en vuelo")
	}
}

// Un apagado le cancela el ctx al pedido EN VUELO al embebedor, en vez de esperar a que un
// proveedor por HTTP termine un lote que ya nadie va a usar.
//
// Sabotaje que la hace fallar: no cancelar el ctx de cierre en Close → el pedido en vuelo no se
// entera y la prueba lo ve esperar hasta su propio techo.
// arnes: archivo="internal/memory/database.go"
// arnes: de="\t\te.cancelarCierre()\n\t}\n\te.lifecycleMu.Unlock()\n"
// arnes: a="\t}\n\te.lifecycleMu.Unlock()\n"
func TestElCierreCancelaElPedidoEnVueloAlEmbebedor(t *testing.T) {
	entro := make(chan struct{})
	var primera sync.Once
	cancelado := make(chan bool, 1)
	cierreConPasadaEnVuelo(t, func(ctx context.Context, textos []string) ([][]float32, error) {
		primera.Do(func() {
			close(entro)
			select {
			case <-ctx.Done():
				cancelado <- true
			case <-time.After(3 * time.Second):
				cancelado <- false
			}
		})
		return enLote(fixedEmbed)(textos)
	}, entro)
	if !<-cancelado {
		t.Error("Close no canceló el ctx del pedido en vuelo: el apagado espera al embebedor")
	}
}

// Entre lotes, la pasada mira el cierre: aunque el lote en vuelo termine bien, no toma el
// siguiente. Una base entera pendiente son cientos de lotes y Close los esperaría todos.
//
// Sabotaje que la hace fallar: sacar la mirada al cierre del principio de cada lote → la pasada
// embebe los tres lotes con el engine cerrándose.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="\t\tif e.cerrando() {\n\t\t\treturn errRellenoCortadoPorCierre\n\t\t}\n\t\tfin :="
// arnes: a="\t\tfin :="
func TestElCierreNoDejaTomarOtroLote(t *testing.T) {
	entro := make(chan struct{})
	var primera sync.Once
	var llamadas atomic.Int32
	cierreConPasadaEnVuelo(t, func(ctx context.Context, textos []string) ([][]float32, error) {
		llamadas.Add(1)
		// El lote en vuelo termina BIEN aunque le cancelen el ctx: así lo único que puede cortar
		// es la mirada entre lotes, no un error del embebedor.
		primera.Do(func() { close(entro); <-ctx.Done() })
		return enLote(fixedEmbed)(textos)
	}, entro)
	if n := llamadas.Load(); n != 1 {
		t.Errorf("el embebedor recibió %d lotes, esperaba 1: la pasada siguió tomando lotes con el engine cerrándose", n)
	}
}

// Si el lote en vuelo FALLA porque el cierre le canceló el ctx, no se lo reintenta texto por texto:
// serían 16 pedidos más, más la sonda, contra un ctx que ya no sirve.
//
// Sabotaje que la hace fallar: sacar el corte por cierre tras el error del lote → embedUnoAUno
// reintenta los 16 textos y la sonda contra el ctx cancelado.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="if err != nil && e.cerrando() {"
// arnes: a="if err != nil && false {"
func TestElCierreNoReintentaDeAUnoElLoteCancelado(t *testing.T) {
	entro := make(chan struct{})
	var primera sync.Once
	var llamadas atomic.Int32
	cierreConPasadaEnVuelo(t, func(ctx context.Context, textos []string) ([][]float32, error) {
		llamadas.Add(1)
		primera.Do(func() { close(entro); <-ctx.Done() })
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return enLote(fixedEmbed)(textos)
	}, entro)
	if n := llamadas.Load(); n != 1 {
		t.Errorf("el embebedor recibió %d pedidos, esperaba 1: el lote cancelado por el cierre se reintentó de a uno", n)
	}
}

// Un panic del embebedor no escapa de la pasada de relleno: en la goroutine de fondo mataría el
// PROCESO entero. Acá la pasada corre en la goroutine de la prueba, así que si el panic escapa lo
// ataja el recover de la prueba y se lee como un fallo con nombre; en la de fondo tumbaría el
// binario y no quedaría ni un `--- FAIL` que leer.
//
// Sabotaje que la hace fallar: neutralizar el recover de la pasada → el panic llega hasta el
// recover de la prueba.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="if r := recover(); r != nil {"
// arnes: a="if r := any(nil); r != nil {"
func TestLaPasadaDeRellenoContieneElPanicoDelEmbebedor(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("p1", "t/x", "contenido p1", nil); err != nil {
		t.Fatal(err)
	}
	e.SetVectorModelID("panic-test")
	llamado := false
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("el panic del embebedor escapó de la pasada de relleno: %v", r)
		}
	}()
	e.pasadaDeRelleno(func([]string) ([][]float32, error) {
		llamado = true
		panic("embebedor roto (test)")
	}, false, e.vindexCfg)
	if !llamado {
		t.Fatal("el embebedor no se llamó nunca: esta prueba no ejerció el recover")
	}
}
