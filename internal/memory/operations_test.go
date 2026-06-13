package memory

import (
	"testing"
)

// newTestEngine crea un DbEngine respaldado por un directorio temporal autolimpiable.
func newTestEngine(t *testing.T) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return engine
}

func TestSaveObservationWithoutEmbedding(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservation("obs1", "topic/a", "contenido de prueba", nil); err != nil {
		t.Fatalf("SaveObservation error: %v", err)
	}

	// Debe encontrarse por FTS pero NO tener embedding (búsqueda semántica vacía).
	fts, err := e.SearchObservationsFTS("prueba", 5)
	if err != nil {
		t.Fatalf("SearchObservationsFTS error: %v", err)
	}
	if len(fts) != 1 || fts[0].ID != "obs1" {
		t.Fatalf("esperaba 1 resultado FTS obs1, obtuve %+v", fts)
	}

	sem, err := e.SearchObservations([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("SearchObservations error: %v", err)
	}
	if len(sem) != 0 {
		t.Fatalf("esperaba 0 resultados semánticos (sin embedding), obtuve %d", len(sem))
	}
}

func TestSaveObservationUpsertByID(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservation("dup", "t", "versión uno", []float32{1, 0}); err != nil {
		t.Fatalf("save 1 error: %v", err)
	}
	if err := e.SaveObservation("dup", "t", "versión dos", []float32{0, 1}); err != nil {
		t.Fatalf("save 2 error: %v", err)
	}

	res, err := e.SearchObservations([]float32{0, 1}, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("esperaba 1 observación tras upsert, obtuve %d", len(res))
	}
	if res[0].Content != "versión dos" {
		t.Errorf("esperaba contenido actualizado, obtuve %q", res[0].Content)
	}
}

func TestSearchObservationsOrderingAndLimit(t *testing.T) {
	e := newTestEngine(t)

	// Vector query = {1,0}. Similitud: a más alineado, mayor score.
	mustSave(t, e, "alto", []float32{1, 0})       // cos = 1
	mustSave(t, e, "medio", []float32{1, 1})      // cos ~ 0.707
	mustSave(t, e, "bajo", []float32{0, 1})       // cos = 0
	mustSave(t, e, "negativo", []float32{-1, 0})  // cos = -1

	res, err := e.SearchObservations([]float32{1, 0}, 2)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("esperaba 2 resultados por límite, obtuve %d", len(res))
	}
	if res[0].ID != "alto" || res[1].ID != "medio" {
		t.Errorf("orden incorrecto: %s, %s", res[0].ID, res[1].ID)
	}
}

func TestSearchObservationsSkipsDimensionMismatch(t *testing.T) {
	e := newTestEngine(t)

	mustSave(t, e, "dim2", []float32{1, 0})
	mustSave(t, e, "dim3", []float32{1, 0, 0})

	// Query de dimensión 3: solo "dim3" es comparable; "dim2" se ignora.
	res, err := e.SearchObservations([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "dim3" {
		t.Fatalf("esperaba solo dim3, obtuve %+v", res)
	}
}

func TestSearchObservationsNegativeLimitNoPanic(t *testing.T) {
	e := newTestEngine(t)
	mustSave(t, e, "x", []float32{1, 0})

	// limit negativo no debe panic: se interpreta como "sin límite".
	res, err := e.SearchObservations([]float32{1, 0}, -1)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("esperaba 1 resultado, obtuve %d", len(res))
	}
}

func TestFTSDeleteTriggerRemovesIndex(t *testing.T) {
	e := newTestEngine(t)
	mustSave(t, e, "borrable", []float32{1, 0})

	if _, err := e.db.Exec(`DELETE FROM observations WHERE id = ?`, "borrable"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	fts, err := e.SearchObservationsFTS("contenido", 5)
	if err != nil {
		t.Fatalf("fts error: %v", err)
	}
	if len(fts) != 0 {
		t.Fatalf("esperaba 0 tras borrar (trigger AFTER DELETE), obtuve %d", len(fts))
	}
}

func mustSave(t *testing.T, e *DbEngine, id string, emb []float32) {
	t.Helper()
	if err := e.SaveObservation(id, "topic", "contenido "+id, emb); err != nil {
		t.Fatalf("SaveObservation(%s) error: %v", id, err)
	}
}
