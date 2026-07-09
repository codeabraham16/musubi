package memory

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
)

// bench_test.go — harness de benchmarks de ESCALA (Track 7 / T7.1). Mide cómo escala el
// motor de memoria (save, recall léxico e híbrido, FTS, mantenimiento) al crecer el número
// de observaciones. Genera datasets sintéticos DETERMINISTAS (seed fija) sin red ni
// embeddings reales (vectores pseudo-aleatorios normalizados). Model-free y sin deps
// nuevas: solo stdlib (testing, math, math/rand). Correr con:
//
//	go test -run='^$' -bench=. -benchmem ./internal/memory/

const benchVecDim = 128

// benchVocab es un vocabulario acotado para generar contenido con solapamiento real
// (así el FTS y el recall tienen candidatos que rankear, no strings únicos sin overlap).
var benchVocab = []string{
	"config", "memory", "recall", "vector", "index", "skill", "hook", "daemon",
	"observation", "telemetry", "sqlite", "embedding", "token", "budget", "consolidate",
	"decay", "schema", "migration", "concurrency", "dispatch", "mutex", "fingerprint",
	"profile", "stack", "detector", "bootstrap", "orchestrate", "agent", "pipeline", "graph",
}

// benchContent arma un (topic_key, content) sintético variado pero determinista.
func benchContent(r *rand.Rand, n int) (string, string) {
	topic := fmt.Sprintf("bench/topic-%d", n%50)
	var b strings.Builder
	nwords := 12 + r.Intn(20)
	for i := 0; i < nwords; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(benchVocab[r.Intn(len(benchVocab))])
	}
	return topic, b.String()
}

// benchVector genera un vector determinista normalizado (norma L2 = 1) de benchVecDim.
func benchVector(r *rand.Rand) []float32 {
	v := make([]float32, benchVecDim)
	var norm float64
	for i := range v {
		v[i] = float32(r.NormFloat64())
		norm += float64(v[i]) * float64(v[i])
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

// benchSeedEngine crea un DbEngine temporal poblado con n observaciones sintéticas.
// Si withVectors es true, cada una lleva un embedding determinista (activa el IVF).
func benchSeedEngine(tb testing.TB, n int, withVectors bool) *DbEngine {
	tb.Helper()
	eng, err := NewDbEngine(tb.TempDir())
	if err != nil {
		tb.Fatalf("NewDbEngine: %v", err)
	}
	tb.Cleanup(func() { _ = eng.Close() })
	r := rand.New(rand.NewSource(42))
	for i := 0; i < n; i++ {
		topic, content := benchContent(r, i)
		var vec []float32
		if withVectors {
			vec = benchVector(r)
		}
		if err := eng.SaveObservation(fmt.Sprintf("obs-%d", i), topic, content, vec); err != nil {
			tb.Fatalf("SaveObservation %d: %v", i, err)
		}
	}
	return eng
}

const benchQuery = "memory recall vector index token budget"

// BenchmarkSaveObservation mide el costo de guardar una observación (UPSERT + FTS + gist).
func BenchmarkSaveObservation(b *testing.B) {
	for _, withVec := range []bool{false, true} {
		name := "lexico"
		if withVec {
			name = "con-vector"
		}
		b.Run(name, func(b *testing.B) {
			eng, err := NewDbEngine(b.TempDir())
			if err != nil {
				b.Fatal(err)
			}
			defer eng.Close()
			r := rand.New(rand.NewSource(1))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				topic, content := benchContent(r, i)
				var vec []float32
				if withVec {
					vec = benchVector(r)
				}
				if err := eng.SaveObservation(fmt.Sprintf("o-%d", i), topic, content, vec); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRecall mide el recall LÉXICO (RRF: keyword + recencia + frecuencia) a escala.
func BenchmarkRecall(b *testing.B) {
	ctx := context.Background()
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			eng := benchSeedEngine(b, n, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.Recall(ctx, benchQuery, RecallOptions{TokenBudget: 400, NoBump: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRecallHybrid mide el recall HÍBRIDO (T5.7 R2): léxico + pool vectorial (coseno)
// unidos por RRF de 4 señales. Usa un vector de query determinista.
func BenchmarkRecallHybrid(b *testing.B) {
	ctx := context.Background()
	qvec := benchVector(rand.New(rand.NewSource(7)))
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			eng := benchSeedEngine(b, n, true)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.Recall(ctx, benchQuery, RecallOptions{TokenBudget: 400, NoBump: true, QueryVector: qvec}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSearchFTS mide la búsqueda FTS5 cruda (sin RRF) a escala.
func BenchmarkSearchFTS(b *testing.B) {
	ctx := context.Background()
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			eng := benchSeedEngine(b, n, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.SearchObservationsFTS(ctx, benchQuery, 50); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSearchVector mide la búsqueda vectorial con el índice IVF ENTRENADO. Fuerza el
// rebuild síncrono tras sembrar para medir la ruta indexada de forma determinista: en
// producción el daemon auto-entrena al arrancar, pero el rebuild de fondo es async y el bench,
// sin esto, podría medir el full-scan transitorio. El punto del IVF es que el costo de búsqueda
// crezca SUB-LINEALmente con n (candidatos ~ nprobe·√n, no n); el bench-guard de CI vigila que
// B/op(10k)/B/op(1k) se mantenga sublineal (una regresión IVF→full-scan lo llevaría a ~lineal).
//
// El caso de escala n=100000 es OPT-IN (env MUSUBI_BENCH_SCALE) porque sembrar 100k
// observaciones tarda minutos (inviable en cada corrida de CI); corré
// `MUSUBI_BENCH_SCALE=1 go test -run=NONE -bench=BenchmarkSearchVector -benchmem ./internal/memory/`
// para el perfil de escala manual.
func BenchmarkSearchVector(b *testing.B) {
	ctx := context.Background()
	qvec := benchVector(rand.New(rand.NewSource(9)))
	sizes := []int{1000, 10000}
	if os.Getenv("MUSUBI_BENCH_SCALE") != "" {
		sizes = append(sizes, 100000)
	}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			eng := benchSeedEngine(b, n, true)
			if err := eng.rebuildVectorIndex(); err != nil {
				b.Fatalf("rebuildVectorIndex: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.SearchObservations(ctx, qvec, 50); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMaintain mide el ciclo completo de mantenimiento (consolidar + decay + purgar)
// a escala. Re-siembra por iteración (fuera del timer) porque Maintain muta la base.
func BenchmarkMaintain(b *testing.B) {
	opts := MaintenanceOptions{
		DedupThreshold:         0.9,
		DecayHalfLifeDays:      30,
		DecayMinSalience:       0.1,
		DecayMinAgeDays:        7,
		DecayProtectImportance: 2.0,
		PurgeArchivedAfterDays: 90,
		Vacuum:                 false,
	}
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				eng := benchSeedEngine(b, n, false)
				b.StartTimer()
				if _, err := eng.Maintain(opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkPrimeContext mide el priming de arranque (lo que paga el hook detect en
// SessionStart) a escala.
func BenchmarkPrimeContext(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			eng := benchSeedEngine(b, n, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.PrimeContext(400); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
