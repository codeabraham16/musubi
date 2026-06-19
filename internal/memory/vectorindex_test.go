package memory

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"musubi/internal/config"
)

// --- helpers de test ---

// clusteredDataset genera n vectores de dimensión dim agrupados en `clusters`
// centros (datos realistas para ANN), de forma determinista vía seed.
func clusteredDataset(seed int64, n, dim, clusters int) []idVec {
	rng := rand.New(rand.NewSource(seed))
	centers := make([][]float32, clusters)
	for c := range centers {
		centers[c] = randomVec(rng, dim)
	}
	data := make([]idVec, n)
	for i := 0; i < n; i++ {
		c := centers[rng.Intn(clusters)]
		v := make([]float32, dim)
		for d := 0; d < dim; d++ {
			v[d] = c[d] + float32(rng.NormFloat64()*0.15) // ruido alrededor del centro
		}
		data[i] = idVec{id: idForIndex(i), vec: v}
	}
	return data
}

func randomVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for d := 0; d < dim; d++ {
		v[d] = float32(rng.NormFloat64())
	}
	return v
}

func idForIndex(i int) string {
	return "obs-" + string(rune('a'+i%26)) + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// exactTopK calcula el top-k EXACTO por coseno sobre todos los vectores.
func exactTopK(query []float32, all map[string][]float32, k int) []string {
	type sc struct {
		id  string
		sim float32
	}
	scored := make([]sc, 0, len(all))
	for id, v := range all {
		s, _ := CosineSimilarity(query, v)
		scored = append(scored, sc{id, s})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].sim > scored[j].sim })
	out := make([]string, 0, k)
	for i := 0; i < k && i < len(scored); i++ {
		out = append(out, scored[i].id)
	}
	return out
}

// annTopK usa el índice IVF para acotar candidatos y los rankea exacto (igual que
// searchExactByIDs), devolviendo el top-k aproximado.
func annTopK(ix *ivfIndex, query []float32, all map[string][]float32, nprobe, k int) []string {
	ids, ok := ix.Search(query, nprobe)
	if !ok {
		return nil
	}
	type sc struct {
		id  string
		sim float32
	}
	scored := make([]sc, 0, len(ids))
	for _, id := range ids {
		s, _ := CosineSimilarity(query, all[id])
		scored = append(scored, sc{id, s})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].sim > scored[j].sim })
	out := make([]string, 0, k)
	for i := 0; i < k && i < len(scored); i++ {
		out = append(out, scored[i].id)
	}
	return out
}

func overlap(a, b []string) int {
	set := make(map[string]bool, len(a))
	for _, x := range a {
		set[x] = true
	}
	n := 0
	for _, x := range b {
		if set[x] {
			n++
		}
	}
	return n
}

// --- tests ---

// TestIVFRecallGate es el GUARDARRAÍL central: una regresión de recall en ANN es
// silenciosa (no crashea, simplemente "no encuentra"). Compara el top-10 del IVF
// contra el top-10 EXACTO sobre el mismo dataset y exige recall@10 >= 0.92.
func TestIVFRecallGate(t *testing.T) {
	const (
		n        = 4000
		dim      = 64
		clusters = 30
		k        = 10
		nprobe   = 12
		queries  = 200
	)
	data := clusteredDataset(1, n, dim, clusters)
	all := make(map[string][]float32, n)
	for _, d := range data {
		all[d.id] = d.vec
	}

	ix := newIVFIndex()
	cfg := config.Default().VectorIndex
	ix.Rebuild(data, cfg, vectorIndexSeed)
	if !ix.Trained() {
		t.Fatal("el índice debería quedar entrenado")
	}
	if ix.Len() != n {
		t.Fatalf("Len=%d, esperaba %d", ix.Len(), n)
	}

	rng := rand.New(rand.NewSource(99))
	var totalRecall float64
	for q := 0; q < queries; q++ {
		// query = un punto del dataset perturbado (consulta realista)
		base := data[rng.Intn(n)].vec
		query := make([]float32, dim)
		for d := 0; d < dim; d++ {
			query[d] = base[d] + float32(rng.NormFloat64()*0.05)
		}
		want := exactTopK(query, all, k)
		got := annTopK(ix, query, all, nprobe, k)
		totalRecall += float64(overlap(want, got)) / float64(k)
	}
	recall := totalRecall / float64(queries)
	if recall < 0.92 {
		t.Fatalf("recall@%d = %.3f < 0.92 (GATE): el ANN perdió calidad", k, recall)
	}
	t.Logf("recall@%d = %.3f (nprobe=%d, N=%d, centroides≈%d)", k, recall, nprobe, n, int(math.Round(math.Sqrt(float64(n)))))
}

