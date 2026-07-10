package memory

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"musubi/internal/config"
	"musubi/internal/logx"
)

// vectorindex.go implementa un índice vectorial IVF (inverted file por centroides
// k-means) model-free y en Go puro, para sacar la búsqueda semántica del full-scan
// O(n) (que cargaba y deserializaba TODOS los embeddings por query y moría a ~10k
// observaciones). Diseño elegido por panel (IVF sobre HNSW/SQ8), "Option-A":
//
//   - NO retiene los vectores en RAM: solo centroides + la membresía de cada celda
//     (ids). Footprint residente ~10-90MB incluso a 1M. La fuente de verdad es
//     SQLite; este índice es un caché RECONSTRUIBLE que solo ACOTA el conjunto
//     candidato. El engine carga los vectores de las celdas sondeadas desde SQLite
//     y hace el ranking final EXACTO (coseno) re-filtrando archived/superseded.
//   - Por eso el índice NUNCA compromete correctness: a lo sumo, staleness entre
//     rebuilds baja el recall (nunca devuelve algo archivado/superseded ni puntúa mal).
//   - Por debajo de ExactThreshold (o sin entrenar / con dim incompatible) la
//     búsqueda es el full-scan exacto de siempre (invariante: exacto bajo el umbral).

// vectorIndexSeed es la semilla fija de k-means en producción: rebuilds
// deterministas (mismos datos => mismos centroides), reproducibles y testeables.
const vectorIndexSeed = 1

// maxCentroids acota la cantidad de celdas IVF (evita sobre-particionar).
const maxCentroids = 4096

// maxSQLParams es el tope conservador de parámetros enlazados por sentencia que
// usamos al trocear un IN(...). SQLite histórico permite 999; modernc puede diferir,
// así que troceamos por debajo para ser seguros en cualquier build.
const maxSQLParams = 900

// idVec asocia el id de una observación con su vector (para entrenar/asignar).
type idVec struct {
	id  string
	vec []float32
}

// ivfIndex es el índice invertido por centroides. Todo su estado mutable está
// protegido por un único RWMutex: Search toma RLock (lecturas concurrentes), y
// Add/Remove/Rebuild toman Lock. El rebuild construye en estructuras locales y hace
// swap atómico bajo Lock, así una query nunca ve un índice a medio construir.
type ivfIndex struct {
	mu        sync.RWMutex
	dim       int            // dimensión entrenada (0 = sin entrenar)
	centroids [][]float32    // centroides normalizados; índice = cellID
	cells     [][]string     // cellID -> ids de observación
	assign    map[string]int // id -> cellID (para Remove/upsert O(1))
	dirty     int            // altas+bajas desde el último rebuild (throttle)
	trained   bool
}

func newIVFIndex() *ivfIndex {
	return &ivfIndex{assign: make(map[string]int)}
}

// Len es la cantidad de vectores indexados (vivos).
func (ix *ivfIndex) Len() int { ix.mu.RLock(); defer ix.mu.RUnlock(); return len(ix.assign) }

// Dim es la dimensión con la que se entrenó el índice (0 si no entrenado).
func (ix *ivfIndex) Dim() int { ix.mu.RLock(); defer ix.mu.RUnlock(); return ix.dim }

// Trained indica si el índice tiene centroides utilizables.
func (ix *ivfIndex) Trained() bool { ix.mu.RLock(); defer ix.mu.RUnlock(); return ix.trained }

// Dirty es la cantidad de altas/bajas desde el último rebuild (dispara re-entrenar).
func (ix *ivfIndex) Dirty() int { ix.mu.RLock(); defer ix.mu.RUnlock(); return ix.dirty }

// seedDirty fija `dirty` al conteo dado mientras el índice NO esté entrenado, para
// que el disparador de entrenamiento (dirty >= ExactThreshold) refleje el conteo
// ABSOLUTO de embeddings y no solo las altas de este proceso. Sin esto, una base que
// cruza el umbral incrementalmente entre reinicios nunca se entrenaría (el contador
// dirty arranca en 0 cada proceso).
func (ix *ivfIndex) seedDirty(n int) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if !ix.trained {
		ix.dirty = n
	}
}

// Add asigna (o reasigna) el vector de una observación a su centroide más cercano.
// Si el índice no está entrenado o la dim no coincide, no lo indexa (el full-scan
// exacto lo cubre) pero cuenta el alta como dirty para el próximo rebuild.
func (ix *ivfIndex) Add(id string, vec []float32) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.dirty++
	if !ix.trained || ix.dim == 0 || len(vec) != ix.dim || len(ix.centroids) == 0 {
		return
	}
	n := normalizeVec(vec)
	c := nearestCentroid(ix.centroids, n)
	if old, ok := ix.assign[id]; ok {
		if old == c {
			return // ya está en la celda correcta
		}
		ix.removeFromCellLocked(id, old)
	}
	ix.cells[c] = append(ix.cells[c], id)
	ix.assign[id] = c
}

