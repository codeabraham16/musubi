package memory

import (
	"context"
	"testing"
)

// TestRecallProjectIsolation valida el aislamiento por proyecto del recall (Track 16 F1
// 16.1b): con ProjectScope y sin Federate, el recall descarta los candidatos de OTROS
// proyectos pero conserva el proyecto pedido y las filas sin atribuir; Federate ve todo;
// y sin ProjectScope el comportamiento es el histórico (federado).
func TestRecallProjectIsolation(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("") // el origen lo fija cada save; "" ⇒ sin atribuir

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Mismo término buscable en las tres, distinto proyecto de origen.
	must(e.SaveObservationTypedFrom("projA", "a1", "deploy/notes", "deploy alpha zqxtoken", 1.0, "semantic", "local", nil))
	must(e.SaveObservationTypedFrom("projB", "b1", "deploy/notes", "deploy beta zqxtoken", 1.0, "semantic", "local", nil))
	must(e.SaveObservationTypedFrom("", "u1", "deploy/notes", "deploy sinproyecto zqxtoken", 1.0, "semantic", "local", nil))

	idsOf := func(r RecallResult) map[string]bool {
		m := make(map[string]bool, len(r.Items))
		for _, it := range r.Items {
			m[it.ID] = true
		}
		return m
	}
	ctx := context.Background()
	const q = "zqxtoken"

	// Aislado a projA ⇒ a1 + u1 (sin atribuir), NO b1.
	r, err := e.Recall(ctx, q, RecallOptions{ProjectScope: "projA"})
	must(err)
	got := idsOf(r)
	if !got["a1"] || !got["u1"] || got["b1"] {
		t.Errorf("aislado projA: esperaba {a1,u1} sin b1, obtuve %v", got)
	}

	// Federado ⇒ los tres, aunque haya ProjectScope.
	r, err = e.Recall(ctx, q, RecallOptions{ProjectScope: "projA", Federate: true})
	must(err)
	if got = idsOf(r); !got["a1"] || !got["b1"] || !got["u1"] {
		t.Errorf("federado: esperaba {a1,b1,u1}, obtuve %v", got)
	}

	// Sin ProjectScope ⇒ histórico (federado): los tres.
	r, err = e.Recall(ctx, q, RecallOptions{})
	must(err)
	if got = idsOf(r); !got["a1"] || !got["b1"] || !got["u1"] {
		t.Errorf("sin scope: esperaba {a1,b1,u1}, obtuve %v", got)
	}
}
