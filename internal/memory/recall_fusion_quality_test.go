package memory

import (
	"context"
	"errors"
	"testing"
)

// TestRecallVectorFloor verifica el piso de coseno del pool vectorial (Q1): los vecinos con
// similitud < VectorFloor se descartan del ranking; VectorFloor <= 0 reproduce el histórico
// (todos entran). Embeddings elegidos para cosenos conocidos vs la query [1,0,0]: hi=1.0,
// mid=0.6, lo=0.0. La query "zzz" no matchea léxico, así que la inclusión la decide el piso.
func TestRecallVectorFloor(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("hi", "t", "alfa", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("mid", "t", "beta", []float32{0.6, 0.8, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("lo", "t", "gamma", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	q := []float32{1, 0, 0}

	// Sin piso (0): los 3 entran por el pool vectorial (union) — comportamiento histórico.
	off, err := e.Recall(context.Background(), "zzz", RecallOptions{QueryVector: q, NoBump: true, VectorFloor: 0})
	if err != nil {
		t.Fatalf("recall sin piso: %v", err)
	}
	for _, id := range []string{"hi", "mid", "lo"} {
		if !has(off.Items, id) {
			t.Errorf("sin piso, %q debería estar presente: %+v", id, off.Items)
		}
	}

	// Con piso 0.5: 'lo' (coseno 0.0) se descarta; 'hi' (1.0) y 'mid' (0.6) sobreviven.
	on, err := e.Recall(context.Background(), "zzz", RecallOptions{QueryVector: q, NoBump: true, VectorFloor: 0.5})
	if err != nil {
		t.Fatalf("recall con piso: %v", err)
	}
	if !has(on.Items, "hi") || !has(on.Items, "mid") {
		t.Errorf("con piso 0.5, 'hi' y 'mid' deben sobrevivir: %+v", on.Items)
	}
	if has(on.Items, "lo") {
		t.Errorf("con piso 0.5, 'lo' (coseno 0) debe descartarse: %+v", on.Items)
	}
}

// TestIsFTSCorruption verifica la clasificación de errores para la degradación elegante (Q2,
// R6/R7): corrupción/malformado → true (el recall degrada); cualquier otro error → false (se
// propaga, para no enmascarar fallos reales).
func TestIsFTSCorruption(t *testing.T) {
	corrupt := []string{
		"database disk image is malformed",
		"fts5: corruption found reading blob 2748779069441",
		"SQLITE_CORRUPT: wrong number of entries in index",
	}
	for _, m := range corrupt {
		if !isFTSCorruption(errors.New(m)) {
			t.Errorf("debería clasificarse como corrupción: %q", m)
		}
	}
	notCorrupt := []error{
		nil,
		errors.New("context canceled"),
		errors.New("no such table: observations_fts"),
		errors.New("disk I/O error"),
	}
	for _, err := range notCorrupt {
		if isFTSCorruption(err) {
			t.Errorf("NO debería clasificarse como corrupción: %v", err)
		}
	}
}

// TestRecallPropagatesNonCorruptionFTSError verifica R7 a nivel Recall: un error de FTS que NO
// es corrupción (aquí, la tabla FTS ausente) se PROPAGA en vez de tragarse silenciosamente.
func TestRecallPropagatesNonCorruptionFTSError(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "hola mundo", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`DROP TABLE observations_fts`); err != nil {
		t.Fatalf("drop fts: %v", err)
	}
	if _, err := e.Recall(context.Background(), "hola", RecallOptions{NoBump: true}); err == nil {
		t.Fatal("un error de FTS no-corrupción (tabla ausente) debe propagarse, no tragarse")
	}
}
