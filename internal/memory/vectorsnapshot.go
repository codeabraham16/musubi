package memory

// vectorsnapshot.go implementa la persistencia del índice IVF (Track 5 / T5.8): un snapshot
// binario de los centroides entrenados en <db>.vindex, para arrancar en CALIENTE sin re-correr
// k-means. El snapshot guarda SOLO los centroides (no el map id→celda): al cargar se reasignan
// los vectores activos a su centroide más cercano (O(n·k), barato), saltando el entrenamiento
// (O(n·k·iters), lo caro). El .vindex es un caché derivado y reconstruible: ante cualquier
// problema (ausente, corrupto, dim distinta por drift de modelo) se cae al rebuild normal.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"musubi/internal/config"
	"musubi/internal/logx"
)

// vindexSnapshotMagic identifica el formato y su versión. Si el formato cambia, bumpear el
// dígito invalida snapshots viejos (magic mismatch ⇒ rebuild), sin migración.
const vindexSnapshotMagic = "MIVF1\n"

// snapshot devuelve la dim y una copia de los centroides entrenados; ok=false si el índice
// no está entrenado. Copia bajo RLock: el caller nunca toca estructuras internas.
func (ix *ivfIndex) snapshot() (dim int, centroids [][]float32, ok bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	if !ix.trained || ix.dim == 0 || len(ix.centroids) == 0 {
		return 0, nil, false
	}
	cs := make([][]float32, len(ix.centroids))
	for i, c := range ix.centroids {
		cs[i] = cloneVec(c)
	}
	return ix.dim, cs, true
}

// loadFromCentroids restaura el índice desde centroides ya entrenados (de un snapshot),
// asignando data a su centroide más cercano SIN re-entrenar k-means. Descarta vectores cuya
// dim no coincida. Swap atómico bajo Lock, igual que Rebuild.
//
// Nota (race aceptada, idéntica a Rebuild): la asignación se computa sobre `data` fuera del
// Lock y `dirty` se resetea a 0 en el swap. Un Add/Remove que llegue en esa ventana puede
// perder su incremento de `dirty` y no quedar reflejado; es benigno (no compromete correctness:
// el engine re-filtra/re-rankea exacto) y auto-recuperable (el próximo Add/Remove vuelve a
// marcar dirty y dispara maybeRebuildVectorIndex).
func (ix *ivfIndex) loadFromCentroids(dim int, centroids [][]float32, data []idVec) {
	cells := make([][]string, len(centroids))
	assign := make(map[string]int, len(data))
	for _, d := range data {
		if len(d.vec) != dim {
			continue
		}
		c := nearestCentroid(centroids, normalizeVec(d.vec))
		cells[c] = append(cells[c], d.id)
		assign[d.id] = c
	}
	ix.mu.Lock()
	ix.dim = dim
	ix.centroids = centroids
	ix.cells = cells
	ix.assign = assign
	ix.dirty = 0
	ix.trained = true
	ix.mu.Unlock()
}

