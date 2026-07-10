package memory

import "testing"

// fixedEmbed es un embebedor de prueba: vector constante no vacío (la prueba valida PERSISTENCIA y
// procedencia, no calidad de búsqueda).
func fixedEmbed(string) ([]float32, error) { return []float32{1, 0, 0}, nil }

func countEmbeddingsWithModel(t *testing.T, e *DbEngine, modelID string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM embeddings WHERE model_id = ?`, modelID).Scan(&n); err != nil {
		t.Fatalf("contar embeddings model_id=%q: %v", modelID, err)
	}
	return n
}

// TestEmbedBackfillReembedsHistory valida el re-embedding del histórico (T17.3): observaciones
// guardadas SIN vector (memoria previa a encender la semántica) quedan recuperables tras el
// backfill; la corrida es idempotente; y un cambio de modelo re-embebe con la nueva procedencia.
func TestEmbedBackfillReembedsHistory(t *testing.T) {
	e := newTestEngine(t)

	// Histórico: 3 observaciones SIN embedding (embedding nil) — como antes de encender semántica.
	for _, id := range []string{"h1", "h2", "h3"} {
		if err := e.SaveObservation(id, "t/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}

	// Sin embedder nombrado ⇒ error (no hay semántica que backfillear).
	if _, err := e.EmbedBackfill(fixedEmbed); err == nil {
		t.Error("EmbedBackfill sin vectorModelID debería fallar")
	}

	// Encender la procedencia (como serve/daemon con un embedder) y backfillear.
	e.SetVectorModelID("static:test-A")
	res, err := e.EmbedBackfill(fixedEmbed)
	if err != nil {
		t.Fatalf("EmbedBackfill: %v", err)
	}
	if res.Scanned != 3 || res.Embedded != 3 || res.Skipped != 0 {
		t.Errorf("esperaba scanned=3 embedded=3 skipped=0, obtuve %+v", res)
	}
	if n := countEmbeddingsWithModel(t, e, "static:test-A"); n != 3 {
		t.Errorf("esperaba 3 embeddings con procedencia test-A, obtuve %d", n)
	}

	// Idempotencia: re-correr no encuentra pendientes (ya tienen el model_id actual).
	res2, err := e.EmbedBackfill(fixedEmbed)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Scanned != 0 || res2.Embedded != 0 {
		t.Errorf("segunda corrida debería no tener pendientes, obtuve %+v", res2)
	}

	// Cambio de modelo: ahora las 3 tienen procedencia distinta a la nueva ⇒ se re-embeben.
	e.SetVectorModelID("static:test-B")
	res3, err := e.EmbedBackfill(fixedEmbed)
	if err != nil {
		t.Fatal(err)
	}
	if res3.Scanned != 3 || res3.Embedded != 3 {
		t.Errorf("cambio de modelo debería re-embeber las 3, obtuve %+v", res3)
	}
	if n := countEmbeddingsWithModel(t, e, "static:test-B"); n != 3 {
		t.Errorf("esperaba 3 embeddings con procedencia test-B, obtuve %d", n)
	}
	// La procedencia vieja ya no existe (INSERT OR REPLACE por observation_id pisó la fila).
	if n := countEmbeddingsWithModel(t, e, "static:test-A"); n != 0 {
		t.Errorf("la procedencia vieja debería haberse reemplazado, quedan %d", n)
	}
}

// TestEmbedBackfillSkipsEmptyVectors valida que un embebedor que devuelve vector vacío no persiste
// nada (cuenta como skipped), sin romper la corrida.
func TestEmbedBackfillSkipsEmptyVectors(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x1", "t/x", "algo", nil); err != nil {
		t.Fatal(err)
	}
	e.SetVectorModelID("static:test")
	res, err := e.EmbedBackfill(func(string) ([]float32, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 1 || res.Embedded != 0 || res.Skipped != 1 {
		t.Errorf("esperaba scanned=1 embedded=0 skipped=1, obtuve %+v", res)
	}
}
