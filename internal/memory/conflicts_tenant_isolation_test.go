package memory

import "testing"

// REGRESIÓN (auditoría 2026-07-26, #6 CRÍTICO): la detección de conflictos en el ingest (incluido el
// que llega por sync al central multi-tenant) armaba su pool de candidatas SIN acotar por proyecto.
// Consecuencias: (a) la respuesta de save_observation filtraba ids/gists de OTROS tenants, y (b) un
// auto-supersede podía OCULTAR la memoria de otro proyecto. La guarda de tenant en DetectRelations
// cierra ambas. Este test fija que una fuente de proj-A nunca se relacione ni oculte una fila de proj-B,
// y —control— que la detección DENTRO del mismo proyecto sigue funcionando (la guarda no apaga todo).
func TestDetectRelationsIsolatesByTenant(t *testing.T) {
	e := newTestEngine(t)
	const content = "elegimos sqlite embebido y model-free para la memoria del cerebro"
	const topic = "arquitectura/db" // topic NO histórico: no lo exime complementaryPair

	// Víctima en el proyecto B (más vieja).
	if err := e.SaveObservationTypedFrom("proj-B", "", "b1", topic, content, 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	// Fuente en el proyecto A: MISMO contenido y topic ⇒ sin la guarda dispararía una relación (y un
	// posible supersede) contra b1.
	if err := e.SaveObservationTypedFrom("proj-A", "", "a1", topic, content, 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	rels, err := e.DetectRelations("a1", ConflictOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rels {
		if r.TargetID == "b1" {
			t.Fatalf("FUGA CROSS-TENANT: a1 (proj-A) generó una relación contra b1 (proj-B): %+v", r)
		}
	}
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='b1'`).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup != "" {
		t.Fatalf("FUGA CROSS-TENANT: b1 (proj-B) quedó superseded_by=%q por una fuente de proj-A", sup)
	}

	// CONTROL: dentro del MISMO proyecto la detección SIGUE viva (si esto encuentra la relación,
	// entonces el caso cross-tenant también la habría encontrado sin la guarda).
	if err := e.SaveObservationTypedFrom("proj-A", "", "a2", topic, content, 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	rels2, err := e.DetectRelations("a2", ConflictOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rels2 {
		if r.TargetID == "a1" {
			found = true
		}
	}
	if !found {
		t.Fatal("la guarda de tenant no debe apagar la detección intra-tenant: a2 debía relacionarse con a1")
	}
}
