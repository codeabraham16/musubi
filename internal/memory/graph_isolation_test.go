package memory

import (
	"context"
	"testing"
)

// graph_isolation_test.go es el guard de regresión del AISLAMIENTO multi-tenant del GRAFO DE
// HECHOS (Track 17, migración v14). Cubre las dos garantías nuevas: (1) LECTURA — el traversal
// (BFS, pagerank, camino) y entity_context sólo ven las aristas del proyecto + las sin atribuir;
// (2) ESCRITURA — la invalidación por cardinalidad de un predicado funcional queda acotada al
// proyecto de origen (un save en A jamás cierra un hecho vivo de B). Las ENTIDADES son globales:
// lo que se aísla son las aristas.

// objectsOf indexa los objetos de un GraphResult (para aserciones set-wise).
func objectsOf(facts []Fact) map[string]bool {
	m := map[string]bool{}
	for _, f := range facts {
		m[f.Object] = true
	}
	return m
}

// TestFactsReadNoBleed: recall_facts (BFS) con scope de proyecto sólo ve las aristas propias +
// las sin atribuir; federado ve todas. Mismo criterio que el aislamiento de observaciones.
func TestFactsReadNoBleed(t *testing.T) {
	e := newTestEngine(t)

	// Misma entidad-sujeto GLOBAL ("Server"), aristas atribuidas a distinto proyecto.
	mustFactFrom(t, e, "crm", "Server", "runs", "Rocky")
	mustFactFrom(t, e, "web", "Server", "runs", "Ubuntu")
	mustFactFrom(t, e, "", "Server", "located_in", "Datacenter") // sin atribuir: visible a todos

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	crmRes, err := e.RecallFactsCtx(crm, "Server", 1, 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := objectsOf(crmRes.Facts)
	if !got["Rocky"] || !got["Datacenter"] || got["Ubuntu"] {
		t.Errorf("crm esperaba {Rocky,Datacenter} SIN Ubuntu, obtuvo %v", got)
	}

	webRes, _ := e.RecallFactsCtx(web, "Server", 1, 50, "", "")
	got = objectsOf(webRes.Facts)
	if !got["Ubuntu"] || !got["Datacenter"] || got["Rocky"] {
		t.Errorf("web esperaba {Ubuntu,Datacenter} SIN Rocky, obtuvo %v", got)
	}

	// Federado (sin scope): ve las 3 aristas.
	fedRes, _ := e.RecallFacts("Server", 1, 50, "", "")
	got = objectsOf(fedRes.Facts)
	if !got["Rocky"] || !got["Ubuntu"] || !got["Datacenter"] {
		t.Errorf("federado esperaba los 3, obtuvo %v", got)
	}
}

// TestFactsCardinalityPerProject es el guard CRÍTICO: la invalidación por cardinalidad de un
// predicado funcional NO cruza proyectos. Un save en 'web' del mismo (sujeto, predicado) con otro
// objeto NO invalida el hecho vivo de 'crm', y viceversa. Dentro de un proyecto, la cardinalidad
// sigue funcionando.
func TestFactsCardinalityPerProject(t *testing.T) {
	e := newTestEngine(t)
	sv := []string{"works_at"}

	// crm: Ana trabaja en Acme. web: Ana trabaja en Globex. MISMO sujeto/predicado, distinto
	// proyecto ⇒ ninguno invalida al otro (cada uno es el primero de SU proyecto).
	r1, err := e.SaveFactFrom("crm", "Ana", "works_at", "Acme", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Invalidated != 0 {
		t.Errorf("primer hecho de crm no debe invalidar nada, obtuvo %d", r1.Invalidated)
	}
	r2, err := e.SaveFactFrom("web", "Ana", "works_at", "Globex", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Invalidated != 0 {
		t.Errorf("save de web NO debe invalidar el hecho vivo de crm (aislamiento de escritura), obtuvo %d", r2.Invalidated)
	}

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	// Cada proyecto ve su verdad actual, viva.
	if got := objectsOf(mustRecall(t, e, crm, "Ana")); !got["Acme"] || got["Globex"] {
		t.Errorf("crm debe ver Acme vivo y NO Globex, obtuvo %v", got)
	}
	if got := objectsOf(mustRecall(t, e, web, "Ana")); !got["Globex"] || got["Acme"] {
		t.Errorf("web debe ver Globex vivo y NO Acme, obtuvo %v", got)
	}

	// Dentro de crm la cardinalidad SÍ funciona: Ana pasa a Beta ⇒ invalida Acme (de crm), sin
	// tocar Globex (de web).
	r3, err := e.SaveFactFrom("crm", "Ana", "works_at", "Beta", "", sv)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Invalidated != 1 {
		t.Errorf("dentro de crm, Beta debe invalidar Acme (1), obtuvo %d", r3.Invalidated)
	}
	if got := objectsOf(mustRecall(t, e, crm, "Ana")); !got["Beta"] || got["Acme"] {
		t.Errorf("crm ahora debe ver Beta y NO Acme, obtuvo %v", got)
	}
	if got := objectsOf(mustRecall(t, e, web, "Ana")); !got["Globex"] {
		t.Errorf("web sigue viendo Globex vivo (intacto), obtuvo %v", got)
	}
}

// TestFactPathProjectScope: el camino más corto sólo cruza aristas visibles al proyecto. Un
// camino que existe únicamente por aristas de 'crm' no lo encuentra 'web' (aunque las entidades
// sean globales).
func TestFactPathProjectScope(t *testing.T) {
	e := newTestEngine(t)
	mustFactFrom(t, e, "crm", "A", "link", "B")
	mustFactFrom(t, e, "crm", "B", "link", "C")

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	crmPath, err := e.FactPathCtx(crm, "A", "C", 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if crmPath.Count != 2 {
		t.Errorf("crm debe encontrar el camino A->B->C (2 aristas), obtuvo %d: %+v", crmPath.Count, crmPath.Facts)
	}

	// web ve las entidades A/B/C (globales) pero ninguna arista ⇒ sin camino.
	webPath, err := e.FactPathCtx(web, "A", "C", 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if webPath.Count != 0 {
		t.Errorf("web NO debe encontrar camino (no ve las aristas de crm), obtuvo %d: %+v", webPath.Count, webPath.Facts)
	}
}

// TestFactsPageRankProjectScope: el recall asociativo (rank='pagerank') respeta el scope igual
// que el BFS, porque comparte el mismo filtro combinado temporal+proyecto.
func TestFactsPageRankProjectScope(t *testing.T) {
	e := newTestEngine(t)
	mustFactFrom(t, e, "crm", "Hub", "connects", "Alpha")
	mustFactFrom(t, e, "web", "Hub", "connects", "Beta")

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	res, err := e.RecallFactsCtx(crm, "Hub", 2, 50, "", "pagerank")
	if err != nil {
		t.Fatal(err)
	}
	got := objectsOf(res.Facts)
	if !got["Alpha"] || got["Beta"] {
		t.Errorf("pagerank en crm esperaba Alpha SIN Beta, obtuvo %v", got)
	}
}

// TestEntityContextProjectScope: entity_context aísla TANTO los hechos (vía RecallFactsCtx) como
// las observaciones (vía observationGistsCtx). crm no ve ni la arista ni la prosa de web.
func TestEntityContextProjectScope(t *testing.T) {
	e := newTestEngine(t)

	mustFactFrom(t, e, "crm", "Payments", "uses", "Stripe")
	mustFactFrom(t, e, "web", "Payments", "uses", "Adyen")

	// Observaciones que mencionan la entidad, atribuidas a cada proyecto.
	if err := e.SaveObservationTypedFrom("crm", "", "o-crm", "t/pay", "Payments module notes for crm", 1, "", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom("web", "", "o-web", "t/pay", "Payments module notes for web", 1, "", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	res, err := e.EntityContextCtx(crm, "Payments", 1, 50, 10)
	if err != nil {
		t.Fatal(err)
	}

	// Hechos: sólo Stripe (crm), no Adyen (web).
	if got := objectsOf(res.Facts); !got["Stripe"] || got["Adyen"] {
		t.Errorf("entity_context de crm: hechos esperaba Stripe SIN Adyen, obtuvo %v", got)
	}
	// Observaciones: sólo la de crm.
	obsIDs := map[string]bool{}
	for _, o := range res.Observations {
		obsIDs[o.ID] = true
	}
	if !obsIDs["o-crm"] || obsIDs["o-web"] {
		t.Errorf("entity_context de crm: obs esperaba {o-crm} SIN o-web, obtuvo %v", obsIDs)
	}
}

// TestFactsUnattributedVisibleToAll: los hechos sin atribuir ('' — legacy tras la migración v14, o
// escritos por admin/stdio) son visibles a CUALQUIER proyecto (espacio federado compartido).
func TestFactsUnattributedVisibleToAll(t *testing.T) {
	e := newTestEngine(t)
	// SaveFact (wrapper) escribe con project_id='' — simula el dato legacy migrado a ''.
	mustFact(t, e, "Company", "founded_in", "2020")

	for _, pid := range []string{"crm", "web", "anything"} {
		ctx := WithProjectScope(context.Background(), ProjectScope{ProjectID: pid})
		if got := objectsOf(mustRecall(t, e, ctx, "Company")); !got["2020"] {
			t.Errorf("proyecto %q debe ver el hecho sin atribuir, obtuvo %v", pid, got)
		}
	}
}

// --- helpers ---

func mustFactFrom(t *testing.T, e *DbEngine, origin, s, p, o string) {
	t.Helper()
	if _, err := e.SaveFactFrom(origin, s, p, o, "", nil); err != nil {
		t.Fatalf("SaveFactFrom(%q,%s,%s,%s) error: %v", origin, s, p, o, err)
	}
}

func mustRecall(t *testing.T, e *DbEngine, ctx context.Context, entity string) []Fact {
	t.Helper()
	res, err := e.RecallFactsCtx(ctx, entity, 2, 50, "", "")
	if err != nil {
		t.Fatalf("RecallFactsCtx(%s): %v", entity, err)
	}
	return res.Facts
}
