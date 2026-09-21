package memory

import (
	"sync"
	"testing"
)

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
// arnes: de="\t\te.incrOtraVuelta = true\n"
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

	e.IncrementalEmbedBackfill(embed)
	<-entro
	if err := e.SaveObservation("a2", "t/x", "contenido a2", nil); err != nil {
		t.Fatal(err)
	}
	e.IncrementalEmbedBackfill(embed) // llega corriendo: tiene que coalescer, no perderse
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
	e.IncrementalEmbedBackfill(enLote(func(s string) ([]float32, error) { llamadas++; return fixedEmbed(s) }))
	e.bgWG.Wait()
	if llamadas != 0 {
		t.Errorf("sin vectorModelID no debe embeber nada, hubo %d llamadas", llamadas)
	}
}