// Remove quita una observación del índice (no-op si no está). Mejora la precisión
// del recall (no deja candidatos muertos); la correctitud ya la garantiza el
// re-filtro SQL del engine.
func (ix *ivfIndex) Remove(id string) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if c, ok := ix.assign[id]; ok {
		ix.removeFromCellLocked(id, c)
		delete(ix.assign, id)
		ix.dirty++
	}
}

// removeFromCellLocked saca id de la celda c por swap-remove. Asume Lock tomado.
func (ix *ivfIndex) removeFromCellLocked(id string, c int) {
	if c < 0 || c >= len(ix.cells) {
		return
	}
	cell := ix.cells[c]
	for i, v := range cell {
		if v == id {
			cell[i] = cell[len(cell)-1]
			ix.cells[c] = cell[:len(cell)-1]
			return
		}
	}
}

// RemoveBatch quita un CONJUNTO de observaciones del índice bajo un único Lock, en una
// sola pasada por cada celda afectada. Equivale a llamar Remove(id) por cada id —misma
// semántica, idempotente con ids ausentes o repetidos— pero sin re-tomar el lock ni
// re-barrer la celda entera por cada id: agrupa los ids por celda y filtra cada celda
// tocada UNA vez. Pasa el borrado de lotes de O(borrados × celda) a O(celdas tocadas),
// que es lo que colapsa el O(n²) del mantenimiento (consolidación/decay/purga borran
// lotes grandes de un saque). La correctitud del recall ya la garantiza el re-filtro
// SQL del engine; esto solo mantiene el índice afilado más rápido.
func (ix *ivfIndex) RemoveBatch(ids []string) {
	if len(ids) == 0 {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	// Agrupar los ids PRESENTES por celda (los ausentes se ignoran). El set por celda
	// deduplica ids repetidos, igual que dos Remove sobre el mismo id (el 2º es no-op).
	byCell := make(map[int]map[string]struct{})
	for _, id := range ids {
		c, ok := ix.assign[id]
		if !ok {
			continue
		}
		set := byCell[c]
		if set == nil {
			set = make(map[string]struct{})
			byCell[c] = set
		}
		set[id] = struct{}{}
	}
	for c, set := range byCell {
		if c < 0 || c >= len(ix.cells) {
			continue
		}
		cell := ix.cells[c]
		kept := cell[:0] // filtrado in-place: kept nunca adelanta al índice de lectura
		for _, v := range cell {
			if _, drop := set[v]; drop {
				continue
			}
			kept = append(kept, v)
		}
		for i := len(kept); i < len(cell); i++ {
			cell[i] = "" // liberar referencias a los strings descartados en la cola
		}
		ix.cells[c] = kept
		ix.dirty += len(set)
		for id := range set {
			delete(ix.assign, id)
		}
	}
}

// Search devuelve los ids de las nprobe celdas más cercanas al query (candidatos a
// rankear exactamente por el caller). ok=false si el índice no sirve para esta query
// (sin entrenar o dim incompatible) => el caller cae al full-scan exacto. Los ids se
// copian a un slice nuevo bajo el RLock: el caller nunca toca estructuras internas.
func (ix *ivfIndex) Search(query []float32, nprobe int) ([]string, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	if !ix.trained || ix.dim == 0 || len(query) != ix.dim || len(ix.centroids) == 0 {
		return nil, false
	}
	q := normalizeVec(query)
	type cellSim struct {
		cell int
		sim  float32
	}
	sims := make([]cellSim, len(ix.centroids))
	for i, c := range ix.centroids {
		sims[i] = cellSim{cell: i, sim: dot(q, c)}
	}
	sort.Slice(sims, func(a, b int) bool { return sims[a].sim > sims[b].sim })
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > len(sims) {
		nprobe = len(sims)
	}
	var ids []string
	for i := 0; i < nprobe; i++ {
		ids = append(ids, ix.cells[sims[i].cell]...)
	}
	return ids, true
}

// Rebuild reconstruye el índice desde cero con los vectores provistos: entrena
// centroides con k-means++ sobre una muestra y asigna TODOS los vectores a su celda.
// La dimensión se deriva de la MAYORÍA (drift de modelo: los de otra dim se descartan,
// igual que el skip-with-warn del full-scan). Construye en estructuras locales y hace
// swap atómico bajo Lock para no bloquear las queries más que un reasignar punteros.
func (ix *ivfIndex) Rebuild(data []idVec, cfg config.VectorIndexConfig, seed int64) {
	dim := majorityDim(data)

	var ids []string
	var vecs [][]float32
	if dim > 0 {
		ids = make([]string, 0, len(data))
		vecs = make([][]float32, 0, len(data))
		for _, d := range data {
			if len(d.vec) == dim {
				ids = append(ids, d.id)
				vecs = append(vecs, normalizeVec(d.vec))
			}
		}
	}
	n := len(vecs)

	if n == 0 {
		ix.mu.Lock()
		ix.dim = 0
		ix.centroids = nil
		ix.cells = nil
		ix.assign = make(map[string]int)
		ix.dirty = 0
		ix.trained = false
		ix.mu.Unlock()
		return
	}

	k := targetCentroidCount(n, cfg)

	rng := rand.New(rand.NewSource(seed))

	// Muestra de entrenamiento: si hay muchos vectores, se entrenan los centroides
	// sobre una muestra (más barato) y luego se asigna TODO el conjunto.
	sample := vecs
	if cfg.KMeansSample > 0 && n > cfg.KMeansSample {
		sample = sampleVecs(vecs, cfg.KMeansSample, rng)
	}

	centroids := kmeansPlusPlus(sample, k, rng)
	iters := cfg.KMeansIters
	if iters < 1 {
		iters = 1
	}
	centroids = lloyd(sample, centroids, iters)

	cells := make([][]string, len(centroids))
	assign := make(map[string]int, n)
	for i, v := range vecs {
		c := nearestCentroid(centroids, v)
		cells[c] = append(cells[c], ids[i])
		assign[ids[i]] = c
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

// --- helpers geométricos (model-free, Go puro) ---

// normalizeVec devuelve una copia L2-normalizada de v (norma 0 => copia tal cual,
// para no producir NaN). Sobre vectores normalizados, coseno == producto interno.
func normalizeVec(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	out := make([]float32, len(v))
	if norm == 0 {
		copy(out, v)
		return out
	}
	inv := 1.0 / math.Sqrt(norm)
	for i, x := range v {
		out[i] = float32(float64(x) * inv)
	}
	return out
}

// dot es el producto interno. Con vectores normalizados equivale al coseno.
func dot(a, b []float32) float32 {
	if len(a) != len(b) {
		return -1 // incompatible: peor similitud posible
	}
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return float32(s)
}

// nearestCentroid devuelve el índice del centroide más similar (mayor coseno) a v.
func nearestCentroid(centroids [][]float32, v []float32) int {
	best := 0
	bestSim := float32(-2)
	for i, c := range centroids {
		if s := dot(v, c); s > bestSim {
			bestSim = s
			best = i
		}
	}
	return best
}

// targetCentroidCount es la cantidad de centroides (celdas) apropiada para n vectores:
// cfg.NumCentroids si está fijado, si no round(sqrt(n)); acotada a [1, n, maxCentroids]. La
// comparten Rebuild y el guard del warm-start (T5.8), para que no diverjan.
func targetCentroidCount(n int, cfg config.VectorIndexConfig) int {
	if n <= 0 {
		return 0
	}
	k := cfg.NumCentroids
	if k <= 0 {
		k = int(math.Round(math.Sqrt(float64(n))))
	}
	if k < 1 {
		k = 1
	}
	if k > n {
		k = n
	}
	if k > maxCentroids {
		k = maxCentroids
	}
	return k
}

// majorityDim devuelve la dimensión más frecuente entre los vectores (desempate por
// la dim mayor). Maneja el drift de modelo de embeddings: se entrena con la mayoría.
func majorityDim(data []idVec) int {
	counts := map[int]int{}
	for _, d := range data {
		if len(d.vec) > 0 {
			counts[len(d.vec)]++
		}
	}
	bestDim, bestN := 0, 0
	for dim, n := range counts {
		if n > bestN || (n == bestN && dim > bestDim) {
			bestN = n
			bestDim = dim
		}
	}
	return bestDim
}

// sampleVecs toma m vectores al azar sin reemplazo (muestra de entrenamiento).
func sampleVecs(vecs [][]float32, m int, rng *rand.Rand) [][]float32 {
	n := len(vecs)
	if m >= n {
		return vecs
	}
	idx := rng.Perm(n)[:m]
	out := make([][]float32, m)
	for i, j := range idx {
		out[i] = vecs[j]
	}
	return out
}

// kmeansPlusPlus elige k centroides iniciales con sembrado D² (k-means++): reduce
// celdas vacías/desbalanceadas vs init aleatorio, lo que protege el recall.
func kmeansPlusPlus(data [][]float32, k int, rng *rand.Rand) [][]float32 {
	n := len(data)
	if k > n {
		k = n
	}
	if n == 0 || k == 0 {
		return nil
	}
	centroids := make([][]float32, 0, k)
	centroids = append(centroids, cloneVec(data[rng.Intn(n)]))
	// dist2[i] = distancia² del punto i al centroide más cercano elegido hasta ahora.
	// Con vectores normalizados, d² = 2(1 - coseno).
	dist2 := make([]float64, n)
	for i := range dist2 {
		dist2[i] = math.MaxFloat64
	}
	for len(centroids) < k {
		last := centroids[len(centroids)-1]
		var sum float64
		for i, v := range data {
			if d := 2 * (1 - float64(dot(v, last))); d < dist2[i] {
				dist2[i] = d
			}
			sum += dist2[i]
		}
		if sum == 0 {
			// Todos los puntos coinciden con algún centroide: completar al azar.
			centroids = append(centroids, cloneVec(data[rng.Intn(n)]))
			continue
		}
		target := rng.Float64() * sum
		var acc float64
		chosen := n - 1
		for i := range data {
			acc += dist2[i]
			if acc >= target {
				chosen = i
				break
			}
		}
		centroids = append(centroids, cloneVec(data[chosen]))
	}
	return centroids
}

// lloyd corre iteraciones de Lloyd sobre los datos: reasigna cada punto a su
// centroide más cercano y recomputa el centroide como la media normalizada de su
// celda. Un centroide muerto (sin puntos) se re-siembra en el punto más lejano.
func lloyd(data, centroids [][]float32, iters int) [][]float32 {
	k := len(centroids)
	if k == 0 || len(data) == 0 {
		return centroids
	}
	dim := len(data[0])
	for it := 0; it < iters; it++ {
		sums := make([][]float64, k)
		counts := make([]int, k)
		for i := range sums {
			sums[i] = make([]float64, dim)
		}
		for _, v := range data {
			c := nearestCentroid(centroids, v)
			counts[c]++
			for d := 0; d < dim; d++ {
				sums[c][d] += float64(v[d])
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				centroids[c] = reseedDeadCentroid(data, centroids)
				continue
			}
			mean := make([]float32, dim)
			inv := 1.0 / float64(counts[c])
			for d := 0; d < dim; d++ {
				mean[d] = float32(sums[c][d] * inv)
			}
			centroids[c] = normalizeVec(mean)
		}
	}
	return centroids
}

// reseedDeadCentroid devuelve el punto peor cubierto (menor similitud a su centroide)
// para revivir un centroide vacío.
func reseedDeadCentroid(data, centroids [][]float32) []float32 {
	worst := 0
	worstSim := float32(2)
	for i, v := range data {
		c := nearestCentroid(centroids, v)
		if s := dot(v, centroids[c]); s < worstSim {
			worstSim = s
			worst = i
		}
	}
	return cloneVec(data[worst])
}

func cloneVec(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	return out
}

// chunkStrings trocea s en sub-slices de a lo sumo size elementos (para IN(...)).
func chunkStrings(s []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	var out [][]string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

// --- integración con el engine (SQLite es la fuente de verdad) ---

const metaVectorIndexRebuild = "vector_index_last_rebuild"

// loadActiveVectors lee de SQLite todos los (id, vector) de observaciones activas
// (no archivadas, no superseded) con embedding. Es la fuente para (re)entrenar.
func (e *DbEngine) loadActiveVectors() ([]idVec, error) {
	rows, err := e.db.Query(`
		SELECT o.id, em.vector
		FROM observations o
		JOIN embeddings em ON o.id = em.observation_id
		WHERE ` + visibleObsPredicate + `
	`)
	if err != nil {
		return nil, fmt.Errorf("error al cargar vectores activos: %w", err)
	}
	defer rows.Close()
	var out []idVec
	for rows.Next() {
		var id string
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return nil, fmt.Errorf("error al escanear vector: %w", err)
		}
		v, err := BytesToFloat32(b)
		if err != nil {
			logx.Warn("vector ilegible omitido al construir el índice", "id", id, "error", err)
			continue
		}
		out = append(out, idVec{id: id, vec: v})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar vectores activos: %w", err)
	}
	return out, nil
}

// rebuildVectorIndex reconstruye el índice IVF desde los embeddings activos (síncrono),
// con la config del engine. Solo desde la goroutine dueña del engine (NewDbEngine, tests).
func (e *DbEngine) rebuildVectorIndex() error {
	return e.rebuildVectorIndexWith(e.vindexCfg)
}

// rebuildVectorIndexWith reconstruye el índice con una config dada. Las goroutines de
// fondo reciben una copia (snapshot) y usan esta variante para NO leer e.vindexCfg de
// forma concurrente con un posible ajuste de config en la goroutine dueña.
func (e *DbEngine) rebuildVectorIndexWith(cfg config.VectorIndexConfig) error {
	if e.index == nil {
		return nil
	}
	data, err := e.loadActiveVectors()
	if err != nil {
		return err
	}
	e.index.Rebuild(data, cfg, vectorIndexSeed)
	e.saveVectorSnapshot() // persistir centroides para arranque caliente (T5.8)
	return nil
}

// countActiveEmbeddings cuenta las observaciones activas con embedding.
func (e *DbEngine) countActiveEmbeddings() (int, error) {
	return e.countActiveEmbeddingsCtx(context.Background())
}

// countActiveEmbeddingsCtx es countActiveEmbeddings acotable por contexto: /metrics le pasa un
// deadline para que este COUNT O(n) no cuelgue el scrape si la base está lenta (T17.5).
func (e *DbEngine) countActiveEmbeddingsCtx(ctx context.Context) (int, error) {
	var n int
	err := e.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM observations o
		JOIN embeddings em ON o.id = em.observation_id
		WHERE `+visibleObsPredicate+`
	`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error al contar embeddings activos: %w", err)
	}
	return n, nil
}

// autoBuildVectorIndex entrena el índice al arrancar si ya hay suficientes embeddings
// (>= ExactThreshold). Recibe la config por copia (snapshot) para no leer e.vindexCfg
// desde esta goroutine de fondo. Si todavía no alcanza el umbral, siembra `dirty` con
// el conteo absoluto para que un crecimiento incremental posterior dispare el train.
func (e *DbEngine) autoBuildVectorIndex(cfg config.VectorIndexConfig) {
	if e.index == nil {
		return
	}
	n, err := e.countActiveEmbeddings()
	if err != nil {
		logx.Warn("no se pudo contar embeddings al arrancar el índice vectorial", "error", err)
		return
	}
	if n < cfg.ExactThreshold {
		e.index.seedDirty(n)
		return
	}
	if !e.rebuilding.CompareAndSwap(false, true) {
		return
	}
	defer e.rebuilding.Store(false)
	// Arranque caliente (T5.8): si hay un snapshot válido, restaurar los centroides y
	// reasignar (salta el k-means). Ante cualquier problema, re-entrenar de cero.
	if e.tryWarmStartVectorIndex(cfg) {
		_ = e.MarkMetaNow(metaVectorIndexRebuild)
		return
	}
	if err := e.rebuildVectorIndexWith(cfg); err != nil {
		logx.Warn("no se pudo construir el índice vectorial al arrancar", "error", err)
		return
	}
	_ = e.MarkMetaNow(metaVectorIndexRebuild)
}

// maybeRebuildVectorIndex re-entrena el índice cuando hace falta: porque cruzó el
// umbral y aún no está entrenado (build proactivo, sin esperar a la primera query),
// o porque acumuló suficientes cambios (dirty) y pasó el piso temporal. Corre en
// segundo plano con guard atómico para no demorar el path de escritura.
func (e *DbEngine) maybeRebuildVectorIndex() {
	if e.index == nil || !e.vindexCfg.Enabled {
		return
	}
	cfg := e.vindexCfg // snapshot en la goroutine del caller, para pasar a la de fondo
	trained := e.index.Trained()
	dirty := e.index.Dirty()
	needTrain := !trained && dirty >= cfg.ExactThreshold
	needRebuild := trained && dirty >= cfg.RebuildEvery
	if !needTrain && !needRebuild {
		return
	}
	if needRebuild {
		due, err := e.MetaDue(metaVectorIndexRebuild, cfg.RebuildMinHours)
		if err != nil || !due {
			return
		}
	}
	if !e.rebuilding.CompareAndSwap(false, true) {
		return
	}
	launched := e.spawnBackground(func() {
		defer e.rebuilding.Store(false)
		if err := e.rebuildVectorIndexWith(cfg); err != nil {
			logx.Warn("rebuild del índice vectorial falló", "error", err)
			return
		}
		_ = e.MarkMetaNow(metaVectorIndexRebuild)
	})
	if !launched {
		// El engine se está cerrando: liberar el guard que tomamos recién.
		e.rebuilding.Store(false)
	}
}
