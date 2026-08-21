package memory

import "testing"

// Tests del DEDUP SEMÁNTICO del acervo (pilar 'Musubi Renaissance', el afilador). Cubren: la detección
// de pares gemelos por coseno con el orden canónico correcto, el aislamiento por tenant, la exclusión de
// pares ya juzgados, el chequeo anti-gemelo, y el archivado que hereda accesos e importancia.

func saveDesignCard(t *testing.T, e *DbEngine, project, id, topic, content string, vec []float32, access int, importance float64) {
	t.Helper()
	if err := e.SaveObservationTypedFrom(project, "seed", id, topic, content, importance, "semantic", "shared", vec); err != nil {
		t.Fatalf("guardar tarjeta %s: %v", id, err)
	}
	if access > 0 {
		if _, err := e.db.Exec(`UPDATE observations SET access_count=? WHERE id=?`, access, id); err != nil {
			t.Fatalf("fijar access de %s: %v", id, err)
		}
	}
}

func TestSemanticDuplicateCandidates(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	// A y B son casi paralelos (coseno ~0.98); C es ortogonal a ambos. A es MÁS FUERTE (más accesos).
	saveDesignCard(t, e, proj, "a", "design-corpus/contraste-a", "contraste 4.5:1", []float32{1, 0, 0, 0}, 5, 1.0)
	saveDesignCard(t, e, proj, "b", "design-corpus/contraste-b", "el texto necesita 4.5:1", []float32{0.98, 0.2, 0, 0}, 0, 1.0)
	saveDesignCard(t, e, proj, "c", "design-corpus/espaciado", "todo múltiplo de 4", []float32{0, 1, 0, 0}, 0, 1.0)
	// De otro tenant, aunque su vector calce con A: jamás debe emparejarse (aislamiento).
	saveDesignCard(t, e, "otro", "z", "design-corpus/ajeno", "de otro proyecto", []float32{1, 0, 0, 0}, 0, 1.0)

	cands, err := e.SemanticDuplicateCandidates(proj, "design-corpus/", 0.9, 10)
	if err != nil {
		t.Fatalf("SemanticDuplicateCandidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("esperaba 1 par (a,b) sobre el piso; obtuve %d: %+v", len(cands), cands)
	}
	if cands[0].A != "a" || cands[0].B != "b" {
		t.Errorf("el canónico (más fuerte) debe ser 'a' y el candidato 'b'; obtuve A=%s B=%s", cands[0].A, cands[0].B)
	}
	if cands[0].Cosine < 0.95 {
		t.Errorf("coseno esperado ~0.98, obtuve %.3f", cands[0].Cosine)
	}

	// Un par YA juzgado (not_duplicate) se excluye: no se re-propone.
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "a", TargetID: "b", Relation: RelNotDuplicate, Status: RelStatusResolved}); err != nil {
		t.Fatalf("marcar not_duplicate: %v", err)
	}
	cands2, err := e.SemanticDuplicateCandidates(proj, "design-corpus/", 0.9, 10)
	if err != nil {
		t.Fatalf("SemanticDuplicateCandidates #2: %v", err)
	}
	if len(cands2) != 0 {
		t.Errorf("un par marcado not_duplicate no debe volver a proponerse; obtuve %+v", cands2)
	}
}

func TestArchiveAsDuplicate(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	saveDesignCard(t, e, proj, "canon", "design-corpus/canon", "la buena", []float32{1, 0, 0, 0}, 5, 1.0)
	saveDesignCard(t, e, proj, "dup", "design-corpus/dup", "la gemela", []float32{0.98, 0.2, 0, 0}, 3, 2.0)

	archived, err := e.ArchiveAsDuplicate(proj, "dup", "canon")
	if err != nil || !archived {
		t.Fatalf("archivar dup: archived=%v err=%v", archived, err)
	}
	// El canónico heredó accesos (5+3) e importancia máxima (max(1,2)=2).
	var access int
	var importance float64
	if err := e.db.QueryRow(`SELECT access_count, importance FROM observations WHERE id='canon'`).Scan(&access, &importance); err != nil {
		t.Fatalf("leer canon: %v", err)
	}
	if access != 8 {
		t.Errorf("el canónico debe heredar accesos: esperaba 8, obtuve %d", access)
	}
	if importance != 2.0 {
		t.Errorf("el canónico debe quedar con la importancia máxima: esperaba 2.0, obtuve %.1f", importance)
	}
	// El perdedor quedó archivado y apuntando al canónico.
	var arch int
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(archived,0), COALESCE(superseded_by,'') FROM observations WHERE id='dup'`).Scan(&arch, &sup); err != nil {
		t.Fatalf("leer dup: %v", err)
	}
	if arch != 1 || sup != "canon" {
		t.Errorf("el perdedor debe quedar archivado apuntando al canónico; archived=%d superseded_by=%q", arch, sup)
	}

	// Idempotente: re-archivar el mismo perdedor es un no-op (archived=false, sin error).
	again, err := e.ArchiveAsDuplicate(proj, "dup", "canon")
	if err != nil || again {
		t.Errorf("re-archivar un perdedor ya archivado debe ser no-op; archived=%v err=%v", again, err)
	}

	// Guarda de tenant: fusionar con un projectID que no es el de las tarjetas debe fallar.
	saveDesignCard(t, e, proj, "otra", "design-corpus/otra", "x", []float32{1, 0, 0, 0}, 0, 1.0)
	if _, err := e.ArchiveAsDuplicate("tenant-ajeno", "otra", "canon"); err == nil {
		t.Error("archivar con un projectID ajeno a las tarjetas debe fallar (aislamiento)")
	}
}

func TestNearestVisibleByVector(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	saveDesignCard(t, e, proj, "a", "design-corpus/a", "uno", []float32{1, 0, 0, 0}, 0, 1.0)
	saveDesignCard(t, e, proj, "c", "design-corpus/c", "dos", []float32{0, 1, 0, 0}, 0, 1.0)

	id, _, cos, err := e.NearestVisibleByVector(proj, "design-corpus/", []float32{0.99, 0.1, 0, 0}, "")
	if err != nil {
		t.Fatalf("NearestVisibleByVector: %v", err)
	}
	if id != "a" || cos < 0.95 {
		t.Errorf("el más cercano a ~[1,0..] debe ser 'a' con coseno alto; obtuve id=%s cos=%.3f", id, cos)
	}
	// Excluyendo 'a' queda 'c' (ortogonal, coseno bajo pero es el único).
	id2, _, _, err := e.NearestVisibleByVector(proj, "design-corpus/", []float32{0.99, 0.1, 0, 0}, "a")
	if err != nil {
		t.Fatalf("NearestVisibleByVector excluyendo a: %v", err)
	}
	if id2 != "c" {
		t.Errorf("excluyendo 'a' el más cercano debe ser 'c'; obtuve %s", id2)
	}
	// Sin vector (nil): vacío, sin error (degrada blando cuando no hay embedder).
	if id3, _, _, err := e.NearestVisibleByVector(proj, "design-corpus/", nil, ""); err != nil || id3 != "" {
		t.Errorf("con vec nil debe devolver vacío sin error; id=%q err=%v", id3, err)
	}
}
