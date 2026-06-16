package memory

import (
	"fmt"
	"strings"
	"testing"
)

func TestRecallBudgetRespected(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("obs%d", i)
		content := fmt.Sprintf("observacion numero %d con bastante texto para ocupar varios tokens", i)
		if err := e.SaveObservation(id, "t", content, nil); err != nil {
			t.Fatalf("save error: %v", err)
		}
	}

	res, err := e.Recall("observacion", RecallOptions{TokenBudget: 30})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if res.UsedTokens > 30 {
		t.Errorf("used_tokens %d excede el presupuesto 30", res.UsedTokens)
	}
	if res.Count < 1 || res.Count >= 10 {
		t.Errorf("esperaba un recorte por presupuesto (1..9), obtuve count=%d", res.Count)
	}
	if res.Count != len(res.Items) {
		t.Errorf("count=%d no coincide con len(items)=%d", res.Count, len(res.Items))
	}
}

func TestRecallReturnsGistsNotFullContent(t *testing.T) {
	e := newTestEngine(t)
	long := "Resumen corto. " + strings.Repeat("relleno ", 100)
	if err := e.SaveObservation("g1", "t", long, nil); err != nil {
		t.Fatalf("save error: %v", err)
	}

	res, err := e.Recall("resumen", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if res.Count != 1 {
		t.Fatalf("esperaba 1 item, obtuve %d", res.Count)
	}
	it := res.Items[0]
	if it.Gist != "Resumen corto." {
		t.Errorf("esperaba gist extractivo 'Resumen corto.', obtuve %q", it.Gist)
	}
	if it.FullTokens <= EstimateTokens(it.Gist) {
		t.Errorf("full_tokens (%d) debería ser mayor que los del gist (%d)", it.FullTokens, EstimateTokens(it.Gist))
	}
}

func TestRecallBumpsAccess(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a1", "t", "alpha beta", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}

	if _, err := e.Recall("alpha", RecallOptions{}); err != nil {
		t.Fatalf("Recall error: %v", err)
	}

	var count int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id=?`, "a1").Scan(&count); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count < 1 {
		t.Errorf("esperaba access_count >= 1 tras recall, obtuve %d", count)
	}
}

func TestSearchObservationsFTSHandlesSpecialChars(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "postgres y redis en produccion", nil); err != nil {
		t.Fatal(err)
	}
	// Caracteres que romperían la sintaxis de FTS5 si se pasaran crudos.
	res, err := e.SearchObservationsFTS(`postgres "y (redis`, 10)
	if err != nil {
		t.Fatalf("la búsqueda keyword no debe fallar con caracteres especiales: %v", err)
	}
	if len(res) == 0 {
		t.Error("debería encontrar la observación pese a los caracteres especiales")
	}
}

func TestCountObservations(t *testing.T) {
	e := newTestEngine(t)
	if n, err := e.CountObservations(); err != nil || n != 0 {
		t.Fatalf("DB vacía debe contar 0, obtuve %d (err=%v)", n, err)
	}
	if err := e.SaveObservation("a", "t", "uno", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("b", "t", "dos", nil); err != nil {
		t.Fatal(err)
	}
	if n, err := e.CountObservations(); err != nil || n != 2 {
		t.Fatalf("esperaba 2 observaciones, obtuve %d (err=%v)", n, err)
	}
}

func TestRecallNoBumpKeepsStats(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a1", "t", "alpha beta", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Recall read-only (inyección por turno): no debe contar como acceso.
	if _, err := e.Recall("alpha", RecallOptions{NoBump: true}); err != nil {
		t.Fatalf("Recall error: %v", err)
	}

	var count int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id=?`, "a1").Scan(&count); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 0 {
		t.Errorf("recall con NoBump no debe incrementar access_count, obtuve %d", count)
	}
}

func TestRecallImportanceBoost(t *testing.T) {
	e := newTestEngine(t)
	// Mismo contenido (igual relevancia keyword), distinta importancia.
	if err := e.SaveObservationWithImportance("low", "t", "alpha beta gamma", 1.0, nil); err != nil {
		t.Fatalf("save low error: %v", err)
	}
	if err := e.SaveObservationWithImportance("high", "t", "alpha beta gamma", 5.0, nil); err != nil {
		t.Fatalf("save high error: %v", err)
	}

	res, err := e.Recall("alpha", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if res.Count < 1 || res.Items[0].ID != "high" {
		t.Errorf("esperaba que el de mayor importancia rankee primero, obtuve %+v", res.Items)
	}
}

func TestRecallNoMatchEmpty(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t", "alpha", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}
	res, err := e.Recall("zzzznomatch", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("esperaba 0 resultados sin match, obtuve %d", res.Count)
	}
}

func TestArchivedExcludedFromRecallAndSearch(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("vis", "t", "memoria visible sobre kubernetes", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("arc", "t", "memoria archivada sobre kubernetes", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1 WHERE id='arc'`); err != nil {
		t.Fatal(err)
	}

	res, err := e.Recall("kubernetes", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("esperaba 1 item (solo visible), obtuve %d", res.Count)
	}
	for _, it := range res.Items {
		if it.ID == "arc" {
			t.Error("recall no debería devolver memorias archivadas")
		}
	}

	fts, err := e.SearchObservationsFTS("kubernetes", 10)
	if err != nil {
		t.Fatalf("fts error: %v", err)
	}
	if len(fts) != 1 || fts[0].ID != "vis" {
		t.Errorf("keyword debería excluir archivadas, obtuve %+v", fts)
	}
}

func TestScoreCandidatesFusion(t *testing.T) {
	// 'c' tiene la peor posición keyword pero importancia alta: debe ganar.
	cands := []candidate{
		{id: "a", accessCount: 0, importance: 1},
		{id: "b", accessCount: 100, importance: 1},
		{id: "c", accessCount: 0, importance: 10},
	}
	scored := scoreCandidates(cands)
	if len(scored) != 3 {
		t.Fatalf("esperaba 3 scored, obtuve %d", len(scored))
	}
	if scored[0].id != "c" {
		t.Errorf("esperaba 'c' primero por importancia, obtuve %s", scored[0].id)
	}
}
