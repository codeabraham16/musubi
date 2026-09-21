package memory

import (
	"context"
	"sync"
	"testing"
)

// vectorPorContenido embebe cada texto a un vector que depende SÓLO de su contenido, para poder
// preguntar después de qué texto salió el vector guardado.
func vectorPorContenido(_ context.Context, textos []string) ([][]float32, error) {
	out := make([][]float32, len(textos))
	for i, tx := range textos {
		switch tx {
		case "alfa original":
			out[i] = []float32{1, 0, 0}
		case "alfa editado":
			out[i] = []float32{0, 1, 0}
		default:
			out[i] = []float32{0, 0, 1}
		}
	}
	return out, nil
}

// vectorGuardado devuelve el vector persistido de id (nil si no tiene).
func vectorGuardado(t *testing.T, e *DbEngine, id string) []float32 {
	t.Helper()
	var raw []byte
	err := e.db.QueryRow(`SELECT vector FROM embeddings WHERE observation_id = ?`, id).Scan(&raw)
	if err != nil {
		return nil
	}
	v, err := BytesToFloat32(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ingestadaConVector baja c1 del central con contenido y le da vector con el relleno incremental.
func ingestadaConVector(t *testing.T, contenido string) *DbEngine {
	t.Helper()
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@aaaa")
	if _, err := e.IngestShared(SharedObs{ID: "c1", TopicKey: "t/a", Content: contenido, Importance: 1}); err != nil {
		t.Fatal(err)
	}
	e.IncrementalEmbedBackfill(vectorPorContenido)
	e.bgWG.Wait()
	if v := vectorGuardado(t, e, "c1"); len(v) != 3 || v[0] != 1 {
		t.Fatalf("precondición: c1 debería tener el vector de %q, tiene %v", contenido, v)
	}
	return e
}

// Una EDICIÓN que baja del central invalida el vector del contenido viejo, y el relleno la
// re-embebe con el texto nuevo.
//
// El central sube el sync_seq en cada update, así que la fila editada vuelve a bajar y el UPSERT de
// IngestShared le pisa el contenido; la fila de embeddings seguía con el vector viejo y el model_id
// actual, fuera de stalePredicate: nadie la re-embebía y la búsqueda semántica la rankeaba por lo
// que ya no dice.
//
// Sabotaje que la hace fallar: no invalidar nunca el vector al pisar el contenido → c1 queda con el
// vector de «alfa original» después de la edición.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="\tcambio := existia && previo != clean\n"
// arnes: a="\tcambio := false\n"
// arnes: colision_ok="TestUnaReentregaSinCambioDeContenidoConservaElVector"
func TestUnaEdicionBajadaInvalidaElVectorViejo(t *testing.T) {
	e := ingestadaConVector(t, "alfa original")

	if _, err := e.IngestShared(SharedObs{ID: "c1", TopicKey: "t/a", Content: "alfa editado", Importance: 1}); err != nil {
		t.Fatal(err)
	}
	if n, err := e.countStaleEmbeddings(); err != nil || n != 1 {
		t.Fatalf("tras la edición c1 debería quedar pendiente de vector, pendientes n=%d err=%v", n, err)
	}
	e.IncrementalEmbedBackfill(vectorPorContenido)
	e.bgWG.Wait()

	if v := vectorGuardado(t, e, "c1"); len(v) != 3 || v[1] != 1 {
		t.Errorf("c1 fue editada a «alfa editado» pero su vector es %v, esperaba [0 1 0]: sigue el del contenido viejo", v)
	}
}

// La otra mitad: si lo que baja NO cambió el contenido —una re-entrega, o un cambio de importancia,
// que también sube el sync_seq— el vector sigue valiendo y no se toca. Invalidar de más le cuesta
// un pedido al embebedor por fila y la deja fuera del recall semántico hasta que vuelva.
//
// Sabotaje que la hace fallar: invalidar el vector en todo UPSERT sobre una fila existente → la
// re-entrega de c1 la deja pendiente y sin vector.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="\tcambio := existia && previo != clean\n"
// arnes: a="\tcambio := existia\n"
// arnes: colision_ok="TestUnaEdicionBajadaInvalidaElVectorViejo"
func TestUnaReentregaSinCambioDeContenidoConservaElVector(t *testing.T) {
	e := ingestadaConVector(t, "alfa original")

	inserto, err := e.IngestShared(SharedObs{ID: "c1", TopicKey: "t/a", Content: "alfa original", Importance: 5})
	if err != nil {
		t.Fatal(err)
	}
	if inserto {
		t.Error("IngestShared reportó una fila nueva al re-entregar c1")
	}
	if n, err := e.countStaleEmbeddings(); err != nil || n != 0 {
		t.Errorf("la re-entrega sin cambio de contenido no debería dejar nada pendiente, pendientes n=%d err=%v", n, err)
	}
	if v := vectorGuardado(t, e, "c1"); len(v) != 3 || v[0] != 1 {
		t.Errorf("la re-entrega sin cambio de contenido tocó el vector de c1: %v", v)
	}
}

// La carrera que la invalidación sola no cubre: la edición baja MIENTRAS una pasada de relleno ya
// listó la fila con su contenido viejo y está esperando al embebedor. IngestShared borra el vector,
// pero la pasada en vuelo lo volvía a escribir con el texto viejo y el model_id actual, y la fila
// quedaba fuera de stalePredicate para siempre. El pull que bajó la edición pide su vuelta; la fila
// tiene que terminar con el vector del contenido NUEVO.
//
// Sabotaje que la hace fallar: persistir el vector sin mirar si el contenido cambió → la pasada en
// vuelo pisa la invalidación con el vector de «alfa original».
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de=" SELECT ?, ?, ? WHERE EXISTS (SELECT 1 FROM observations WHERE id = ? AND content = ?)`,\n\t\t\t\tp.id, vectorBytes, e.vectorModelID, p.id, p.content,"
// arnes: a=" VALUES (?, ?, ?)`,\n\t\t\t\tp.id, vectorBytes, e.vectorModelID,"
func TestUnaEdicionQueBajaDuranteElRellenoNoQuedaConElVectorViejo(t *testing.T) {
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@aaaa")
	if _, err := e.IngestShared(SharedObs{ID: "c1", TopicKey: "t/a", Content: "alfa original", Importance: 1}); err != nil {
		t.Fatal(err)
	}

	entro, soltar := make(chan struct{}), make(chan struct{})
	var primera sync.Once
	e.IncrementalEmbedBackfill(func(ctx context.Context, textos []string) ([][]float32, error) {
		// La primera pasada ya listó c1 con «alfa original» cuando se queda acá adentro.
		primera.Do(func() { close(entro); <-soltar })
		return vectorPorContenido(ctx, textos)
	})
	<-entro
	if _, err := e.IngestShared(SharedObs{ID: "c1", TopicKey: "t/a", Content: "alfa editado", Importance: 1}); err != nil {
		t.Fatal(err)
	}
	e.IncrementalEmbedBackfill(vectorPorContenido) // el pull que bajó la edición pide su vuelta
	close(soltar)
	e.bgWG.Wait()

	if v := vectorGuardado(t, e, "c1"); len(v) != 3 || v[1] != 1 {
		t.Errorf("c1 terminó con el vector %v, esperaba el de «alfa editado» [0 1 0]: la pasada en vuelo le repuso el del contenido viejo", v)
	}
}
