package memory

import (
	"context"
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

	res, err := e.Recall(context.Background(), "observacion", RecallOptions{TokenBudget: 30})
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

	res, err := e.Recall(context.Background(), "resumen", RecallOptions{})
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

	if _, err := e.Recall(context.Background(), "alpha", RecallOptions{}); err != nil {
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
	res, err := e.SearchObservationsFTS(context.Background(), `postgres "y (redis`, 10)
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
	if _, err := e.Recall(context.Background(), "alpha", RecallOptions{NoBump: true}); err != nil {
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

	res, err := e.Recall(context.Background(), "alpha", RecallOptions{})
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
	res, err := e.Recall(context.Background(), "zzzznomatch", RecallOptions{})
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

	res, err := e.Recall(context.Background(), "kubernetes", RecallOptions{})
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

	fts, err := e.SearchObservationsFTS(context.Background(), "kubernetes", 10)
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
	// lexRank por orden del slice = el comportamiento keyword-meaningful histórico.
	lexRank := map[string]int{"a": 0, "b": 1, "c": 2}
	scored := scoreCandidates(cands, lexRank, nil)
	if len(scored) != 3 {
		t.Fatalf("esperaba 3 scored, obtuve %d", len(scored))
	}
	if scored[0].id != "c" {
		t.Errorf("esperaba 'c' primero por importancia, obtuve %s", scored[0].id)
	}
}

// TestScoreCandidatesLexRankEquivalence verifica la equivalencia del refactor multi-pool
// (T5.7 R1): pasar un lexRank por orden de slice == el viejo keywordMeaningful=true; pasar
// nil == keywordMeaningful=false (omite el término keyword). Bit-idéntico al histórico.
func TestScoreCandidatesLexRankEquivalence(t *testing.T) {
	cands := []candidate{
		{id: "a", accessCount: 0, importance: 1},
		{id: "b", accessCount: 5, importance: 1},
		{id: "c", accessCount: 0, importance: 1},
	}
	full := map[string]int{"a": 0, "b": 1, "c": 2}

	withKeyword := scoreCandidates(cands, full, nil)
	scoreOf := func(scored []scoredCandidate, id string) float64 {
		for _, s := range scored {
			if s.id == id {
				return s.score
			}
		}
		return -1
	}
	// Con lexRank completo, cada candidato suma su término keyword: score estrictamente
	// mayor que sin él.
	noKeyword := scoreCandidates(cands, nil, nil)
	for _, id := range []string{"a", "b", "c"} {
		if scoreOf(withKeyword, id) <= scoreOf(noKeyword, id) {
			t.Errorf("%s: con lexRank el score debe ser mayor que sin él (%v vs %v)",
				id, scoreOf(withKeyword, id), scoreOf(noKeyword, id))
		}
	}
	// Un id ausente del lexRank no recibe término keyword (igual que nil para ese id).
	partial := map[string]int{"a": 0} // solo 'a' tiene rank keyword
	mixed := scoreCandidates(cands, partial, nil)
	if scoreOf(mixed, "b") != scoreOf(noKeyword, "b") {
		t.Errorf("'b' ausente del lexRank no debe sumar término keyword: %v vs %v",
			scoreOf(mixed, "b"), scoreOf(noKeyword, "b"))
	}
	if scoreOf(mixed, "a") <= scoreOf(noKeyword, "a") {
		t.Errorf("'a' presente en lexRank sí debe sumar término keyword")
	}
}

// TestScoreCandidatesVectorSignal verifica la 4ta señal RRF (T5.7 R2): un candidato con
// rango vectorial suma ese término; uno ausente del vecRank no.
func TestScoreCandidatesVectorSignal(t *testing.T) {
	cands := []candidate{
		{id: "a", importance: 1},
		{id: "b", importance: 1},
	}
	scoreOf := func(scored []scoredCandidate, id string) float64 {
		for _, s := range scored {
			if s.id == id {
				return s.score
			}
		}
		return -1
	}
	base := scoreCandidates(cands, nil, nil)
	withVec := scoreCandidates(cands, nil, map[string]int{"a": 0}) // solo 'a' tiene rango vectorial
	if scoreOf(withVec, "a") <= scoreOf(base, "a") {
		t.Errorf("'a' con rango vectorial debe sumar término RRF (%v vs %v)", scoreOf(withVec, "a"), scoreOf(base, "a"))
	}
	if scoreOf(withVec, "b") != scoreOf(base, "b") {
		t.Errorf("'b' ausente del vecRank no debe cambiar (%v vs %v)", scoreOf(withVec, "b"), scoreOf(base, "b"))
	}
}

// TestRecallHybridUnionViaVector verifica el recall híbrido end-to-end (T5.7 R2): una obs
// que NO matchea la query léxica pero está cerca en el espacio vectorial entra al resultado
// por el pool vectorial (union, no intersección).
func TestRecallHybridUnionViaVector(t *testing.T) {
	e := newTestEngine(t)
	// 'a' matchea FTS "kubernetes"; 'b' no comparte palabras pero su vector == queryVec.
	if err := e.SaveObservation("a", "t", "despliegue en kubernetes", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("b", "t", "orquestacion de contenedores", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	queryVec := []float32{0, 1, 0} // coseno máximo con 'b'

	// Sin vector: recall léxico solo encuentra 'a'.
	lex, err := e.Recall(context.Background(), "kubernetes", RecallOptions{NoBump: true})
	if err != nil {
		t.Fatalf("Recall léxico error: %v", err)
	}
	if has(lex.Items, "b") {
		t.Fatalf("sin vector, 'b' (sin match léxico) no debería aparecer: %+v", lex.Items)
	}

	// Híbrido: el pool vectorial trae 'b' aunque no matchee FTS.
	hyb, err := e.Recall(context.Background(), "kubernetes", RecallOptions{QueryVector: queryVec, NoBump: true})
	if err != nil {
		t.Fatalf("Recall híbrido error: %v", err)
	}
	if !has(hyb.Items, "a") {
		t.Errorf("el híbrido debe conservar el match léxico 'a': %+v", hyb.Items)
	}
	if !has(hyb.Items, "b") {
		t.Errorf("el híbrido debe traer 'b' por el pool vectorial (union): %+v", hyb.Items)
	}
}

func has(items []RecallItem, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}
