package memory

import (
	"context"
	"testing"
)

// accesos devuelve el access_count de una observación, o falla.
func accesos(t *testing.T, e *DbEngine, id string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("leyendo access_count de %s: %v", id, err)
	}
	return n
}

// G8 — hidratar para fundamentar NO cuenta como un segundo acceso.
//
// Recall ya bumpea lo que devuelve; si la hidratación del grounding bumpeara de nuevo, cada
// musubi_ask contaría DOS accesos por memoria. El ranking del recall usa frecuencia, así que eso
// sería el ranker alimentándose de su propia salida — lo que la invariante N4 prohíbe.
func TestG8HidratarParaGroundingNoCuentaAcceso(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.SaveObservationTyped("G8", "t/k", "el router de flota escala al siguiente motor", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	antes := accesos(t, e, "G8")

	if _, _, err := e.HydrateForGroundingCtx(ctx, []string{"G8"}, 0); err != nil {
		t.Fatalf("HydrateForGroundingCtx: %v", err)
	}
	if got := accesos(t, e, "G8"); got != antes {
		t.Errorf("FUGA G8: hidratar para grounding contó un acceso (%d → %d); el recall ya lo había contado", antes, got)
	}
}

// Contracara de G8: la hidratación NORMAL (musubi_memory_expand) SÍ debe seguir contando el acceso.
// Sin esta mitad, "no cuenta" se podría cumplir rompiendo el conteo para todos.
func TestHidratacionNormalSigueContandoAcceso(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.SaveObservationTyped("EXP", "t/k", "el caché de cognición guarda por clave con prefijo de largo", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	antes := accesos(t, e, "EXP")

	if _, _, err := e.GetObservationsBudgetCtx(ctx, []string{"EXP"}, 0); err != nil {
		t.Fatalf("GetObservationsBudgetCtx: %v", err)
	}
	if got := accesos(t, e, "EXP"); got <= antes {
		t.Errorf("memory_expand debe seguir contando el acceso (%d → %d)", antes, got)
	}
}

// Las dos puertas devuelven exactamente el mismo contenido: sólo difieren en el efecto de escritura.
func TestLasDosPuertasHidratanIgual(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.SaveObservationTyped("MISMO", "t/k", "el portero repone el marcador en la respuesta al caller", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}

	a, usadoA, err := e.HydrateForGroundingCtx(ctx, []string{"MISMO"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, usadoB, err := e.GetObservationsBudgetCtx(ctx, []string{"MISMO"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) || usadoA != usadoB {
		t.Fatalf("las dos puertas deben hidratar igual: %d/%d items, %d/%d tokens", len(a), len(b), usadoA, usadoB)
	}
	if len(a) == 1 && a[0].Content != b[0].Content {
		t.Errorf("contenido distinto entre las dos puertas:\n%q\n%q", a[0].Content, b[0].Content)
	}
}
