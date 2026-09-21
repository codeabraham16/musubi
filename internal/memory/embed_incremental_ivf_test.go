package memory

import (
	"context"
	"testing"
)

// Con el índice IVF ENTRENADO, lo que vectoriza el relleno incremental entra al índice en el acto.
//
// Es la mitad que ninguna prueba cubría: debajo de ExactThreshold (hoy davantis-1, ~3.400 vectores)
// la búsqueda es exacta y cualquier vector persistido aparece; con el índice entrenado,
// SearchObservations rankea SÓLO lo que el índice devuelve, y un vector persistido pero no agregado
// queda invisible hasta el próximo rebuild —horas: RebuildEvery + RebuildMinHours—. El umbral se
// baja a 1 para entrenar con pocas filas, y la consulta es el vector de una fila existente, así la
// celda sondeada nunca está vacía (vacía, SearchObservations caería al full-scan y la prueba
// pasaría sin haber mirado el índice).
//
// Sabotaje que la hace fallar: no pasar el Add del índice a la pasada incremental → el vector se
// persiste pero no entra al IVF, y la búsqueda por el índice no lo devuelve.
// arnes: archivo="internal/memory/embed_backfill.go"
// arnes: de="\t\talGuardar = e.index.Add\n"
// arnes: a=""
func TestIncrementalEmbedBackfillAgregaAlIndiceEntrenado(t *testing.T) {
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@aaaa")

	const dim = 16
	data := clusteredDataset(21, 120, dim, 6)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "c "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	e.vindexCfg.Enabled = true
	e.vindexCfg.ExactThreshold = 1
	e.vindexCfg.NProbe = 1 // una sola celda: lo que no esté en el índice no puede colarse por otra
	if err := e.rebuildVectorIndex(); err != nil {
		t.Fatal(err)
	}
	if !e.index.Trained() {
		t.Fatal("precondición: el índice debería estar entrenado")
	}

	// La fila nueva baja del central SIN vector, como en el sync entrante. Su vector va a ser el de
	// una fila ya indexada: misma celda, así que la celda sondeada tiene candidatos.
	consulta := data[0].vec
	if _, err := e.IngestShared(SharedObs{ID: "nueva", TopicKey: "t", Content: "bajada del central", Importance: 1}); err != nil {
		t.Fatal(err)
	}
	if ids, ok := e.index.Search(consulta, e.vindexCfg.NProbe); !ok || len(ids) == 0 {
		t.Fatalf("precondición: la celda sondeada debería tener candidatos (ok=%v, %d ids): si no, la búsqueda cae al full-scan y esta prueba no mira el índice", ok, len(ids))
	}

	e.IncrementalEmbedBackfill(func(_ context.Context, textos []string) ([][]float32, error) {
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = consulta
		}
		return out, nil
	})
	e.bgWG.Wait()

	if n := countEmbeddingsWithModel(t, e, "static:tabla@aaaa"); n != len(data)+1 {
		t.Fatalf("precondición: esperaba %d vectores persistidos, hay %d", len(data)+1, n)
	}
	res, err := e.SearchObservations(context.Background(), consulta, len(data)+1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.ID == "nueva" {
			return
		}
	}
	t.Errorf("«nueva» tiene vector persistido pero la búsqueda por el índice entrenado no la devuelve (%d resultados): no entró al IVF", len(res))
}