// TestIVFExactWhenUntrainedFallsBack verifica que un índice sin entrenar devuelve
// ok=false en Search (el caller cae al full-scan exacto).
func TestIVFExactWhenUntrainedFallsBack(t *testing.T) {
	ix := newIVFIndex()
	if _, ok := ix.Search(randomVec(rand.New(rand.NewSource(1)), 32), 8); ok {
		t.Error("un índice sin entrenar no debería servir candidatos (ok=true)")
	}
}

// TestIVFDimDriftUsesMajority verifica que con dimensiones mezcladas, el índice se
// entrena con la dim mayoritaria y descarta la minoritaria.
func TestIVFDimDriftUsesMajority(t *testing.T) {
	data := clusteredDataset(2, 300, 48, 10) // mayoría dim 48
	// agregar 20 vectores de otra dim (minoría)
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 20; i++ {
		data = append(data, idVec{id: "other-" + itoa(i), vec: randomVec(rng, 16)})
	}
	ix := newIVFIndex()
	ix.Rebuild(data, config.Default().VectorIndex, vectorIndexSeed)
	if ix.Dim() != 48 {
		t.Errorf("Dim=%d, esperaba la mayoría 48", ix.Dim())
	}
	if ix.Len() != 300 {
		t.Errorf("Len=%d, esperaba 300 (los 20 de dim 16 se descartan)", ix.Len())
	}
}

// TestIVFUpsertAndRemove verifica que Add reasigna (no duplica) y Remove saca el id.
func TestIVFUpsertAndRemove(t *testing.T) {
	data := clusteredDataset(3, 500, 32, 12)
	ix := newIVFIndex()
	ix.Rebuild(data, config.Default().VectorIndex, vectorIndexSeed)
	start := ix.Len()

	// Re-Add un id existente con otro vector: el Len no cambia (upsert).
	ix.Add(data[0].id, randomVec(rand.New(rand.NewSource(5)), 32))
	if ix.Len() != start {
		t.Errorf("tras upsert Len=%d, esperaba %d (no debe duplicar)", ix.Len(), start)
	}
	// Remove baja el Len en 1 y no aparece en ninguna celda.
	ix.Remove(data[0].id)
	if ix.Len() != start-1 {
		t.Errorf("tras Remove Len=%d, esperaba %d", ix.Len(), start-1)
	}
	ix.mu.RLock()
	for _, cell := range ix.cells {
		for _, id := range cell {
			if id == data[0].id {
				ix.mu.RUnlock()
				t.Fatalf("el id removido sigue en una celda")
			}
		}
	}
	ix.mu.RUnlock()
}

// TestIVFConcurrentRace ejercita Search + Add + Remove + Rebuild en paralelo para
// que `go test -race` detecte cualquier acceso sin sincronizar.
func TestIVFConcurrentRace(t *testing.T) {
	data := clusteredDataset(4, 1000, 32, 15)
	all := make(map[string][]float32, len(data))
	for _, d := range data {
		all[d.id] = d.vec
	}
	ix := newIVFIndex()
	cfg := config.Default().VectorIndex
	ix.Rebuild(data, cfg, vectorIndexSeed)

	var wg sync.WaitGroup
	// lectores
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(s)))
			for i := 0; i < 200; i++ {
				ix.Search(randomVec(rng, 32), 8)
			}
		}(g)
	}
	// escritores
	wg.Add(1)
	go func() {
		defer wg.Done()
		rng := rand.New(rand.NewSource(123))
		for i := 0; i < 200; i++ {
			ix.Add("new-"+itoa(i), randomVec(rng, 32))
			ix.Remove(data[i%len(data)].id)
		}
	}()
	// rebuild concurrente
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			ix.Rebuild(data, cfg, vectorIndexSeed)
		}
	}()
	wg.Wait()
}

