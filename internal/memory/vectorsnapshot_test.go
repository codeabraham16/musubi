package memory

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestIndexSnapshotRoundTrip verifica que escribir y leer el snapshot preserva dim y
// centroides exactamente (T5.8).
func TestIndexSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.vindex")
	dim := 4
	centroids := [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}

	if err := writeIndexSnapshot(path, dim, centroids); err != nil {
		t.Fatalf("write: %v", err)
	}
	gotDim, gotC, err := readIndexSnapshot(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if gotDim != dim {
		t.Errorf("dim %d != %d", gotDim, dim)
	}
	if len(gotC) != len(centroids) {
		t.Fatalf("k %d != %d", len(gotC), len(centroids))
	}
	for i := range centroids {
		for j := range centroids[i] {
			if gotC[i][j] != centroids[i][j] {
				t.Errorf("centroide[%d][%d] %v != %v", i, j, gotC[i][j], centroids[i][j])
			}
		}
	}
}

// TestReadIndexSnapshotRejectsCorrupt verifica que un archivo ausente, con magic inválido o
// truncado da error (el caller cae al rebuild en vez de cargar basura).
func TestReadIndexSnapshotRejectsCorrupt(t *testing.T) {
	dir := t.TempDir()

	if _, _, err := readIndexSnapshot(filepath.Join(dir, "missing.vindex")); err == nil {
		t.Error("archivo ausente debe dar error")
	}

	bad := filepath.Join(dir, "bad.vindex")
	if err := os.WriteFile(bad, []byte("NOPE-not-a-snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readIndexSnapshot(bad); err == nil {
		t.Error("magic inválido debe dar error")
	}

	trunc := filepath.Join(dir, "trunc.vindex")
	var buf bytes.Buffer
	buf.WriteString(vindexSnapshotMagic)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(4)) // dim
	_ = binary.Write(&buf, binary.LittleEndian, uint32(3)) // k=3 pero sin centroides
	if err := os.WriteFile(trunc, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readIndexSnapshot(trunc); err == nil {
		t.Error("centroides truncados deben dar error")
	}
}

// TestVectorIndexWarmStart verifica el arranque caliente end-to-end (T5.8): tras un rebuild
// el snapshot existe, y restaurar desde él (índice fresco) da los MISMOS resultados ANN que
// el índice entrenado por k-means (mismos centroides ⇒ misma asignación).
func TestVectorIndexWarmStart(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	const (
		n   = 300
		dim = 16
	)
	data := clusteredDataset(6, n, dim, 12)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "contenido "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	e.vindexCfg.ExactThreshold = 1
	e.vindexCfg.NProbe = 8
	if err := e.rebuildVectorIndex(); err != nil { // entrena (k-means) + guarda snapshot
		t.Fatalf("rebuild: %v", err)
	}
	if !e.index.Trained() {
		t.Fatal("el índice debería estar entrenado tras el rebuild")
	}
	if _, err := os.Stat(e.vectorSnapshotPath()); err != nil {
		t.Fatalf("el snapshot debería existir tras el rebuild: %v", err)
	}

	ctx := context.Background()
	query := data[0].vec
	ref, err := e.SearchObservations(ctx, query, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := resultIDs(ref)

	// Simular reinicio: índice fresco, restaurado SOLO desde el snapshot (sin k-means).
	e.index = newIVFIndex()
	if !e.tryWarmStartVectorIndex(e.vindexCfg) {
		t.Fatal("el warm-start debería tener éxito con un snapshot válido")
	}
	if !e.index.Trained() {
		t.Fatal("el índice restaurado debería quedar entrenado")
	}
	got2, err := e.SearchObservations(ctx, query, 10)
	if err != nil {
		t.Fatal(err)
	}
	got := resultIDs(got2)
	if len(want) == 0 || overlap(want, got) != len(want) {
		t.Errorf("el warm-start debería dar los mismos resultados que el rebuild:\n want %v\n got  %v", want, got)
	}
}

// TestVectorIndexWarmStartDimMismatch verifica el fallback por drift de modelo: si el
// snapshot tiene otra dimensión que los embeddings actuales, el warm-start falla (false) y
// el caller re-entrena.
func TestVectorIndexWarmStartDimMismatch(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	const dim = 8
	data := clusteredDataset(4, 60, dim, 12)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "c "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	// Snapshot con OTRA dimensión (16) que la de los embeddings (8): drift simulado.
	bogus := [][]float32{make([]float32, 16), make([]float32, 16)}
	bogus[0][0], bogus[1][1] = 1, 1
	if err := writeIndexSnapshot(e.vectorSnapshotPath(), 16, bogus); err != nil {
		t.Fatal(err)
	}
	if e.tryWarmStartVectorIndex(e.vindexCfg) {
		t.Error("con dim distinta el warm-start debe fallar (fallback a rebuild)")
	}
}

// TestVectorIndexWarmStartRejectsStaleK verifica el guard de k (fix de la revisión adversarial
// de T5.8): un snapshot con muchos más centroides que los naturales para el n actual (dataset
// que se encogió entre sesiones) se descarta para no degradar el recall con NProbe fijo.
func TestVectorIndexWarmStartRejectsStaleK(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	const dim = 8
	data := clusteredDataset(3, 40, dim, 12) // n actual pequeño => k natural ~6
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "c "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	// Snapshot con 200 centroides (como si se hubiera entrenado con n enorme).
	const staleK = 200
	centroids := make([][]float32, staleK)
	for i := range centroids {
		centroids[i] = make([]float32, dim)
		centroids[i][i%dim] = 1
	}
	if err := writeIndexSnapshot(e.vectorSnapshotPath(), dim, centroids); err != nil {
		t.Fatal(err)
	}
	if e.tryWarmStartVectorIndex(e.vindexCfg) {
		t.Error("un snapshot con k incompatible (200 celdas para 40 vectores) debe descartarse")
	}
}
