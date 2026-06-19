package memory

import (
	"math/rand"
	"testing"
)

// TestEngineCloseBlocksNewBackgroundWork verifica el fix de lifecycle: tras Close(),
// spawnBackground NO lanza trabajo nuevo (no hay use-after-close del *sql.DB). El
// hallazgo de la verificación adversarial: las goroutines de rebuild sobrevivían a
// Close y consultaban una base ya cerrada.
func TestEngineCloseBlocksNewBackgroundWork(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Dejar que la goroutine de autobuild del arranque termine.
	e.bgWG.Wait()

	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ran := false
	if e.spawnBackground(func() { ran = true }) {
		t.Error("spawnBackground devolvió true tras Close (no debería lanzar)")
	}
	e.bgWG.Wait()
	if ran {
		t.Error("se ejecutó trabajo de fondo después de Close")
	}
}

// TestProactiveTrainAcrossThreshold valida el fix del hallazgo HIGH: una base que
// arranca por DEBAJO del umbral y crece a través de él (incrementalmente) SÍ entrena
// el índice. Antes el disparador miraba solo las altas del proceso (dirty arranca en
// 0), así que nunca se entrenaba; ahora autobuild siembra dirty con el conteo absoluto.
func TestProactiveTrainAcrossThreshold(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	// Settle del autobuild de arranque (base vacía => seedDirty(0)).
	e.bgWG.Wait()

	const dim = 16
	e.vindexCfg.ExactThreshold = 4

	// Simular que ya existían 3 embeddings al arrancar (justo debajo del umbral 4):
	// autobuild habría sembrado dirty=3 sin entrenar.
	e.index.seedDirty(3)
	if e.index.Trained() {
		t.Fatal("el índice no debería estar entrenado por debajo del umbral")
	}

	// Guardar una observación más cruza el umbral (dirty 3 -> 4): debe disparar el
	// entrenamiento proactivo en segundo plano.
	rng := rand.New(rand.NewSource(1))
	if err := e.SaveObservation("cross", "t", "c", randomVec(rng, dim)); err != nil {
		t.Fatal(err)
	}
	// Esperar a que el rebuild en segundo plano termine.
	e.bgWG.Wait()

	if !e.index.Trained() {
		t.Fatal("el índice debería haberse entrenado al cruzar el umbral incrementalmente")
	}
}