// TestSearchObservationsIVFMatchesExact es el test de INTEGRACIÓN end-to-end: con un
// engine real y el umbral bajado para forzar ANN, la ruta IVF de SearchObservations
// devuelve esencialmente el mismo top-k que el full-scan exacto.
func TestSearchObservationsIVFMatchesExact(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	const (
		n      = 600
		dim    = 32
		k      = 10
		nprobe = 16
	)
	data := clusteredDataset(8, n, dim, 12)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "contenido "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}

	// Forzar ANN: umbral bajo + nprobe alto, y entrenar el índice sincrónicamente.
	e.vindexCfg.ExactThreshold = 1
	e.vindexCfg.NProbe = nprobe
	if err := e.rebuildVectorIndex(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !e.index.Trained() {
		t.Fatal("el índice del engine debería estar entrenado")
	}

	ctx := context.Background()
	rng := rand.New(rand.NewSource(55))
	var totalRecall float64
	const queries = 60
	for q := 0; q < queries; q++ {
		base := data[rng.Intn(n)].vec
		query := make([]float32, dim)
		for d := 0; d < dim; d++ {
			query[d] = base[d] + float32(rng.NormFloat64()*0.05)
		}
		// ruta exacta (full-scan) como verdad de referencia
		exact, err := e.searchExactFullScan(ctx, query, k)
		if err != nil {
			t.Fatal(err)
		}
		// ruta ANN (dispatch de SearchObservations)
		ann, err := e.SearchObservations(ctx, query, k)
		if err != nil {
			t.Fatal(err)
		}
		wantIDs := resultIDs(exact)
		gotIDs := resultIDs(ann)
		totalRecall += float64(overlap(wantIDs, gotIDs)) / float64(len(wantIDs))
	}
	recall := totalRecall / float64(queries)
	if recall < 0.92 {
		t.Fatalf("recall@%d end-to-end = %.3f < 0.92", k, recall)
	}
	t.Logf("recall@%d end-to-end (engine) = %.3f", k, recall)
}

// TestSearchObservationsArchivedExcludedFromIVF verifica que una observación
// archivada NO aparece en los resultados ANN (re-filtro SQL + Remove del índice).
func TestSearchObservationsArchivedExcludedFromIVF(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	dim := 16
	data := clusteredDataset(11, 200, dim, 6)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "c "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	e.vindexCfg.ExactThreshold = 1
	e.vindexCfg.NProbe = 32 // sondear casi todo para que el candidato sí esté
	if err := e.rebuildVectorIndex(); err != nil {
		t.Fatal(err)
	}

	target := data[0].id
	// archivar el target directamente y removerlo del índice (como hace Decay)
	if _, err := e.db.Exec(`UPDATE observations SET archived=1 WHERE id=?`, target); err != nil {
		t.Fatal(err)
	}
	e.index.Remove(target)

	// Buscar usando el propio vector del target: no debe volver (está archivado).
	res, err := e.SearchObservations(context.Background(), data[0].vec, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.ID == target {
			t.Fatalf("la observación archivada %q no debería aparecer en resultados", target)
		}
	}
}

// TestChunkStrings verifica el troceo del IN(...) para el límite de parámetros.
func TestChunkStrings(t *testing.T) {
	in := make([]string, 0, 2050)
	for i := 0; i < 2050; i++ {
		in = append(in, itoa(i))
	}
	chunks := chunkStrings(in, maxSQLParams)
	total := 0
	for _, c := range chunks {
		if len(c) > maxSQLParams {
			t.Fatalf("chunk de %d > %d", len(c), maxSQLParams)
		}
		total += len(c)
	}
	if total != len(in) {
		t.Errorf("se perdieron elementos al trocear: %d != %d", total, len(in))
	}
}

func resultIDs(rs []SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}
