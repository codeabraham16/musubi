package memory

import "testing"

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
