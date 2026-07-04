package memory

import (
	"reflect"
	"testing"
)

// factInvolves indica si un hecho toca ambas entidades (en cualquier dirección).
func factInvolves(f Fact, x, y string) bool {
	return (f.Subject == x && f.Object == y) || (f.Subject == y && f.Object == x)
}

// Escenario (a): cadena A-knows-B-knows-C-knows-D. Desde A, PPR rankea los hechos por
// cercanía asociativa: (A,B) > (B,C) > (C,D).
func TestPageRankChainOrder(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "knows", "B")
	mustFact(t, e, "B", "knows", "C")
	mustFact(t, e, "C", "knows", "D")

	res, err := e.RecallFacts("A", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatalf("RecallFacts pagerank: %v", err)
	}
	if res.Count != 3 {
		t.Fatalf("esperaba 3 hechos, obtuve %d: %+v", res.Count, res.Facts)
	}
	if !factInvolves(res.Facts[0], "A", "B") {
		t.Errorf("el hecho más relevante desde A debe ser (A,B), obtuve %+v", res.Facts[0])
	}
	if !factInvolves(res.Facts[2], "C", "D") {
		t.Errorf("el hecho menos relevante debe ser (C,D), obtuve %+v", res.Facts[2])
	}
}

// Escenario (b): rank=” y rank='bfs' son equivalentes entre sí y al BFS histórico.
func TestPageRankEmptyRankEqualsBFS(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "Alice", "works_at", "ACME")
	mustFact(t, e, "Alice", "knows", "Bob")
	mustFact(t, e, "ACME", "located_in", "NYC")

	empty, err := e.RecallFacts("Alice", 2, 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	bfs, err := e.RecallFacts("Alice", 2, 50, "", "bfs")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(empty, bfs) {
		t.Errorf("rank='' y rank='bfs' deben ser idénticos:\n empty=%+v\n bfs=%+v", empty, bfs)
	}
	// Y coincide con el comportamiento BFS conocido (3 hechos a 2 hops).
	if empty.Count != 3 {
		t.Errorf("BFS a 2 hops debe dar 3 hechos, obtuve %d", empty.Count)
	}
}

// Escenario (c): semilla inexistente → vacío sin error, también en modo pagerank.
func TestPageRankUnknownSeedEmpty(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "knows", "B")

	res, err := e.RecallFacts("Zzz", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatalf("semilla inexistente no debe fallar: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("semilla inexistente debe dar 0 hechos, obtuve %d: %+v", res.Count, res.Facts)
	}
}

// Escenario (d): grafo desconectado {A-B} y {X-Y}. Desde A, la otra componente queda con
// score 0 y, al cortar por maxFacts, se excluye.
func TestPageRankDisconnectedComponentExcluded(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "knows", "B")
	mustFact(t, e, "X", "knows", "Y")

	// maxFacts=1: sólo debe entrar el hecho de la componente de A.
	res, err := e.RecallFacts("A", 0, 1, "", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 {
		t.Fatalf("maxFacts=1 debe devolver 1 hecho, obtuve %d", res.Count)
	}
	if !factInvolves(res.Facts[0], "A", "B") {
		t.Errorf("el único hecho debe ser (A,B), obtuve %+v", res.Facts[0])
	}
	// Con maxFacts amplio, (X,Y) queda al fondo (score 0), nunca por encima de (A,B).
	full, _ := e.RecallFacts("A", 0, 50, "", "pagerank")
	if !factInvolves(full.Facts[0], "A", "B") {
		t.Errorf("(A,B) debe rankear primero sobre la componente desconectada, obtuve %+v", full.Facts)
	}
}

// Escenario (e): hub compartido. Desde A, el hecho (A,hub) es el más relevante.
func TestPageRankSharedHub(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "rel", "hub")
	mustFact(t, e, "C", "rel", "hub")
	mustFact(t, e, "D", "rel", "hub")

	res, err := e.RecallFacts("A", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 3 {
		t.Fatalf("esperaba 3 hechos, obtuve %d", res.Count)
	}
	if !factInvolves(res.Facts[0], "A", "hub") {
		t.Errorf("desde A, (A,hub) debe rankear primero, obtuve %+v", res.Facts[0])
	}
}

// Escenario (f): PageRank point-in-time. Un hecho invalidado antes de as_of reaparece en el
// grafo PPR de esa fecha (composición con lo bi-temporal).
func TestPageRankPointInTime(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	if _, err := e.SaveFact("Ana", "works_at", "Acme", "2020-01-01", sv); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFact("Ana", "works_at", "Globex", "2023-01-01", sv); err != nil {
		t.Fatal(err)
	}

	// Verdad actual con pagerank: sólo Globex.
	cur, err := e.RecallFacts("Ana", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	if cur.Count != 1 || cur.Facts[0].Object != "Globex" {
		t.Fatalf("pagerank verdad actual debe ser Globex, obtuve %+v", cur.Facts)
	}
	// Point-in-time 2021 con pagerank: Acme era la verdad.
	past, err := e.RecallFacts("Ana", 0, 50, "2021-01-01", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	if past.Count != 1 || past.Facts[0].Object != "Acme" {
		t.Fatalf("pagerank as_of=2021 debe devolver Acme, obtuve %+v", past.Facts)
	}
}

// Escenario (g) + determinismo: dos exports del mismo grafo son idénticos; grafo vacío y
// nodo único no rompen la iteración.
func TestPageRankDeterministicAndDegenerate(t *testing.T) {
	e := newTestEngine(t)
	mustFact(t, e, "A", "knows", "B")
	mustFact(t, e, "B", "knows", "C")

	a, err := e.RecallFacts("A", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := e.RecallFacts("A", 0, 50, "", "pagerank")
	if !reflect.DeepEqual(a, b) {
		t.Error("dos recalls pagerank del mismo grafo deben ser idénticos (determinista)")
	}

	// Grafo vacío: PPR sobre un pprGraph sin nodos no debe entrar en pánico.
	empty := &pprGraph{index: map[int64]int{}}
	if scores := personalizedPageRank(empty, 0); len(scores) != 0 {
		t.Errorf("PPR sobre grafo vacío debe dar mapa vacío, obtuve %v", scores)
	}

	// Semilla fuera de rango: mapa vacío, sin pánico.
	g := &pprGraph{ids: []int64{1}, index: map[int64]int{1: 0}, adj: [][]int{nil}}
	if scores := personalizedPageRank(g, 5); len(scores) != 0 {
		t.Errorf("semilla fuera de rango debe dar mapa vacío, obtuve %v", scores)
	}

	// Nodo único colgante (sin aristas): converge, toda la masa se queda en él.
	if scores := personalizedPageRank(g, 0); len(scores) != 1 {
		t.Errorf("nodo único debe producir 1 score, obtuve %v", scores)
	}
}

// PPR sobre una entidad que existe pero no tiene relaciones vivas → vacío sin pánico.
func TestPageRankEntityWithoutLiveRelations(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}
	// Ana->Acme y luego Ana->Globex invalida Acme; Acme queda sin relaciones vivas.
	e.SaveFact("Ana", "works_at", "Acme", "", sv)
	e.SaveFact("Ana", "works_at", "Globex", "", sv)

	// Desde Acme (existe como entidad, pero su única relación está invalidada).
	res, err := e.RecallFacts("Acme", 0, 50, "", "pagerank")
	if err != nil {
		t.Fatalf("no debe fallar: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("Acme no tiene relaciones vivas, esperaba 0 hechos, obtuve %+v", res.Facts)
	}
}
