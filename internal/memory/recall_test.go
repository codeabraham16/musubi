package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// testNow es el reloj FIJO de los tests de scoring: scoreCandidates recibe `now` inyectado (no llama
// a time.Now() adentro) justamente para seguir siendo una función pura y determinista.
func testNow() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) }

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

// TestCountSavedItems verifica que la señal de captura sube ante cualquiera de las TRES
// superficies (observación, hecho, code), no solo observaciones (Frente #3 d).
func TestCountSavedItems(t *testing.T) {
	e := newTestEngine(t)
	base, err := e.CountSavedItems()
	if err != nil {
		t.Fatal(err)
	}

	if err := e.SaveObservation("o1", "t", "una observación", nil); err != nil {
		t.Fatal(err)
	}
	afterObs, _ := e.CountSavedItems()
	if afterObs <= base {
		t.Fatalf("guardar una observación debe subir el conteo: base=%d after=%d", base, afterObs)
	}

	if _, err := e.SaveFact("Ana", "trabaja_en", "Acme", "", nil); err != nil {
		t.Fatal(err)
	}
	afterFact, _ := e.CountSavedItems()
	if afterFact <= afterObs {
		t.Fatalf("guardar un hecho debe subir el conteo: obs=%d fact=%d", afterObs, afterFact)
	}

	if err := e.SaveCodeMemory(CodeMemory{Path: "a.go", Gist: "gist", Fingerprint: "f1", Tokens: 1}); err != nil {
		t.Fatal(err)
	}
	afterCode, _ := e.CountSavedItems()
	if afterCode <= afterFact {
		t.Fatalf("guardar un gist de código debe subir el conteo: fact=%d code=%d", afterFact, afterCode)
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

// TestScoreCandidatesImportanceNoOverride verifica el fix Q3: la importancia dejó de ser un
// multiplicador sin techo que ANULABA la relevancia. 'c' tiene importancia máxima (10) pero la peor
// posición en todos los pools; 'a' tiene importancia default pero la mejor relevancia keyword. La
// relevancia DEBE dominar — 'a' gana. (Antes, con `rrf*imp`, 'c' barría a 'a': ése era el bug.)
func TestScoreCandidatesImportanceNoOverride(t *testing.T) {
	cands := []candidate{
		{id: "a", accessCount: 0, importance: 1},
		{id: "b", accessCount: 100, importance: 1},
		{id: "c", accessCount: 0, importance: 10},
	}
	lexRank := map[string]int{"a": 0, "b": 1, "c": 2}
	scored := scoreCandidates(cands, lexRank, nil, nil, nil, testNow())
	if len(scored) != 3 {
		t.Fatalf("esperaba 3 scored, obtuve %d", len(scored))
	}
	if scored[0].id != "a" {
		t.Errorf("la relevancia debe dominar: esperaba 'a' primero (mejor pool), obtuve %s", scored[0].id)
	}
	// El importance:10 con la peor relevancia NO debe treparse al tope solo por importancia.
	if scored[0].id == "c" {
		t.Error("importance:10 no debe overridear una relevancia claramente superior (Q3)")
	}
}

// TestImportanceRankDense verifica el rango denso tie-aware (Q3.a/R2/R3): igual importancia efectiva
// ⇒ mismo rango; el rango sólo incrementa al bajar de valor; importance<=0 normaliza a 1.0.
func TestImportanceRankDense(t *testing.T) {
	cands := []candidate{
		{id: "hi", importance: 10},
		{id: "mid", importance: 5},
		{id: "def", importance: 1},
		{id: "zero", importance: 0}, // normaliza a 1.0 ⇒ empata con 'def'
	}
	r := importanceRank(cands)
	want := map[string]int{"hi": 0, "mid": 1, "def": 2, "zero": 2}
	for id, w := range want {
		if r[id] != w {
			t.Errorf("importanceRank[%s] = %d, esperaba %d (rangos densos %v)", id, r[id], w, want)
		}
	}
}

// TestScoreCandidatesImportanceTiebreak verifica R5/Q3.b: a igual rango en TODOS los pools, la mayor
// importancia desempata (rankea primero). La importancia conserva su rol de desempate.
func TestScoreCandidatesImportanceTiebreak(t *testing.T) {
	cands := []candidate{
		{id: "low", accessCount: 3, importance: 1},
		{id: "high", accessCount: 3, importance: 5},
	}
	// Mismo rango léxico para ambos ⇒ sólo la importancia los diferencia.
	scored := scoreCandidates(cands, map[string]int{"low": 0, "high": 0}, nil, nil, nil, testNow())
	if scored[0].id != "high" {
		t.Errorf("a igual relevancia, la mayor importancia debe desempatar: esperaba 'high', obtuve %s", scored[0].id)
	}
}

// TestScoreCandidatesImportanceUniform verifica R4/Q3.d: con importancia uniforme el término de
// importancia es constante y NO altera el orden — lo fija el pool léxico.
func TestScoreCandidatesImportanceUniform(t *testing.T) {
	cands := []candidate{
		{id: "a", importance: 1},
		{id: "b", importance: 1},
		{id: "c", importance: 1},
	}
	// Mejor lexRank ⇒ debe rankear primero, sin que la importancia (uniforme) lo altere.
	scored := scoreCandidates(cands, map[string]int{"a": 2, "b": 0, "c": 1}, nil, nil, nil, testNow())
	if scored[0].id != "b" {
		t.Errorf("con importancia uniforme el orden lo fija el lexRank: esperaba 'b' primero, obtuve %s", scored[0].id)
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

	withKeyword := scoreCandidates(cands, full, nil, nil, nil, testNow())
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
	noKeyword := scoreCandidates(cands, nil, nil, nil, nil, testNow())
	for _, id := range []string{"a", "b", "c"} {
		if scoreOf(withKeyword, id) <= scoreOf(noKeyword, id) {
			t.Errorf("%s: con lexRank el score debe ser mayor que sin él (%v vs %v)",
				id, scoreOf(withKeyword, id), scoreOf(noKeyword, id))
		}
	}
	// Un id ausente del lexRank no recibe término keyword (igual que nil para ese id).
	partial := map[string]int{"a": 0} // solo 'a' tiene rank keyword
	mixed := scoreCandidates(cands, partial, nil, nil, nil, testNow())
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
	base := scoreCandidates(cands, nil, nil, nil, nil, testNow())
	withVec := scoreCandidates(cands, nil, map[string]int{"a": 0}, nil, nil, testNow()) // solo 'a' tiene rango vectorial
	if scoreOf(withVec, "a") <= scoreOf(base, "a") {
		t.Errorf("'a' con rango vectorial debe sumar término RRF (%v vs %v)", scoreOf(withVec, "a"), scoreOf(base, "a"))
	}
	if scoreOf(withVec, "b") != scoreOf(base, "b") {
		t.Errorf("'b' ausente del vecRank no debe cambiar (%v vs %v)", scoreOf(withVec, "b"), scoreOf(base, "b"))
	}
}

// TestScoreCandidatesGraphSignal verifica la 5ª señal RRF (B4): un candidato con rango de
// centralidad de grafo suma ese término; uno ausente del graphRank no. Simétrico a las otras.
func TestScoreCandidatesGraphSignal(t *testing.T) {
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
	base := scoreCandidates(cands, nil, nil, nil, nil, testNow())
	withGraph := scoreCandidates(cands, nil, nil, map[string]int{"a": 0}, nil, testNow()) // solo 'a' tiene centralidad
	if scoreOf(withGraph, "a") <= scoreOf(base, "a") {
		t.Errorf("'a' con rango de centralidad debe sumar término RRF (%v vs %v)", scoreOf(withGraph, "a"), scoreOf(base, "a"))
	}
	if scoreOf(withGraph, "b") != scoreOf(base, "b") {
		t.Errorf("'b' ausente del graphRank no debe cambiar (%v vs %v)", scoreOf(withGraph, "b"), scoreOf(base, "b"))
	}
	// graphRank=nil ⇒ idéntico al histórico (equivalencia R1).
	if scoreOf(scoreCandidates(cands, nil, nil, nil, nil, testNow()), "a") != scoreOf(base, "a") {
		t.Error("graphRank=nil debe dar score idéntico al histórico")
	}
}

// TestRecallGraphCentralityNoRelationsEquivalent verifica la equivalencia end-to-end (R1/R6):
// con GraphCentrality on pero sin relaciones (grafo vacío), el recall debe dar EXACTAMENTE el
// mismo resultado que con la señal apagada.
func TestRecallGraphCentralityNoRelationsEquivalent(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"o1", "o2", "o3"} {
		saveObs(t, e, id)
	}
	off, err := e.Recall(context.Background(), "contenido", RecallOptions{NoBump: true})
	if err != nil {
		t.Fatalf("recall off: %v", err)
	}
	on, err := e.Recall(context.Background(), "contenido", RecallOptions{NoBump: true, GraphCentrality: true})
	if err != nil {
		t.Fatalf("recall on: %v", err)
	}
	if len(on.Items) != len(off.Items) {
		t.Fatalf("sin relaciones el recall debe ser idéntico: %d vs %d items", len(on.Items), len(off.Items))
	}
	for i := range off.Items {
		if on.Items[i].ID != off.Items[i].ID || on.Items[i].Score != off.Items[i].Score {
			t.Errorf("item %d difiere con la señal on sin relaciones: %+v vs %+v", i, on.Items[i], off.Items[i])
		}
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
