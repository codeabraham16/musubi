package memory

import (
	"context"
	"testing"
)

// La columna de procedencia existe tras la migración v12.
func TestEmbeddingsHasModelIDColumn(t *testing.T) {
	e := newTestEngine(t)
	rows, err := e.db.Query(`PRAGMA table_info(embeddings)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(embeddings): %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == "model_id" {
			found = true
		}
	}
	if !found {
		t.Fatal("embeddings no tiene la columna model_id tras la migración v12")
	}
}

// Regla de homogeneidad (F2.2): la búsqueda semántica sólo compara vectores de la MISMA
// procedencia que el embedder activo. Un vector de OTRO modelo, aunque sea un match de
// coseno perfecto, queda EXCLUIDO del recall (no se mezcla ni corrompe el ranking).
func TestVectorProvenanceHomogeneity(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	query := []float32{1, 0, 0, 0}

	// obs "a1": match PERFECTO con la query, pero producido por el modelo A.
	e.SetVectorModelID("model-a")
	if err := e.SaveObservation("a1", "topic", "contenido a", []float32{1, 0, 0, 0}); err != nil {
		t.Fatalf("SaveObservation a1: %v", err)
	}
	// obs "b1": match nulo (ortogonal), pero producido por el modelo B (el activo).
	e.SetVectorModelID("model-b")
	if err := e.SaveObservation("b1", "topic", "contenido b", []float32{0, 1, 0, 0}); err != nil {
		t.Fatalf("SaveObservation b1: %v", err)
	}

	// Con el modelo B activo, la búsqueda NO debe ver a1 (otra procedencia), aunque sea el
	// match perfecto: sólo b1, de la procedencia homogénea.
	res, err := e.SearchObservations(ctx, query, 10)
	if err != nil {
		t.Fatalf("SearchObservations (model-b): %v", err)
	}
	ids := map[string]bool{}
	for _, r := range res {
		ids[r.ID] = true
	}
	if ids["a1"] {
		t.Error("a1 (otra procedencia) NO debería aparecer: se estaría mezclando coseno entre modelos")
	}
	if !ids["b1"] {
		t.Error("b1 (misma procedencia que el embedder activo) debería aparecer")
	}

	// Al volver al modelo A, a1 reaparece y b1 desaparece: el filtro es POR FILA según la
	// procedencia registrada, no un estado global.
	e.SetVectorModelID("model-a")
	res, err = e.SearchObservations(ctx, query, 10)
	if err != nil {
		t.Fatalf("SearchObservations (model-a): %v", err)
	}
	ids = map[string]bool{}
	for _, r := range res {
		ids[r.ID] = true
	}
	if !ids["a1"] {
		t.Error("con el modelo A activo, a1 debería aparecer")
	}
	if ids["b1"] {
		t.Error("con el modelo A activo, b1 (procedencia B) no debería aparecer")
	}
}

// Backward-compat: un engine sin model_id ("" = procedencia desconocida) compara contra
// los vectores "" (legacy y los de tests/bench sin embedder nombrado). El comportamiento
// histórico no cambia.
func TestVectorProvenanceEmptyMatchesEmpty(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t) // vectorModelID == "" por defecto
	if err := e.SaveObservation("o1", "topic", "hola", []float32{1, 0, 0, 0}); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}
	res, err := e.SearchObservations(ctx, []float32{1, 0, 0, 0}, 10)
	if err != nil {
		t.Fatalf("SearchObservations: %v", err)
	}
	if len(res) != 1 || res[0].ID != "o1" {
		t.Fatalf("esperaba encontrar o1 con procedencia vacía, obtuve %+v", res)
	}
}
