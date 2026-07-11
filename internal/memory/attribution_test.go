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
	if err := e.SaveObservationTypedFrom("laptop-repo", "", "id1", "t/a", "hecho A", 0.5, "semantic", "shared", nil); err != nil {
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
	if err := e.SaveObservationTypedFrom("", "", "id2b", "t/b2", "hecho B2", 0.5, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if got := projectOf("id2b"); got != "central" {
		t.Errorf("id2b project_id = %q, origen vacío debía caer al engine 'central'", got)
	}

	// Variante deduped *From.
	id3, _, err := e.SaveObservationDedupedTypedFrom("pc-repo", "", "t/c", "hecho C único", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectOf(id3); got != "pc-repo" {
		t.Errorf("id3 project_id = %q, esperaba 'pc-repo'", got)
	}
}

// TestSaveObservationStampsAuthor valida la atribución por PERSONA (C5.1 / R3): las variantes *From
// estampan el author que pasa el caller (derivado de la credencial en el handler), un re-save por
// OTRA persona PRESERVA el autor original (no reasigna crédito, igual que project_id/scope), y un
// guardado sin author queda vacío (backward-compat, R3.6).
func TestSaveObservationStampsAuthor(t *testing.T) {
	e := newTestEngine(t)

	authorOf := func(id string) string {
		var a string
		if err := e.db.QueryRow(`SELECT author FROM observations WHERE id=?`, id).Scan(&a); err != nil {
			t.Fatalf("no se pudo leer author de %s: %v", id, err)
		}
		return a
	}

	// author explícito ⇒ se estampa.
	if err := e.SaveObservationTypedFrom("acme", "ana", "o1", "t/a", "hecho A", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if got := authorOf("o1"); got != "ana" {
		t.Errorf("author de o1 = %q, esperaba 'ana'", got)
	}

	// Re-save por OTRA persona (mismo id) ⇒ PRESERVA el autor original (no reasigna crédito).
	if err := e.SaveObservationTypedFrom("acme", "juan", "o1", "t/a", "hecho A editado por juan", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if got := authorOf("o1"); got != "ana" {
		t.Errorf("tras re-save por juan, author de o1 = %q, debía preservar 'ana'", got)
	}

	// Variante deduped con author.
	id2, _, err := e.SaveObservationDedupedTypedFrom("acme", "juan", "t/b", "hecho B único", 1, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := authorOf(id2); got != "juan" {
		t.Errorf("author de id2 = %q, esperaba 'juan'", got)
	}

	// Guardado sin author (vía sin From, captura local) ⇒ vacío (backward-compat).
	if err := e.SaveObservationTyped("o3", "t/c", "hecho C", 1, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if got := authorOf("o3"); got != "" {
		t.Errorf("guardado local: author de o3 = %q, esperaba ''", got)
	}
}
