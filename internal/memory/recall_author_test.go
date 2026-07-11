package memory

import (
	"context"
	"testing"
)

// TestRecallSurfacesAuthor valida el surfacing de la atribución por persona (C5.1 / R3.5): un
// recall devuelve el author de cada item (quién aportó la memoria), y vacío cuando no hay
// atribución (captura local). Complementa el sellado de TestSaveObservationStampsAuthor.
func TestRecallSurfacesAuthor(t *testing.T) {
	e := newTestEngine(t)

	// Con atribución (memoria de equipo).
	if err := e.SaveObservationTypedFrom("acme", "ana", "o1", "deploy/notes", "el deploy usa rclone hacia b2", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	// Sin atribución (captura local).
	if err := e.SaveObservationTyped("o2", "deploy/local", "el deploy local corre rclone tambien", 1, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}

	res, err := e.Recall(context.Background(), "deploy rclone", RecallOptions{TokenBudget: 400})
	if err != nil {
		t.Fatal(err)
	}

	authors := map[string]string{}
	for _, it := range res.Items {
		authors[it.ID] = it.Author
	}
	if _, ok := authors["o1"]; !ok {
		t.Fatalf("recall no devolvió o1; items=%d", len(res.Items))
	}
	if authors["o1"] != "ana" {
		t.Errorf("recall item o1 author = %q, esperaba 'ana'", authors["o1"])
	}
	if got, ok := authors["o2"]; ok && got != "" {
		t.Errorf("recall item o2 (captura local) author = %q, esperaba vacío", got)
	}
}
