package memory

import (
	"context"
	"testing"
)

// TestQuarantineIsAllowlist: la visibilidad del grafo de hechos es un ALLOWLIST ('agent'), simétrico
// con la autoridad (isAuthoritative). Un source NO autoritativo que NO sea 'llm-extract:*' — p. ej.
// 'heuristic' — también queda en cuarentena por default. Antes (denylist) quedaba visible sin
// corroborar, un estado contradictorio (auditoría v0.98.0).
func TestQuarantineIsAllowlist(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if _, err := e.SaveFactFromSourced("", "x", "usa", "y", "", "heuristic", nil); err != nil {
		t.Fatal(err)
	}

	res, err := e.RecallFactsCtx(ctx, "x", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 0 {
		t.Errorf("un source no-agent ('heuristic') debe estar en cuarentena por default; got %d", len(res.Facts))
	}

	// La superficie de revisión (include_proposed) sí lo revela: la cuarentena cubre TODO lo no autoritativo.
	res, err = e.RecallFactsCtx(WithIncludeProposed(ctx), "x", 2, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 {
		t.Errorf("con include_proposed el hecho no-agent debe verse; got %d", len(res.Facts))
	}
}
