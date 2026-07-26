package memory

import (
	"context"
	"testing"
)

// REGRESIÓN (auditoría 2026-07-26, #4): el pull entrante paginaba por rowid, que NO cambia en un
// UPDATE ⇒ una edición de una obs shared ya sincronizada nunca se re-entregaba. Con sync_seq (que sube
// en cada insert/update) el pull con un cursor que YA pasó la fila la vuelve a devolver tras editarla.
func TestSyncSeqRepullsEditedSharedObs(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("acme")

	// Dos obs shared.
	if err := e.SaveObservationTyped("a", "t/a", "alpha original", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("b", "t/b", "beta original", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	// Un cliente baja todo y su cursor queda en el mayor sync_seq entregado.
	first, err := e.ListSharedForPull(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("esperaba 2 items en el primer pull, obtuve %d", len(first))
	}
	var cursor int64
	for _, o := range first {
		if o.RowID > cursor {
			cursor = o.RowID
		}
	}

	// Con ese cursor, un pull incremental no trae nada nuevo (todo ya bajado).
	none, err := e.ListSharedForPull(context.Background(), cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("tras bajar todo, el pull incremental no debe traer nada, obtuve %d", len(none))
	}

	// EDITAR 'a' (mismo id, contenido nuevo): su sync_seq debe subir por encima del cursor.
	if err := e.SaveObservationTyped("a", "t/a", "alpha EDITADO", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	// El mismo cursor ahora SÍ vuelve a traer 'a' con el contenido editado (antes: nunca, hueco stale).
	after, err := e.ListSharedForPull(context.Background(), cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].ID != "a" {
		t.Fatalf("el pull incremental debe re-entregar 'a' editada; obtuve %d items", len(after))
	}
	if after[0].Content != "alpha EDITADO" {
		t.Fatalf("la re-entrega debe traer el contenido nuevo; obtuve %q", after[0].Content)
	}
}
