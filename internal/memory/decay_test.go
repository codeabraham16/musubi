package memory

import (
	"fmt"
	"testing"
)

func TestDecayArchivesColdLowSalience(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservation("vieja", "t", "memoria vieja y poco usada", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("nueva", "t", "memoria reciente", nil); err != nil {
		t.Fatal(err)
	}
	// Envejecer 'vieja' 400 días (last_accessed sigue NULL -> usa created_at).
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-400 days') WHERE id='vieja'`); err != nil {
		t.Fatal(err)
	}

	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("Decay error: %v", err)
	}
	if res.Archived != 1 {
		t.Errorf("esperaba 1 archivada, obtuve %d", res.Archived)
	}

	var oldArch, newArch int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='vieja'`).Scan(&oldArch); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='nueva'`).Scan(&newArch); err != nil {
		t.Fatal(err)
	}
	if oldArch != 1 {
		t.Error("la memoria vieja debería quedar archivada")
	}
	if newArch != 0 {
		t.Error("la memoria reciente NO debería archivarse")
	}
}

// TestDecayPaginationEquivalence verifica la NO-regresión clave de T5.5: el scan paginado
// archiva EXACTAMENTE el mismo conjunto sin importar el tamaño de página. Compara batch=2
// (muchas páginas) contra batch grande (una sola pasada, el comportamiento histórico) sobre
// datos idénticos, sin re-derivar la fórmula.
func TestDecayPaginationEquivalence(t *testing.T) {
	ages := []int{5, 20, 60, 120, 200, 400, 10, 90, 15, 300} // días desde created_at
	opts := DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14}

	archivedSet := func(batch int) map[string]bool {
		orig := decayBatchSize
		decayBatchSize = batch
		defer func() { decayBatchSize = orig }()

		e := newTestEngine(t)
		for i, age := range ages {
			id := fmt.Sprintf("o%d", i)
			if err := e.SaveObservation(id, "t", fmt.Sprintf("memoria %d", i), nil); err != nil {
				t.Fatal(err)
			}
			if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now', ?) WHERE id=?`,
				fmt.Sprintf("-%d days", age), id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := e.Decay(opts); err != nil {
			t.Fatalf("Decay error: %v", err)
		}
		set := map[string]bool{}
		rows, _ := e.db.Query(`SELECT id FROM observations WHERE archived=1`)
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			set[id] = true
		}
		rows.Close()
		return set
	}

	paginated := archivedSet(2)    // muchas páginas
	full := archivedSet(len(ages)) // una sola pasada (histórico)

	if len(paginated) == 0 {
		t.Fatal("el set de prueba debería archivar algo (datos viejos y fríos)")
	}
	if len(paginated) != len(full) {
		t.Fatalf("paginado archivó %d, una-pasada archivó %d (deben coincidir)", len(paginated), len(full))
	}
	for id := range full {
		if !paginated[id] {
			t.Errorf("la obs %s se archiva en una-pasada pero no paginada (regresión de paginación)", id)
		}
	}
}

// TestDecayProtectsHighImportance verifica la protección por importancia (T5.5): una obs
// vieja y fría pero importante NO se archiva con protección activa, y SÍ sin ella.
func TestDecayProtectsHighImportance(t *testing.T) {
	seed := func(e *DbEngine) {
		if err := e.SaveObservationWithImportance("dec", "t", "decisión arquitectónica clave", 3.0, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-400 days') WHERE id='dec'`); err != nil {
			t.Fatal(err)
		}
	}

	// Con protección (>=2.0): la importante sobrevive pese a ser vieja y fría.
	e1 := newTestEngine(t)
	seed(e1)
	r1, err := e1.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14, ProtectImportance: 2.0})
	if err != nil {
		t.Fatalf("Decay error: %v", err)
	}
	if r1.Archived != 0 {
		t.Errorf("con protección, la obs importante no debía archivarse, archivó %d", r1.Archived)
	}

	// Sin protección: la misma obs SÍ se archiva (control).
	e2 := newTestEngine(t)
	seed(e2)
	r2, err := e2.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("Decay error: %v", err)
	}
	if r2.Archived != 1 {
		t.Errorf("sin protección, la obs vieja y fría debía archivarse, archivó %d", r2.Archived)
	}
}

func TestDecayRespectsMinAge(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("reciente", "t", "algo guardado hoy", nil); err != nil {
		t.Fatal(err)
	}
	// Aún con umbral de saliencia alto, no debe archivar nada por debajo de MinAgeDays.
	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 100, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("Decay error: %v", err)
	}
	if res.Archived != 0 {
		t.Errorf("no debería archivar memorias recientes (< MinAgeDays), archivó %d", res.Archived)
	}
}
