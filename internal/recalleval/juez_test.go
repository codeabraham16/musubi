package recalleval

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
)

// Invariantes del spec «El juez se puede medir» (specs/juez-medible/) que viven en el banco.

// juezInvertido es un juez falso DETERMINISTA: devuelve los ids en orden inverso al que recibió.
//
// No necesita red ni cuota, y su efecto sobre las métricas es predecible — que es lo que permite
// afirmar que el brazo está enchufado de verdad. Un juez falso que devolviera el mismo orden dejaría
// a J4 pasando con el brazo desconectado.
type juezInvertido struct{ llamadas atomic.Int64 }

func (j *juezInvertido) Name() string { return "juez-invertido" }

func (j *juezInvertido) Ask(_ context.Context, _, user string) (string, error) {
	j.llamadas.Add(1)
	// Los ids llegan en el user como "[id] gist" por línea; se devuelven al revés.
	var ids []string
	for _, linea := range strings.Split(user, "\n") {
		if !strings.HasPrefix(linea, "[") {
			continue
		}
		if cierre := strings.IndexByte(linea, ']'); cierre > 1 {
			ids = append(ids, linea[1:cierre])
		}
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	b, _ := json.Marshal(ids)
	return string(b), nil
}

// J4 — CON EL BRAZO, EL ORDEN EVALUADO CAMBIA.
//
// Un juez que invierte la cabeza del ranking tiene que EMPEORAR las métricas de forma medible. Si no
// se mueven, el brazo está desconectado y el banco mentiría diciendo «el juez no aporta» — la peor
// salida posible, porque es una conclusión falsa con cara de medición.
func TestJ4ElBrazoDelJuezCambiaLasMetricas(t *testing.T) {
	fx := loadGolden(t)
	ks := []int{1, 5, 10}
	juez := &juezInvertido{}

	conJuez := lexicalConfig
	conJuez.Name = "lexico+juez-invertido"
	conJuez.Juez = juez

	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig, conJuez}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sinJuez, conJuezScores := scores[0], scores[1]

	if juez.llamadas.Load() == 0 {
		t.Fatal("el juez nunca fue llamado: el brazo no está enchufado")
	}
	if conJuezScores.MRR >= sinJuez.MRR {
		t.Errorf("un juez que INVIERTE el tope debería empeorar el MRR: sin juez %.6f, con juez %.6f",
			sinJuez.MRR, conJuezScores.MRR)
	}
	t.Logf("MRR sin juez=%.4f  con juez invertido=%.4f  (llamadas al motor: %d)",
		sinJuez.MRR, conJuezScores.MRR, juez.llamadas.Load())
}

// J5 — SIN EL BRAZO, NADA SE MUEVE.
//
// Control de J4: una Config sin juez tiene que dar exactamente los mismos números que antes de que
// este spec existiera. Si J4 fuera la única prueba, romper el camino model-free pasaría en verde.
func TestJ5SinJuezElBancoEsBitIdentico(t *testing.T) {
	fx := loadGolden(t)
	ks := []int{1, 5, 10}

	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := scores[0]
	want := readBaseline(t)

	const eps = 1e-9
	if diff := got.MRR - want.MRR; diff > eps || diff < -eps {
		t.Errorf("el brazo del juez movió el camino model-free: MRR = %.9f, baseline %.9f", got.MRR, want.MRR)
	}
	for _, k := range ks {
		if d := got.RecallAtK[k] - want.RecallAtK[k]; d > eps || d < -eps {
			t.Errorf("Recall@%d = %.9f, baseline %.9f", k, got.RecallAtK[k], want.RecallAtK[k])
		}
		if d := got.NDCGAtK[k] - want.NDCGAtK[k]; d > eps || d < -eps {
			t.Errorf("nDCG@%d = %.9f, baseline %.9f", k, got.NDCGAtK[k], want.NDCGAtK[k])
		}
	}
}

// J6 — EL BANCO NO MEMOIZA.
//
// Un banco que reusara respuestas mediría el caché en vez del juez, y el número mejoraría cuanto más
// se repitiera el fixture.
//
// LA FORMA DE MEDIRLO IMPORTA. Contar «una llamada por query» no sirve: dentro de una corrida cada
// query aparece una sola vez, así que un caché no se notaría — y además las queries cuyo recall
// devuelve menos de 2 candidatos NO llegan al juez (no hay nada que ordenar, igual que en
// producción), con lo cual el conteo esperado tampoco es el número de queries. La prueba corre la
// MISMA configuración DOS veces: sin memoización, las llamadas se duplican exactas.
func TestJ6ElBancoNoMemoiza(t *testing.T) {
	fx := loadGolden(t)

	unaVez := &juezInvertido{}
	cfg1 := lexicalConfig
	cfg1.Name = "una-vez"
	cfg1.Juez = unaVez
	if _, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{cfg1}, []int{5}); err != nil {
		t.Fatalf("Run (una vez): %v", err)
	}
	base := unaVez.llamadas.Load()
	if base == 0 {
		t.Fatal("el juez nunca fue llamado: la prueba no está midiendo lo que cree")
	}

	dosVeces := &juezInvertido{}
	cfgA, cfgB := lexicalConfig, lexicalConfig
	cfgA.Name, cfgB.Name = "pasada-a", "pasada-b"
	cfgA.Juez, cfgB.Juez = dosVeces, dosVeces
	if _, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{cfgA, cfgB}, []int{5}); err != nil {
		t.Fatalf("Run (dos veces): %v", err)
	}

	if n := dosVeces.llamadas.Load(); n != base*2 {
		t.Fatalf("dos pasadas idénticas deberían dar %d llamadas y dieron %d: el banco está memoizando", base*2, n)
	}
	t.Logf("una pasada = %d llamadas al motor; dos pasadas = %d", base, dosVeces.llamadas.Load())
}
