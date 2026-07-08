package memory

import "testing"

// TestSaveObservationPreservesOriginProjectID valida la atribución multi-tenant (Track 16
// F1 / 16.1a): las variantes *From estampan el project_id de ORIGEN que pasa el caller (el
// ingest del central preservando el proyecto de la máquina que originó la memoria), mientras
// las variantes sin From siguen usando el project_id del engine (guardado local).
func TestSaveObservationPreservesOriginProjectID(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("central")

	projectOf := func(id string) string {
		var pid string
		if err := e.db.QueryRow(`SELECT COALESCE(project_id,'') FROM observations WHERE id=?`, id).Scan(&pid); err != nil {
			t.Fatalf("no se pudo leer project_id de %s: %v", id, err)
		}
		return pid
	}

	// *From con origen explícito ⇒ se estampa el ORIGEN, no el del engine.
	if err := e.SaveObservationTypedFrom("laptop-repo", "id1", "t/a", "hecho A", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if got := projectOf("id1"); got != "laptop-repo" {
		t.Errorf("id1 project_id = %q, esperaba el origen 'laptop-repo'", got)
	}

	// Sin From ⇒ se estampa el project_id del engine (comportamiento de siempre).
	if err := e.SaveObservationTyped("id2", "t/b", "hecho B", 0.5, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if got := projectOf("id2"); got != "central" {
		t.Errorf("id2 project_id = %q, esperaba el del engine 'central'", got)
	}

	// *From con origen "" ⇒ cae al del engine (backward-compat).
	if err := e.SaveObservationTypedFrom("", "id2b", "t/b2", "hecho B2", 0.5, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if got := projectOf("id2b"); got != "central" {
		t.Errorf("id2b project_id = %q, origen vacío debía caer al engine 'central'", got)
	}

	// Variante deduped *From.
	id3, _, err := e.SaveObservationDedupedTypedFrom("pc-repo", "t/c", "hecho C único", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectOf(id3); got != "pc-repo" {
		t.Errorf("id3 project_id = %q, esperaba 'pc-repo'", got)
	}
}
