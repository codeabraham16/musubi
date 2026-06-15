package memory

import "testing"

func TestEntityContextBridgesFactsAndObservations(t *testing.T) {
	e := newTestEngine(t)

	// Prosa: una observación que menciona la entidad.
	if err := e.SaveObservation("o1", "infra", "desplegamos el cluster de kubernetes en produccion", nil); err != nil {
		t.Fatal(err)
	}
	// Grafo: hechos sobre la entidad.
	mustFact(t, e, "kubernetes", "corre_en", "cloud")
	mustFact(t, e, "kubernetes", "orquesta", "contenedores")

	ctx, err := e.EntityContext("kubernetes", 1, 50, 5)
	if err != nil {
		t.Fatalf("EntityContext error: %v", err)
	}
	if len(ctx.Facts) != 2 {
		t.Errorf("esperaba 2 hechos, obtuve %d: %+v", len(ctx.Facts), ctx.Facts)
	}
	if len(ctx.Observations) != 1 || ctx.Observations[0].ID != "o1" {
		t.Errorf("esperaba 1 observación que menciona kubernetes, obtuve %+v", ctx.Observations)
	}
	if ctx.Observations[0].Gist == "" {
		t.Error("la observación debería traer gist, no contenido completo")
	}
}

func TestEntityContextExcludesArchivedObservations(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "t", "nota visible sobre redis", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("o2", "t", "nota archivada sobre redis", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1 WHERE id='o2'`); err != nil {
		t.Fatal(err)
	}

	ctx, err := e.EntityContext("redis", 1, 50, 5)
	if err != nil {
		t.Fatalf("EntityContext error: %v", err)
	}
	if len(ctx.Observations) != 1 || ctx.Observations[0].ID != "o1" {
		t.Errorf("debería excluir observaciones archivadas, obtuve %+v", ctx.Observations)
	}
}

func TestEntityContextUnknownEntity(t *testing.T) {
	e := newTestEngine(t)
	ctx, err := e.EntityContext("inexistente", 2, 50, 5)
	if err != nil {
		t.Fatalf("EntityContext error: %v", err)
	}
	if len(ctx.Facts) != 0 || len(ctx.Observations) != 0 {
		t.Errorf("entidad sin datos debería dar contexto vacío, obtuve %+v", ctx)
	}
}