// writeIndexSnapshot serializa dim + centroides a path (binario little-endian). Escritura
// ATÓMICA (tmp + rename) para que un crash a mitad nunca deje un .vindex truncado: el path
// final siempre tiene un snapshot completo o el anterior. En Windows os.Rename reemplaza el
// destino (MoveFileEx). El lector igual tolera corrupción (cae a rebuild) como segunda red.
func writeIndexSnapshot(path string, dim int, centroids [][]float32) error {
	var buf bytes.Buffer
	buf.WriteString(vindexSnapshotMagic)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(dim))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(centroids)))
	for _, c := range centroids {
		if len(c) != dim {
			return fmt.Errorf("snapshot de índice: centroide de dim %d, esperaba %d", len(c), dim)
		}
		if err := binary.Write(&buf, binary.LittleEndian, c); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// readIndexSnapshot deserializa dim + centroides; error si falta, magic inválido, está
// truncado o las dimensiones están fuera de rango (defensa contra archivos corruptos).
func readIndexSnapshot(path string) (int, [][]float32, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	r := bytes.NewReader(raw)
	magic := make([]byte, len(vindexSnapshotMagic))
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != vindexSnapshotMagic {
		return 0, nil, fmt.Errorf("snapshot de índice: magic inválido")
	}
	var dim32, k32 uint32
	if err := binary.Read(r, binary.LittleEndian, &dim32); err != nil {
		return 0, nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &k32); err != nil {
		return 0, nil, err
	}
	dim, k := int(dim32), int(k32)
	// Cotas de cordura: un archivo corrupto no debe disparar una asignación gigante.
	if dim <= 0 || dim > 100000 || k <= 0 || k > 1000000 {
		return 0, nil, fmt.Errorf("snapshot de índice: dim=%d k=%d fuera de rango", dim, k)
	}
	centroids := make([][]float32, k)
	for i := 0; i < k; i++ {
		c := make([]float32, dim)
		if err := binary.Read(r, binary.LittleEndian, c); err != nil {
			return 0, nil, fmt.Errorf("snapshot de índice: centroides truncados: %w", err)
		}
		centroids[i] = c
	}
	return dim, centroids, nil
}

// vectorSnapshotPath es la ruta del snapshot, junto al archivo SQLite. Vacío si la base no
// tiene archivo (en memoria): ahí no tiene sentido persistir un caché.
func (e *DbEngine) vectorSnapshotPath() string {
	if e.path == "" || strings.Contains(e.path, ":memory:") {
		return ""
	}
	return e.path + ".vindex"
}

// saveVectorSnapshot persiste los centroides actuales (best-effort). La llama el rebuild tras
// entrenar; un fallo se loguea y no afecta el índice en memoria.
func (e *DbEngine) saveVectorSnapshot() {
	path := e.vectorSnapshotPath()
	if path == "" || e.index == nil {
		return
	}
	dim, centroids, ok := e.index.snapshot()
	if !ok {
		return
	}
	if err := writeIndexSnapshot(path, dim, centroids); err != nil {
		logx.Warn("no se pudo guardar el snapshot del índice vectorial", "error", err)
	}
}

// tryWarmStartVectorIndex intenta restaurar el índice desde el snapshot, saltando k-means.
// Devuelve true si lo logró. Cae a false (⇒ el caller re-entrena de cero) ante cualquier
// problema: sin snapshot, corrupto, dim distinta a la de los embeddings actuales (drift de
// modelo), o cantidad de centroides incompatible con el n actual (el dataset cambió mucho de
// tamaño entre sesiones; mantener centroides viejos degradaría el recall con NProbe fijo).
//
// Limitación conocida: NO detecta un cambio de modelo de embeddings de la MISMA dimensión
// (no hay fingerprint del modelo en el snapshot; agregarlo cruzaría la capa "model-free" del
// motor). En ese caso el .vindex stale se reemplaza en el próximo rebuild programado; nunca
// compromete correctness (el engine re-rankea exacto), a lo sumo baja transitoriamente el recall.
func (e *DbEngine) tryWarmStartVectorIndex(cfg config.VectorIndexConfig) bool {
	path := e.vectorSnapshotPath()
	if path == "" || e.index == nil {
		return false
	}
	dim, centroids, err := readIndexSnapshot(path)
	if err != nil {
		return false
	}
	data, err := e.loadActiveVectors()
	if err != nil {
		return false
	}
	if majorityDim(data) != dim {
		return false // drift de modelo (otra dimensión): re-entrenar
	}
	// Guard de k: si la cantidad de centroides del snapshot diverge mucho de la natural para
	// el n actual, el dataset cambió de tamaño sustancialmente entre sesiones. Reusar esos
	// centroides degradaría el recall (demasiadas/pocas celdas para NProbe): mejor re-entrenar.
	matchingN := 0
	for _, d := range data {
		if len(d.vec) == dim {
			matchingN++
		}
	}
	expectedK := targetCentroidCount(matchingN, cfg)
	loadedK := len(centroids)
	if expectedK > 0 && (loadedK > 2*expectedK || 2*loadedK < expectedK) {
		logx.Info("snapshot del índice vectorial descartado: k incompatible con el dataset actual",
			"snapshot_k", loadedK, "expected_k", expectedK, "n", matchingN)
		return false
	}
	e.index.loadFromCentroids(dim, centroids, data)
	logx.Info("índice vectorial restaurado desde snapshot (arranque caliente)",
		"dim", dim, "centroids", len(centroids), "vectors", len(data))
	return true
}
