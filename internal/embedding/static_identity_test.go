package embedding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// N1: el model_id del StaticProvider debe identificar el CONTENIDO de la tabla, no sólo el nombre
// de su carpeta. Antes era "static:"+basename ⇒ re-destilar la tabla in-place NO cambiaba el id,
// los vectores viejos seguían pareciendo compatibles y la búsqueda los comparaba por coseno contra
// los de la tabla nueva ⇒ ranking corrupto EN SILENCIO.

func modelIDOf(t *testing.T, dir string) string {
	t.Helper()
	p, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("NewStaticProvider(%s): %v", dir, err)
	}
	return p.Name()
}

// N1.a — el checksum es determinista: recargar la MISMA tabla da el mismo model_id.
func TestStaticModelIDStableAcrossLoads(t *testing.T) {
	dir := t.TempDir()
	writeSyntheticModel(t, dir, [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}})

	first := modelIDOf(t, dir)
	second := modelIDOf(t, dir)
	if first != second {
		t.Errorf("el model_id debe ser estable entre cargas: %q vs %q", first, second)
	}
	// Forma: static:<basename>@<checksum>, y el basename sigue estando (R6).
	if !strings.HasPrefix(first, "static:"+filepath.Base(dir)+"@") {
		t.Errorf("model_id %q no tiene la forma static:<basename>@<checksum>", first)
	}
}

// N1.b — re-destilar la tabla IN-PLACE (mismo dir, otros vectores) DEBE cambiar el model_id.
// Éste es el bug de raíz: antes daba el mismo id y corrompía el ranking en silencio.
func TestStaticModelIDChangesOnTableRedistill(t *testing.T) {
	dir := t.TempDir()
	writeSyntheticModel(t, dir, [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}})
	before := modelIDOf(t, dir)

	// Re-destilación in-place: mismo directorio y nombre, vectores distintos.
	writeSyntheticModel(t, dir, [][]float32{{0, 1}, {1, 0}, {1, 1}, {0, 0}, {0, 1}})
	after := modelIDOf(t, dir)

	if before == after {
		t.Errorf("re-destilar la tabla in-place DEBE cambiar el model_id, pero siguió siendo %q", before)
	}
}

// N1.c — cambiar SÓLO el tokenizer también debe cambiar el model_id: otra tokenización ⇒ otro
// mean-pool ⇒ otros vectores. Un checksum que sólo mirara el safetensors dejaría pasar esto.
func TestStaticModelIDChangesOnTokenizerChange(t *testing.T) {
	dir := t.TempDir()
	writeSyntheticModel(t, dir, [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}})
	before := modelIDOf(t, dir)

	// Mismo model.safetensors; se toca sólo el tokenizer (otro vocab).
	tokPath := filepath.Join(dir, "tokenizer.json")
	raw, err := os.ReadFile(tokPath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(raw), `"deploy":1`, `"deployed":1`, 1)
	if mutated == string(raw) {
		// El JSON puede venir con espacios; hacemos un cambio garantizado igualmente válido.
		mutated = strings.Replace(string(raw), "deploy", "deploym", 1)
	}
	if err := os.WriteFile(tokPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	after := modelIDOf(t, dir)
	if before == after {
		t.Errorf("cambiar el tokenizer DEBE cambiar el model_id, pero siguió siendo %q", before)
	}
}

// El basename sigue formando parte de la identidad: dos dirs con contenido IDÉNTICO pero nombre
// distinto siguen teniendo model_id distinto (R6).
func TestStaticModelIDIncludesBasename(t *testing.T) {
	rows := [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}}

	dirA := filepath.Join(t.TempDir(), "tabla-a")
	dirB := filepath.Join(t.TempDir(), "tabla-b")
	for _, d := range []string{dirA, dirB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		writeSyntheticModel(t, d, rows)
	}

	idA, idB := modelIDOf(t, dirA), modelIDOf(t, dirB)
	if idA == idB {
		t.Error("dos tablas con nombre distinto deben tener model_id distinto aunque el contenido coincida")
	}
	// Mismo contenido ⇒ el checksum (lo que va tras la @) debe coincidir.
	sumA := idA[strings.LastIndex(idA, "@")+1:]
	sumB := idB[strings.LastIndex(idB, "@")+1:]
	if sumA != sumB {
		t.Errorf("mismo contenido debe dar el mismo checksum: %q vs %q", sumA, sumB)
	}
}
